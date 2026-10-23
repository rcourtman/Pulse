package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func TestPBSDatastoreDiscoveryFailureQualifiesRetainedHistory(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		status int
		body   string
		denied bool
	}{
		{name: "server failure", status: 503},
		{name: "unauthenticated", status: 401, denied: true},
		{name: "forbidden", status: 403, denied: true},
		{name: "missing endpoint is not permission evidence", status: 404},
		{name: "malformed envelope", body: `{"data":`},
		{name: "missing data", body: `{}`},
		{name: "null data", body: `{"data":null}`},
		{name: "nameless datastore", body: `{"data":[{}]}`},
	} {
		for _, cold := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cold-%t", scenario.name, cold), func(t *testing.T) {
				f := newPBSNamespacePollFixture(t)
				f.poll(t, "pbs-main")
				before := f.points(t, "pbs-main")
				if len(before) != 2 {
					t.Fatal("healthy control did not ingest root and tenant")
				}
				if cold {
					vms := f.m.state.GetSnapshot().VMs
					f.m.state = models.NewState()
					f.m.state.UpdateVMs(vms)
				}
				raw := f.m.state.GetSnapshot().PBSBackups
				key := pbsBackupGroupKey{datastore: "archive", backupType: "vm", backupID: "100"}
				cacheAt := f.m.pbsBackupCacheTimeFor("pbs-main", key)
				reads := f.backupReads.Load()
				f.datastoreStatus.Store(int32(scenario.status))
				if scenario.body != "" {
					f.datastoreBody.Store(&scenario.body)
				}
				for range 2 {
					f.poll(t, "pbs-main")
					if !reflect.DeepEqual(before, f.points(t, "pbs-main")) || !reflect.DeepEqual(raw, f.m.state.GetSnapshot().PBSBackups) {
						t.Error("failed discovery deleted or renewed retained artifacts")
					}
					if f.backupReads.Load() != reads || !f.m.pbsBackupCacheTimeFor("pbs-main", key).Equal(cacheAt) {
						t.Error("failed discovery guessed a backup read or renewed cache")
					}
					permissions := operationaltrust.EvidencePermissionsUnknown
					if scenario.denied {
						permissions = operationaltrust.EvidencePermissionsDenied
					}
					for _, p := range f.postures(t) {
						if p.State == recovery.ProtectionStateProtected || len(p.ProviderStates) != 1 {
							t.Errorf("failed discovery left protection healthy: %+v", p)
							continue
						}
						provider := p.ProviderStates[0]
						if provider.HistoryCompleteness != recovery.ProtectionHistoryUnavailable || provider.Permissions != permissions || provider.JobState != recovery.OutcomeFailed {
							t.Errorf("failure quality lost: %+v", provider)
						}
					}
					// Backup history availability is not node connectivity.
					inst := pbsInstanceByName(t, f.m.state.GetSnapshot(), "pbs-main")
					if inst.Status != "online" || inst.ConnectionHealth != "healthy" {
						t.Errorf("discovery failure rewrote connectivity: %+v", inst)
					}
				}
				f.datastoreStatus.Store(0)
				f.datastoreBody.Store(nil)
				f.poll(t, "pbs-main")
				if len(f.points(t, "pbs-main")) != 2 {
					t.Error("ordinary recovery did not retain both archives")
				}
				for _, p := range f.postures(t) {
					if p.State != recovery.ProtectionStateProtected {
						t.Errorf("complete recovery did not restore protection: %+v", p)
					}
				}
			})
		}
	}
}

