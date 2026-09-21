package configapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Reproduction probe for issue #2140: a PVE node's "Verify SSL certificate"
// toggle must survive a PUT that disables it, and the GET projection must
// report the disabled value back to the form.
func TestRepro2140VerifySSLPersistsOnUpdate(t *testing.T) {
	cfg := &config.Config{
		DataPath: t.TempDir(),
		PVEInstances: []config.PVEInstance{{
			Name:        "node-a",
			Host:        "https://pve.local:8006",
			TokenName:   "pulse-monitor@pve!pulse",
			TokenValue:  "secret",
			Fingerprint: "AA:BB:CC",
			VerifySSL:   true,
		}},
	}
	handler := newTestConfigHandlers(t, cfg)

	body, _ := json.Marshal(map[string]any{"verifySSL": false})
	req := httptest.NewRequest(http.MethodPut, "/api/config/nodes/pve-0", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateNode(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", rec.Code, rec.Body.String())
	}
	if cfg.PVEInstances[0].VerifySSL {
		t.Fatalf("stored VerifySSL = true, want false after disabling")
	}

	nodes := handler.GetAllNodesForAPI(req.Context())
	if len(nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(nodes))
	}
	if nodes[0].VerifySSL {
		t.Fatalf("GET projection VerifySSL = true, want false")
	}
}
