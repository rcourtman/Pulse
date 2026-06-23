package ai

import "testing"

func TestFindingsStore_AddRecordsDetectedLifecycleEvent(t *testing.T) {
	store := NewFindingsStore()
	f := &Finding{
		ID:           "lf-1",
		ResourceID:   "node-1",
		ResourceName: "node-1",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		Title:        "High CPU usage trend",
		Description:  "CPU trend indicates sustained pressure",
	}

	if !store.Add(f) {
		t.Fatal("expected first add to create a finding")
	}

	got := store.Get("lf-1")
	if got == nil {
		t.Fatal("expected finding to exist")
	}
	if got.TimesRaised != 1 {
		t.Fatalf("expected timesRaised=1, got %d", got.TimesRaised)
	}
	if len(got.Lifecycle) == 0 {
		t.Fatal("expected lifecycle events to be recorded")
	}
	last := got.Lifecycle[len(got.Lifecycle)-1]
	if last.Type != "detected" {
		t.Fatalf("expected last lifecycle event type=detected, got %q", last.Type)
	}
}

func TestFindingsStore_RedetectionDoesNotAppendHeartbeatLifecycleEvent(t *testing.T) {
	store := NewFindingsStore()
	f := &Finding{
		ID:           "lf-heartbeat",
		ResourceID:   "host-runtime-error",
		ResourceName: "host-runtime-error",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
		Title:        "Provider analysis error",
		Description:  "Pulse Patrol reached the configured provider, but the provider did not complete the request.",
	}
	if !store.Add(f) {
		t.Fatal("expected first add to create finding")
	}
	initialLen := len(store.Get(f.ID).Lifecycle)
	if initialLen == 0 {
		t.Fatal("expected first add to record at least one lifecycle event")
	}

	// Simulate three additional Patrol scans re-detecting the same active
	// finding. None of these are state transitions — TimesRaised and
	// LastSeenAt should still update, but no new lifecycle events should
	// be appended (the lifecycle records transitions, not heartbeats).
	for i := 0; i < 3; i++ {
		// Add returns false for existing-finding updates (it returns true
		// only when a new finding is created); the test asserts that
		// behaviour so a regression in the existing/new branch surfaces.
		if store.Add(&Finding{
			ID:           f.ID,
			ResourceID:   "host-runtime-error",
			ResourceName: "host-runtime-error",
			Severity:     FindingSeverityWarning,
			Category:     FindingCategoryReliability,
			Title:        "Provider analysis error",
			Description:  "Pulse Patrol reached the configured provider, but the provider did not complete the request.",
		}) {
			t.Fatalf("expected re-detection %d to update existing (return false), not create new", i+1)
		}
	}

	got := store.Get(f.ID)
	if got.TimesRaised != 1+3 {
		t.Fatalf("expected timesRaised=4 after three re-detections, got %d", got.TimesRaised)
	}
	if len(got.Lifecycle) != initialLen {
		t.Fatalf("expected lifecycle length to remain %d after heartbeat re-detections, got %d (events: %+v)",
			initialLen, len(got.Lifecycle), got.Lifecycle)
	}
}

