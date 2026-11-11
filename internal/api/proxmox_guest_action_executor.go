package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	proxmoxVMLifecycleHandler = "proxmox.vm.lifecycle"
	proxmoxCTLifecycleHandler = "proxmox.ct.lifecycle"
)

type proxmoxGuestKind string

const (
	proxmoxGuestVM proxmoxGuestKind = "vm"
	proxmoxGuestCT proxmoxGuestKind = "ct"
)

type proxmoxGuestActionExecutor struct {
	resources *ResourceHandlers
	agents    actionAgentCommander
	observer  proxmoxGuestPostconditionObserver
}

type proxmoxGuestLifecycleSnapshot struct {
	Instance   string
	Node       string
	VMID       int
	Kind       proxmoxGuestKind
	Status     string
	Uptime     uint64
	ObservedAt time.Time
}

type proxmoxGuestPostconditionObservation struct {
	ObserverID  string
	TrustDomain string
	Method      string
	Snapshot    proxmoxGuestLifecycleSnapshot
	ReceivedAt  time.Time
}

type proxmoxGuestPostconditionObserver interface {
	ObserveProxmoxGuest(context.Context, string, string, string, int, proxmoxGuestKind) (proxmoxGuestPostconditionObservation, error)
}

type proxmoxGuestLifecycleAgentCommander interface {
	GetActionRunnersForOrganization(organizationID string) []agentexec.ConnectedAgent
	ExecuteProxmoxGuestLifecycle(context.Context, string, agentexec.ProxmoxGuestLifecyclePayload) (*agentexec.ProxmoxGuestLifecycleResultPayload, error)
}

// proxmoxGuestStartSessionLister lists the tenant's collector command sessions
// with what each reported about starting guests. It is optional: a commander
// without it routes guest actions through action runners only.
type proxmoxGuestStartSessionLister interface {
	GetProxmoxGuestStartSessionsForOrganization(organizationID string) []agentexec.ConnectedAgent
}

func newProxmoxGuestActionExecutor(resources *ResourceHandlers, agents actionAgentCommander, observer proxmoxGuestPostconditionObserver) ActionExecutor {
	if resources == nil || agents == nil {
		return nil
	}
	return proxmoxGuestActionExecutor{resources: resources, agents: agents, observer: observer}
}

