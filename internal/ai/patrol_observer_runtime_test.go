package ai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func createInstallablePatrolObserver(t *testing.T, store *PatrolObjectiveStore, now time.Time, resourceIDs ...string) PatrolObjective {
	t.Helper()
	objective, err := store.Create(CreatePatrolObjectiveInput{
		Brief:       "Keep scoped resources online",
		ResourceIDs: resourceIDs,
	}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	objective, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision,
		EvidenceFit:      PatrolObserverEvidenceFitDirect,
		Interpretation:   "Every scoped canonical resource remains online.",
		TriggerKinds:     []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
		ProbeJSON:        `{"runtime":"pulse-resource-state/v1","path":"status","operator":"equals","value":"online","sample_interval_seconds":10,"wake_after_consecutive_failures":2}`,
		WakeEvidence:     "A scoped resource is not online for two consecutive samples.",
		RequirementsJSON: `{}`,
		Actor:            "patrol:model",
	}, now)
	if err != nil {
		t.Fatalf("propose observer: %v", err)
	}
	return objective
}

func TestPatrolObserverRuntimeInstallsEvaluatesAndLeasesWithoutObjectiveRevisionChurn(t *testing.T) {
	now := time.Date(2026, 8, 14, 1, 0, 0, 0, time.UTC)
	store := NewInMemoryPatrolObjectiveStore()
	objective := createInstallablePatrolObserver(t, store, now, "node-1")
	proposalRevision := objective.Revision
	patrol := NewPatrolService(nil, mockPatrolStateProvider{state: models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-1", Name: "node-1", Status: "online"}},
	}})
	patrol.SetObjectiveStore(store)

	patrol.processObjectiveObservers(now.Add(time.Second))
	got, ok := store.Get(objective.ID, now.Add(time.Second))
	if !ok || got.Observer == nil {
		t.Fatalf("installed objective missing: %+v, found=%v", got, ok)
	}
	if got.Observer.State != PatrolObserverInstalled || got.Coverage.State != PatrolObjectiveCovered || got.Coverage.ReasonCode != "observer_healthy" {
		t.Fatalf("installed observer = %+v, coverage = %+v", got.Observer, got.Coverage)
	}
	if got.Observer.ValidUntil == nil || got.Observer.LastEvidenceAt == nil {
		t.Fatalf("observer lease was not persisted: %+v", got.Observer)
	}
	if got.Revision != proposalRevision+2 {
		t.Fatalf("objective revision = %d, want proposal revision + validation/install only (%d)", got.Revision, proposalRevision+2)
	}

	patrol.processObjectiveObservers(now.Add(2 * time.Minute))
	refreshed, _ := store.Get(objective.ID, now.Add(2*time.Minute))
	if refreshed.Revision != got.Revision {
		t.Fatalf("health heartbeat changed objective revision from %d to %d", got.Revision, refreshed.Revision)
	}
	if !refreshed.Observer.ValidUntil.After(*got.Observer.ValidUntil) {
		t.Fatalf("health lease did not advance: old=%v new=%v", got.Observer.ValidUntil, refreshed.Observer.ValidUntil)
	}
}

func TestPatrolObserverRuntimeRecordsExplicitUnsupportedValidationReason(t *testing.T) {
	now := time.Date(2026, 8, 14, 1, 0, 0, 0, time.UTC)
	store := NewInMemoryPatrolObjectiveStore()
	objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep playback smooth", ResourceIDs: []string{"jellyfin"}}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	objective, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision,
		Interpretation:   "Observe playback buffering events.",
		TriggerKinds:     []PatrolObserverTriggerKind{PatrolObserverTriggerAPI},
		ProbeJSON:        `{"runtime":"jellyfin-api/v1","event":"playback"}`,
		WakeEvidence:     "Playback buffers.",
		RequirementsJSON: `{"network":["jellyfin"]}`,
	}, now)
	if err != nil {
		t.Fatalf("propose observer: %v", err)
	}
	patrol := NewPatrolService(nil, nil)
	patrol.SetObjectiveStore(store)
	patrol.processObjectiveObservers(now.Add(time.Second))

	got, _ := store.Get(objective.ID, now.Add(time.Second))
	if got.Observer == nil || got.Observer.State != PatrolObserverRejected {
		t.Fatalf("unsupported proposal state = %+v", got.Observer)
	}
	if got.Observer.FailureCode != "observer_trigger_unsupported" || got.Coverage.ReasonCode != "observer_trigger_unsupported" {
		t.Fatalf("validation failure was not projected explicitly: observer=%+v coverage=%+v", got.Observer, got.Coverage)
	}
	if got.Coverage.State != PatrolObjectiveUncovered || !strings.Contains(got.Coverage.Summary, "could not be validated") {
		t.Fatalf("unsupported proposal coverage = %+v", got.Coverage)
	}
	revised, err := store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: got.Revision,
		Interpretation:   "Keep the canonical Jellyfin resource online.",
		TriggerKinds:     []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
		ProbeJSON:        `{"runtime":"pulse-resource-state/v1","path":"status","operator":"equals","value":"online","sample_interval_seconds":30,"wake_after_consecutive_failures":2}`,
		WakeEvidence:     "The Jellyfin resource is not online twice.",
		RequirementsJSON: `{}`,
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("replace rejected observer proposal: %v", err)
	}
	if revised.Observer == nil || revised.Observer.Version != got.Observer.Version+1 || revised.Observer.State != PatrolObserverProposed {
		t.Fatalf("replacement proposal = %+v", revised.Observer)
	}
}

