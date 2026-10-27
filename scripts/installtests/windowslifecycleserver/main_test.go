package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUninstallAcknowledgementMatchesExactRequestedIdentity(t *testing.T) {
	for _, id := range []string{"windows-lifecycle-agent", "same-name:other-host"} {
		recorder := httptest.NewRecorder()
		confirmUninstall(recorder, httptest.NewRequest(http.MethodPost, "/api/agents/agent/uninstall", strings.NewReader(`{"agentId":"`+id+`"}`)))
		var response struct {
			Success bool   `json:"success"`
			AgentID string `json:"agentId"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || recorder.Code != http.StatusOK || !response.Success || response.AgentID != id {
			t.Fatalf("wrong acknowledgement: %d %s (%v)", recorder.Code, recorder.Body, err)
		}
	}
	for _, test := range []struct{ method, body string }{{http.MethodGet, `{"agentId":"other"}`}, {http.MethodPost, `{}`}, {http.MethodPost, `{`}} {
		recorder := httptest.NewRecorder()
		confirmUninstall(recorder, httptest.NewRequest(test.method, "/api/agents/agent/uninstall", strings.NewReader(test.body)))
		if recorder.Code == http.StatusOK {
			t.Fatalf("invalid request received confirmation: %s %s", test.method, test.body)
		}
	}
}
