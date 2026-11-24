package configapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Issue #1664: a node joining a PVE cluster after setup could never be
// trusted when the primary was fingerprint-pinned. The pinned fingerprint
// mismatch surfaces from the first API call, not from client construction,
// so validateNodeAPI judged the member "not a Proxmox node" and discarded
// the fingerprint it had already captured.
func TestValidateNodeAPI_PinnedPrimaryFingerprintMismatch(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/nodes" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"node":"member-b","status":"online"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// The base config carries the primary's pinned fingerprint, which can
	// never match the member's own self-signed certificate. The primary is a
	// different endpoint: its own address is never trusted on first use.
	baseConfig := proxmox.ClientConfig{
		Host:        strings.Replace(server.URL, "127.0.0.1", "localhost", 1),
		TokenName:   "root@pam!pulse",
		TokenValue:  "secret",
		Fingerprint: "DE:AD:BE:EF:DE:AD:BE:EF:DE:AD:BE:EF:DE:AD:BE:EF:DE:AD:BE:EF:DE:AD:BE:EF:DE:AD:BE:EF:DE:AD:BE:EF",
	}
	clusterNode := proxmox.ClusterStatus{
		Name:   "member-b",
		ID:     "node/member-b",
		Online: 1,
		IP:     "127.0.0.1",
	}

	isValid, fingerprint, reason := validateNodeAPI(context.Background(), clusterNode, baseConfig, "")
	if !isValid {
		t.Fatalf("expected member with its own certificate to validate despite primary pin, got invalid (reason %q)", reason)
	}
	if fingerprint == "" {
		t.Fatal("expected captured member fingerprint to be returned for TOFU persistence")
	}
	if reason != "" {
		t.Fatalf("expected empty failure reason on success, got %q", reason)
	}
}

// First use is the only case in which discovery may trust the certificate a
// member presents. A member that fails CA validation, its saved pin, or the
// primary's pin on the primary's own endpoint must be refused before the
// cluster token or password is sent, and must not hand back a pin to save.
func TestValidateNodeAPI_FailedVerificationNeverReceivesCredentials(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	otherPrimary := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	otherPin := strings.Repeat("ab", 32)
	clusterNode := proxmox.ClusterStatus{Name: "member-b", ID: "node/member-b", Online: 1, IP: "127.0.0.1"}

	for _, tc := range []struct {
		name             string
		primary          proxmox.ClientConfig
		savedFingerprint string
	}{
		{"ca-validation", proxmox.ClientConfig{Host: otherPrimary, VerifySSL: true}, ""},
		{"saved-pin-under-ca-validation", proxmox.ClientConfig{Host: otherPrimary, VerifySSL: true}, otherPin},
		{"saved-pin-under-pinned-primary", proxmox.ClientConfig{Host: otherPrimary, Fingerprint: strings.Repeat("cd", 32)}, otherPin},
		{"saved-pin-without-verification", proxmox.ClientConfig{Host: otherPrimary}, otherPin},
		{"primary-endpoint-under-its-own-pin", proxmox.ClientConfig{Host: server.URL, Fingerprint: otherPin}, ""},
	} {
		for _, auth := range []struct {
			name  string
			apply func(*proxmox.ClientConfig)
		}{
			{"token", func(cfg *proxmox.ClientConfig) { cfg.TokenName, cfg.TokenValue = "root@pam!pulse", "secret" }},
			{"password", func(cfg *proxmox.ClientConfig) { cfg.User, cfg.Password = "root@pam", "secret" }},
		} {
			t.Run(tc.name+"/"+auth.name, func(t *testing.T) {
				requests.Store(0)
				baseConfig := tc.primary
				auth.apply(&baseConfig)

				isValid, fingerprint, reason := validateNodeAPI(context.Background(), clusterNode, baseConfig, tc.savedFingerprint)
				if isValid || fingerprint != "" {
					t.Fatalf("member that failed verification was accepted: valid=%v fingerprint=%q", isValid, fingerprint)
				}
				if reason != "TLS certificate validation failed" {
					t.Fatalf("failure reason = %q, want TLS certificate validation failed", reason)
				}
				if got := requests.Load(); got != 0 {
					t.Fatalf("member that failed verification received %d credentialed request(s)", got)
				}
			})
		}
	}
}

