package pbs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClient_TokenAuth_SetsAuthorizationHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/version" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "PBSAPIToken=root@pam!pulse-token:secret" {
			t.Fatalf("Authorization = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": Version{Version: "3.0", Release: "1", Repoid: "abc"},
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!pulse-token",
		TokenValue: "secret",
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	v, err := client.GetVersion(context.Background())
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if v.Version != "3.0" {
		t.Fatalf("unexpected version: %+v", v)
	}
}

func TestClient_GetJobHealthEvidence_MergesConfigAndTaskFacts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/localhost/tasks":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{
					"upid":        "UPID:sync:1",
					"worker-type": "syncjob",
					"worker-id":   "sync-remote-a",
					"status":      "OK",
					"starttime":   1700000000,
					"endtime":     1700000060,
				},
				{
					"upid":        "UPID:backup:1",
					"worker-type": "backup",
					"worker-id":   "vm/100",
					"status":      "OK",
					"endtime":     1700000100,
				},
			}})
		case "/api2/json/config/sync":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
				"id":               "sync-remote-a",
				"store":            "fast",
				"remote":           "remote-a",
				"schedule":         "hourly",
				"last-run-state":   "OK",
				"last-run-upid":    "UPID:sync:1",
				"last-run-endtime": 1700000060,
				"next-run":         1700003600,
			}}})
		case "/api2/json/config/verify":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
				"id":             "verify-fast",
				"store":          "fast",
				"last-run-state": "OK",
			}}})
		case "/api2/json/config/prune":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("permission denied"))
		case "/api2/json/admin/datastore/fast/gc":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"schedule":         "daily",
				"last-run-state":   "OK",
				"last-run-endtime": 1700000200,
			}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Host: server.URL, TokenName: "root@pam!token", TokenValue: "secret"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	evidence, err := client.GetJobHealthEvidence(context.Background(), []string{"fast"}, JobHealthOptions{
		MonitorBackups:     true,
		MonitorSyncJobs:    true,
		MonitorVerifyJobs:  true,
		MonitorPruneJobs:   true,
		MonitorGarbageJobs: true,
	})
	if err != nil {
		t.Fatalf("GetJobHealthEvidence: %v", err)
	}

	byID := make(map[string]JobHealthEvidence)
	for _, item := range evidence {
		byID[item.ID] = item
	}
	if got := byID["sync-remote-a"]; got.Confidence != "direct-task-match" || got.EvidenceSource != JobEvidenceSourcePBSJobConfig || got.EvidenceScope != JobEvidenceScopeConfiguredJob || got.UPID != "UPID:sync:1" || got.LastRunState != "OK" || got.NextRun != 1700003600 {
		t.Fatalf("expected direct sync evidence with raw last-run fields, got %+v", got)
	}
	if got := byID["verify-fast"]; got.Confidence != "direct-config-last-run" || got.LastRunState != "OK" {
		t.Fatalf("expected config last-run verify evidence, got %+v", got)
	}
	if got := byID["prune:partial"]; got.Confidence != "partial-permission" || got.EvidenceScope != JobEvidenceScopePartialRead || got.Error == "" {
		t.Fatalf("expected partial permission prune evidence, got %+v", got)
	}
	if got := byID["backup:vm/100"]; got.Confidence != JobEvidenceConfidenceObservedBackupTask || got.EvidenceSource != JobEvidenceSourcePBSTaskHistory || got.EvidenceScope != JobEvidenceScopeObservedTask || got.Schedule != "" || got.NextRun != 0 || got.TaskStatus != "OK" {
		t.Fatalf("expected observed backup task evidence without scheduled compliance, got %+v", got)
	}
	if got := byID["garbage:fast"]; got.Confidence != "direct-config-last-run" || got.Store != "fast" {
		t.Fatalf("expected garbage config evidence, got %+v", got)
	}
}

