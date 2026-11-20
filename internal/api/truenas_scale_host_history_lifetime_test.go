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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestTrueNASScaleHostHistoryDisconnectPreservesPollingSession(t *testing.T) {
	previous, multi := truenas.IsFeatureEnabled(), IsMultiTenantEnabled()
	truenas.SetFeatureEnabled(true)
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous); SetMultiTenantEnabled(multi) })
	t.Setenv("PULSE_DEV", "true")
	const id, org = "scale-target", "org-scale-history"
	const token = "scale-history-reader.1234567890123456789"

	var sessions, auths, polls, reads atomic.Int32
	held, release := make(chan struct{}), make(chan struct{})
	releaseHistory := sync.OnceFunc(func() { close(release) })
	// Keep native samples fixed across the abandoned and fresh requests.
	start := time.Now().Add(-54 * time.Minute).Unix()
	upgrader := websocket.Upgrader{}
	nas := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/current" {
			t.Errorf("unexpected SCALE transport: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		sessions.Add(1)
		for {
			var req struct {
				ID     any
				Method string
				Params []json.RawMessage
			}
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			var result any
			var collection string
			switch req.Method {
			case "auth.login_with_api_key":
				auths.Add(1)
				result = true
			case "system.info":
				polls.Add(1)
				result = map[string]any{"hostname": "synthetic-scale", "version": "TrueNAS-SCALE-25.04.2", "cores": 4, "physmem": 4 << 30}
			case "reporting.get_data":
				var graphs []struct{ Name string }
				if len(req.Params) != 2 || json.Unmarshal(req.Params[0], &graphs) != nil || len(graphs) == 0 {
					t.Error("invalid SCALE reporting request")
					return
				}
				result = []any{}
				if graphs[0].Name == "cpu" {
					if reads.Add(1) == 1 {
						close(held)
						<-release
					}
					rows := func(i string, r, w float64) map[string]any {
						return map[string]any{"name": "disk", "identifier": i, "start": start, "end": start + 3000, "step": 1000,
							"legend": []string{"read", "write"}, "data": [][]float64{{r, w}, {r, w}, {r, w}, {r, w}}}
					}
					result = []any{rows("sda", 100, 200), rows("sdb", 1000, 2000)}
				}
			case "core.subscribe":
				if len(req.Params) != 1 || json.Unmarshal(req.Params[0], &collection) != nil || !strings.HasPrefix(collection, "reporting.realtime:") {
					t.Error("unexpected fixture subscription")
					return
				}
				result = "synthetic-realtime-subscription"
			case "core.unsubscribe":
				result = nil
			case "disk.query", "alert.list", "service.query", "app.query", "vm.query", "sharing.smb.query", "sharing.nfs.query":
				result = []any{}
			default:
				t.Errorf("unexpected SCALE method %q", req.Method)
				return
			}
			if err := conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}); err != nil {
				return
			}
			if collection != "" {
				if err := conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "method": "collection_update", "params": map[string]any{
					"collection": collection, "fields": map[string]any{"cpu": map[string]any{"usage": 12}},
				}}); err != nil {
					return
				}
			}
		}
	}))
	t.Cleanup(nas.Close)
	client, err := truenas.NewClient(truenas.ClientConfig{
		Host: nas.URL, APIKey: "synthetic-scale-key", InsecureSkipVerify: true,
		Fingerprint: fmt.Sprintf("%x", sha256.Sum256(nas.Certificate().Raw)),
		SkipPools:   true, SkipDatasets: true, SkipReplication: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	t.Cleanup(releaseHistory)
	provider := truenas.NewLiveProviderForConnection(&truenas.APIFetcher{Client: client}, id)
	poll := func() {
		t.Helper()
		if err := provider.Refresh(context.Background()); err != nil {
			t.Fatal(err)
		}
		s := provider.Snapshot().System
		if s.Telemetry == nil || !s.Telemetry.CPU || s.CPUPercent != 12 {
			t.Fatalf("ordinary SCALE poll did not collect realtime CPU: %+v", s.Telemetry)
		}
	}
	poll()
	assertSession := func() {
		t.Helper()
		s := provider.TransportStatus()
		if s.Mode != truenas.TransportJSONRPC || !s.Connected || s.LastError != "" || s.Reconnects != 0 || sessions.Load() != 1 || auths.Load() != 1 {
			t.Fatalf("shared SCALE session changed: mode %s connected %t error %q reconnects %d sessions/auths %d/%d",
				s.Mode, s.Connected, s.LastError, s.Reconnects, sessions.Load(), auths.Load())
		}
	}
	assertSession()
	poller := monitoring.NewTrueNASPoller(nil, time.Minute, nil)
	monitor := &monitoring.Monitor{}
	monitor.SetOrgID(org)
	setUnexportedField(t, monitor, "state", models.NewState())
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(1024, 24*time.Hour))
	setUnexportedField(t, poller, "providersByOrg", map[string]map[string]*truenas.Provider{org: {id: provider}})
	setUnexportedField(t, poller, "cachedRecordsByOrg", map[string]map[string][]unifiedresources.IngestRecord{org: {id: provider.Records()}})
	monitor.SetResourceStoreAndProviders(unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
		map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: poller})
	record := newTokenRecord(t, token, []string{config.ScopeMonitoringRead}, nil)
	record.OrgID = org
	cfg := newTestConfigWithTokens(t, record)
	cfg.AuthUser, cfg.AuthPass = "fixture-admin", "synthetic-unused-password-hash"
	if err := os.MkdirAll(filepath.Join(cfg.DataPath, "orgs", org), 0700); err != nil {
		t.Fatal(err)
	}
	router := newTestRouter(t, cfg, monitor, nil, nil, nil, "test")
	t.Cleanup(router.ShutdownBackgroundWorkers)
	t.Cleanup(router.ShutdownResourceStores)
	t.Cleanup(router.ShutdownRBAC)
	mtm := monitoring.NewMultiTenantMonitor(cfg, nil, nil)
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{org: monitor})
	router.mtMonitor = mtm
	router.resourceHandlers.SetTenantStateProvider(NewMultiTenantStateProvider(mtm, monitor))
	router.setMonitorSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, poller)

	const path = "/api/metrics-store/history?resourceType=agent&resourceId=" + id + "&range=1h&maxPoints=240"
	reqHeaders := func(req *http.Request) {
		req.Header.Set("X-API-Token", token)
		req.Header.Set("X-Pulse-Org-ID", org)
	}
	handler := router.Handler()
	ctx, disconnect := context.WithCancel(context.Background())
	defer disconnect()
	request := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	reqHeaders(request)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}()
	defer func() { releaseHistory(); <-done }()
	select {
	case <-held:
	case <-time.After(3 * time.Second):
		t.Fatal("shared SCALE reporting request never dispatched")
	}
	disconnect()
	select {
	case <-done:
		t.Fatal("History cancelled the outstanding shared SCALE RPC")
	case <-time.After(200 * time.Millisecond):
	}
	assertSession()
	releaseHistory()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("shared SCALE History did not finish after the held reply was released")
	}
	assertSession()

	poll()
	assertSession()
	if polls.Load() != 2 {
		t.Fatalf("expected initial and next ordinary SCALE poll, got %d", polls.Load())
	}
	rec := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, path, nil)
	reqHeaders(request)
	handler.ServeHTTP(rec, request)
	var result struct {
		Metrics map[string][]hostIOHistoryPoint
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &result) != nil ||
		len(result.Metrics["diskread"]) != 4 || len(result.Metrics["diskwrite"]) != 4 || reads.Load() != 2 {
		t.Fatalf("fresh SCALE History lost native data: %d, reads %d, body %s", rec.Code, reads.Load(), rec.Body)
	}
	for i := range result.Metrics["diskread"] {
		r, w := result.Metrics["diskread"][i], result.Metrics["diskwrite"][i]
		at := ((start+int64(i)*1000)/15*15 + 7) * 1000
		if r != (hostIOHistoryPoint{at, 1100, 1100, 1100}) || w != (hostIOHistoryPoint{at, 2200, 2200, 2200}) {
			t.Fatalf("fresh aligned SCALE point %d changed: read %+v write %+v", i, r, w)
		}
	}
	assertSession()
	t.Log("SCALE held across caller cancellation; no transport error, one session/one auth, next ordinary poll and fresh 4+4 points")
}
