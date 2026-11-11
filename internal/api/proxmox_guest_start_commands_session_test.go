package api

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// fakeProxmoxStartSessionCommander is a node with no action runner whose own
// Pulse agent may hold a command session. The embedded fake still panics if
// anything asks for generic connectivity or sends a generic command.
type fakeProxmoxStartSessionCommander struct {
	*fakeProxmoxActionAgentCommander
	sessions []agentexec.ConnectedAgent
}

func (f *fakeProxmoxStartSessionCommander) GetProxmoxGuestStartSessionsForOrganization(string) []agentexec.ConnectedAgent {
	return f.sessions
}

func newFakeProxmoxStartSessionCommander(sessions ...agentexec.ConnectedAgent) *fakeProxmoxStartSessionCommander {
	return &fakeProxmoxStartSessionCommander{
		fakeProxmoxActionAgentCommander: &fakeProxmoxActionAgentCommander{connected: map[string]bool{}},
		sessions:                        sessions,
	}
}

func guestStartSession(agentID, hostname string) agentexec.ConnectedAgent {
	return agentexec.ConnectedAgent{
		AgentID: agentID, Hostname: hostname, OperationReceiptVersion: 1,
		ProxmoxGuestStartVersion: agentexec.ProxmoxGuestStartProtocolVersion,
	}
}

// guestStartLab is one Proxmox node, pmxlab, monitored through the API and
// through its own Pulse agent. A stopped container (gitea) and a running one
// (web) live on it. No action runner is connected.
type guestStartLab struct {
	handlers *ResourceHandlers
	broker   aicontracts.OrchestratorActionBroker
	stopped  string
	running  string
}

func newGuestStartLab(t *testing.T, commandsEnabled bool, agents actionAgentCommander) guestStartLab {
	t.Helper()
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceStateProvider{snapshot: models.StateSnapshot{
		LastUpdate: now,
		Nodes: []models.Node{{
			ID: "pmxlab-pmxlab", Name: "pmxlab", Instance: "pmxlab", Host: "https://10.0.0.5:8006",
			Status: "online", Type: "node", LastSeen: now,
		}},
		Containers: []models.Container{
			{ID: "pmxlab:pmxlab:112", VMID: 112, Name: "gitea", Node: "pmxlab", Instance: "pmxlab", Status: "stopped", Type: "lxc", LastSeen: now},
			{ID: "pmxlab:pmxlab:113", VMID: 113, Name: "web", Node: "pmxlab", Instance: "pmxlab", Status: "running", Type: "lxc", LastSeen: now},
		},
		Hosts: []models.Host{{
			ID: "agent-pmxlab", Hostname: "pmxlab", MachineID: "machine-pmxlab",
			Status: "online", LastSeen: now, CommandsEnabled: commandsEnabled,
		}},
	}})
	h.SetActionExecutor(newRoutedActionExecutor(h, newProxmoxGuestActionExecutor(h, agents, nil)))
	registry, err := h.buildRegistry("default")
	if err != nil {
		t.Fatalf("buildRegistry: %v", err)
	}
	lab := guestStartLab{handlers: h, broker: NewPatrolActionBroker("default", h)}
	for _, container := range registry.Containers() {
		if container == nil {
			continue
		}
		switch container.Name() {
		case "gitea":
			lab.stopped = container.ID()
		case "web":
			lab.running = container.ID()
		}
	}
	if lab.stopped == "" || lab.running == "" {
		t.Fatal("the lab containers are not in the registry")
	}
	return lab
}

func (lab guestStartLab) catalog(t *testing.T, resourceID string) aicontracts.ActionCapabilityCatalog {
	t.Helper()
	catalog, err := lab.broker.Capabilities(context.Background(), resourceID)
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	return catalog
}

