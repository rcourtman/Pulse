package pbs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

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
			client, err := NewClient(ClientConfig{Host: host, TokenName: "fixture@pbs!pulse", TokenValue: "synthetic-token"})
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
		client, err := NewClient(ClientConfig{Host: host, TokenName: "fixture@pbs!pulse", TokenValue: "synthetic-token"})
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

// Initial authentication must obey the same caller lifetime as ordinary reads.
func TestClientConstructionHonoursContext(t *testing.T) {
	testClientConstructionHonoursContext(t)
}

func TestClientCancelledAuthResponseOwnsErrorAndSession(t *testing.T) {
	testClientCancelledAuthResponseOwnsErrorAndSession(t)
}

// Non-parallel: the production package logger is shared. All response bytes and
// credentials here are synthetic; no PBS, listener or guest operation is used.
func TestPBSStatusDiagnostics(t *testing.T) {
	priorLogger, priorLevel := log.Logger, zerolog.GlobalLevel()
	defer func() { log.Logger = priorLogger; zerolog.SetGlobalLevel(priorLevel) }()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	for _, level := range []zerolog.Level{zerolog.InfoLevel, zerolog.DebugLevel} {
		for _, phase := range []string{"node", "rrd", "status", "gc", "malformed"} {
			for _, size := range []int{1024, 1 << 20} {
				t.Run(fmt.Sprintf("%s/%s/bytes=%d", level, phase, size), func(t *testing.T) {
					const marker = "synthetic-provider-extension-"
					private := marker + strings.Repeat("x", size)
					payload := map[string]any{"total": 1000, "used": 600, "avail": 400,
						"total-space": 900, "used-space": 500, "avail-space": 400,
						"status": "available", "gc-status": "idle", "extension": map[string]any{"unknown": private}}
					if phase == "node" {
						payload = map[string]any{"cpu": 0.25, "memory": map[string]int{"used": 12, "total": 48}, "extension": private}
					} else if phase == "status" {
						payload["deduplication-factor"] = 4
					}
					bodyBytes, err := json.Marshal(map[string]any{"data": payload})
					if err != nil {
						t.Fatal(err)
					}
					if phase == "malformed" {
						bodyBytes = bodyBytes[:len(bodyBytes)-1]
					}
					var captured bytes.Buffer
					log.Logger = zerolog.New(&captured).Level(level)
					client, err := NewClient(ClientConfig{Host: "https://pbs.invalid", TokenName: "fixture@pbs!pulse", TokenValue: "fixture-token"})
					if err != nil {
						t.Fatal(err)
					}
					var paths []string
					closes := 0
					client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
						if r.Method != http.MethodGet || r.Header.Get("Authorization") != "PBSAPIToken=fixture@pbs!pulse:fixture-token" {
							t.Error("diagnostic change altered read-only token authentication")
						}
						paths = append(paths, r.URL.RequestURI())
						body := string(bodyBytes)
						switch r.URL.Path {
						case "/api2/json/nodes/localhost/status":
							if phase != "node" {
								t.Error("unexpected node read")
							}
						case "/api2/json/admin/datastore":
							body = `{"data":[{"store":"chosen"},{"store":"excluded"}]}`
						case "/api2/json/admin/datastore/chosen/rrd":
							body = `{"data":[{"time":1234,"dedup_factor":2.5}]}`
							if phase == "gc" {
								body = `{"data":[]}`
							}
						case "/api2/json/admin/datastore/chosen/status":
						case "/api2/json/admin/datastore/chosen/gc":
							if phase != "gc" {
								t.Error("unexpected GC fallback")
							}
							body = `{"data":{"index-data-bytes":900,"disk-bytes":300}}`
						default:
							t.Error("unexpected request or excluded datastore read")
						}
						return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header),
							Body: &statusDiagnosticBody{Reader: strings.NewReader(body), closes: &closes}, Request: r}, nil
					})
					expectedPaths := []string{"/api2/json/nodes/localhost/status"}
					if phase == "node" {
						status, err := client.GetNodeStatus(context.Background())
						if err != nil || status == nil || status.CPU != 0.25 || status.Memory.Used != 12 || status.Memory.Total != 48 {
							t.Error("status diagnostics changed returned node measurements")
						}
					} else {
						expectedPaths = []string{"/api2/json/admin/datastore", "/api2/json/admin/datastore/chosen/rrd?cf=AVERAGE&timeframe=hour", "/api2/json/admin/datastore/chosen/status"}
						if phase == "gc" {
							expectedPaths = append(expectedPaths, "/api2/json/admin/datastore/chosen/gc")
						}
						stores, err := client.GetDatastoresFiltered(context.Background(), func(name string) bool { return name == "chosen" })
						if err != nil || len(stores) != 1 {
							t.Fatalf("datastore reader failed: %v", err)
						}
						if phase == "malformed" {
							if stores[0].Status != "unavailable" || !strings.Contains(stores[0].Error, "Failed to parse status:") {
								t.Error("parse failure became available metrics")
							}
						} else {
							dedup := 2.5
							if phase == "status" {
								dedup = 4
							} else if phase == "gc" {
								dedup = 3
							}
							want := Datastore{Store: "chosen", Total: 1000, Used: 600, Avail: 400, TotalSpace: 900, UsedSpace: 500, AvailSpace: 400, Status: "available", GCStatus: "idle", DeduplicationFactor: dedup}
							if !reflect.DeepEqual(stores[0], want) {
								t.Error("diagnostic change altered datastore fields or dedup precedence")
							}
						}
					}
					if !reflect.DeepEqual(paths, expectedPaths) || closes != len(paths) {
						t.Errorf("request/cleanup changed: reads=%d closes=%d", len(paths), closes)
					}
					lines := []string{}
					if captured.Len() > 0 {
						lines = strings.Split(strings.TrimSuffix(captured.String(), "\n"), "\n")
					}
					leaked := strings.Contains(captured.String(), marker)
					t.Logf("PBS_STATUS_DIAGNOSTICS level=%s phase=%s response_bytes=%d log_bytes=%d events=%d provider_extension_logged=%t", level, phase, len(bodyBytes), captured.Len(), len(lines), leaked)
					if leaked {
						t.Error("status diagnostic retained an unmodelled provider field")
					}
					if captured.Len() > 4096 {
						t.Error("status diagnostic size scales with raw provider payload")
					}
					for _, line := range lines {
						var event map[string]any
						if err := json.Unmarshal([]byte(line), &event); err != nil {
							t.Fatal("diagnostic is not a complete JSON event")
						}
						if _, ok := event["response"]; ok {
							t.Error("raw response diagnostic remains")
						}
						if _, ok := event["all_fields"]; ok {
							t.Error("unbounded datastore diagnostic remains")
						}
					}
					if level == zerolog.InfoLevel {
						want := 0
						if phase == "malformed" {
							want = 1
						}
						if len(lines) != want {
							t.Errorf("routine polling produced %d info events; want %d parse failures only", len(lines), want)
						}
					}
					if phase == "malformed" && (!strings.Contains(captured.String(), "Failed to parse datastore status") || !strings.Contains(captured.String(), `"store":"chosen"`)) {
						t.Error("parse failure lost its actionable diagnostic")
					}
					// Bound actual producer frames for the consuming UI, without ever
					// printing the synthetic oversized body or failed baseline dumps.
					if size == 1024 && !leaked && captured.Len() <= 4096 && !t.Failed() {
						frame, err := json.Marshal(map[string]any{"phase": phase, "level": level.String(), "lines": lines, "responseBytes": len(bodyBytes), "logBytes": captured.Len(), "requestPaths": paths, "closedBodies": closes})
						if err != nil {
							t.Fatal(err)
						}
						t.Logf("PBS_STATUS_LOG_FRAME %s", base64.StdEncoding.EncodeToString(frame))
					}
				})
			}
		}
	}
}