func (e proxmoxGuestActionExecutor) BindActionDispatch(ctx context.Context, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (unified.ActionDispatchAttempt, error) {
	record, err := unified.NormalizeActionAuditRecord(record)
	if err != nil {
		return unified.ActionDispatchAttempt{}, err
	}
	resource, kind, err := e.currentProxmoxGuestResource(ctx, record.Request.ResourceID, record.Request.CapabilityName)
	if err != nil {
		return unified.ActionDispatchAttempt{}, err
	}
	agentID, err := e.proxmoxGuestActionAgentID(ctx, resource, record.Request.CapabilityName)
	if err != nil {
		return unified.ActionDispatchAttempt{}, err
	}
	request, err := proxmoxGuestLifecycleRequest(attempt.ID, record.ID, string(kind), record.Request.CapabilityName, resource)
	if err != nil {
		return unified.ActionDispatchAttempt{}, err
	}
	return unified.BindActionDispatchAttempt(attempt, unified.ActionDispatchBinding{OperationKind: request.Operation, OperationVersion: request.OperationVersion, RequestDigest: request.RequestDigest, AgentID: agentID})
}

func (e proxmoxGuestActionExecutor) ActionHandlerNames() []string {
	return []string{proxmoxVMLifecycleHandler, proxmoxCTLifecycleHandler}
}

func (e proxmoxGuestActionExecutor) ActionDispatchOperationKinds() []string {
	return []string{"start", "stop", "shutdown", "reboot"}
}

func (e proxmoxGuestActionExecutor) ExecuteAction(ctx context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	attempt, ok := actionlifecycle.DispatchAttemptFromContext(ctx)
	if !ok || attempt.ActionID != record.ID {
		return nil, fmt.Errorf("committed action dispatch authority is required")
	}
	record, err := unified.NormalizeActionAuditRecord(record)
	if err != nil {
		return nil, err
	}
	operation := strings.TrimSpace(record.Request.CapabilityName)
	if !isProxmoxGuestLifecycleOperation(operation) {
		return nil, fmt.Errorf("unsupported proxmox guest lifecycle capability %q", operation)
	}

	resource, kind, err := e.currentProxmoxGuestResource(ctx, record.Request.ResourceID, operation)
	if err != nil {
		return nil, err
	}
	vmid := resource.Proxmox.VMID
	route := e.proxmoxGuestActionRoute(ctx, resource, operation)
	if route.agentID == "" {
		return nil, fmt.Errorf("Proxmox guest %s is not available: %s", operation, route.reason)
	}
	agentID := route.agentID
	var independentBefore *proxmoxGuestPostconditionObservation
	if e.observer != nil {
		if observation, observeErr := e.observer.ObserveProxmoxGuest(ctx, record.Request.ResourceID, resource.Proxmox.Instance, resource.Proxmox.NodeName, vmid, kind); observeErr == nil && validProxmoxGuestObservation(resource, kind, observation) {
			independentBefore = &observation
		}
	}

	actionStartedAt := time.Now().UTC()
	typedAgents := e.agents.(proxmoxGuestLifecycleAgentCommander)
	request, err := proxmoxGuestLifecycleRequest(attempt.ID, record.ID, string(kind), operation, resource)
	if err != nil {
		return nil, err
	}
	if attempt.OperationKind != "" && agentexec.ProxmoxGuestLifecycleOperationIdentity(agentID, request) != (operationreceipt.Identity{AttemptID: attempt.ID, ActionID: attempt.ActionID, OperationKind: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, AgentID: attempt.AgentID}) {
		return nil, fmt.Errorf("Proxmox guest lifecycle dispatch binding drift")
	}
	// The session that carries the request is looked up again at dispatch. A
	// route planned on the runner must still be a runner there: the result is
	// judged by the runner's rules below, and a collector session that took its
	// place under the same agent ID has to be refused, not believed.
	dispatchCtx := agentCommandContext(ctx)
	if route.runner {
		dispatchCtx = agentexec.RequireActionRunnerSession(dispatchCtx)
	}
	result, err := typedAgents.ExecuteProxmoxGuestLifecycle(dispatchCtx, agentID, request)
	agentReceivedAt := time.Now().UTC()
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("typed Proxmox guest lifecycle returned no result")
	}
	if err := agentexec.ValidateProxmoxGuestLifecycleResultForRequest(request, *result); err != nil {
		return nil, fmt.Errorf("invalid typed Proxmox guest lifecycle result: %w", err)
	}
	succeeded := result.MutationCompleted
	exitCode := 0
	if !succeeded {
		exitCode = 1
	}
	output := ""
	if result.ReadbackRan {
		output = "status: " + result.After.Status
	}
	agentVerification := &unified.ActionVerificationResult{Ran: false}
	// Completing the mutation and observing its postcondition are separate
	// facts. A status-only read cannot establish that a reboot occurred.
	if succeeded && result.ReadbackRan && operation != "reboot" {
		evaluation, _ := agentexec.EvaluateCapabilityPostcondition(proxmoxPostconditionCapability(kind, operation), nil,
			proxmoxGuestPostconditionValues(kind, result.After.Status, 0), actionStartedAt)
		agentVerification = &unified.ActionVerificationResult{
			Ran: evaluation.Conclusive, Command: proxmoxGuestStatusCommand(kind, vmid), Output: output,
			Success: evaluation.Matched, RanAt: result.After.ObservedAt, Note: result.Error,
		}
	}
	independentAfter, independentEvaluation := e.observeProxmoxGuestPostcondition(ctx, record.Request.ResourceID, resource, kind, operation, independentBefore, actionStartedAt)
	// A start a collector's command session ran is confirmed by Pulse's own
	// Proxmox check and by nothing else. The agent that ran it holds a general
	// credential, so its word that the guest is running is evidence, not proof.
	return proxmoxGuestExecutionResult(record.ID, record.Request.ResourceID, agentID, kind, operation, exitCode, output, result.Error, agentVerification, independentBefore, independentAfter, independentEvaluation, actionStartedAt, agentReceivedAt, !route.runner)
}

