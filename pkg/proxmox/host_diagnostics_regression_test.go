package proxmox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
