package proxmox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// A complete HTTP envelope is not a complete task log if a numbered line was
// lost or its text silently decoded to a zero value. Keep errors bounded: log
// contents may include private guest names and command output.
func TestTaskLogRequiresNumberedTextSequence(t *testing.T) {
	for _, tc := range []struct{ name, rows string }{
		{"missing-number", `[{"t":"private-log-fixture"}]`},
		{"null-number", `[{"n":null,"t":"private-log-fixture"}]`},
		{"zero-number", `[{"n":0,"t":"private-log-fixture"}]`},
		{"negative-number", `[{"n":-1,"t":"private-log-fixture"}]`},
		{"prefix-missing", `[{"n":2,"t":"private-log-fixture"}]`},
		{"gap", `[{"n":1,"t":"start"},{"n":3,"t":"private-log-fixture"}]`},
		{"duplicate", `[{"n":1,"t":"start"},{"n":1,"t":"private-log-fixture"}]`},
		{"reordered", `[{"n":2,"t":"start"},{"n":1,"t":"private-log-fixture"}]`},
		{"missing-text", `[{"n":1,"t":"private-log-fixture"},{"n":2}]`},
		{"null-text", `[{"n":1,"t":"private-log-fixture"},{"n":2,"t":null}]`},
		{"wrong-number-type", `[{"n":"1","t":"private-log-fixture"}]`},
		{"wrong-text-type", `[{"n":1,"t":7}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("start") != "0" || r.URL.Query().Get("limit") != fmt.Sprint(taskLogLineLimit) {
					t.Error("task log request no longer starts at the bounded origin")
				}
				fmt.Fprint(w, `{"data":`+tc.rows+`}`)
			}))
			defer server.Close()
			lines, err := backupTestClient(t, server.URL).GetTaskLog(context.Background(), "node", "job")
			if err == nil || lines != nil {
				t.Fatalf("incomplete sequence published: %d rows, error=%v", len(lines), err)
			}
			if strings.Contains(err.Error(), "private-log-fixture") || isAuthError(err) {
				t.Fatal("invalid log became private diagnostics or an authentication diagnosis")
			}
		})
	}
	for _, rows := range []string{`[]`, `[{"n":1,"t":""},{"n":2,"t":"  "},{"n":3,"t":"INFO: complete","metadata":true}]`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":`+rows+`,"errors":null,"metadata":true}`)
		}))
		lines, err := backupTestClient(t, server.URL).GetTaskLog(context.Background(), "node", "job")
		server.Close()
		if err != nil || (rows != `[]` && (len(lines) != 3 || lines[0].Text != "" || lines[1].Text != "  ")) {
			t.Fatalf("complete empty/blank/additive log changed: %+v, %v", lines, err)
		}
	}
}

