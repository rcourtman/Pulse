package securityutil

import (
	"net"
	"testing"
)

// This file raises branch coverage for the SSRF guard helpers in httpurl.go:
// joinURLPath, IsLocalNetworkHost, isLocalNetworkIP, and
// isCarrierGradeNATIPv4. Every case targets an arm the existing suite reaches
// only indirectly (or not at all): empty inputs, every network class the
// classifier must recognise, and the CGNAT range boundaries. No DNS, daemon,
// or network is exercised.

func TestBranchcov0724pmJoinURLPath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		relPath  string
		want     string
	}{
		// Branch: relative trims to empty (no append) AND joined is empty
		// (path.Join("") == "") -> default arm, no leading slash -> "/" prepended.
		{name: "empty base empty path", basePath: "", relPath: "", want: "/"},
		{name: "empty base slash only path", basePath: "", relPath: "/", want: "/"},

		// Branch: switch case "/" -> return "".
		{name: "root base slash only path", basePath: "/", relPath: "/", want: ""},
		{name: "root base empty path", basePath: "/", relPath: "", want: ""},
		// Parent references collapse the join to "/".
		{name: "parent ref collapse to root", basePath: "/api", relPath: "/..", want: ""},

		// Branch: switch case "." -> return "" (basePath that Clean()s to ".").
		{name: "dot base empty path", basePath: ".", relPath: "", want: ""},
		{name: "dot base slash only path", basePath: ".", relPath: "/", want: ""},

		// Branch: default arm, joined already has leading slash (HasPrefix true).
		{name: "base keeps nonempty path", basePath: "/api", relPath: "/users", want: "/api/users"},
		{name: "trailing slash base normalised", basePath: "/api/", relPath: "/v1", want: "/api/v1"},
		// relative trims to empty so the existing base path is returned verbatim.
		{name: "nonempty base slash only path", basePath: "/api", relPath: "/", want: "/api"},

		// Branch: default arm, joined has NO leading slash (HasPrefix false).
		{name: "empty base with content path", basePath: "", relPath: "/users", want: "/users"},
		{name: "relative base no slash", basePath: "api", relPath: "/users", want: "/api/users"},

		// Branch: an encoded segment is joined verbatim (no decoding here).
		{name: "encoded segment preserved", basePath: "/api", relPath: "/us%20ers", want: "/api/us%20ers"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinURLPath(tt.basePath, tt.relPath)
			if got != tt.want {
				t.Fatalf("joinURLPath(%q, %q) = %q, want %q", tt.basePath, tt.relPath, got, tt.want)
			}
		})
	}
}