// A container died on a node whose owner ticked "Allow Pulse to run commands
// on this host". The node's agent reported that it can start a guest. Patrol's
// catalog offers start, still behind an admin's approval, and offers nothing
// else: stopping or restarting a guest stays with the action runner.
func TestPatrolCatalogOffersStartThroughACommandsOnAgentAndNothingElse(t *testing.T) {
	agents := newFakeProxmoxStartSessionCommander(guestStartSession("agent-pmxlab", "pmxlab"))
	lab := newGuestStartLab(t, true, agents)

	stopped := lab.catalog(t, lab.stopped)
	if len(stopped.Capabilities) != 1 || stopped.Capabilities[0].Name != "start" || len(stopped.Unavailable) != 0 {
		t.Fatalf("stopped guest catalog = %#v, want start runnable and nothing refused", stopped)
	}
	if got := stopped.Capabilities[0]; got.MinimumApprovalLevel != string(unified.ApprovalAdmin) || got.AutoAuthorization != string(unified.AutoAuthorizeNever) {
		t.Fatalf("start = %#v, want admin approval and no automatic authorization", got)
	}
	if stopped.GuestActionsUnavailable != nil || stopped.CommandsDisabled != nil {
		t.Fatalf("stopped guest catalog carries a refusal fact: %#v", stopped)
	}

	running := lab.catalog(t, lab.running)
	if len(running.Capabilities) != 0 {
		t.Fatalf("a commands session ran something other than start: %#v", running.Capabilities)
	}
	wantReason := actionlifecycle.ProxmoxGuestRunnerOnlyReason("pmxlab")
	var refused []string
	for _, refusal := range running.Unavailable {
		if refusal.ReasonCode != actionlifecycle.ReasonCodeActionRunnerUnavailable || refusal.Reason != wantReason {
			t.Fatalf("refusal = %#v, want the runner-only refusal", refusal)
		}
		refused = append(refused, refusal.Name)
	}
	sort.Strings(refused)
	if len(refused) == 0 || strings.Contains(strings.Join(refused, ","), "start") {
		t.Fatalf("running guest refusals = %v, want its stop actions and no start", refused)
	}
	for _, verb := range []string{"shutdown", "stop"} {
		if !strings.Contains(","+strings.Join(refused, ",")+",", ","+verb+",") {
			t.Fatalf("running guest refusals = %v, want %s among them", refused, verb)
		}
	}
	fact := running.GuestActionsUnavailable
	if fact == nil || fact.Reason != wantReason || fact.NodeName != "pmxlab" || strings.Join(fact.Actions, ",") != strings.Join(refused, ",") {
		t.Fatalf("GuestActionsUnavailable = %#v, want the refused actions %v with the runner-only sentence", fact, refused)
	}

	disposition, err := lab.broker.Submit(context.Background(), aicontracts.ActionProposal{
		ProposalID: "proposal-start", FindingID: "finding-1", InvestigationID: "investigation-1",
		ResourceID: lab.stopped, CapabilityName: "start", Reason: "gitea died",
	})
	if err != nil {
		t.Fatalf("Submit(start) = %v, want the plan accepted", err)
	}
	if !disposition.Plan.RequiresApproval || disposition.Plan.ApprovalPolicy != string(unified.ApprovalAdmin) {
		t.Fatalf("start plan = %#v, want it to wait for an admin's approval", disposition.Plan)
	}
	// Nothing runs until an admin approves.
	if len(agents.calls) != 0 {
		t.Fatalf("a start ran without an approval: %#v", agents.calls)
	}
}

