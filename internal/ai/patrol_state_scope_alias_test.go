package ai

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// TestDockerServiceAlertScopeAliasesMatchAlertResourceIDs pins the contract that
// makes #1699 work: the alias Patrol registers on a Docker host must be byte
// identical to the resource ID the alert subsystem stamps on a Swarm service
// alert. If either side changes its ID format independently, service alerts stop
// resolving to a scope and Patrol silently declines to investigate them.
func TestDockerServiceAlertScopeAliasesMatchAlertResourceIDs(t *testing.T) {
	services := []models.DockerService{
		{ID: "svc-1", Name: "web"},
		{ID: "svc-2", Name: "api"},
	}

	got := dockerServiceAlertScopeAliases("host-1", services)

	want := make([]string, 0, len(services))
	for _, service := range services {
		want = append(want, alerts.DockerServiceResourceID("host-1", service.ID, service.Name))
	}

	if len(got) != len(want) {
		t.Fatalf("alias count = %d, want %d (got %v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("alias[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	// Guard the format itself, so a coordinated rename on both sides that broke
	// the stored alert IDs would still be caught here.
	if got[0] != "docker:host-1/service/svc-1" {
		t.Errorf("alias[0] = %q, want %q", got[0], "docker:host-1/service/svc-1")
	}
}

// TestDockerServiceAlertScopeAliasesSkipHostlessServices covers the
// anti-aliasing guard. Without a host ID the alert subsystem falls back to
// "docker-service:<name>", which is not host qualified and would collide across
// hosts, so Patrol must register nothing rather than a colliding alias.
func TestDockerServiceAlertScopeAliasesSkipHostlessServices(t *testing.T) {
	services := []models.DockerService{{ID: "svc-1", Name: "web"}}

	for _, hostID := range []string{"", "   ", "\t"} {
		got := dockerServiceAlertScopeAliases(hostID, services)
		if got != nil {
			t.Errorf("hostID %q: aliases = %v, want nil", hostID, got)
		}
		for _, alias := range got {
			if strings.HasPrefix(alias, "docker-service:") {
				t.Errorf("hostID %q: registered host-less alias %q, which collides across hosts", hostID, alias)
			}
		}
	}

	// Sanity check that the collidable form is really what a host-less ID
	// produces, so this test keeps its meaning if the fallback changes.
	if fallback := alerts.DockerServiceResourceID("", "svc-1", "web"); !strings.HasPrefix(fallback, "docker-service:") {
		t.Fatalf("host-less resource ID = %q, expected the un-host-qualified docker-service: form", fallback)
	}
}

func TestDockerServiceAlertScopeAliasesWithoutServices(t *testing.T) {
	if got := dockerServiceAlertScopeAliases("host-1", nil); got != nil {
		t.Errorf("nil services: aliases = %v, want nil", got)
	}
	if got := dockerServiceAlertScopeAliases("host-1", []models.DockerService{}); got != nil {
		t.Errorf("empty services: aliases = %v, want nil", got)
	}
}

// TestDockerServiceAlertScopeAliasesDeriveIDFromName covers services that carry
// no ID, where the canonical builder derives one from the service name. The
// alias has to follow that derivation rather than formatting its own string.
func TestDockerServiceAlertScopeAliasesDeriveIDFromName(t *testing.T) {
	got := dockerServiceAlertScopeAliases("host-1", []models.DockerService{{Name: "Web API"}})

	want := alerts.DockerServiceResourceID("host-1", "", "Web API")
	if len(got) != 1 || got[0] != want {
		t.Fatalf("aliases = %v, want [%q]", got, want)
	}
	if got[0] != "docker:host-1/service/web-api" {
		t.Errorf("alias = %q, want %q", got[0], "docker:host-1/service/web-api")
	}
}