func proxmoxGuestLifecycleRequest(attemptID, actionID, kind, operation string, resource unified.Resource) (agentexec.ProxmoxGuestLifecyclePayload, error) {
	expectedStatus := ""
	if resource.Proxmox != nil {
		expectedStatus = strings.ToLower(strings.TrimSpace(resource.Proxmox.RuntimeStatus))
	}
	if expectedStatus == "" {
		switch resource.Status {
		case unified.StatusOnline:
			expectedStatus = "running"
		case unified.StatusOffline:
			expectedStatus = "stopped"
		}
	}
	if expectedStatus != "running" && expectedStatus != "stopped" {
		return agentexec.ProxmoxGuestLifecyclePayload{}, fmt.Errorf("Proxmox guest lifecycle requires a current running or stopped state")
	}
	request := agentexec.ProxmoxGuestLifecyclePayload{
		RequestID: attemptID, ActionID: actionID, Operation: operation, GuestKind: kind,
		VMID: resource.Proxmox.VMID, ExpectedStatus: expectedStatus, Timeout: proxmoxGuestLifecycleTimeout(operation),
	}
	if err := agentexec.BindProxmoxGuestLifecyclePayload(&request); err != nil {
		return agentexec.ProxmoxGuestLifecyclePayload{}, err
	}
	return request, nil
}

func (e proxmoxGuestActionExecutor) CheckActionAvailable(ctx context.Context, req unified.ActionRequest, resource unified.Resource) unified.ResourceActionReadiness {
	operation := strings.TrimSpace(req.CapabilityName)
	if _, ok := findProxmoxLifecycleCapability(resource.Capabilities, operation); !ok {
		return unified.ResourceActionReadiness{}
	}
	readiness := unified.ResourceActionReadiness{Name: operation, Available: true}
	if e.agents == nil {
		return unavailableProxmoxActionReadiness(operation, "command_agent_unavailable", "Proxmox command execution is not available.")
	}
	if _, _, err := e.executableProxmoxGuestResource(resource, operation); err != nil {
		return unavailableProxmoxActionReadiness(operation, proxmoxActionUnavailableReasonCode(err), proxmoxActionUnavailableReason(err))
	}
	if _, ok := e.agents.(proxmoxGuestLifecycleAgentCommander); !ok {
		return unavailableProxmoxActionReadiness(operation, "typed_operation_unavailable", "Typed Proxmox lifecycle execution is not available.")
	}
	route := e.proxmoxGuestActionRoute(ctx, resource, operation)
	if route.agentID == "" {
		// A refusal a person reads. It says what Pulse cannot do and what to
		// do now, in the same words the Patrol catalog and a stored run use.
		return unavailableProxmoxActionReadiness(operation, route.reasonCode, route.reason)
	}
	if liveAgentOperationReceiptVersion(ctx, e.agents, route.agentID) != operationreceipt.ProtocolVersion {
		if !route.runner {
			return unavailableProxmoxActionReadiness(operation, "operation_receipt_unsupported", proxmoxGuestStartReceiptsReason)
		}
		return unavailableProxmoxActionReadiness(operation, "operation_receipt_unsupported", "The Proxmox action runner cannot retain durable action receipts. Update the runner or restore its writable state directory, then retry.")
	}
	return readiness
}

const proxmoxGuestStartReceiptsReason = "The Pulse agent on this node cannot run reviewed actions: it is on an older version, or its durable state directory is unavailable. Update the agent, or check the agent logs if it is already current, then retry."

// proxmoxGuestActionRoute is where one guest action would run, or the refusal
// a person reads when it cannot.
type proxmoxGuestActionRoute struct {
	agentID    string
	runner     bool
	reasonCode string
	reason     string
}