// Every reason a start cannot run on a node without a runner has its own code
// and its own sentence, so the page and Patrol can tell a setting from an old
// agent from a broken one.
func TestGuestStartRefusalSaysWhyOnEachKindOfNode(t *testing.T) {
	session := func(change func(*agentexec.ConnectedAgent)) []agentexec.ConnectedAgent {
		agent := guestStartSession("agent-pmxlab", "pmxlab")
		change(&agent)
		return []agentexec.ConnectedAgent{agent}
	}
	for _, tc := range []struct {
		name            string
		commandsEnabled bool
		sessions        []agentexec.ConnectedAgent
		code            string
		reason          string
		fact            bool
		commandsFact    bool
	}{
		{
			name: "no agent takes commands", commandsEnabled: true,
			code: actionlifecycle.ReasonCodeGuestStartAgentNotConnected, reason: actionlifecycle.ProxmoxGuestStartAgentNotConnectedReason("pmxlab"), fact: true,
		},
		{
			name: "agent predates guest start", commandsEnabled: true,
			sessions: session(func(a *agentexec.ConnectedAgent) { a.ProxmoxGuestStartVersion = 0 }),
			code:     actionlifecycle.ReasonCodeGuestStartAgentUpdateRequired, reason: actionlifecycle.ProxmoxGuestStartAgentUpdateReason("pmxlab"), fact: true,
		},
		{
			name: "agent reports a version this server does not know", commandsEnabled: true,
			sessions: session(func(a *agentexec.ConnectedAgent) { a.ProxmoxGuestStartVersion = 2 }),
			code:     actionlifecycle.ReasonCodeGuestStartAgentUpdateRequired, reason: actionlifecycle.ProxmoxGuestStartAgentUpdateReason("pmxlab"), fact: true,
		},
		{
			name: "agent is not a systemd service", commandsEnabled: true,
			sessions: session(func(a *agentexec.ConnectedAgent) {
				a.ProxmoxGuestStartVersion, a.ProxmoxGuestStartUnavailable = 0, agentexec.ProxmoxGuestStartUnavailableNotSystemdService
			}),
			code: actionlifecycle.ReasonCodeGuestStartUnavailable, reason: actionlifecycle.ProxmoxGuestStartNotSystemdReason("pmxlab"), fact: true,
		},
		{
			name: "agent could not set up containment", commandsEnabled: true,
			sessions: session(func(a *agentexec.ConnectedAgent) {
				a.ProxmoxGuestStartVersion, a.ProxmoxGuestStartUnavailable = 0, agentexec.ProxmoxGuestStartUnavailableContainment
			}),
			code: actionlifecycle.ReasonCodeGuestStartUnavailable, reason: actionlifecycle.ProxmoxGuestStartContainmentReason("pmxlab"), fact: true,
		},
		{
			name: "agent keeps no receipts", commandsEnabled: true,
			sessions: session(func(a *agentexec.ConnectedAgent) { a.ProxmoxGuestStartVersion, a.OperationReceiptVersion = 0, 0 }),
			code:     "operation_receipt_unsupported", reason: proxmoxGuestStartReceiptsReason,
		},
		{
			// A monitoring-only agent holds no command session at all.
			name: "monitoring-only agent", commandsEnabled: false,
			code: actionlifecycle.ReasonCodeHostCommandsDisabled, reason: actionlifecycle.HostCommandsDisabledRefusal("pmxlab"), commandsFact: true,
		},
		{
			// The agent says commands are off while an old session is still
			// listed. The agent's own report wins.
			name: "commands turned off, session not closed yet", commandsEnabled: false,
			sessions: session(func(*agentexec.ConnectedAgent) {}),
			code:     actionlifecycle.ReasonCodeHostCommandsDisabled, reason: actionlifecycle.HostCommandsDisabledRefusal("pmxlab"), commandsFact: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agents := newFakeProxmoxStartSessionCommander(tc.sessions...)
			lab := newGuestStartLab(t, tc.commandsEnabled, agents)
			catalog := lab.catalog(t, lab.stopped)
			if len(catalog.Capabilities) != 0 {
				t.Fatalf("start is runnable: %#v", catalog.Capabilities)
			}
			if len(catalog.Unavailable) != 1 || catalog.Unavailable[0].Name != "start" || catalog.Unavailable[0].ReasonCode != tc.code || catalog.Unavailable[0].Reason != tc.reason {
				t.Fatalf("Unavailable = %#v, want start refused with %s: %q", catalog.Unavailable, tc.code, tc.reason)
			}
			if tc.fact {
				want := aicontracts.GuestActionsUnavailable{GuestName: "gitea", NodeName: "pmxlab", Actions: []string{"start"}, Reason: tc.reason}
				if got := catalog.GuestActionsUnavailable; got == nil || got.GuestName != want.GuestName || got.NodeName != want.NodeName || strings.Join(got.Actions, ",") != "start" || got.Reason != want.Reason {
					t.Fatalf("GuestActionsUnavailable = %#v, want %#v", got, want)
				}
			} else if catalog.GuestActionsUnavailable != nil {
				t.Fatalf("GuestActionsUnavailable = %#v, want unset", catalog.GuestActionsUnavailable)
			}
			if (catalog.CommandsDisabled != nil) != tc.commandsFact {
				t.Fatalf("CommandsDisabled = %#v, want set=%v", catalog.CommandsDisabled, tc.commandsFact)
			}
			if tc.commandsFact && catalog.CommandsDisabled.HostName != "pmxlab" {
				t.Fatalf("CommandsDisabled = %#v, want the node's own agent", catalog.CommandsDisabled)
			}
			_, err := lab.broker.Submit(context.Background(), aicontracts.ActionProposal{
				ProposalID: "proposal-start", FindingID: "finding-1", InvestigationID: "investigation-1",
				ResourceID: lab.stopped, CapabilityName: "start", Reason: "gitea died",
			})
			if err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("Submit(start) = %v, want the same refusal", err)
			}
			if len(agents.calls) != 0 {
				t.Fatalf("a refused start was dispatched: %#v", agents.calls)
			}
		})
	}
}

