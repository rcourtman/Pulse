package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	"github.com/rs/zerolog"
)

// collectorGuestRun is a Proxmox node as the collector's typed containment
// sees it: one container that is stopped until it is started.
type collectorGuestRun struct {
	mu      sync.Mutex
	running bool
	calls   []string
	units   []string
}

func (r *collectorGuestRun) run(_ context.Context, supervisor typedActionSupervisor, env []string, catalog typedActionCatalog, name string, args ...string) typedActionCommandResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	if supervisor.allow == nil {
		return typedActionCommandResult{exitCode: -1, err: errors.New("the collector ran a command without its own catalog")}
	}
	if err := supervisor.allow(catalog, name, args); err != nil {
		return typedActionCommandResult{exitCode: -1, err: err}
	}
	if len(env) != 0 {
		return typedActionCommandResult{exitCode: -1, err: errors.New("unexpected environment")}
	}
	r.calls = append(r.calls, strings.Join(append([]string{string(catalog), name}, args...), " "))
	r.units = append(r.units, supervisor.serviceUnit+"|"+supervisor.unitPrefix)
	switch args[0] {
	case "status":
		if r.running {
			return typedActionCommandResult{stdout: "status: running\n"}
		}
		return typedActionCommandResult{stdout: "status: stopped\n"}
	case "start":
		r.running = true
		return typedActionCommandResult{}
	}
	return typedActionCommandResult{exitCode: -1, err: errors.New("unexpected verb")}
}

func (r *collectorGuestRun) seen() ([]string, []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...), append([]string(nil), r.units...)
}

func collectorGuestRequest(t *testing.T, operation, expectedStatus string) agentexec.ProxmoxGuestLifecyclePayload {
	t.Helper()
	request := agentexec.ProxmoxGuestLifecyclePayload{
		RequestID: "pve.collector." + operation, ActionID: "pve.collector.action." + operation, Operation: operation,
		GuestKind: "ct", VMID: 112, ExpectedStatus: expectedStatus, Timeout: 30,
	}
	if err := agentexec.BindProxmoxGuestLifecyclePayload(&request); err != nil {
		t.Fatal(err)
	}
	return request
}

// startCollectorSession runs a real command server and a real collector
// command client against it. prepare stands in for the systemd checks.
func startCollectorSession(t *testing.T, node *collectorGuestRun, pingInterval time.Duration, prepare func(context.Context, typedActionSupervisor) (string, error)) (*agentexec.Server, *CommandClient, agentexec.AgentAdmission) {
	t.Helper()
	admission := agentexec.AgentAdmission{
		TokenID: "collector-token", AgentID: "agent-pve", Hostname: "pve.example.test",
		RuntimeRole: agentexec.RuntimeRoleLegacyFullTrust,
	}
	server := agentexec.NewServerWithAdmissionValidator(func(token, _, _ string) (agentexec.AgentAdmission, bool) {
		return admission, token == admission.TokenID
	}, func(agentexec.AgentAdmission) bool { return true })
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	t.Cleanup(httpServer.Close)

	logs := &lockedLogBuffer{}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("collector log:\n%s", logs.String())
		}
	})
	logger := zerolog.New(logs).With().Timestamp().Logger()
	client := NewCommandClient(Config{
		PulseURL: httpServer.URL, APIToken: admission.TokenID, StateDir: t.TempDir(),
		EnableCommands: true, Logger: &logger,
	}, admission.AgentID, admission.Hostname, "linux", "test")
	if client.guestStart == nil {
		t.Fatal("a collector with commands enabled has no typed containment")
	}
	client.guestStart.prepare = prepare
	client.guestStart.run = node.run
	client.pingInterval = pingInterval

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- client.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("the collector command client did not stop")
		}
	})
	deadline := time.Now().Add(3 * time.Second)
	for !server.IsAgentConnected(admission.AgentID) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !server.IsAgentConnected(admission.AgentID) {
		t.Fatal("the collector command session did not connect")
	}
	return server, client, admission
}

