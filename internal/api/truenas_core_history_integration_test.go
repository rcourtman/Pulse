package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/testutil/truenascore"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Complete the resource bridge at the actual authenticated chart route. Only
// the CORE server is synthetic: pinned TLS, REST decoding, provider, native
// poller fallback, monitor, tenant/scope checks and router are production code.
// No persistent database is needed for this already-supported native route.
func TestCORE13RESTReportingChartJourney(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })
	previousMulti := IsMultiTenantEnabled()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(previousMulti) })
	t.Setenv("PULSE_DEV", "true")
	fixture := truenascore.NewRESTFixture(t)
	fixture.WideHistory = true
	client, err := truenas.NewClient(truenas.ClientConfig{
		Host: fixture.Server.URL, APIKey: "synthetic-core-key", InsecureSkipVerify: true,
		Fingerprint: fmt.Sprintf("%x", sha256.Sum256(fixture.Server.Certificate().Raw)),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	provider := truenas.NewLiveProviderForConnection(&truenas.APIFetcher{Client: client}, "core-rest-fixture")
	poller := monitoring.NewTrueNASPoller(nil, time.Minute, nil)
	setUnexportedField(t, poller, "providersByOrg", map[string]map[string]*truenas.Provider{"org-core": {"core-rest-fixture": provider}})
	monitor := &monitoring.Monitor{}
	monitor.SetOrgID("org-core")
	setUnexportedField(t, monitor, "state", models.NewState())
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(20, time.Hour))
	const token = "core-history-read-fixture.12345678"
	readToken := newTokenRecord(t, token, []string{config.ScopeMonitoringRead}, nil)
	readToken.OrgID = "org-core"
	const wrongScope = "core-history-settings-fixture.12345678"
	scopeToken := newTokenRecord(t, wrongScope, []string{config.ScopeSettingsRead}, nil)
	scopeToken.OrgID = "org-core"
	cfg := newTestConfigWithTokens(t, readToken, scopeToken)
	cfg.AuthUser, cfg.AuthPass = "fixture-admin", "synthetic-password-hash"
	for _, org := range []string{"org-core", "org-other"} {
		if err := os.MkdirAll(filepath.Join(cfg.DataPath, "orgs", org), 0700); err != nil {
			t.Fatal(err)
		}
	}
	router := NewRouter(cfg, monitor, nil, nil, nil, "test")
	t.Cleanup(router.ShutdownBackgroundWorkers)
	t.Cleanup(router.ShutdownResourceStores)
	t.Cleanup(router.ShutdownRBAC)
	mtm := monitoring.NewMultiTenantMonitor(cfg, nil, nil)
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"org-core": monitor, "org-other": {}})
	router.mtMonitor = mtm
	// Match the production tenant resource binding as well as the chart
	// monitor binding; assigning the chart owner alone leaves resource reads
	// without their tenant snapshot provider.
	router.resourceHandlers.SetTenantStateProvider(NewMultiTenantStateProvider(mtm, monitor))
	router.setMonitorSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, poller)
	const resourcePath = "/api/resources?source=truenas&type=agent,storage,physical_disk&limit=100"
	const historyPath = "/api/metrics-store/history?resourceType=agent&resourceId=core-rest-fixture&range=1h&maxPoints=240"
	request := func(raw, org, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Pulse-Org-ID", org)
		if raw != "" {
			req.Header.Set("X-API-Token", raw)
		}
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}
	for _, test := range []struct {
		raw, org string
		want     int
	}{
		{"", "org-core", 401}, {wrongScope, "org-core", 403}, {token, "org-other", 403},
	} {
		rec := request(test.raw, test.org, historyPath)
		if rec.Code != test.want || fixture.Posts.Load() != 0 {
			t.Fatalf("auth/scope/tenant boundary: status %d want %d native reads %d", rec.Code, test.want, fixture.Posts.Load())
		}
	}
	type point struct {
		Timestamp       int64 `json:"timestamp"`
		Value, Min, Max float64
	}
	var lastCollected time.Time
	for i, phase := range []string{"complete", "rejected", "partial", "zero", "recovered"} {
		t.Run(phase, func(t *testing.T) {
			fixture.Phase.Store(int64(i))
			beforePosts := fixture.Posts.Load()
			if err := provider.Refresh(context.Background()); err != nil {
				t.Fatal(err)
			}
			snapshot := provider.Snapshot()
			if !snapshot.CollectedAt.After(lastCollected) {
				t.Fatal("collection stopped advancing")
			}
			lastCollected = snapshot.CollectedAt
			setUnexportedField(t, poller, "cachedRecordsByOrg", map[string]map[string][]unifiedresources.IngestRecord{"org-core": {"core-rest-fixture": provider.Records()}})
			// Publish the refreshed canonical records through the normal wiring boundary.
			monitor.SetResourceStoreAndProviders(unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)), map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: poller})
			resources := request(token, "org-core", resourcePath)
			var listed struct {
				Data []unifiedresources.Resource `json:"data"`
			}
			if resources.Code != 200 || json.Unmarshal(resources.Body.Bytes(), &listed) != nil {
				t.Fatalf("resources %d: %s", resources.Code, resources.Body.String())
			}
			var host *unifiedresources.Resource
			for j := range listed.Data {
				if listed.Data[j].Type == unifiedresources.ResourceTypeAgent {
					host = &listed.Data[j]
				}
			}
			if host == nil || host.MetricsTarget == nil || host.MetricsTarget.ResourceID != "core-rest-fixture" || host.Agent == nil || host.Agent.Memory == nil {
				t.Fatalf("canonical host/target lost: %s", resources.Body.String())
			}
			memory := host.Agent.Memory
			knownCPU, knownMemory := i != 1, i != 1 && i != 2
			if (host.Metrics.CPU != nil) != knownCPU || (host.Metrics.Memory != nil) != knownMemory || memory.UsageUnavailable == knownMemory {
				t.Fatalf("unknown became idle: %+v %+v", host.Metrics, memory)
			}
			if memory.Total != 16<<30 || host.Metrics.Disk == nil || host.Metrics.Disk.Percent != 40 {
				t.Fatal("capacity was lost or substituted for telemetry")
			}
			if knownMemory {
				used, free, cache := int64(6<<30), int64(2<<30), int64(8<<30)
				if i == 3 {
					used, free, cache = 0, 16<<30, 0
				}
				if memory.Used != used || memory.Free != free || memory.Cache != cache {
					t.Fatalf("free/ARC/effective used changed: %+v", memory)
				}
			}
			history24 := request(token, "org-core", strings.Replace(historyPath, "range=1h", "range=24h", 1))
			history := request(token, "org-core", historyPath)
			wantCalls := int64(3)
			if i == 1 {
				wantCalls = 21
			}
			if fixture.Posts.Load()-beforePosts != wantCalls || fixture.RejectedOldKey.Load() != 0 {
				t.Fatal("chart did not traverse native reporting, or repeated the rejected envelope")
			}
			if history24.Code != history.Code {
				t.Fatal("default-range chart status disagrees with selected range")
			}
			if i == 1 {
				if history.Code != 503 || snapshot.System.Telemetry.ErrorCategory != "request_rejected" {
					t.Fatalf("rejection disguised as healthy History: %d %s", history.Code, history.Body.String())
				}
			} else {
				var data struct {
					ResourceType, ResourceID string
					Metrics                  map[string][]point
				}
				if history.Code != 200 || json.Unmarshal(history.Body.Bytes(), &data) != nil || data.ResourceType != "agent" || data.ResourceID != "core-rest-fixture" {
					t.Fatalf("chart response %d: %s", history.Code, history.Body.String())
				}
				expected := map[string]float64{"cpu": 12, "memory": 37.5, "diskread": 1024, "diskwrite": 2048, "netin": 32, "netout": 64, "temperature": 42}
				for metric, value := range expected {
					known := knownMemory || metric == "cpu"
					if !known {
						if len(data.Metrics[metric]) != 0 {
							t.Fatalf("partial chart fabricated %s", metric)
						}
						continue
					}
					if i == 3 && metric != "temperature" {
						value = 0
					}
					points := data.Metrics[metric]
					if len(points) != 4 {
						t.Fatalf("native %s observations lost: %+v", metric, points)
					}
					for j, p := range points {
						if p.Value != value || p.Min != value || p.Max != value || (j > 0 && p.Timestamp <= points[j-1].Timestamp) {
							t.Fatalf("native sample/bounds/order changed for %s: %+v", metric, points)
						}
					}
					if points[3].Timestamp-points[0].Timestamp < 57*60*1000 {
						t.Fatal("hour chart became a current-value shortcut")
					}
				}
			}
			payload, err := json.Marshal(map[string]any{"phase": phase, "resources": json.RawMessage(resources.Body.Bytes()), "history": map[string]any{"status": history.Code, "body": json.RawMessage(history.Body.Bytes())}, "history24": map[string]any{"status": history24.Code, "body": json.RawMessage(history24.Body.Bytes())}, "telemetry": snapshot.System.Telemetry})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("CORE_CHART_BROWSER %s", payload)
		})
	}
	before := fixture.Posts.Load()
	if cfg.RemoveAPIToken(readToken.ID) == nil {
		t.Fatal("fixture token missing")
	}
	cfg.SortAPITokens()
	if rec := request(token, "org-core", historyPath); rec.Code != 401 || fixture.Posts.Load() != before {
		t.Fatal("revoked token still reached native History")
	}
}