// proxmoxGuestActionRoute decides which session on the guest's node carries an
// action. The node's action runner carries all four. Without one, the node's
// own Pulse agent carries a start when it reported that it can, and nothing
// else: stop, shutdown and reboot are refused here and again at dispatch.
func (e proxmoxGuestActionExecutor) proxmoxGuestActionRoute(ctx context.Context, resource unified.Resource, operation string) proxmoxGuestActionRoute {
	if agentID, err := e.connectedProxmoxNodeActionRunnerID(ctx, resource); err == nil {
		return proxmoxGuestActionRoute{agentID: agentID, runner: true}
	}
	node := proxmoxGuestNodeLabel(resource)
	if strings.TrimSpace(operation) != "start" {
		return proxmoxGuestActionRoute{reasonCode: actionlifecycle.ReasonCodeActionRunnerUnavailable, reason: actionlifecycle.ProxmoxGuestRunnerOnlyReason(node)}
	}
	nodeAgent := e.proxmoxGuestNodeAgent(ctx, resource)
	switch nodeAgent.link {
	case tools.GuestNodeAgentConfirmed:
	case tools.GuestNodeAgentUnconfirmed:
		// Said before anything about the agent's settings. Telling a person
		// to switch commands on for an agent that may sit on another machine
		// would open that machine and still leave this start refused.
		return proxmoxGuestActionRoute{reasonCode: actionlifecycle.ReasonCodeGuestStartAgentUnconfirmed, reason: actionlifecycle.ProxmoxGuestStartAgentUnconfirmedReason(node)}
	default:
		return proxmoxGuestActionRoute{reasonCode: actionlifecycle.ReasonCodeGuestStartAgentNotConnected, reason: actionlifecycle.ProxmoxGuestStartAgentNotConnectedReason(node)}
	}
	// The node's agent says commands are off. That wins over a session that
	// has not closed yet, and it is a setting with its own step, not a fault.
	if nodeAgent.commandsDisabled {
		return proxmoxGuestActionRoute{reasonCode: actionlifecycle.ReasonCodeHostCommandsDisabled, reason: actionlifecycle.HostCommandsDisabledRefusal(firstNonEmpty(nodeAgent.hostName, node))}
	}
	switch e.proxmoxGuestStartSessionState(ctx, nodeAgent.agentID) {
	case proxmoxGuestStartReady:
		return proxmoxGuestActionRoute{agentID: nodeAgent.agentID}
	case proxmoxGuestStartNoReceipts:
		return proxmoxGuestActionRoute{reasonCode: "operation_receipt_unsupported", reason: proxmoxGuestStartReceiptsReason}
	case proxmoxGuestStartAgentTooOld:
		return proxmoxGuestActionRoute{reasonCode: actionlifecycle.ReasonCodeGuestStartAgentUpdateRequired, reason: actionlifecycle.ProxmoxGuestStartAgentUpdateReason(node)}
	case proxmoxGuestStartNotSystemd:
		return proxmoxGuestActionRoute{reasonCode: actionlifecycle.ReasonCodeGuestStartUnavailable, reason: actionlifecycle.ProxmoxGuestStartNotSystemdReason(node)}
	case proxmoxGuestStartNoContainment:
		return proxmoxGuestActionRoute{reasonCode: actionlifecycle.ReasonCodeGuestStartUnavailable, reason: actionlifecycle.ProxmoxGuestStartContainmentReason(node)}
	default:
		return proxmoxGuestActionRoute{reasonCode: actionlifecycle.ReasonCodeGuestStartAgentNotConnected, reason: actionlifecycle.ProxmoxGuestStartAgentNotConnectedReason(node)}
	}
}

// proxmoxGuestActionAgentID is the route for binding and dispatch, where a
// refusal is an error.
func (e proxmoxGuestActionExecutor) proxmoxGuestActionAgentID(ctx context.Context, resource unified.Resource, operation string) (string, error) {
	route := e.proxmoxGuestActionRoute(ctx, resource, operation)
	if route.agentID == "" {
		return "", fmt.Errorf("Proxmox guest %s is not available: %s", strings.TrimSpace(operation), route.reason)
	}
	return route.agentID, nil
}

// proxmoxGuestStartState is what the command session on a guest's node said
// about starting guests.
type proxmoxGuestStartState int

const (
	proxmoxGuestStartNoSession proxmoxGuestStartState = iota
	proxmoxGuestStartReady
	proxmoxGuestStartNoReceipts
	proxmoxGuestStartAgentTooOld
	proxmoxGuestStartNotSystemd
	proxmoxGuestStartNoContainment
)