// Non-parallel, like the other diagnostic tests in this file. Large/additive
// replies must not copy guest data into logs even when Debug is enabled.
func TestGuestFilesystemResponseDiagnosticsNoPayload(t *testing.T) {
	const marker = "synthetic-private-filesystem-metadata"
	row := `{"name":"` + marker + `","mountpoint":"/data","type":"ext4","total-bytes":1000,"used-bytes":0}`
	unrecognized := `{"unused":"` + marker + `"}`
	largeUnrecognized := `{"unused":"` + marker + strings.Repeat("x", 128*1024) + `"}`
	for _, tc := range []struct {
		name, payload string
		wantRows      int
		wantError     bool
		resultBytes   int
	}{
		{"array", `{"data":{"result":[` + row + `]}}`, 1, false, 0},
		{"object", `{"data":{"result":` + row + `}}`, 1, false, 0},
		{"large-additive", `{"data":{"result":[` + row + `],"unused":"` + marker + strings.Repeat("x", 128*1024) + `"}}`, 1, false, 0},
		{"incomplete", `{"data":{"result":[` + row + `,{"mountpoint":"/other","type":"ext4","total-bytes":9000}],"unused":"` + marker + `"}}`, 1, true, 0},
		{"empty", `{"data":{"result":[],"unused":"` + marker + `"}}`, 0, false, 0},
		{"null", `{"data":{"result":null,"unused":"` + marker + `"}}`, 0, false, 0},
		{"unrecognized-object", `{"data":{"result":` + unrecognized + `}}`, 0, false, len(unrecognized)},
		{"large-unrecognized-object", `{"data":{"result":` + largeUnrecognized + `}}`, 0, false, len(largeUnrecognized)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var commands atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/config"):
					fmt.Fprint(w, `{"data":{}}`)
				case strings.HasSuffix(r.URL.Path, "/agent/get-fsinfo"):
					commands.Add(1)
					fmt.Fprint(w, tc.payload)
				default:
					t.Error("unexpected diagnostic fixture request")
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client := backupTestClient(t, server.URL)
			var captured bytes.Buffer
			prior, priorLevel := log.Logger, zerolog.GlobalLevel()
			log.Logger = zerolog.New(zerolog.SyncWriter(&captured)).Level(zerolog.DebugLevel)
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
			defer func() { log.Logger = prior; zerolog.SetGlobalLevel(priorLevel) }()
			rows, err := client.GetVMFSInfo(context.Background(), "node", 105)
			if (err != nil) != tc.wantError || len(rows) != tc.wantRows || commands.Load() != 1 {
				t.Fatal("diagnostic filtering changed decoding, completeness or command count")
			}
			if len(rows) > 0 && (rows[0].Name != marker || rows[0].UsedBytes != 0 || rows[0].TotalBytes != 1000 || rows[0].Mountpoint != "/data") {
				t.Fatal("diagnostic filtering changed a measured zero or guest metadata")
			}
			if strings.Contains(captured.String(), marker) || strings.Contains(captured.String(), `"response":`) || strings.Contains(captured.String(), `"result":`) {
				t.Fatal("filesystem reply contents escaped into diagnostics")
			}
			if captured.Len() > 2048 {
				t.Fatal("filesystem diagnostics grew with unused response data")
			}
			decoder := json.NewDecoder(&captured)
			found := false
			foundUnrecognized := false
			logBytes := captured.Len()
			for decoder.More() {
				var event map[string]interface{}
				if err := decoder.Decode(&event); err != nil {
					t.Fatal("diagnostic is not a complete JSON event")
				}
				if event["message"] == "Received guest agent filesystem response" {
					found = true
					if event["response_bytes"] != float64(len(tc.payload)) || event["node"] != "node" || event["vmid"] != float64(105) {
						t.Fatal("diagnostic lost useful response size or guest attribution")
					}
				}
				if event["message"] == "GetVMFSInfo received unrecognized result instead of filesystem array" {
					foundUnrecognized = true
					if event["result_bytes"] != float64(tc.resultBytes) || event["node"] != "node" || event["vmid"] != float64(105) {
						t.Fatal("unrecognized-result diagnostic lost useful size or guest attribution")
					}
				}
			}
			if !found {
				t.Fatal("bounded filesystem diagnostic is absent")
			}
			if foundUnrecognized != (tc.resultBytes > 0) {
				t.Fatal("unrecognized-result diagnostic does not match the decoded shape")
			}
			t.Logf("response_bytes=%d log_bytes=%d rows=%d incomplete=%t commands=%d", len(tc.payload), logBytes, len(rows), err != nil, commands.Load())
		})
	}
}

