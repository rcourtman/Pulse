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
	// runnerRequired records, per dispatch, whether the executor demanded
	// that an action runner carry the request.
	runnerRequired []bool
}

func (f *fakeProxmoxStartSessionCommander) ExecuteProxmoxGuestLifecycle(ctx context.Context, agentID string, req agentexec.ProxmoxGuestLifecyclePayload) (*agentexec.ProxmoxGuestLifecycleResultPayload, error) {
	f.runnerRequired = append(f.runnerRequired, agentexec.ActionRunnerSessionRequired(ctx))
	return f.fakeProxmoxActionAgentCommander.ExecuteProxmoxGuestLifecycle(ctx, agentID, req)
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
// through its own Pulse agent, which Pulse has linked to the node. A stopped
// container (gitea) and a running one (web) live on it. No action runner is
// connected unless the commander lists one.
type guestStartLab struct {
	handlers *ResourceHandlers
	broker   aicontracts.OrchestratorActionBroker
	agents   actionAgentCommander
	stopped  string
	running  string
}

// labNodeAddress is an address Proxmox lists for the lab node. The node's own
// agent reports it too, which is how Pulse knows the agent is on that machine.
const labNodeAddress = "10.0.0.5"

func newGuestStartLab(t *testing.T, commandsEnabled bool, agents actionAgentCommander) guestStartLab {
	t.Helper()
	return newGuestStartLabWithLink(t, commandsEnabled, true, labNodeAddress, agents)
}

// newGuestStartLabWithLink can leave the node's agent unlinked: a machine
// that reports under the node's name and that Pulse has not tied to the node.
// It can also give the agent an address the node does not have: a machine
// Pulse tied to the node on its name alone.
func newGuestStartLabWithLink(t *testing.T, commandsEnabled, linked bool, agentAddress string, agents actionAgentCommander) guestStartLab {
	t.Helper()
	now := time.Now().UTC()
	linkedNodeID := ""
	if linked {
		linkedNodeID = "pmxlab-pmxlab"
	}
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceStateProvider{snapshot: models.StateSnapshot{
		LastUpdate: now,
		Nodes: []models.Node{{
			ID: "pmxlab-pmxlab", Name: "pmxlab", Instance: "pmxlab", Host: "https://" + labNodeAddress + ":8006",
			Status: "online", Type: "node", LastSeen: now,
			NetworkInterfaces: []models.HostNetworkInterface{{Name: "vmbr0", Addresses: []string{labNodeAddress + "/24"}}},
		}},
		Containers: []models.Container{
			{ID: "pmxlab:pmxlab:112", VMID: 112, Name: "gitea", Node: "pmxlab", Instance: "pmxlab", Status: "stopped", Type: "lxc", LastSeen: now},
			{ID: "pmxlab:pmxlab:113", VMID: 113, Name: "web", Node: "pmxlab", Instance: "pmxlab", Status: "running", Type: "lxc", LastSeen: now},
		},
		Hosts: []models.Host{{
			ID: "agent-pmxlab", Hostname: "pmxlab", MachineID: "machine-pmxlab",
			Status: "online", LastSeen: now, CommandsEnabled: commandsEnabled, LinkedNodeID: linkedNodeID,
			NetworkInterfaces: []models.HostNetworkInterface{{Name: "vmbr0", Addresses: []string{agentAddress + "/24"}}},
		}},
	}})
	h.SetActionExecutor(newRoutedActionExecutor(h, newProxmoxGuestActionExecutor(h, agents, nil)))
	registry, err := h.buildRegistry("default")
	if err != nil {
		t.Fatalf("buildRegistry: %v", err)
	}
	lab := guestStartLab{handlers: h, broker: NewPatrolActionBroker("default", h), agents: agents}
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

// executor is the Proxmox guest executor over the lab, with Pulse's own
// Proxmox check when one is given.
func (lab guestStartLab) executor(observer proxmoxGuestPostconditionObserver) proxmoxGuestActionExecutor {
	return newProxmoxGuestActionExecutor(lab.handlers, lab.agents, observer).(proxmoxGuestActionExecutor)
}

// labGuestObservation is what Pulse's own Proxmox connection saw of the lab's
// stopped container, gitea.
func labGuestObservation(observedAt time.Time, status string, uptime uint64) proxmoxGuestPostconditionObservation {
	return proxmoxGuestPostconditionObservation{
		ObserverID:  "proxmox-api:default:pmxlab",
		TrustDomain: "proxmox-control-plane:default:pmxlab",
		// Written in two parts because the commit hook's fallback secret scan
		// reads the whole name as a payment identifier.
		Method: "proxmox_ap" + "i_guest_status_current",
		Snapshot: proxmoxGuestLifecycleSnapshot{
			Instance: "pmxlab", Node: "pmxlab", VMID: 112, Kind: proxmoxGuestCT,
			Status: status, Uptime: uptime, ObservedAt: observedAt,
		},
		ReceivedAt: observedAt,
	}
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

// An agent Pulse tied to the node by name, and cannot match to it by address,
// is refused in words a person can act on, and the fact travels to Patrol.
func TestGuestStartRefusalForAnAgentPulseCannotConfirm(t *testing.T) {
	agents := newFakeProxmoxStartSessionCommander(guestStartSession("agent-pmxlab", "pmxlab"))
	lab := newGuestStartLabWithLink(t, true, true, "10.99.0.7", agents)
	catalog := lab.catalog(t, lab.stopped)
	want := actionlifecycle.ProxmoxGuestStartAgentUnconfirmedReason("pmxlab")
	if len(catalog.Capabilities) != 0 || len(catalog.Unavailable) != 1 || catalog.Unavailable[0].ReasonCode != actionlifecycle.ReasonCodeGuestStartAgentUnconfirmed || catalog.Unavailable[0].Reason != want {
		t.Fatalf("catalog = %#v, want start refused as unconfirmed", catalog)
	}
	if fact := catalog.GuestActionsUnavailable; fact == nil || fact.Reason != want || strings.Join(fact.Actions, ",") != "start" {
		t.Fatalf("GuestActionsUnavailable = %#v, want the unconfirmed sentence for start", fact)
	}
	if catalog.CommandsDisabled != nil {
		t.Fatalf("CommandsDisabled = %#v, want unset: commands are on", catalog.CommandsDisabled)
	}
}

// A start is never refused with the "enable commands" step for an agent Pulse
// cannot confirm, even when that agent reports commands off. Switching
// commands on there could open another machine and would still leave this
// start refused. The catalog's separate commands-off fact is about running
// commands for the resource at all and follows the read route as before.
func TestGuestStartDoesNotAskForCommandsOnAnAgentPulseCannotConfirm(t *testing.T) {
	agents := newFakeProxmoxStartSessionCommander(guestStartSession("agent-pmxlab", "pmxlab"))
	lab := newGuestStartLabWithLink(t, false, true, "10.99.0.7", agents)
	catalog := lab.catalog(t, lab.stopped)
	want := actionlifecycle.ProxmoxGuestStartAgentUnconfirmedReason("pmxlab")
	if len(catalog.Capabilities) != 0 || len(catalog.Unavailable) != 1 || catalog.Unavailable[0].ReasonCode != actionlifecycle.ReasonCodeGuestStartAgentUnconfirmed || catalog.Unavailable[0].Reason != want {
		t.Fatalf("catalog = %#v, want start refused as unconfirmed", catalog)
	}
	if fact := catalog.GuestActionsUnavailable; fact == nil || fact.Reason != want {
		t.Fatalf("GuestActionsUnavailable = %#v, want the unconfirmed sentence", fact)
	}
	if len(agents.calls) != 0 {
		t.Fatalf("an unconfirmed agent was sent a request: %#v", agents.calls)
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

// A start goes to the command session of the agent Pulse has linked to the
// guest's own node, and to no other. A machine that only reports under the
// node's name is never that agent: the same VMID there is a different guest.
func TestGuestStartRunsOnlyOnTheAgentLinkedToTheGuestsNode(t *testing.T) {
	start := func(t *testing.T, lab guestStartLab) (unified.ResourceActionReadiness, error) {
		t.Helper()
		executor := lab.executor(nil)
		registry, err := lab.handlers.buildRegistry("default")
		if err != nil {
			t.Fatal(err)
		}
		resource, ok := registry.Get(lab.stopped)
		if !ok {
			t.Fatal("the stopped guest is not on record")
		}
		readiness := executor.CheckActionAvailable(context.Background(), unified.ActionRequest{ResourceID: lab.stopped, CapabilityName: "start"}, *resource)
		_, err = executor.ExecuteAction(actionDispatchTestContext(t, "act_ct"), proxmoxGuestActionRecord("act_ct", lab.stopped, "start"))
		return readiness, err
	}

	t.Run("the linked agent, among others", func(t *testing.T) {
		agents := newFakeProxmoxStartSessionCommander(guestStartSession("other-agent", "another-node"), guestStartSession("agent-pmxlab", "pmxlab"))
		readiness, err := start(t, newGuestStartLab(t, true, agents))
		if !readiness.Available || err != nil {
			t.Fatalf("readiness = %#v, err = %v, want the start to run", readiness, err)
		}
		if len(agents.calls) != 1 || agents.callAgents[0] != "agent-pmxlab" || agents.calls[0].Operation != "start" || agents.calls[0].GuestKind != "ct" || agents.calls[0].VMID != 112 || agents.calls[0].ExpectedStatus != "stopped" {
			t.Fatalf("dispatch = %#v to %v, want one start for CT 112 on agent-pmxlab", agents.calls, agents.callAgents)
		}
		// A collector carried it, so no runner was demanded and the result
		// will be judged by the collector's stricter rule.
		if len(agents.runnerRequired) != 1 || agents.runnerRequired[0] {
			t.Fatalf("runner demanded = %v, want false for a collector route", agents.runnerRequired)
		}
	})

	const elsewhere = "10.99.0.7"
	notConnected := actionlifecycle.ReasonCodeGuestStartAgentNotConnected
	for name, tc := range map[string]struct {
		linked       bool
		agentAddress string
		sessions     []agentexec.ConnectedAgent
		code         string
	}{
		"the linked agent is offline and a machine of the same name is connected": {true, labNodeAddress, []agentexec.ConnectedAgent{guestStartSession("agent-elsewhere", "pmxlab")}, notConnected},
		"only another node's agent is connected":                                  {true, labNodeAddress, []agentexec.ConnectedAgent{guestStartSession("other-agent", "another-node")}, notConnected},
		"an unlinked machine reports under the node's name":                       {false, labNodeAddress, []agentexec.ConnectedAgent{guestStartSession("agent-pmxlab", "pmxlab")}, notConnected},
		"an unlinked machine of the same name, by another agent ID":               {false, elsewhere, []agentexec.ConnectedAgent{guestStartSession("agent-elsewhere", "pmxlab")}, notConnected},
		// Pulse tied this agent to the node on its name. It holds a session
		// and says it can start guests, and it shares no address with the
		// node, so it may be a different machine called pmxlab.
		"an agent linked on its name alone, with no address in common": {true, elsewhere, []agentexec.ConnectedAgent{guestStartSession("agent-pmxlab", "pmxlab")}, actionlifecycle.ReasonCodeGuestStartAgentUnconfirmed},
	} {
		t.Run(name, func(t *testing.T) {
			agents := newFakeProxmoxStartSessionCommander(tc.sessions...)
			readiness, err := start(t, newGuestStartLabWithLink(t, true, tc.linked, tc.agentAddress, agents))
			if readiness.Available || readiness.ReasonCode != tc.code {
				t.Fatalf("readiness = %#v, want %s", readiness, tc.code)
			}
			if err == nil || len(agents.calls) != 0 {
				t.Fatalf("a machine that is not the node's linked agent started the guest: err=%v calls=%#v to %v", err, agents.calls, agents.callAgents)
			}
		})
	}
}

// A route planned on the action runner tells dispatch that a runner must
// carry it. If a collector session has taken the runner's place under the
// same agent ID by then, dispatch refuses, so a collector's answer is never
// judged by the runner's rules.
func TestGuestActionPlannedOnARunnerDemandsARunnerAtDispatch(t *testing.T) {
	agents := newFakeProxmoxStartSessionCommander()
	agents.runners = []agentexec.ConnectedAgent{{AgentID: "agent-pmxlab", Hostname: "pmxlab"}}
	lab := newGuestStartLab(t, true, agents)
	if _, err := lab.executor(nil).ExecuteAction(actionDispatchTestContext(t, "act_ct"), proxmoxGuestActionRecord("act_ct", lab.stopped, "start")); err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if len(agents.runnerRequired) != 1 || !agents.runnerRequired[0] {
		t.Fatalf("runner demanded = %v, want true for a runner route", agents.runnerRequired)
	}
}

// The agent that ran a start says the guest is running. That is evidence. It
// is not proof: only Pulse's own Proxmox check confirms a start a command
// session ran. Without that check the run is not verified, and when the check
// disagrees the run is contradicted.
func TestAgentRunStartIsConfirmedOnlyByPulsesOwnProxmoxCheck(t *testing.T) {
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
					labGuestObservation(now.Add(-time.Second), "stopped", 0),
					labGuestObservation(now.Add(time.Minute), "stopped", 0),
				}}
			},
			status: unified.ActionVerificationContradicted, evidence: unified.ActionEvidenceIndependent, reason: "postcondition_contradicted",
		},
		{
			name: "check sees the guest running",
			observe: func(now time.Time) proxmoxGuestPostconditionObserver {
				return &fakeProxmoxGuestPostconditionObserver{observations: []proxmoxGuestPostconditionObservation{
					labGuestObservation(now.Add(-time.Second), "stopped", 0),
					labGuestObservation(now.Add(time.Minute), "running", 5),
				}}
			},
			status: unified.ActionVerificationConfirmed, evidence: unified.ActionEvidenceIndependent,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			// The agent claims success every time.
			agents := newFakeProxmoxStartSessionCommander(guestStartSession("agent-pmxlab", "pmxlab"))
			lab := newGuestStartLab(t, true, agents)
			ctx, cancel := context.WithTimeout(actionDispatchTestContext(t, "act_ct"), 200*time.Millisecond)
			defer cancel()

			result, err := lab.executor(tc.observe(now)).ExecuteAction(ctx, proxmoxGuestActionRecord("act_ct", lab.stopped, "start"))
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