// A collector with commands enabled tells Pulse it can start a guest, and
// then starts one through its own containment with a durable receipt. Pulse
// refuses to send it a stop, a shutdown or a reboot.
func TestRealServerCollectorStartsAStoppedGuestAndIsSentNothingElse(t *testing.T) {
	node := &collectorGuestRun{}
	prepared := 0
	server, _, admission := startCollectorSession(t, node, 0, func(_ context.Context, supervisor typedActionSupervisor) (string, error) {
		prepared++
		if supervisor.serviceUnit != typedActionCollectorServiceUnit || supervisor.unitPrefix != typedActionCollectorUnitPrefix {
			t.Errorf("containment prepared for %q with prefix %q", supervisor.serviceUnit, supervisor.unitPrefix)
		}
		return "", nil
	})

	sessions := server.GetProxmoxGuestStartSessionsForOrganization("default")
	if len(sessions) != 1 || sessions[0].AgentID != admission.AgentID || sessions[0].ProxmoxGuestStartVersion != agentexec.ProxmoxGuestStartProtocolVersion || sessions[0].ProxmoxGuestStartUnavailable != "" {
		t.Fatalf("sessions = %#v, want the collector reporting guest start", sessions)
	}
	if runners := server.GetActionRunnersForOrganization("default"); len(runners) != 0 {
		t.Fatalf("the collector was listed as an action runner: %#v", runners)
	}

	for _, verb := range []string{"stop", "shutdown", "reboot"} {
		if _, err := server.ExecuteProxmoxGuestLifecycle(context.Background(), admission.AgentID, collectorGuestRequest(t, verb, "running")); err == nil || !strings.Contains(err.Error(), "requires a typed action-runner session") {
			t.Fatalf("%s: %v, want the server to refuse it", verb, err)
		}
	}
	if calls, _ := node.seen(); len(calls) != 0 {
		t.Fatalf("a refused request reached the node: %v", calls)
	}

	request := collectorGuestRequest(t, "start", "stopped")
	result, err := server.ExecuteProxmoxGuestLifecycle(context.Background(), admission.AgentID, request)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !result.MutationStarted || !result.MutationCompleted || !result.ReadbackRan || result.Before.Status != "stopped" || result.After.Status != "running" || result.ExecutionPhase != agentexec.ProxmoxGuestPhaseComplete {
		t.Fatalf("start result = %+v", result)
	}
	calls, units := node.seen()
	if got, want := strings.Join(calls, "; "), "proxmox pct status 112; proxmox-handoff pct start 112; proxmox pct status 112"; got != want {
		t.Fatalf("node commands = %q, want %q", got, want)
	}
	for _, unit := range units {
		if unit != typedActionCollectorServiceUnit+"|"+typedActionCollectorUnitPrefix {
			t.Fatalf("a command ran under %q, want the collector's own service and prefix", unit)
		}
	}

	// The receipt is durable: a replay answers from it without a second start.
	replayed, err := server.ExecuteProxmoxGuestLifecycle(context.Background(), admission.AgentID, request)
	if err != nil || !replayed.MutationCompleted {
		t.Fatalf("replay = %+v, %v", replayed, err)
	}
	if calls, _ := node.seen(); len(calls) != 3 {
		t.Fatalf("the replay ran the node again: %v", calls)
	}
	query, err := server.QueryAgentOperation(context.Background(), admission.AgentID, agentexec.ProxmoxGuestLifecycleOperationIdentity(admission.AgentID, request))
	if err != nil || query.Status != operationreceipt.QueryFoundTerminal {
		t.Fatalf("receipt query = %+v, %v", query, err)
	}
	if prepared != 1 {
		t.Fatalf("containment was prepared %d times, want once per process", prepared)
	}
}

