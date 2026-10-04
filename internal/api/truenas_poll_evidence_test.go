package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

type trueNASPollEvidenceFixture struct {
	handler     *TrueNASHandlers
	persistence *config.ConfigPersistence
	poller      *monitoring.TrueNASPoller
	connection  config.TrueNASInstance
	failPools   atomic.Bool
	failLogin   atomic.Bool
}

// A deterministic REST appliance analogue: system.info can succeed while a
// required inventory method fails. This proves the source distinction, not
// the cause or transport of either #2382 operator's native failure.
func newTrueNASPollEvidenceFixture(t *testing.T) *trueNASPollEvidenceFixture {
	t.Helper()
	setTrueNASFeatureForTest(t, true)
	f := &trueNASPollEvidenceFixture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2.0/system/info":
			if f.failLogin.Load() {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"probe login denied"}`))
				return
			}
			_, _ = w.Write([]byte(`{"hostname":"poll-fixture","version":"TrueNAS-SCALE-24.10.2","uptime_seconds":100}`))
		case "/api/v2.0/pool":
			if f.failPools.Load() {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"inventory unavailable"}`))
				return
			}
			_, _ = w.Write([]byte(`[{"id":1,"name":"tank","status":"ONLINE","size":1000,"allocated":400,"free":600}]`))
		case "/api/v2.0/pool/dataset", "/api/v2.0/disk", "/api/v2.0/alert/list":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	cfg := &config.Config{DataPath: t.TempDir()}
	f.handler, f.persistence, _ = newTrueNASHandlersForTest(t, cfg)
	f.connection = trueNASInstanceFromRawURL(t, "poll-fixture", server.URL, true)
	if err := f.persistence.SaveTrueNASConfig([]config.TrueNASInstance{f.connection}); err != nil {
		t.Fatal(err)
	}
	f.poller = monitoring.NewTrueNASPoller(config.NewMultiTenantPersistence(cfg.DataPath), 50*time.Millisecond, nil)
	f.handler.getPoller = func(context.Context) *monitoring.TrueNASPoller { return f.poller }
	t.Cleanup(f.poller.Stop)
	return f
}

func (f *trueNASPollEvidenceFixture) summary() monitoring.TrueNASConnectionSummary {
	return f.poller.ConnectionSummaries("default", []config.TrueNASInstance{f.connection})[f.connection.ID]
}

func waitForTrueNASPollEvidence(t *testing.T, f *trueNASPollEvidenceFixture, ready func(monitoring.TrueNASConnectionSummary) bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ready(f.summary()) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("missing runtime poll evidence: %+v", f.summary())
}
