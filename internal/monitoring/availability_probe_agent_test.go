package monitoring

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	pkgmetrics "github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
)

func TestPollTaskWorkerLimiterCapsAvailabilityAgentWorkAcrossTenants(t *testing.T) {
	const (
		limit   = 4
		tenants = 12
	)
	limiter := newPollTaskWorkerLimiter(limit)
	ctx := context.Background()
	entered := make(chan int, tenants)
	release := make(chan struct{})

	var wg sync.WaitGroup
	for tenant := 0; tenant < tenants; tenant++ {
		wg.Add(1)
		go func(tenant int) {
			defer wg.Done()
			if !limiter.acquire(ctx) {
				t.Errorf("tenant %d failed to acquire limiter", tenant)
				return
			}
			defer limiter.release()
			entered <- tenant
			<-release
		}(tenant)
	}

	for i := 0; i < limit; i++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatalf("only %d of %d process-wide slots became available", i, limit)
		}
	}
	select {
	case tenant := <-entered:
		t.Fatalf("tenant %d exceeded the process-wide limit of %d", tenant, limit)
	case <-time.After(25 * time.Millisecond):
	}

	close(release)
	wg.Wait()
}

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
		state:                  models.NewState(),
		configPersist:          persistence,
		availabilityStatuses:   make(map[string]AvailabilityProbeStatus),
		availabilityByLocation: make(map[string]map[string]AvailabilityProbeStatus),
		pollStatusMap:          make(map[string]*pollStatus),
		failureCounts:          make(map[string]int),
		lastOutcome:            make(map[string]taskOutcome),
		circuitBreakers:        make(map[string]*circuitBreaker),
		taskQueue:              NewTaskQueue(),
	}
}

func TestMultiLocationAvailabilityPreservesDisagreementUntilEveryPathFails(t *testing.T) {
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID:                     "service",
		Name:                   "Customer API",
		Address:                "api.service.local",
		Protocol:               config.AvailabilityProbeHTTPS,
		Enabled:                true,
		PollIntervalSecs:       60,
		FailureThreshold:       2,
		ObservationLocationIDs: []string{config.AvailabilityObservationLocationLocal, config.AvailabilityAgentObservationLocationID("edge-1")},
	})
	monitor := newProbeAgentTestMonitor(t, target)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))
	now := time.Now().UTC()

	monitor.applyAvailabilityObservation(target, "local-ok", now, 8*time.Millisecond, AvailabilityProbeReachable, nil, nil, "", time.Time{})
	for attempt := 0; attempt < 2; attempt++ {
		monitor.applyAvailabilityObservation(target, "edge-fail-"+string(rune('a'+attempt)), now.Add(time.Duration(attempt)*time.Second), 30*time.Millisecond, AvailabilityProbeUnreachable, context.DeadlineExceeded, nil, "edge-1", now.Add(time.Duration(attempt)*time.Second))
	}

	status := monitor.AvailabilityStatusSnapshot()[target.ID]
	if status.AggregateState != AvailabilityAggregateDegraded || !status.Disagreement || !status.Available {
		t.Fatalf("aggregate status = %+v, want available disagreement", status)
	}
	if status.ExpectedLocations != 2 || status.ReportingLocations != 2 || len(status.Locations) != 2 {
		t.Fatalf("location coverage = %+v, want 2/2 locations", status)
	}
	if want := now.Add(time.Second); !status.FreshnessTime().Equal(want) {
		t.Fatalf("aggregate freshness = %s, want latest server-authored path freshness %s", status.FreshnessTime(), want)
	}
	if status.LatencyMillis != 8 {
		t.Fatalf("aggregate latency = %dms, want the current reachable path latency", status.LatencyMillis)
	}
	resource, _ := availabilityResourceFromTarget(target, status, "", now)
	if resource.Status != unifiedresources.StatusWarning || len(resource.Incidents) != 0 {
		t.Fatalf("resource = status %q incidents %+v, one path failure must not author an outage", resource.Status, resource.Incidents)
	}

	for attempt := 0; attempt < 2; attempt++ {
		monitor.applyAvailabilityObservation(target, "local-fail-"+string(rune('a'+attempt)), now.Add(time.Duration(attempt+2)*time.Second), 20*time.Millisecond, AvailabilityProbeUnreachable, context.DeadlineExceeded, nil, "", time.Time{})
	}
	status = monitor.AvailabilityStatusSnapshot()[target.ID]
	if status.AggregateState != AvailabilityAggregateUnavailable || status.Available || status.ConsecutiveFailures != 2 {
		t.Fatalf("aggregate status = %+v, want thresholded all-location outage", status)
	}
	if status.LatencyMillis != 0 {
		t.Fatalf("unavailable aggregate latency = %dms, want no reachable latency", status.LatencyMillis)
	}
	resource, _ = availabilityResourceFromTarget(target, status, "", now.Add(4*time.Second))
	if resource.Status != unifiedresources.StatusOffline || len(resource.Incidents) != 1 {
		t.Fatalf("resource = status %q incidents %+v, want one universal outage", resource.Status, resource.Incidents)
	}
}

