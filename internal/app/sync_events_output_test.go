package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/wacli/internal/out"
	"github.com/openclaw/wacli/internal/wa"
	"go.mau.fi/whatsmeow/types"
)

func TestSyncEventsOutputStaysNDJSONDuringProgress(t *testing.T) {
	a := newTestApp(t)
	f := newFakeWA()
	a.wa = f

	chat := types.JID{User: "123", Server: types.DefaultUserServer}
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	ids := make([]string, 25)
	for i := range ids {
		ids[i] = "m" + string(rune('a'+i))
	}
	f.connectEvents = []interface{}{historySyncWithTextMessages(chat, base, ids...)}

	raw := captureStderr(t, func() {
		a.opts.Events = out.NewEventWriter(os.Stderr, true)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := a.Sync(ctx, SyncOptions{
			Mode:         SyncModeOnce,
			AllowQR:      false,
			IdleExit:     50 * time.Millisecond,
			WarnNoLimits: false,
		})
		if err != nil {
			t.Fatalf("Sync: %v", err)
		}
	})

	if strings.Contains(raw, "Synced ") || strings.Contains(raw, "Processing history sync") {
		t.Fatalf("human progress leaked into --events stderr:\n%s", raw)
	}

	var sawProgress bool
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var evt map[string]any
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			t.Fatalf("stderr line is not valid JSON %q: %v\nfull stderr:\n%s", line, err, raw)
		}
		if evt["event"] == "progress" {
			data, ok := evt["data"].(map[string]any)
			if !ok || data["messages_synced"] != float64(25) {
				t.Fatalf("unexpected progress event: %#v", evt)
			}
			sawProgress = true
		}
	}
	if !sawProgress {
		t.Fatalf("expected progress event in:\n%s", raw)
	}
}

func TestSyncTTYProgressUsesSingleStatusLine(t *testing.T) {
	oldTerminal := syncStatusTerminal
	syncStatusTerminal = func() bool { return true }
	t.Cleanup(func() { syncStatusTerminal = oldTerminal })

	a := newTestApp(t)
	f := newFakeWA()
	a.wa = f

	chat := types.JID{User: "123", Server: types.DefaultUserServer}
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	ids := make([]string, 30)
	for i := range ids {
		ids[i] = "m" + string(rune('a'+i))
	}
	f.connectEvents = []interface{}{historySyncWithTextMessages(chat, base, ids...)}

	raw := captureStderr(t, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := a.Sync(ctx, SyncOptions{
			Mode:         SyncModeOnce,
			AllowQR:      false,
			IdleExit:     time.Millisecond,
			WarnNoLimits: false,
		})
		if err != nil {
			t.Fatalf("Sync: %v", err)
		}
	})

	if strings.Contains(raw, "\nProcessing history sync") || strings.Contains(raw, "\nSynced 25 messages") {
		t.Fatalf("TTY progress should update one status line, got:\n%q", raw)
	}
	if !strings.Contains(raw, "\rConnected. Waiting for history sync...") {
		t.Fatalf("missing connected status in:\n%q", raw)
	}
	if !strings.Contains(raw, "\rSyncing history: 1 conversations, 25 messages stored") {
		t.Fatalf("missing history progress status in:\n%q", raw)
	}
	if !strings.Contains(raw, "\rSyncing history: 1 conversations, 30 messages stored") {
		t.Fatalf("missing final history status in:\n%q", raw)
	}
}

func TestSyncTTYWarningBreaksThroughStatusLine(t *testing.T) {
	oldTerminal := syncStatusTerminal
	syncStatusTerminal = func() bool { return true }
	t.Cleanup(func() { syncStatusTerminal = oldTerminal })

	a := newTestApp(t)
	f := newFakeWA()
	f.appStateFetchErr = errors.New("not connected")
	a.wa = f

	raw := captureStderr(t, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := a.Sync(ctx, SyncOptions{
			Mode:         SyncModeOnce,
			AllowQR:      false,
			IdleExit:     time.Millisecond,
			WarnNoLimits: false,
		})
		if err != nil {
			t.Fatalf("Sync: %v", err)
		}
	})

	if !strings.Contains(raw, "warning: failed to sync WhatsApp app state regular_high: not connected\n") {
		t.Fatalf("missing regular_high warning in:\n%q", raw)
	}
	if !strings.Contains(raw, "warning: failed to sync WhatsApp app state regular_low: not connected\n") {
		t.Fatalf("missing regular_low warning in:\n%q", raw)
	}
	if strings.Contains(raw, "\nConnected.\n") {
		t.Fatalf("connected status should not become a separate noisy line:\n%q", raw)
	}
}

