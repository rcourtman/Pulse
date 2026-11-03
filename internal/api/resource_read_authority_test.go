package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// All inventory-backed entrances into the unified query service. The same
// admitted provider pause covers summaries, details, facets and nested reads,
// not just a handler wrapped directly by the test.
var resourceReadAuthorityPaths = []string{
	"/api/resources",
	"/api/resources/stats",
	"/api/resources/storage-summary",
	"/api/resources/storage-incidents",
	"/api/resources/k8s/namespaces?cluster=synthetic-cluster",
	"/api/resources/vm:101",
	"/api/resources/vm:101/facets",
	"/api/resources/vm:101/children",
	"/api/resources/vm:101/metrics",
}

func newResourceReadAuthorityFixture(t *testing.T, path string, scopes []string) *snapshotAuthorityFixture {
	t.Helper()
	f := newSnapshotAuthorityFixture(t, path, scopes)
	now := time.Now().UTC()
	parent := "vm:101"
	f.provider.resources[0].Metrics = &unified.ResourceMetrics{CPU: &unified.MetricValue{Value: 23, Percent: 23, Unit: "%"}}
	f.provider.resources = append(f.provider.resources,
		unified.Resource{ID: "storage:synthetic", Type: unified.ResourceTypeStorage, Name: "synthetic-pool", Status: unified.StatusOnline, LastSeen: now, UpdatedAt: now, Storage: &unified.StorageMeta{Type: "zfs", Enabled: true, Active: true}, ParentID: &parent},
		unified.Resource{ID: "pod:synthetic", Type: unified.ResourceTypePod, Name: "synthetic-pod", Status: unified.StatusOnline, LastSeen: now, UpdatedAt: now, Kubernetes: &unified.K8sData{ClusterName: "synthetic-cluster", Namespace: "synthetic-namespace"}},
	)
	return f
}

func checkResourceReadAuthorityCurrent(t *testing.T, path string, wire snapshotAuthorityWire) {
	t.Helper()
	// Assert content, not merely a successful but empty JSON response.
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(wire.Body), &body); err != nil {
		t.Fatal(err)
	}
	switch {
	case path == "/api/resources":
		if !strings.Contains(wire.Body, "synthetic-database") || !strings.Contains(wire.Body, "synthetic-pool") {
			t.Fatalf("inventory lost resources: %s", wire.Body)
		}
	case strings.HasSuffix(path, "/stats"):
		var total int
		if json.Unmarshal(body["total"], &total) != nil || total != 3 {
			t.Fatalf("statistics lost inventory: %s", wire.Body)
		}
	case strings.HasSuffix(path, "/storage-summary"):
		var total int
		if json.Unmarshal(body["totalResources"], &total) != nil || total != 1 {
			t.Fatalf("summary lost storage count: %s", wire.Body)
		}
	case strings.HasSuffix(path, "/storage-incidents"):
		if len(body["sections"]) == 0 {
			t.Fatalf("incidents lost their envelope: %s", wire.Body)
		}
	case strings.Contains(path, "/k8s/namespaces"):
		if !strings.Contains(wire.Body, "synthetic-namespace") {
			t.Fatalf("namespace lost its pod: %s", wire.Body)
		}
	case strings.HasSuffix(path, "/facets"):
		if !strings.Contains(wire.Body, "synthetic_restart") {
			t.Fatalf("facets lost capabilities: %s", wire.Body)
		}
	case strings.HasSuffix(path, "/children"):
		if !strings.Contains(wire.Body, "synthetic-pool") {
			t.Fatalf("children lost parentage: %s", wire.Body)
		}
	case strings.HasSuffix(path, "/metrics"):
		if !strings.Contains(wire.Body, `"value":23`) {
			t.Fatalf("metrics lost readings: %s", wire.Body)
		}
	default:
		if !strings.Contains(wire.Body, "synthetic-database") {
			t.Fatalf("detail lost resource: %s", wire.Body)
		}
	}
}

func TestResourceReadsWithdrawnTokenCannotDisclose(t *testing.T) {
	for _, path := range resourceReadAuthorityPaths {
		for _, transport := range []string{"header", "bearer"} {
			for _, change := range []string{"revoked", "expired", "scope", "organization", "credential", "principal"} {
				t.Run(path+"/"+transport+"/"+change, func(t *testing.T) {
					f := newResourceReadAuthorityFixture(t, path, []string{config.ScopeMonitoringRead})
					control := f.begin(t, snapshotAuthorityControl, "header")
					primary := f.begin(t, snapshotAuthorityToken, transport)
					f.changeToken(t, change)
					f.provider.unblock()
					readSnapshotAuthorityResult(t, primary, http.StatusUnauthorized)
					checkResourceReadAuthorityCurrent(t, path, readSnapshotAuthorityResult(t, control, http.StatusOK))
					if change == "credential" || change == "principal" {
						checkResourceReadAuthorityCurrent(t, path, f.current(t, snapshotAuthorityToken, transport, http.StatusOK))
					} else {
						response, err := f.server.Client().Do(f.request(t, snapshotAuthorityToken, transport))
						if err != nil {
							t.Fatal(err)
						}
						response.Body.Close()
						if response.StatusCode == http.StatusOK {
							t.Error("fresh read reused withdrawn authority")
						}
					}
				})
			}
		}
	}
}

