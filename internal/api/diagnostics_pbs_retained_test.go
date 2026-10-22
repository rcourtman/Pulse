package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

// Exercise the lossy monitor boundary, not only a typed live-probe error.
// The real aggregator stores Error() text; JSON cannot retain HTTPStatus's type.
func retainedPBSDiagnostics(t *testing.T, message string) DiagnosticsInfo {
	t.Helper()
	now := time.Date(2026, 10, 20, 5, 0, 0, 0, time.UTC)
	previousSuccess := now.Add(-time.Minute)
	connections := buildConnections(aggregatorInputs{
		pbsInstances: []config.PBSInstance{{Name: "pbs-retained-fixture", Host: "https://pbs.fixture.invalid:8007"}},
		instanceHealth: map[string]monitoring.InstanceHealth{
			"pbs::pbs-retained-fixture": {PollStatus: monitoring.InstancePollStatus{
				LastSuccess: &previousSuccess,
				LastError:   &monitoring.ErrorDetail{At: now, Message: message},
			}},
		},
		now: now,
	})
	if len(connections) != 1 {
		t.Fatalf("connection count = %d, want 1", len(connections))
	}
	stored, err := json.Marshal(connections[0])
	if err != nil {
		t.Fatal(err)
	}
	var connection Connection
	if err := json.Unmarshal(stored, &connection); err != nil {
		t.Fatal(err)
	}
	if connection.LastError == nil || connection.LastError.Message != message {
		t.Fatal("serialized connection lost its outstanding error")
	}
	diag := EmptyDiagnosticsInfo()
	diag.PBS = []PBSDiagnostic{{ID: "pbs-retained-fixture", Name: "PBS fixture", Host: "https://pbs.fixture.invalid:8007"}}
	applyPBSConnectionDiagnostic(&diag.PBS[0], connection)
	appendDiagnosticsAPIRejections(&diag)
	if diag.PBS[0].Connected || diag.PBS[0].State == ConnectionStateActive || diag.PBS[0].LastSeen == nil || !diag.PBS[0].LastSeen.Equal(previousSuccess) {
		t.Fatal("outstanding failure became healthy or lost the prior poll timestamp")
	}
	return diag
}

func TestDiagnosticsPBSRetainedHTTPFailures(t *testing.T) {
	for _, operation := range []string{"api", "auth"} {
		for _, status := range []int{401, 403, 429, 503} {
			for _, wrapped := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s-%d/wrapped-%t", operation, status, wrapped), func(t *testing.T) {
					var requests atomic.Int32
					server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests.Add(1)
						wantPath := "/api2/json/version"
						if operation == "auth" {
							wantPath = "/api2/json/access/ticket"
						}
						if r.URL.Path != wantPath {
							t.Errorf("unexpected provider path %q", r.URL.Path)
						}
						w.WriteHeader(status)
						io.WriteString(w, "synthetic-private-response: API error 401: permission denied; x509: fixture; connection timed out; fixture-password; fixture-token")
					}))
					defer server.Close()
					pin := sha256.Sum256(server.Certificate().Raw)
					cfg := pbs.ClientConfig{Host: server.URL, VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:])}
					var err error
					if operation == "api" {
						cfg.TokenName, cfg.TokenValue = "fixture@pbs!reader", "fixture-token"
						client, initErr := pbs.NewClient(cfg)
						if initErr != nil {
							t.Fatal(initErr)
						}
						_, err = client.GetVersion(context.Background())
					} else {
						cfg.User, cfg.Password = "fixture@pbs", "fixture-password"
						_, err = pbs.NewClient(cfg)
					}
					if err == nil || requests.Load() != 1 {
						t.Fatal("provider refusal was accepted or replayed")
					}
					if wrapped {
						err = fmt.Errorf("poll failed: version read: %w", err)
					}
					if got, ok := pbs.HTTPStatus(err); !ok || got != status {
						t.Fatal("actual PBS producer lost its typed HTTP status")
					}
					diag := retainedPBSDiagnostics(t, err.Error())
					want := classifyDiagnosticsConnectionFailure(err, "probe failed")
					wantKind := map[int]string{401: "proxmox_auth_rejected", 403: "proxmox_permission_denied", 429: "proxmox_api_error", 503: "proxmox_api_error"}[status]
					if want.kind != wantKind {
						t.Fatalf("typed producer kind = %q, want %q", want.kind, wantKind)
					}
					got := diag.PBS[0]
					if got.ErrorKind != want.kind || got.Error != want.message || got.Troubleshooting != want.troubleshooting {
						t.Errorf("serialized kind = %q, want %q and identical bounded guidance", got.ErrorKind, want.kind)
					}
					wantError := "PBS connection 1 monitor: " + want.message + ". " + want.troubleshooting
					if len(diag.Errors) != 1 || diag.Errors[0] != wantError {
						t.Errorf("Errors Detected count = %d, want one correctly scoped monitor rejection", len(diag.Errors))
					}
					wire, marshalErr := json.Marshal(diag)
					if marshalErr != nil {
						t.Fatal(marshalErr)
					}
					for _, private := range []string{"synthetic-private-response", "fixture-password", "fixture-token", "x509: fixture", "connection timed out"} {
						if strings.Contains(string(wire), private) {
							t.Errorf("diagnostics leaked synthetic provider body marker %q", private)
						}
					}
					if len(err.Error()) > 256 {
						t.Error("provider error is not bounded")
					}
					if !t.Failed() {
						t.Logf("PBS_RETAINED_DIAGNOSTIC %s", wire)
					}
				})
			}
		}
	}
}

func TestDiagnosticsPBSRetainedLegacyMarkers(t *testing.T) {
	for _, tc := range []struct{ message, kind string }{
		{"authentication error: API error 403 (Forbidden): provider says authentication failed", "proxmox_permission_denied"},
		{"poll failed: authentication error: API error 503: API error 403: permission denied", "proxmox_api_error"},
		{"authentication failed (status 403): API error 401: private-fixture-body", "proxmox_permission_denied"},
		{"poll failed: authentication failed (status 429): API error 401: private-fixture-body", "proxmox_api_error"},
		{"API error 401 (Unauthorized): private-fixture-body", "proxmox_auth_rejected"},
		{"API error 503: connection timed out; x509: private-fixture-body", "proxmox_api_error"},
	} {
		t.Run(tc.message, func(t *testing.T) {
			diag := retainedPBSDiagnostics(t, tc.message)
			if diag.PBS[0].ErrorKind != tc.kind || len(diag.Errors) != 1 {
				t.Fatalf("legacy kind = %q, errors = %d; want %q and one monitor rejection", diag.PBS[0].ErrorKind, len(diag.Errors), tc.kind)
			}
			if strings.Contains(strings.Join(diag.Errors, " "), "private-fixture-body") {
				t.Fatal("shareable Errors Detected introduced provider text")
			}
		})
	}
}