// agentLookupGuestStartSession memoizes one agent's answer for an availability
// pass. It sits apart from the session lookups in action_executor.go, which it
// does not use. It is keyed by the agent's ID, never by a node name: which
// agent belongs to a guest is read from that guest's own record every time.
const agentLookupGuestStartSession agentLookupKind = 101

// proxmoxGuestNodeAgent is the agent Pulse confirmed on a guest's node
// (tools.GuestNodeAgentFor), with what that agent's own report says about
// command access. agentID, hostName and commandsDisabled are set only for a
// confirmed agent.
type proxmoxGuestNodeAgent struct {
	link             tools.GuestNodeAgentLink
	agentID          string
	hostName         string
	commandsDisabled bool
}

// proxmoxGuestNodeAgent resolves the agent for one guest from that guest's own
// record, and how sure Pulse is that it runs on the guest's node. A hostname
// never selects it: another machine can hold the node's name, and a
// collector's credential is not bound to one host by an admin the way a
// runner's is. An agent inside the guest is a different machine and is never
// the answer. Nothing is cached across guests.
func (e proxmoxGuestActionExecutor) proxmoxGuestNodeAgent(ctx context.Context, resource unified.Resource) proxmoxGuestNodeAgent {
	if e.resources == nil || resource.Proxmox == nil {
		return proxmoxGuestNodeAgent{}
	}
	registry, err := e.resources.buildRegistry(GetOrgID(ctx))
	if err != nil || registry == nil {
		return proxmoxGuestNodeAgent{}
	}
	found := tools.GuestNodeAgentFor(registry, resource.ID)
	nodeAgent := proxmoxGuestNodeAgent{link: found.Link}
	if found.Link != tools.GuestNodeAgentConfirmed || found.AgentID == "" {
		if found.Link == tools.GuestNodeAgentConfirmed {
			nodeAgent.link = tools.GuestNodeAgentUnconfirmed
		}
		return nodeAgent
	}
	nodeAgent.agentID = found.AgentID
	for _, host := range registry.Hosts() {
		if host == nil || host.AgentID() != nodeAgent.agentID {
			continue
		}
		nodeAgent.hostName = firstNonEmpty(host.Name(), host.Hostname())
		if hostResource, ok := registry.Get(host.ID()); ok {
			nodeAgent.commandsDisabled = actionlifecycle.HostCommandsDisabled(hostResource)
		}
		break
	}
	return nodeAgent
}

// proxmoxGuestStartSessionState reads what the collector command session of
// one agent reported about starting guests. Nothing here is inferred from an
// agent version.
func (e proxmoxGuestActionExecutor) proxmoxGuestStartSessionState(ctx context.Context, agentID string) proxmoxGuestStartState {
	lister, ok := e.agents.(proxmoxGuestStartSessionLister)
	if !ok || strings.TrimSpace(agentID) == "" {
		return proxmoxGuestStartNoSession
	}
	return proxmoxGuestStartState(memoizedAgentLookup(ctx, agentLookupKey{kind: agentLookupGuestStartSession, first: agentID}, func() agentLookupResult {
		for _, agent := range lister.GetProxmoxGuestStartSessionsForOrganization(GetOrgID(ctx)) {
			if agent.AgentID != agentID {
				continue
			}
			state := proxmoxGuestStartAgentTooOld
			switch {
			case agent.ProxmoxGuestStartVersion == agentexec.ProxmoxGuestStartProtocolVersion:
				state = proxmoxGuestStartReady
			case agent.ProxmoxGuestStartUnavailable == agentexec.ProxmoxGuestStartUnavailableNotSystemdService:
				state = proxmoxGuestStartNotSystemd
			case agent.ProxmoxGuestStartUnavailable != "":
				state = proxmoxGuestStartNoContainment
			case agent.OperationReceiptVersion != operationreceipt.ProtocolVersion:
				state = proxmoxGuestStartNoReceipts
			}
			return agentLookupResult{agentID: agentID, ok: true, version: int(state)}
		}
		return agentLookupResult{version: int(proxmoxGuestStartNoSession)}
	}).version)
}

