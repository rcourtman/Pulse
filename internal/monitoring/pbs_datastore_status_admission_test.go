package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/stretchr/testify/require"
)

// Pinned TLS -> ordinary PBS poll -> real datastore/storage/alert consumers ->
// canonical presentation. Only synthetic GETs; no backup, guest or live service.
func TestPBSDatastoreStatusAdmissionLifecycle(t *testing.T) {
	var phase atomic.Int32
	var mu sync.Mutex
	reads := map[int32][]string{}
	bodies := []string{
		`{"data":{"total":1073741824,"used":1020054732,"avail":53687092}}`,
		`{"data":null,"extension":"private-status-provider-body"}`,
		`{"data":{"total":1073741824,"used":1020054732,"used":0,"avail":1073741824}}`,
		`{"data":{"total":1073741824,"avail":1073741824}}`,
		`{"data":{"total":1073741824,"used":0,"avail":1073741824},"errors":{"private":"private-status-provider-body"}}`,
		`{"data":{"total-space":1073741824,"used-space":268435456,"avail-space":805306368}}`,
		`{"data":{"total":1073741824,"used":0,"avail":1073741824}}`,
	}
	labels := []string{"high", "null", "duplicate", "missing", "errors", "recovered", "empty"}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "PBSAPIToken=fixture@pbs!reader:fixture-only", r.Header.Get("Authorization"))
		p := phase.Load()
		mu.Lock()
		reads[p] = append(reads[p], r.URL.RequestURI())
		mu.Unlock()
		body := `{"data":[]}`
		switch {
		case strings.HasSuffix(r.URL.Path, "/version"):
			body = `{"data":{"version":"3.4.2"}}`
		case strings.HasSuffix(r.URL.Path, "/nodes/localhost/status"):
			body = `{"data":{"cpu":0.1,"memory":{"used":512,"total":1024},"uptime":120}}`
		case strings.HasSuffix(r.URL.Path, "/admin/datastore"):
			body = `{"data":[{"store":"archive"},{"store":"sibling"},{"store":"excluded"}]}`
		case strings.HasSuffix(r.URL.Path, "/rrd"):
			body = `{"data":[{"time":1,"dedup_factor":2}]}`
		case strings.HasSuffix(r.URL.Path, "/archive/status"):
			body = bodies[p]
		case strings.HasSuffix(r.URL.Path, "/sibling/status"):
			body = `{"data":{"total":1073741824,"used":134217728,"avail":939524096}}`
		}
		fmt.Fprint(w, body)
	}))
	defer server.Close()
	pin := sha256.Sum256(server.Certificate().Raw)
	cfg := config.PBSInstance{Name: "pbs-main", Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only", Fingerprint: hex.EncodeToString(pin[:]), VerifySSL: true, MonitorDatastores: true, ExcludeDatastores: []string{"excluded"}}
	m := newPBSHealthAuthorityMonitor([]config.PBSInstance{cfg})
	client, err := pbs.NewClient(pbs.ClientConfig{Host: cfg.Host, TokenName: cfg.TokenName, TokenValue: cfg.TokenValue, Fingerprint: cfg.Fingerprint, VerifySSL: true, Timeout: time.Second})
	require.NoError(t, err)
	m.alertManager = alerts.NewManagerWithDataDir(t.TempDir())
	defer m.alertManager.Stop()
	alertCfg := m.alertManager.GetConfig()
	alertCfg.Enabled, alertCfg.ActivationState = true, alerts.ActivationActive
	alertCfg.Schedule.QuietHours.Enabled = false
	alertCfg.StorageDefault = alerts.HysteresisThreshold{Trigger: 80, Clear: 70}
	alertCfg.TimeThresholds["storage"] = 0
	m.alertManager.UpdateConfig(alertCfg)
	registry := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	var incidentID string
	for i, label := range labels {
		t.Run(label, func(t *testing.T) {
			phase.Store(int32(i))
			m.pollPBSInstance(context.Background(), cfg.Name, client)
			snapshot := m.state.GetSnapshot()
			require.Len(t, snapshot.PBSInstances, 1)
			instance := snapshot.PBSInstances[0]
			require.Equal(t, "healthy", instance.ConnectionHealth, "capacity failure is not server/auth failure")
			require.Len(t, instance.Datastores, 2)
			target := instance.Datastores[0]
			require.Equal(t, "archive", target.Name)
			if i > 0 && i < 5 {
				require.Equal(t, "unavailable", target.Status, "malformed success cannot clear datastore unavailability")
				require.NotEmpty(t, target.Error)
				require.Zero(t, target.Total)
				require.Zero(t, target.Used)
			} else {
				require.Equal(t, "available", target.Status)
				require.Empty(t, target.Error)
				require.Equal(t, int64(1073741824), target.Total)
			}
			require.Equal(t, 12.5, instance.Datastores[1].Usage, "readable sibling continues")
			foundStorage := false
			for _, storage := range snapshot.Storage {
				if storage.Name == "archive" {
					foundStorage = true
					require.Equal(t, target.Status, storage.Status)
					require.Equal(t, target.Total, storage.Total)
					require.Equal(t, target.Usage, storage.Usage)
				}
			}
			require.True(t, foundStorage)
			usageFound := false
			for _, alert := range m.alertManager.GetActiveAlerts() {
				if alert.ResourceName == "archive" && alert.Type == "usage" {
					usageFound = true
					if i == 0 {
						incidentID = alert.ID
					} else {
						require.Equal(t, incidentID, alert.ID, "missing observation cannot replace the firing capacity incident")
					}
				}
			}
			require.Equal(t, i < 5, usageFound, "only real low/empty readings may recover capacity")
			// Ordinary monitoring replaces the registry generation. Reusing a raw
			// Registry would incorrectly carry old incidents/metrics into this
			// fixture's next full snapshot, unlike MonitorAdapter's real path.
			registry.PopulateFromSnapshot(snapshot)
			resources := registry.GetAll()
			for _, resource := range resources {
				if resource.Type == unifiedresources.ResourceTypeStorage && resource.Name == "archive" {
					if i > 0 && i < 5 {
						require.Equal(t, unifiedresources.StatusOffline, resource.Status)
					} else if i >= 5 {
						require.Equal(t, unifiedresources.StatusOnline, resource.Status)
						require.Empty(t, resource.Incidents, "ordinary full recovery must not inherit old datastore incidents")
					}
				}
			}
			wire, err := json.Marshal(map[string]any{"phase": label, "resources": resources})
			require.NoError(t, err)
			require.NotContains(t, string(wire), "private-status-provider-body")
			if !t.Failed() {
				t.Logf("PBS_STATUS_CONSUMER %s", base64.StdEncoding.EncodeToString(wire))
			}
			mu.Lock()
			defer mu.Unlock()
			require.Len(t, reads[int32(i)], 9, "same bounded reads; no retry or GC after rejected status")
			for _, path := range reads[int32(i)] {
				require.NotContains(t, path, "excluded")
			}
		})
	}
}
