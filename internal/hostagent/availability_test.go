package hostagent

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/availabilityprobe"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

func testProbeModule(t *testing.T, probe func(context.Context, config.AvailabilityTarget) (availabilityprobe.Outcome, error)) *availabilityProbeModule {
	t.Helper()
	logger := zerolog.New(io.Discard)
	module := newAvailabilityProbeModule(logger, nil)
	if probe != nil {
		module.probe = probe
	}
	return module
}

func availabilitySetting(entries ...map[string]interface{}) map[string]interface{} {
	values := make([]interface{}, 0, len(entries))
	for _, entry := range entries {
		values = append(values, entry)
	}
	return map[string]interface{}{availabilitySettingsKey: values}
}

func TestAvailabilityTargetsFromSettingsDecodesServerPayload(t *testing.T) {
	// Mirrors the payload shape built by the server for an assigned agent,
	// including a key the agent does not know about.
	settings := availabilitySetting(map[string]interface{}{
		"id":                  "udp-check",
		"name":                "UDP check",
		"targetKind":          "device",
		"address":             "sensor.local",
		"protocol":            "udp",
		"port":                float64(5353),
		"udpMode":             "response-required",
		"udpRequest":          "ping",
		"udpExpectedResponse": "pong",
		"enabled":             true,
		"pollIntervalSeconds": float64(45),
		"timeoutMillis":       float64(1500),
		"unknownFutureField":  "ignored",
	})

	targets, err := availabilityTargetsFromSettings(settings)
	if err != nil {
		t.Fatalf("availabilityTargetsFromSettings() error = %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("targets = %+v, want one assignment", targets)
	}
	target := targets[0]
	if target.ID != "udp-check" || target.Address != "sensor.local" {
		t.Fatalf("target = %+v", target)
	}
	if target.Protocol != config.AvailabilityProbeUDP || target.Port != 5353 {
		t.Fatalf("target protocol/port = %v/%d", target.Protocol, target.Port)
	}
	if target.UDPRequest != "ping" || target.UDPExpected != "pong" {
		t.Fatalf("target udp payloads = %q/%q", target.UDPRequest, target.UDPExpected)
	}
	if target.EffectivePollIntervalSecs() != 45 || target.EffectiveTimeoutMillis() != 1500 {
		t.Fatalf("target schedule = %ds/%dms", target.EffectivePollIntervalSecs(), target.EffectiveTimeoutMillis())
	}
	if !target.Enabled {
		t.Fatal("target should be enabled")
	}
	// Server-only concerns are never carried in an agent assignment.
	if target.ProbeAgentID != "" || target.LinkedResourceID != "" {
		t.Fatalf("target carried server-side fields: %+v", target)
	}
}

func TestAvailabilityTargetsFromSettingsHandlesMissingAndUnusableValues(t *testing.T) {
	targets, err := availabilityTargetsFromSettings(nil)
	if err != nil || targets != nil {
		t.Fatalf("nil settings = (%+v, %v), want no assignments", targets, err)
	}

	targets, err = availabilityTargetsFromSettings(map[string]interface{}{"interval": "30s"})
	if err != nil || targets != nil {
		t.Fatalf("settings without the key = (%+v, %v), want no assignments", targets, err)
	}

	targets, err = availabilityTargetsFromSettings(map[string]interface{}{availabilitySettingsKey: nil})
	if err != nil || targets != nil {
		t.Fatalf("null value = (%+v, %v), want no assignments", targets, err)
	}

	if _, err := availabilityTargetsFromSettings(map[string]interface{}{availabilitySettingsKey: "not-a-list"}); err == nil {
		t.Fatal("garbage value error = nil, want a decode failure")
	}

	// An entry without an ID cannot be reported against and is dropped.
	targets, err = availabilityTargetsFromSettings(availabilitySetting(
		map[string]interface{}{"address": "orphan.local", "protocol": "icmp", "enabled": true},
		map[string]interface{}{"id": "keep", "address": "keep.local", "protocol": "icmp", "enabled": true},
	))
	if err != nil {
		t.Fatalf("availabilityTargetsFromSettings() error = %v", err)
	}
	if len(targets) != 1 || targets[0].ID != "keep" {
		t.Fatalf("targets = %+v, want only the identified assignment", targets)
	}
}

func TestAvailabilityTargetsFromSettingsPreservesHTTPExecutionSecrets(t *testing.T) {
	settings := availabilitySetting(map[string]interface{}{
		"id": "http-contract", "address": "https://service.local/health", "protocol": "https", "enabled": true,
		"http": map[string]interface{}{
			"method": "POST", "body": `{"operation":"health"}`,
			"expectedStatusMin": float64(200), "expectedStatusMax": float64(299),
			"authentication": map[string]interface{}{"type": "bearer", "bearerToken": "agent-secret-token"},
			"headers":        []interface{}{map[string]interface{}{"id": "tenant", "name": "X-Tenant", "value": "tenant-a"}},
			"jsonPath":       "status", "jsonEquals": "healthy",
		},
	})

	targets, err := availabilityTargetsFromSettings(settings)
	if err != nil || len(targets) != 1 || targets[0].HTTP == nil {
		t.Fatalf("availabilityTargetsFromSettings() = %+v, %v", targets, err)
	}
	contract := targets[0].HTTP
	if contract.Body == nil || *contract.Body != `{"operation":"health"}` ||
		contract.Authentication.BearerToken == nil || *contract.Authentication.BearerToken != "agent-secret-token" ||
		len(contract.Headers) != 1 || contract.Headers[0].Value == nil || *contract.Headers[0].Value != "tenant-a" {
		t.Fatalf("decoded HTTP contract = %+v, want complete execution values", contract)
	}

	decodedAgain, err := availabilityTargetsFromSettings(settings)
	if err != nil || !availabilityAssignmentsEqual(targets, decodedAgain) {
		t.Fatalf("identical pointer-backed HTTP assignments were reported as changed: %+v, %v", decodedAgain, err)
	}
}

func TestApplyRemoteAvailabilityTargetsReconcilesAssignments(t *testing.T) {
	agent := &Agent{
		logger:       zerolog.New(io.Discard),
		availability: testProbeModule(t, nil),
	}

	assigned := availabilitySetting(map[string]interface{}{
		"id": "remote-a", "address": "a.local", "protocol": "icmp", "enabled": true,
	})
	agent.ApplyRemoteConfig(assigned, nil)
	if got := agent.availability.assignments(); len(got) != 1 || got[0].ID != "remote-a" {
		t.Fatalf("assignments = %+v, want remote-a", got)
	}

	// A repeated fetch of the same assignment must not restart the schedule.
	if agent.availability.applyTargets(agent.availability.assignments()) {
		t.Fatal("unchanged assignments reported a schedule change")
	}

	// An unreadable payload leaves the current schedule alone.
	agent.ApplyRemoteConfig(map[string]interface{}{availabilitySettingsKey: "not-a-list"}, nil)
	if got := agent.availability.assignments(); len(got) != 1 {
		t.Fatalf("assignments after a garbage payload = %+v, want the previous schedule", got)
	}

	// The server omits the key once nothing is assigned: that unassigns.
	agent.ApplyRemoteConfig(map[string]interface{}{"interval": "30s"}, nil)
	if got := agent.availability.assignments(); len(got) != 0 {
		t.Fatalf("assignments after unassignment = %+v, want none", got)
	}
}

func TestNewSeedsAvailabilityAssignmentsFromStartupConfig(t *testing.T) {
	recorder := &availabilityReportRecorder{}
	agent := newAvailabilityDeliveryAgent(t, recorder, config.AvailabilityTarget{
		ID:       "boot-target",
		Address:  "boot.local",
		Protocol: config.AvailabilityProbeICMP,
		Enabled:  true,
	})

	// The assignment fetched before startup must be live without waiting for
	// the first remote-config refresh.
	if got := agent.availability.assignments(); len(got) != 1 || got[0].ID != "boot-target" {
		t.Fatalf("assignments = %+v, want the startup assignment", got)
	}
	status, ok := agent.availability.moduleStatus()
	if !ok || status.Name != availabilityModuleName {
		t.Fatalf("module status = (%+v, %v), want the availability module", status, ok)
	}
}

func TestAvailabilityProbeIntervalClampsToServerBounds(t *testing.T) {
	tests := []struct {
		seconds int
		want    time.Duration
	}{
		{seconds: 0, want: time.Duration(config.DefaultAvailabilityPollIntervalSecs) * time.Second},
		{seconds: 1, want: availabilityMinInterval},
		{seconds: 45, want: 45 * time.Second},
		{seconds: 86400, want: availabilityMaxInterval},
	}
	for _, test := range tests {
		target := config.AvailabilityTarget{PollIntervalSecs: test.seconds}
		if got := availabilityProbeInterval(target); got != test.want {
			t.Fatalf("availabilityProbeInterval(%ds) = %v, want %v", test.seconds, got, test.want)
		}
	}
}

func TestAvailabilityModuleSchedulerRunsAssignedTargetsAndReschedules(t *testing.T) {
	var (
		mu      sync.Mutex
		checked []string
	)
	module := testProbeModule(t, func(_ context.Context, target config.AvailabilityTarget) (availabilityprobe.Outcome, error) {
		mu.Lock()
		checked = append(checked, target.ID)
		mu.Unlock()
		if target.ID == "down" {
			return availabilityprobe.OutcomeUnreachable, errors.New("icmp probe timed out")
		}
		return availabilityprobe.OutcomeReachable, nil
	})
	module.applyTargets([]config.AvailabilityTarget{
		{ID: "up", Address: "up.local", Protocol: config.AvailabilityProbeICMP, Enabled: true},
		{ID: "down", Address: "down.local", Protocol: config.AvailabilityProbeICMP, Enabled: true},
		{ID: "paused", Address: "paused.local", Protocol: config.AvailabilityProbeICMP, Enabled: false},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		module.Run(ctx)
	}()

	waitForPending := func(want int) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			if module.pending.Len() >= want {
				return
			}
			select {
			case <-time.After(5 * time.Millisecond):
			case <-deadline:
				t.Fatalf("timed out waiting for %d queued results, have %d", want, module.pending.Len())
			}
		}
	}

	// Enabled targets probe immediately rather than after a full interval.
	waitForPending(2)
	results := module.snapshotForReport()
	if len(results) != 2 {
		t.Fatalf("results = %+v, want one per enabled target", results)
	}
	byTarget := make(map[string]agentshost.AvailabilityProbeResult, len(results))
	for _, result := range results {
		byTarget[result.TargetID] = result
	}
	if got := byTarget["up"].Outcome; got != string(availabilityprobe.OutcomeReachable) {
		t.Fatalf("up outcome = %q", got)
	}
	if got := byTarget["down"]; got.Outcome != string(availabilityprobe.OutcomeUnreachable) || got.Error == "" {
		t.Fatalf("down result = %+v, want a failure with an explanation", got)
	}
	if byTarget["up"].CheckedAt.IsZero() {
		t.Fatal("result is missing its observation time")
	}
	if _, ok := byTarget["paused"]; ok {
		t.Fatalf("disabled target was probed: %+v", results)
	}

	module.commitDelivered()
	if module.pending.Len() != 0 {
		t.Fatalf("pending = %d after delivery, want 0", module.pending.Len())
	}

	// Reassignment stops the old workers and starts the new set at once.
	mu.Lock()
	checked = nil
	mu.Unlock()
	module.applyTargets([]config.AvailabilityTarget{
		{ID: "replacement", Address: "new.local", Protocol: config.AvailabilityProbeICMP, Enabled: true},
	})
	waitForPending(1)
	reassigned := module.snapshotForReport()
	if len(reassigned) != 1 || reassigned[0].TargetID != "replacement" {
		t.Fatalf("results after reassignment = %+v, want only the replacement", reassigned)
	}
	mu.Lock()
	for _, id := range checked {
		if id != "replacement" {
			t.Fatalf("target %q ran after it was unassigned", id)
		}
	}
	mu.Unlock()

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not stop on context cancellation")
	}
}

