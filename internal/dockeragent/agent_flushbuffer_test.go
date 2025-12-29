package dockeragent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/buffer"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

func TestFlushBuffer(t *testing.T) {
	t.Run("empty buffer", func(t *testing.T) {
		agent := &Agent{
			logger:       zerolog.Nop(),
			reportBuffer: buffer.New[agentsdocker.Report](5),
		}
		agent.flushBuffer(context.Background())
	})

	t.Run("send report error stops", func(t *testing.T) {
		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: {Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("send failed")
				})},
			},
			reportBuffer: buffer.New[agentsdocker.Report](5),
		}
		agent.reportBuffer.Push(agentsdocker.Report{Agent: agentsdocker.AgentInfo{ID: "queued"}})

		agent.flushBuffer(context.Background())
		if agent.reportBuffer.Len() != 1 {
			t.Fatalf("expected buffered report to remain")
		}
	})

	t.Run("stop requested halts flush", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"host was removed","code":"invalid_report"}`))
		}))
		defer server.Close()

		agent := &Agent{
			logger:  zerolog.Nop(),
			hostID:  "host1",
			targets: []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
			reportBuffer: buffer.New[agentsdocker.Report](5),
		}
		agent.reportBuffer.Push(agentsdocker.Report{Agent: agentsdocker.AgentInfo{ID: "queued"}})

		agent.flushBuffer(context.Background())
		if agent.reportBuffer.Len() != 1 {
			t.Fatalf("expected buffered report to remain on stop")
		}
	})

	t.Run("drains buffer", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
			reportBuffer: buffer.New[agentsdocker.Report](5),
		}
		agent.reportBuffer.Push(agentsdocker.Report{Agent: agentsdocker.AgentInfo{ID: "queued"}})
		agent.reportBuffer.Push(agentsdocker.Report{Agent: agentsdocker.AgentInfo{ID: "queued2"}})

		agent.flushBuffer(context.Background())
		if agent.reportBuffer.Len() != 0 {
			t.Fatalf("expected buffer to be drained")
		}
	})
}