func TestFindingsStore_TransitionDoesNotAlsoEmitGenericLoopStateEvent(t *testing.T) {
	// Every semantic transition (resolved, auto_resolved, regressed, dismissed,
	// acknowledged, snoozed, etc.) is emitted by its own caller before
	// syncLoopStateLocked runs. The loop-state sync used to ALSO emit a
	// generic "loop_state" event with the same from/to, doubling every row in
	// the finding's lifecycle drawer. This test locks in: a single transition
	// produces a single semantic lifecycle event, never a paired "loop_state"
	// duplicate. (Invalid transitions still emit "loop_transition_violation"
	// — that's a guard signal, not a duplicate; covered by a separate test.)
	store := NewFindingsStore()
	f := &Finding{
		ID:           "lf-no-loop-state-dup",
		ResourceID:   "vm-555",
		ResourceName: "web-555",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
		Title:        "Service crashed",
		Description:  "Service exited unexpectedly",
	}
	if !store.Add(f) {
		t.Fatal("expected first add to create finding")
	}
	if !store.Resolve(f.ID, true) {
		t.Fatal("expected auto-resolve to succeed")
	}

	got := store.Get(f.ID)
	if got == nil {
		t.Fatal("expected finding to exist after auto-resolve")
	}
	for _, e := range got.Lifecycle {
		if e.Type == "loop_state" {
			t.Fatalf("did not expect a generic loop_state event after a semantic transition; lifecycle: %+v", got.Lifecycle)
		}
	}
	// Sanity: the auto_resolved semantic event is still recorded.
	foundAutoResolved := false
	for _, e := range got.Lifecycle {
		if e.Type == "auto_resolved" {
			foundAutoResolved = true
			break
		}
	}
	if !foundAutoResolved {
		t.Fatalf("expected auto_resolved lifecycle event; got: %+v", got.Lifecycle)
	}
}

func TestFindingsStore_RegressionIncrementsAndRecordsLifecycleEvent(t *testing.T) {
	store := NewFindingsStore()
	f := &Finding{
		ID:           "lf-2",
		ResourceID:   "vm-101",
		ResourceName: "web",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
		Title:        "Restart loop",
		Description:  "Service repeatedly restarts",
	}
	if !store.Add(f) {
		t.Fatal("expected first add to create finding")
	}
	if !store.Resolve("lf-2", false) {
		t.Fatal("expected resolve to succeed")
	}
	if store.Get("lf-2").RegressionCount != 0 {
		t.Fatal("expected no regressions before re-detection")
	}
	if store.Add(&Finding{
		ID:           "lf-2",
		ResourceID:   "vm-101",
		ResourceName: "web",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
		Title:        "Restart loop",
		Description:  "Service repeatedly restarts again",
	}) {
		t.Fatal("expected second add to update existing finding")
	}

	got := store.Get("lf-2")
	if got == nil {
		t.Fatal("expected finding to exist")
	}
	if got.RegressionCount != 1 {
		t.Fatalf("expected regressionCount=1, got %d", got.RegressionCount)
	}
	if got.LastRegressionAt == nil {
		t.Fatal("expected lastRegressionAt to be set")
	}
	if got.AcknowledgedAt != nil {
		t.Fatal("expected acknowledgement to clear when the finding regresses")
	}
	foundRegressed := false
	for _, e := range got.Lifecycle {
		if e.Type == "regressed" {
			foundRegressed = true
			break
		}
	}
	if !foundRegressed {
		t.Fatal("expected regressed lifecycle event")
	}
}

func TestFindingsStore_RegressionClearsPriorAcknowledgement(t *testing.T) {
	store := NewFindingsStore()
	f := &Finding{
		ID:           "lf-ack-regress",
		ResourceID:   "vm-202",
		ResourceName: "api",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
		Title:        "Restart loop",
		Description:  "Service repeatedly restarts",
	}
	if !store.Add(f) {
		t.Fatal("expected first add to create finding")
	}
	if !store.Acknowledge(f.ID) {
		t.Fatal("expected acknowledge to succeed")
	}
	if !store.Resolve(f.ID, true) {
		t.Fatal("expected resolve to succeed")
	}
	if store.Add(&Finding{
		ID:           f.ID,
		ResourceID:   "vm-202",
		ResourceName: "api",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
		Title:        "Restart loop",
		Description:  "Service repeatedly restarts again",
	}) {
		t.Fatal("expected second add to update existing finding")
	}

	got := store.Get(f.ID)
	if got == nil {
		t.Fatal("expected finding to exist")
	}
	if got.AcknowledgedAt != nil {
		t.Fatal("expected acknowledgement to clear after regression")
	}
	foundRegressed := false
	for _, e := range got.Lifecycle {
		if e.Type == "regressed" {
			if e.Metadata["previous_acknowledged"] != "true" {
				t.Fatal("expected regressed lifecycle event to note prior acknowledgement")
			}
			foundRegressed = true
			break
		}
	}
	if !foundRegressed {
		t.Fatal("expected regressed lifecycle event")
	}
}

