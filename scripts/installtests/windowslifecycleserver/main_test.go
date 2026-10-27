package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestUninstallProofRequiresTheSavedCredentialAndExactIdentity(t *testing.T) {
	for _, test := range []struct {
		name, method, token, body     string
		status, requests, auth, exact int
	}{
		{"saved", "POST", lifecycleToken, `{"agentId":"windows-lifecycle-agent"}`, 200, 1, 1, 1},
		{"missing token", "POST", "", `{"agentId":"windows-lifecycle-agent"}`, 401, 1, 0, 1},
		{"different token", "POST", "wrong", `{"agentId":"windows-lifecycle-agent"}`, 401, 1, 0, 1},
		{"different identity", "POST", lifecycleToken, `{"agentId":"same-name-other-host"}`, 400, 1, 1, 0},
		{"different identity case", "POST", lifecycleToken, `{"agentId":"Windows-lifecycle-agent"}`, 400, 1, 1, 0},
		{"missing identity", "POST", lifecycleToken, `{}`, 400, 1, 1, 0},
		{"malformed", "POST", lifecycleToken, `{`, 400, 1, 1, 0},
		{"overbound", "POST", lifecycleToken, `{"agentId":"` + strings.Repeat("x", 17*1024) + `"}`, 400, 1, 1, 0},
		{"not POST", "GET", lifecycleToken, `{"agentId":"windows-lifecycle-agent"}`, 405, 0, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			peer, err := newUninstallProof("confirm")
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(test.method, "/api/agents/agent/uninstall", strings.NewReader(test.body))
			req.Header.Set("X-API-Token", test.token)
			recorder := httptest.NewRecorder()
			peer.confirm(recorder, req)
			if recorder.Code != test.status {
				t.Fatalf("status %d: %s", recorder.Code, recorder.Body)
			}
			if test.status == http.StatusOK && recorder.Body.String() != "{\"agentId\":\"windows-lifecycle-agent\",\"success\":true}\n" {
				t.Fatalf("wrong exact-ID acknowledgement: %s", recorder.Body)
			}
			observed := httptest.NewRecorder()
			peer.observe(observed, httptest.NewRequest(http.MethodGet, "/__pulse_lifecycle__/uninstall", nil))
			var receipt uninstallReceipt
			if err := json.Unmarshal(observed.Body.Bytes(), &receipt); err != nil {
				t.Fatal(err)
			}
			want := uninstallReceipt{"confirm", test.requests, test.auth, test.exact}
			if receipt != want {
				t.Fatalf("observed %+v, want %+v", receipt, want)
			}
			for _, private := range []string{lifecycleToken, lifecycleAgentID, privateResponseMarker} {
				if strings.Contains(observed.Body.String(), private) {
					t.Fatal("observation exposed request/response contents")
				}
			}
		})
	}
}

func TestUninstallProofServesEveryAdverseResultOverRealHTTP(t *testing.T) {
	for _, mode := range []string{"confirm", "http-503", "wrong-id", "string-success", "missing-id"} {
		t.Run(mode, func(t *testing.T) {
			peer, err := newUninstallProof(mode)
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			mux.HandleFunc("/api/agents/agent/uninstall", peer.confirm)
			mux.HandleFunc("/__pulse_lifecycle__/uninstall", peer.observe)
			server := httptest.NewServer(mux)
			defer server.Close()
			req, err := http.NewRequest(http.MethodPost, server.URL+"/api/agents/agent/uninstall", strings.NewReader(`{"agentId":"windows-lifecycle-agent"}`))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("X-API-Token", lifecycleToken)
			response, err := server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if mode == "http-503" {
				if response.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(body), privateResponseMarker) {
					t.Fatalf("missing deliberate private HTTP failure: %d %s", response.StatusCode, body)
				}
			} else {
				var result map[string]any
				if err := json.Unmarshal(body, &result); err != nil {
					t.Fatal(err)
				}
				if response.StatusCode != http.StatusOK {
					t.Fatal(response.StatusCode)
				}
				confirmed := result["success"] == true && result["agentId"] == lifecycleAgentID
				if confirmed != (mode == "confirm") {
					t.Fatalf("wrong adverse response for %s: %s", mode, body)
				}
				if mode == "wrong-id" && result["agentId"] != "same-name-other-host" {
					t.Fatal("wrong-ID control missing")
				}
				if mode == "string-success" && result["success"] != "true" {
					t.Fatal("string-success control missing")
				}
				if mode == "missing-id" && result["agentId"] != nil {
					t.Fatal("missing-ID control missing")
				}
			}
			observed, err := server.Client().Get(server.URL + "/__pulse_lifecycle__/uninstall")
			if err != nil {
				t.Fatal(err)
			}
			defer observed.Body.Close()
			var receipt uninstallReceipt
			if err := json.NewDecoder(observed.Body).Decode(&receipt); err != nil {
				t.Fatal(err)
			}
			if receipt != (uninstallReceipt{mode, 1, 1, 1}) {
				t.Fatalf("wrong request witness: %+v", receipt)
			}
		})
	}
}

func TestUninstallProofObservationIsReadOnlyAndRaceSafe(t *testing.T) {
	peer, err := newUninstallProof("confirm")
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for i := 0; i < 25; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/uninstall", strings.NewReader(`{"agentId":"windows-lifecycle-agent"}`))
			req.Header.Set("X-API-Token", lifecycleToken)
			peer.confirm(httptest.NewRecorder(), req)
			peer.observe(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/__pulse_lifecycle__/uninstall", nil))
		}()
	}
	workers.Wait()
	recorder := httptest.NewRecorder()
	peer.observe(recorder, httptest.NewRequest(http.MethodPost, "/__pulse_lifecycle__/uninstall", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatal("observation accepted mutation")
	}
	if peer.receipt != (uninstallReceipt{"confirm", 25, 25, 25}) {
		t.Fatalf("lost request observations: %+v", peer.receipt)
	}
}

func TestUninstallProofAdmissionIsLoopbackAndClosedModes(t *testing.T) {
	for _, host := range []string{"127.0.0.1:17655", "[::1]:17655"} {
		if err := validateListen(host); err != nil {
			t.Fatal(err)
		}
	}
	for _, host := range []string{":17655", "0.0.0.0:17655", "[::]:17655", "192.0.2.1:17655", "localhost:17655", "malformed"} {
		t.Run(fmt.Sprint(host), func(t *testing.T) {
			if validateListen(host) == nil {
				t.Fatal("admitted non-literal/non-loopback fixture listener")
			}
		})
	}
	if _, err := newUninstallProof("arbitrary"); err == nil {
		t.Fatal("admitted an unknown result mode")
	}
}
