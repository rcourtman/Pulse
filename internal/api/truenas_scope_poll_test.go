package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/stretchr/testify/require"
)

func TestTrueNASCollectionScopePollsWhatTheEditorSaves(t *testing.T) {
	for _, frame := range observeTrueNASScopeJourney(t) {
		t.Run(frame.Phase, func(t *testing.T) {
			datasets := frame.Phase == "initial" || strings.HasPrefix(frame.Phase, "datasets")
			all := frame.Phase == "initial"
			want := [3]bool{datasets, all, all}
			assertScope := func(body []byte) {
				t.Helper()
				require.NotContains(t, string(body), "collectionScopeConfigured")
				var got config.TrueNASInstance
				require.NoError(t, json.Unmarshal(body, &got))
				require.Equal(t, want, [3]bool{got.MonitorDatasets, got.MonitorPools, got.MonitorReplication})
				require.True(t, got.Enabled)
				require.True(t, got.UseHTTPS)
				require.Len(t, got.Fingerprint, 64)
				require.True(t, config.IsTrueNASSensitiveMask(got.APIKey))
			}
			if frame.Saved != nil {
				assertScope(frame.Saved)
			}
			var rows []json.RawMessage
			require.NoError(t, json.Unmarshal(frame.Listed, &rows))
			require.Len(t, rows, 1)
			assertScope(rows[0])

			require.True(t, frame.RPC.Reads["system.info"])
			require.True(t, frame.RPC.Reads["disk.query"])
			require.True(t, frame.RPC.Reads["alert.list"])
			for method, wanted := range map[string]bool{
				"pool.query": all, "boot.get_state": all, "replication.query": all,
				"pool.dataset.query": datasets, "pool.snapshot.query": datasets,
				"zfs.resource.snapshot.query": datasets,
			} {
				require.Equal(t, wanted, frame.RPC.Reads[method], method)
			}
			if datasets {
				require.JSONEq(t, `[{"paths":[],"recursive":true,"properties":["creation","used","referenced"]}]`, frame.RPC.Native)
				require.JSONEq(t, `[[],{"extra":{"properties":["creation","used","referenced"]}}]`, frame.RPC.Fallback)
			}
			var listed trueNASConnectionResponse
			require.NoError(t, json.Unmarshal(rows[0], &listed))
			wantPools, wantDatasets, wantArtifacts := 0, 0, 0
			if all {
				wantPools, wantArtifacts = 2, 1 // regular and boot pool; one replication task
			}
			if datasets {
				wantDatasets = 1
				wantArtifacts++ // a nonempty result through the fallback, not merely a request
			}
			require.Equal(t, wantPools, listed.Observed.StoragePools)
			require.Equal(t, wantDatasets, listed.Observed.Datasets)
			require.Equal(t, wantArtifacts, listed.Observed.RecoveryArtifacts)
		})
	}
}