func TestPatrolAvailabilityObserverUsesCanonicalScopedTargetAndWakesOnOutcomeBreach(t *testing.T) {
	now := time.Date(2026, 8, 14, 3, 0, 0, 0, time.UTC)
	store := NewInMemoryPatrolObjectiveStore()
	objective, err := store.Create(CreatePatrolObjectiveInput{
		Brief:       "Keep the front cameras reachable",
		ResourceIDs: []string{"frigate-1"},
	}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	objective, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision,
		EvidenceFit:      PatrolObserverEvidenceFitDirect,
		Interpretation:   "The existing camera availability check remains reachable.",
		TriggerKinds:     []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
		ProbeJSON:        `{"runtime":"pulse-availability-state/v1","target_id":"camera-front-http","path":"probe_outcome","operator":"equals","value":"reachable","sample_interval_seconds":10,"wake_after_consecutive_failures":2}`,
		WakeEvidence:     "The canonical camera endpoint is not reachable twice.",
		RequirementsJSON: `{}`,
	}, now)
	if err != nil {
		t.Fatalf("propose availability observer: %v", err)
	}

	checkedAt := now
	outcome := "reachable"
	provider := &mockUnifiedResourceProvider{getAllFunc: func() []unifiedresources.Resource {
		return []unifiedresources.Resource{{
			ID: "frigate-1", Type: unifiedresources.ResourceTypeAppContainer,
			AvailabilityChecks: []unifiedresources.AvailabilityData{{
				TargetID: "camera-front-http", LinkedResourceID: "frigate-1", Enabled: true,
				ProbeOutcome: outcome, LastChecked: &checkedAt,
			}},
		}}
	}}
	patrol := NewPatrolService(nil, nil)
	patrol.SetObjectiveStore(store)
	patrol.SetUnifiedResourceProvider(provider)
	tm := NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 10})
	patrol.SetTriggerManager(tm)

	patrol.processObjectiveObservers(now.Add(time.Second))
	installed, _ := store.Get(objective.ID, now.Add(time.Second))
	if installed.Observer == nil || installed.Observer.State != PatrolObserverInstalled || installed.Coverage.State != PatrolObjectiveCovered {
		t.Fatalf("availability observer was not installed: observer=%+v coverage=%+v", installed.Observer, installed.Coverage)
	}
	if tm.GetPendingCount() != 0 {
		t.Fatal("reachable availability target woke Patrol")
	}

	outcome = "unreachable"
	checkedAt = now.Add(11 * time.Second)
	patrol.processObjectiveObservers(now.Add(11 * time.Second))
	patrol.processObjectiveObservers(now.Add(21 * time.Second))
	if tm.GetPendingCount() != 1 {
		t.Fatalf("availability breach did not wake Patrol once; pending=%d", tm.GetPendingCount())
	}
	queued := tm.pendingTriggers[0]
	if queued.ObjectiveContext == nil || len(queued.ObjectiveContext.ObservedResourceIDs) != 1 || queued.ObjectiveContext.ObservedResourceIDs[0] != "frigate-1" {
		t.Fatalf("availability wake lost canonical owner context: %+v", queued.ObjectiveContext)
	}
	if !strings.Contains(queued.ObjectiveContext.Evidence, "camera-front-http") || !strings.Contains(queued.ObjectiveContext.Evidence, "equals reachable") {
		t.Fatalf("availability wake evidence = %q", queued.ObjectiveContext.Evidence)
	}
}

