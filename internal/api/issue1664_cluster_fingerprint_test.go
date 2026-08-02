package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
	// never match the member's own self-signed certificate.
	baseConfig := proxmox.ClientConfig{
		Host:        server.URL,
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

	isValid, fingerprint, reason := validateNodeAPI(clusterNode, baseConfig)
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
