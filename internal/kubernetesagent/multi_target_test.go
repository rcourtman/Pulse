package kubernetesagent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	"github.com/rs/zerolog"
)

func TestKubernetesDestinationBuffersAreIndependent(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer primary.Close()
	observer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer observer.Close()

	primaryTarget := &reportTarget{config: TargetConfig{Name: "primary", URL: primary.URL, Token: "p", Authoritative: true}, client: primary.Client(), buffer: utils.New[agentsk8s.Report](10)}
	observerTarget := &reportTarget{config: TargetConfig{Name: "dev", URL: observer.URL, Token: "o"}, client: observer.Client(), buffer: utils.New[agentsk8s.Report](10)}
	a := &Agent{logger: zerolog.Nop(), agentVersion: "test", targets: []*reportTarget{primaryTarget, observerTarget}, reportBuffer: primaryTarget.buffer}
	report := agentsk8s.Report{}
	for _, target := range a.targets {
		if err := a.sendReportToTarget(context.Background(), report, target); err != nil {
			target.buffer.Push(report)
		}
	}
	if primaryTarget.buffer.Len() != 0 || observerTarget.buffer.Len() != 1 {
		t.Fatalf("buffer depths primary=%d observer=%d", primaryTarget.buffer.Len(), observerTarget.buffer.Len())
	}
}

func TestKubernetesObserverPlaintextPolicyIsDestinationScoped(t *testing.T) {
	cfg := Config{
		Targets: []TargetConfig{
			{Name: "primary", URL: "https://primary.example.test", Token: "p", Authoritative: true},
			{Name: "observer", URL: "http://203.0.113.10:7655", Token: "o"},
		},
	}
	if _, err := normalizeKubernetesTargets(cfg); err == nil {
		t.Fatal("expected observer plaintext URL rejection without destination opt-in")
	}
	cfg.Targets[1].AllowPlaintextHTTP = true
	normalized, err := normalizeKubernetesTargets(cfg)
	if err != nil {
		t.Fatalf("explicit observer plaintext opt-in: %v", err)
	}
	if !normalized[1].AllowPlaintextHTTP {
		t.Fatal("observer plaintext policy was not preserved after normalization")
	}
}