func TestAvailabilityModuleQueueDropsOldestBeyondCapacity(t *testing.T) {
	module := testProbeModule(t, nil)
	for i := 0; i < availabilityPendingCapacity+50; i++ {
		module.enqueue(agentshost.AvailabilityProbeResult{
			TargetID:  "target",
			Outcome:   string(availabilityprobe.OutcomeReachable),
			CheckedAt: time.Unix(int64(i), 0).UTC(),
		})
	}

	if got := module.pending.Len(); got != availabilityPendingCapacity {
		t.Fatalf("pending = %d, want the capacity %d", got, availabilityPendingCapacity)
	}
	results := module.snapshotForReport()
	if len(results) != availabilityPendingCapacity {
		t.Fatalf("results = %d, want %d", len(results), availabilityPendingCapacity)
	}
	// The oldest 50 observations were dropped, not the newest.
	if got := results[0].CheckedAt; !got.Equal(time.Unix(50, 0).UTC()) {
		t.Fatalf("oldest retained result = %v, want the 51st observation", got)
	}
	if got := results[len(results)-1].CheckedAt; !got.Equal(time.Unix(availabilityPendingCapacity+49, 0).UTC()) {
		t.Fatalf("newest retained result = %v", got)
	}
}

func TestAvailabilityModuleConcurrentEnqueuePreservesSequenceOrder(t *testing.T) {
	module := testProbeModule(t, nil)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < availabilityPendingCapacity*4; i++ {
		workers.Add(1)
		go func(id int) {
			defer workers.Done()
			<-start
			module.enqueue(agentshost.AvailabilityProbeResult{TargetID: fmt.Sprintf("target-%d", id)})
		}(i)
	}
	close(start)
	workers.Wait()

	items := module.pending.Items()
	if len(items) != availabilityPendingCapacity {
		t.Fatalf("queued results = %d, want capacity %d", len(items), availabilityPendingCapacity)
	}
	for i := 1; i < len(items); i++ {
		if items[i].sequence <= items[i-1].sequence {
			t.Fatalf("queue sequence is not increasing at %d: %d followed %d", i, items[i].sequence, items[i-1].sequence)
		}
	}
}