func TestClient_GetJobHealthEvidence_UsesBoundedFilteredTaskHistory(t *testing.T) {
	oldLimit := pbsTaskHistoryPageLimit
	oldPages := pbsTaskHistoryMaxPages
	oldLookback := pbsTaskHistoryLookback
	pbsTaskHistoryPageLimit = 2
	pbsTaskHistoryMaxPages = 2
	pbsTaskHistoryLookback = time.Hour
	t.Cleanup(func() {
		pbsTaskHistoryPageLimit = oldLimit
		pbsTaskHistoryMaxPages = oldPages
		pbsTaskHistoryLookback = oldLookback
	})

	var queries []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/localhost/tasks" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		queries = append(queries, q)
		if q.Get("typefilter") != "backup" {
			t.Fatalf("typefilter = %q, want backup", q.Get("typefilter"))
		}
		if q.Get("store") != "fast" {
			t.Fatalf("store = %q, want fast", q.Get("store"))
		}
		if q.Get("since") == "" || q.Get("until") == "" {
			t.Fatalf("expected bounded since/until filters, got %s", r.URL.RawQuery)
		}
		if q.Get("limit") != "2" {
			t.Fatalf("limit = %q, want 2", q.Get("limit"))
		}

		statusFilters := q["statusfilter"]
		if slices.Contains(statusFilters, "error") {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
				"upid":      "UPID:backup:error",
				"type":      "backup",
				"id":        "vm/failed",
				"status":    "error",
				"starttime": 1700000400,
				"endtime":   1700000500,
			}}})
			return
		}
		if slices.Contains(statusFilters, "warning") {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			return
		}

		switch q.Get("start") {
		case "":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"upid": "UPID:backup:1", "type": "backup", "id": "vm/100", "status": "OK", "starttime": 1700000000, "endtime": 1700000100},
				{"upid": "UPID:backup:2", "type": "backup", "id": "vm/101", "status": "OK", "starttime": 1700000200, "endtime": 1700000300},
			}})
		case "2":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"upid": "UPID:backup:3", "type": "backup", "id": "vm/102", "status": "OK", "starttime": 1700000400, "endtime": 1700000500},
				{"upid": "UPID:backup:4", "type": "backup", "id": "vm/103", "status": "OK", "starttime": 1700000600, "endtime": 1700000700},
			}})
		default:
			t.Fatalf("unexpected start=%q", q.Get("start"))
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Host: server.URL, TokenName: "root@pam!token", TokenValue: "secret"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	evidence, err := client.GetJobHealthEvidence(context.Background(), []string{"fast"}, JobHealthOptions{MonitorBackups: true})
	if err != nil {
		t.Fatalf("GetJobHealthEvidence: %v", err)
	}

	byID := make(map[string]JobHealthEvidence)
	for _, item := range evidence {
		byID[item.ID] = item
	}
	if got := byID["backup:vm/failed"]; got.Confidence != JobEvidenceConfidenceObservedBackupTask || got.TaskStatus != "error" {
		t.Fatalf("expected error statusfilter task to be preserved as observed backup evidence, got %+v", got)
	}
	if got := byID["backup:task-history:fast:truncated"]; got.Confidence != JobEvidenceConfidenceHistoryTruncated || got.EvidenceScope != JobEvidenceScopePartialRead || got.Error == "" {
		t.Fatalf("expected visible truncation evidence, got %+v", got)
	}

	var sawSecondPage, sawErrorStatusFilter bool
	for _, q := range queries {
		if q.Get("start") == "2" {
			sawSecondPage = true
		}
		if slices.Contains(q["statusfilter"], "error") {
			sawErrorStatusFilter = true
		}
	}
	if !sawSecondPage || !sawErrorStatusFilter {
		t.Fatalf("expected pagination and statusfilter fallback queries, got %#v", queries)
	}
}

func TestNewClient_PasswordAuth_FallsBackToFormOnUnsupportedMediaType(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/access/ticket" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		switch atomic.AddInt32(&calls, 1) {
		case 1:
			if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
				t.Fatalf("expected json content-type, got %q", ct)
			}
			w.WriteHeader(http.StatusUnsupportedMediaType)
			_, _ = w.Write([]byte("unsupported"))
		case 2:
			if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/x-www-form-urlencoded") {
				t.Fatalf("expected form content-type, got %q", ct)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"ticket":              "ticket123",
					"CSRFPreventionToken": "csrf456",
				},
			})
		default:
			t.Fatalf("unexpected call count")
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:     server.URL,
		User:     "root@pam",
		Password: "password",
		Timeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.auth.ticket != "ticket123" || client.auth.csrfToken != "csrf456" {
		t.Fatalf("expected auth fields to be set, got ticket=%q csrf=%q", client.auth.ticket, client.auth.csrfToken)
	}
}

func TestClient_request_SendsTicketAndCSRFToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/test" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Cookie"); got != "PBSAuthCookie=ticket123" {
			t.Fatalf("Cookie = %q", got)
		}
		if got := r.Header.Get("CSRFPreventionToken"); got != "csrf456" {
			t.Fatalf("CSRFPreventionToken = %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:    server.URL,
		User:    "root@pam",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	client.auth.ticket = "ticket123"
	client.auth.csrfToken = "csrf456"
	client.auth.expiresAt = time.Now().Add(time.Hour)

	resp, err := client.request(context.Background(), http.MethodPost, "/test", url.Values{"a": {"b"}})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()
}

func TestClient_GetNodeStatus_PermissionDeniedReturnsNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/localhost/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("permission denied"))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!pulse-token",
		TokenValue: "secret",
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	status, err := client.GetNodeStatus(context.Background())
	if err != nil {
		t.Fatalf("GetNodeStatus: %v", err)
	}
	if status != nil {
		t.Fatalf("expected nil status on permission error, got: %+v", status)
	}
}

func TestClient_GetDatastores_HTMLResponseOnHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/admin/datastore" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>error</body></html>"))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL, // http://...
		TokenName:  "root@pam!pulse-token",
		TokenValue: "secret",
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.GetDatastores(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "Try changing your URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}