func TestMultiLocationAvailabilityTreatsLapsedPathAsUnknownCoverage(t *testing.T) {
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID:                     "service",
		Name:                   "Customer API",
		Address:                "api.service.local",
		Protocol:               config.AvailabilityProbeHTTPS,
		Enabled:                true,
		PollIntervalSecs:       60,
		FailureThreshold:       2,
		ObservationLocationIDs: []string{config.AvailabilityObservationLocationLocal, config.AvailabilityAgentObservationLocationID("edge-1")},
	})
	monitor := newProbeAgentTestMonitor(t, target)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))
	now := time.Now().UTC()
	old := time.Now().UTC().Add(-time.Hour)
	monitor.applyAvailabilityObservation(target, "local-ok", now, 8*time.Millisecond, AvailabilityProbeReachable, nil, nil, "", time.Time{})
	monitor.applyAvailabilityObservation(target, "edge-ok", old, 18*time.Millisecond, AvailabilityProbeReachable, nil, nil, "edge-1", old)

	status := monitor.availabilityStatusSnapshotForTargets([]config.AvailabilityTarget{target}, now)[target.ID]
	if status.AggregateState != AvailabilityAggregateDegraded || status.Disagreement {
		t.Fatalf("aggregate status = %+v, want healthy path plus unknown coverage", status)
	}
	if status.ReportingLocations != 1 || status.ExpectedLocations != 2 {
		t.Fatalf("coverage = %d/%d, want the lapsed path excluded", status.ReportingLocations, status.ExpectedLocations)
	}
	if !status.Locations[1].Stale || status.Locations[1].Outcome != string(AvailabilityProbeIndeterminate) {
		t.Fatalf("remote location = %+v, want stale unknown evidence", status.Locations[1])
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
		"configRevision":      int64(1),
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
	statuses := monitor.AvailabilityStatusSnapshot()
	remote, ok := statuses["remote"]
	if !ok {
		t.Fatal("assigned remote target missing from first-report grace state")
	}
	if !remote.LastChecked.IsZero() || remote.Outcome != "" || remote.Available {
		t.Fatalf("remote status = %+v, want no observation from an agent that owns nothing", remote)
	}
	if _, ok := statuses["local"]; ok {
		t.Fatalf("local status = %+v, want no state from an agent that owns nothing", statuses["local"])
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

func TestAvailabilityHistoryUsesServerReceiptForRemoteResultsAndDeduplicatesRetries(t *testing.T) {
	remoteTarget := probeAgentTarget("remote", "agent-1")
	remoteTarget.ConfigRevision = 3
	monitor := newProbeAgentTestMonitor(t, remoteTarget)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))
	store, err := pkgmetrics.NewStore(pkgmetrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	monitor.metricsStore = store

	receivedAt := time.Now().UTC().Truncate(time.Minute).Add(-10 * time.Minute)
	result := ProbeAvailabilityResult{
		ObservationID: "agent-observation-1", TargetID: "remote", ConfigRevision: 3,
		Outcome: AvailabilityProbeUnreachable, CheckedAt: receivedAt.Add(-24 * time.Hour),
	}
	monitor.applyProbeAvailabilityResultsAt("agent-1", []ProbeAvailabilityResult{result}, receivedAt)
	monitor.applyProbeAvailabilityResultsAt("agent-1", []ProbeAvailabilityResult{result}, receivedAt.Add(time.Second))

	rows, _, _, err := store.AvailabilityHistoryRowCounts("remote")
	if err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("remote observation rows = %d, want one idempotent row", rows)
	}
	history, err := store.QueryAvailabilityHistory(
		[]string{"remote"}, receivedAt.Add(-time.Minute), receivedAt.Add(6*time.Minute), 120,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := history["remote"].Summary.UnreachableSeconds; got != 300 {
		t.Fatalf("remote unreachable duration = %v, want 300 seconds on the server receipt timeline", got)
	}

	stale := result
	stale.ObservationID = "obsolete-revision"
	stale.ConfigRevision = 2
	monitor.applyProbeAvailabilityResultsAt("agent-1", []ProbeAvailabilityResult{stale}, receivedAt.Add(2*time.Second))
	rows, _, _, err = store.AvailabilityHistoryRowCounts("remote")
	if err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("obsolete revision created a history row; rows = %d", rows)
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
	monitor.applyProbeAvailabilityResultsAt("agent-1", []ProbeAvailabilityResult{
		{TargetID: "remote", Outcome: AvailabilityProbeReachable, LatencyMillis: 9, CheckedAt: checkedAt},
	}, checkedAt)

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

func TestAvailabilityProbeStalenessUsesServerReceiptTimeAcrossAgentClockSkew(t *testing.T) {
	target := config.NormalizeAvailabilityTarget(probeAgentTarget("remote", "agent-1"))
	monitor := newProbeAgentTestMonitor(t, target)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))

	receivedAt := time.Now().UTC()
	for _, test := range []struct {
		name      string
		checkedAt time.Time
	}{
		{name: "slow agent clock", checkedAt: receivedAt.Add(-24 * time.Hour)},
		{name: "fast agent clock", checkedAt: receivedAt.Add(24 * time.Hour)},
	} {
		t.Run(test.name, func(t *testing.T) {
			monitor.applyProbeAvailabilityResultsAt("agent-1", []ProbeAvailabilityResult{
				{TargetID: "remote", Outcome: AvailabilityProbeReachable, CheckedAt: test.checkedAt},
			}, receivedAt)

			fresh := monitor.availabilityStatusSnapshotForTargets(
				[]config.AvailabilityTarget{target},
				receivedAt.Add(availabilityProbeStaleFloor),
			)["remote"]
			if !fresh.Available || availabilityProbeStatusIsStale(fresh) {
				t.Fatalf("boundary status = %+v, server receipt must keep the report fresh", fresh)
			}
			if !fresh.LastChecked.Equal(test.checkedAt) {
				t.Fatalf("last checked = %v, want agent observation time %v preserved", fresh.LastChecked, test.checkedAt)
			}
			resource, _ := availabilityResourceFromTarget(target, fresh, "", receivedAt)
			if !resource.LastSeen.Equal(receivedAt) {
				t.Fatalf("resource last seen = %v, want server receipt %v", resource.LastSeen, receivedAt)
			}
			evidence := resource.Availability.Evidence
			if evidence == nil || !evidence.IngestedAt.Equal(receivedAt) {
				t.Fatalf("evidence = %+v, want server receipt as ingest time", evidence)
			}
			wantValidUntil := receivedAt.Add(availabilityProbeStaleWindow(target))
			if evidence.ValidUntil == nil || !evidence.ValidUntil.Equal(wantValidUntil) {
				t.Fatalf("evidence valid until = %v, want %v", evidence.ValidUntil, wantValidUntil)
			}
			if evidence.ObservedAt.After(evidence.IngestedAt) {
				t.Fatalf("evidence chronology = observed %v after ingested %v", evidence.ObservedAt, evidence.IngestedAt)
			}

			stale := monitor.availabilityStatusSnapshotForTargets(
				[]config.AvailabilityTarget{target},
				receivedAt.Add(availabilityProbeStaleFloor+time.Second),
			)["remote"]
			if !availabilityProbeStatusIsStale(stale) {
				t.Fatalf("post-window status = %+v, want stale by server receipt time", stale)
			}
		})
	}
}

