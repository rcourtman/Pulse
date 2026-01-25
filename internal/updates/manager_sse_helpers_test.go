package updates

import (
	"net/http/httptest"
	"testing"
	"time"
)

type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushRecorder) Flush() {}

func TestManagerSSEHelpers(t *testing.T) {
	m := &Manager{
		sseBroadcast: NewSSEBroadcaster(),
	}

	if m.GetSSEBroadcaster() == nil {
		t.Fatal("expected non-nil SSE broadcaster")
	}

	rec := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	client := m.AddSSEClient(rec, "client-1")
	if client == nil {
		t.Fatal("expected AddSSEClient to return client")
	}

	status, ts := m.GetSSECachedStatus()
	if status.Status == "" {
		t.Fatalf("expected cached status, got empty")
	}
	if ts.IsZero() {
		t.Fatalf("expected cached status time to be set")
	}

	m.RemoveSSEClient("client-1")

	// Give background goroutines a moment to handle send and removal.
	time.Sleep(10 * time.Millisecond)
}
