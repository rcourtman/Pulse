package configapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Non-parallel. Drive the real ingress handlers, without credentials or any
// monitor/persistence, so their pre-validation diagnostics cannot perform I/O.
func TestNodeRequestEndpointDiagnosticsExcludeRejectedInput(t *testing.T) {
	for _, nodeType := range []string{"pve", "pbs", "pmg"} {
		for index, host := range []string{
			"http://synthetic-user:synthetic-secret@gateway.invalid",
			"https://synthetic-user:synthetic-secret@gateway.invalid",
			"synthetic-user:synthetic-secret@gateway.invalid",
			"http://gateway.invalid?token=synthetic-secret",
			"https://gateway.invalid#synthetic-secret",
			"gateway.invalid?token=synthetic-secret",
			"http://gateway.invalid:bad-port?token=synthetic-secret",
			"https://gateway.invalid/%invalid?token=synthetic-secret",
		} {
			for _, operation := range []string{"add", "test"} {
				t.Run(fmt.Sprintf("%s/%d/%s", nodeType, index, operation), func(t *testing.T) {
					var captured bytes.Buffer
					prior := log.Logger
					log.Logger = zerolog.New(&captured).Level(zerolog.DebugLevel)
					defer func() { log.Logger = prior }()
					body, err := json.Marshal(NodeConfigRequest{Type: nodeType, Name: "fixture", Host: host})
					if err != nil {
						t.Fatal(err)
					}
					h := &ConfigHandlers{}
					req := httptest.NewRequest(http.MethodPost, "/api/config/nodes/"+operation, bytes.NewReader(body))
					w := httptest.NewRecorder()
					if operation == "add" {
						h.HandleAddNode(w, req)
					} else {
						h.HandleTestConnection(w, req)
					}
					if w.Code != http.StatusBadRequest {
						t.Fatal("credential-less request did not preserve its HTTP refusal")
					}
					for _, marker := range []string{"synthetic-secret", "synthetic-user"} {
						if strings.Contains(captured.String(), marker) || strings.Contains(w.Body.String(), marker) {
							t.Error("request diagnostic disclosed rejected endpoint input")
						}
					}
					var event map[string]interface{}
					if err := json.NewDecoder(&captured).Decode(&event); err != nil {
						t.Fatal(err)
					}
					if event["host"] != "[invalid endpoint]" || event["type"] != nodeType || event["name"] != "fixture" {
						t.Error("request diagnostic lost safe endpoint, type or attribution")
					}
					t.Logf("status=%d response=%q", w.Code, w.Body.String())
				})
			}
		}
	}
}

func TestNodeRequestAcceptedEndpointDiagnosticPreservesResponse(t *testing.T) {
	for _, nodeType := range []string{"pve", "pbs", "pmg"} {
		for _, operation := range []string{"add", "test"} {
			t.Run(nodeType+"/"+operation, func(t *testing.T) {
				var captured bytes.Buffer
				prior := log.Logger
				log.Logger = zerolog.New(&captured).Level(zerolog.DebugLevel)
				defer func() { log.Logger = prior }()
				body, err := json.Marshal(NodeConfigRequest{Type: nodeType, Name: "fixture", Host: "gateway.invalid:8443/tenant"})
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(http.MethodPost, "/api/config/nodes/"+operation, bytes.NewReader(body))
				w := httptest.NewRecorder()
				h := &ConfigHandlers{}
				if operation == "add" {
					h.HandleAddNode(w, req)
				} else {
					h.HandleTestConnection(w, req)
				}
				if w.Code != http.StatusBadRequest || w.Body.String() != "Authentication credentials required\n" {
					t.Fatal("diagnostic filtering changed the HTTP response")
				}
				var event map[string]interface{}
				if err := json.NewDecoder(&captured).Decode(&event); err != nil {
					t.Fatal(err)
				}
				if event["host"] != "https://gateway.invalid:8443/tenant" {
					t.Fatal("accepted endpoint lost its useful diagnostic")
				}
			})
		}
	}
}

// A saved malformed PMG URL still returns the same generic response to the
// Connections consumer, but its detailed internal error must not copy the URL.
func TestSavedPMGRejectedEndpointDiagnosticsPreserveHTTPResponse(t *testing.T) {
	for index, host := range []string{
		"http://synthetic-user:synthetic-secret@pmg.invalid:bad-port",
		"http://pmg.invalid:bad-port?token=synthetic-secret",
		"https://pmg.invalid/%invalid?token=synthetic-secret",
	} {
		t.Run(fmt.Sprintf("case-%d", index), func(t *testing.T) {
			var captured bytes.Buffer
			prior := log.Logger
			log.Logger = zerolog.New(&captured).Level(zerolog.DebugLevel)
			defer func() { log.Logger = prior }()
			h := &ConfigHandlers{defaultConfig: &config.Config{PMGInstances: []config.PMGInstance{{Name: "fixture", Host: host, TokenName: "fixture@pmg!pulse", TokenValue: "synthetic-token"}}}}
			w := httptest.NewRecorder()
			h.HandleTestNode(w, httptest.NewRequest(http.MethodPost, "/api/config/nodes/pmg-0/test", nil))
			if w.Code != http.StatusBadRequest || w.Body.String() != "{\"message\":\"Failed to initialize connection\",\"status\":\"error\"}\n" {
				t.Fatal("PMG diagnostic filtering changed the saved-node HTTP response")
			}
			for _, marker := range []string{"synthetic-secret", "synthetic-user", "synthetic-token"} {
				if strings.Contains(captured.String(), marker) || strings.Contains(w.Body.String(), marker) {
					t.Error("saved-node diagnostic disclosed endpoint or authentication input")
				}
			}
			t.Logf("status=%d response=%q", w.Code, w.Body.String())
		})
	}
}
