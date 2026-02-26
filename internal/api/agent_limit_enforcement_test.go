package api

import (
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
		if !hostReportTargetsExistingHost(snapshot, report, nil) {
			t.Fatal("expected match by host ID")
		}
	})

	t.Run("no_match_for_new_host", func(t *testing.T) {
		report := agentshost.Report{
			Host: agentshost.HostInfo{ID: "host-new", Hostname: "new-server"},
		}
		if hostReportTargetsExistingHost(snapshot, report, nil) {
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
		if !hostReportTargetsExistingHost(snapshot, report, token) {
			t.Fatal("expected match with matching token")
		}
		wrongToken := &config.APITokenRecord{ID: "token-b"}
		if hostReportTargetsExistingHost(snapshot, report, wrongToken) {
			t.Fatal("expected no match with different token")
		}
	})
}