type statusDiagnosticBody struct {
	io.Reader
	closes *int
}

func (b *statusDiagnosticBody) Close() error { *b.closes++; return nil }

// Failure bodies, unlike typed measurements, are never diagnostic content.
func TestPBSProviderFailureDiagnostics(t *testing.T) { testPBSFailureDiagnostics(t) }
func TestPBSTokenFailureDiagnostics(t *testing.T)    { testPBSTokenFailureDiagnostics(t) }

// Token construction is socket-free. URL parsing must never reinterpret an
// explicit scheme as an HTTPS hostname before an authenticated request is built.
func TestClientEndpointAuthorityNormalization(t *testing.T) {
	for _, tc := range []struct{ name, host, want string }{
		{"upper-https", "HTTPS://pve.example.invalid:8443/tenant", "https://pve.example.invalid:8443/tenant/api2/json"},
		{"mixed-https", "hTtPs://pve.example.invalid:8443/tenant/", "https://pve.example.invalid:8443/tenant/api2/json"},
		{"upper-http", "HTTP://pve.example.invalid:8080/tenant", "http://pve.example.invalid:8080/tenant/api2/json"},
		{"trimmed-bare", "  pve.example.invalid:8443/tenant  ", "https://pve.example.invalid:8443/tenant/api2/json"},
		{"ipv6", "HTTPS://[2001:db8::1]:8443/tenant", "https://[2001:db8::1]:8443/tenant/api2/json"},
		{"zoned-ipv6", "HTTPS://[fe80::1%25ZoneA]:8443/tenant", "https://[fe80::1%25ZoneA]:8443/tenant/api2/json"},
		{"ordinary", "https://pve.example.invalid:8443/tenant", "https://pve.example.invalid:8443/tenant/api2/json"},
	} {
		for _, trust := range []struct {
			name, pin string
			verify    bool
		}{{"ca", "", true}, {"pin", strings.Repeat("a", 64), true}, {"explicit-opt-out", "", false}} {
			t.Run(tc.name+"/"+trust.name, func(t *testing.T) {
				cfg := ClientConfig{Host: tc.host, TokenName: "fixture@pam!pulse", TokenValue: "synthetic-token", VerifySSL: trust.verify, Fingerprint: trust.pin, Timeout: 3 * time.Second}
				client, err := NewClient(cfg)
				if err != nil {
					t.Fatal(err)
				}
				defer client.httpClient.CloseIdleConnections()
				if client.baseURL != tc.want {
					t.Errorf("request destination = %q, want %q", client.baseURL, tc.want)
				}
				if client.config.VerifySSL != cfg.VerifySSL || client.config.Fingerprint != cfg.Fingerprint || client.auth.tokenValue != cfg.TokenValue || client.httpClient.Timeout != cfg.Timeout {
					t.Error("URL normalization changed trust, credentials or request budget")
				}
				transport := client.httpClient.Transport.(*http.Transport)
				if transport.TLSClientConfig.InsecureSkipVerify != (trust.pin != "" || !trust.verify) || (trust.pin != "" && transport.TLSClientConfig.VerifyPeerCertificate == nil) || client.httpClient.CheckRedirect == nil {
					t.Error("URL normalization changed TLS or redirect policy")
				}
			})
		}
	}
}

func TestClientEndpointExplicitSchemeRefused(t *testing.T) {
	for _, host := range []string{
		"ftp://pve.example.invalid:8006/tenant", "ws://pve.example.invalid:8006/tenant", "wss://pve.example.invalid:8006/tenant",
		"file://pve.example.invalid/tenant", "ssh://pve.example.invalid:22/tenant", "https://fixture:synthetic-secret@pve.example.invalid",
		"HTTPS://fixture:synthetic-secret@pve.example.invalid", "HTTPS://pve.example.invalid?token=synthetic-secret", "HTTPS://pve.example.invalid#synthetic-secret",
	} {
		t.Run(host, func(t *testing.T) {
			client, err := NewClient(ClientConfig{Host: host, TokenName: "fixture@pam!pulse", TokenValue: "synthetic-token", VerifySSL: true})
			if err == nil || client != nil {
				if client != nil {
					client.httpClient.CloseIdleConnections()
				}
				t.Fatal("invalid endpoint became an authenticated client")
			}
			if strings.Contains(err.Error(), "synthetic-secret") || strings.Contains(err.Error(), "synthetic-token") {
				t.Error("rejected URL diagnostic disclosed fixture credentials")
			}
		})
	}
}