func TestValidateNodeAPI_SavedMemberPinIsTheOnlyTrust(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"node":"member-b","status":"online"}]}`)
	}))
	defer server.Close()
	sum := sha256.Sum256(server.Certificate().Raw)
	savedPin := strings.ToUpper(hex.EncodeToString(sum[:]))

	// Neither CA validation nor the primary's pin would accept this member; its
	// saved pin does, and is handed back unchanged.
	for _, primary := range []proxmox.ClientConfig{
		{VerifySSL: true},
		{Fingerprint: strings.Repeat("cd", 32)},
	} {
		primary.Host = strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
		primary.TokenName, primary.TokenValue = "root@pam!pulse", "secret"
		clusterNode := proxmox.ClusterStatus{Name: "member-b", ID: "node/member-b", Online: 1, IP: "127.0.0.1"}

		isValid, fingerprint, reason := validateNodeAPI(context.Background(), clusterNode, primary, savedPin)
		if !isValid || fingerprint != savedPin || reason != "" {
			t.Fatalf("saved member pin: valid=%v fingerprint=%q reason=%q, want valid with the saved pin", isValid, fingerprint, reason)
		}
	}
}

func TestDetectPVECluster_SavedMemberPinIsValidatedAndKept(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/cluster/status" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[`+
			`{"type":"cluster","name":"production","quorate":1},`+
			`{"type":"node","id":"node/a","name":"pve-a","ip":"127.0.0.1","online":1},`+
			`{"type":"node","id":"node/b","name":"pve-b","ip":"127.0.0.1","online":1}]}`)
	}))
	defer server.Close()

	offered := make(map[string]string)
	validator := func(_ context.Context, node proxmox.ClusterStatus, _ proxmox.ClientConfig, savedFingerprint string) (bool, string, string) {
		offered[node.Name] = savedFingerprint
		return true, "presented-" + node.Name, ""
	}
	_, _, endpoints := detectPVEClusterWithValidator(
		context.Background(),
		proxmox.ClientConfig{Host: server.URL, TokenName: "root@pam!pulse", TokenValue: "secret", Timeout: time.Second},
		"pve-a",
		[]config.ClusterEndpoint{{NodeName: "pve-b", Fingerprint: "saved-pve-b"}},
		validator,
	)

	if offered["pve-a"] != "" || offered["pve-b"] != "saved-pve-b" {
		t.Fatalf("validator was offered %+v, want only pve-b's saved pin", offered)
	}
	firstUse, _ := findExistingClusterEndpoint("pve-a", endpoints)
	known, _ := findExistingClusterEndpoint("pve-b", endpoints)
	if firstUse.Fingerprint != "presented-pve-a" {
		t.Fatalf("first-use member pin = %q, want the validated certificate", firstUse.Fingerprint)
	}
	if known.Fingerprint != "saved-pve-b" {
		t.Fatalf("discovery replaced a saved member pin with %q", known.Fingerprint)
	}
}

func TestClusterEndpointsWithSavedTrustAppliesExplicitOverrides(t *testing.T) {
	instance := config.PVEInstance{
		Name:      "cluster",
		Host:      "https://192.0.2.10:8006",
		IsCluster: true,
		ClusterEndpoints: []config.ClusterEndpoint{
			{NodeName: "pve-a", Host: "https://pve-a:8006", IP: "192.0.2.10", Fingerprint: "pin-a"},
			{NodeName: "pve-b", Host: "https://pve-b:8006", IP: "192.0.2.11"},
		},
		TLSFingerprintOverrides: map[string]string{
			"https://192.0.2.11:8006": "override-pin-b",
		},
	}

	resolved := clusterEndpointsWithSavedTrust(instance, config.DiscoveryConfig{})
	if len(resolved) != 2 {
		t.Fatalf("len(resolved) = %d, want 2", len(resolved))
	}
	if resolved[0].Fingerprint != "pin-a" {
		t.Fatalf("pve-a pin = %q, want pin-a", resolved[0].Fingerprint)
	}
	if resolved[1].Fingerprint != "override-pin-b" {
		t.Fatalf("pve-b pin = %q, want override-pin-b", resolved[1].Fingerprint)
	}
	if instance.ClusterEndpoints[1].Fingerprint != "" {
		t.Fatalf("original ClusterEndpoints mutated: %q", instance.ClusterEndpoints[1].Fingerprint)
	}
}

func TestNodeValidationFailureReason(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("certificate fingerprint mismatch: expected aa, got bb"), "TLS certificate validation failed"},
		{errors.New(`Get "https://b:8006": dial tcp: lookup b: no such host`), "hostname did not resolve from the Pulse server"},
		{errors.New("dial tcp 10.0.0.5:8006: connect: connection refused"), "connection refused"},
		{errors.New("context deadline exceeded"), "connection timed out"},
		{errors.New("500 internal server error"), "not reachable"},
		{nil, ""},
	}
	for _, tc := range cases {
		if got := nodeValidationFailureReason(tc.err); got != tc.want {
			t.Errorf("nodeValidationFailureReason(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}
