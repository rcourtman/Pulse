package pbs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newRootNodeNameClient(t *testing.T, host string) *Client {
	t.Helper()
	client, err := NewClient(ClientConfig{
		Host:    host,
		User:    "root@pam",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

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

func TestClient_ListRunningDataTasks(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/localhost/tasks" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("running"); got != "true" {
			t.Fatalf("running query = %q, want true", got)
		}
		if got := r.URL.Query().Get("limit"); got != "200" {
			t.Fatalf("limit query = %q, want 200", got)
		}
		workerType := r.URL.Query().Get("typefilter")
		requests = append(requests, workerType)
		data := []map[string]any{}
		switch workerType {
		case "backup":
			data = append(data,
				map[string]any{"worker_type": "backup", "worker_id": "main:vm/117", "starttime": 1800000000},
				map[string]any{"worker_type": "verificationjob", "worker_id": "verify-main", "starttime": 1800000200},
			)
		case "syncjob":
			data = append(data, map[string]any{"worker-type": "syncjob", "worker-id": "offsite", "start-time": 1800000100})
		default:
			t.Fatalf("unexpected typefilter: %q", workerType)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Host: server.URL, TokenName: "root@pam!token", TokenValue: "secret"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	tasks, err := client.ListRunningDataTasks(context.Background())
	if err != nil {
		t.Fatalf("ListRunningDataTasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("running data tasks = %#v, want backup and sync only", tasks)
	}
	if !slices.Equal(requests, []string{"backup", "syncjob"}) {
		t.Fatalf("type-filtered requests = %v, want backup then syncjob", requests)
	}
	if tasks[0].WorkerType != "backup" || tasks[0].WorkerID != "main:vm/117" || tasks[0].StartTime != 1800000000 {
		t.Fatalf("backup task = %#v", tasks[0])
	}
	if tasks[1].WorkerType != "syncjob" || tasks[1].WorkerID != "offsite" || tasks[1].StartTime != 1800000100 {
		t.Fatalf("sync task = %#v", tasks[1])
	}
}

func TestClient_ListRunningDataTasksRejectsTruncatedWriterSet(t *testing.T) {
	oldLimit, oldMaxPages := pbsRunningTaskPageLimit, pbsRunningTaskMaxPages
	pbsRunningTaskPageLimit, pbsRunningTaskMaxPages = 2, 1
	t.Cleanup(func() {
		pbsRunningTaskPageLimit, pbsRunningTaskMaxPages = oldLimit, oldMaxPages
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"worker_type": "backup", "worker_id": "main:vm/117"},
			{"worker_type": "backup", "worker_id": "main:vm/118"},
		}})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{Host: server.URL, TokenName: "root@pam!token", TokenValue: "secret"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	tasks, err := client.ListRunningDataTasks(context.Background())
	if err == nil || !strings.Contains(err.Error(), "bounded cap") {
		t.Fatalf("ListRunningDataTasks() tasks = %#v, err = %v; want bounded-cap error", tasks, err)
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

func TestPBSJobTaskHistoryQueries_UsesWorkerTypeFilters(t *testing.T) {
	queries := buildTaskHistoryQueries([]string{"fast", "fast", "slow"}, JobHealthOptions{
		MonitorBackups:     true,
		MonitorSyncJobs:    true,
		MonitorVerifyJobs:  true,
		MonitorPruneJobs:   true,
		MonitorGarbageJobs: true,
	}, 1700000000, 1700003600)

	got := make(map[string][]taskHistoryQuery)
	for _, query := range queries {
		got[query.Family] = append(got[query.Family], query)
		if query.Since != 1700000000 || query.Until != 1700003600 {
			t.Fatalf("query %s bounds = (%d, %d), want (1700000000, 1700003600)", query.Family, query.Since, query.Until)
		}
	}

	backupQueries := got["backup"]
	if len(backupQueries) != 2 {
		t.Fatalf("backup query count = %d, want 2: %#v", len(backupQueries), backupQueries)
	}
	backupStores := map[string]bool{}
	for _, query := range backupQueries {
		if query.TypeFilter != "backup" {
			t.Fatalf("backup typefilter = %q, want backup", query.TypeFilter)
		}
		backupStores[query.Store] = true
	}
	if !backupStores["fast"] || !backupStores["slow"] {
		t.Fatalf("backup stores = %#v, want fast and slow", backupStores)
	}

	for family, want := range map[string]string{
		"sync":    "syncjob",
		"verify":  "verificationjob",
		"prune":   "prunejob",
		"garbage": "garbage_collection",
	} {
		familyQueries := got[family]
		if len(familyQueries) != 1 {
			t.Fatalf("%s query count = %d, want 1: %#v", family, len(familyQueries), familyQueries)
		}
		query := familyQueries[0]
		if query.TypeFilter != want {
			t.Fatalf("%s typefilter = %q, want %q", family, query.TypeFilter, want)
		}
		if query.Store != "" {
			t.Fatalf("%s store filter = %q, want empty", family, query.Store)
		}
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

func TestClient_GetNodeName_CachesResult(t *testing.T) {
	var nodesCalls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		nodesCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"node": "pbs-main"}},
		})
	}))
	defer server.Close()

	client := newRootNodeNameClient(t, server.URL)

	for i := 0; i < 3; i++ {
		name, err := client.GetNodeName(context.Background())
		if err != nil {
			t.Fatalf("GetNodeName call %d: %v", i, err)
		}
		if name != "pbs-main" {
			t.Fatalf("GetNodeName call %d = %q, want pbs-main", i, name)
		}
	}
	if got := nodesCalls.Load(); got != 1 {
		t.Fatalf("/nodes hit %d times, want 1 (cached after first success)", got)
	}
}

