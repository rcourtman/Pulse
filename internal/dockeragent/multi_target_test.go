package dockeragent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

func TestDeliverReportBuffersOnlyFailedDockerDestination(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer primary.Close()
	observer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer observer.Close()

	targets := []TargetConfig{
		{Name: "primary", URL: primary.URL, Token: "p", Authoritative: true},
		{Name: "dev", URL: observer.URL, Token: "o"},
	}
	a := &Agent{
		logger: zerolog.Nop(), targets: targets,
		httpClients:        map[bool]*http.Client{false: http.DefaultClient},
		trustedHTTPClients: map[string]*http.Client{},
		reportBuffer:       utils.New[agentsdocker.Report](10),
		reportBuffers: map[string]*utils.Queue[agentsdocker.Report]{
			"primary": utils.New[agentsdocker.Report](10),
			"dev":     utils.New[agentsdocker.Report](10),
		},
	}
	a.reportBuffer = a.reportBuffers["primary"]
	if err := a.deliverReport(context.Background(), agentsdocker.Report{}); err != nil {
		t.Fatalf("deliver report: %v", err)
	}
	if a.reportBuffers["primary"].Len() != 0 || a.reportBuffers["dev"].Len() != 1 {
		t.Fatalf("buffer depths primary=%d observer=%d", a.reportBuffers["primary"].Len(), a.reportBuffers["dev"].Len())
	}
}

func TestDockerObserverCannotIssueCommands(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"commands":[{"id":"cmd1","type":"stop"}]}`))
	}))
	defer server.Close()
	a := &Agent{logger: zerolog.Nop(), httpClients: map[bool]*http.Client{false: server.Client()}, trustedHTTPClients: map[string]*http.Client{}}
	if err := a.sendReportToTarget(context.Background(), TargetConfig{Name: "dev", URL: server.URL, Token: "o"}, []byte(`{}`), 0); err != nil {
		t.Fatalf("observer response must be acknowledgement-only: %v", err)
	}
}

func TestDockerObserverPlaintextPolicyIsDestinationScoped(t *testing.T) {
	targets := []TargetConfig{
		{Name: "primary", URL: "https://primary.example.test", Token: "p", Authoritative: true},
		{Name: "observer", URL: "http://203.0.113.10:7655", Token: "o"},
	}
	if _, err := normalizeTargets(targets); err == nil {
		t.Fatal("expected observer plaintext URL rejection without destination opt-in")
	}
	targets[1].AllowPlaintextHTTP = true
	normalized, err := normalizeTargets(targets)
	if err != nil {
		t.Fatalf("explicit observer plaintext opt-in: %v", err)
	}
	if !normalized[1].AllowPlaintextHTTP {
		t.Fatal("observer plaintext policy was not preserved after normalization")
	}
}
