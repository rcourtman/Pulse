package hostagent

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

func TestAgent_deliverPrimaryReport_DrainsBufferedReportsBeforeCurrent(t *testing.T) {
	var (
		mu       sync.Mutex
		received []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body io.Reader = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			defer reader.Close()
			body = reader
		}
		var report agentshost.Report
		if err := json.NewDecoder(body).Decode(&report); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		received = append(received, report.SequenceID)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := &Agent{
		cfg:             Config{APIToken: "token"},
		logger:          zerolog.Nop(),
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
		reportBuffer:    utils.New[agentshost.Report](10),
	}
	a.reportBuffer.Push(agentshost.Report{SequenceID: "oldest"})
	a.reportBuffer.Push(agentshost.Report{SequenceID: "older"})

	if err := a.deliverPrimaryReport(context.Background(), agentshost.Report{SequenceID: "current"}); err != nil {
		t.Fatalf("deliverPrimaryReport: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"oldest", "older", "current"}
	if len(received) != len(want) {
		t.Fatalf("received = %v, want %v", received, want)
	}
	for i := range want {
		if received[i] != want[i] {
			t.Fatalf("received = %v, want %v", received, want)
		}
	}
}

func TestAgent_flushBuffer_StopsOnFailureAndDoesNotDropReport(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requestCount, 1)
		if n == 1 {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := &Agent{
		cfg:             Config{APIToken: "token"},
		logger:          zerolog.Nop(),
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
		reportBuffer:    utils.New[agentshost.Report](10),
	}

	report1 := agentshost.Report{Agent: agentshost.AgentInfo{ID: "r1"}}
	report2 := agentshost.Report{Agent: agentshost.AgentInfo{ID: "r2"}}
	a.reportBuffer.Push(report1)
	a.reportBuffer.Push(report2)

	a.flushBuffer(context.Background())

	if got := a.reportBuffer.Len(); got != 1 {
		t.Fatalf("buffer len = %d, want %d", got, 1)
	}

	peek, ok := a.reportBuffer.Peek()
	if !ok {
		t.Fatalf("expected buffered report")
	}
	if peek.Agent.ID != "r2" {
		t.Fatalf("remaining buffered report = %q, want %q", peek.Agent.ID, "r2")
	}
}

func TestAgent_flushBuffer_RetryAfterTransientFailure(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := &Agent{
		cfg:             Config{APIToken: "token"},
		logger:          zerolog.Nop(),
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
		reportBuffer:    utils.New[agentshost.Report](10),
	}
	a.reportBuffer.Push(agentshost.Report{Agent: agentshost.AgentInfo{ID: "r1"}})
	a.reportBuffer.Push(agentshost.Report{Agent: agentshost.AgentInfo{ID: "r2"}})

	a.flushBuffer(context.Background())
	if got := a.reportBuffer.Len(); got != 2 {
		t.Fatalf("buffer len after failure = %d, want %d", got, 2)
	}

	fail.Store(false)
	a.flushBuffer(context.Background())
	if !a.reportBuffer.IsEmpty() {
		t.Fatalf("expected buffer to be empty, has %d items", a.reportBuffer.Len())
	}
}

func TestAgent_flushBuffer_ForbiddenDropsBuffer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	a := &Agent{
		cfg:             Config{APIToken: "wrong-scope"},
		logger:          zerolog.Nop(),
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
		reportBuffer:    utils.New[agentshost.Report](10),
	}
	a.reportBuffer.Push(agentshost.Report{Agent: agentshost.AgentInfo{ID: "r1"}})
	a.reportBuffer.Push(agentshost.Report{Agent: agentshost.AgentInfo{ID: "r2"}})

	a.flushBuffer(context.Background())
	if !a.reportBuffer.IsEmpty() {
		t.Fatalf("buffer should be drained after forbidden flush, len=%d", a.reportBuffer.Len())
	}
}
