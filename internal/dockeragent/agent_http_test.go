package dockeragent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

func TestSendReport(t *testing.T) {
	t.Run("marshal error", func(t *testing.T) {
		agent := &Agent{logger: zerolog.Nop()}
		report := agentsdocker.Report{
			Host: agentsdocker.HostInfo{
				CPUUsagePercent: math.NaN(),
			},
		}

		if err := agent.sendReport(context.Background(), report); err == nil {
			t.Fatal("expected marshal error")
		}
	})

	t.Run("stop requested", func(t *testing.T) {
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
		}

		if err := agent.sendReport(context.Background(), agentsdocker.Report{}); !errors.Is(err, ErrStopRequested) {
			t.Fatalf("expected ErrStopRequested, got %v", err)
		}
	})

	t.Run("errors join", func(t *testing.T) {
		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://one", Token: "t1"}, {URL: "http://two", Token: "t2"}},
			httpClients: map[bool]*http.Client{
				false: {Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("send failed")
				})},
			},
		}

		if err := agent.sendReport(context.Background(), agentsdocker.Report{}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("large payload succeeds", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}

		report := agentsdocker.Report{
			Containers: []agentsdocker.Container{
				{ID: strings.Repeat("a", 500000)},
			},
		}

		if err := agent.sendReport(context.Background(), report); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestSendReportToTarget(t *testing.T) {
	t.Run("request error", func(t *testing.T) {
		agent := &Agent{logger: zerolog.Nop()}
		if err := agent.sendReportToTarget(context.Background(), TargetConfig{URL: "http://example.com/\x7f"}, []byte(`{}`), 0); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("host removed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"host was removed","code":"invalid_report"}`))
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			hostID: "host1",
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}
		err := agent.sendReportToTarget(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, []byte(`{}`), 0)
		if !errors.Is(err, ErrStopRequested) {
			t.Fatalf("expected ErrStopRequested, got %v", err)
		}
	})

	t.Run("command continue on nil error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"commands":[{"id":"cmd1","type":"unknown"}]}`))
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}

		if err := agent.sendReportToTarget(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, []byte(`{}`), 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("status error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("bad request"))
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}

		if err := agent.sendReportToTarget(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, []byte(`{}`), 0); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("status error with empty body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}

		if err := agent.sendReportToTarget(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, []byte(`{}`), 0); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("read error", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       errReadCloser{err: errors.New("read failed")},
					Header:     make(http.Header),
				}, nil
			}),
		}

		agent := &Agent{
			logger: zerolog.Nop(),
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}

		if err := agent.sendReportToTarget(context.Background(), TargetConfig{URL: "http://example", Token: "token"}, []byte(`{}`), 0); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid json response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{"))
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}

		if err := agent.sendReportToTarget(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, []byte(`{}`), 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty response body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}

		if err := agent.sendReportToTarget(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, []byte(`{}`), 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("stop command", func(t *testing.T) {
		prevPath := os.Getenv("PATH")
		_ = os.Setenv("PATH", "")
		t.Cleanup(func() {
			_ = os.Setenv("PATH", prevPath)
		})

		var ackBody bytes.Buffer
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/report"):
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"commands":[{"id":"cmd1","type":"stop"}]}`))
			case strings.Contains(r.URL.Path, "/commands/"):
				body, _ := io.ReadAll(r.Body)
				ackBody.Write(body)
				w.WriteHeader(http.StatusOK)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			hostID: "host1",
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}

		err := agent.sendReportToTarget(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, []byte(`{}`), 0)
		if !errors.Is(err, ErrStopRequested) {
			t.Fatalf("expected ErrStopRequested, got %v", err)
		}
	})

	t.Run("command error bubbles", func(t *testing.T) {
		prevPath := os.Getenv("PATH")
		_ = os.Setenv("PATH", "")
		t.Cleanup(func() {
			_ = os.Setenv("PATH", prevPath)
		})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/report"):
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"commands":[{"id":"cmd1","type":"stop"}]}`))
			case strings.Contains(r.URL.Path, "/commands/"):
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("boom"))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			hostID: "host1",
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}

		if err := agent.sendReportToTarget(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, []byte(`{}`), 0); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSendCommandAck(t *testing.T) {
	t.Run("missing host id", func(t *testing.T) {
		agent := &Agent{}
		if err := agent.sendCommandAck(context.Background(), TargetConfig{URL: "http://example"}, "cmd", "status", "msg"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("marshal error", func(t *testing.T) {
		swap(t, &jsonMarshalFn, func(any) ([]byte, error) {
			return nil, errors.New("marshal failed")
		})

		agent := &Agent{hostID: "host1"}
		if err := agent.sendCommandAck(context.Background(), TargetConfig{URL: "http://example"}, "cmd", "status", "msg"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("request error", func(t *testing.T) {
		agent := &Agent{hostID: "host1"}
		badURL := "http://example.com/\x7f"
		if err := agent.sendCommandAck(context.Background(), TargetConfig{URL: badURL}, "cmd", "status", "msg"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("client error", func(t *testing.T) {
		agent := &Agent{
			hostID: "host1",
			httpClients: map[bool]*http.Client{
				false: {Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("send failed")
				})},
			},
		}

		if err := agent.sendCommandAck(context.Background(), TargetConfig{URL: "http://example", Token: "token"}, "cmd", "status", "msg"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("status error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		}))
		defer server.Close()

		agent := &Agent{
			hostID: "host1",
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}

		if err := agent.sendCommandAck(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, "cmd", "status", "msg"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		var got agentsdocker.CommandAck
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &got)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		agent := &Agent{
			hostID: "host1",
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}

		if err := agent.sendCommandAck(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, "cmd", "completed", "ok"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Status != "completed" {
			t.Fatalf("expected status to be sent, got %q", got.Status)
		}
	})
}

func TestHandleCommand(t *testing.T) {
	agent := &Agent{logger: zerolog.Nop()}
	if err := agent.handleCommand(context.Background(), TargetConfig{}, agentsdocker.Command{Type: "unknown"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("stop command", func(t *testing.T) {
		prev := os.Getenv("PATH")
		_ = os.Setenv("PATH", "")
		t.Cleanup(func() {
			_ = os.Setenv("PATH", prev)
		})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			hostID: "host1",
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}

		err := agent.handleCommand(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, agentsdocker.Command{ID: "cmd", Type: agentsdocker.CommandTypeStop})
		if !errors.Is(err, ErrStopRequested) {
			t.Fatalf("expected ErrStopRequested, got %v", err)
		}
	})

	t.Run("update command", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			hostID: "host1",
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
			docker: &fakeDockerClient{
				containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
					return containertypes.InspectResponse{}, errors.New("inspect failed")
				},
			},
		}

		cmd := agentsdocker.Command{
			ID:   "cmd2",
			Type: agentsdocker.CommandTypeUpdateContainer,
			Payload: map[string]any{
				"containerId": "container1",
			},
		}

		if err := agent.handleCommand(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, cmd); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestHandleStopCommand(t *testing.T) {
	t.Run("disable error sends failure ack", func(t *testing.T) {
		writeSystemctl(t, "echo 'access denied' >&2\nexit 1")

		var ack agentsdocker.CommandAck
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &ack)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			hostID: "host1",
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}

		if err := agent.handleStopCommand(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, agentsdocker.Command{ID: "cmd"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ack.Status != agentsdocker.CommandStatusFailed {
			t.Fatalf("expected failed status, got %q", ack.Status)
		}
	})

	t.Run("disable error ack failure", func(t *testing.T) {
		writeSystemctl(t, "echo 'access denied' >&2\nexit 1")

		agent := &Agent{
			logger: zerolog.Nop(),
			hostID: "host1",
		}

		if err := agent.handleStopCommand(context.Background(), TargetConfig{URL: "http://example.com/\x7f", Token: "token"}, agentsdocker.Command{ID: "cmd"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("success returns stop requested", func(t *testing.T) {
		prev := os.Getenv("PATH")
		_ = os.Setenv("PATH", "")
		t.Cleanup(func() {
			_ = os.Setenv("PATH", prev)
		})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			hostID: "host1",
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}

		if err := agent.handleStopCommand(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, agentsdocker.Command{ID: "cmd"}); !errors.Is(err, ErrStopRequested) {
			t.Fatalf("expected ErrStopRequested, got %v", err)
		}
	})

	t.Run("completion ack error", func(t *testing.T) {
		prev := os.Getenv("PATH")
		_ = os.Setenv("PATH", "")
		t.Cleanup(func() {
			_ = os.Setenv("PATH", prev)
		})

		agent := &Agent{
			logger: zerolog.Nop(),
			hostID: "host1",
			httpClients: map[bool]*http.Client{
				false: {Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("send failed")
				})},
			},
		}

		if err := agent.handleStopCommand(context.Background(), TargetConfig{URL: "http://example", Token: "token"}, agentsdocker.Command{ID: "cmd"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("stop service goroutine executes", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "called")
		writeSystemctl(t, "if [ \"$1\" = \"disable\" ]; then exit 0; fi\nif [ \"$1\" = \"stop\" ]; then : > "+marker+"; exit 2; fi\nexit 0")
		swap(t, &sleepFn, func(time.Duration) {})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		agent := &Agent{
			logger: zerolog.Nop(),
			hostID: "host1",
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}

		if err := agent.handleStopCommand(context.Background(), TargetConfig{URL: server.URL, Token: "token"}, agentsdocker.Command{ID: "cmd"}); !errors.Is(err, ErrStopRequested) {
			t.Fatalf("expected ErrStopRequested, got %v", err)
		}

		deadline := time.Now().Add(200 * time.Millisecond)
		for {
			if _, err := os.Stat(marker); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("expected stopSystemdService to be invoked")
			}
			time.Sleep(5 * time.Millisecond)
		}
	})
}

func TestDisableSelf(t *testing.T) {
	prev := os.Getenv("PATH")
	_ = os.Setenv("PATH", "")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", prev)
	})

	baseDir := t.TempDir()
	scriptDir := filepath.Join(baseDir, "script")
	if err := os.MkdirAll(scriptDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scriptDir, "file"), []byte("x"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
	logDir := filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "file"), []byte("x"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	swap(t, &unraidStartupScriptPath, scriptDir)
	swap(t, &agentLogPath, logDir)

	agent := &Agent{logger: zerolog.Nop()}
	if err := agent.disableSelf(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