func TestAvailabilitySupplementalRecordsPresentStaleProbeAsIndeterminate(t *testing.T) {
	monitor := newProbeAgentTestMonitor(t, probeAgentTarget("remote", "agent-1"))
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))

	old := time.Now().UTC().Add(-time.Hour)
	monitor.applyProbeAvailabilityResultsAt("agent-1", []ProbeAvailabilityResult{
		{TargetID: "remote", Outcome: AvailabilityProbeReachable, LatencyMillis: 9, CheckedAt: old},
	}, old)

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
	if data.ProbeAgentID != "agent-1" {
		t.Fatalf("availability probe agent = %q, want agent-1 so the UI can attribute the source", data.ProbeAgentID)
	}
	if len(records[0].Resource.Incidents) != 0 {
		t.Fatalf("stale target incidents = %+v, probe lifecycle must not be tied to an arbitrary target", records[0].Resource.Incidents)
	}
	if records[0].Resource.Status != unifiedresources.StatusWarning {
		t.Fatalf("stale resource status = %q, want warning", records[0].Resource.Status)
	}

	statuses := availabilityPollProvider{}.ConnectionStatuses(monitor)
	if statuses["availability-remote"] {
		t.Fatalf("connection statuses = %+v, want a stale probe to read as not connected", statuses)
	}
}