func TestPatrolAvailabilityObserverBindingFailsClosed(t *testing.T) {
	now := time.Date(2026, 8, 14, 3, 0, 0, 0, time.UTC)
	checkedAt := now
	for _, test := range []struct {
		name       string
		resourceID string
		targetID   string
		enabled    bool
		wantCode   string
	}{
		{name: "missing", resourceID: "camera-1", targetID: "different-target", enabled: true, wantCode: "observer_availability_target_missing"},
		{name: "disabled", resourceID: "camera-1", targetID: "camera-http", enabled: false, wantCode: "observer_availability_target_disabled"},
		{name: "out of scope", resourceID: "camera-2", targetID: "camera-http", enabled: true, wantCode: "observer_availability_target_out_of_scope"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := NewInMemoryPatrolObjectiveStore()
			objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep camera online", ResourceIDs: []string{"camera-1"}}, now)
			if err != nil {
				t.Fatalf("create objective: %v", err)
			}
			objective, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
				ExpectedRevision: objective.Revision,
				Interpretation:   "Keep the endpoint reachable.",
				TriggerKinds:     []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
				ProbeJSON:        `{"runtime":"pulse-availability-state/v1","target_id":"camera-http","path":"probe_outcome","operator":"equals","value":"reachable","sample_interval_seconds":30,"wake_after_consecutive_failures":2}`,
				WakeEvidence:     "Endpoint reachability breached.", RequirementsJSON: `{}`,
			}, now)
			if err != nil {
				t.Fatalf("propose observer: %v", err)
			}
			provider := &mockUnifiedResourceProvider{getAllFunc: func() []unifiedresources.Resource {
				return []unifiedresources.Resource{{
					ID: test.resourceID,
					AvailabilityChecks: []unifiedresources.AvailabilityData{{
						TargetID: test.targetID, Enabled: test.enabled, ProbeOutcome: "reachable", LastChecked: &checkedAt,
					}},
				}}
			}}
			patrol := NewPatrolService(nil, nil)
			patrol.SetObjectiveStore(store)
			patrol.SetUnifiedResourceProvider(provider)
			patrol.processObjectiveObservers(now.Add(time.Second))

			got, _ := store.Get(objective.ID, now.Add(time.Second))
			if got.Observer == nil || got.Observer.State != PatrolObserverRejected || got.Observer.FailureCode != test.wantCode {
				t.Fatalf("binding result = %+v, want rejected/%s", got.Observer, test.wantCode)
			}
		})
	}
}

func TestPatrolResourceMetricObserverWakesOnFreshThresholdBreach(t *testing.T) {
	now := time.Date(2026, 8, 14, 4, 0, 0, 0, time.UTC)
	store := NewInMemoryPatrolObjectiveStore()
	objective, err := store.Create(CreatePatrolObjectiveInput{
		Brief: "Keep this node cool", ResourceIDs: []string{"node-1"},
	}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	objective, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision,
		Interpretation:   "The canonical node temperature remains below 75 Celsius.",
		TriggerKinds:     []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
		ProbeJSON:        `{"runtime":"pulse-resource-metric/v1","metric":"temperature_celsius","operator":"less_than","threshold":75,"sample_interval_seconds":10,"wake_after_consecutive_failures":2,"max_evidence_age_seconds":60}`,
		WakeEvidence:     "Fresh temperature evidence is at or above 75 Celsius twice.",
		RequirementsJSON: `{}`,
	}, now)
	if err != nil {
		t.Fatalf("propose metric observer: %v", err)
	}
	temperature := 82.5
	provider := &mockUnifiedResourceProvider{getAllFunc: func() []unifiedresources.Resource {
		return []unifiedresources.Resource{{
			ID: "node-1", LastSeen: now.Add(20 * time.Second), Temperature: &temperature,
		}}
	}}
	patrol := NewPatrolService(nil, nil)
	patrol.SetObjectiveStore(store)
	patrol.SetUnifiedResourceProvider(provider)
	tm := NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 10})
	patrol.SetTriggerManager(tm)

	patrol.processObjectiveObservers(now.Add(time.Second))
	patrol.processObjectiveObservers(now.Add(11 * time.Second))
	if tm.GetPendingCount() != 1 {
		t.Fatalf("metric breach did not wake Patrol once; pending=%d", tm.GetPendingCount())
	}
	queued := tm.pendingTriggers[0]
	if queued.ObjectiveContext == nil || !strings.Contains(queued.ObjectiveContext.Evidence, "temperature_celsius") || !strings.Contains(queued.ObjectiveContext.Evidence, "node-1=82.50") {
		t.Fatalf("metric evidence = %+v", queued.ObjectiveContext)
	}
}

