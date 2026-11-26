package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestPVEConnectionsLedgerSurfacesStartupFailuresAndAlerts(t *testing.T) {
	authFailSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"authentication failure"}`, http.StatusUnauthorized)
	}))
	defer authFailSrv.Close()

	outageSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"service unavailable during boot"}`, http.StatusServiceUnavailable)
	}))
	outageURL := outageSrv.URL
	outagePin := func() string {
		sum := sha256.Sum256(outageSrv.Certificate().Raw)
		return hex.EncodeToString(sum[:])
	}()
	// Close the listener before startup so PVE client construction hits a real
	// transport outage (connection refused) and must classify as Unreachable,
	// not Unauthorized.
	outageSrv.Close()

	tokenSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"unpolled"}`, http.StatusServiceUnavailable)
	}))
	defer tokenSrv.Close()

	pin := func(s *httptest.Server) string {
		sum := sha256.Sum256(s.Certificate().Raw)
		return hex.EncodeToString(sum[:])
	}

	cfg := &config.Config{
		DataPath:          t.TempDir(),
		ConnectionTimeout: 2 * time.Second,
		PVEInstances: []config.PVEInstance{
			{
				Name:        "pve-auth-fail",
				Host:        authFailSrv.URL,
				User:        "root@pam",
				Password:    "wrong-password",
				VerifySSL:   true,
				Fingerprint: pin(authFailSrv),
			},
			{
				Name:        "pve-startup-outage",
				Host:        outageURL,
				User:        "root@pam",
				Password:    "fixture-password",
				VerifySSL:   true,
				Fingerprint: outagePin,
			},
			{
				Name:        "pve-wrong-pin",
				Host:        authFailSrv.URL,
				User:        "root@pam",
				Password:    "fixture-password",
				VerifySSL:   true,
				Fingerprint: "0000000000000000000000000000000000000000000000000000000000000000",
			},
			{
				Name:        "pve-token-unpolled",
				Host:        tokenSrv.URL,
				TokenName:   "root@pam!pulse",
				TokenValue:  "fixture-token",
				VerifySSL:   true,
				Fingerprint: pin(tokenSrv),
			},
			{
				Name:     "pve-paused",
				Host:     "://invalid",
				Disabled: true,
			},
		},
	}

	mon, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(mon.Stop)

	statuses := mon.GetConnectionStatuses()
	for _, name := range []string{"pve-auth-fail", "pve-startup-outage", "pve-wrong-pin", "pve-token-unpolled"} {
		if statuses["pve-"+name] {
			t.Fatalf("GetConnectionStatuses()[%q] = true before any poll succeeded", "pve-"+name)
		}
	}

	handler := NewConnectionsHandlers(
		func(context.Context) *config.Config { return cfg },
		func(context.Context) *config.ConfigPersistence { return nil },
		func(context.Context) *monitoring.Monitor { return mon },
	)

	rec := httptest.NewRecorder()
	handler.HandleList(rec, httptest.NewRequest(http.MethodGet, "/api/connections", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/connections = %d: %s", rec.Code, rec.Body.String())
	}

	var listResp ConnectionsListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode connections response: %v", err)
	}

	byName := make(map[string]Connection, len(listResp.Connections))
	for _, conn := range listResp.Connections {
		byName[conn.Name] = conn
	}

	authConn := byName["pve-auth-fail"]
	if authConn.State != ConnectionStateUnauthorized || authConn.LastError == nil || authConn.LastError.Message == "" {
		t.Fatalf("pve-auth-fail = %+v, want unauthorized with lastError", authConn)
	}

	outageConn := byName["pve-startup-outage"]
	if outageConn.State != ConnectionStateUnreachable || outageConn.LastError == nil || outageConn.LastError.Message == "" {
		t.Fatalf("pve-startup-outage = %+v, want unreachable with lastError", outageConn)
	}

	wrongPinConn := byName["pve-wrong-pin"]
	if wrongPinConn.State != ConnectionStateUnreachable || wrongPinConn.LastError == nil || wrongPinConn.LastError.Message == "" {
		t.Fatalf("pve-wrong-pin = %+v, want unreachable with lastError", wrongPinConn)
	}

	tokenConn := byName["pve-token-unpolled"]
	if tokenConn.State != ConnectionStatePending || tokenConn.LastError != nil {
		t.Fatalf("pve-token-unpolled = %+v, want pending with nil lastError before first poll", tokenConn)
	}

	pausedConn := byName["pve-paused"]
	if pausedConn.State != ConnectionStatePaused || pausedConn.LastError != nil {
		t.Fatalf("pve-paused = %+v, want paused with nil lastError", pausedConn)
	}

	// Verify connection-degraded alerts activate for the startup-failing PVE connections
	// and do not fire for pending or paused connections.
	alertMgr := mon.GetAlertManager()
	snaps := snapshotConnectionsForAlerts(listResp.Connections)
	for i := 0; i < 3; i++ {
		for _, snap := range snaps {
			alertMgr.CheckConnection(snap)
		}
	}
	active := alertMgr.GetActiveAlerts()
	activeByResource := make(map[string]alerts.Alert, len(active))
	for _, alert := range active {
		activeByResource[alert.ResourceID] = alert
	}
	for _, id := range []string{"pve:pve-auth-fail", "pve:pve-startup-outage", "pve:pve-wrong-pin"} {
		alert, ok := activeByResource[id]
		if !ok {
			t.Fatalf("expected active connection-degraded alert for %s, active=%+v", id, active)
		}
		if alert.Level != alerts.AlertLevelCritical {
			t.Fatalf("alert for %s level = %q, want critical", id, alert.Level)
		}
	}
	for _, id := range []string{"pve:pve-token-unpolled", "pve:pve-paused"} {
		if alert, ok := activeByResource[id]; ok {
			t.Fatalf("unexpected alert for %s: %+v", id, alert)
		}
	}
}