func TestPBSDatastoreDiscoveryEmptyReconcilesOnlyAcceptedScope(t *testing.T) {
	for _, excluded := range []bool{false, true} {
		t.Run(fmt.Sprintf("excluded-%t", excluded), func(t *testing.T) {
			f := newPBSNamespacePollFixture(t)
			f.poll(t, "pbs-other")
			other := f.points(t, "pbs-other")
			f.poll(t, "pbs-main")
			reads := f.backupReads.Load()
			if excluded {
				f.m.config.PBSInstances[0].ExcludeDatastores = []string{"archive"}
			} else {
				empty := `{"data":[]}`
				f.datastoreBody.Store(&empty)
			}
			for range 2 {
				f.poll(t, "pbs-main")
				if len(f.points(t, "pbs-main")) != 0 {
					t.Error("accepted empty scope retained nonexistent/removed archives")
				}
				for _, row := range f.m.state.GetSnapshot().PBSBackups {
					if row.Instance == "pbs-main" {
						t.Error("accepted empty scope retained a live archive")
					}
				}
				if !reflect.DeepEqual(other, f.points(t, "pbs-other")) || f.backupReads.Load() != reads {
					t.Error("empty reconciliation crossed scope or guessed a backup request")
				}
				f.m.mu.RLock()
				cacheSize := len(f.m.pbsBackupCacheTime["pbs-main"])
				f.m.mu.RUnlock()
				if cacheSize != 0 {
					t.Error("confirmed empty scope retained snapshot cache timestamps")
				}
			}
			f.datastoreBody.Store(nil)
			f.m.config.PBSInstances[0].ExcludeDatastores = nil
			f.poll(t, "pbs-main")
			if len(f.points(t, "pbs-main")) != 2 || !reflect.DeepEqual(other, f.points(t, "pbs-other")) {
				t.Error("scope restoration did not rediscover archives independently")
			}
		})
	}
}

func TestPBSDatastoreDiscoverySkippedScopeIsNotEmpty(t *testing.T) {
	for _, skip := range []string{"backups disabled", "datastores disabled", "global disabled", "not due", "in flight", "cancelled cycle", "cancelled runtime"} {
		t.Run(skip, func(t *testing.T) {
			f := newPBSNamespacePollFixture(t)
			f.poll(t, "pbs-main")
			before, raw := f.points(t, "pbs-main"), f.m.state.GetSnapshot().PBSBackups
			empty := `{"data":[]}`
			f.datastoreBody.Store(&empty)
			ctx := context.Background()
			switch skip {
			case "backups disabled":
				f.m.config.PBSInstances[0].MonitorBackups = false
			case "datastores disabled":
				f.m.config.PBSInstances[0].MonitorDatastores = false
			case "global disabled":
				f.m.SetBackupPollingEnabled(false)
			case "not due":
				f.m.SetBackupPollingInterval(time.Hour)
			case "in flight":
				f.m.pbsBackupPollers["pbs-main"] = true
			case "cancelled cycle":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			case "cancelled runtime":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				f.m.runtimeCtx = cancelled
			}
			f.m.pollPBSInstance(ctx, "pbs-main", f.client)
			if skip == "in flight" {
				delete(f.m.pbsBackupPollers, "pbs-main")
			}
			f.wait(t)
			if !reflect.DeepEqual(before, f.points(t, "pbs-main")) || !reflect.DeepEqual(raw, f.m.state.GetSnapshot().PBSBackups) {
				t.Error("skipped/cancelled discovery was treated as accepted empty scope")
			}
		})
	}
}

func TestPBSDatastoreDiscoveryFailedConnectivityFallback(t *testing.T) {
	f := newPBSNamespacePollFixture(t)
	f.poll(t, "pbs-main")
	before := f.points(t, "pbs-main")
	f.versionStatus.Store(http.StatusServiceUnavailable)
	f.datastoreStatus.Store(http.StatusServiceUnavailable)
	f.poll(t, "pbs-main")
	if !reflect.DeepEqual(before, f.points(t, "pbs-main")) {
		t.Error("failed fallback destroyed history")
	}
	for _, p := range f.postures(t) {
		if p.State == recovery.ProtectionStateProtected || len(p.ProviderStates) != 1 || p.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryUnavailable {
			t.Errorf("failed fallback left last good protection: %+v", p)
		}
	}
}
