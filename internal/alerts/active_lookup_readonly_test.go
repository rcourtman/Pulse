package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

// Read-lock callers include SMART evidence checks, linked-agent ownership and
// storage capacity evaluation. Lookup must not repair the shared alias index
// under that lock, even for old storage keys or a dangling restored alias.
func TestActiveAlertLookupsAreReadOnly(t *testing.T) {
	resourceID := "agent:lookup-fixture"
	stateID := canonicalMetricStateID(resourceID, "cpu")
	for _, scenario := range []string{
		"direct", "alias", "missing", "dangling-alias", "canonical-fallback",
		"nil-alias-index", "dangling-alias-fallback", "metadata-identity",
		"inferred-identity", "namesake", "nil-direct-entry",
	} {
		for _, operation := range []string{"get", "has"} {
			t.Run(scenario+"/"+operation, func(t *testing.T) {
				alert := &Alert{
					ID: "legacy-alert", Type: "cpu", ResourceID: resourceID,
					CanonicalSpecID: "metric-threshold:cpu", CanonicalKind: "metric_threshold",
					CanonicalState: stateID, Value: 93,
				}
				m := &Manager{
					activeAlerts:     map[string]*Alert{"legacy-storage": alert},
					activeAlertAlias: make(map[string]string),
				}
				id, want := stateID, true
				switch scenario {
				case "direct":
					id = "legacy-storage"
				case "alias":
					id = "saved-alias"
					m.activeAlertAlias[id] = "legacy-storage"
				case "missing":
					id, want = "absent", false
				case "dangling-alias":
					id, want = "retired-alias", false
					m.activeAlertAlias[id] = "retired-storage"
				case "nil-alias-index":
					m.activeAlertAlias = nil
				case "dangling-alias-fallback":
					m.activeAlertAlias[id] = "retired-storage"
				case "metadata-identity":
					alert.CanonicalSpecID, alert.CanonicalKind, alert.CanonicalState = "", "", ""
					alert.Metadata = map[string]interface{}{
						"canonicalSpecID": "metric-threshold:cpu", "canonicalAlertKind": "metric_threshold",
					}
				case "inferred-identity":
					alert.ID, alert.Type, alert.ResourceID = "host-offline-readonly", "connectivity", ""
					alert.CanonicalSpecID, alert.CanonicalKind, alert.CanonicalState = "", "", ""
					id = canonicalConnectivityStateID("agent:readonly")
				case "namesake":
					id, want = canonicalMetricStateID("agent:lookup-fixture::other-site", "cpu"), false
				case "nil-direct-entry":
					// Keep existing semantics: a direct storage entry wins over
					// fallback, and has reports key membership even for nil.
					m.activeAlerts[id] = nil
					want = operation == "has"
				}
				aliasesBefore := maps.Clone(m.activeAlertAlias)
				alertsBefore, err := json.Marshal(m.activeAlerts)
				if err != nil {
					t.Fatal(err)
				}
				m.mu.RLock()
				var got bool
				if operation == "get" {
					var found *Alert
					found, got = m.getActiveAlertNoLock(id)
					if got && found != alert {
						t.Error("lookup returned another occurrence")
					}
				} else {
					got = m.hasActiveAlertNoLock(id)
				}
				m.mu.RUnlock()
				if got != want {
					t.Errorf("lookup(%q) = %v, want %v", id, got, want)
				}
				if !reflect.DeepEqual(m.activeAlertAlias, aliasesBefore) {
					t.Errorf("read changed alias index: got %#v, before %#v", m.activeAlertAlias, aliasesBefore)
				}
				alertsAfter, err := json.Marshal(m.activeAlerts)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(alertsBefore, alertsAfter) {
					t.Error("read changed alert identity or payload")
				}
			})
		}
	}
}

