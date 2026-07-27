package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func newProbeAgentTestMonitor(t *testing.T, targets ...config.AvailabilityTarget) *Monitor {
	t.Helper()
	persistence := config.NewConfigPersistence(t.TempDir())
	normalized := make([]config.AvailabilityTarget, 0, len(targets))
	for _, target := range targets {
		normalized = append(normalized, config.NormalizeAvailabilityTarget(target))
	}
	if err := persistence.SaveAvailabilityTargets(normalized); err != nil {
		t.Fatalf("SaveAvailabilityTargets() error = %v", err)
	}
	return &Monitor{
		state:                models.NewState(),
		configPersist:        persistence,
		availabilityStatuses: make(map[string]AvailabilityProbeStatus),
		pollStatusMap:        make(map[string]*pollStatus),
		failureCounts:        make(map[string]int),
		lastOutcome:          make(map[string]taskOutcome),
		circuitBreakers:      make(map[string]*circuitBreaker),
		taskQueue:            NewTaskQueue(),
	}
}

func licenseWithExternalProbe(enabled bool) func(string) bool {
	return func(feature string) bool {
		return enabled && feature == pkglicensing.FeatureExternalProbe
	}
}

func probeAgentTarget(id string, agentID string) config.AvailabilityTarget {
	return config.AvailabilityTarget{
		ID:               id,
		Name:             id,
		Address:          id + ".local",
		Protocol:         config.AvailabilityProbeICMP,
		Enabled:          true,
		PollIntervalSecs: 60,
		FailureThreshold: 2,
		ProbeAgentID:     agentID,
	}
}

func TestEffectiveProbeAgentIDRequiresEntitlement(t *testing.T) {
	monitor := newProbeAgentTestMonitor(t)
	assigned := config.NormalizeAvailabilityTarget(probeAgentTarget("remote", "agent-1"))
	local := config.NormalizeAvailabilityTarget(probeAgentTarget("local", ""))

	if got := monitor.effectiveProbeAgentID(assigned); got != "" {
		t.Fatalf("effectiveProbeAgentID() = %q without a license checker, want empty", got)
	}

	monitor.SetLicenseChecker(licenseWithExternalProbe(true))
	if got := monitor.effectiveProbeAgentID(assigned); got != "agent-1" {
		t.Fatalf("effectiveProbeAgentID() = %q, want agent-1", got)
	}
	if got := monitor.effectiveProbeAgentID(local); got != "" {
		t.Fatalf("effectiveProbeAgentID() = %q for an unassigned target, want empty", got)
	}

	monitor.SetLicenseChecker(licenseWithExternalProbe(false))
	if got := monitor.effectiveProbeAgentID(assigned); got != "" {
		t.Fatalf("effectiveProbeAgentID() = %q after a lapse, want empty", got)
	}
}

func TestAvailabilityPollProviderSkipsProbeAssignedTargetsAndResumesOnLapse(t *testing.T) {
	monitor := newProbeAgentTestMonitor(t,
		probeAgentTarget("remote", "agent-1"),
		probeAgentTarget("local", ""),
	)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))

	names := availabilityPollProvider{}.ListInstances(monitor)
	if len(names) != 1 || names[0] != "local" {
		t.Fatalf("ListInstances() = %+v, want only the locally executed target", names)
	}
	if _, err := (availabilityPollProvider{}).BuildPollTask(monitor, "remote"); err == nil {
		t.Fatal("BuildPollTask() error = nil, want refusal for a probe-assigned target")
	}
	if _, err := (availabilityPollProvider{}).BuildPollTask(monitor, "local"); err != nil {
		t.Fatalf("BuildPollTask(local) error = %v", err)
	}

	// The entitlement lapses: the poller must resume the target without a
	// restart, on the next planning pass.
	monitor.SetLicenseChecker(licenseWithExternalProbe(false))
	names = availabilityPollProvider{}.ListInstances(monitor)
	if len(names) != 2 {
		t.Fatalf("ListInstances() after lapse = %+v, want both targets", names)
	}
	if _, err := (availabilityPollProvider{}).BuildPollTask(monitor, "remote"); err != nil {
		t.Fatalf("BuildPollTask(remote) after lapse error = %v", err)
	}
}

func TestRefreshAvailabilityTargetsUnschedulesProbeAssignedTargets(t *testing.T) {
	monitor := newProbeAgentTestMonitor(t,
		probeAgentTarget("remote", "agent-1"),
		probeAgentTarget("local", ""),
	)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))

	monitor.RefreshAvailabilityTargets()
	if depth := monitor.taskQueue.Snapshot().Depth; depth != 1 {
		t.Fatalf("queue depth = %d, want only the locally executed target", depth)
	}

	monitor.SetLicenseChecker(licenseWithExternalProbe(false))
	monitor.RefreshAvailabilityTargets()
	if depth := monitor.taskQueue.Snapshot().Depth; depth != 2 {
		t.Fatalf("queue depth after lapse = %d, want both targets scheduled locally", depth)
	}
}