func TestFindingsStore_BlocksInvalidLoopStateTransition(t *testing.T) {
	store := NewFindingsStore()
	f := &Finding{
		ID:                   "lf-3",
		ResourceID:           "ct-1",
		ResourceName:         "container-1",
		Severity:             FindingSeverityWarning,
		Category:             FindingCategoryPerformance,
		Title:                "CPU burst",
		Description:          "Unexpected sustained CPU burst",
		LoopState:            string(FindingLoopStateResolved),
		InvestigationOutcome: string(InvestigationOutcomeFixExecuted), // would derive to remediating
	}

	// Directly call lock-only helper to validate transition guard behavior.
	store.mu.Lock()
	store.syncLoopStateLocked(f)
	store.mu.Unlock()

	if f.LoopState != string(FindingLoopStateResolved) {
		t.Fatalf("expected loop state to remain resolved, got %q", f.LoopState)
	}
	if len(f.Lifecycle) == 0 {
		t.Fatal("expected lifecycle event for blocked transition")
	}
	last := f.Lifecycle[len(f.Lifecycle)-1]
	if last.Type != "loop_transition_violation" {
		t.Fatalf("expected loop_transition_violation, got %q", last.Type)
	}
}

// --- Key-collision identity-shift events ----------------------------------
//
// The LLM-assigned finding key can collide: a substantially different report
// for the same resource+category+key lands on the existing finding's ID and
// overwrites its text. The merge is intentional (key forking would split LLM
// rephrasings of one issue into duplicate findings), but the identity shift
// must be recorded honestly as a content_replaced lifecycle event carrying
// the previous title.

func newCollisionFinding(title, description string) *Finding {
	return &Finding{
		ID:           "lf-collision",
		Key:          "restart-loop",
		ResourceID:   "vm-100",
		ResourceName: "vm-100",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
		Title:        title,
		Description:  description,
	}
}

func findLifecycleEvents(f *Finding, typ string) []FindingLifecycleEvent {
	var events []FindingLifecycleEvent
	for _, e := range f.Lifecycle {
		if e.Type == typ {
			events = append(events, e)
		}
	}
	return events
}

func TestFindingsStore_KeyCollisionRecordsContentReplacedEvent(t *testing.T) {
	store := NewFindingsStore()
	store.Add(newCollisionFinding(
		"frigate service stuck in restart loop",
		"frigate restarts every 30s after OOM kill",
	))

	// A distinct issue arrives under the same resource+category+key.
	store.Add(newCollisionFinding(
		"homeassistant zigbee bridge crashing repeatedly",
		"zigbee2mqtt bridge exits with USB disconnect errors",
	))

	got := store.Get("lf-collision")
	if got == nil {
		t.Fatal("expected merged finding to exist")
	}
	events := findLifecycleEvents(got, "content_replaced")
	if len(events) != 1 {
		t.Fatalf("expected exactly one content_replaced event, got %d (lifecycle=%+v)", len(events), got.Lifecycle)
	}
	if events[0].Metadata["previous_title"] != "frigate service stuck in restart loop" {
		t.Fatalf("expected previous title preserved in event meta, got %q", events[0].Metadata["previous_title"])
	}
	if events[0].Metadata["new_title"] != "homeassistant zigbee bridge crashing repeatedly" {
		t.Fatalf("expected new title in event meta, got %q", events[0].Metadata["new_title"])
	}
	// Merge semantics are unchanged: the latest report owns the text.
	if got.Title != "homeassistant zigbee bridge crashing repeatedly" {
		t.Fatalf("expected merge to keep overwriting the title, got %q", got.Title)
	}
}