// Non-parallel: temporarily replace the package logger, then restore it before
// parallel tests can run. Only synthetic URLs/credentials enter this fixture.
func TestClientRejectedHostDiagnosticsContainNoCredentials(t *testing.T) {
	for index, host := range []string{
		"https://synthetic-user:synthetic-secret@pve.invalid",
		"http://synthetic-user:synthetic-secret@pve.invalid",
		"synthetic-user:synthetic-secret@pve.invalid",
		"https://pve.invalid?token=synthetic-secret",
		"http://pve.invalid#synthetic-secret",
		"pve.invalid?token=synthetic-secret",
		"https://synthetic-user:synthetic-secret@pve.invalid:bad-port",
		"https://pve.invalid/%invalid?token=synthetic-secret",
	} {
		// Use a safe index rather than the supplied host as the test name/output.
		t.Run(fmt.Sprintf("case-%d", index), func(t *testing.T) {
			var captured bytes.Buffer
			prior := log.Logger
			log.Logger = zerolog.New(&captured).Level(zerolog.DebugLevel)
			defer func() { log.Logger = prior }()
			client, err := NewClient(ClientConfig{Host: host, TokenName: "fixture@pve!pulse", TokenValue: "synthetic-token"})
			if client != nil || err == nil {
				t.Fatal("invalid URL must be refused before authentication")
			}
			if strings.Contains(err.Error(), "synthetic-secret") || strings.Contains(err.Error(), "synthetic-user") {
				t.Error("constructor diagnostic exposed rejected URL credentials")
			}
			if strings.Contains(captured.String(), "synthetic-secret") || strings.Contains(captured.String(), "synthetic-user") {
				t.Error("constructor log exposed rejected URL credentials")
			}
			t.Logf("refused=true captured_log_bytes=%d private_error=%t", captured.Len(), !strings.Contains(err.Error(), "synthetic-secret"))
		})
	}
}

func TestClientAcceptedHostDiagnosticsRemainUseful(t *testing.T) {
	for _, host := range []string{"pve.invalid/tenant", "http://pve.invalid/tenant", "https://pve.invalid/tenant"} {
		var captured bytes.Buffer
		prior := log.Logger
		log.Logger = zerolog.New(&captured).Level(zerolog.DebugLevel)
		client, err := NewClient(ClientConfig{Host: host, TokenName: "fixture@pve!pulse", TokenValue: "synthetic-token"})
		log.Logger = prior
		if err != nil {
			t.Fatal(err)
		}
		defer client.httpClient.CloseIdleConnections()
		if !strings.HasSuffix(client.baseURL, "/tenant/api2/json") {
			t.Fatal("diagnostic filtering changed the route")
		}
		if strings.Contains(captured.String(), "synthetic-token") {
			t.Fatal("accepted diagnostic disclosed a token")
		}
		decoder := json.NewDecoder(&captured)
		for decoder.More() {
			var event map[string]interface{}
			if err := decoder.Decode(&event); err != nil {
				t.Fatal(err)
			}
			if value, ok := event["host"].(string); ok {
				parsed, err := url.Parse(value)
				if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
					t.Fatal("host diagnostic is not a validated base URL")
				}
			}
		}
	}
}

func TestClusterRejectedEndpointDiagnosticsContainNoCredentials(t *testing.T) {
	for _, host := range []string{
		"https://synthetic-user:synthetic-secret@pve.invalid",
		"https://pve.invalid?token=synthetic-secret",
		"https://synthetic-user:synthetic-secret@pve.invalid:bad-port",
	} {
		for _, count := range []int{1, 2} {
			var captured bytes.Buffer
			prior := log.Logger
			log.Logger = zerolog.New(zerolog.SyncWriter(&captured)).Level(zerolog.DebugLevel)
			func() {
				defer func() { log.Logger = prior }()
				endpoints := []string{host}
				if count == 2 {
					endpoints = append(endpoints, "https://synthetic-user:synthetic-secret@other.invalid")
				}
				cc := NewClusterClient("fixture", ClientConfig{Host: host, TokenName: "fixture@pve!pulse", TokenValue: "synthetic-token"}, endpoints, nil)
				_, err := cc.GetNodes(context.Background())
				if err == nil {
					t.Error("rejected endpoints did not stop cluster routing")
				} else if strings.Contains(err.Error(), "synthetic-secret") || strings.Contains(err.Error(), "synthetic-user") {
					t.Error("cluster error exposed rejected endpoint credentials")
				}
				for _, status := range cc.GetHealthStatusWithErrors() {
					if strings.Contains(status.LastError, "synthetic-secret") {
						t.Error("cluster health error exposed rejected input")
					}
				}
				cc.mu.Lock()
				for ep := range cc.lastHealthCheck {
					cc.lastHealthCheck[ep] = time.Now().Add(-time.Hour)
				}
				cc.mu.Unlock()
				cc.recoverUnhealthyNodes(context.Background())
				for ep := range cc.GetHealthStatus() {
					if !strings.Contains(ep, "synthetic-secret") {
						t.Error("diagnostic filtering rewrote configured routing identity")
					}
				}
			}()
			if strings.Contains(captured.String(), "synthetic-secret") || strings.Contains(captured.String(), "synthetic-user") {
				t.Error("cluster log exposed rejected endpoint credentials")
			}
			t.Logf("configured_endpoints=%d captured_log_bytes=%d", count, captured.Len())
		}
	}
}