func TestGetHostAgentConfigIncludesOnlyOwnAssignedTargets(t *testing.T) {
	monitor := newProbeAgentTestMonitor(t,
		probeAgentTarget("remote-a", "agent-1"),
		probeAgentTarget("remote-b", "agent-2"),
		probeAgentTarget("local", ""),
	)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))

	cfg := monitor.GetHostAgentConfig("agent-1")
	raw, ok := cfg.Settings["availabilityTargets"]
	if !ok {
		t.Fatalf("settings = %+v, want availabilityTargets", cfg.Settings)
	}
	payload, ok := raw.([]map[string]interface{})
	if !ok {
		t.Fatalf("availabilityTargets type = %T, want []map[string]interface{}", raw)
	}
	if len(payload) != 1 || payload[0]["id"] != "remote-a" {
		t.Fatalf("availabilityTargets = %+v, want only remote-a", payload)
	}
	for _, forbidden := range []string{"failureThreshold", "linkedResourceId", "probeAgentId"} {
		if _, present := payload[0][forbidden]; present {
			t.Fatalf("availability target payload leaked %q: %+v", forbidden, payload[0])
		}
	}
	if cfg.DesiredConfig == nil {
		t.Fatal("desired config metadata = nil, want signed config metadata")
	}

	if cfg := monitor.GetHostAgentConfig("agent-3"); cfg.Settings["availabilityTargets"] != nil {
		t.Fatalf("unassigned agent settings = %+v, want no availabilityTargets key", cfg.Settings)
	}

	monitor.SetLicenseChecker(licenseWithExternalProbe(false))
	if cfg := monitor.GetHostAgentConfig("agent-1"); cfg.Settings["availabilityTargets"] != nil {
		t.Fatalf("unlicensed settings = %+v, want no availabilityTargets key", cfg.Settings)
	}
}

func TestGetHostAgentConfigPayloadCarriesProbeParameters(t *testing.T) {
	monitor := newProbeAgentTestMonitor(t, config.AvailabilityTarget{
		ID:               "udp-check",
		Name:             "UDP check",
		TargetKind:       config.AvailabilityTargetDevice,
		Address:          "sensor.local",
		Protocol:         config.AvailabilityProbeUDP,
		Port:             5353,
		UDPMode:          config.AvailabilityUDPResponseRequired,
		UDPRequest:       "ping",
		UDPExpected:      "pong",
		Enabled:          true,
		PollIntervalSecs: 45,
		TimeoutMillis:    1500,
		ProbeAgentID:     "agent-1",
	})
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))

	payload := monitor.availabilityProbeTargetsForAgent("agent-1")
	if len(payload) != 1 {
		t.Fatalf("payload = %+v, want one target", payload)
	}
	want := map[string]interface{}{
		"id":                  "udp-check",
		"name":                "UDP check",
		"targetKind":          string(config.AvailabilityTargetDevice),
		"address":             "sensor.local",
		"protocol":            string(config.AvailabilityProbeUDP),
		"port":                5353,
		"udpMode":             string(config.AvailabilityUDPResponseRequired),
		"udpRequest":          "ping",
		"udpExpectedResponse": "pong",
		"enabled":             true,
		"pollIntervalSeconds": 45,
		"timeoutMillis":       1500,
	}
	for key, expected := range want {
		if got := payload[0][key]; got != expected {
			t.Fatalf("payload[%q] = %v, want %v", key, got, expected)
		}
	}
	if len(payload[0]) != len(want) {
		t.Fatalf("payload keys = %+v, want exactly %d probe parameters", payload[0], len(want))
	}
}

