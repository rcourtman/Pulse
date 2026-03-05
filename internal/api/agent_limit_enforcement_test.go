package api

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func TestAgentCountNilMonitor(t *testing.T) {
	got := agentCount(nil)
	if got != 0 {
		t.Fatalf("expected 0 for nil monitor, got %d", got)
	}
}

func TestHostReportTargetsExistingHostBridge(t *testing.T) {
	snapshot := models.StateSnapshot{
		Hosts: []models.Host{{ID: "host-1"}},
	}

	t.Run("matches_existing", func(t *testing.T) {
		report := agentshost.Report{
			Host: agentshost.HostInfo{ID: "host-1"},
		}
		if !hostReportTargetsExistingHost(snapshot.Hosts, report, nil) {
			t.Fatal("expected match by host ID")
		}
	})

	t.Run("no_match_for_new_host", func(t *testing.T) {
		report := agentshost.Report{
			Host: agentshost.HostInfo{ID: "host-new", Hostname: "new-server"},
		}
		if hostReportTargetsExistingHost(snapshot.Hosts, report, nil) {
			t.Fatal("expected no match for unknown host")
		}
	})

	t.Run("token_record_forwarded", func(t *testing.T) {
		snapshot := models.StateSnapshot{
			Hosts: []models.Host{{Hostname: "srv-1", TokenID: "token-a"}},
		}
		report := agentshost.Report{
			Host: agentshost.HostInfo{Hostname: "srv-1"},
		}
		token := &config.APITokenRecord{ID: "token-a"}
		if !hostReportTargetsExistingHost(snapshot.Hosts, report, token) {
			t.Fatal("expected match with matching token")
		}
		wrongToken := &config.APITokenRecord{ID: "token-b"}
		if hostReportTargetsExistingHost(snapshot.Hosts, report, wrongToken) {
			t.Fatal("expected no match with different token")
		}
	})
}

func TestDeployReservedCount(t *testing.T) {
	// Nil counter returns 0.
	SetDeployReservationCounter(nil)
	if got := deployReservedCount(context.Background()); got != 0 {
		t.Fatalf("expected 0 with nil counter, got %d", got)
	}

	// Wired counter returns value.
	SetDeployReservationCounter(func(_ context.Context) int { return 5 })
	t.Cleanup(func() { SetDeployReservationCounter(nil) })

	if got := deployReservedCount(context.Background()); got != 5 {
		t.Fatalf("expected 5, got %d", got)
	}
}