func TestClient_GetNodeName_SuperuserPermissionFailureRetries(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var nodesCalls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api2/json/nodes" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				if nodesCalls.Add(1) == 1 {
					w.WriteHeader(status)
					_, _ = w.Write([]byte(`{"message":"access denied"}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": []map[string]any{{"node": "pbs-recovered"}},
				})
			}))
			defer server.Close()

			client := newRootNodeNameClient(t, server.URL)

			_, firstErr := client.GetNodeName(context.Background())
			if firstErr == nil {
				t.Fatal("first GetNodeName: expected error")
			}
			if got, ok := pbsHTTPStatus(firstErr); !ok || got != status {
				t.Fatalf("first GetNodeName status = (%d, %v), want (%d, true): %v", got, ok, status, firstErr)
			}
			name, err := client.GetNodeName(context.Background())
			if err != nil {
				t.Fatalf("second GetNodeName: %v", err)
			}
			if name != "pbs-recovered" {
				t.Fatalf("second GetNodeName = %q, want pbs-recovered", name)
			}
			if got := nodesCalls.Load(); got != 2 {
				t.Fatalf("/nodes hit %d times, want 2 (%d is retryable for direct root session)", got, status)
			}
		})
	}
}

func TestClient_GetNodeName_TransientHTTPFailuresRetryAndRecover(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{"message":"slow down"}`},
		{name: "internal server error", status: http.StatusInternalServerError, body: `{"message":"temporary failure"}`},
		{name: "service unavailable body mentions permission", status: http.StatusServiceUnavailable, body: `{"message":"permission service unavailable"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var nodesCalls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api2/json/nodes" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				if nodesCalls.Add(1) == 1 {
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.body))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": []map[string]any{{"node": "pbs-recovered"}},
				})
			}))
			defer server.Close()

			client := newRootNodeNameClient(t, server.URL)

			_, firstErr := client.GetNodeName(context.Background())
			if firstErr == nil {
				t.Fatal("first GetNodeName: expected error")
			}
			if got, ok := pbsHTTPStatus(firstErr); !ok || got != tc.status {
				t.Fatalf("first GetNodeName status = (%d, %v), want (%d, true): %v", got, ok, tc.status, firstErr)
			}
			name, err := client.GetNodeName(context.Background())
			if err != nil {
				t.Fatalf("recovery GetNodeName: %v", err)
			}
			if name != "pbs-recovered" {
				t.Fatalf("recovery GetNodeName = %q, want pbs-recovered", name)
			}
			if got := nodesCalls.Load(); got != 2 {
				t.Fatalf("/nodes hit %d times, want 2 (transient failure then recovery)", got)
			}
		})
	}
}

func TestClient_GetNodeName_NetworkFailureRetriesAndRecovers(t *testing.T) {
	client := newRootNodeNameClient(t, "http://pbs.example.test")

	var calls atomic.Int64
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			return nil, errors.New("network permission proxy failure")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"node":"pbs-network-recovered"}]}`)),
			Request:    req,
		}, nil
	})

	if _, err := client.GetNodeName(context.Background()); err == nil {
		t.Fatal("first GetNodeName: expected network error")
	}

	name, err := client.GetNodeName(context.Background())
	if err != nil {
		t.Fatalf("recovery GetNodeName: %v", err)
	}
	if name != "pbs-network-recovered" {
		t.Fatalf("recovery GetNodeName = %q, want pbs-network-recovered", name)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("transport called %d times, want 2", got)
	}
}