func TestApplyProbeAvailabilityResultsRejectsForeignAgents(t *testing.T) {
	monitor := newProbeAgentTestMonitor(t,
		probeAgentTarget("remote", "agent-1"),
		probeAgentTarget("local", ""),
	)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))

	checkedAt := time.Now().UTC()
	monitor.ApplyProbeAvailabilityResults("agent-2", []ProbeAvailabilityResult{
		{TargetID: "remote", Outcome: AvailabilityProbeReachable, LatencyMillis: 5, CheckedAt: checkedAt},
		{TargetID: "local", Outcome: AvailabilityProbeReachable, LatencyMillis: 5, CheckedAt: checkedAt},
		{TargetID: "missing", Outcome: AvailabilityProbeReachable, LatencyMillis: 5, CheckedAt: checkedAt},
	})
	if statuses := monitor.AvailabilityStatusSnapshot(); len(statuses) != 0 {
		t.Fatalf("statuses = %+v, want no state from an agent that owns nothing", statuses)
	}

	// The owning agent may not claim a locally executed target either.
	monitor.ApplyProbeAvailabilityResults("agent-1", []ProbeAvailabilityResult{
		{TargetID: "local", Outcome: AvailabilityProbeReachable, LatencyMillis: 5, CheckedAt: checkedAt},
	})
	if _, ok := monitor.AvailabilityStatusSnapshot()["local"]; ok {
		t.Fatal("locally executed target accepted a probe agent result")
	}

	// A lapsed entitlement withdraws ownership too.
	monitor.SetLicenseChecker(licenseWithExternalProbe(false))
	monitor.ApplyProbeAvailabilityResults("agent-1", []ProbeAvailabilityResult{
		{TargetID: "remote", Outcome: AvailabilityProbeReachable, LatencyMillis: 5, CheckedAt: checkedAt},
	})
	if _, ok := monitor.AvailabilityStatusSnapshot()["remote"]; ok {
		t.Fatal("unlicensed probe agent result was accepted")
	}
}

func TestApplyProbeAvailabilityResultsAccountsFailuresAndAttribution(t *testing.T) {
	monitor := newProbeAgentTestMonitor(t, probeAgentTarget("remote", "agent-1"))
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))

	checkedAt := time.Now().UTC()
	monitor.ApplyProbeAvailabilityResults("agent-1", []ProbeAvailabilityResult{
		{TargetID: "remote", Outcome: AvailabilityProbeReachable, LatencyMillis: 12, CheckedAt: checkedAt},
	})
	status := monitor.AvailabilityStatusSnapshot()["remote"]
	if !status.Available || status.ProbeAgentID != "agent-1" {
		t.Fatalf("status = %+v, want available and attributed to agent-1", status)
	}
	if !status.LastSuccess.Equal(checkedAt) {
		t.Fatalf("last success = %v, want %v", status.LastSuccess, checkedAt)
	}
	if healthy, ok := monitor.state.GetSnapshot().ConnectionHealth["availability-remote"]; !ok || !healthy {
		t.Fatalf("connection health = %v/%v, want healthy", healthy, ok)
	}

	// Two failures reach the threshold; an unreachable report with no message
	// must still count.
	for i := 1; i <= 2; i++ {
		monitor.ApplyProbeAvailabilityResults("agent-1", []ProbeAvailabilityResult{
			{TargetID: "remote", Outcome: AvailabilityProbeUnreachable, CheckedAt: checkedAt.Add(time.Duration(i) * time.Minute)},
		})
		status = monitor.AvailabilityStatusSnapshot()["remote"]
		if status.ConsecutiveFailures != i {
			t.Fatalf("consecutive failures = %d after %d unreachable reports, want %d", status.ConsecutiveFailures, i, i)
		}
	}
	if status.Available || status.LastError == "" {
		t.Fatalf("status = %+v, want an unavailable target with an error", status)
	}
	if !status.LastSuccess.Equal(checkedAt) {
		t.Fatalf("last success = %v, want the carried-forward success %v", status.LastSuccess, checkedAt)
	}
	if healthy := monitor.state.GetSnapshot().ConnectionHealth["availability-remote"]; healthy {
		t.Fatal("connection health = healthy, want unhealthy after failures")
	}

	// A later success resets the accounting.
	recovered := checkedAt.Add(3 * time.Minute)
	monitor.ApplyProbeAvailabilityResults("agent-1", []ProbeAvailabilityResult{
		{TargetID: "remote", Outcome: AvailabilityProbeReachable, LatencyMillis: 8, CheckedAt: recovered},
	})
	status = monitor.AvailabilityStatusSnapshot()["remote"]
	if status.ConsecutiveFailures != 0 || status.LastError != "" || !status.Available {
		t.Fatalf("status = %+v, want a recovered target", status)
	}
	if !status.LastSuccess.Equal(recovered) {
		t.Fatalf("last success = %v, want %v", status.LastSuccess, recovered)
	}
}

func TestApplyProbeAvailabilityResultsIndeterminateClearsFailures(t *testing.T) {
	monitor := newProbeAgentTestMonitor(t, probeAgentTarget("remote", "agent-1"))
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))

	checkedAt := time.Now().UTC()
	monitor.ApplyProbeAvailabilityResults("agent-1", []ProbeAvailabilityResult{
		{TargetID: "remote", Outcome: AvailabilityProbeUnreachable, Error: "icmp probe timed out", CheckedAt: checkedAt},
	})
	monitor.ApplyProbeAvailabilityResults("agent-1", []ProbeAvailabilityResult{
		{TargetID: "remote", Outcome: AvailabilityProbeIndeterminate, CheckedAt: checkedAt.Add(time.Minute)},
	})

	status := monitor.AvailabilityStatusSnapshot()["remote"]
	if status.ConsecutiveFailures != 0 {
		t.Fatalf("consecutive failures = %d after an indeterminate report, want 0", status.ConsecutiveFailures)
	}
	if status.Available {
		t.Fatalf("status = %+v, want indeterminate to stay non-available", status)
	}
	if status.Outcome != string(AvailabilityProbeIndeterminate) {
		t.Fatalf("outcome = %q, want indeterminate", status.Outcome)
	}
}