func TestPatrolEstateWideObjectiveUsesCurrentCanonicalResourceSet(t *testing.T) {
	now := time.Date(2026, 8, 14, 4, 30, 0, 0, time.UTC)
	store := NewInMemoryPatrolObjectiveStore()
	objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep disk use below 85 percent"}, now)
	if err != nil {
		t.Fatalf("create estate objective: %v", err)
	}
	objective, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision, Interpretation: "Every current canonical resource with disk telemetry stays below 85 percent.",
		EvidenceFit:  PatrolObserverEvidenceFitDirect,
		TriggerKinds: []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
		ProbeJSON:    `{"runtime":"pulse-resource-metric/v1","metric":"disk_percent","operator":"less_than","threshold":85,"sample_interval_seconds":10,"wake_after_consecutive_failures":1,"max_evidence_age_seconds":60}`,
		WakeEvidence: "A current resource breaches the disk objective.", RequirementsJSON: `{}`,
	}, now)
	if err != nil {
		t.Fatalf("propose observer: %v", err)
	}
	disk := &unifiedresources.MetricValue{Percent: 40}
	provider := &mockUnifiedResourceProvider{getAllFunc: func() []unifiedresources.Resource {
		return []unifiedresources.Resource{{ID: "node-1", LastSeen: now.Add(time.Second), Metrics: &unifiedresources.ResourceMetrics{Disk: disk}}}
	}}
	patrol := NewPatrolService(nil, nil)
	patrol.SetObjectiveStore(store)
	patrol.SetUnifiedResourceProvider(provider)
	tm := NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 10})
	patrol.SetTriggerManager(tm)
	patrol.processObjectiveObservers(now.Add(time.Second))
	installed, _ := store.Get(objective.ID, now.Add(time.Second))
	if installed.Coverage.State != PatrolObjectiveCovered {
		t.Fatalf("estate-wide objective coverage = %+v", installed.Coverage)
	}
	disk.Percent = 90
	patrol.processObjectiveObservers(now.Add(11 * time.Second))
	if tm.GetPendingCount() != 1 || tm.pendingTriggers[0].ObjectiveContext == nil || len(tm.pendingTriggers[0].ObjectiveContext.ObservedResourceIDs) != 1 || tm.pendingTriggers[0].ObjectiveContext.ObservedResourceIDs[0] != "node-1" {
		t.Fatalf("estate-wide breach did not bind the current canonical resource: %+v", tm.pendingTriggers)
	}
}

func TestPatrolResourceMetricObserverFailsClosedForStaleOrMissingEvidence(t *testing.T) {
	now := time.Date(2026, 8, 14, 4, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name       string
		resource   unifiedresources.Resource
		wantDetail string
	}{
		{name: "stale", resource: unifiedresources.Resource{ID: "node-1", LastSeen: now.Add(-10 * time.Minute), Metrics: &unifiedresources.ResourceMetrics{Disk: &unifiedresources.MetricValue{Percent: 20}}}, wantDetail: "metric_stale"},
		{name: "missing", resource: unifiedresources.Resource{ID: "node-1", LastSeen: now}, wantDetail: "metric_missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := NewInMemoryPatrolObjectiveStore()
			objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep disk below 85 percent", ResourceIDs: []string{"node-1"}}, now)
			if err != nil {
				t.Fatalf("create objective: %v", err)
			}
			objective, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
				ExpectedRevision: objective.Revision, Interpretation: "Disk remains below 85 percent.",
				TriggerKinds: []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
				ProbeJSON:    `{"runtime":"pulse-resource-metric/v1","metric":"disk_percent","operator":"less_than","threshold":85,"sample_interval_seconds":10,"wake_after_consecutive_failures":1,"max_evidence_age_seconds":60}`,
				WakeEvidence: "Disk metric is high or unavailable.", RequirementsJSON: `{}`,
			}, now)
			if err != nil {
				t.Fatalf("propose observer: %v", err)
			}
			patrol := NewPatrolService(nil, nil)
			patrol.SetObjectiveStore(store)
			patrol.SetUnifiedResourceProvider(&mockUnifiedResourceProvider{getAllFunc: func() []unifiedresources.Resource { return []unifiedresources.Resource{test.resource} }})
			tm := NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 10})
			patrol.SetTriggerManager(tm)
			patrol.processObjectiveObservers(now.Add(time.Second))
			if tm.GetPendingCount() != 1 || tm.pendingTriggers[0].ObjectiveContext == nil || !strings.Contains(tm.pendingTriggers[0].ObjectiveContext.Evidence, test.wantDetail) {
				t.Fatalf("fail-closed evidence = %+v", tm.pendingTriggers)
			}
		})
	}
}