func TestAvailabilityModuleStatusOnlyWhileAssigned(t *testing.T) {
	module := testProbeModule(t, nil)
	if _, ok := module.moduleStatus(); ok {
		t.Fatal("module status reported without an assignment")
	}

	module.applyTargets([]config.AvailabilityTarget{
		{ID: "remote", Address: "remote.local", Protocol: config.AvailabilityProbeICMP, Enabled: true},
	})
	status, ok := module.moduleStatus()
	if !ok {
		t.Fatal("module status missing for an assigned agent")
	}
	if status.Name != availabilityModuleName || !status.Enabled {
		t.Fatalf("status = %+v", status)
	}
	if status.State != availabilityModuleStateStarting {
		t.Fatalf("state = %q before the scheduler runs, want %q", status.State, availabilityModuleStateStarting)
	}
	if status.UpdatedAt.IsZero() {
		t.Fatal("status is missing its assignment time")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		module.Run(ctx)
	}()
	deadline := time.After(5 * time.Second)
	for {
		status, _ = module.moduleStatus()
		if status.State == availabilityModuleStateRunning {
			break
		}
		select {
		case <-time.After(5 * time.Millisecond):
		case <-deadline:
			t.Fatalf("state = %q, want %q once the scheduler is up", status.State, availabilityModuleStateRunning)
		}
	}
	cancel()
	<-done

	module.applyTargets(nil)
	if _, ok := module.moduleStatus(); ok {
		t.Fatal("module status survived unassignment")
	}
}