func TestTrueNASAddRetainsExplicitAllOff(t *testing.T) {
	setTrueNASFeatureForTest(t, true)
	setMockModeForTest(t, false)
	handler, _, _ := newTrueNASHandlersForTest(t, nil)
	rec := httptest.NewRecorder()
	handler.HandleAdd(rec, httptest.NewRequest(http.MethodPost, "/api/truenas/connections", strings.NewReader(
		`{"host":"nas.example.invalid","apiKey":"fixture-key","username":"fixture-user","monitorDatasets":false,"monitorPools":false,"monitorReplication":false}`,
	)))
	require.Equal(t, http.StatusCreated, rec.Code)
	rec = httptest.NewRecorder()
	handler.HandleList(rec, httptest.NewRequest(http.MethodGet, "/api/truenas/connections", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var rows []trueNASConnectionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	require.Len(t, rows, 1)
	require.True(t, rows[0].Enabled)
	require.Equal(t, [3]bool{}, [3]bool{rows[0].MonitorDatasets, rows[0].MonitorPools, rows[0].MonitorReplication})
}

type trueNASScopeFrame struct {
	Phase  string            `json:"phase"`
	Saved  json.RawMessage   `json:"saved,omitempty"`
	Listed json.RawMessage   `json:"listed"`
	RPC    trueNASScopeReads `json:"rpc"`
}

type trueNASScopeReads struct {
	Reads    map[string]bool `json:"reads"`
	Native   string          `json:"native,omitempty"`
	Fallback string          `json:"fallback,omitempty"`
}

// Exercise the real settings handlers, encrypted storage and periodic poller.
// Stop the fixture loop between observations so earlier in-flight RPCs cannot
// count in the next window. Warm observations retain the poller and its
// provider; cold observations open the saved file from scratch.
func observeTrueNASScopeJourney(t *testing.T) []trueNASScopeFrame {
	t.Helper()
	setTrueNASFeatureForTest(t, true)
	setMockModeForTest(t, false)
	server, takeReads := newTrueNASScopeAppliance(t)
	instance := trueNASInstanceFromRawURL(t, "scope-nas", server.URL, true)
	instance.Fingerprint = fmt.Sprintf("%x", sha256.Sum256(server.Certificate().Raw))
	// The loopback certificate is self-signed. Verify its exact pin.
	instance.InsecureSkipVerify = true
	dir := t.TempDir()
	newEndpoint := func() (*TrueNASHandlers, *monitoring.TrueNASPoller) {
		mt := config.NewMultiTenantPersistence(dir)
		storage, err := mt.GetPersistence("default")
		require.NoError(t, err)
		poller := monitoring.NewTrueNASPoller(mt, 100*time.Millisecond, nil)
		t.Cleanup(poller.Stop)
		return &TrueNASHandlers{
			getPersistence: func(_ context.Context) *config.ConfigPersistence { return storage },
			getPoller:      func(_ context.Context) *monitoring.TrueNASPoller { return poller },
		}, poller
	}
	handler, warm := newEndpoint()
	save := func(method string, body []byte) json.RawMessage {
		t.Helper()
		rec := httptest.NewRecorder()
		path := "/api/truenas/connections"
		if method == http.MethodPost {
			handler.HandleAdd(rec, httptest.NewRequest(method, path, bytes.NewReader(body)))
			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		} else {
			handler.HandleUpdate(rec, httptest.NewRequest(method, path+"/"+instance.ID, bytes.NewReader(body)))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		}
		require.NotContains(t, rec.Body.String(), instance.APIKey)
		return append(json.RawMessage(nil), rec.Body.Bytes()...)
	}
	observe := func(phase string, h *TrueNASHandlers, p *monitoring.TrueNASPoller, saved json.RawMessage) trueNASScopeFrame {
		t.Helper()
		since := time.Now()
		p.Start(t.Context())
		require.Eventually(t, func() bool {
			status := p.ConnectionSummaries("default", []config.TrueNASInstance{instance})[instance.ID].Poll
			return status.LastSuccessAt != nil && status.LastSuccessAt.After(since)
		}, 4*time.Second, 10*time.Millisecond, phase)
		p.Stop()
		rec := httptest.NewRecorder()
		h.HandleList(rec, httptest.NewRequest(http.MethodGet, "/api/truenas/connections", nil))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.NotContains(t, rec.Body.String(), instance.APIKey)
		var rows []trueNASConnectionResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
		require.Len(t, rows, 1)
		row := rows[0]
		require.Equal(t, instance.ID, row.ID)
		require.Equal(t, instance.Fingerprint, row.Fingerprint)
		require.NotNil(t, row.Poll)
		require.Nil(t, row.Poll.LastError)
		require.NotNil(t, row.Transport)
		require.Equal(t, truenas.TransportJSONRPC, row.Transport.Mode)
		require.True(t, row.Transport.TLS)
		require.NotNil(t, row.Observed)
		require.Equal(t, "scope-nas", row.Observed.Host)
		require.Equal(t, 1, row.Observed.Systems)
		require.Equal(t, 1, row.Observed.Disks)
		require.NotNil(t, row.Observed.Telemetry)
		require.True(t, row.Observed.Telemetry.CPU)
		systemAlert := false
		for _, record := range p.GetCurrentRecordsForOrg("default") {
			if record.Resource.Name != "scope-nas" {
				continue
			}
			for _, incident := range record.Resource.Incidents {
				systemAlert = systemAlert || incident.NativeID == "scope-alert"
			}
		}
		require.True(t, systemAlert, "the NAS still publishes its native warning")
		return trueNASScopeFrame{Phase: phase, Saved: saved, Listed: rec.Body.Bytes(), RPC: takeReads()}
	}

	initial := save(http.MethodPost, marshalTrueNASRequest(t, map[string]any{
		"id": instance.ID, "name": "Workshop NAS", "host": instance.Host, "port": instance.Port,
		"username": instance.Username, "apiKey": instance.APIKey, "fingerprint": instance.Fingerprint,
		"useHttps": true, "insecureSkipVerify": true,
	})) // omitted scope should keep the all-on default
	frames := []trueNASScopeFrame{observe("initial", handler, warm, initial)}
	off := save(http.MethodPut, []byte(`{"monitorDatasets":false,"monitorPools":false,"monitorReplication":false,"apiKey":"********"}`))
	frames = append(frames, observe("off-warm", handler, warm, off))
	renamed := save(http.MethodPut, []byte(`{"name":"Workshop NAS","apiKey":"********"}`))
	coldHandler, cold := newEndpoint()
	frames = append(frames, observe("off-cold", coldHandler, cold, renamed))

	datasets := save(http.MethodPut, []byte(`{"monitorDatasets":true,"monitorPools":false,"monitorReplication":false,"apiKey":"********"}`))
	frames = append(frames, observe("datasets-warm", handler, warm, datasets))
	datasetsHandler, datasetsCold := newEndpoint()
	return append(frames, observe("datasets-cold", datasetsHandler, datasetsCold, nil))
}

func newTrueNASScopeAppliance(t *testing.T) (*httptest.Server, func() trueNASScopeReads) {
	t.Helper()
	var mu sync.Mutex
	reads := trueNASScopeReads{Reads: make(map[string]bool)}
	conns := make(map[*websocket.Conn]struct{})
	upgrader := websocket.Upgrader{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/current" || r.TLS == nil {
			http.NotFound(w, r)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		mu.Lock()
		conns[conn] = struct{}{}
		mu.Unlock()
		defer func() {
			mu.Lock()
			delete(conns, conn)
			mu.Unlock()
			conn.Close()
		}()
		for {
			var req struct {
				ID     int64           `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if conn.ReadJSON(&req) != nil {
				return
			}
			mu.Lock()
			reads.Reads[req.Method] = true
			switch req.Method {
			case "zfs.resource.snapshot.query":
				reads.Native = string(req.Params)
			case "pool.snapshot.query":
				reads.Fallback = string(req.Params)
			}
			mu.Unlock()
			response := map[string]any{"jsonrpc": "2.0", "id": req.ID}
			result := `[]`
			switch req.Method {
			case "auth.login_ex":
				result = `{"response_type":"SUCCESS"}`
			case "system.info":
				result = `{"hostname":"scope-nas","system_serial":"SCOPE-NAS","version":"TrueNAS-SCALE-25.10.8"}`
			case "pool.query":
				result = `[{"id":1,"name":"tank","status":"ONLINE","size":1000,"allocated":400,"free":600}]`
			case "boot.get_state":
				result = `{"name":"boot-pool","status":"ONLINE","size":1000,"allocated":100,"free":900}`
			case "pool.dataset.query":
				result = `[{"id":"tank/apps","name":"tank/apps","type":"FILESYSTEM","pool":"tank"}]`
			case "disk.query":
				result = `[{"identifier":"{serial}SCOPE-DISK","name":"sda","serial":"SCOPE-DISK","size":1000000,"type":"HDD"}]`
			case "alert.list":
				result = `[{"id":"scope-alert","level":"WARNING","formatted":"Fixture warning","source":"fixture"}]`
			case "zfs.resource.snapshot.query": // unavailable on 25.10
				result = ""
				response["error"] = map[string]any{"code": -32601, "message": "Method does not exist"}
			case "pool.snapshot.query":
				result = `[{"id":"tank/apps@nightly","properties":{"creation":{"rawvalue":"1710000000"},"used":{"parsed":1024},"referenced":{"parsed":2048}}}]`
			case "replication.query":
				result = `[{"id":42,"name":"nightly replica","source_datasets":["tank/apps"],"target_dataset":"replica/apps","state":{"state":"FINISHED"}}]`
			case "core.subscribe":
				result = `"scope-sub"`
			case "core.unsubscribe":
				result = `null`
			}
			if result != "" {
				response["result"] = json.RawMessage(result)
			}
			if conn.WriteJSON(response) != nil {
				return
			}
			if req.Method == "core.subscribe" {
				var collections []string
				if json.Unmarshal(req.Params, &collections) != nil || len(collections) != 1 {
					return
				}
				if conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "method": "collection_update", "params": map[string]any{
					"collection": collections[0], "fields": map[string]any{"cpu": map[string]any{"usage": 10}},
				}}) != nil {
					return
				}
			}
		}
	}))
	t.Cleanup(func() {
		mu.Lock()
		for conn := range conns {
			conn.Close()
		}
		mu.Unlock()
		server.Close()
	})
	return server, func() trueNASScopeReads {
		mu.Lock()
		defer mu.Unlock()
		got := reads
		reads = trueNASScopeReads{Reads: make(map[string]bool)}
		return got
	}
}