func (e proxmoxGuestActionExecutor) currentProxmoxGuestResource(ctx context.Context, resourceID, operation string) (unified.Resource, proxmoxGuestKind, error) {
	if e.resources == nil {
		return unified.Resource{}, "", fmt.Errorf("resource handler unavailable")
	}
	registry, err := e.resources.buildRegistry(GetOrgID(ctx))
	if err != nil {
		return unified.Resource{}, "", err
	}
	resource, ok := registry.Get(resourceID)
	if !ok || resource == nil {
		return unified.Resource{}, "", fmt.Errorf("resource %q is no longer present", resourceID)
	}
	return e.executableProxmoxGuestResource(*resource, operation)
}

func (e proxmoxGuestActionExecutor) executableProxmoxGuestResource(resource unified.Resource, operation string) (unified.Resource, proxmoxGuestKind, error) {
	kind, handler, err := proxmoxGuestKindAndHandler(resource)
	if err != nil {
		return unified.Resource{}, "", err
	}
	if resource.Proxmox == nil {
		return unified.Resource{}, "", fmt.Errorf("proxmox resource metadata missing")
	}
	if resource.Proxmox.VMID <= 0 {
		return unified.Resource{}, "", fmt.Errorf("proxmox guest resource %q has no executable vmid", resource.ID)
	}
	if strings.TrimSpace(resource.Proxmox.NodeName) == "" {
		return unified.Resource{}, "", fmt.Errorf("proxmox guest resource %q has no owning node", resource.ID)
	}
	if resource.Proxmox.Template {
		return unified.Resource{}, "", fmt.Errorf("proxmox guest resource %q is a template", resource.ID)
	}
	if lock := strings.TrimSpace(resource.Proxmox.Lock); lock != "" {
		return unified.Resource{}, "", fmt.Errorf("proxmox guest resource %q is locked: %s", resource.ID, lock)
	}
	if status := resource.SourceStatus[unified.SourceProxmox]; strings.EqualFold(strings.TrimSpace(status.Status), "stale") || strings.EqualFold(strings.TrimSpace(status.Status), "offline") || strings.EqualFold(strings.TrimSpace(status.Status), "missing") {
		return unified.Resource{}, "", fmt.Errorf("proxmox inventory for resource %q is %s", resource.ID, status.Status)
	}
	capability, ok := findProxmoxLifecycleCapability(resource.Capabilities, operation)
	if !ok {
		return unified.Resource{}, "", fmt.Errorf("resource %q does not currently advertise %s capability", resource.ID, operation)
	}
	if capability.InternalHandler != handler {
		return unified.Resource{}, "", fmt.Errorf("resource %q advertises %s through unsupported handler %q", resource.ID, operation, capability.InternalHandler)
	}
	return resource, kind, nil
}

// connectedProxmoxNodeActionRunnerID finds the typed runner on the machine hosting
// this guest. Node names and VMIDs repeat across connections, so a runner that only
// shares the node's name could change a different guest with the same VMID.
func (e proxmoxGuestActionExecutor) connectedProxmoxNodeActionRunnerID(ctx context.Context, resource unified.Resource) (string, error) {
	if resource.Proxmox == nil {
		return "", fmt.Errorf("proxmox resource metadata missing")
	}
	runners, ok := e.agents.(proxmoxGuestLifecycleAgentCommander)
	if !ok {
		return "", fmt.Errorf("typed Proxmox lifecycle execution is unavailable")
	}
	runner := memoizedAgentLookup(ctx, agentLookupKey{kind: agentLookupActionRunner, first: resource.Proxmox.NodeName, second: resource.Proxmox.Instance}, func() agentLookupResult {
		if e.resources == nil {
			return agentLookupResult{}
		}
		registry, err := e.resources.buildRegistry(GetOrgID(ctx))
		if err != nil {
			return agentLookupResult{}
		}
		agent, connected := tools.ConnectedAgentForGuestNode(registry, resource.ID, resource.Proxmox.Instance, resource.Proxmox.NodeName, runners.GetActionRunnersForOrganization(GetOrgID(ctx)))
		return agentLookupResult{agentID: agent.AgentID, ok: connected}
	})
	if runner.ok && strings.TrimSpace(runner.agentID) != "" {
		return runner.agentID, nil
	}
	return "", fmt.Errorf("a typed action runner is not connected on the Proxmox node")
}

