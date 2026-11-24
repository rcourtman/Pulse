package configapi

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestShouldPreserveExistingAutoRegisterHost(t *testing.T) {
	tests := []struct {
		name           string
		existingHost   string
		candidateHosts []string
		want           bool
	}{
		{
			name:           "existing host outranked in candidate list is replaced",
			existingHost:   "https://pve02:8006",
			candidateHosts: []string{"https://192.168.1.12:8006", "https://pve02:8006"},
			want:           false,
		},
		{
			name:           "existing host matches candidate case-insensitively",
			existingHost:   "https://PVE02:8006",
			candidateHosts: []string{"https://192.168.1.12:8006", "https://pve02:8006"},
			want:           false,
		},
		{
			name:           "admin-managed host absent from candidates is preserved",
			existingHost:   "https://pve02.internal.example.com:8006",
			candidateHosts: []string{"https://192.168.1.12:8006", "https://pve02:8006"},
			want:           true,
		},
		{
			name:           "empty candidate list preserves existing host",
			existingHost:   "https://pve02:8006",
			candidateHosts: nil,
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldPreserveExistingAutoRegisterHost(tt.existingHost, tt.candidateHosts); got != tt.want {
				t.Fatalf("shouldPreserveExistingAutoRegisterHost(%q, %v) = %v, want %v",
					tt.existingHost, tt.candidateHosts, got, tt.want)
			}
		})
	}
}

// A stored name that resolves to the registering host today can resolve
// elsewhere tomorrow. localhost stands in for such a name: it shares resolved
// identity with 127.0.0.1 and is absent from the incoming candidate list.
func TestFindCanonicalAutoRegisterMatchKeepsCredentialWithRegisteringHost(t *testing.T) {
	const (
		incomingHost = "https://127.0.0.1:8006"
		pin          = "AA:BB"
	)
	recorded := func(want string) func(string) bool {
		return func(host string) bool { return host == want }
	}
	tests := []struct {
		name           string
		candidates     []canonicalAutoRegisterCandidate
		fingerprint    string
		recordedHost   func(string) bool
		wantIndex      int
		wantPreserve   bool
		wantContinuity bool
	}{
		{
			name:        "plain HTTP stored name is not matched by resolved identity",
			candidates:  []canonicalAutoRegisterCandidate{{Host: "http://localhost:8006"}},
			fingerprint: pin,
			wantIndex:   -1,
		},
		{
			name:       "stored name is not matched without a captured fingerprint",
			candidates: []canonicalAutoRegisterCandidate{{Host: "https://localhost:8006"}},
			wantIndex:  -1,
		},
		{
			name:        "unpinned agent-created HTTPS name is not matched by resolved identity",
			candidates:  []canonicalAutoRegisterCandidate{{Host: "https://localhost:8006", Source: "agent"}},
			fingerprint: pin,
			wantIndex:   -1,
		},
		{
			name:        "clustered agent-created HTTPS name is not matched by resolved identity",
			candidates:  []canonicalAutoRegisterCandidate{{Host: "https://localhost:8006", Source: "agent", Fingerprint: pin, HasClusterEndpoints: true}},
			fingerprint: pin,
			wantIndex:   -1,
		},
		{
			name:         "admin TLS stored name is preserved under the captured pin",
			candidates:   []canonicalAutoRegisterCandidate{{Host: "https://localhost:8006"}},
			fingerprint:  pin,
			wantIndex:    0,
			wantPreserve: true,
		},
		{
			name:         "address recorded by the caller's install grant is preserved",
			candidates:   []canonicalAutoRegisterCandidate{{Host: "http://localhost:8006"}},
			recordedHost: recorded("http://localhost:8006"),
			wantIndex:    0,
			wantPreserve: true,
		},
		{
			name: "exact host match wins over an earlier resolved identity match",
			candidates: []canonicalAutoRegisterCandidate{
				{Host: "https://localhost:8006"},
				{Host: incomingHost},
			},
			fingerprint: pin,
			wantIndex:   1,
		},
		{
			name: "DHCP continuity match reports continuity flag",
			candidates: []canonicalAutoRegisterCandidate{
				{Host: "https://192.0.2.99:8006", Name: "pve-b", TokenName: "pulse-monitor@pve!pulse-b"},
			},
			fingerprint:    pin,
			wantIndex:      0,
			wantPreserve:   false,
			wantContinuity: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index, preserve, continuity := findCanonicalAutoRegisterMatch("pve", canonicalAutoRegisterIdentity{
				host:           incomingHost,
				serverName:     "pve-b",
				fingerprint:    tt.fingerprint,
				tokenName:      "pulse-monitor@pve!pulse-b",
				candidateHosts: []string{incomingHost},
				recordedHost:   tt.recordedHost,
			}, tt.candidates)
			if index != tt.wantIndex || preserve != tt.wantPreserve || continuity != tt.wantContinuity {
				t.Fatalf("findCanonicalAutoRegisterMatch() = (%d, %v, %v), want (%d, %v, %v)", index, preserve, continuity, tt.wantIndex, tt.wantPreserve, tt.wantContinuity)
			}
		})
	}
}