func TestClient_GetNodeName_UnsupportedAuthNeverCallsNodes(t *testing.T) {
	tests := []struct {
		name   string
		config ClientConfig
	}{
		{
			name: "API token including root owner",
			config: ClientConfig{
				TokenName:  "root@pam!pulse-token",
				TokenValue: "secret",
			},
		},
		{
			name: "non-superuser password identity",
			config: ClientConfig{
				User: "admin@pbs",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var nodesCalls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nodesCalls.Add(1)
				t.Fatalf("unsupported auth requested %s", r.URL.Path)
			}))
			defer server.Close()

			tc.config.Host = server.URL
			tc.config.Timeout = 2 * time.Second
			client, err := NewClient(tc.config)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}

			for i := 0; i < 3; i++ {
				name, err := client.GetNodeName(context.Background())
				if name != "" {
					t.Fatalf("GetNodeName call %d = %q, want empty", i, name)
				}
				if !errors.Is(err, ErrNodeNameUnavailableForAuth) {
					t.Fatalf("GetNodeName call %d error = %v, want ErrNodeNameUnavailableForAuth", i, err)
				}
			}
			if got := nodesCalls.Load(); got != 0 {
				t.Fatalf("/nodes hit %d times, want 0", got)
			}
		})
	}
}

func TestClient_GetNodeName_ConcurrentTransientFailureIsSingleFlight(t *testing.T) {
	const callers = 16

	var nodesCalls atomic.Int64
	requestStarted := make(chan struct{})
	releaseFirstRequest := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if nodesCalls.Add(1) == 1 {
			close(requestStarted)
			<-releaseFirstRequest
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"message":"temporary permission backend outage"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"node": "pbs-concurrent-recovery"}},
		})
	}))
	defer server.Close()

	client := newRootNodeNameClient(t, server.URL)

	start := make(chan struct{})
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := client.GetNodeName(context.Background())
			errs <- err
		}()
	}
	close(start)
	<-requestStarted

	deadline := time.Now().Add(2 * time.Second)
	for {
		client.nodeNameMu.Lock()
		joined := 0
		if client.nodeNameAttempt != nil {
			joined = client.nodeNameAttempt.joined
		}
		client.nodeNameMu.Unlock()
		if joined == callers-1 {
			break
		}
		if time.Now().After(deadline) {
			close(releaseFirstRequest)
			t.Fatalf("only %d of %d concurrent callers joined the in-flight request", joined, callers-1)
		}
		time.Sleep(time.Millisecond)
	}
	close(releaseFirstRequest)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err == nil {
			t.Fatal("concurrent GetNodeName: expected transient error")
		}
		if got, ok := pbsHTTPStatus(err); !ok || got != http.StatusServiceUnavailable {
			t.Fatalf("concurrent GetNodeName status = (%d, %v), want (503, true): %v", got, ok, err)
		}
	}
	if got := nodesCalls.Load(); got != 1 {
		t.Fatalf("/nodes hit %d times for concurrent transient failure, want 1", got)
	}

	name, err := client.GetNodeName(context.Background())
	if err != nil {
		t.Fatalf("recovery GetNodeName: %v", err)
	}
	if name != "pbs-concurrent-recovery" {
		t.Fatalf("recovery GetNodeName = %q, want pbs-concurrent-recovery", name)
	}
	if _, err := client.GetNodeName(context.Background()); err != nil {
		t.Fatalf("cached GetNodeName: %v", err)
	}
	if got := nodesCalls.Load(); got != 2 {
		t.Fatalf("/nodes hit %d times after recovery and cache read, want 2", got)
	}
}
