package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openclaw/wacli/internal/out"
	"github.com/openclaw/wacli/internal/store"
	"github.com/spf13/cobra"
)

func newMessagesWaitCmd(flags *rootFlags) *cobra.Command {
	var chat string
	var sender string
	var afterStr string
	var afterID string
	var fromMe bool
	var fromThem bool
	var count int
	var pollInterval time.Duration

	cmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait until matching messages are stored locally",
		Long: `Block until at least --count messages matching the filters are in the local
database, then print them oldest first. Another process, usually
"wacli sync --follow", must be storing new messages; wait only reads.

--after-id ID matches messages of --chat stored after message ID, such as the
id printed by "send text --json". It is the precise way to wait for a reply:
local storage order separates messages that share a timestamp second.

--after T matches messages timestamped at or after T at second precision,
including messages stored before wait started, so a reply that arrives between
send and wait is not missed.

Without either, only messages that were not already stored when wait started
match. The global --timeout bounds the wait; the command fails when it expires.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromMe && fromThem {
				return fmt.Errorf("--from-me and --from-them are mutually exclusive")
			}
			if afterStr != "" && afterID != "" {
				return fmt.Errorf("--after and --after-id are mutually exclusive")
			}
			if afterID != "" && strings.TrimSpace(chat) == "" {
				return fmt.Errorf("--after-id requires --chat")
			}
			if count < 1 {
				return fmt.Errorf("--count must be at least 1")
			}
			if pollInterval <= 0 {
				return fmt.Errorf("--poll-interval must be positive")
			}
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, false, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			q := messageWaitQuery{params: store.ListMessagesParams{Asc: true}, afterID: strings.TrimSpace(afterID), count: count}
			if q.params.ChatJIDs, err = messageChatJIDFilter(ctx, a, chat); err != nil {
				return err
			}
			// The same PN/LID expansion as --chat: a group member may be stored
			// under either identity depending on what the session has learned.
			if q.params.SenderJIDs, err = messageChatJIDFilter(ctx, a, sender); err != nil {
				return err
			}
			switch {
			case fromMe:
				v := true
				q.params.FromMe = &v
			case fromThem:
				v := false
				q.params.FromMe = &v
			}
			switch {
			case q.afterID != "":
			case afterStr != "":
				after, err := parseTime(afterStr)
				if err != nil {
					return err
				}
				q.params.After = waitLowerBound(after)
			default:
				q.params.After = waitLowerBound(time.Now())
				existing, err := a.DB().ListMessages(withLimit(q.params, maxWaitBaseline))
				if err != nil {
					return err
				}
				q.exclude = make(map[string]bool, len(existing))
				for _, m := range existing {
					q.exclude[waitMessageKey(m)] = true
				}
			}

			// Poll through fresh read-only handles: a store opened while no writer
			// had it open is immutable to SQLite and would never show new rows.
			dbPath := filepath.Join(a.StoreDir(), "wacli.db")
			openDB := func() (*store.DB, error) { return store.OpenReadOnly(dbPath) }
			msgs, err := waitForMessages(ctx, openDB, q, pollInterval)
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("timed out after %s waiting for %d matching message(s)", flags.timeout, count)
			}
			if err != nil {
				return err
			}
			msgs = resolveMessageSenderNames(ctx, a, msgs)

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"messages": msgs})
			}
			return writeMessagesList(os.Stdout, msgs, fullTableOutput(flags.fullOutput))
		},
	}

	cmd.Flags().StringVar(&chat, "chat", "", "chat JID or phone number (matches its phone and LID forms)")
	cmd.Flags().StringVar(&sender, "sender", "", "sender JID or phone number (matches its phone and LID forms)")
	cmd.Flags().StringVar(&afterID, "after-id", "", "match messages of --chat stored after this message ID")
	cmd.Flags().StringVar(&afterStr, "after", "", "match messages at or after this time (RFC3339), including already stored ones")
	cmd.Flags().BoolVar(&fromMe, "from-me", false, "only messages sent by me")
	cmd.Flags().BoolVar(&fromThem, "from-them", false, "only messages received (not sent by me)")
	cmd.Flags().IntVar(&count, "count", 1, "number of matching messages to wait for")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 200*time.Millisecond, "how often to re-read the local database")
	return cmd
}

type messageWaitQuery struct {
	params store.ListMessagesParams
	// afterID anchors the wait at a stored message. Until that message is
	// stored nothing can follow it, so the wait keeps polling.
	afterID string
	// exclude holds messages already stored when a wait without a lower
	// bound started.
	exclude map[string]bool
	count   int
}

// maxWaitBaseline caps how many pre-existing messages wait remembers when it
// runs without a lower bound. Only messages in the current second can match
// both the baseline query and later polls, so the cap is not reached in practice.
const maxWaitBaseline = 1000

// waitLowerBound makes ListMessages' exclusive After inclusive of t's second.
// Stored timestamps have second precision, so a reply in the same second as t
// would otherwise be dropped.
func waitLowerBound(t time.Time) *time.Time {
	bound := time.Unix(t.Unix()-1, 0).UTC()
	return &bound
}

func waitMessageKey(m store.Message) string {
	return m.ChatJID + "\x00" + m.MsgID
}

func withLimit(p store.ListMessagesParams, limit int) store.ListMessagesParams {
	p.Limit = limit
	return p
}

func waitForMessages(ctx context.Context, openDB func() (*store.DB, error), q messageWaitQuery, interval time.Duration) ([]store.Message, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		msgs, err := pollWaitCandidates(openDB, q)
		if err != nil {
			return nil, err
		}
		if len(msgs) >= q.count {
			return msgs[:q.count], nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func pollWaitCandidates(openDB func() (*store.DB, error), q messageWaitQuery) ([]store.Message, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	params := withLimit(q.params, q.count+len(q.exclude))
	if q.afterID != "" {
		rowID, ts, found, err := db.MessageRowID(params.ChatJIDs, q.afterID)
		if err != nil || !found {
			return nil, err
		}
		// The timestamp bound keeps older history imported later out.
		params.AfterRowID, params.After = rowID, waitLowerBound(ts)
	}
	msgs, err := db.ListMessages(params)
	if err != nil {
		return nil, err
	}
	matched := msgs[:0]
	for _, m := range msgs {
		if !q.exclude[waitMessageKey(m)] {
			matched = append(matched, m)
		}
	}
	return matched, nil
}
