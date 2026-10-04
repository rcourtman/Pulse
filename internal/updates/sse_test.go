package updates

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSSEBroadcaster_AddRemoveClient(t *testing.T) {
	broadcaster := NewSSEBroadcaster()
	defer broadcaster.Close()

	if broadcaster.GetClientCount() != 0 {
		t.Error("Initial client count should be 0")
	}

	// Create a mock response writer with Flusher
	w := httptest.NewRecorder()

	// httptest.ResponseRecorder actually implements http.Flusher in Go 1.21+
	// So AddClient will succeed
	client := broadcaster.AddClient(w, "client-1")
	if client == nil {
		t.Error("AddClient should succeed for ResponseRecorder with Flusher")
	}

	if broadcaster.GetClientCount() != 1 {
		t.Error("Client count should be 1 after adding client")
	}

	// Test removal
	broadcaster.RemoveClient("client-1")
	if broadcaster.GetClientCount() != 0 {
		t.Error("Client count should be 0 after removal")
	}

	// Test removal of non-existent client (should not panic)
	broadcaster.RemoveClient("non-existent")
}

func TestSSEBroadcaster_Broadcast(t *testing.T) {
	broadcaster := NewSSEBroadcaster()
	defer broadcaster.Close()

	// Broadcast a status
	status := UpdateStatus{
		Status:    "downloading",
		Progress:  50,
		Message:   "Downloading update...",
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	broadcaster.Broadcast(status)

	// Verify cached status
	cachedStatus, cacheTime := broadcaster.GetCachedStatus()
	if cachedStatus.Status != status.Status {
		t.Errorf("Cached status should be %s, got %s", status.Status, cachedStatus.Status)
	}
	if cachedStatus.Progress != status.Progress {
		t.Errorf("Cached progress should be %d, got %d", status.Progress, cachedStatus.Progress)
	}
	if time.Since(cacheTime) > 1*time.Second {
		t.Error("Cache time should be recent")
	}
}

func TestSSEBroadcaster_GetCachedStatus(t *testing.T) {
	broadcaster := NewSSEBroadcaster()
	defer broadcaster.Close()

	// Get initial cached status
	status, _ := broadcaster.GetCachedStatus()
	if status.Status != "idle" {
		t.Errorf("Initial status should be idle, got %s", status.Status)
	}

	// Broadcast new status
	newStatus := UpdateStatus{
		Status:    "downloading",
		Progress:  25,
		Message:   "Test message",
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	broadcaster.Broadcast(newStatus)

	// Verify cached status updated
	cachedStatus, _ := broadcaster.GetCachedStatus()
	if cachedStatus.Status != "downloading" {
		t.Errorf("Cached status should be downloading, got %s", cachedStatus.Status)
	}
	if cachedStatus.Progress != 25 {
		t.Errorf("Cached progress should be 25, got %d", cachedStatus.Progress)
	}
}

// Mock writer that implements http.Flusher
type mockFlushWriter struct {
	*httptest.ResponseRecorder
	flushed int
}

func (m *mockFlushWriter) Flush() {
	m.flushed++
}

func TestSSEBroadcaster_SendToClient(t *testing.T) {
	broadcaster := NewSSEBroadcaster()
	defer broadcaster.Close()

	// Create mock writer with Flusher
	mockWriter := &mockFlushWriter{
		ResponseRecorder: httptest.NewRecorder(),
	}

	client := &SSEClient{
		ID:         "test-client",
		Writer:     mockWriter,
		Flusher:    mockWriter,
		Done:       make(chan bool, 1),
		LastActive: time.Now(),
	}

	status := UpdateStatus{
		Status:    "downloading",
		Progress:  50,
		Message:   "Test message",
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	broadcaster.sendToClient(client, status)

	// Verify message was written
	body := mockWriter.Body.String()
	if !strings.Contains(body, "data:") {
		t.Error("Message should contain 'data:' prefix")
	}
	if !strings.Contains(body, "downloading") {
		t.Error("Message should contain status")
	}
	if !strings.Contains(body, "Test message") {
		t.Error("Message should contain message")
	}

	// Verify flushed
	if mockWriter.flushed < 1 {
		t.Error("Should have flushed at least once")
	}
}

func TestSSEBroadcaster_SendHeartbeat(t *testing.T) {
	broadcaster := NewSSEBroadcaster()
	defer broadcaster.Close()

	mockWriter := &mockFlushWriter{
		ResponseRecorder: httptest.NewRecorder(),
	}

	client := &SSEClient{
		ID:         "test-client",
		Writer:     mockWriter,
		Flusher:    mockWriter,
		Done:       make(chan bool, 1),
		LastActive: time.Now(),
	}

	// Manually add client to broadcaster
	broadcaster.mu.Lock()
	broadcaster.clients["test-client"] = client
	broadcaster.mu.Unlock()

	broadcaster.SendHeartbeat()

	// Verify heartbeat was written
	body := mockWriter.Body.String()
	if !strings.Contains(body, ": heartbeat") {
		t.Errorf("Should contain heartbeat comment, got: %s", body)
	}
}

func TestSSEBroadcaster_Close(t *testing.T) {
	broadcaster := NewSSEBroadcaster()

	mockWriter := &mockFlushWriter{
		ResponseRecorder: httptest.NewRecorder(),
	}

	client := &SSEClient{
		ID:         "test-client",
		Writer:     mockWriter,
		Flusher:    mockWriter,
		Done:       make(chan bool, 1),
		LastActive: time.Now(),
	}

	broadcaster.mu.Lock()
	broadcaster.clients["test-client"] = client
	broadcaster.mu.Unlock()

	if broadcaster.GetClientCount() != 1 {
		t.Error("Should have 1 client")
	}

	broadcaster.Close()

	if broadcaster.GetClientCount() != 0 {
		t.Error("Should have 0 clients after close")
	}

	// Verify client was signaled
	select {
	case <-client.Done:
		// Channel closed as expected
	default:
		t.Error("Client Done channel should be closed")
	}
}

func TestSSEBroadcaster_CloseIsIdempotentAndPostCloseSafe(t *testing.T) {
	broadcaster := NewSSEBroadcaster()

	broadcaster.Close()
	broadcaster.Close()

	// Post-close operations should be no-ops and must not panic.
	broadcaster.Broadcast(UpdateStatus{
		Status:    "idle",
		Progress:  0,
		Message:   "after close",
		UpdatedAt: time.Now().Format(time.RFC3339),
	})
	broadcaster.SendHeartbeat()

	if client := broadcaster.AddClient(httptest.NewRecorder(), "client-after-close"); client != nil {
		t.Fatal("expected AddClient to return nil after broadcaster close")
	}
	if broadcaster.GetClientCount() != 0 {
		t.Fatal("expected no connected clients after close")
	}
}

// A (re)connecting update modal must see the current stage immediately, not
// after the next stage change, which during a long download can be minutes.
func TestSSEBroadcaster_AddClientWritesCurrentStatusBeforeReturning(t *testing.T) {
	broadcaster := NewSSEBroadcaster()
	defer broadcaster.Close()

	broadcaster.Broadcast(UpdateStatus{Status: "extracting", Progress: 40, Message: "Extracting update..."})

	w := &mockFlushWriter{ResponseRecorder: httptest.NewRecorder()}
	if client := broadcaster.AddClient(w, "client-now"); client == nil {
		t.Fatal("AddClient returned nil")
	}

	body := w.Body.String()
	if !strings.HasPrefix(body, ": connected\n\n") {
		t.Fatalf("expected connection preamble first, got %q", body)
	}
	if !strings.Contains(body, `data: {"status":"extracting","progress":40`) {
		t.Fatalf("expected current status on connect, got %q", body)
	}
	if w.flushed < 1 {
		t.Fatal("initial status must be flushed, not left buffered")
	}
}

// Broadcasts used to be sent from one goroutine per message, so stages
// emitted in quick succession could reach a client out of order.
func TestSSEBroadcaster_DeliversStagesInOrder(t *testing.T) {
	broadcaster := NewSSEBroadcaster()
	defer broadcaster.Close()

	w := &syncFlushWriter{}
	if client := broadcaster.AddClient(w, "client-order"); client == nil {
		t.Fatal("AddClient returned nil")
	}

	const stages = 40
	for i := 1; i <= stages; i++ {
		broadcaster.Broadcast(UpdateStatus{Status: "verifying", Progress: i})
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(w.String(), `"progress":40,`) {
		time.Sleep(5 * time.Millisecond)
	}
	body := w.String()
	last := -1
	for i := 1; i <= stages; i++ {
		idx := strings.Index(body, `"progress":`+strconv.Itoa(i)+`,`)
		if idx < 0 {
			t.Fatalf("stage %d never delivered:\n%s", i, body)
		}
		if idx < last {
			t.Fatalf("stage %d delivered out of order:\n%s", i, body)
		}
		last = idx
	}
}

type syncFlushWriter struct {
	mu     sync.Mutex
	header http.Header
	buf    strings.Builder
}

func (w *syncFlushWriter) Header() http.Header {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *syncFlushWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *syncFlushWriter) WriteHeader(int) {}

func (w *syncFlushWriter) Flush() {}

func (w *syncFlushWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}
