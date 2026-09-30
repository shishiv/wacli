package main

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/wacli/internal/store"
)

const (
	waitGroup  = "120363000000000001@g.us"
	waitBotPN  = "15550002001@s.whatsapp.net"
	waitBotLID = "900000201@lid"
)

func seedWaitStore(t *testing.T) string {
	t.Helper()
	storeDir := t.TempDir()
	db := openSystemImportStore(t, storeDir)
	if err := db.UpsertChat(waitGroup, "group", "Casal", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	session, err := sql.Open("sqlite3", filepath.Join(storeDir, "session.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if _, err := session.Exec(`CREATE TABLE whatsmeow_lid_map (lid TEXT PRIMARY KEY, pn TEXT UNIQUE NOT NULL);
		INSERT INTO whatsmeow_lid_map VALUES ('900000201', '15550002001');`); err != nil {
		t.Fatal(err)
	}
	return storeDir
}

func storeWaitMessage(t *testing.T, storeDir string, p store.UpsertMessageParams) {
	t.Helper()
	if err := upsertWaitMessage(storeDir, p); err != nil {
		t.Fatal(err)
	}
}

// storeWaitMessageLater stores p while wait is polling, as sync --follow would.
func storeWaitMessageLater(t *testing.T, storeDir string, delay time.Duration, p store.UpsertMessageParams) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		time.Sleep(delay)
		p.Timestamp = time.Now()
		done <- upsertWaitMessage(storeDir, p)
	}()
	t.Cleanup(func() {
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
}

func upsertWaitMessage(storeDir string, p store.UpsertMessageParams) error {
	db, err := store.Open(filepath.Join(storeDir, "wacli.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	if p.ChatJID == "" {
		p.ChatJID = waitGroup
	}
	return db.UpsertMessage(p)
}

func runMessagesWait(t *testing.T, storeDir string, timeout time.Duration, args ...string) ([]string, error) {
	t.Helper()
	cmd := newMessagesWaitCmd(&rootFlags{storeDir: storeDir, readOnly: true, asJSON: true, timeout: timeout})
	cmd.SetArgs(append(args, "--poll-interval", "20ms"))
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	var runErr error
	raw := captureRootStdout(t, func() { runErr = cmd.Execute() })
	if runErr != nil {
		return nil, runErr
	}
	var result struct {
		Data struct{ Messages []store.Message }
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	ids := make([]string, 0, len(result.Data.Messages))
	for _, m := range result.Data.Messages {
		ids = append(ids, m.MsgID)
	}
	return ids, nil
}

func TestMessagesWaitBlocksUntilReplyIsStored(t *testing.T) {
	storeDir := seedWaitStore(t)
	sent := time.Now()
	storeWaitMessage(t, storeDir, store.UpsertMessageParams{MsgID: "mine", FromMe: true, Timestamp: sent, Text: "gastei 10 no mercado"})
	storeWaitMessageLater(t, storeDir, 150*time.Millisecond, store.UpsertMessageParams{MsgID: "bot-reply", SenderJID: waitBotPN, Text: "Anotado"})

	ids, err := runMessagesWait(t, storeDir, 5*time.Second, "--chat", waitGroup, "--from-them", "--after", sent.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ids, ","); got != "bot-reply" {
		t.Fatalf("messages = %q, want bot-reply", got)
	}
}

func TestMessagesWaitAfterIncludesReplyStoredBeforeWaitInSameSecond(t *testing.T) {
	storeDir := seedWaitStore(t)
	second := time.Now().Truncate(time.Second)
	storeWaitMessage(t, storeDir, store.UpsertMessageParams{MsgID: "earlier", SenderJID: waitBotPN, Timestamp: second.Add(-time.Second), Text: "old"})
	storeWaitMessage(t, storeDir, store.UpsertMessageParams{MsgID: "same-second", SenderJID: waitBotPN, Timestamp: second, Text: "fast reply"})

	// The caller's instant has sub-second precision; the stored reply does not.
	after := second.Add(900 * time.Millisecond).Format(time.RFC3339Nano)
	ids, err := runMessagesWait(t, storeDir, 2*time.Second, "--chat", waitGroup, "--after", after)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ids, ","); got != "same-second" {
		t.Fatalf("messages = %q, want same-second", got)
	}
}

func TestMessagesWaitWithoutAfterIgnoresMessagesStoredBeforeStart(t *testing.T) {
	storeDir := seedWaitStore(t)
	storeWaitMessage(t, storeDir, store.UpsertMessageParams{MsgID: "already-there", SenderJID: waitBotPN, Timestamp: time.Now(), Text: "old"})
	storeWaitMessageLater(t, storeDir, 150*time.Millisecond, store.UpsertMessageParams{MsgID: "new", SenderJID: waitBotPN, Text: "new"})

	ids, err := runMessagesWait(t, storeDir, 5*time.Second, "--chat", waitGroup)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ids, ","); got != "new" {
		t.Fatalf("messages = %q, want new", got)
	}
}

func TestMessagesWaitSenderMatchesPhoneAndLIDForms(t *testing.T) {
	storeDir := seedWaitStore(t)
	sent := time.Now()
	storeWaitMessage(t, storeDir, store.UpsertMessageParams{MsgID: "partner", SenderJID: "15550002002@s.whatsapp.net", Timestamp: sent, Text: "eu"})
	storeWaitMessage(t, storeDir, store.UpsertMessageParams{MsgID: "bot-as-lid", SenderJID: waitBotLID, Timestamp: sent, Text: "Oi"})
	storeWaitMessage(t, storeDir, store.UpsertMessageParams{MsgID: "bot-as-pn", SenderJID: waitBotPN, Timestamp: sent.Add(time.Second), Text: "Tudo certo"})

	ids, err := runMessagesWait(t, storeDir, 2*time.Second, "--chat", waitGroup, "--sender", "15550002001", "--after", sent.Format(time.RFC3339Nano), "--count", "2")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ids, ","); got != "bot-as-lid,bot-as-pn" {
		t.Fatalf("messages = %q, want bot-as-lid,bot-as-pn", got)
	}
}

func TestMessagesWaitAfterIDSkipsEarlierReplyInSameSecond(t *testing.T) {
	storeDir := seedWaitStore(t)
	second := time.Now().Truncate(time.Second)
	storeWaitMessage(t, storeDir, store.UpsertMessageParams{MsgID: "previous-reply", SenderJID: waitBotPN, Timestamp: second, Text: "passo anterior"})
	storeWaitMessage(t, storeDir, store.UpsertMessageParams{MsgID: "mine", FromMe: true, Timestamp: second, Text: "gastei 10"})
	storeWaitMessage(t, storeDir, store.UpsertMessageParams{MsgID: "reply", SenderJID: waitBotLID, Timestamp: second, Text: "anotado"})

	ids, err := runMessagesWait(t, storeDir, 2*time.Second, "--chat", waitGroup, "--after-id", "mine", "--sender", waitBotPN)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ids, ","); got != "reply" {
		t.Fatalf("messages = %q, want reply", got)
	}
}

func TestMessagesWaitAfterIDWaitsForAnchorToArrive(t *testing.T) {
	storeDir := seedWaitStore(t)
	storeWaitMessage(t, storeDir, store.UpsertMessageParams{MsgID: "old", SenderJID: waitBotPN, Timestamp: time.Now(), Text: "old"})
	// The other account's message reaches this store after wait starts, then
	// the bot answers it.
	storeWaitMessageLater(t, storeDir, 100*time.Millisecond, store.UpsertMessageParams{MsgID: "partner-msg", SenderJID: "15550002002@s.whatsapp.net", Text: "gastei 20"})
	storeWaitMessageLater(t, storeDir, 250*time.Millisecond, store.UpsertMessageParams{MsgID: "bot-answer", SenderJID: waitBotPN, Text: "anotado"})

	ids, err := runMessagesWait(t, storeDir, 5*time.Second, "--chat", waitGroup, "--after-id", "partner-msg", "--sender", waitBotPN)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ids, ","); got != "bot-answer" {
		t.Fatalf("messages = %q, want bot-answer", got)
	}
}

func TestMessagesWaitRejectsAmbiguousBounds(t *testing.T) {
	storeDir := seedWaitStore(t)
	for _, args := range [][]string{
		{"--chat", waitGroup, "--after-id", "x", "--after", "2024-01-01"},
		{"--after-id", "x"},
		{"--chat", waitGroup, "--count", "0"},
	} {
		if _, err := runMessagesWait(t, storeDir, time.Second, args...); err == nil {
			t.Fatalf("args %v accepted", args)
		}
	}
}

func TestMessagesWaitFailsWhenTimeoutExpires(t *testing.T) {
	storeDir := seedWaitStore(t)
	sent := time.Now()
	storeWaitMessage(t, storeDir, store.UpsertMessageParams{MsgID: "mine", FromMe: true, Timestamp: sent, Text: "sem resposta"})

	_, err := runMessagesWait(t, storeDir, 300*time.Millisecond, "--chat", waitGroup, "--from-them", "--after", sent.Format(time.RFC3339Nano))
	if err == nil || !strings.Contains(err.Error(), "waiting for 1 matching message(s)") {
		t.Fatalf("err = %v, want timeout error", err)
	}
}

func TestMessagesListSenderMatchesPhoneAndLIDForms(t *testing.T) {
	storeDir := seedWaitStore(t)
	sent := time.Now()
	storeWaitMessage(t, storeDir, store.UpsertMessageParams{MsgID: "bot-as-lid", SenderJID: waitBotLID, Timestamp: sent, Text: "Oi"})
	storeWaitMessage(t, storeDir, store.UpsertMessageParams{MsgID: "partner", SenderJID: "15550002002@s.whatsapp.net", Timestamp: sent, Text: "eu"})

	cmd := newMessagesListCmd(&rootFlags{storeDir: storeDir, readOnly: true, asJSON: true, timeout: 5 * time.Second})
	cmd.SetArgs([]string{"--chat", waitGroup, "--sender", waitBotPN})
	raw := captureRootStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("messages list: %v", err)
		}
	})
	var result struct {
		Data struct{ Messages []store.Message }
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Data.Messages) != 1 || result.Data.Messages[0].MsgID != "bot-as-lid" {
		t.Fatalf("messages = %+v, want only bot-as-lid", result.Data.Messages)
	}
}