func TestAvailabilityProbeStalenessDerivesIndeterminateAtReadTime(t *testing.T) {
	target := config.NormalizeAvailabilityTarget(probeAgentTarget("remote", "agent-1"))
	monitor := newProbeAgentTestMonitor(t, target)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))

	checkedAt := time.Now().UTC()
	monitor.ApplyProbeAvailabilityResults("agent-1", []ProbeAvailabilityResult{
		{TargetID: "remote", Outcome: AvailabilityProbeReachable, LatencyMillis: 9, CheckedAt: checkedAt},
	})

	// Effective poll interval is 60s, so the floor of five minutes governs.
	fresh := monitor.availabilityStatusSnapshotForTargets([]config.AvailabilityTarget{target}, checkedAt.Add(5*time.Minute))
	if !fresh["remote"].Available || fresh["remote"].LastError != "" {
		t.Fatalf("status at the boundary = %+v, want the reported observation", fresh["remote"])
	}

	stale := monitor.availabilityStatusSnapshotForTargets([]config.AvailabilityTarget{target}, checkedAt.Add(5*time.Minute+time.Second))
	if stale["remote"].Outcome != string(AvailabilityProbeIndeterminate) {
		t.Fatalf("stale outcome = %q, want indeterminate", stale["remote"].Outcome)
	}
	if stale["remote"].Available {
		t.Fatal("stale status reported available")
	}
	if stale["remote"].LastError != availabilityProbeStaleError {
		t.Fatalf("stale last error = %q, want %q", stale["remote"].LastError, availabilityProbeStaleError)
	}

	// Stored state is untouched: the derivation is read-time only.
	monitor.mu.RLock()
	stored := monitor.availabilityStatuses["remote"]
	monitor.mu.RUnlock()
	if !stored.Available || stored.LastError != "" {
		t.Fatalf("stored status = %+v, want the unmodified observation", stored)
	}

	// A long poll interval widens the window past the floor.
	slow := target
	slow.PollIntervalSecs = 600
	slowSnapshot := monitor.availabilityStatusSnapshotForTargets([]config.AvailabilityTarget{slow}, checkedAt.Add(29*time.Minute))
	if !slowSnapshot["remote"].Available {
		t.Fatalf("status = %+v, want 3x the poll interval to govern", slowSnapshot["remote"])
	}

	// Local targets never derive probe staleness.
	monitor.SetLicenseChecker(licenseWithExternalProbe(false))
	lapsed := monitor.availabilityStatusSnapshotForTargets([]config.AvailabilityTarget{target}, checkedAt.Add(time.Hour))
	if !lapsed["remote"].Available {
		t.Fatalf("status after lapse = %+v, want local execution semantics", lapsed["remote"])
	}
}

func TestAvailabilitySupplementalRecordsPresentStaleProbeAsIndeterminate(t *testing.T) {
	monitor := newProbeAgentTestMonitor(t, probeAgentTarget("remote", "agent-1"))
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))

	monitor.ApplyProbeAvailabilityResults("agent-1", []ProbeAvailabilityResult{
		{TargetID: "remote", Outcome: AvailabilityProbeReachable, LatencyMillis: 9, CheckedAt: time.Now().UTC().Add(-time.Hour)},
	})

	records := availabilityPollProvider{}.SupplementalRecords(monitor, "org-a")
	if len(records) != 1 {
		t.Fatalf("SupplementalRecords() length = %d, want 1", len(records))
	}
	data := records[0].Resource.Availability
	if data == nil {
		t.Fatal("availability payload = nil")
	}
	if data.ProbeOutcome != string(AvailabilityProbeIndeterminate) || data.Available {
		t.Fatalf("availability payload = %+v, want a stale indeterminate projection", data)
	}
	if data.LastError != availabilityProbeStaleError {
		t.Fatalf("availability last error = %q, want %q", data.LastError, availabilityProbeStaleError)
	}

	statuses := availabilityPollProvider{}.ConnectionStatuses(monitor)
	if statuses["availability-remote"] {
		t.Fatalf("connection statuses = %+v, want a stale probe to read as not connected", statuses)
	}
}