func TestAgentModuleStatusAppendsAvailabilityToRuntimeModules(t *testing.T) {
	agent := &Agent{
		logger:       zerolog.New(io.Discard),
		availability: testProbeModule(t, nil),
		cfg: Config{
			ModuleStatus: func() []agentshost.ModuleStatus {
				return []agentshost.ModuleStatus{{Name: "host", Enabled: true, State: "running"}}
			},
		},
	}

	statuses := agent.currentModuleStatus()
	if len(statuses) != 1 || statuses[0].Name != "host" {
		t.Fatalf("modules = %+v, want only the runtime modules", statuses)
	}

	agent.availability.applyTargets([]config.AvailabilityTarget{
		{ID: "remote", Address: "remote.local", Protocol: config.AvailabilityProbeICMP, Enabled: true},
	})
	statuses = agent.currentModuleStatus()
	if len(statuses) != 2 || statuses[1].Name != availabilityModuleName {
		t.Fatalf("modules = %+v, want the availability module appended", statuses)
	}
}

// availabilityReportRecorder is a Pulse report endpoint that can be switched
// into failure so retention across a delivery outage is observable.
type availabilityReportRecorder struct {
	mu      sync.Mutex
	reports []agentshost.Report
	fail    bool
}

func (r *availabilityReportRecorder) setFail(fail bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fail = fail
}

func (r *availabilityReportRecorder) received() []agentshost.Report {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]agentshost.Report(nil), r.reports...)
}

