package out

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEventWriterDisabledIsNoOp(t *testing.T) {
	var b bytes.Buffer
	w := NewEventWriter(&b, false)
	if err := w.Emit("connected", nil); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if b.Len() != 0 {
		t.Fatalf("expected no output when disabled, got %q", b.String())
	}
}

func TestEventWriterEmitsNDJSON(t *testing.T) {
	var b bytes.Buffer
	w := NewEventWriter(&b, true)
	w.clockNow = func() time.Time { return time.UnixMilli(1234).UTC() }

	if err := w.Emit("progress", map[string]any{"messages_synced": 25}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	line := strings.TrimSpace(b.String())
	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["event"] != "progress" {
		t.Fatalf("unexpected event: %v", got["event"])
	}
	if got["ts"] != float64(1234) {
		t.Fatalf("unexpected ts: %v", got["ts"])
	}
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %T", got["data"])
	}
	if data["messages_synced"] != float64(25) {
		t.Fatalf("unexpected data payload: %#v", data)
	}
}

func TestEventWriterAddWriter(t *testing.T) {
	var b1, b2 bytes.Buffer
	w := NewEventWriter(&b1, true)
	w.AddWriter(&b2)

	if err := w.Emit("custom", map[string]any{"ok": true}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if !strings.Contains(b1.String(), `"event":"custom"`) {
		t.Fatalf("expected b1 to receive event, got %q", b1.String())
	}
	if !strings.Contains(b2.String(), `"event":"custom"`) {
		t.Fatalf("expected b2 to receive event, got %q", b2.String())
	}
}