func TestValidatePatrolResourceMetricObserverRejectsUnsafeBounds(t *testing.T) {
	now := time.Now().UTC()
	store := NewInMemoryPatrolObjectiveStore()
	objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep node cool", ResourceIDs: []string{"node-1"}}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	objective, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision, Interpretation: "Temperature stays safe.",
		TriggerKinds: []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
		ProbeJSON:    `{"runtime":"pulse-resource-metric/v1","metric":"temperature_celsius","operator":"less_than","threshold":1000,"sample_interval_seconds":10,"wake_after_consecutive_failures":2,"max_evidence_age_seconds":60}`,
		WakeEvidence: "Temperature is high.", RequirementsJSON: `{}`,
	}, now)
	if err != nil {
		t.Fatalf("propose observer: %v", err)
	}
	artifact, _ := store.GetObserverArtifact(objective.ID)
	_, validationErr := validatePatrolObserverArtifact(objective, artifact)
	if validationErr == nil || validationErr.code != "observer_threshold_out_of_bounds" {
		t.Fatalf("threshold validation error = %v", validationErr)
	}
}

func TestPatrolHTTPJSONObserverUsesScopedDiscoveryOriginSecretReferenceAndWakes(t *testing.T) {
	requestObserved := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/stats" || r.URL.Query().Get("window") != "active" {
			t.Errorf("request target = %s", r.URL.String())
		}
		if r.Header.Get("X-Api-Key") != "stored-secret" {
			t.Errorf("resolved auth header = %q", r.Header.Get("X-Api-Key"))
		}
		requestObserved <- struct{}{}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"playback":{"buffering_sessions":1}}`))
	}))
	defer server.Close()

	discoveryStore, err := servicediscovery.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("create discovery store: %v", err)
	}
	if err := discoveryStore.Save(&servicediscovery.ResourceDiscovery{
		ID: "system-container:node1:101", ResourceType: servicediscovery.ResourceTypeSystemContainer,
		ResourceID: "101", TargetID: "node1", SuggestedURL: server.URL + "/ignored-base",
		UserSecrets: map[string]string{"jellyfin_api_key": "stored-secret"},
	}); err != nil {
		t.Fatalf("save discovery: %v", err)
	}

	now := time.Now().UTC()
	store := NewInMemoryPatrolObjectiveStore()
	objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep Jellyfin playback from buffering", ResourceIDs: []string{"101"}}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	objective, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision, Interpretation: "No active playback session is buffering.",
		EvidenceFit:  PatrolObserverEvidenceFitDirect,
		TriggerKinds: []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
		ProbeJSON:    `{"runtime":"pulse-http-json/v1","discovery_id":"system-container:node1:101","request_path":"/api/stats?window=active","json_pointer":"/playback/buffering_sessions","operator":"less_than","expected":1,"auth":{"header_name":"X-Api-Key","secret_ref":"jellyfin_api_key"},"timeout_seconds":2,"sample_interval_seconds":10,"wake_after_consecutive_failures":1}`,
		WakeEvidence: "The local service API reports one or more buffering sessions.", RequirementsJSON: `{}`,
	}, now)
	if err != nil {
		t.Fatalf("propose HTTP JSON observer: %v", err)
	}
	patrol := NewPatrolService(nil, nil)
	patrol.SetObjectiveStore(store)
	patrol.SetDiscoveryStore(discoveryStore)
	tm := NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 10})
	patrol.SetTriggerManager(tm)
	patrol.processObjectiveObservers(now.Add(time.Second))

	select {
	case <-requestObserved:
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP JSON observer did not execute")
	}
	// The HTTP sample runs on its own goroutine and persists the health lease
	// strictly after it queues the wake, so the lease is the one side effect
	// that orders the whole sample against these assertions. Gating on the wake
	// instead let a loaded runner read a half-applied sample: a queued trigger
	// with a still-nil lease, reported as degraded/observer_health_unknown.
	// Coverage is evaluated at a fixed instant derived from the sample time so
	// the verdict never depends on how long the goroutine took to get there.
	evaluatedAt := now.Add(2 * time.Second)
	deadline := time.Now().Add(10 * time.Second)
	var installed PatrolObjective
	for {
		installed, _ = store.Get(objective.ID, evaluatedAt)
		if installed.Observer != nil && installed.Observer.ValidUntil != nil {
			break
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("HTTP JSON observer never persisted a health lease: %+v", installed.Observer)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if tm.GetPendingCount() != 1 {
		t.Fatalf("HTTP JSON breach did not wake Patrol; pending=%d", tm.GetPendingCount())
	}
	queued := tm.pendingTriggers[0]
	if queued.ObjectiveContext == nil || !strings.Contains(queued.ObjectiveContext.Evidence, "buffering_sessions") || !strings.Contains(queued.ObjectiveContext.Evidence, "actual_1") {
		t.Fatalf("HTTP JSON objective evidence = %+v", queued.ObjectiveContext)
	}
	if installed.Observer.State != PatrolObserverInstalled || installed.Coverage.State != PatrolObjectiveCovered || installed.Coverage.ReasonCode != "observer_healthy" {
		t.Fatalf("HTTP observer coverage = %+v / %+v", installed.Observer, installed.Coverage)
	}
}

func TestValidatePatrolHTTPJSONObserverRejectsNetworkAuthorityAndScopeEscapes(t *testing.T) {
	now := time.Now().UTC()
	store := NewInMemoryPatrolObjectiveStore()
	objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep service healthy", ResourceIDs: []string{"101"}}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	objective, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision, Interpretation: "Service health is true.",
		TriggerKinds: []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
		ProbeJSON:    `{"runtime":"pulse-http-json/v1","discovery_id":"system-container:node1:102","request_path":"//169.254.169.254/latest/meta-data","json_pointer":"/healthy","operator":"equals","expected":true,"timeout_seconds":2,"sample_interval_seconds":10,"wake_after_consecutive_failures":1}`,
		WakeEvidence: "Health is false.", RequirementsJSON: `{}`,
	}, now)
	if err != nil {
		t.Fatalf("propose observer: %v", err)
	}
	artifact, _ := store.GetObserverArtifact(objective.ID)
	_, validationErr := validatePatrolObserverArtifact(objective, artifact)
	if validationErr == nil || validationErr.code != "observer_http_path_invalid" {
		t.Fatalf("model-supplied network authority validation = %v", validationErr)
	}

	artifact.Probe = []byte(`{"runtime":"pulse-http-json/v1","discovery_id":"system-container:node1:102","request_path":"/api/health","json_pointer":"/healthy","operator":"equals","expected":true,"timeout_seconds":2,"sample_interval_seconds":10,"wake_after_consecutive_failures":1}`)
	probe, validationErr := validatePatrolObserverArtifact(objective, artifact)
	if validationErr != nil {
		t.Fatalf("valid HTTP artifact rejected before binding: %v", validationErr)
	}
	discoveryStore, err := servicediscovery.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("create discovery store: %v", err)
	}
	if err := discoveryStore.Save(&servicediscovery.ResourceDiscovery{
		ID: "system-container:node1:102", ResourceType: servicediscovery.ResourceTypeSystemContainer,
		ResourceID: "102", TargetID: "node1", SuggestedURL: "http://127.0.0.1:8080",
	}); err != nil {
		t.Fatalf("save discovery: %v", err)
	}
	patrol := NewPatrolService(nil, nil)
	patrol.SetDiscoveryStore(discoveryStore)
	if bindingErr := patrol.validatePatrolObserverBinding(objective, probe, patrolRuntimeState{}); bindingErr == nil || bindingErr.code != "observer_discovery_out_of_scope" {
		t.Fatalf("out-of-scope discovery binding = %v", bindingErr)
	}
}

func TestPatrolObserverRuntimeWakesModelOnlyAfterLocalFailureWindow(t *testing.T) {
	now := time.Date(2026, 8, 14, 1, 0, 0, 0, time.UTC)
	store := NewInMemoryPatrolObjectiveStore()
	objective := createInstallablePatrolObserver(t, store, now, "camera-1")
	patrol := NewPatrolService(nil, mockPatrolStateProvider{state: models.StateSnapshot{
		Hosts: []models.Host{{ID: "camera-1", Hostname: "camera-1", Status: "offline"}},
	}})
	patrol.SetObjectiveStore(store)
	tm := NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 10})
	patrol.SetTriggerManager(tm)

	patrol.processObjectiveObservers(now.Add(time.Second))
	if got := tm.GetPendingCount(); got != 0 {
		t.Fatalf("observer woke Patrol after first failed local sample; pending=%d", got)
	}
	patrol.processObjectiveObservers(now.Add(11 * time.Second))
	if got := tm.GetPendingCount(); got != 1 {
		t.Fatalf("observer did not queue one Patrol wake after failure window; pending=%d", got)
	}
	queued := tm.pendingTriggers[0]
	if queued.ObjectiveContext == nil {
		t.Fatal("observer wake discarded the triggering objective context")
	}
	currentObjective, found := store.Get(objective.ID, now.Add(11*time.Second))
	if !found {
		t.Fatal("triggering objective disappeared from store")
	}
	if queued.ObjectiveContext.ObjectiveID != currentObjective.ID || queued.ObjectiveContext.Revision != currentObjective.Revision || queued.ObjectiveContext.Brief != currentObjective.Brief {
		t.Fatalf("queued objective context = %+v, want exact objective %+v", queued.ObjectiveContext, currentObjective)
	}
	if queued.ObjectiveContext.ObserverID != currentObjective.Observer.ID || queued.ObjectiveContext.ObserverVersion != currentObjective.Observer.Version {
		t.Fatalf("queued observer binding = %+v, want %s/%d", queued.ObjectiveContext, currentObjective.Observer.ID, currentObjective.Observer.Version)
	}
	if len(queued.ObjectiveContext.ObservedResourceIDs) != 1 || queued.ObjectiveContext.ObservedResourceIDs[0] != "camera-1" || !strings.Contains(queued.ObjectiveContext.Evidence, "2 consecutive local samples") {
		t.Fatalf("queued objective evidence = %+v", queued.ObjectiveContext)
	}
	patrol.processObjectiveObservers(now.Add(21 * time.Second))
	if got := tm.GetPendingCount(); got != 1 {
		t.Fatalf("observer repeated wake while failure remained active; pending=%d", got)
	}
	got, _ := store.Get(objective.ID, now.Add(21*time.Second))
	if got.Coverage.State != PatrolObjectiveCovered {
		t.Fatalf("runtime health and objective outcome were conflated: %+v", got.Coverage)
	}
}

func TestPatrolObserverRuntimeRetriesOnlyUntilDurableObjectiveRunSucceeds(t *testing.T) {
	now := time.Date(2026, 8, 14, 2, 0, 0, 0, time.UTC)
	store := NewInMemoryPatrolObjectiveStore()
	objective := createInstallablePatrolObserver(t, store, now, "camera-1")
	patrol := NewPatrolService(nil, mockPatrolStateProvider{state: models.StateSnapshot{
		Hosts: []models.Host{{ID: "camera-1", Hostname: "camera-1", Status: "offline"}},
	}})
	patrol.SetObjectiveStore(store)
	tm := NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 10})
	patrol.SetTriggerManager(tm)

	patrol.processObjectiveObservers(now.Add(time.Second))
	firstWakeAt := now.Add(11 * time.Second)
	patrol.processObjectiveObservers(firstWakeAt)
	if tm.GetPendingCount() != 1 {
		t.Fatal("initial objective wake was not queued")
	}
	clearPendingPatrolTriggers(tm)

	retryAt := firstWakeAt.Add(patrolObserverWakeRetryInterval + time.Second)
	patrol.processObjectiveObservers(retryAt)
	if tm.GetPendingCount() != 1 {
		t.Fatal("unacknowledged objective wake was not retried after bounded interval")
	}
	retried := tm.pendingTriggers[0]
	clearPendingPatrolTriggers(tm)

	patrol.runHistoryStore.Add(PatrolRunRecord{
		ID:               "objective-run-success",
		StartedAt:        retryAt,
		CompletedAt:      retryAt.Add(time.Second),
		Type:             "scoped",
		TriggerReason:    string(TriggerReasonObjectiveEvidence),
		Status:           "issues_found",
		FindingIDs:       []string{"camera-offline"},
		ObjectiveContext: clonePatrolObjectiveContext(retried.ObjectiveContext),
	})
	patrol.processObjectiveObservers(retryAt.Add(patrolObserverWakeRetryInterval + time.Second))
	if tm.GetPendingCount() != 0 {
		t.Fatal("observer retried after a successful durable objective run acknowledged delivery")
	}

	current, found := store.Get(objective.ID, retryAt)
	if !found || !patrol.objectiveWakeDelivered(current, retryAt) {
		t.Fatal("successful run was not recognized as exact objective delivery")
	}
}

func clearPendingPatrolTriggers(tm *TriggerManager) {
	tm.mu.Lock()
	tm.pendingTriggers = nil
	tm.mu.Unlock()
}

func TestValidatePatrolObserverArtifactRejectsUnknownExecutableFields(t *testing.T) {
	now := time.Now().UTC()
	store := NewInMemoryPatrolObjectiveStore()
	objective := createInstallablePatrolObserver(t, store, now, "node-1")
	artifact, _ := store.GetObserverArtifact(objective.ID)
	artifact.Probe = []byte(`{"runtime":"pulse-resource-state/v1","path":"status","operator":"equals","value":"online","sample_interval_seconds":30,"wake_after_consecutive_failures":2,"command":"rm -rf /"}`)
	_, err := validatePatrolObserverArtifact(objective, artifact)
	if err == nil || err.code != "observer_probe_invalid" {
		t.Fatalf("unknown executable field validation error = %v", err)
	}
}

func TestValidatePatrolAvailabilityObserverRejectsModelSuppliedNetworkAuthority(t *testing.T) {
	now := time.Now().UTC()
	store := NewInMemoryPatrolObjectiveStore()
	objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep camera reachable", ResourceIDs: []string{"camera-1"}}, now)
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	objective, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
		ExpectedRevision: objective.Revision,
		Interpretation:   "Keep the endpoint reachable.",
		TriggerKinds:     []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
		ProbeJSON:        `{"runtime":"pulse-availability-state/v1","target_id":"camera-http","path":"probe_outcome","operator":"equals","value":"reachable","sample_interval_seconds":30,"wake_after_consecutive_failures":2}`,
		WakeEvidence:     "Endpoint reachability breached.", RequirementsJSON: `{}`,
	}, now)
	if err != nil {
		t.Fatalf("propose observer: %v", err)
	}
	artifact, _ := store.GetObserverArtifact(objective.ID)
	artifact.Probe = []byte(`{"runtime":"pulse-availability-state/v1","target_id":"camera-http","path":"probe_outcome","operator":"equals","value":"reachable","sample_interval_seconds":30,"wake_after_consecutive_failures":2,"url":"http://169.254.169.254/latest/meta-data"}`)
	_, validationErr := validatePatrolObserverArtifact(objective, artifact)
	if validationErr == nil || validationErr.code != "observer_probe_invalid" {
		t.Fatalf("model-supplied URL validation error = %v", validationErr)
	}
}

func TestPatrolObjectiveIntentEditInvalidatesInstalledObserver(t *testing.T) {
	now := time.Date(2026, 8, 14, 1, 0, 0, 0, time.UTC)
	store := NewInMemoryPatrolObjectiveStore()
	objective := createInstallablePatrolObserver(t, store, now, "node-1")
	patrol := NewPatrolService(nil, mockPatrolStateProvider{state: models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-1", Status: "online"}},
	}})
	patrol.SetObjectiveStore(store)
	patrol.processObjectiveObservers(now.Add(time.Second))
	installed, _ := store.Get(objective.ID, now.Add(time.Second))
	if installed.Coverage.State != PatrolObjectiveCovered {
		t.Fatalf("observer was not installed before edit: %+v", installed.Coverage)
	}

	revisedBrief := "Keep this node online and its workload responsive"
	updated, err := store.Update(objective.ID, UpdatePatrolObjectiveInput{
		ExpectedRevision: installed.Revision,
		Brief:            &revisedBrief,
		Actor:            "operator",
	}, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("update objective intent: %v", err)
	}
	if updated.Observer == nil || updated.Observer.State != PatrolObserverDisabled || updated.Observer.ValidUntil != nil {
		t.Fatalf("intent edit retained stale observer authority: %+v", updated.Observer)
	}
	if updated.Coverage.State != PatrolObjectiveUncovered || updated.Coverage.ReasonCode != "observer_disabled" {
		t.Fatalf("intent edit coverage = %+v", updated.Coverage)
	}
}