func (r *availabilityReportRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	fail := r.fail
	r.mu.Unlock()
	if fail {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var body io.Reader = req.Body
	if req.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer gz.Close()
		body = gz
	}
	var report agentshost.Report
	if err := json.NewDecoder(body).Decode(&report); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	r.mu.Lock()
	r.reports = append(r.reports, report)
	r.mu.Unlock()
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}

func newAvailabilityDeliveryAgent(t *testing.T, recorder *availabilityReportRecorder, targets ...config.AvailabilityTarget) *Agent {
	t.Helper()
	server := httptest.NewServer(recorder)
	t.Cleanup(server.Close)

	logger := zerolog.New(io.Discard)
	agent, err := New(Config{
		PulseURL:            server.URL,
		APIToken:            "test-token",
		Interval:            time.Minute,
		HostnameOverride:    "probe-agent",
		StateDir:            t.TempDir(),
		Logger:              &logger,
		Collector:           &mockCollector{},
		AvailabilityTargets: targets,
		packageUpdates:      newPackageUpdateManager("windows", nil),
		storageCleanup:      newStorageCleanupManager("windows", nil),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return agent
}

func TestAvailabilityResultsSurviveFailedDeliveryAndAreSentOnce(t *testing.T) {
	recorder := &availabilityReportRecorder{}
	agent := newAvailabilityDeliveryAgent(t, recorder)
	ctx := context.Background()

	for _, id := range []string{"target-a", "target-b"} {
		agent.availability.enqueue(agentshost.AvailabilityProbeResult{
			TargetID:  id,
			Outcome:   string(availabilityprobe.OutcomeReachable),
			CheckedAt: time.Now().UTC(),
		})
	}

	recorder.setFail(true)
	if err := agent.process(ctx); err != nil {
		t.Fatalf("process() during outage error = %v", err)
	}
	if got := agent.availability.pending.Len(); got != 2 {
		t.Fatalf("pending = %d after a failed delivery, want the results retained", got)
	}
	buffered, ok := agent.reportBuffer.Peek()
	if !ok {
		t.Fatal("failed report was not buffered")
	}
	if len(buffered.AvailabilityResults) != 0 {
		t.Fatalf("buffered report kept %d results; a retry would double count them", len(buffered.AvailabilityResults))
	}

	// A result observed during the outage joins the same batch.
	agent.availability.enqueue(agentshost.AvailabilityProbeResult{
		TargetID:  "target-c",
		Outcome:   string(availabilityprobe.OutcomeUnreachable),
		CheckedAt: time.Now().UTC(),
		Error:     "icmp probe timed out",
	})

	recorder.setFail(false)
	if err := agent.process(ctx); err != nil {
		t.Fatalf("process() after recovery error = %v", err)
	}
	if got := agent.availability.pending.Len(); got != 0 {
		t.Fatalf("pending = %d after a successful delivery, want 0", got)
	}

	// A later report must not repeat what the server already counted.
	if err := agent.process(ctx); err != nil {
		t.Fatalf("process() error = %v", err)
	}

	delivered := map[string]int{}
	for _, report := range recorder.received() {
		for _, result := range report.AvailabilityResults {
			delivered[result.TargetID]++
		}
	}
	if len(delivered) != 3 {
		t.Fatalf("delivered targets = %+v, want all three observations", delivered)
	}
	for id, count := range delivered {
		if count != 1 {
			t.Fatalf("target %q was delivered %d times, want exactly once", id, count)
		}
	}
}

func TestAvailabilityResultsAreNotRetiredWhenTheTokenIsRejected(t *testing.T) {
	recorder := &availabilityReportRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	agent := newAvailabilityDeliveryAgent(t, recorder)
	agent.trimmedPulseURL = server.URL
	agent.availability.enqueue(agentshost.AvailabilityProbeResult{
		TargetID:  "target-a",
		Outcome:   string(availabilityprobe.OutcomeReachable),
		CheckedAt: time.Now().UTC(),
	})

	// A rejected token drops the report instead of buffering it, so the
	// results have to stay queued for whenever the token is replaced.
	if err := agent.process(context.Background()); err != nil {
		t.Fatalf("process() error = %v", err)
	}
	if got := agent.availability.pending.Len(); got != 1 {
		t.Fatalf("pending = %d after a rejected report, want the result retained", got)
	}
	if !agent.reportBuffer.IsEmpty() {
		t.Fatal("a rejected report should not be buffered")
	}
}