func TestClusterDiagnosticHealthSnapshotKeepsEveryEndpoint(t *testing.T) {
	cc := &ClusterClient{nodeHealth: map[string]bool{
		"https://pve.invalid/tenant":                      true,
		"https://fixture:synthetic-secret@first.invalid":  false,
		"https://fixture:synthetic-secret@second.invalid": true,
	}}
	cc.mu.Lock()
	defer cc.mu.Unlock()
	statuses := cc.endpointHealthDiagnosticsLocked()
	if len(statuses) != 3 || len(cc.nodeHealth) != 3 {
		t.Fatal("diagnostic labels collapsed membership")
	}
	invalid, healthy := 0, 0
	for _, status := range statuses {
		if status.Endpoint == "[invalid endpoint]" {
			invalid++
		}
		if status.Healthy {
			healthy++
		}
		if strings.Contains(status.Endpoint, "synthetic-secret") {
			t.Error("health diagnostic copied a raw map key")
		}
	}
	if invalid != 2 || healthy != 2 {
		t.Error("display snapshot lost endpoint status observations")
	}
}

// Initial authentication must obey the same caller lifetime as ordinary reads.
func TestClientConstructionHonoursContext(t *testing.T) {
	testClientConstructionHonoursContext(t)
}

// Unlike unpublished construction, renewal belongs to every live reader.
// Exercise both authentication encodings so restoring constructor cancellation
// cannot accidentally restore cancellation of a shared renewal too.
func TestClientSessionRenewalStarterCancellation(t *testing.T) {
	testClientSessionRenewalStarterCancellation(t)
}

func TestClusterCancelledCallerStartsNoSession(t *testing.T) {
	testClusterCancelledCallerStartsNoSession(t)
}

func TestClusterSessionConstructionHonoursCallerBudget(t *testing.T) {
	testClusterSessionConstructionHonoursCallerBudget(t)
}

// A malformed snapshot inventory is a collection failure, not a new empty
// list or an instruction to change credentials. Ordinary rereading recovers.
func TestSnapshotInventoryReadRecovery(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		t.Run(kind, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					fmt.Fprint(w, `{"data":null,"detail":"fixture-private-detail"}`)
					return
				}
				fmt.Fprint(w, `{"data":[{"name":"current"},{"name":"weekly","snaptime":1700000000}]}`)
			}))
			defer server.Close()
			client, err := NewClient(ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture"})
			if err != nil {
				t.Fatal(err)
			}
			defer client.httpClient.CloseIdleConnections()
			read := func() ([]Snapshot, error) {
				if kind == "lxc" {
					return client.GetContainerSnapshots(context.Background(), "member", 105)
				}
				return client.GetVMSnapshots(context.Background(), "member", 105)
			}
			if rows, err := read(); err == nil || rows != nil || strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "fixture-private-detail") {
				t.Fatalf("unknown inventory became empty success or unsafe diagnosis: %+v / %v", rows, err)
			}
			rows, err := read()
			if err != nil || len(rows) != 1 || rows[0].VMID != 105 || calls.Load() != 2 {
				t.Fatalf("ordinary snapshot reread failed: %+v / %v / %d calls", rows, err, calls.Load())
			}
		})
	}
}

