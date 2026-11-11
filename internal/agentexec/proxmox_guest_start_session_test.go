package agentexec

import (
	"context"
	"sort"
	"strings"
	"testing"
)

func boundProxmoxGuestOperation(t *testing.T, operation, expectedStatus string) ProxmoxGuestLifecyclePayload {
	t.Helper()
	payload := ProxmoxGuestLifecyclePayload{RequestID: "attempt-" + operation, ActionID: "action-" + operation, Operation: operation, GuestKind: "ct", VMID: 112, ExpectedStatus: expectedStatus, Timeout: 30}
	if err := BindProxmoxGuestLifecyclePayload(&payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func guestStartTestServer() (*Server, func(org, id, host, role, capability, token string, startVersion int) *agentConn) {
	s := NewServerWithAdmissionValidator(func(string, string, string) (AgentAdmission, bool) {
		return AgentAdmission{}, false
	}, func(a AgentAdmission) bool { return a.TokenID != "revoked" })
	add := func(org, id, host, role, capability, token string, startVersion int) *agentConn {
		ac := &agentConn{
			admission: AgentAdmission{OrganizationID: org, AgentID: id, Hostname: host, RuntimeRole: role, ActionCapability: capability, TokenID: token},
			agent:     ConnectedAgent{AgentID: id, Hostname: host, OperationReceiptVersion: 1},
			done:      make(chan struct{}),
		}
		ac.recordProxmoxGuestStart(startVersion, "")
		s.agents[agentSessionKey(org, id)] = ac
		return ac
	}
	return s, add
}

// The gate decides from the session's credential and from what its agent
// reported, never from an agent version. A collector's command session carries
// a start when its agent reported the protocol, and carries nothing else.
func TestProxmoxGuestLifecycleGateAllowsOnlyStartOnACollectorCommandSession(t *testing.T) {
	_, add := guestStartTestServer()
	capable := add("org-a", "capable", "pve1", RuntimeRoleLegacyFullTrust, "", "valid", ProxmoxGuestStartProtocolVersion)
	old := add("org-a", "old", "pve2", RuntimeRoleLegacyFullTrust, "", "valid", 0)
	future := add("org-a", "future", "pve3", RuntimeRoleLegacyFullTrust, "", "valid", ProxmoxGuestStartProtocolVersion+1)
	runner := add("org-a", "runner", "pve4", RuntimeRoleActionRunner, ActionCapabilityTypedV1, "valid", 0)
	// A legacy credential that carries a typed capability is not a collector
	// session this gate knows.
	odd := add("org-a", "odd", "pve5", RuntimeRoleLegacyFullTrust, ActionCapabilityTypedV1, "valid", ProxmoxGuestStartProtocolVersion)

	if !proxmoxGuestLifecycleSessionAllows(capable, "start") {
		t.Fatal("a collector session that reported guest start was refused a start")
	}
	for _, verb := range []string{"stop", "shutdown", "reboot", "", "START", "status"} {
		if proxmoxGuestLifecycleSessionAllows(capable, verb) {
			t.Fatalf("a collector session was allowed %q", verb)
		}
	}
	for name, ac := range map[string]*agentConn{"old agent": old, "unknown protocol version": future, "odd credential": odd, "no session": nil} {
		if proxmoxGuestLifecycleSessionAllows(ac, "start") {
			t.Fatalf("%s was allowed a start", name)
		}
	}
	for _, verb := range []string{"start", "stop", "shutdown", "reboot"} {
		if !proxmoxGuestLifecycleSessionAllows(runner, verb) {
			t.Fatalf("the action runner was refused %s", verb)
		}
	}
	// What a runner reports about guest start does not make it a collector.
	if commandSessionStartsProxmoxGuests(runner) {
		t.Fatal("an action runner was read as a collector command session")
	}
}

// The same decisions at dispatch, on the session that would carry the request.
// A refusal returns before anything is written to the agent.
func TestProxmoxGuestLifecycleDispatchRefusesEveryVerbButStartOnACollectorSession(t *testing.T) {
	s, add := guestStartTestServer()
	add("org-a", "capable", "pve1", RuntimeRoleLegacyFullTrust, "", "valid", ProxmoxGuestStartProtocolVersion)
	add("org-a", "old", "pve2", RuntimeRoleLegacyFullTrust, "", "valid", 0)
	ctx := WithOrganizationID(context.Background(), "org-a")

	for _, verb := range []string{"stop", "shutdown", "reboot"} {
		expected := "running"
		if _, err := s.ExecuteProxmoxGuestLifecycle(ctx, "capable", boundProxmoxGuestOperation(t, verb, expected)); err == nil || !strings.Contains(err.Error(), "requires a typed action-runner session") {
			t.Fatalf("%s on a collector session: %v, want the runner refusal", verb, err)
		}
	}
	// A monitoring-only agent holds no command session, and an agent that
	// predates guest start reports nothing. Both are refused a start.
	if _, err := s.ExecuteProxmoxGuestLifecycle(ctx, "old", boundProxmoxGuestOperation(t, "start", "stopped")); err == nil || !strings.Contains(err.Error(), "requires a typed action-runner session") {
		t.Fatalf("start on an agent that did not report guest start: %v", err)
	}
	if _, err := s.ExecuteProxmoxGuestLifecycle(ctx, "monitoring-only", boundProxmoxGuestOperation(t, "start", "stopped")); err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("start on an agent with no command session: %v", err)
	}
	// Another tenant cannot reach the capable session.
	if _, err := s.ExecuteProxmoxGuestLifecycle(WithOrganizationID(context.Background(), "org-b"), "capable", boundProxmoxGuestOperation(t, "start", "stopped")); err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("cross-tenant start: %v", err)
	}
}