func TestAvailabilityProbeFirstReportGraceThenStale(t *testing.T) {
	target := config.NormalizeAvailabilityTarget(probeAgentTarget("remote", "agent-1"))
	monitor := newProbeAgentTestMonitor(t, target)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))

	assignedAt := time.Now().UTC()
	fresh := monitor.availabilityStatusSnapshotForTargets(
		[]config.AvailabilityTarget{target},
		assignedAt,
	)
	if availabilityProbeStatusIsStale(fresh["remote"]) {
		t.Fatalf("new assignment status = %+v, must receive the first-report grace window", fresh["remote"])
	}

	stale := monitor.availabilityStatusSnapshotForTargets(
		[]config.AvailabilityTarget{target},
		assignedAt.Add(availabilityProbeStaleFloor+time.Second),
	)
	if !availabilityProbeStatusIsStale(stale["remote"]) {
		t.Fatalf("post-grace assignment status = %+v, want stale without a first report", stale["remote"])
	}
	if stale["remote"].ProbeAgentID != "agent-1" {
		t.Fatalf("post-grace probe agent = %q, want agent-1", stale["remote"].ProbeAgentID)
	}
}

func TestAvailabilityProbeReassignmentReceivesFreshGrace(t *testing.T) {
	target := config.NormalizeAvailabilityTarget(probeAgentTarget("remote", "agent-2"))
	monitor := newProbeAgentTestMonitor(t, target)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))
	old := time.Now().UTC().Add(-time.Hour)
	monitor.availabilityStatuses["remote"] = AvailabilityProbeStatus{
		TargetID:     "remote",
		ProbeAgentID: "agent-1",
		LastChecked:  old,
		Outcome:      string(AvailabilityProbeReachable),
		Available:    true,
	}

	reassignedAt := time.Now().UTC()
	statuses := monitor.availabilityStatusSnapshotForTargets(
		[]config.AvailabilityTarget{target},
		reassignedAt,
	)
	status := statuses["remote"]
	if availabilityProbeStatusIsStale(status) {
		t.Fatalf("reassigned status = %+v, must receive a fresh first-report grace window", status)
	}
	if status.ProbeAgentID != "agent-2" {
		t.Fatalf("reassigned probe agent = %q, want agent-2", status.ProbeAgentID)
	}
	if !status.LastChecked.IsZero() || status.Outcome != "" || status.Available {
		t.Fatalf("reassigned status = %+v, must not present the old probe's observation as current", status)
	}
}

func TestAvailabilityProbeStaleIncidentDeduplicatesPerAgent(t *testing.T) {
	monitor := newProbeAgentTestMonitor(t,
		probeAgentTarget("remote-a", "agent-1"),
		probeAgentTarget("remote-b", "agent-1"),
		probeAgentTarget("remote-c", "agent-2"),
	)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))
	alertManager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(alertManager.Stop)
	monitor.alertManager = alertManager
	old := time.Now().UTC().Add(-time.Hour)
	monitor.applyProbeAvailabilityResultsAt("agent-1", []ProbeAvailabilityResult{
		{TargetID: "remote-a", Outcome: AvailabilityProbeReachable, CheckedAt: old},
		{TargetID: "remote-b", Outcome: AvailabilityProbeReachable, CheckedAt: old},
	}, old)
	monitor.applyProbeAvailabilityResultsAt("agent-2", []ProbeAvailabilityResult{
		{TargetID: "remote-c", Outcome: AvailabilityProbeReachable, CheckedAt: old},
	}, old)

	_ = (availabilityPollProvider{}).SupplementalRecords(monitor, "")
	alertsByAgent := make(map[string]int)
	for _, alert := range alertManager.GetActiveAlerts() {
		if alert.Type != alerts.ExternalProbeUnavailableAlertType {
			continue
		}
		agentID, _ := alert.Metadata["hostId"].(string)
		alertsByAgent[agentID]++
	}
	if alertsByAgent["agent-1"] != 1 || alertsByAgent["agent-2"] != 1 {
		t.Fatalf("stale alerts by agent = %+v, want exactly one per disconnected probe", alertsByAgent)
	}
}