func TestSyncRefreshFailuresWarnAndContinue(t *testing.T) {
	a := newTestApp(t)
	f := newFakeWA()
	f.getAllContactsErr = errors.New("contacts offline")
	f.getJoinedGroupsErr = errors.New("groups offline")
	f.getSubscribedNewslettersErr = errors.New("channels offline")
	a.wa = f

	raw := captureStderr(t, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := a.Sync(ctx, SyncOptions{
			Mode:            SyncModeOnce,
			AllowQR:         false,
			IdleExit:        time.Millisecond,
			RefreshContacts: true,
			RefreshGroups:   true,
			RefreshChannels: true,
		})
		if err != nil {
			t.Fatalf("Sync: %v", err)
		}
	})

	for _, want := range []string{
		"warning: failed to refresh contacts: contacts offline\n",
		"warning: failed to refresh groups: groups offline\n",
		"warning: failed to refresh channels: channels offline\n",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("missing warning %q in:\n%q", want, raw)
		}
	}
}

func TestSyncLiveMessageEmitsMessageEvent(t *testing.T) {
	a := newTestApp(t)
	var b bytes.Buffer
	a.opts.Events = out.NewEventWriter(&b, true)

	chat := types.JID{User: "120363429631482848", Server: types.GroupServer}
	sender := types.JID{User: "553492009508", Server: types.DefaultUserServer}
	pm := wa.ParsedMessage{
		Chat:      chat,
		ID:        "TEST-MSG-1",
		SenderJID: sender.String(),
		PushName:  "Gastei",
		Timestamp: time.Now().UTC(),
		Text:      "Resumo disponível",
	}

	if err := a.InjectParsedMessage(context.Background(), pm); err != nil {
		t.Fatalf("InjectParsedMessage: %v", err)
	}

	outStr := b.String()
	if !strings.Contains(outStr, `"event":"message"`) {
		t.Fatalf("expected message event, got %q", outStr)
	}
	if !strings.Contains(outStr, `"text":"Resumo disponível"`) {
		t.Fatalf("expected text in message event, got %q", outStr)
	}
	if !strings.Contains(outStr, `"sender_name":"Gastei"`) {
		t.Fatalf("expected sender_name in message event, got %q", outStr)
	}
}

func TestSyncMockModeFollow(t *testing.T) {
	a := newTestApp(t)
	var b bytes.Buffer
	a.opts.Events = out.NewEventWriter(&b, true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	afterConnectDone := make(chan struct{})
	afterConnect := func(ctx context.Context) error {
		close(afterConnectDone)
		return nil
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := a.Sync(ctx, SyncOptions{
			Mode:         SyncModeFollow,
			Mock:         true,
			AfterConnect: afterConnect,
		})
		errCh <- err
	}()

	select {
	case <-afterConnectDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for afterConnect in mock follow")
	}

	pm := wa.ParsedMessage{
		Chat:      types.JID{User: "123", Server: types.DefaultUserServer},
		ID:        "MOCK-LIVE-1",
		SenderJID: "123@s.whatsapp.net",
		Timestamp: time.Now().UTC(),
		Text:      "Hello Mock",
	}
	if err := a.InjectParsedMessage(ctx, pm); err != nil {
		t.Fatalf("InjectParsedMessage: %v", err)
	}

	cancel()
	err := <-errCh
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Sync mock: %v", err)
	}

	outStr := b.String()
	if !strings.Contains(outStr, `"event":"message"`) || !strings.Contains(outStr, "Hello Mock") {
		t.Fatalf("expected message event in mock follow output, got %q", outStr)
	}
}
