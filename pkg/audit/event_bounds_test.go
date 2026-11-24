package audit

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

func TestBoundEventTextKeepsFittingTextAndCutsOnCharacterBoundaries(t *testing.T) {
	fitting := strings.Repeat("界", maxEventUserBytes/3)
	if got := boundEventText(fitting, maxEventUserBytes); got != fitting {
		t.Fatalf("fitting text was changed: %d bytes, want %d", len(got), len(fitting))
	}

	for _, oversized := range []string{
		strings.Repeat("\x01", 750_000),
		strings.Repeat("界", 250_000),
		"a" + strings.Repeat("🔐", 200_000),
		strings.Repeat("\x80", 750_000),
	} {
		got := boundEventText(oversized, maxEventUserBytes)
		if len(got) > maxEventUserBytes {
			t.Fatalf("bounded text has %d bytes, want at most %d", len(got), maxEventUserBytes)
		}
		if len(got) < maxEventUserBytes-utf8.UTFMax {
			t.Fatalf("bounded text has only %d bytes", len(got))
		}
		if !strings.HasSuffix(got, eventTruncationTag) {
			t.Fatalf("bounded text does not say it was truncated: %q", got[len(got)-32:])
		}
		kept := strings.TrimSuffix(got, eventTruncationTag)
		if !strings.HasPrefix(oversized, kept) {
			t.Fatal("bounded text is not a prefix of the original")
		}
		if utf8.ValidString(oversized) && !utf8.ValidString(got) {
			t.Fatal("bounding split a multi-byte character")
		}
	}
}

func TestSQLiteLoggerRecordBoundsEventTextBeforeSigning(t *testing.T) {
	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:   t.TempDir(),
		CryptoMgr: newMockCryptoManager(),
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger failed: %v", err)
	}
	defer logger.Close()

	// The user name of a failed Basic sign-in and the request path are chosen
	// by a client without credentials.
	oversized := strings.Repeat("\x01", 750_000)
	event := Event{
		ID:        uuid.NewString(),
		Timestamp: time.Now(),
		EventType: "login",
		User:      oversized,
		IP:        "203.0.113.66",
		Path:      "/api/" + oversized,
		Success:   false,
		Details:   oversized,
	}
	if err := logger.Record(event); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	events, err := logger.Query(QueryFilter{ID: event.ID})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Query returned %d events, want 1", len(events))
	}
	stored := events[0]
	for name, field := range map[string]struct {
		text string
		max  int
	}{
		"user":    {stored.User, maxEventUserBytes},
		"path":    {stored.Path, maxEventPathBytes},
		"details": {stored.Details, maxEventDetailsBytes},
	} {
		if len(field.text) != field.max || !strings.HasSuffix(field.text, eventTruncationTag) {
			t.Errorf("stored %s has %d bytes, want %d ending in the truncation tag", name, len(field.text), field.max)
		}
	}
	if stored.EventType != event.EventType || stored.IP != event.IP {
		t.Errorf("fitting fields were changed: event %q, ip %q", stored.EventType, stored.IP)
	}
	if !logger.VerifySignature(stored) {
		t.Error("the signature does not cover the stored, bounded event")
	}
}

type recordingLogger struct {
	ConsoleLogger
	recorded chan Event
}

func (l *recordingLogger) Record(event Event) error {
	l.recorded <- event
	return nil
}

func TestAsyncLoggerBoundsEventTextBeforeQueueing(t *testing.T) {
	backend := &recordingLogger{recorded: make(chan Event, 1)}
	logger := NewAsyncLogger(backend, AsyncLoggerConfig{BufferSize: 1})
	defer logger.Close()

	if err := logger.Record(Event{ID: "oversized", EventType: "login", User: strings.Repeat("\x01", 750_000)}); err != nil {
		t.Fatalf("Record failed: %v", err)
	}
	select {
	case event := <-backend.recorded:
		if len(event.User) != maxEventUserBytes {
			t.Fatalf("queued user has %d bytes, want %d", len(event.User), maxEventUserBytes)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the backend did not receive the event")
	}
}