// A caller that planned a start on the action runner says so. If a collector
// session has taken the runner's place under the same agent ID by the time
// the request is dispatched, it is refused: the caller would judge its answer
// by the runner's rules.
func TestProxmoxGuestLifecycleDispatchRefusesACollectorWhereARunnerWasPlanned(t *testing.T) {
	s, add := guestStartTestServer()
	add("org-a", "pve1-agent", "pve1", RuntimeRoleLegacyFullTrust, "", "valid", ProxmoxGuestStartProtocolVersion)
	ctx := RequireActionRunnerSession(WithOrganizationID(context.Background(), "org-a"))
	if !ActionRunnerSessionRequired(ctx) || ActionRunnerSessionRequired(context.Background()) {
		t.Fatal("the runner requirement did not travel with the context")
	}
	if _, err := s.ExecuteProxmoxGuestLifecycle(ctx, "pve1-agent", boundProxmoxGuestOperation(t, "start", "stopped")); err == nil || !strings.Contains(err.Error(), "requires a typed action-runner session") {
		t.Fatalf("a collector session carried a start planned on a runner: %v", err)
	}
}

// The list the executor reads holds collector command sessions only, with what
// each reported. It never holds a runner, another tenant's session or a
// session whose credential was revoked.
func TestProxmoxGuestStartSessionsListCollectorSessionsOnly(t *testing.T) {
	s, add := guestStartTestServer()
	add("org-a", "capable", "pve1", RuntimeRoleLegacyFullTrust, "", "valid", ProxmoxGuestStartProtocolVersion)
	unavailable := add("org-a", "no-systemd", "pve2", RuntimeRoleLegacyFullTrust, "", "valid", 0)
	unavailable.recordProxmoxGuestStart(0, ProxmoxGuestStartUnavailableNotSystemdService)
	add("org-a", "old", "pve3", RuntimeRoleLegacyFullTrust, "", "valid", 0)
	add("org-a", "runner", "pve4", RuntimeRoleActionRunner, ActionCapabilityTypedV1, "valid", 0)
	add("org-a", "revoked", "pve5", RuntimeRoleLegacyFullTrust, "", "revoked", ProxmoxGuestStartProtocolVersion)
	add("org-b", "other-tenant", "pve1", RuntimeRoleLegacyFullTrust, "", "valid", ProxmoxGuestStartProtocolVersion)

	describe := func(organizationID string) string {
		var rows []string
		for _, session := range s.GetProxmoxGuestStartSessionsForOrganization(organizationID) {
			rows = append(rows, session.AgentID+"="+strings.Join([]string{string(rune('0' + session.ProxmoxGuestStartVersion)), session.ProxmoxGuestStartUnavailable}, "/"))
		}
		sort.Strings(rows)
		return strings.Join(rows, " ")
	}
	if got, want := describe("org-a"), "capable=1/ no-systemd=0/not_systemd_service old=0/"; got != want {
		t.Fatalf("org-a sessions = %q, want %q", got, want)
	}
	if got, want := describe("org-b"), "other-tenant=1/"; got != want {
		t.Fatalf("org-b sessions = %q, want %q", got, want)
	}
	if runners := s.GetActionRunnersForOrganization("org-a"); len(runners) != 1 || runners[0].AgentID != "runner" {
		t.Fatalf("runners = %#v, want the runner only", runners)
	}
}