// A collector that cannot set up containment says why and is refused a start.
func TestRealServerCollectorWithoutContainmentReportsWhyAndIsRefused(t *testing.T) {
	node := &collectorGuestRun{}
	server, client, admission := startCollectorSession(t, node, 0, func(context.Context, typedActionSupervisor) (string, error) {
		return agentexec.ProxmoxGuestStartUnavailableNotSystemdService, errors.New("not running as pulse-agent.service")
	})
	sessions := server.GetProxmoxGuestStartSessionsForOrganization("default")
	if len(sessions) != 1 || sessions[0].ProxmoxGuestStartVersion != 0 || sessions[0].ProxmoxGuestStartUnavailable != agentexec.ProxmoxGuestStartUnavailableNotSystemdService {
		t.Fatalf("sessions = %#v, want the reason and no version", sessions)
	}
	if _, err := server.ExecuteProxmoxGuestLifecycle(context.Background(), admission.AgentID, collectorGuestRequest(t, "start", "stopped")); err == nil || !strings.Contains(err.Error(), "requires a typed action-runner session") {
		t.Fatalf("start: %v, want the server to refuse it", err)
	}
	// And the agent refuses by itself, whatever it is sent.
	result := client.proxmoxGuestLifecycle.Apply(context.Background(), collectorGuestRequest(t, "start", "stopped"))
	if result.ReasonCode != agentexec.ActionRefusalCapabilityUnavailable || result.MutationStarted || !strings.Contains(result.Error, "containment is not ready") {
		t.Fatalf("agent result = %+v, want a refusal", result)
	}
	if calls, _ := node.seen(); len(calls) != 0 {
		t.Fatalf("the node ran %v", calls)
	}
}

// On a host that is not a Proxmox node the collector reports nothing at all.
func TestCollectorOnAHostThatIsNotAProxmoxNodeReportsNothing(t *testing.T) {
	node := &collectorGuestRun{}
	server, _, _ := startCollectorSession(t, node, 0, func(context.Context, typedActionSupervisor) (string, error) {
		return "", errCollectorTypedActionsNotApplicable
	})
	sessions := server.GetProxmoxGuestStartSessionsForOrganization("default")
	if len(sessions) != 1 || sessions[0].ProxmoxGuestStartVersion != 0 || sessions[0].ProxmoxGuestStartUnavailable != "" {
		t.Fatalf("sessions = %#v, want no version and no reason", sessions)
	}
}

func readyCollectorManager(t *testing.T, commandsEnabled bool, node *collectorGuestRun) *proxmoxGuestLifecycleManager {
	t.Helper()
	containment := newCollectorTypedActions(t.TempDir(), zerolog.Nop())
	containment.prepare = func(context.Context, typedActionSupervisor) (string, error) { return "", nil }
	containment.run = node.run
	if !containment.ensure(context.Background()) {
		t.Fatal("containment did not become ready")
	}
	return newCollectorProxmoxGuestLifecycleManager(commandsEnabled, containment)
}

// The agent refuses stop, shutdown and reboot on its own, with containment
// ready and commands enabled, before it reads or changes anything.
func TestCollectorAgentRefusesStopShutdownAndRebootItself(t *testing.T) {
	for _, verb := range []string{"stop", "shutdown", "reboot"} {
		t.Run(verb, func(t *testing.T) {
			node := &collectorGuestRun{running: true}
			result := readyCollectorManager(t, true, node).Apply(context.Background(), collectorGuestRequest(t, verb, "running"))
			if result.ReasonCode != agentexec.ActionRefusalCapabilityUnavailable || result.MutationStarted || result.MutationCompleted || !strings.Contains(result.Error, "starts guests only") {
				t.Fatalf("result = %+v, want a refusal", result)
			}
			if err := agentexec.ValidateProxmoxGuestLifecycleResultForRequest(collectorGuestRequest(t, verb, "running"), result); err != nil {
				t.Fatalf("the refusal is not a valid result: %v", err)
			}
			if calls, _ := node.seen(); len(calls) != 0 {
				t.Fatalf("the node ran %v", calls)
			}
		})
	}
}