func TestFindingsStore_RephrasedRedetectionDoesNotRecordContentReplaced(t *testing.T) {
	store := NewFindingsStore()
	store.Add(newCollisionFinding(
		"frigate service stuck in restart loop",
		"frigate restarts every 30s after OOM kill",
	))

	// Same issue, rephrased — shares the core keywords (frigate, restart).
	store.Add(newCollisionFinding(
		"frigate container keeps hitting a restart loop",
		"the frigate container restarts continuously after an OOM kill",
	))

	got := store.Get("lf-collision")
	if events := findLifecycleEvents(got, "content_replaced"); len(events) != 0 {
		t.Fatalf("rephrasing of the same issue must not record content_replaced, got %+v", events)
	}
}

func TestFindingsStore_IdenticalRedetectionDoesNotRecordContentReplaced(t *testing.T) {
	store := NewFindingsStore()
	store.Add(newCollisionFinding("frigate service stuck in restart loop", "d"))
	store.Add(newCollisionFinding("frigate service stuck in restart loop", "d"))

	got := store.Get("lf-collision")
	if events := findLifecycleEvents(got, "content_replaced"); len(events) != 0 {
		t.Fatalf("identical re-detection must not record content_replaced, got %+v", events)
	}
}

func TestFindingsStore_KeyCollisionOnResolvedFindingRecordsBothEvents(t *testing.T) {
	// The worst collision damage: a distinct new issue reusing a RESOLVED
	// finding's identity counts as a regression of the old issue. The
	// regression accounting stands (merge semantics), but the lifecycle must
	// show the content shift so the "regression" can be read for what it is.
	store := NewFindingsStore()
	store.Add(newCollisionFinding(
		"frigate service stuck in restart loop",
		"frigate restarts every 30s after OOM kill",
	))
	if !store.Resolve("lf-collision", false) {
		t.Fatal("expected resolve to succeed")
	}

	store.Add(newCollisionFinding(
		"homeassistant zigbee bridge crashing repeatedly",
		"zigbee2mqtt bridge exits with USB disconnect errors",
	))

	got := store.Get("lf-collision")
	if got.ResolvedAt != nil {
		t.Fatal("expected finding to be reactivated")
	}
	if len(findLifecycleEvents(got, "content_replaced")) != 1 {
		t.Fatalf("expected content_replaced on resolved-finding collision, lifecycle=%+v", got.Lifecycle)
	}
	if len(findLifecycleEvents(got, "regressed")) != 1 {
		t.Fatalf("expected regressed event to remain, lifecycle=%+v", got.Lifecycle)
	}
}

func TestFindingsStore_MergesEquivalentActiveSiblingWithHigherSeverity(t *testing.T) {
	store := NewFindingsStore()
	first := &Finding{
		ID:           "provider-warning",
		Key:          "provider-connection",
		ResourceID:   "patrol-runtime",
		ResourceName: "Patrol runtime",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
		Title:        "Provider connection issue",
		Description:  "The configured provider could not complete Patrol analysis.",
	}
	second := &Finding{
		ID:           "provider-critical",
		Key:          "openrouter-provider-unavailable",
		ResourceID:   "patrol-runtime",
		ResourceName: "Patrol runtime",
		Severity:     FindingSeverityCritical,
		Category:     FindingCategoryReliability,
		Title:        "Provider connection issue",
		Description:  "OpenRouter could not complete the selected Patrol model request.",
	}

	if !store.Add(first) {
		t.Fatal("expected first provider finding to be new")
	}
	if store.Add(second) {
		t.Fatal("expected equivalent provider finding to merge into the active issue")
	}

	active := store.GetActive(FindingSeverityWarning)
	if len(active) != 1 {
		t.Fatalf("expected one active finding after equivalent merge, got %d", len(active))
	}
	got := store.Get("provider-warning")
	if got == nil {
		t.Fatal("expected original finding to remain as merged active issue")
	}
	if store.Get("provider-critical") != nil {
		t.Fatal("expected duplicate finding ID not to be stored as a second active issue")
	}
	if got.Severity != FindingSeverityCritical {
		t.Fatalf("expected merged issue to keep highest severity critical, got %s", got.Severity)
	}
	if got.TimesRaised != 2 {
		t.Fatalf("expected merged issue timesRaised=2, got %d", got.TimesRaised)
	}
	events := findLifecycleEvents(got, "duplicate_merged")
	if len(events) != 1 {
		t.Fatalf("expected one duplicate_merged lifecycle event, got %d (lifecycle=%+v)", len(events), got.Lifecycle)
	}
	if events[0].Metadata["duplicate_id"] != "provider-critical" {
		t.Fatalf("expected duplicate_id metadata, got %+v", events[0].Metadata)
	}
}

