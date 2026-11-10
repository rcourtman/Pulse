package api

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// A container on a Proxmox node died. The node's Pulse agent has commands
// enabled, and no action runner is connected there. Guest actions run only
// through the runner, so Patrol cannot start the container, and the catalog
// has to say that in words a person can act on. A commands-on agent must not
// count as a runner: the fake commander panics if generic connectivity is
// asked for.
func TestPatrolCatalogSaysPulseCannotStartAGuestWithoutARunner(t *testing.T) {
	now := time.Now().UTC()
	setup := func(t *testing.T, runnerConnected bool) (aicontracts.OrchestratorActionBroker, string) {
		t.Helper()
		h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
		h.SetStateProvider(resourceStateProvider{snapshot: models.StateSnapshot{
			LastUpdate: now,
			Nodes: []models.Node{{
				ID: "pmxlab-pmxlab", Name: "pmxlab", Instance: "pmxlab", Host: "https://10.0.0.5:8006",
				Status: "online", Type: "node", LastSeen: now,
			}},
			Containers: []models.Container{{
				ID: "pmxlab:pmxlab:112", VMID: 112, Name: "gitea", Node: "pmxlab", Instance: "pmxlab",
				Status: "stopped", Type: "lxc", LastSeen: now,
			}},
			Hosts: []models.Host{{
				ID: "agent-pmxlab", Hostname: "pmxlab", MachineID: "machine-pmxlab",
				Status: "online", LastSeen: now, CommandsEnabled: true,
			}},
		}})
		agents := &fakeProxmoxActionAgentCommander{
			agentByHost: map[string]string{"pmxlab": "agent-pmxlab"},
			connected:   map[string]bool{"agent-pmxlab": runnerConnected},
		}
		h.SetActionExecutor(newRoutedActionExecutor(h, newProxmoxGuestActionExecutor(h, agents, nil)))
		registry, err := h.buildRegistry("default")
		if err != nil {
			t.Fatalf("buildRegistry: %v", err)
		}
		for _, container := range registry.Containers() {
			if container != nil && container.Name() == "gitea" {
				return NewPatrolActionBroker("default", h), container.ID()
			}
		}
		t.Fatal("the gitea container is not in the registry")
		return nil, ""
	}
	wantReason := actionlifecycle.ProxmoxGuestActionsUnavailableReason("pmxlab")

	t.Run("commands on, no runner", func(t *testing.T) {
		broker, resourceID := setup(t, false)
		catalog, err := broker.Capabilities(context.Background(), resourceID)
		if err != nil {
			t.Fatalf("Capabilities: %v", err)
		}
		if len(catalog.Capabilities) != 0 {
			t.Fatalf("no guest action can run without a runner, got %#v", catalog.Capabilities)
		}
		if len(catalog.Unavailable) != 1 || catalog.Unavailable[0].Name != "start" ||
			catalog.Unavailable[0].ReasonCode != actionlifecycle.ReasonCodeActionRunnerUnavailable ||
			catalog.Unavailable[0].Reason != wantReason {
			t.Fatalf("Unavailable = %#v, want start refused for the missing runner", catalog.Unavailable)
		}
		if strings.Contains(catalog.Unavailable[0].Reason, "typed action runner") {
			t.Fatalf("refusal still speaks in internal terms: %q", catalog.Unavailable[0].Reason)
		}
		want := &aicontracts.GuestActionsUnavailable{GuestName: "gitea", NodeName: "pmxlab", Actions: []string{"start"}, Reason: wantReason}
		if !reflect.DeepEqual(catalog.GuestActionsUnavailable, want) {
			t.Fatalf("GuestActionsUnavailable = %#v, want %#v", catalog.GuestActionsUnavailable, want)
		}
		// Commands are on. The commands-off hand-over and its install step
		// would be false here.
		if catalog.CommandsDisabled != nil {
			t.Fatalf("CommandsDisabled = %#v, want unset on a commands-on node", catalog.CommandsDisabled)
		}

		_, err = broker.Submit(context.Background(), aicontracts.ActionProposal{
			ProposalID: "proposal-1", FindingID: "finding-1", InvestigationID: "investigation-1",
			ResourceID: resourceID, CapabilityName: "start", Reason: "gitea died",
		})
		if !errors.Is(err, unified.ErrActionExecutionUnavailable) || !strings.Contains(err.Error(), wantReason) {
			t.Fatalf("Submit error = %v, want the same refusal", err)
		}
		var refused *actionlifecycle.AvailabilityRefusedError
		if !errors.As(err, &refused) || refused.Readiness.ReasonCode != actionlifecycle.ReasonCodeActionRunnerUnavailable {
			t.Fatalf("Submit refusal readiness = %#v", refused)
		}
	})

	t.Run("runner connected", func(t *testing.T) {
		broker, resourceID := setup(t, true)
		catalog, err := broker.Capabilities(context.Background(), resourceID)
		if err != nil {
			t.Fatalf("Capabilities: %v", err)
		}
		if len(catalog.Capabilities) != 1 || catalog.Capabilities[0].Name != "start" || len(catalog.Unavailable) != 0 {
			t.Fatalf("catalog = %#v, want start runnable through the runner", catalog)
		}
		if catalog.GuestActionsUnavailable != nil {
			t.Fatalf("GuestActionsUnavailable = %#v, want unset when the runner is connected", catalog.GuestActionsUnavailable)
		}
	})
}