// Drive real read-lock consumers alongside lookup and index readers. The
// fixture deliberately has no workers, event store or persistence, so this
// test isolates alias ownership instead of depending on a timing-sensitive
// alert fire or a database failure.
func TestActiveAlertReadLockConsumersConcurrent(t *testing.T) {
	resourceID := "agent:read-lock"
	metricID := canonicalMetricStateID(resourceID, "cpu")
	capacityID := canonicalMetricStateID(resourceID, "usage")
	diskResourceID := resourceID + "/disk:serial"
	diskID := buildCanonicalStateID(diskResourceID, diskResourceID+"-disk-health")
	m := &Manager{
		core: reducer.NewState(),
		activeAlerts: map[string]*Alert{
			"legacy-cpu": {ID: "old-cpu", Type: "cpu", CanonicalState: metricID, Value: 93},
			"legacy-capacity": {ID: "old-capacity", Type: "usage", CanonicalState: capacityID,
				Metadata: map[string]interface{}{capacityAlertOriginKey: capacityAlertOriginForecast}},
			"legacy-disk": {ID: "old-disk", Type: "disk-health", CanonicalState: diskID,
				Metadata: map[string]interface{}{"riskCodes": []string{"smart-health-failed"}}},
		},
		activeAlertAlias: map[string]string{"retired": "retired-storage"},
	}
	aliasesBefore := maps.Clone(m.activeAlertAlias)
	const readers, iterations = 8, 200
	start := make(chan struct{})
	errs := make(chan error, readers)
	var wg sync.WaitGroup
	for worker := 0; worker < readers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				if value, ok := m.activeAlertValue(metricID); !ok || value != 93 {
					errs <- fmt.Errorf("active metric value = %v/%v", value, ok)
					return
				}
				if !m.hostAgentMetricOpen(resourceID, "cpu") {
					errs <- fmt.Errorf("linked agent lost its open metric")
					return
				}
				if active, origin := m.activeStorageCapacityOrigin(resourceID); !active || origin != capacityAlertOriginForecast {
					errs <- fmt.Errorf("storage capacity origin = %v/%q", active, origin)
					return
				}
				if m.hostSMARTDiskAlertEvidenceKnown(diskResourceID, "disk-health", models.HostDiskSMART{Standby: true}, nil, storagehealth.SMARTThresholds{HealthFailure: true}, 0) {
					errs <- fmt.Errorf("standby became positive SMART recovery evidence")
					return
				}
				m.mu.RLock()
				_, retired := m.getActiveAlertNoLock("retired")
				for alias, storageKey := range m.activeAlertAlias {
					if alias != "retired" || storageKey != "retired-storage" {
						errs <- fmt.Errorf("lookup published an alias")
						m.mu.RUnlock()
						return
					}
				}
				m.mu.RUnlock()
				if retired {
					errs <- fmt.Errorf("dangling alias became an active occurrence")
					return
				}
				runtime.Gosched()
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if !reflect.DeepEqual(m.activeAlertAlias, aliasesBefore) {
		t.Fatal("read-lock consumers changed the alias index")
	}
}

func TestActiveAlertAliasWriterLifecycle(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(m.Stop)
	resourceID := "agent:restored-alias"
	legacyID := resourceID + "-cpu"
	stateID := canonicalMetricStateID(resourceID, "cpu")
	now := time.Now()
	if err := m.restoreActiveAlertSnapshots([]*Alert{{
		ID: legacyID, Type: "cpu", Level: AlertLevelWarning, ResourceID: resourceID,
		StartTime: now.Add(-time.Minute), LastSeen: now, Value: 93,
	}}, "read-only lookup fixture", false); err != nil {
		t.Fatal(err)
	}
	m.mu.RLock()
	alias, registered := m.activeAlertAlias[legacyID]
	m.mu.RUnlock()
	if !registered || alias != stateID {
		t.Fatal("restoration did not register the saved ID under the writer lock")
	}
	for _, id := range []string{legacyID, stateID} {
		if value, ok := m.activeAlertValue(id); !ok || value != 93 {
			t.Fatalf("restored value by %q = %v/%v", id, value, ok)
		}
		if err := m.AcknowledgeAlert(id, "fixture-operator"); err != nil {
			t.Fatal(err)
		}
		if err := m.UnacknowledgeAlert(id); err != nil {
			t.Fatal(err)
		}
	}
	if !m.ClearAlert(legacyID) {
		t.Fatal("saved ID no longer clears the occurrence")
	}
	for _, id := range []string{legacyID, stateID} {
		if _, ok := m.activeAlertValue(id); ok {
			t.Fatalf("cleared occurrence still found by %q", id)
		}
	}
}