// proxmoxGuestNodeLabel is the node name a person recognises: the display
// name when Pulse has one, else the name Proxmox gives the node.
func proxmoxGuestNodeLabel(resource unified.Resource) string {
	if resource.Proxmox == nil {
		return ""
	}
	if label := strings.TrimSpace(resource.Proxmox.NodeDisplayName); label != "" {
		return label
	}
	return strings.TrimSpace(resource.Proxmox.NodeName)
}

func proxmoxGuestKindAndHandler(resource unified.Resource) (proxmoxGuestKind, string, error) {
	switch resource.Type {
	case unified.ResourceTypeVM:
		return proxmoxGuestVM, proxmoxVMLifecycleHandler, nil
	case unified.ResourceTypeSystemContainer:
		return proxmoxGuestCT, proxmoxCTLifecycleHandler, nil
	default:
		return "", "", fmt.Errorf("resource %q is not a Proxmox VM or LXC", resource.ID)
	}
}

func unavailableProxmoxActionReadiness(operation, reasonCode, reason string) unified.ResourceActionReadiness {
	return unified.ResourceActionReadiness{
		Name:       strings.TrimSpace(operation),
		Available:  false,
		ReasonCode: strings.TrimSpace(reasonCode),
		Reason:     strings.TrimSpace(reason),
	}
}

func proxmoxActionUnavailableReasonCode(err error) string {
	if err == nil {
		return "unavailable"
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "not a proxmox vm or lxc"):
		return "unsupported_resource"
	case strings.Contains(message, "inventory") && (strings.Contains(message, "stale") || strings.Contains(message, "offline") || strings.Contains(message, "missing")):
		return "stale_inventory"
	case strings.Contains(message, "template"):
		return "template"
	case strings.Contains(message, "locked"):
		return "guest_locked"
	case strings.Contains(message, "does not currently advertise"):
		return "capability_unavailable"
	case strings.Contains(message, "unsupported handler"):
		return "unsupported_handler"
	case strings.Contains(message, "vmid") || strings.Contains(message, "owning node"):
		return "incomplete_metadata"
	default:
		return "unavailable"
	}
}

func proxmoxActionUnavailableReason(err error) string {
	switch proxmoxActionUnavailableReasonCode(err) {
	case "unsupported_resource":
		return "Resource is not a Proxmox VM or LXC."
	case "stale_inventory":
		return "Proxmox inventory is not fresh enough to run lifecycle actions."
	case "template":
		return "Proxmox templates cannot run lifecycle actions."
	case "guest_locked":
		return "Proxmox reports this guest is locked."
	case "capability_unavailable":
		return "Pulse does not currently advertise a fresh Proxmox lifecycle capability for this guest."
	case "unsupported_handler":
		return "This Proxmox action is not routed through the supported lifecycle executor."
	case "incomplete_metadata":
		return "Proxmox guest metadata is missing the VMID or owning node needed for command execution."
	default:
		return "Proxmox lifecycle action is not currently available."
	}
}

func findProxmoxLifecycleCapability(capabilities []unified.ResourceCapability, operation string) (unified.ResourceCapability, bool) {
	operation = strings.TrimSpace(operation)
	for _, capability := range capabilities {
		if strings.TrimSpace(capability.Name) == operation && isProxmoxLifecycleHandler(capability.InternalHandler) {
			return capability, true
		}
	}
	return unified.ResourceCapability{}, false
}

func isProxmoxLifecycleHandler(handler string) bool {
	switch strings.TrimSpace(handler) {
	case proxmoxVMLifecycleHandler, proxmoxCTLifecycleHandler:
		return true
	default:
		return false
	}
}

func isProxmoxGuestLifecycleOperation(operation string) bool {
	switch strings.TrimSpace(operation) {
	case "start", "shutdown", "reboot", "stop":
		return true
	default:
		return false
	}
}

func proxmoxGuestStatusCommand(kind proxmoxGuestKind, vmid int) string {
	tool := "qm"
	if kind == proxmoxGuestCT {
		tool = "pct"
	}
	return strings.Join([]string{tool, "status", strconv.Itoa(vmid)}, " ")
}