func TestNormalizeProxmoxGuestStartUnavailableKeepsTheClosedSet(t *testing.T) {
	for in, want := range map[string]string{
		"": "", "  ": "",
		ProxmoxGuestStartUnavailableNotSystemdService: ProxmoxGuestStartUnavailableNotSystemdService,
		" " + ProxmoxGuestStartUnavailableContainment: ProxmoxGuestStartUnavailableContainment,
		"something a newer agent says":                ProxmoxGuestStartUnavailableContainment,
	} {
		if got := NormalizeProxmoxGuestStartUnavailable(in); got != want {
			t.Fatalf("NormalizeProxmoxGuestStartUnavailable(%q) = %q, want %q", in, got, want)
		}
	}
}

// A collector's later capabilities message replaces what it registered with.
// A runner's session never holds a collector's report, whatever it sends.
func TestAgentConnRecordsGuestStartForCollectorSessionsOnly(t *testing.T) {
	_, add := guestStartTestServer()
	collector := add("org-a", "collector", "pve1", RuntimeRoleLegacyFullTrust, "", "valid", 0)
	if commandSessionStartsProxmoxGuests(collector) {
		t.Fatal("a session that reported nothing can start guests")
	}
	collector.recordProxmoxGuestStart(0, "something a newer agent says")
	if got := collector.proxmoxGuestStart(); got.version != 0 || got.unavailable != ProxmoxGuestStartUnavailableContainment {
		t.Fatalf("report = %+v, want the reason kept inside the closed set", got)
	}
	collector.recordProxmoxGuestStart(ProxmoxGuestStartProtocolVersion, "")
	if !commandSessionStartsProxmoxGuests(collector) || !proxmoxGuestLifecycleSessionAllows(collector, "start") {
		t.Fatal("the restated answer did not take effect")
	}
	collector.recordProxmoxGuestStart(0, ProxmoxGuestStartUnavailableNotSystemdService)
	if commandSessionStartsProxmoxGuests(collector) {
		t.Fatal("a session that withdrew guest start can still start guests")
	}

	runner := add("org-a", "runner", "pve2", RuntimeRoleActionRunner, ActionCapabilityTypedV1, "valid", 0)
	runner.recordProxmoxGuestStart(ProxmoxGuestStartProtocolVersion, "")
	if got := runner.proxmoxGuestStart(); got != (proxmoxGuestStartReport{}) || commandSessionStartsProxmoxGuests(runner) {
		t.Fatalf("a runner session holds a collector report: %+v", got)
	}
}