func TestFindingsStore_DoesNotMergeDistinctSameResourceCategoryIssues(t *testing.T) {
	store := NewFindingsStore()
	if !store.Add(&Finding{
		ID:           "cpu-high",
		Key:          "cpu-high",
		ResourceID:   "vm-100",
		ResourceName: "vm-100",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		Title:        "High CPU usage",
		Description:  "CPU saturation is sustained.",
	}) {
		t.Fatal("expected CPU finding to be new")
	}
	if !store.Add(&Finding{
		ID:           "memory-high",
		Key:          "memory-high",
		ResourceID:   "vm-100",
		ResourceName: "vm-100",
		Severity:     FindingSeverityCritical,
		Category:     FindingCategoryPerformance,
		Title:        "High memory usage",
		Description:  "Memory pressure is sustained.",
	}) {
		t.Fatal("expected memory finding to remain a separate active issue")
	}

	active := store.GetActive(FindingSeverityWarning)
	if len(active) != 2 {
		t.Fatalf("expected two distinct active findings, got %d", len(active))
	}
}

func TestFindingsStore_MergesRelatedStorageCapacitySibling(t *testing.T) {
	store := NewFindingsStore()
	if !store.Add(&Finding{
		ID:           "tower-array-critical",
		Key:          "unraid-array-no-parity-capacity-risk",
		ResourceID:   "storage/tower-array",
		ResourceName: "Tower Array",
		ResourceType: "storage",
		Severity:     FindingSeverityCritical,
		Category:     FindingCategoryReliability,
		Title:        "Unraid array running without parity protection while at 86% capacity",
		Description:  "The Unraid array is at 85.9% capacity while parity is unavailable.",
	}) {
		t.Fatal("expected first storage finding to be new")
	}
	if store.Add(&Finding{
		ID:           "tower-array-warning",
		Key:          "storage-pool-usage",
		ResourceID:   "storage/tower",
		ResourceName: "Tower",
		ResourceType: "storage",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		Title:        "Storage pool Tower Array at 85.9% usage",
		Description:  "Usage at 86% exceeds the configured storage warning threshold.",
	}) {
		t.Fatal("expected related storage capacity sibling to merge into the active issue")
	}

	active := store.GetActive(FindingSeverityWarning)
	if len(active) != 1 {
		t.Fatalf("expected one active storage issue after merge, got %d", len(active))
	}
	got := store.Get("tower-array-critical")
	if got == nil {
		t.Fatal("expected original critical issue to remain")
	}
	if got.Severity != FindingSeverityCritical {
		t.Fatalf("expected merged issue to keep critical severity, got %s", got.Severity)
	}
	if got.Title != "Unraid array running without parity protection while at 86% capacity" {
		t.Fatalf("expected broader critical title to remain, got %q", got.Title)
	}
	if store.Get("tower-array-warning") != nil {
		t.Fatal("expected generic storage usage warning not to be stored as a second active issue")
	}
	if got.TimesRaised != 2 {
		t.Fatalf("expected merged issue timesRaised=2, got %d", got.TimesRaised)
	}
	if events := findLifecycleEvents(got, "duplicate_merged"); len(events) != 1 {
		t.Fatalf("expected duplicate_merged event, got %d lifecycle=%+v", len(events), got.Lifecycle)
	}
}