// startableProxmoxVM is the seeded VM the executor tests use, stopped, with
// the start capability a stopped guest declares.
func startableProxmoxVM(now time.Time) unified.Resource {
	resource := proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "stopped", now)
	resource.Capabilities = []unified.ResourceCapability{{
		Name: "start", Type: unified.CapabilityTypeCommon, Description: "Start this Proxmox VM",
		MinimumApprovalLevel: unified.ApprovalAdmin, Platform: "qemu", InternalHandler: proxmoxVMLifecycleHandler,
	}}
	return resource
}

// Stop, shutdown and reboot never ride a collector's command session, however
// capable its agent says it is. Each is refused when planned, when bound and
// when executed, and nothing reaches the agent.
func TestCommandsSessionNeverCarriesStopShutdownOrReboot(t *testing.T) {
	for _, verb := range []string{"stop", "shutdown", "reboot"} {
		t.Run(verb, func(t *testing.T) {
			now := time.Now().UTC()
			resource := proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now)
			h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
			h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{resource}})
			agents := newFakeProxmoxStartSessionCommander(guestStartSession("node-agent-1", "delly"))
			executor := newProxmoxGuestActionExecutor(h, agents, nil).(proxmoxGuestActionExecutor)

			readiness := executor.CheckActionAvailable(context.Background(), unified.ActionRequest{ResourceID: "vm:160", CapabilityName: verb}, resource)
			if readiness.Available || readiness.ReasonCode != actionlifecycle.ReasonCodeActionRunnerUnavailable || readiness.Reason != actionlifecycle.ProxmoxGuestRunnerOnlyReason("delly") {
				t.Fatalf("readiness = %#v, want the runner-only refusal", readiness)
			}
			attempt, err := unified.NewActionDispatchAttempt("act_vm", now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := executor.BindActionDispatch(context.Background(), proxmoxGuestActionRecord("act_vm", "vm:160", verb), attempt); err == nil {
				t.Fatalf("%s was bound to a command session", verb)
			}
			if _, err := executor.ExecuteAction(actionDispatchTestContext(t, "act_vm"), proxmoxGuestActionRecord("act_vm", "vm:160", verb)); err == nil || !strings.Contains(err.Error(), "needs the Pulse action runner") {
				t.Fatalf("ExecuteAction(%s) = %v, want the runner-only refusal", verb, err)
			}
			if len(agents.calls) != 0 {
				t.Fatalf("%s reached the agent: %#v", verb, agents.calls)
			}
		})
	}
}

// A start goes to the command session on the guest's own node and to no other
// session, and it is bound to that agent before it is sent.
func TestGuestStartRunsOnTheNodesOwnCommandSession(t *testing.T) {
	now := time.Now().UTC()
	resource := startableProxmoxVM(now)
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{resource}})
	agents := newFakeProxmoxStartSessionCommander(guestStartSession("other-agent", "another-node"), guestStartSession("node-agent-1", "delly"))
	executor := newProxmoxGuestActionExecutor(h, agents, nil).(proxmoxGuestActionExecutor)

	if readiness := executor.CheckActionAvailable(context.Background(), unified.ActionRequest{ResourceID: "vm:160", CapabilityName: "start"}, resource); !readiness.Available {
		t.Fatalf("readiness = %#v, want start available", readiness)
	}
	if _, err := executor.ExecuteAction(actionDispatchTestContext(t, "act_vm"), proxmoxGuestActionRecord("act_vm", "vm:160", "start")); err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if len(agents.calls) != 1 || agents.callAgents[0] != "node-agent-1" || agents.calls[0].Operation != "start" || agents.calls[0].VMID != 160 || agents.calls[0].ExpectedStatus != "stopped" {
		t.Fatalf("dispatch = %#v to %v, want one start for VM 160 on node-agent-1", agents.calls, agents.callAgents)
	}

	// Only another node's agent is connected: nothing may start this guest.
	elsewhere := newFakeProxmoxStartSessionCommander(guestStartSession("other-agent", "another-node"))
	executor = newProxmoxGuestActionExecutor(h, elsewhere, nil).(proxmoxGuestActionExecutor)
	if readiness := executor.CheckActionAvailable(context.Background(), unified.ActionRequest{ResourceID: "vm:160", CapabilityName: "start"}, resource); readiness.Available || readiness.ReasonCode != actionlifecycle.ReasonCodeGuestStartAgentNotConnected {
		t.Fatalf("readiness = %#v, want no agent connected on this node", readiness)
	}
	if _, err := executor.ExecuteAction(actionDispatchTestContext(t, "act_vm"), proxmoxGuestActionRecord("act_vm", "vm:160", "start")); err == nil || len(elsewhere.calls) != 0 {
		t.Fatalf("another node's agent started the guest: err=%v calls=%#v", err, elsewhere.calls)
	}
}