// A commands-off collector carries the launcher in its binary and still
// refuses the typed message. Nothing it is sent starts a guest.
func TestCommandsOffCollectorRefusesTheTypedGuestMessage(t *testing.T) {
	logger := zerolog.Nop()
	client := NewCommandClient(Config{PulseURL: "http://127.0.0.1:1", APIToken: "token", StateDir: t.TempDir(), EnableCommands: false, Logger: &logger}, "agent-pve", "pve.example.test", "linux", "test")
	t.Cleanup(func() { _ = client.Close() })
	if client.guestStart != nil {
		t.Fatal("a commands-off collector built typed containment")
	}
	if version, unavailable := client.guestStartRegistration(); version != 0 || unavailable != "" {
		t.Fatalf("a commands-off collector reports guest start: %d %q", version, unavailable)
	}

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	serverConnections := make(chan *websocket.Conn, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConnections <- conn
		<-release
		_ = conn.Close()
	}))
	defer server.Close()
	defer close(release)
	remote, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	agentSide := <-serverConnections

	for _, verb := range []string{"start", "stop", "shutdown", "reboot"} {
		expected := "running"
		if verb == "start" {
			expected = "stopped"
		}
		request := collectorGuestRequest(t, verb, expected)
		client.handleProxmoxGuestLifecycle(context.Background(), agentSide, request)
		var message wsMessage
		if err := remote.ReadJSON(&message); err != nil {
			t.Fatal(err)
		}
		var result agentexec.ProxmoxGuestLifecycleResultPayload
		if message.Type != msgTypeProxmoxGuestLifecycleResult || json.Unmarshal(message.Payload, &result) != nil {
			t.Fatalf("%s: message = %+v", verb, message)
		}
		if result.ReasonCode != agentexec.ActionRefusalCapabilityUnavailable || result.MutationStarted || !strings.Contains(result.Error, "commands are not enabled") {
			t.Fatalf("%s: result = %+v, want a refusal for commands being off", verb, result)
		}
	}
}

// The action runner keeps the whole guest catalog and its own containment.
func TestActionRunnerClientKeepsItsOwnGuestLifecycleManager(t *testing.T) {
	logger := zerolog.Nop()
	stateDir := t.TempDir()
	client := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: "https://127.0.0.1:1", APIToken: "token", StateDir: stateDir,
		HealthPath: filepath.Join(stateDir, "health.json"), ActivationNonce: strings.Repeat("a", 32), Logger: &logger,
	}, "agent-pve", "pve.example.test", "test")
	t.Cleanup(func() { _ = client.Close() })
	if client.guestStart != nil || client.proxmoxGuestLifecycle == nil || client.proxmoxGuestLifecycle.refuse != nil {
		t.Fatalf("the action runner was given the collector's start-only manager")
	}
	if version, unavailable := client.guestStartRegistration(); version != 0 || unavailable != "" {
		t.Fatalf("the action runner reports collector guest start: %d %q", version, unavailable)
	}
}

// The collector's catalog is the probe, a status read and a start. Everything
// else in the closed catalog belongs to the action runner.
func TestCollectorTypedActionCatalogIsProbeStatusAndStart(t *testing.T) {
	type invocation struct {
		catalog typedActionCatalog
		name    string
		args    []string
	}
	for _, allowed := range []invocation{
		{typedActionCatalogProbe, "true", nil},
		{typedActionCatalogProxmox, "pct", []string{"status", "112"}},
		{typedActionCatalogProxmox, "qm", []string{"status", "160"}},
		{typedActionCatalogProxmoxHandoff, "pct", []string{"start", "112"}},
		{typedActionCatalogProxmoxHandoff, "qm", []string{"start", "160"}},
	} {
		if err := validateCollectorTypedActionInvocation(allowed.catalog, allowed.name, allowed.args); err != nil {
			t.Fatalf("%v refused: %v", allowed, err)
		}
	}
	for _, refused := range []invocation{
		{typedActionCatalogProxmox, "pct", []string{"stop", "112"}},
		{typedActionCatalogProxmox, "pct", []string{"shutdown", "112"}},
		{typedActionCatalogProxmox, "qm", []string{"stop", "160"}},
		{typedActionCatalogProxmox, "qm", []string{"shutdown", "160"}},
		{typedActionCatalogProxmox, "qm", []string{"reboot", "160"}},
		{typedActionCatalogProxmoxHandoff, "pct", []string{"reboot", "112"}},
		{typedActionCatalogProxmox, "pct", []string{"start", "112"}},
		{typedActionCatalogProxmoxHandoff, "pct", []string{"status", "112"}},
		{typedActionCatalogProxmoxHandoff, "pct", []string{"start", "112", "--skiplock"}},
		{typedActionCatalogProxmoxHandoff, "pct", []string{"start", "0"}},
		{typedActionCatalogPackage, "apt-get", []string{"update"}},
		{typedActionCatalogPackage, "apt-get", []string{"-s", "-o", "Debug::NoLocking=1", "upgrade"}},
		{typedActionCatalogPackage, "dpkg", []string{"--audit"}},
		{typedActionCatalogProbe, "sh", nil},
	} {
		if err := validateCollectorTypedActionInvocation(refused.catalog, refused.name, refused.args); err == nil {
			t.Fatalf("%v was allowed on a collector", refused)
		}
	}
}

