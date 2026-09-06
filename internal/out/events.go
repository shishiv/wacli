package out

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

type EventWriter struct {
	mu       sync.Mutex
	writers  []io.Writer
	enabled  bool
	clockNow func() time.Time
}

type eventEnvelope struct {
	Event string `json:"event"`
	Data  any    `json:"data,omitempty"`
	TS    int64  `json:"ts"`
}

func NewEventWriter(w io.Writer, enabled bool) *EventWriter {
	ew := &EventWriter{
		enabled:  enabled,
		clockNow: time.Now,
	}
	if enabled && w != nil {
		ew.writers = []io.Writer{w}
	}
	return ew
}

func (e *EventWriter) AddWriter(w io.Writer) {
	if e == nil || w == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, existing := range e.writers {
		if existing == w {
			return
		}
	}
	e.writers = append(e.writers, w)
	e.enabled = true
}

func (e *EventWriter) Enabled() bool {
	return e != nil && e.enabled && len(e.writers) > 0
}

func (e *EventWriter) Emit(event string, data any) error {
	if !e.Enabled() {
		return nil
	}

	payload := eventEnvelope{
		Event: event,
		Data:  data,
		TS:    e.clockNow().UTC().UnixMilli(),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	line := append(b, '\n')
	for _, w := range e.writers {
		if _, err := w.Write(line); err != nil {
			return err
		}
	}
	return nil
}
