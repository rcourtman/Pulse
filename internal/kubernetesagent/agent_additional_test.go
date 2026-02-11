package kubernetesagent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/buffer"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDiscoverClusterMetadata(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	discovery := clientset.Discovery().(*fakediscovery.FakeDiscovery)
	discovery.FakedServerVersion = &version.Info{GitVersion: "v1.2.3"}

	agent := &Agent{
		kubeClient: clientset,
	}

	if err := agent.discoverClusterMetadata(context.Background()); err != nil {
		t.Fatalf("discoverClusterMetadata: %v", err)
	}
	if agent.clusterVersion != "v1.2.3" {
		t.Fatalf("clusterVersion = %q, want v1.2.3", agent.clusterVersion)
	}
}

func TestRun_StopsOnContextCancel(t *testing.T) {
	requested := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agents/kubernetes/report" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		select {
		case requested <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := zerolog.New(io.Discard)
	agent := &Agent{
		cfg:          Config{APIToken: "token"},
		logger:       logger,
		httpClient:   server.Client(),
		pulseURL:     server.URL,
		agentID:      "agent-1",
		agentVersion: "v1",
		interval:     10 * time.Millisecond,
		kubeClient:   fake.NewSimpleClientset(),
		reportBuffer: buffer.New[agentsk8s.Report](5),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- agent.Run(ctx)
	}()

	select {
	case <-requested:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected report request")
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Run did not return after cancel")
	}
}

type closeTrackingRoundTripper struct {
	closed bool
}

func (c *closeTrackingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, context.Canceled
}

func (c *closeTrackingRoundTripper) CloseIdleConnections() {
	c.closed = true
}

func TestRun_ClosesIdleHTTPConnectionsOnShutdown(t *testing.T) {
	transport := &closeTrackingRoundTripper{}
	logger := zerolog.New(io.Discard)
	agent := &Agent{
		cfg:          Config{APIToken: "token"},
		logger:       logger,
		httpClient:   &http.Client{Transport: transport},
		interval:     10 * time.Millisecond,
		kubeClient:   fake.NewSimpleClientset(),
		reportBuffer: buffer.New[agentsk8s.Report](1),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := agent.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !transport.closed {
		t.Fatal("expected Run to close idle HTTP connections")
	}
}

func TestSendReport_ErrorWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("nope"))
	}))
	defer server.Close()

	logger := zerolog.New(io.Discard)
	agent := &Agent{
		cfg:          Config{APIToken: "token"},
		logger:       logger,
		httpClient:   server.Client(),
		pulseURL:     server.URL,
		agentVersion: "v1",
	}

	err := agent.sendReport(context.Background(), agentsk8s.Report{Timestamp: time.Now().UTC()})
	if err == nil {
		t.Fatal("expected error from sendReport")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Fatalf("error = %v, want body in message", err)
	}
}