func TestAvailabilityProbeFreshReportResolvesStaleIncident(t *testing.T) {
	monitor := newProbeAgentTestMonitor(t, probeAgentTarget("remote", "agent-1"))
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))
	alertManager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(alertManager.Stop)
	monitor.alertManager = alertManager
	old := time.Now().UTC().Add(-time.Hour)
	monitor.applyProbeAvailabilityResultsAt("agent-1", []ProbeAvailabilityResult{
		{TargetID: "remote", Outcome: AvailabilityProbeReachable, CheckedAt: old},
	}, old)
	_ = (availabilityPollProvider{}).SupplementalRecords(monitor, "")
	if got := alertManager.GetActiveAlerts(); len(got) != 1 || got[0].Type != alerts.ExternalProbeUnavailableAlertType {
		t.Fatalf("stale alerts = %+v, want one before recovery", got)
	}

	monitor.ApplyProbeAvailabilityResults("agent-1", []ProbeAvailabilityResult{
		{TargetID: "remote", Outcome: AvailabilityProbeReachable, CheckedAt: time.Now().UTC()},
	})
	recovered := (availabilityPollProvider{}).SupplementalRecords(monitor, "")[0].Resource
	if got := alertManager.GetActiveAlerts(); len(got) != 0 {
		t.Fatalf("active alerts after recovery = %+v, want none", got)
	}
	if recovered.Status != "online" {
		t.Fatalf("recovered resource = %+v, want online", recovered)
	}
}

func TestAvailabilityProbeHostOfflineOwnsConnectivityLifecycle(t *testing.T) {
	monitor := newProbeAgentTestMonitor(t, probeAgentTarget("remote", "agent-1"))
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))
	alertManager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(alertManager.Stop)
	monitor.alertManager = alertManager
	monitor.state.UpsertHost(models.Host{
		ID:              "agent-1",
		Hostname:        "probe.local",
		LastSeen:        time.Now().UTC().Add(-time.Hour),
		IntervalSeconds: 30,
	})
	old := time.Now().UTC().Add(-time.Hour)
	monitor.applyProbeAvailabilityResultsAt("agent-1", []ProbeAvailabilityResult{
		{TargetID: "remote", Outcome: AvailabilityProbeReachable, CheckedAt: old},
	}, old)

	_ = (availabilityPollProvider{}).SupplementalRecords(monitor, "")
	for _, alert := range alertManager.GetActiveAlerts() {
		if alert.Type == alerts.ExternalProbeUnavailableAlertType {
			t.Fatalf("probe-specific alert duplicated host-offline lifecycle: %+v", alert)
		}
	}
}

func TestHasExternalProbeAssignmentsRequiresEnabledLicensedTarget(t *testing.T) {
	enabled := probeAgentTarget("enabled", "agent-1")
	disabled := probeAgentTarget("disabled", "agent-2")
	disabled.Enabled = false
	monitor := newProbeAgentTestMonitor(t, enabled, disabled)

	monitor.SetLicenseChecker(licenseWithExternalProbe(true))
	if !monitor.HasExternalProbeAssignments("agent-1") {
		t.Fatal("licensed agent with an enabled assignment was not recognized")
	}
	if monitor.HasExternalProbeAssignments("agent-2") {
		t.Fatal("disabled assignment was treated as an active external probe")
	}

	monitor.SetLicenseChecker(licenseWithExternalProbe(false))
	if monitor.HasExternalProbeAssignments("agent-1") {
		t.Fatal("lapsed entitlement retained external probe notification routing")
	}
}