// Containment is prepared once it passes and never again in that process:
// preparing stops this collector's stale units, which must not happen while
// one of them could be running a start.
func TestCollectorTypedActionsPrepareOnceAndRetryOnlyAfterFailure(t *testing.T) {
	containment := newCollectorTypedActions(t.TempDir(), zerolog.Nop())
	attempts := 0
	fail := true
	containment.prepare = func(context.Context, typedActionSupervisor) (string, error) {
		attempts++
		if fail {
			return "something a newer build says", errors.New("probe failed")
		}
		return "", nil
	}
	ran := 0
	containment.run = func(context.Context, typedActionSupervisor, []string, typedActionCatalog, string, ...string) typedActionCommandResult {
		ran++
		return typedActionCommandResult{}
	}
	if containment.ensure(context.Background()) || containment.isReady() {
		t.Fatal("containment is ready after a failed probe")
	}
	if version, unavailable := containment.registration(); version != 0 || unavailable != agentexec.ProxmoxGuestStartUnavailableContainment {
		t.Fatalf("registration = %d %q, want the closed containment reason", version, unavailable)
	}
	if result := containment.runCommand(context.Background(), nil, typedActionCatalogProbe, "true"); !errors.Is(result.err, errTypedActionContainmentUnavailable) || ran != 0 {
		t.Fatalf("a command ran before containment was ready: %+v", result)
	}
	fail = false
	if !containment.ensure(context.Background()) || !containment.ensure(context.Background()) {
		t.Fatal("containment did not become ready")
	}
	if attempts != 2 {
		t.Fatalf("prepare ran %d times, want one failure and one success", attempts)
	}
	if version, unavailable := containment.registration(); version != agentexec.ProxmoxGuestStartProtocolVersion || unavailable != "" {
		t.Fatalf("registration = %d %q", version, unavailable)
	}
	if result := containment.runCommand(context.Background(), nil, typedActionCatalogProbe, "true"); result.err != nil || ran != 1 {
		t.Fatalf("a ready containment did not run: %+v", result)
	}
}

// Containment waits on systemd without a limit of its own. A registration
// does not wait with it: the command session connects without guest start,
// and the next registration reports what the preparation found.
func TestCollectorRegistrationDoesNotWaitForASlowContainmentProbe(t *testing.T) {
	containment := newCollectorTypedActions(t.TempDir(), zerolog.Nop())
	containment.wait = 20 * time.Millisecond
	release := make(chan struct{})
	started := make(chan struct{}, 4)
	containment.prepare = func(context.Context, typedActionSupervisor) (string, error) {
		started <- struct{}{}
		<-release
		return "", nil
	}
	begun := time.Now()
	if containment.ensure(context.Background()) {
		t.Fatal("containment is ready while its probe is still running")
	}
	if waited := time.Since(begun); waited > 2*time.Second {
		t.Fatalf("the registration waited %s for the probe", waited)
	}
	if version, unavailable := containment.registration(); version != 0 || unavailable != agentexec.ProxmoxGuestStartUnavailableContainment {
		t.Fatalf("registration = %d %q while the probe runs", version, unavailable)
	}
	// A second registration does not start another probe and does not wait
	// for the running one at all.
	containment.wait = time.Hour
	begun = time.Now()
	if containment.ensure(context.Background()) {
		t.Fatal("containment is ready while its probe is still running")
	}
	if waited := time.Since(begun); waited > 2*time.Second {
		t.Fatalf("a later registration waited %s for a probe an earlier one already gave up on", waited)
	}
	close(release)
	deadline := time.Now().Add(3 * time.Second)
	for !containment.isReady() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !containment.ensure(context.Background()) {
		t.Fatal("containment did not become ready after its probe finished")
	}
	if len(started) != 1 {
		t.Fatalf("the probe started %d times, want once", len(started))
	}
}

