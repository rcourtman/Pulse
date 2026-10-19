package pbs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
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

// A repeated setup must not invalidate another installation's saved secret.
// Exercise the real request and version readers against a stateful transport;
// no listener, live PBS, credential rotation or guest command is used.
func TestSetupMonitoringAccessCollisionKeepsExistingReader(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusConflict, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			const oldAuth = "PBSAPIToken=pulse-monitor@pbs!monitor1:fixture-existing-secret"
			live := true
			creates, deletes, tokenACLs, reads := 0, 0, 0, 0
			transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				code, body := http.StatusOK, `{"data":null}`
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/api2/json/version":
					reads++
					if !live || r.Header.Get("Authorization") != oldAuth {
						code, body = http.StatusUnauthorized, "fixture token no longer valid"
					} else {
						body = `{"data":{"version":"3.4","release":"1","repoid":"fixture"}}`
					}
				case r.Method == http.MethodPost && r.URL.Path == "/api2/json/access/users":
				case r.Method == http.MethodPost && r.URL.Path == "/api2/json/access/acl":
					if err := r.ParseForm(); err != nil {
						t.Fatal(err)
					}
					if strings.Contains(r.Form.Get("auth-id"), "!") {
						tokenACLs++
					}
				case r.URL.Path == "/api2/json/access/users/pulse-monitor@pbs/token/monitor1":
					if r.Method == http.MethodDelete {
						deletes++
						live = false
					} else if r.Method == http.MethodPost {
						creates++
						if live {
							code, body = status, "API token already exists"
						} else {
							body = `{"data":{"tokenid":"pulse-monitor@pbs!monitor1","value":"fixture-replacement-secret"}}`
						}
					} else {
						t.Fatalf("unexpected token method: %s", r.Method)
					}
				default:
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
			})
			admin, err := NewClient(ClientConfig{Host: "https://pbs.invalid", TokenName: "root@pam!fixture-admin", TokenValue: "fixture-admin-secret"})
			if err != nil {
				t.Fatal(err)
			}
			reader, err := NewClient(ClientConfig{Host: "https://pbs.invalid", TokenName: "pulse-monitor@pbs!monitor1", TokenValue: "fixture-existing-secret"})
			if err != nil {
				t.Fatal(err)
			}
			admin.httpClient.Transport, reader.httpClient.Transport = transport, transport
			for phase := 0; phase < 2; phase++ {
				version, err := reader.GetVersion(context.Background())
				if err != nil || version == nil || version.Version != "3.4" {
					t.Errorf("saved-token reader failed at phase %d: %v", phase, err)
				}
				if phase == 0 {
					id, secret, err := admin.SetupMonitoringAccess(context.Background(), "monitor1")
					if !errors.Is(err, ErrMonitoringTokenExists) || id != "" || secret != "" {
						t.Errorf("collision must not return replacement credentials: error=%v", err)
					}
					if got, ok := HTTPStatus(err); !ok || got != status {
						t.Errorf("lost provider status: %d, %v", got, ok)
					}
				}
			}
			if !live || creates != 1 || deletes != 0 || tokenACLs != 0 || reads != 2 {
				t.Errorf("collision changed credentials or dispatched more setup: live=%v creates=%d deletes=%d tokenACLs=%d reads=%d", live, creates, deletes, tokenACLs, reads)
			}
		})
	}
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
			if got, ok := HTTPStatus(firstErr); !ok || got != status {
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
			if got, ok := HTTPStatus(firstErr); !ok || got != tc.status {
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
		if got, ok := HTTPStatus(err); !ok || got != http.StatusServiceUnavailable {
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

// Restricted node metrics must not turn a later outage into a permission
// fallback, nor prevent recovery on the same long-lived polling client.
func TestClient_GetNodeStatus_PermissionOutageRecovery(t *testing.T) {
	var responseCode atomic.Int32
	responseCode.Store(http.StatusForbidden)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/localhost/status" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		code := int(responseCode.Load())
		w.WriteHeader(code)
		if code != http.StatusOK {
			// Deliberately misleading body: classification must use HTTP status.
			_, _ = w.Write([]byte("permission denied: upstream authentication error"))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"cpu":0.25,"memory":{"used":500,"total":1000}}}`))
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{
		Host: server.URL, TokenName: "monitor@pbs!pulse",
		TokenValue: "test-only", Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	status, err := client.GetNodeStatus(context.Background())
	if err != nil || status != nil {
		t.Fatalf("restricted metrics = (%+v, %v), want (nil, nil)", status, err)
	}
	for _, code := range []int{http.StatusServiceUnavailable, http.StatusBadGateway} {
		responseCode.Store(int32(code))
		status, err = client.GetNodeStatus(context.Background())
		if status != nil || err == nil {
			t.Fatalf("outage %d = (%+v, %v), want nil status and error", code, status, err)
		}
		if got, ok := HTTPStatus(err); !ok || got != code {
			t.Fatalf("outage status = (%d, %v), want %d", got, ok, code)
		}
	}
	responseCode.Store(http.StatusOK)
	status, err = client.GetNodeStatus(context.Background())
	if err != nil || status == nil {
		t.Fatalf("recovery = (%+v, %v), want metrics", status, err)
	}
	if status.CPU != 0.25 {
		t.Fatalf("recovered CPU = %v, want 0.25", status.CPU)
	}
}

func TestClient_GetNodeStatus_MissingData(t *testing.T) {
	for _, body := range []string{`{"data":null}`, `{}`, `null`} {
		t.Run(body, func(t *testing.T) {
			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			})
			defer server.Close()
			status, err := client.GetNodeStatus(context.Background())
			if status != nil || err == nil {
				t.Fatalf("absent metrics = (%+v, %v), want nil status and error", status, err)
			}
		})
	}
}

// Missing measurements must not be indistinguishable from measured zero usage.
func TestClient_GetNodeStatus_MetricCompleteness(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		valid      bool
	}{
		{"empty", `{}`, false},
		{"cpu only", `{"cpu":0}`, false},
		{"missing cpu", `{"memory":{"used":0,"total":1000}}`, false},
		{"null cpu", `{"cpu":null,"memory":{"used":0,"total":1000}}`, false},
		{"null memory", `{"cpu":0,"memory":null}`, false},
		{"missing used", `{"cpu":0,"memory":{"total":1000}}`, false},
		{"null used", `{"cpu":0,"memory":{"used":null,"total":1000}}`, false},
		{"missing total", `{"cpu":0,"memory":{"used":0}}`, false},
		{"null total", `{"cpu":0,"memory":{"used":0,"total":null}}`, false},
		{"zero total", `{"cpu":0,"memory":{"used":0,"total":0}}`, false},
		{"negative total", `{"cpu":0,"memory":{"used":0,"total":-1}}`, false},
		{"negative used", `{"cpu":0,"memory":{"used":-1,"total":1000}}`, false},
		{"negative cpu", `{"cpu":-1,"memory":{"used":0,"total":1000}}`, false},
		{"wrong type", `{"cpu":0,"memory":{"used":"0","total":1000}}`, false},
		{"measured zero", `{"cpu":0,"memory":{"used":0,"total":1000}}`, true},
		{"extra fields", `{"cpu":0.2,"memory":{"used":500,"total":1000,"free":500},"future":true}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"data":` + tc.data + `}`))
			})
			defer server.Close()
			status, err := client.GetNodeStatus(context.Background())
			if tc.valid {
				if err != nil || status == nil {
					t.Fatalf("valid measurements = (%+v, %v)", status, err)
				}
			} else if err == nil || status != nil {
				t.Fatalf("incomplete/invalid measurements = (%+v, %v), want nil status and error", status, err)
			}
		})
	}
}

type backupInventoryTestBody struct {
	io.Reader
	tailErr, closeErr error
	closed            bool
}

func (b *backupInventoryTestBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if err == io.EOF && b.tailErr != nil {
		return n, b.tailErr
	}
	return n, err
}

func (b *backupInventoryTestBody) Close() error { b.closed = true; return b.closeErr }

// Exercise both real HTTP readers, including errors delivered after a complete
// JSON prefix. A JSON decoder alone can return success without seeing EOF.
func TestPBSBackupInventoryAdmission(t *testing.T) {
	type inventoryCase struct {
		name, body        string
		status            int
		tailErr, closeErr error
		want              int
		valid             bool
	}
	const group = `{"backup-type":"vm","backup-id":"100","backup-count":1,"last-backup":1792028400}`
	const snapshot = `{"backup-type":"vm","backup-id":"100","backup-time":1792028400,"size":1024,"files":["index.json.blob"],"verification":{"state":"ok"}}`
	for _, endpoint := range []string{"groups", "snapshots"} {
		t.Run(endpoint, func(t *testing.T) {
			row := group
			if endpoint == "snapshots" {
				row = snapshot
			}
			good := `{"data":[` + row + `]}`
			cases := []inventoryCase{
				{name: "complete", body: good, valid: true, want: 1},
				{name: "empty", body: `{"data":[]}`, valid: true},
				{name: "extended", body: `{"extra":{"count":3},"errors":null,"data":[` + strings.TrimSuffix(row, "}") + `,"extra":"supported"}]}`, valid: true, want: 1},
				{name: "whitespace", body: good + " \r\n\t", valid: true, want: 1},
				{name: "exact-size-limit", body: good + strings.Repeat(" ", int(maxResponseBodyBytes)-len(good)), valid: true, want: 1},
				{name: "missing-data", body: `{}`},
				{name: "null-data", body: `{"data":null}`},
				{name: "null-envelope", body: `null`},
				{name: "case-data", body: strings.Replace(good, `"data"`, `"Data"`, 1)},
				{name: "duplicate-data", body: `{"data":[],"data":[` + row + `]}`},
				{name: "conflicting-errors", body: strings.TrimSuffix(good, "}") + `,"errors":{"fixture":"private-provider-marker"}}`},
				{name: "trailing-value", body: good + ` {"data":[]}`},
				{name: "trailing-garbage", body: good + ` private-provider-marker`},
				{name: "incomplete", body: strings.TrimSuffix(good, "}")},
				{name: "read-failure-after-json", body: good, tailErr: io.ErrUnexpectedEOF},
				{name: "close-failure", body: good, closeErr: errors.New("fixture close failed")},
				{name: "over-limit-after-json", body: good + strings.Repeat(" ", int(maxResponseBodyBytes))},
				{name: "partial-http-status", body: good, status: http.StatusPartialContent},
				{name: "null-row", body: `{"data":[` + row + `,null]}`},
				{name: "empty-row", body: `{"data":[` + row + `,{}]}`},
				{name: "duplicate-row", body: `{"data":[` + row + `,` + row + `]}`},
				{name: "invalid-utf8", body: strings.TrimSuffix(good, "}") + `,"comment":"` + string([]byte{0xff}) + `"}`},
				{name: "duplicate-id", body: `{"data":[` + strings.TrimSuffix(row, "}") + `,"backup-id":"101"}]}`},
				{name: "case-id", body: strings.Replace(good, `"backup-id"`, `"Backup-Id"`, 1)},
				{name: "blank-id", body: strings.Replace(good, `"100"`, `" "`, 1)},
				{name: "null-id", body: strings.Replace(good, `"100"`, `null`, 1)},
			}
			if endpoint == "groups" {
				cases = append(cases,
					inventoryCase{name: "missing-count", body: `{"data":[{"backup-type":"vm","backup-id":"100","last-backup":1792028400}]}`},
					inventoryCase{name: "negative-count", body: strings.Replace(good, `"backup-count":1`, `"backup-count":-1`, 1)},
					inventoryCase{name: "missing-last-backup", body: strings.Replace(good, `,"last-backup":1792028400`, "", 1)},
					inventoryCase{name: "empty-group", body: `{"data":[{"backup-type":"host","backup-id":"server","backup-count":0}]}`, valid: true, want: 1},
				)
			} else {
				cases = append(cases,
					inventoryCase{name: "wrong-group", body: strings.Replace(good, `"100"`, `"101"`, 1)},
					inventoryCase{name: "missing-time", body: strings.Replace(good, `,"backup-time":1792028400`, "", 1)},
					inventoryCase{name: "duplicate-verification-state", body: strings.Replace(good, `"state":"ok"`, `"state":"failed","state":"ok"`, 1)},
					inventoryCase{name: "string-verification", body: strings.Replace(good, `{"state":"ok"}`, `"ok"`, 1), valid: true, want: 1},
					inventoryCase{name: "object-files", body: strings.Replace(good, `["index.json.blob"]`, `[{"filename":"index.json.blob","size":100}]`, 1), valid: true, want: 1},
					inventoryCase{name: "in-progress", body: `{"data":[{"backup-type":"vm","backup-id":"100","backup-time":1792028400}]}`, valid: true, want: 1},
				)
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					body := &backupInventoryTestBody{Reader: strings.NewReader(tc.body), tailErr: tc.tailErr, closeErr: tc.closeErr}
					status := tc.status
					if status == 0 {
						status = http.StatusOK
					}
					calls := 0
					c, err := NewClient(ClientConfig{Host: "https://pbs-inventory.invalid", TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
					if err != nil {
						t.Fatal(err)
					}
					c.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
						calls++
						if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/"+endpoint) || r.URL.Query().Get("ns") != "department" {
							t.Errorf("unexpected request: %s %s", r.Method, r.URL)
						}
						return &http.Response{StatusCode: status, Header: make(http.Header), Body: body}, nil
					})
					var count int
					if endpoint == "groups" {
						v, e := c.ListBackupGroups(context.Background(), "main", "department")
						count, err = len(v), e
					} else {
						v, e := c.ListBackupSnapshots(context.Background(), "main", "department", "vm", "100")
						count, err = len(v), e
					}
					if tc.valid {
						if err != nil || count != tc.want {
							t.Fatalf("complete inventory count=%d err=%v, want %d", count, err, tc.want)
						}
					} else if err == nil || count != 0 {
						t.Errorf("unsafe inventory admitted: count=%d err=%v", count, err)
					}
					if err != nil && strings.Contains(err.Error(), "private-provider-marker") {
						t.Error("provider body escaped into error")
					}
					if calls != 1 || !body.closed {
						t.Errorf("requests=%d closed=%t", calls, body.closed)
					}
				})
			}
		})
	}
}

func TestPBSNamespaceInventoryAdmission(t *testing.T) {
	const good = `{"data":[{"ns":"team/production","parent":"team"}]}`
	for _, tc := range []struct {
		name, body        string
		status            int
		tailErr, closeErr error
		valid             bool
		want              int
	}{
		{name: "complete", body: good, valid: true, want: 1},
		{name: "root", body: `{"data":[{"ns":""}]}`, valid: true, want: 1},
		{name: "path-alias", body: `{"data":[{"path":"team/production"}]}`, valid: true, want: 1},
		{name: "name-alias", body: `{"data":[{"name":"team"}]}`, valid: true, want: 1},
		{name: "empty", body: `{"data":[]}`, valid: true},
		{name: "legacy-not-found", body: "fixture absent", status: http.StatusNotFound, valid: true},
		{name: "extended", body: `{"extra":true,"errors":null,"data":[{"ns":"team","extra":7}]}`, valid: true, want: 1},
		{name: "exact-limit", body: good + strings.Repeat(" ", int(maxResponseBodyBytes)-len(good)), valid: true, want: 1},
		{name: "unavailable-mentions-404", body: "fixture refers to 404", status: http.StatusServiceUnavailable},
		{name: "access-denied", body: "fixture denied", status: http.StatusForbidden},
		{name: "missing-data", body: `{}`},
		{name: "null-data", body: `{"data":null}`},
		{name: "wrong-data-type", body: `{"data":{}}`},
		{name: "duplicate-data", body: `{"data":[],"data":[{"ns":"team"}]}`},
		{name: "case-data", body: `{"Data":[]}`},
		{name: "provider-error", body: `{"data":[],"errors":{"private-provider-marker":"unavailable"}}`},
		{name: "trailing-json", body: good + ` {"data":[]}`},
		{name: "trailing-garbage", body: good + ` private-provider-marker`},
		{name: "truncated", body: strings.TrimSuffix(good, "}")},
		{name: "null-row", body: `{"data":[null]}`},
		{name: "missing-identity", body: `{"data":[{}]}`},
		{name: "null-identity", body: `{"data":[{"ns":null}]}`},
		{name: "nontext-identity", body: `{"data":[{"ns":1}]}`},
		{name: "duplicate-identity", body: `{"data":[{"ns":"team","ns":"other"}]}`},
		{name: "case-identity", body: `{"data":[{"NS":"team"}]}`},
		{name: "duplicate-namespace", body: `{"data":[{"ns":"team"},{"name":"team"}]}`},
		{name: "duplicate-root", body: `{"data":[{"ns":""},{"path":"/"}]}`},
		{name: "invalid-utf8", body: `{"data":[{"ns":"` + string([]byte{0xff}) + `"}]}`},
		{name: "partial-http-status", body: good, status: http.StatusPartialContent},
		{name: "read-error-after-json", body: good, tailErr: errors.New("private-provider-marker")},
		{name: "close-error", body: good, closeErr: errors.New("private-provider-marker")},
		{name: "over-limit", body: good + strings.Repeat(" ", int(maxResponseBodyBytes))},
		{name: "read-truncation", body: good, tailErr: io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &backupInventoryTestBody{Reader: strings.NewReader(tc.body), tailErr: tc.tailErr, closeErr: tc.closeErr}
			status := tc.status
			if status == 0 {
				status = http.StatusOK
			}
			calls := 0
			c, err := NewClient(ClientConfig{Host: "https://pbs-namespaces.invalid", TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
			if err != nil {
				t.Fatal(err)
			}
			c.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != "/api2/json/admin/datastore/main/namespace" || r.URL.Query().Get("ns") != "team" || r.URL.Query().Get("max-depth") != "7" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: body}, nil
			})
			values, err := c.ListNamespaces(context.Background(), "main", "team", 7)
			if tc.valid {
				if err != nil || len(values) != tc.want {
					t.Fatalf("complete namespace count=%d err=%v, want %d", len(values), err, tc.want)
				}
			} else if err == nil || len(values) != 0 {
				t.Errorf("unsafe namespace inventory admitted: count=%d err=%v", len(values), err)
			}
			if err != nil && strings.Contains(err.Error(), "private-provider-marker") {
				t.Error("private provider details escaped through namespace diagnostic")
			}
			if calls != 1 || !body.closed {
				t.Errorf("requests=%d closed=%t", calls, body.closed)
			}
		})
	}
}

// Root discovery decides whether a complete empty backup inventory is real.
// Validate the entire name list before filtering or making any detail request.
func TestPBSDatastoreInventoryAdmission(t *testing.T) {
	const good = `{"data":[{"store":"archive"},{"store":"mirror","comment":"fixture"}]}`
	for _, tc := range []struct {
		name, body        string
		status            int
		tailErr, closeErr error
		valid             bool
		names             []string
	}{
		{name: "complete", body: good, valid: true, names: []string{"archive", "mirror"}},
		{name: "empty", body: `{"data":[]}`, valid: true},
		{name: "extended", body: `{"errors":null,"extra":7,"data":[{"store":"archive","extra":true}]}`, valid: true, names: []string{"archive"}},
		{name: "missing", body: `{}`}, {name: "null", body: `{"data":null}`},
		{name: "wrong-type", body: `{"data":{}}`}, {name: "null-row", body: `{"data":[null]}`},
		{name: "missing-store", body: `{"data":[{}]}`}, {name: "blank-store", body: `{"data":[{"store":""}]}`},
		{name: "null-store", body: `{"data":[{"store":null}]}`}, {name: "space-store", body: `{"data":[{"store":" archive "}]}`},
		{name: "nontext-store", body: `{"data":[{"store":7}]}`},
		{name: "duplicate-store", body: `{"data":[{"store":"archive","store":"mirror"}]}`},
		{name: "case-store", body: `{"data":[{"Store":"archive"}]}`},
		{name: "duplicate-row", body: `{"data":[{"store":"archive"},{"store":"archive"}]}`},
		{name: "duplicate-data", body: `{"data":[],"data":[{"store":"archive"}]}`},
		{name: "case-data", body: `{"Data":[]}`},
		{name: "provider-error", body: `{"data":[],"errors":{"private-provider-marker":"unavailable"}}`},
		{name: "trailing-json", body: good + ` {"data":[]}`},
		{name: "trailing-garbage", body: good + ` private-provider-marker`},
		{name: "truncated", body: strings.TrimSuffix(good, "}")},
		{name: "invalid-utf8", body: `{"data":[{"store":"` + string([]byte{0xff}) + `"}]}`},
		{name: "read-error", body: good, tailErr: errors.New("private-provider-marker")},
		{name: "close-error", body: good, closeErr: errors.New("private-provider-marker")},
		{name: "truncated-body", body: good, tailErr: io.ErrUnexpectedEOF},
		{name: "partial-http", body: good, status: http.StatusPartialContent},
		{name: "exact-limit", body: good + strings.Repeat(" ", int(maxResponseBodyBytes)-len(good)), valid: true, names: []string{"archive", "mirror"}},
		{name: "over-limit", body: good + strings.Repeat(" ", int(maxResponseBodyBytes))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &backupInventoryTestBody{Reader: strings.NewReader(tc.body), tailErr: tc.tailErr, closeErr: tc.closeErr}
			status := tc.status
			if status == 0 {
				status = http.StatusOK
			}
			client, err := NewClient(ClientConfig{Host: "https://pbs-datastores.invalid", TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != "/api2/json/admin/datastore" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: body}, nil
			})
			var included []string
			values, err := client.GetDatastoresFiltered(context.Background(), func(name string) bool { included = append(included, name); return false })
			if tc.valid {
				if err != nil || !reflect.DeepEqual(included, tc.names) {
					t.Errorf("complete list names=%v err=%v, want %v", included, err, tc.names)
				}
			} else if err == nil || len(included) != 0 {
				t.Errorf("unsafe datastore inventory was filtered/admitted: names=%v err=%v", included, err)
			}
			if len(values) != 0 {
				t.Error("excluded stores received detail responses")
			}
			if err != nil && strings.Contains(err.Error(), "private-provider-marker") {
				t.Error("private provider details escaped the discovery diagnostic")
			}
			if calls != 1 || !body.closed {
				t.Errorf("requests=%d closed=%t", calls, body.closed)
			}
		})
	}
}