func proxmoxGuestLifecycleTimeout(operation string) int {
	switch strings.TrimSpace(operation) {
	case "shutdown", "reboot":
		return 180
	default:
		return 120
	}
}

func (e proxmoxGuestActionExecutor) observeProxmoxGuestPostcondition(ctx context.Context, resourceID string, resource unified.Resource, kind proxmoxGuestKind, operation string, before *proxmoxGuestPostconditionObservation, actionStartedAt time.Time) (*proxmoxGuestPostconditionObservation, agentexec.PostconditionEvaluation) {
	if e.observer == nil || resource.Proxmox == nil {
		return nil, agentexec.PostconditionEvaluation{}
	}
	capability := proxmoxPostconditionCapability(kind, operation)
	postcondition, ok := agentexec.LookupCapabilityPostcondition(capability)
	if !ok {
		return nil, agentexec.PostconditionEvaluation{ReasonCode: "postcondition_unregistered"}
	}
	window := postcondition.Window
	if window <= 0 {
		window = 2 * time.Minute
	}
	verifyCtx, cancel := context.WithTimeout(ctx, window)
	defer cancel()

	var beforeValues map[agentexec.PostconditionField]string
	if before != nil {
		beforeValues = proxmoxGuestPostconditionValues(kind, before.Snapshot.Status, before.Snapshot.Uptime)
	}
	var last *proxmoxGuestPostconditionObservation
	lastEvaluation := agentexec.PostconditionEvaluation{ReasonCode: "independent_observation_unavailable"}
	for {
		observation, err := e.observer.ObserveProxmoxGuest(verifyCtx, resourceID, resource.Proxmox.Instance, resource.Proxmox.NodeName, resource.Proxmox.VMID, kind)
		if err == nil && validProxmoxGuestObservation(resource, kind, observation) {
			last = &observation
			lastEvaluation, _ = agentexec.EvaluateCapabilityPostcondition(capability, beforeValues, proxmoxGuestPostconditionValues(kind, observation.Snapshot.Status, observation.Snapshot.Uptime), actionStartedAt)
			if lastEvaluation.Conclusive && lastEvaluation.Matched {
				return last, lastEvaluation
			}
		}

		timer := time.NewTimer(time.Second)
		select {
		case <-verifyCtx.Done():
			timer.Stop()
			return last, lastEvaluation
		case <-timer.C:
		}
	}
}

func proxmoxPostconditionCapability(kind proxmoxGuestKind, operation string) string {
	tool := "qm"
	if kind == proxmoxGuestCT {
		tool = "pct"
	}
	return tool + "." + strings.ToLower(strings.TrimSpace(operation))
}

func proxmoxGuestPostconditionValues(kind proxmoxGuestKind, status string, uptime uint64) map[agentexec.PostconditionField]string {
	statusField := agentexec.FieldVMStatus
	if kind == proxmoxGuestCT {
		statusField = agentexec.FieldContainerStatus
	}
	return map[agentexec.PostconditionField]string{
		statusField:                strings.ToLower(strings.TrimSpace(status)),
		agentexec.FieldGuestUptime: strconv.FormatUint(uptime, 10),
	}
}

func validProxmoxGuestObservation(resource unified.Resource, kind proxmoxGuestKind, observation proxmoxGuestPostconditionObservation) bool {
	if resource.Proxmox == nil || observation.Snapshot.VMID != resource.Proxmox.VMID || observation.Snapshot.Kind != kind {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(observation.Snapshot.Instance), strings.TrimSpace(resource.Proxmox.Instance)) &&
		strings.EqualFold(strings.TrimSpace(observation.Snapshot.Node), strings.TrimSpace(resource.Proxmox.NodeName)) &&
		strings.TrimSpace(observation.Snapshot.Status) != "" && !observation.Snapshot.ObservedAt.IsZero() && !observation.ReceivedAt.IsZero()
}

func parseProxmoxGuestStatus(output string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(output)))
	for i, field := range fields {
		field = strings.TrimSuffix(field, ":")
		if field == "status" && i+1 < len(fields) {
			return strings.Trim(strings.TrimSuffix(fields[i+1], ":"), " \t\r\n")
		}
		if field == "running" || field == "stopped" {
			return field
		}
	}
	return ""
}