// Storage replies can contain archive names and operator notes. A malformed
// success must be an unavailable inventory, never an auth diagnosis or a copy
// of the private payload in an error/log, including with Debug enabled.
func TestStorageInventoryDiagnosticsNoPayload(t *testing.T) {
	const marker = "synthetic-private-archive-metadata"
	for _, endpoint := range []string{"node-storages", "all-storages", "content"} {
		for _, malformed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/malformed=%t", endpoint, malformed), func(t *testing.T) {
				body := `{"data":[` + storageInventoryFixtureRow(endpoint) + `],"metadata":"` + marker + strings.Repeat("x", 128*1024) + `"}`
				if malformed {
					body = `{"data":[],"errors":{"private":"` + marker + strings.Repeat("x", 128*1024) + `"}}`
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
				defer server.Close()
				var captured bytes.Buffer
				prior, priorLevel := log.Logger, zerolog.GlobalLevel()
				log.Logger = zerolog.New(zerolog.SyncWriter(&captured)).Level(zerolog.DebugLevel)
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
				defer func() { log.Logger = prior; zerolog.SetGlobalLevel(priorLevel) }()
				rows, err := storageInventoryReader(t, server.URL, "single", endpoint)(context.Background())
				if (err != nil) != malformed || (!malformed && len(rows) != 1) || (malformed && (len(rows) != 0 || isAuthError(err))) {
					t.Fatal("diagnostic filtering changed inventory admission or auth classification")
				}
				if strings.Contains(captured.String(), marker) || (err != nil && strings.Contains(err.Error(), marker)) || captured.Len() > 2048 {
					t.Fatal("storage reply payload escaped or inflated diagnostics")
				}
			})
		}
	}
}

