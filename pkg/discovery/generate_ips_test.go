package discovery

import (
	"net"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/discovery/envdetect"
)

func TestGenerateIPs(t *testing.T) {
	scanner := &Scanner{policy: envdetect.DefaultScanPolicy()}

	_, subnet30, err := net.ParseCIDR("192.168.0.0/30")
	if err != nil {
		t.Fatalf("failed to parse subnet: %v", err)
	}
	ips := scanner.generateIPs(subnet30)
	if len(ips) != 2 || ips[0] != "192.168.0.1" || ips[1] != "192.168.0.2" {
		t.Fatalf("unexpected /30 IPs: %v", ips)
	}

	_, subnet32, err := net.ParseCIDR("10.0.0.5/32")
	if err != nil {
		t.Fatalf("failed to parse /32 subnet: %v", err)
	}
	ips = scanner.generateIPs(subnet32)
	if len(ips) != 1 || ips[0] != "10.0.0.5" {
		t.Fatalf("unexpected /32 IPs: %v", ips)
	}

	_, subnet31, err := net.ParseCIDR("10.0.0.0/31")
	if err != nil {
		t.Fatalf("failed to parse /31 subnet: %v", err)
	}
	ips = scanner.generateIPs(subnet31)
	if len(ips) != 2 || ips[0] != "10.0.0.0" || ips[1] != "10.0.0.1" {
		t.Fatalf("unexpected /31 IPs: %v", ips)
	}
}

func TestGenerateIPsRespectsLimit(t *testing.T) {
	policy := envdetect.DefaultScanPolicy()
	policy.MaxHostsPerScan = 2
	scanner := &Scanner{policy: policy}

	_, subnet29, err := net.ParseCIDR("192.168.1.0/29")
	if err != nil {
		t.Fatalf("failed to parse subnet: %v", err)
	}
	ips := scanner.generateIPs(subnet29)
	if len(ips) != 2 || ips[0] != "192.168.1.1" || ips[1] != "192.168.1.2" {
		t.Fatalf("unexpected limited IPs: %v", ips)
	}
}

func TestGenerateIPsIPv6ReturnsNil(t *testing.T) {
	scanner := &Scanner{policy: envdetect.DefaultScanPolicy()}

	_, subnet6, err := net.ParseCIDR("2001:db8::/64")
	if err != nil {
		t.Fatalf("failed to parse ipv6 subnet: %v", err)
	}
	if ips := scanner.generateIPs(subnet6); ips != nil {
		t.Fatalf("expected nil for ipv6 subnet, got %v", ips)
	}
}
