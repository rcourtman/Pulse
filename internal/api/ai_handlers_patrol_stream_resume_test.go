package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type sseMessage struct {
	ID   string
	Data string
}

type errWriteFlusher struct {
	header http.Header
	err    error
}

func (w *errWriteFlusher) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *errWriteFlusher) Write(p []byte) (int, error) {
	if w.err == nil {
		w.err = errors.New("write failed")
	}
	return 0, w.err
}

func (w *errWriteFlusher) WriteHeader(_ int) {}
func (w *errWriteFlusher) Flush()            {}

func readNextSSEMessage(ctx context.Context, r *bufio.Reader) (sseMessage, error) {
	type result struct {
		msg sseMessage
		err error
	}
	ch := make(chan result, 1)

	go func() {
		var msg sseMessage
		var dataLines []string
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				ch <- result{err: err}
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				// End of an SSE event.
				if msg.ID != "" || len(dataLines) > 0 {
					msg.Data = strings.Join(dataLines, "\n")
					ch <- result{msg: msg}
					return
				}
				// Ignore empty events (e.g. multiple newlines).
				continue
			}
			if strings.HasPrefix(line, ":") {
				// Comment line / heartbeat.
				continue
			}
			if strings.HasPrefix(line, "id:") {
				msg.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
				continue
			}
			if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
				continue
			}
		}
	}()

	select {
	case <-ctx.Done():
		return sseMessage{}, ctx.Err()
	case res := <-ch:
		return res.msg, res.err
	}
}

func runPatrolResumeReplayScenario(t *testing.T, buildResumeURL func(baseURL, lastID string) string, setResumeHeaders func(req *http.Request, lastID string)) {
	t.Helper()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	handler := newTestAISettingsHandler(cfg, persistence, nil)
	handler.legacyAIService.SetStateProvider(&stubStateProvider{})
	patrol := handler.legacyAIService.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service to be initialized")
	}

	patrol.DebugResetStreamForRun("run-1")
	patrol.DebugSetStreamPhase("analyzing")

	srv := newIPv4HTTPServer(t, http.HandlerFunc(handler.HandlePatrolStream))
	defer srv.Close()

	// Connect and read until we see a real event id (snapshot may have id=0 before any events).
	reqCtx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, srv.URL+"/api/ai/patrol/stream", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		t.Fatalf("status=%d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)

	// Wait for the initial snapshot so we know subscription setup is complete.
	ctxInit, cancelInit := context.WithTimeout(context.Background(), 2*time.Second)
	_, err = readNextSSEMessage(ctxInit, reader)
	cancelInit()
	if err != nil {
		_ = resp.Body.Close()
		cancel()
		t.Fatalf("read initial sse: %v", err)
	}

	// Emit one content event and capture its SSE id.
	patrol.DebugAppendStreamContent("A")

	// Read until we receive the "A" content event, and store its SSE id.
	var lastID string
	deadline := time.Now().Add(2 * time.Second)
	for (lastID == "" || lastID == "0") && time.Now().Before(deadline) {
		ctxRead, cancelRead := context.WithTimeout(context.Background(), 2*time.Second)
		msg, err := readNextSSEMessage(ctxRead, reader)
		cancelRead()
		if err != nil {
			_ = resp.Body.Close()
			cancel()
			t.Fatalf("read sse: %v", err)
		}
		if msg.Data == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(msg.Data), &payload); err != nil {
			_ = resp.Body.Close()
			cancel()
			t.Fatalf("unmarshal: %v", err)
		}
		if payload["type"] == "content" && payload["content"] == "A" {
			lastID = msg.ID
		}
	}
	if lastID == "" || lastID == "0" {
		_ = resp.Body.Close()
		cancel()
		t.Fatalf("did not observe content=A event with a non-zero SSE id")
	}

	// Disconnect.
	_ = resp.Body.Close()
	cancel()

	// Emit a couple events while disconnected.
	patrol.DebugAppendStreamContent("B")
	patrol.DebugAppendStreamContent("C")

	// Reconnect with last_event_id, expecting buffered replay.
	req2Ctx, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	req2, err := http.NewRequestWithContext(req2Ctx, http.MethodGet, buildResumeURL(srv.URL, lastID), nil)
	if err != nil {
		t.Fatalf("new request2: %v", err)
	}
	if setResumeHeaders != nil {
		setResumeHeaders(req2, lastID)
	}
	resp2, err := srv.Client().Do(req2)
	if err != nil {
		t.Fatalf("do2: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("status2=%d", resp2.StatusCode)
	}

	reader2 := bufio.NewReader(resp2.Body)
	var got []string
	deadline2 := time.Now().Add(2 * time.Second)
	for len(got) < 2 && time.Now().Before(deadline2) {
		ctxRead, cancelRead := context.WithTimeout(context.Background(), 2*time.Second)
		msg, err := readNextSSEMessage(ctxRead, reader2)
		cancelRead()
		if err != nil {
			t.Fatalf("read sse2: %v", err)
		}
		if msg.Data == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(msg.Data), &payload); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if payload["type"] == "content" {
			if s, _ := payload["content"].(string); s != "" {
				got = append(got, s)
			}
		}
	}

	if len(got) != 2 || got[0] != "B" || got[1] != "C" {
		t.Fatalf("replay contents=%v, want [B C]", got)
	}
}