// The agent replaces its command client when commands are switched or its
// identity changes. Every client of one state directory shares one
// containment, so a new client does not prepare again, which would stop units
// an old client's start may still be using.
func TestCommandClientsOfOneStateDirShareOneContainment(t *testing.T) {
	logger := zerolog.Nop()
	stateDir := t.TempDir()
	build := func(dir string) *CommandClient {
		client := NewCommandClient(Config{PulseURL: "http://127.0.0.1:1", APIToken: "token", StateDir: dir, EnableCommands: true, Logger: &logger}, "agent-pve", "pve.example.test", "linux", "test")
		t.Cleanup(func() { _ = client.Close() })
		return client
	}
	first := build(stateDir)
	prepared := 0
	first.guestStart.prepare = func(context.Context, typedActionSupervisor) (string, error) {
		prepared++
		return "", nil
	}
	first.prepareGuestStart(context.Background())
	if version, _ := first.guestStartRegistration(); version != agentexec.ProxmoxGuestStartProtocolVersion {
		t.Fatalf("the first client is not ready: version %d", version)
	}

	second := build(stateDir)
	if second.guestStart != first.guestStart {
		t.Fatal("a replacement command client built its own containment")
	}
	second.prepareGuestStart(context.Background())
	if version, _ := second.guestStartRegistration(); version != agentexec.ProxmoxGuestStartProtocolVersion || prepared != 1 {
		t.Fatalf("the replacement client prepared again: version %d, prepared %d times", version, prepared)
	}
	if other := build(t.TempDir()); other.guestStart == first.guestStart {
		t.Fatal("another state directory shares this one's containment")
	}
}

// Containment can become ready after a session registered without it. The
// collector says so on the live session at its next ping, Pulse takes the new
// answer, and the next start runs. The session is never dropped, so nothing
// running on it is interrupted.
func TestRealServerCollectorRestatesGuestStartOnTheLiveSession(t *testing.T) {
	const pingInterval = 20 * time.Millisecond

	node := &collectorGuestRun{}
	var mu sync.Mutex
	fail := true
	server, client, admission := startCollectorSession(t, node, pingInterval, func(context.Context, typedActionSupervisor) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if fail {
			return agentexec.ProxmoxGuestStartUnavailableContainment, errors.New("probe failed")
		}
		return "", nil
	})
	sessions := server.GetProxmoxGuestStartSessionsForOrganization("default")
	if len(sessions) != 1 || sessions[0].ProxmoxGuestStartVersion != 0 || sessions[0].ProxmoxGuestStartUnavailable != agentexec.ProxmoxGuestStartUnavailableContainment {
		t.Fatalf("sessions = %#v, want the session registered without guest start", sessions)
	}
	connectedAt := sessions[0].ConnectedAt
	// Several ping ticks pass and nothing changes, so the answer stands.
	time.Sleep(5 * pingInterval)
	if sessions = server.GetProxmoxGuestStartSessionsForOrganization("default"); len(sessions) != 1 || sessions[0].ProxmoxGuestStartVersion != 0 {
		t.Fatalf("sessions = %#v, want the registered answer to stand", sessions)
	}

	mu.Lock()
	fail = false
	mu.Unlock()
	client.prepareGuestStart(context.Background())
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		sessions = server.GetProxmoxGuestStartSessionsForOrganization("default")
		if len(sessions) == 1 && sessions[0].ProxmoxGuestStartVersion == agentexec.ProxmoxGuestStartProtocolVersion {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(sessions) != 1 || sessions[0].ProxmoxGuestStartVersion != agentexec.ProxmoxGuestStartProtocolVersion || sessions[0].ProxmoxGuestStartUnavailable != "" {
		t.Fatalf("sessions = %#v, want Pulse to hold the restated answer", sessions)
	}
	if !sessions[0].ConnectedAt.Equal(connectedAt) || !server.IsAgentConnected(admission.AgentID) {
		t.Fatal("the session was dropped or replaced to restate a capability")
	}
	result, err := server.ExecuteProxmoxGuestLifecycle(context.Background(), admission.AgentID, collectorGuestRequest(t, "start", "stopped"))
	if err != nil || !result.MutationCompleted {
		t.Fatalf("start after the restated answer = %+v, %v", result, err)
	}
}