func TestBranchcov0724pmIsLocalNetworkHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want bool
	}{
		// Branch: normalised to empty -> return false.
		{name: "empty string", host: "", want: false},
		{name: "whitespace only", host: "   ", want: false},
		{name: "brackets only", host: "[]", want: false},

		// Branch: IsLoopbackHost true -> return true.
		{name: "localhost", host: "localhost", want: true},
		{name: "localhost upper", host: "LOCALHOST", want: true},
		{name: "localhost subdomain", host: "foo.localhost", want: true},
		{name: "ipv4 loopback literal", host: "127.0.0.1", want: true},
		{name: "ipv6 loopback literal", host: "::1", want: true},
		{name: "ipv6 loopback bracketed", host: "[::1]", want: true},
		// Trailing dot is stripped before classification.
		{name: "localhost trailing dot", host: "localhost.", want: true},

		// Branch: IP literal -> isLocalNetworkIP true (private / link-local).
		{name: "ipv4 private 10/8", host: "10.0.0.1", want: true},
		{name: "ipv4 private 172.16/12", host: "172.16.5.4", want: true},
		{name: "ipv4 private 192.168/16", host: "192.168.1.1", want: true},
		{name: "ipv4 link local", host: "169.254.1.1", want: true},
		{name: "ipv6 link local", host: "fe80::1", want: true},
		{name: "ipv6 unique local", host: "fc00::1", want: true},
		{name: "ipv4 cgnat first address", host: "100.64.0.0", want: true},
		{name: "ipv4 cgnat last address", host: "100.127.255.255", want: true},

		// Branch: single-label name (no dot) -> return true.
		{name: "single label name", host: "myhost", want: true},
		{name: "single label upper", host: "MYHOST", want: true},

		// Branch: operator-local suffix match -> return true.
		{name: "dot-local suffix", host: "nas.local", want: true},
		{name: "dot-lan suffix", host: "router.lan", want: true},
		{name: "dot-home suffix", host: "server.home", want: true},
		{name: "dot-home-arpa suffix", host: "server.home.arpa", want: true},
		{name: "dot-internal suffix", host: "host.internal", want: true},

		// Branch: fall-through public hostname -> return false.
		{name: "public hostname", host: "example.com", want: false},
		{name: "multi-label public hostname", host: "api.example.com", want: false},
		{name: "public ipv4 literal", host: "203.0.113.1", want: false},
		{name: "public ipv6 literal", host: "2001:db8::1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLocalNetworkHost(tt.host)
			if got != tt.want {
				t.Fatalf("IsLocalNetworkHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestBranchcov0724pmIsLocalNetworkIP(t *testing.T) {
	tests := []struct {
		name string
		ip   net.IP
		want bool
	}{
		// Branch: nil guard -> false.
		{name: "nil ip", ip: nil, want: false},

		// Branch: loopback.
		{name: "ipv4 loopback", ip: net.ParseIP("127.0.0.1"), want: true},
		{name: "ipv6 loopback", ip: net.ParseIP("::1"), want: true},

		// Branch: private (RFC1918 + ULA).
		{name: "ipv4 private 10", ip: net.ParseIP("10.1.2.3"), want: true},
		{name: "ipv4 private 172.16", ip: net.ParseIP("172.16.0.1"), want: true},
		{name: "ipv4 private 192.168", ip: net.ParseIP("192.168.50.50"), want: true},
		{name: "ipv6 unique local", ip: net.ParseIP("fd00::1"), want: true},

		// Branch: link-local unicast.
		{name: "ipv4 link local", ip: net.ParseIP("169.254.3.4"), want: true},
		{name: "ipv6 link local", ip: net.ParseIP("fe80::abcd"), want: true},

		// Branch: CGNAT 100.64.0.0/10.
		{name: "cgnat first octet boundary", ip: net.ParseIP("100.64.0.1"), want: true},
		{name: "cgnat last octet boundary", ip: net.ParseIP("100.127.255.255"), want: true},

		// Branch: all classifiers false (public).
		{name: "public ipv4", ip: net.ParseIP("203.0.113.10"), want: false},
		{name: "public ipv6", ip: net.ParseIP("2001:db8::1"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLocalNetworkIP(tt.ip)
			if got != tt.want {
				t.Fatalf("isLocalNetworkIP(%v) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestBranchcov0724pmIsCarrierGradeNATIPv4(t *testing.T) {
	tests := []struct {
		name string
		ip   net.IP
		want bool
	}{
		// Branch: To4() == nil (IPv6 or nil) -> false.
		{name: "nil ip", ip: nil, want: false},
		{name: "ipv6 loopback not v4", ip: net.ParseIP("::1"), want: false},
		{name: "ipv6 public not v4", ip: net.ParseIP("2001:db8::1"), want: false},

		// Branch: inside 100.64.0.0/10 -> true, both boundaries.
		{name: "first address of range", ip: net.ParseIP("100.64.0.0"), want: true},
		{name: "first host of range", ip: net.ParseIP("100.64.0.1"), want: true},
		{name: "last address of range", ip: net.ParseIP("100.127.255.255"), want: true},

		// Branch: just outside the range -> false.
		{name: "just below range second octet", ip: net.ParseIP("100.63.255.255"), want: false},
		{name: "just above range second octet", ip: net.ParseIP("100.128.0.0"), want: false},

		// Branch: wrong first octet -> false.
		{name: "first octet 99", ip: net.ParseIP("99.64.0.1"), want: false},
		{name: "first octet 101", ip: net.ParseIP("101.64.0.1"), want: false},

		// Branch: ordinary public address -> false.
		{name: "public ipv4", ip: net.ParseIP("203.0.113.1"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCarrierGradeNATIPv4(tt.ip)
			if got != tt.want {
				t.Fatalf("isCarrierGradeNATIPv4(%v) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}