func TestHandlePatrolStream_ResumeViaLastEventIDQuery_ReplaysBufferedEvents(t *testing.T) {
	runPatrolResumeReplayScenario(
		t,
		func(baseURL, lastID string) string {
			return baseURL + "/api/ai/patrol/stream?last_event_id=" + lastID
		},
		nil,
	)
}

func TestHandlePatrolStream_ResumeViaLastEventIDHeader_ReplaysBufferedEvents(t *testing.T) {
	runPatrolResumeReplayScenario(
		t,
		func(baseURL, _ string) string {
			return baseURL + "/api/ai/patrol/stream"
		},
		func(req *http.Request, lastID string) {
			req.Header.Set("Last-Event-ID", lastID)
		},
	)
}

func TestHandlePatrolStream_LastEventIDHeaderTakesPrecedenceOverQueryFallback(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	handler := newTestAISettingsHandler(cfg, persistence, nil)
	handler.legacyAIService.SetStateProvider(&stubStateProvider{})
	patrol := handler.legacyAIService.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service to be initialized")
	}

	patrol.DebugResetStreamForRun("run-precedence")
	patrol.DebugSetStreamPhase("analyzing")

	srv := newIPv4HTTPServer(t, http.HandlerFunc(handler.HandlePatrolStream))
	defer srv.Close()

	reqCtx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, srv.URL+"/api/ai/patrol/stream", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		t.Fatalf("status=%d", resp.StatusCode)
	}
	reader := bufio.NewReader(resp.Body)

	// Initial snapshot to ensure subscription is active.
	ctxInit, cancelInit := context.WithTimeout(context.Background(), 2*time.Second)
	_, err = readNextSSEMessage(ctxInit, reader)
	cancelInit()
	if err != nil {
		_ = resp.Body.Close()
		cancel()
		t.Fatalf("read initial sse: %v", err)
	}

	// Emit A, capture id(A), then disconnect and emit B/C while disconnected.
	patrol.DebugAppendStreamContent("A")
	var idA int
	deadline := time.Now().Add(2 * time.Second)
	for idA == 0 && time.Now().Before(deadline) {
		ctxRead, cancelRead := context.WithTimeout(context.Background(), 2*time.Second)
		msg, err := readNextSSEMessage(ctxRead, reader)
		cancelRead()
		if err != nil {
			_ = resp.Body.Close()
			cancel()
			t.Fatalf("read sse: %v", err)
		}
		if msg.Data == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(msg.Data), &payload); err != nil {
			_ = resp.Body.Close()
			cancel()
			t.Fatalf("unmarshal: %v", err)
		}
		if payload["type"] == "content" && payload["content"] == "A" {
			v, _ := strconv.Atoi(msg.ID)
			idA = v
		}
	}
	if idA == 0 {
		_ = resp.Body.Close()
		cancel()
		t.Fatalf("failed to capture id for content=A")
	}

	_ = resp.Body.Close()
	cancel()
	patrol.DebugAppendStreamContent("B")
	patrol.DebugAppendStreamContent("C")

	// Query asks for full replay from 1, header asks resume from B seq (idA+1),
	// so we should receive only C if header takes precedence.
	req2Ctx, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	req2, err := http.NewRequestWithContext(req2Ctx, http.MethodGet, srv.URL+"/api/ai/patrol/stream?last_event_id=1", nil)
	if err != nil {
		t.Fatalf("new request2: %v", err)
	}
	req2.Header.Set("Last-Event-ID", strconv.Itoa(idA+1))
	resp2, err := srv.Client().Do(req2)
	if err != nil {
		t.Fatalf("do2: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("status2=%d", resp2.StatusCode)
	}

	reader2 := bufio.NewReader(resp2.Body)
	var got []string
	deadline2 := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline2) {
		ctxRead, cancelRead := context.WithTimeout(context.Background(), 2*time.Second)
		msg, err := readNextSSEMessage(ctxRead, reader2)
		cancelRead()
		if err != nil {
			break
		}
		if msg.Data == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(msg.Data), &payload); err != nil {
			t.Fatalf("unmarshal replay payload: %v", err)
		}
		if payload["type"] == "content" {
			if s, _ := payload["content"].(string); s != "" {
				got = append(got, s)
				if len(got) >= 2 {
					break
				}
			}
		}
	}

	if len(got) != 1 || got[0] != "C" {
		t.Fatalf("expected header precedence replay [C], got %v", got)
	}
}

