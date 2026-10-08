package configapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestPMGCollectionScopeSavedPatch(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir(), PMGInstances: []config.PMGInstance{{Name: "gateway", Host: "https://example.invalid:8006"}}}
	h := newTestConfigHandlers(t, cfg)
	for _, tc := range []struct {
		name                         string
		patch                        string
		wantMail, configured, queues bool
	}{
		{"unrelated-edit", `{"guestURL":"https://example.invalid"}`, true, false, false},
		{"partial-false-preserves-legacy-default", `{"monitorQueues":false}`, true, true, false},
		{"explicit-all-off", `{"monitorMailStats":false,"monitorQueues":false,"monitorQuarantine":false,"monitorDomainStats":false}`, false, true, false},
		{"omission-preserves-opt-out", `{"guestURL":"https://example.invalid/new"}`, false, true, false},
		{"select-queues", `{"monitorQueues":true}`, false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPut, "/api/config/nodes/pmg-0", bytes.NewBufferString(tc.patch))
			h.HandleUpdateNode(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			got := cfg.PMGInstances[0]
			if got.MailStatsEnabled() != tc.wantMail || got.MonitoringConfigured != tc.configured || got.MonitorQueues != tc.queues {
				t.Fatalf("runtime scope=%+v", got)
			}
			nodes := h.GetAllNodesForAPI(context.Background())
			if len(nodes) != 1 || nodes[0].MonitorMailStats != tc.wantMail || nodes[0].MonitorQueues != tc.queues {
				t.Fatalf("readback scope=%+v", nodes)
			}
			saved, err := config.NewConfigPersistence(cfg.DataPath).LoadNodesConfig()
			if err != nil {
				t.Fatal(err)
			}
			if len(saved.PMGInstances) != 1 || saved.PMGInstances[0] != got {
				t.Fatalf("durable scope=%+v want=%+v", saved.PMGInstances, got)
			}
		})
	}
}
func TestPMGCollectionScopeFailedSave(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir(), PMGInstances: []config.PMGInstance{{Name: "gateway", Host: "https://example.invalid:8006"}}}
	h := newTestConfigHandlers(t, cfg)
	if err := os.Mkdir(filepath.Join(cfg.DataPath, "nodes.enc"), 0700); err != nil {
		t.Fatal(err)
	}
	before := cfg.PMGInstances[0]
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/api/config/nodes/pmg-0", bytes.NewBufferString(`{"monitorMailStats":false,"enabled":false}`))
	h.HandleUpdateNode(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected failed save, got %d", w.Code)
	}
	if cfg.PMGInstances[0] != before {
		t.Fatal("failed save changed live collection scope/pause")
	}
}
func TestPMGCollectionScopeNewConnection(t *testing.T) {
	for _, tc := range []struct {
		name     string
		options  map[string]any
		wantMail bool
	}{
		{"default", nil, true},
		{"explicit-all-off", map[string]any{"monitorMailStats": false, "monitorQueues": false, "monitorQuarantine": false, "monitorDomainStats": false}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{DataPath: t.TempDir()}
			h := newTestConfigHandlers(t, cfg)
			body := map[string]any{"type": "pmg", "name": "gateway", "host": "https://example.invalid:8006", "user": "fixture@pmg", "password": "synthetic-password"}
			for k, v := range tc.options {
				body[k] = v
			}
			b, _ := json.Marshal(body)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/api/config/nodes", bytes.NewReader(b))
			h.HandleAddNode(w, r)
			if w.Code != http.StatusCreated {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if len(cfg.PMGInstances) != 1 || !cfg.PMGInstances[0].MonitoringConfigured || cfg.PMGInstances[0].MailStatsEnabled() != tc.wantMail {
				t.Fatalf("new scope=%+v", cfg.PMGInstances)
			}
			saved, err := config.NewConfigPersistence(cfg.DataPath).LoadNodesConfig()
			if err != nil {
				t.Fatal(err)
			}
			if len(saved.PMGInstances) != 1 || saved.PMGInstances[0].MailStatsEnabled() != tc.wantMail || !saved.PMGInstances[0].MonitoringConfigured {
				t.Fatal("new scope not durable")
			}
		})
	}
}