// A short fixture cadence belongs to that client only. Other clients, including
// one with a nonpositive override, keep the production 30-second interval.
func TestCollectorPingCadenceIsClientLocal(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		t.Run(interval.String(), func(t *testing.T) {
			var ready atomic.Bool
			prepare := func(context.Context, typedActionSupervisor) (string, error) {
				if !ready.Load() {
					return agentexec.ProxmoxGuestStartUnavailableContainment, errors.New("probe failed")
				}
				return "", nil
			}
			ordinaryServer, ordinary, admission := startCollectorSession(t, &collectorGuestRun{}, interval, prepare)
			fastServer, fast, _ := startCollectorSession(t, &collectorGuestRun{}, 10*time.Millisecond, prepare)
			ready.Store(true)
			ordinary.prepareGuestStart(context.Background())
			fast.prepareGuestStart(context.Background())

			deadline := time.Now().Add(3 * time.Second)
			for {
				sessions := fastServer.GetProxmoxGuestStartSessionsForOrganization("default")
				if len(sessions) == 1 && sessions[0].ProxmoxGuestStartVersion == agentexec.ProxmoxGuestStartProtocolVersion {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("the fast client did not restate at its own cadence")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if sessions := ordinaryServer.GetProxmoxGuestStartSessionsForOrganization("default"); len(sessions) != 1 || sessions[0].ProxmoxGuestStartVersion != 0 || sessions[0].ProxmoxGuestStartUnavailable != agentexec.ProxmoxGuestStartUnavailableContainment {
				t.Fatalf("the short cadence affected another client's session: %#v", sessions)
			}
			if !ordinaryServer.IsAgentConnected(admission.AgentID) {
				t.Fatal("the ordinary client disconnected instead of keeping its cadence")
			}
		})
	}
}

// Only a collector with containment restates, and only on its live
// connection. A ping loop that outlived its connection says nothing about the
// session that replaced it.
func TestRestateGuestStartIsForTheLiveCollectorConnectionOnly(t *testing.T) {
	logger := zerolog.Nop()
	stale := &websocket.Conn{}

	runner := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: "https://127.0.0.1:1", APIToken: "token", StateDir: t.TempDir(),
		HealthPath: filepath.Join(t.TempDir(), "health.json"), ActivationNonce: strings.Repeat("a", 32), Logger: &logger,
	}, "agent-pve", "pve.example.test", "test")
	t.Cleanup(func() { _ = runner.Close() })
	if restated, err := runner.restateGuestStartIfChanged(stale); restated || err != nil {
		t.Fatalf("the action runner restated a collector capability: %v, %v", restated, err)
	}

	commandsOff := NewCommandClient(Config{PulseURL: "http://127.0.0.1:1", APIToken: "token", StateDir: t.TempDir(), EnableCommands: false, Logger: &logger}, "agent-pve", "pve.example.test", "linux", "test")
	t.Cleanup(func() { _ = commandsOff.Close() })
	if restated, err := commandsOff.restateGuestStartIfChanged(stale); restated || err != nil {
		t.Fatalf("a commands-off collector restated a capability: %v, %v", restated, err)
	}

	// A collector whose answer has changed, asked on a connection that is not
	// its live one: nothing is sent and the session's bookkeeping is untouched.
	collector := NewCommandClient(Config{PulseURL: "http://127.0.0.1:1", APIToken: "token", StateDir: t.TempDir(), EnableCommands: true, Logger: &logger}, "agent-pve", "pve.example.test", "linux", "test")
	t.Cleanup(func() { _ = collector.Close() })
	collector.guestStart.prepare = func(context.Context, typedActionSupervisor) (string, error) { return "", nil }
	collector.prepareGuestStart(context.Background())
	if restated, err := collector.restateGuestStartIfChanged(stale); restated || err != nil {
		t.Fatalf("a stale connection restated: %v, %v", restated, err)
	}
	collector.connMu.Lock()
	restating := collector.guestStartRestating
	collector.connMu.Unlock()
	if restating {
		t.Fatal("a stale ping marked the live session as changed")
	}
}