func TestResourceReadsWithdrawnSessionCannotDisclose(t *testing.T) {
	for _, path := range resourceReadAuthorityPaths {
		for _, change := range []string{"logout", "expired", "replaced", "principal", "store"} {
			t.Run(path+"/"+change, func(t *testing.T) {
				f := newResourceReadAuthorityFixture(t, path, []string{config.ScopeMonitoringRead})
				const raw = "synthetic-resource-read-session"
				GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", f.router.config.AuthUser)
				t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
				control := f.begin(t, snapshotAuthorityControl, "header")
				primary := f.begin(t, raw, "session")
				if change == "logout" {
					logoutReadSession(t, f.server.URL, raw)
				} else if change == "store" {
					InitSessionStore(t.TempDir())
				} else {
					mutateReadSession(raw, change)
				}
				f.provider.unblock()
				readSnapshotAuthorityResult(t, primary, http.StatusUnauthorized)
				checkResourceReadAuthorityCurrent(t, path, readSnapshotAuthorityResult(t, control, http.StatusOK))
			})
		}
	}
}

func TestResourceReadsWithdrawalWithholdsErrors(t *testing.T) {
	for _, path := range []string{"/api/resources/missing", "/api/resources/missing/facets", "/api/resources/missing/metrics", "/api/resources?type=synthetic-invalid-type"} {
		t.Run(path, func(t *testing.T) {
			f := newResourceReadAuthorityFixture(t, path, []string{config.ScopeMonitoringRead})
			primary := f.begin(t, snapshotAuthorityToken, "header")
			f.changeToken(t, "revoked")
			f.provider.unblock()
			readSnapshotAuthorityResult(t, primary, http.StatusUnauthorized)
			status := http.StatusNotFound
			if strings.Contains(path, "?type=") {
				status = http.StatusBadRequest
			}
			f.current(t, snapshotAuthorityControl, "header", status)
		})
	}
}

func TestResourceReadsKeepAdmissionAndRecovery(t *testing.T) {
	for _, path := range resourceReadAuthorityPaths {
		t.Run(path, func(t *testing.T) {
			f := newResourceReadAuthorityFixture(t, path, []string{config.ScopeSettingsRead})
			f.provider.unblock()
			f.current(t, "", "header", http.StatusUnauthorized)
			f.current(t, snapshotAuthorityToken, "header", http.StatusForbidden)
			checkResourceReadAuthorityCurrent(t, path, f.current(t, snapshotAuthorityControl, "bearer", http.StatusOK))
			config.Mu.Lock()
			f.router.config.APITokens[0].Scopes = []string{config.ScopeMonitoringRead}
			config.Mu.Unlock()
			checkResourceReadAuthorityCurrent(t, path, f.current(t, snapshotAuthorityToken, "header", http.StatusOK))
			checkResourceReadAuthorityCurrent(t, path, f.current(t, "", "proxy", http.StatusOK))
		})
	}
}

// The store-only query routes share the same guarded registration. Exercise
// their live routing/admission and stable envelopes without database locks or
// crash experiments; the response writer's delayed-write tests cover the guard.
func TestResourceReadsStoreOnlyEntrancesKeepAdmission(t *testing.T) {
	for _, path := range []string{"/api/resources/timeline", "/api/resources/vm:101/timeline", "/api/resources/vm:101/operator-state?view=lookup", "/api/resources/vm:101/maintenance-verifications"} {
		t.Run(path, func(t *testing.T) {
			f := newResourceReadAuthorityFixture(t, path, []string{config.ScopeSettingsRead})
			f.provider.unblock()
			f.current(t, "", "header", http.StatusUnauthorized)
			f.current(t, snapshotAuthorityToken, "header", http.StatusForbidden)
			wire := f.current(t, snapshotAuthorityControl, "header", http.StatusOK)
			if strings.Contains(path, "timeline") && (!strings.Contains(wire.Body, `"recentChanges":[]`) || !strings.Contains(wire.Body, `"count":0`)) {
				t.Fatalf("timeline lost empty evidence: %s", wire.Body)
			}
		})
	}
}

func TestResourceReadMissingVerificationKeepsAdmission(t *testing.T) {
	f := newResourceReadAuthorityFixture(t, "/api/maintenance-verifications/missing", []string{config.ScopeSettingsRead})
	f.provider.unblock()
	f.current(t, "", "header", http.StatusUnauthorized)
	f.current(t, snapshotAuthorityToken, "header", http.StatusForbidden)
	f.current(t, snapshotAuthorityControl, "header", http.StatusNotFound)
}

func TestResourceReadScopeCannotMutate(t *testing.T) {
	for _, mutation := range []struct{ method, path string }{
		{http.MethodPost, "/api/resources/vm:101/link"},
		{http.MethodPost, "/api/resources/vm:101/unlink"},
		{http.MethodPost, "/api/resources/vm:101/report-merge"},
		{http.MethodPost, "/api/resources/vm:101/maintenance-verifications/rerun"},
		{http.MethodPut, "/api/resources/vm:101/operator-state"},
		{http.MethodDelete, "/api/resources/vm:101/operator-state"},
		{http.MethodPost, "/api/maintenance-verifications/missing/review"},
	} {
		t.Run(mutation.method+mutation.path, func(t *testing.T) {
			f := newResourceReadAuthorityFixture(t, mutation.path, []string{config.ScopeMonitoringRead})
			f.provider.unblock()
			req := f.request(t, snapshotAuthorityToken, "header")
			req.Method = mutation.method
			response, err := f.server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusForbidden {
				t.Errorf("read credential mutation = %d", response.StatusCode)
			}
		})
	}
}