// The agent that ran a start says the guest is running. That is evidence. It
// is not proof: only Pulse's own Proxmox check confirms a start a command
// session ran. Without that check the run is not verified, and when the check
// disagrees the run is contradicted.
func TestAgentRunStartIsConfirmedOnlyByPulsesOwnProxmoxCheck(t *testing.T) {
	const controlPlane = "proxmox-control-plane:default:homelab"
	for _, tc := range []struct {
		name     string
		observe  func(time.Time) proxmoxGuestPostconditionObserver
		status   unified.ActionVerificationStatus
		evidence unified.ActionEvidenceClass
		reason   string
	}{
		{
			name:    "no check available",
			observe: func(time.Time) proxmoxGuestPostconditionObserver { return nil },
			status:  unified.ActionVerificationInconclusive, evidence: unified.ActionEvidenceAgentAttested, reason: "independent_observation_required",
		},
		{
			name: "check times out",
			observe: func(time.Time) proxmoxGuestPostconditionObserver {
				return &fakeProxmoxGuestPostconditionObserver{}
			},
			status: unified.ActionVerificationInconclusive, evidence: unified.ActionEvidenceAgentAttested, reason: "independent_observation_required",
		},
		{
			name: "check sees the guest still stopped",
			observe: func(now time.Time) proxmoxGuestPostconditionObserver {
				return &fakeProxmoxGuestPostconditionObserver{observations: []proxmoxGuestPostconditionObservation{
					proxmoxGuestActionObservation(now.Add(-time.Second), "stopped", 0, controlPlane),
					proxmoxGuestActionObservation(now.Add(time.Minute), "stopped", 0, controlPlane),
				}}
			},
			status: unified.ActionVerificationContradicted, evidence: unified.ActionEvidenceIndependent, reason: "postcondition_contradicted",
		},
		{
			name: "check sees the guest running",
			observe: func(now time.Time) proxmoxGuestPostconditionObserver {
				return &fakeProxmoxGuestPostconditionObserver{observations: []proxmoxGuestPostconditionObservation{
					proxmoxGuestActionObservation(now.Add(-time.Second), "stopped", 0, controlPlane),
					proxmoxGuestActionObservation(now.Add(time.Minute), "running", 5, controlPlane),
				}}
			},
			status: unified.ActionVerificationConfirmed, evidence: unified.ActionEvidenceIndependent,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			resource := startableProxmoxVM(now)
			h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
			h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{resource}})
			// The agent claims success every time.
			agents := newFakeProxmoxStartSessionCommander(guestStartSession("node-agent-1", "delly"))
			executor := newProxmoxGuestActionExecutor(h, agents, tc.observe(now))
			ctx, cancel := context.WithTimeout(actionDispatchTestContext(t, "act_vm"), 200*time.Millisecond)
			defer cancel()

			result, err := executor.ExecuteAction(ctx, proxmoxGuestActionRecord("act_vm", "vm:160", "start"))
			if err != nil {
				t.Fatalf("ExecuteAction: %v", err)
			}
			if result == nil || result.ActionResultV2 == nil || len(agents.calls) != 1 {
				t.Fatalf("result = %#v, calls = %#v", result, agents.calls)
			}
			truth := result.ActionResultV2.Verification
			if truth.Status != tc.status || truth.EvidenceClass != tc.evidence || truth.ReasonCode != tc.reason {
				t.Fatalf("verification = %#v, want %s by %s evidence (%q)", truth, tc.status, tc.evidence, tc.reason)
			}
			confirmed := tc.status == unified.ActionVerificationConfirmed
			if (result.Verification != nil && result.Verification.Success) != confirmed {
				t.Fatalf("legacy verification = %#v, want success=%v", result.Verification, confirmed)
			}
		})
	}
}