func TestHandlePatrolStream_InvalidHeaderFallsBackToQueryLastEventID(t *testing.T) {
	runPatrolResumeReplayScenario(
		t,
		func(baseURL, lastID string) string {
			return baseURL + "/api/ai/patrol/stream?last_event_id=" + lastID
		},
		func(req *http.Request, _ string) {
			req.Header.Set("Last-Event-ID", "not-a-number")
		},
	)
}

func TestHandlePatrolStream_ReturnsOnInitialWriteError(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	handler := newTestAISettingsHandler(cfg, persistence, nil)
	handler.legacyAIService.SetStateProvider(&stubStateProvider{})
	patrol := handler.legacyAIService.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service to be initialized")
	}
	patrol.DebugResetStreamForRun("run-write-error")
	patrol.DebugSetStreamPhase("analyzing")

	req := http.Request{
		Method: http.MethodGet,
		URL:    mustParseURL(t, "/api/ai/patrol/stream"),
		Header: make(http.Header),
	}
	req = *req.WithContext(context.Background())

	w := &errWriteFlusher{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.HandlePatrolStream(w, &req)
	}()

	select {
	case <-done:
		// expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler did not return promptly on initial write error")
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	return u
}

func TestHandlePatrolStream_ResumeWithStaleLastEventIDQuery_EmitsBufferRotatedSnapshot(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	handler := newTestAISettingsHandler(cfg, persistence, nil)
	handler.legacyAIService.SetStateProvider(&stubStateProvider{})
	patrol := handler.legacyAIService.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service to be initialized")
	}

	patrol.DebugResetStreamForRun("run-2")
	patrol.DebugSetStreamPhase("analyzing")

	srv := newIPv4HTTPServer(t, http.HandlerFunc(handler.HandlePatrolStream))
	defer srv.Close()

	// Emit enough events to rotate the replay buffer (size 200).
	for i := 0; i < 210; i++ {
		patrol.DebugAppendStreamContent("x")
	}

	// Reconnect with a very old last_event_id.
	reqCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, srv.URL+"/api/ai/patrol/stream?last_event_id=1", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	ctxRead, cancelRead := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelRead()
	msg, err := readNextSSEMessage(ctxRead, reader)
	if err != nil {
		t.Fatalf("read sse: %v", err)
	}
	// First message may be a comment; keep reading until we get JSON.
	for msg.Data == "" {
		ctxMore, cancelMore := context.WithTimeout(context.Background(), 2*time.Second)
		msg, err = readNextSSEMessage(ctxMore, reader)
		cancelMore()
		if err != nil {
			t.Fatalf("read sse: %v", err)
		}
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(msg.Data), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["type"] != "snapshot" {
		t.Fatalf("type=%v, want snapshot (data=%s)", payload["type"], msg.Data)
	}
	if payload["resync_reason"] != "buffer_rotated" {
		t.Fatalf("resync_reason=%v, want buffer_rotated (data=%s)", payload["resync_reason"], msg.Data)
	}
}