func TestClusterMemberOverrideIdentity(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "url with port", value: "https://192.168.1.12:8006", want: "192.168.1.12:8006"},
		{name: "bare host defaults port", value: "192.168.1.12", want: "192.168.1.12:8006"},
		{name: "host with port", value: "pve02:9000", want: "pve02:9000"},
		{name: "hostname lowercased and port defaulted", value: "PVE02", want: "pve02:8006"},
		{name: "empty", value: "  ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clusterMemberOverrideIdentity(tt.value); got != tt.want {
				t.Fatalf("clusterMemberOverrideIdentity(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestFindCanonicalAutoRegisterClusterMember(t *testing.T) {
	instances := []config.PVEInstance{
		{
			Name: "clusterA", Host: "https://192.0.2.10:8006", IsCluster: true,
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "pve01", Host: "https://pve01:8006", IP: "192.0.2.10"},
				{NodeName: "pve02", Host: "https://pve02:8006", IP: "192.0.2.11"},
			},
		},
		{
			Name: "clusterB", Host: "https://198.51.100.10:8006", IsCluster: true,
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "alpha", Host: "https://alpha:8006", IP: "198.51.100.11"},
			},
		},
	}

	t.Run("endpoint IP identity match wins", func(t *testing.T) {
		i, j, ok := findCanonicalAutoRegisterClusterMember(instances, "unrelated-name", []string{"https://192.0.2.11:8006"})
		if !ok || i != 0 || j != 1 {
			t.Fatalf("got (%d, %d, %v), want (0, 1, true)", i, j, ok)
		}
	})

	t.Run("unique node-name match", func(t *testing.T) {
		i, j, ok := findCanonicalAutoRegisterClusterMember(instances, "pve02", []string{"https://203.0.113.5:8006"})
		if !ok || i != 0 || j != 1 {
			t.Fatalf("got (%d, %d, %v), want (0, 1, true)", i, j, ok)
		}
	})

	t.Run("fqdn agent hostname matches short corosync name", func(t *testing.T) {
		i, j, ok := findCanonicalAutoRegisterClusterMember(instances, "pve02.internal.example.com", []string{"https://203.0.113.5:8006"})
		if !ok || i != 0 || j != 1 {
			t.Fatalf("got (%d, %d, %v), want (0, 1, true)", i, j, ok)
		}
	})

	t.Run("no match", func(t *testing.T) {
		if _, _, ok := findCanonicalAutoRegisterClusterMember(instances, "unknown", []string{"https://203.0.113.5:8006"}); ok {
			t.Fatal("expected no match")
		}
	})

	t.Run("ambiguous node name across clusters is rejected", func(t *testing.T) {
		ambiguous := append([]config.PVEInstance(nil), instances...)
		ambiguous[1].ClusterEndpoints = []config.ClusterEndpoint{
			{NodeName: "pve02", Host: "https://pve02.other:8006", IP: "198.51.100.12"},
		}
		if _, _, ok := findCanonicalAutoRegisterClusterMember(ambiguous, "pve02", []string{"https://203.0.113.5:8006"}); ok {
			t.Fatal("expected ambiguous name match to be rejected")
		}
	})
}

func TestShouldAdoptClusterMemberAutoRegisterAddress(t *testing.T) {
	tests := []struct {
		name             string
		existingOverride string
		candidateHosts   []string
		want             bool
	}{
		{
			name:             "empty override always adopts",
			existingOverride: "",
			candidateHosts:   nil,
			want:             true,
		},
		{
			name:             "override outranked in candidate list is replaced",
			existingOverride: "192.168.1.12:8006",
			candidateHosts:   []string{"https://192.168.1.50:8006", "https://192.168.1.12:8006"},
			want:             true,
		},
		{
			name:             "portless override matches default-port candidate",
			existingOverride: "192.168.1.12",
			candidateHosts:   []string{"https://192.168.1.50:8006", "https://192.168.1.12:8006"},
			want:             true,
		},
		{
			name:             "admin-managed override absent from candidates is preserved",
			existingOverride: "10.99.0.5:8006",
			candidateHosts:   []string{"https://192.168.1.50:8006", "https://192.168.1.12:8006"},
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldAdoptClusterMemberAutoRegisterAddress(tt.existingOverride, tt.candidateHosts); got != tt.want {
				t.Fatalf("shouldAdoptClusterMemberAutoRegisterAddress(%q, %v) = %v, want %v",
					tt.existingOverride, tt.candidateHosts, got, tt.want)
			}
		})
	}
}