func TestFindingsStore_MergesRelatedStorageCapacitySiblingEscalation(t *testing.T) {
	store := NewFindingsStore()
	if !store.Add(&Finding{
		ID:           "tower-array-warning",
		Key:          "storage-pool-usage",
		ResourceID:   "storage/tower",
		ResourceName: "Tower",
		ResourceType: "storage",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		Title:        "Storage pool Tower Array at 85.9% usage",
		Description:  "Usage at 86% exceeds the configured storage warning threshold.",
	}) {
		t.Fatal("expected first storage finding to be new")
	}
	if store.Add(&Finding{
		ID:           "tower-array-critical",
		Key:          "unraid-array-no-parity-capacity-risk",
		ResourceID:   "storage/tower-array",
		ResourceName: "Tower Array",
		ResourceType: "storage",
		Severity:     FindingSeverityCritical,
		Category:     FindingCategoryReliability,
		Title:        "Unraid array running without parity protection while at 86% capacity",
		Description:  "The Unraid array is at 85.9% capacity while parity is unavailable.",
	}) {
		t.Fatal("expected broader storage capacity sibling to merge into the active issue")
	}

	active := store.GetActive(FindingSeverityWarning)
	if len(active) != 1 {
		t.Fatalf("expected one active storage issue after merge, got %d", len(active))
	}
	got := store.Get("tower-array-warning")
	if got == nil {
		t.Fatal("expected original warning issue to remain as merged record")
	}
	if got.Severity != FindingSeverityCritical {
		t.Fatalf("expected merged issue to escalate to critical, got %s", got.Severity)
	}
	if got.Title != "Unraid array running without parity protection while at 86% capacity" {
		t.Fatalf("expected merged issue to adopt higher-severity title, got %q", got.Title)
	}
	if store.Get("tower-array-critical") != nil {
		t.Fatal("expected critical sibling not to be stored as a second active issue")
	}
}

func TestFindingsStore_DoesNotMergeStorageCapacityWithDistinctBackupIssue(t *testing.T) {
	store := NewFindingsStore()
	if !store.Add(&Finding{
		ID:           "tower-array-usage",
		Key:          "storage-pool-usage",
		ResourceID:   "storage/tower-array",
		ResourceName: "Tower Array",
		ResourceType: "storage",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		Title:        "Storage pool Tower Array at 85.9% usage",
		Description:  "Usage at 86% exceeds the configured storage warning threshold.",
	}) {
		t.Fatal("expected storage capacity finding to be new")
	}
	if !store.Add(&Finding{
		ID:           "tower-array-backup",
		Key:          "backup-job-failed",
		ResourceID:   "storage/tower-array",
		ResourceName: "Tower Array",
		ResourceType: "storage",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryBackup,
		Title:        "Backup job failed for Tower Array",
		Description:  "The latest backup job failed and no new restore point was created.",
	}) {
		t.Fatal("expected distinct backup finding to remain separate")
	}

	active := store.GetActive(FindingSeverityWarning)
	if len(active) != 2 {
		t.Fatalf("expected two distinct storage findings, got %d", len(active))
	}
}

func TestFindingsStore_DoesNotMergeStoragePoolWithPhysicalDiskChild(t *testing.T) {
	store := NewFindingsStore()
	if !store.Add(&Finding{
		ID:           "pool-usage",
		Key:          "storage-pool-usage",
		ResourceID:   "storage/local-zfs-data",
		ResourceName: "local-zfs (data)",
		ResourceType: "storage",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		Title:        "Storage pool local-zfs (data) at 86% usage",
		Description:  "Usage at 86% exceeds the configured storage warning threshold.",
	}) {
		t.Fatal("expected pool finding to be new")
	}
	if !store.Add(&Finding{
		ID:           "disk-usage",
		Key:          "physical-disk-usage",
		ResourceID:   "storage/local-zfs-data-dev-sda4",
		ResourceName: "local-zfs (data, /dev/sda4)",
		ResourceType: "storage",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		Title:        "Storage disk local-zfs (data, /dev/sda4) at 96% usage",
		Description:  "Usage at 96% exceeds the configured storage critical threshold.",
	}) {
		t.Fatal("expected physical disk child finding to remain separate")
	}

	active := store.GetActive(FindingSeverityWarning)
	if len(active) != 2 {
		t.Fatalf("expected pool and disk child to remain separate, got %d", len(active))
	}
}
