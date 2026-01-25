package api

import (
	"net"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestFindExistingIPOverride(t *testing.T) {
	endpoints := []config.ClusterEndpoint{
		{NodeName: "node1", IPOverride: "10.0.0.10"},
		{NodeName: "node2", IPOverride: "10.0.0.11"},
	}

	if got := findExistingIPOverride("node2", endpoints); got != "10.0.0.11" {
		t.Fatalf("findExistingIPOverride = %q, want 10.0.0.11", got)
	}
	if got := findExistingIPOverride("missing", endpoints); got != "" {
		t.Fatalf("findExistingIPOverride = %q, want empty", got)
	}
}

func TestExtractIPFromHost(t *testing.T) {
	ip := extractIPFromHost("https://10.1.1.5:8006")
	if ip == nil || !ip.Equal(net.ParseIP("10.1.1.5")) {
		t.Fatalf("extractIPFromHost returned %v, want 10.1.1.5", ip)
	}

	ip = extractIPFromHost("10.2.3.4")
	if ip == nil || !ip.Equal(net.ParseIP("10.2.3.4")) {
		t.Fatalf("extractIPFromHost returned %v, want 10.2.3.4", ip)
	}
}

func TestIPsOnSameNetwork(t *testing.T) {
	if !ipsOnSameNetwork(net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.50")) {
		t.Fatalf("expected 10.0.0.1 and 10.0.0.50 to match")
	}
	if ipsOnSameNetwork(net.ParseIP("10.0.0.1"), net.ParseIP("10.1.0.1")) {
		t.Fatalf("expected 10.0.0.1 and 10.1.0.1 to differ")
	}

	ipv6a := net.ParseIP("2001:db8::1")
	ipv6b := net.ParseIP("2001:db8::2")
	if !ipsOnSameNetwork(ipv6a, ipv6b) {
		t.Fatalf("expected IPv6 addresses to match")
	}
}

func TestFindPreferredIP(t *testing.T) {
	interfaces := []proxmox.NodeNetworkInterface{
		{Active: 0, Address: "10.0.0.10"},
		{Active: 1, Address: "10.0.0.11"},
		{Active: 1, Address: "10.0.1.10"},
	}

	ref := net.ParseIP("10.0.0.50")
	if got := findPreferredIP(interfaces, ref); got != "10.0.0.11" {
		t.Fatalf("findPreferredIP = %q, want 10.0.0.11", got)
	}

	if got := findPreferredIP(nil, ref); got != "" {
		t.Fatalf("findPreferredIP = %q, want empty", got)
	}
}