func TestApplyHostReportIngestsAssignedAvailabilityResults(t *testing.T) {
	monitor := newProbeAgentTestMonitor(t,
		probeAgentTarget("remote", "probe-host"),
		probeAgentTarget("foreign", "other-agent"),
	)
	monitor.alertManager = alerts.NewManager()
	t.Cleanup(func() { monitor.alertManager.Stop() })
	monitor.config = &config.Config{}
	monitor.rateTracker = NewRateTracker()
	monitor.hostTokenBindings = make(map[string]string)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))

	checkedAt := time.Now().UTC()
	host, err := monitor.ApplyHostReport(agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "probe-host", Version: "6.0.0", IntervalSeconds: 30},
		Host: agentshost.HostInfo{
			ID:       "probe-host",
			Hostname: "probe-host.local",
			Platform: "linux",
		},
		AvailabilityResults: []agentshost.AvailabilityProbeResult{
			{TargetID: "remote", Outcome: "reachable", TransportOutcome: "reachable", ApplicationOutcome: "passed", ApplicationStatusCode: 204, LatencyMillis: 17, CheckedAt: checkedAt},
			{TargetID: "foreign", Outcome: "reachable", LatencyMillis: 3, CheckedAt: checkedAt},
		},
		Timestamp: checkedAt,
	}, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport() error = %v", err)
	}
	// The ownership check compares the host ID that GetHostAgentConfig is
	// keyed by, so the report must resolve to exactly that identity.
	if host.ID != "probe-host" {
		t.Fatalf("host ID = %q, want the assigned probe agent ID", host.ID)
	}

	statuses := monitor.AvailabilityStatusSnapshot()
	status, ok := statuses["remote"]
	if !ok {
		t.Fatalf("statuses = %+v, want the assigned target applied", statuses)
	}
	if !status.Available || status.LatencyMillis != 17 {
		t.Fatalf("status = %+v, want the reported observation", status)
	}
	if status.ProbeAgentID != "probe-host" {
		t.Fatalf("probe agent attribution = %q, want probe-host", status.ProbeAgentID)
	}
	if status.TransportOutcome != "reachable" || status.ApplicationOutcome != "passed" || status.ApplicationStatusCode != 204 {
		t.Fatalf("application evidence = %+v, want reachable transport and passed HTTP 204", status)
	}
	if !status.LastChecked.Equal(checkedAt) {
		t.Fatalf("last checked = %v, want %v", status.LastChecked, checkedAt)
	}
	foreign, ok := statuses["foreign"]
	if !ok {
		t.Fatal("foreign assigned target missing from first-report grace state")
	}
	if !foreign.LastChecked.IsZero() || foreign.Outcome != "" || foreign.Available {
		t.Fatalf("foreign status = %+v, a report claimed a target assigned to another agent", foreign)
	}
}

func TestProbeAvailabilityResultsFromReportNormalizesOutcomes(t *testing.T) {
	if got := probeAvailabilityResultsFromReport(nil); got != nil {
		t.Fatalf("results = %+v, want nil for a report without probe work", got)
	}

	checkedAt := time.Now().UTC()
	reportedCertificate := &tlsutil.CertificateObservation{Subject: "pulse.example.test", DNSNames: []string{"pulse.example.test"}}
	results := probeAvailabilityResultsFromReport([]agentshost.AvailabilityProbeResult{
		{TargetID: " padded ", Outcome: " REACHABLE ", LatencyMillis: 4, CheckedAt: checkedAt, Certificate: reportedCertificate},
		{TargetID: "b", Outcome: "unreachable", Error: " icmp probe timed out "},
		{TargetID: "c", Outcome: "who-knows"},
		{TargetID: "d", Outcome: ""},
	})
	if len(results) != 4 {
		t.Fatalf("results = %+v, want one per reported observation", results)
	}
	if results[0].TargetID != "padded" || results[0].Outcome != AvailabilityProbeReachable {
		t.Fatalf("first result = %+v", results[0])
	}
	if results[0].Certificate == nil || results[0].Certificate.Subject != "pulse.example.test" {
		t.Fatalf("first certificate = %+v", results[0].Certificate)
	}
	results[0].Certificate.DNSNames[0] = "changed.example.test"
	if reportedCertificate.DNSNames[0] != "pulse.example.test" {
		t.Fatal("wire certificate was not cloned")
	}
	if results[1].Error != "icmp probe timed out" || results[1].Outcome != AvailabilityProbeUnreachable {
		t.Fatalf("second result = %+v", results[1])
	}
	for _, result := range results[2:] {
		if result.Outcome != AvailabilityProbeIndeterminate {
			t.Fatalf("result %+v: unknown outcomes must read as indeterminate", result)
		}
	}
}