// A bounded wire body can contain thousands of rejected rows. It must not turn
// one ordinary read into one warning and one reserved filesystem slot per row.
// Synthetic HTTP only: no native QGA, backup or guest recovery is attempted.
func TestGuestFilesystemMalformedReplyCost(t *testing.T) {
	for _, malformed := range []int{0, 1, 32, 4096} {
		for _, level := range []zerolog.Level{zerolog.WarnLevel, zerolog.DebugLevel} {
			t.Run(fmt.Sprintf("rows-%d/%s", malformed, level), func(t *testing.T) {
				const marker = "synthetic-private-rejected-volume"
				peer := `{"name":"root","mountpoint":"/","type":"ext4","total-bytes":1000,"used-bytes":0}`
				bad := `{"mountpoint":"/` + marker + `","type":"ext4","total-bytes":9000}`
				// Valid peers at both ends pin ordering and the last measured zero.
				payload := `{"data":{"result":[` + peer + "," + strings.Repeat(bad+",", malformed) + peer + `]}}`
				var commands, configs atomic.Int32
				var healthy atomic.Bool
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.URL.Path, "/config") {
						configs.Add(1)
						fmt.Fprint(w, `{"data":{}}`)
						return
					}
					commands.Add(1)
					if strings.HasSuffix(r.URL.Path, "/get-fsinfo") && !healthy.Load() {
						fmt.Fprint(w, payload)
					} else {
						backupAgentPayload(w, r)
					}
				}))
				defer server.Close()
				client := commandReplyTestClient(t, server.URL+"/malformed-reply-cost/"+fmt.Sprint(malformed)+"/"+level.String())
				var captured bytes.Buffer
				prior, priorLevel := log.Logger, zerolog.GlobalLevel()
				log.Logger = zerolog.New(zerolog.SyncWriter(&captured)).Level(level)
				zerolog.SetGlobalLevel(level)
				defer func() { log.Logger = prior; zerolog.SetGlobalLevel(priorLevel) }()
				rows, err := client.GetVMFSInfo(context.Background(), "node", 105)
				if (err != nil) != (malformed > 0) || len(rows) != 2 || commands.Load() != 1 || configs.Load() != 2 {
					t.Fatal("diagnostic bound changed valid peers, completeness or command/lock count")
				}
				if err != nil && err.Error() != fmt.Sprintf("guest filesystem inventory is incomplete: %d malformed entries", malformed) {
					t.Fatal("diagnostic bound changed the existing counted error")
				}
				for _, row := range rows {
					if row.Name != "root" || row.TotalBytes != 1000 || row.UsedBytes != 0 || row.Disk != "root-filesystem" {
						t.Fatal("valid peer identity, order, bytes or measured zero changed")
					}
				}
				if strings.Contains(captured.String(), marker) {
					t.Fatal("rejected volume payload escaped into diagnostics")
				}
				logBytes := captured.Len()
				decoder := json.NewDecoder(bytes.NewReader(captured.Bytes()))
				var warnings []map[string]interface{}
				for decoder.More() {
					var event map[string]interface{}
					if err := decoder.Decode(&event); err != nil {
						t.Fatal("malformed log event")
					}
					if event["level"] == "warn" {
						warnings = append(warnings, event)
					}
				}
				t.Logf("response_bytes=%d malformed=%d log_bytes=%d warnings=%d accepted=%d commands=%d configs=%d", len(payload), malformed, logBytes, len(warnings), len(rows), commands.Load(), configs.Load())
				wantWarnings := 0
				if malformed > 0 {
					wantWarnings = 1
				}
				if len(warnings) != wantWarnings || logBytes > 2048 {
					t.Errorf("diagnostics amplify rejected rows: warnings=%d want=%d bytes=%d limit=2048", len(warnings), wantWarnings, logBytes)
				} else if malformed > 0 {
					event := warnings[0]
					if event["malformed_entries"] != float64(malformed) || event["filesystem_entries"] != float64(malformed+2) || event["first_invalid_index"] != float64(1) || event["node"] != "node" || event["vmid"] != float64(105) {
						t.Error("counted warning lost rejection total, first index or guest attribution")
					} else if malformed == 4096 && level == zerolog.WarnLevel {
						// Real producer event, consumed unchanged by the browser proof.
						wire, _ := json.Marshal(event)
						t.Logf("FILESYSTEM_LOG_PRODUCER %s", wire)
					}
				}
				// Malformed completed readings are not uncertain command completion.
				if _, err := client.GetVMAgentInfo(context.Background(), "migrated", 105); err != nil {
					t.Fatalf("completed reading incorrectly fenced metadata: %v", err)
				}
				healthy.Store(true)
				if next, err := client.GetVMFSInfo(context.Background(), "migrated", 105); err != nil || len(next) != 0 || commands.Load() != 3 || configs.Load() != 6 {
					t.Fatal("ordinary complete inventory did not resume with unchanged lock checks")
				}
			})
		}
	}
}

// Measure the unchanged public reader, including guarded HTTP and parsing, on
// an identical small valid inventory and a large rejected inventory. Allocation
// observations are comparative evidence, not a timing/CI performance threshold.
func BenchmarkGuestFilesystemReplyCost(b *testing.B) {
	prior, priorLevel := log.Logger, zerolog.GlobalLevel()
	log.Logger = zerolog.New(io.Discard).Level(zerolog.Disabled)
	zerolog.SetGlobalLevel(zerolog.Disabled)
	defer func() { log.Logger = prior; zerolog.SetGlobalLevel(priorLevel) }()
	for _, malformed := range []int{0, 4096} {
		b.Run(fmt.Sprintf("malformed-%d", malformed), func(b *testing.B) {
			peer := `{"mountpoint":"/","type":"ext4","total-bytes":1000,"used-bytes":0}`
			payload := `{"data":{"result":[` + peer + "," + strings.Repeat("null,", malformed) + peer + `]}}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/config") {
					fmt.Fprint(w, `{"data":{}}`)
				} else {
					fmt.Fprint(w, payload)
				}
			}))
			defer server.Close()
			client, err := NewClient(ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 15 * time.Second})
			if err != nil {
				b.Fatal(err)
			}
			defer client.httpClient.CloseIdleConnections()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				rows, err := client.GetVMFSInfo(context.Background(), "node", 105)
				if len(rows) != 2 || (err != nil) != (malformed > 0) {
					b.Fatal("reader lost valid peers or counted failure")
				}
			}
		})
	}
}
