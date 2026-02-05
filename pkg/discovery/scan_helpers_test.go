package discovery

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/discovery/envdetect"
)

func TestCollectExtraTargets(t *testing.T) {
	scanner := &Scanner{policy: envdetect.DefaultScanPolicy()}
	seen := map[string]struct{}{
		"10.0.0.2": {},
	}

	profile := &envdetect.EnvironmentProfile{
		ExtraTargets: []net.IP{
			net.ParseIP("10.0.0.1"),
			net.ParseIP("10.0.0.2"),
			net.ParseIP("2001:db8::1"),
			nil,
		},
	}

	targets := scanner.collectExtraTargets(profile, seen)
	if len(targets) != 1 || targets[0] != "10.0.0.1" {
		t.Fatalf("unexpected targets: %v", targets)
	}
	if _, ok := seen["10.0.0.1"]; !ok {
		t.Fatalf("expected seen to include new target")
	}
}

func TestExpandPhaseIPs(t *testing.T) {
	scanner := &Scanner{policy: envdetect.DefaultScanPolicy()}
	seen := map[string]struct{}{
		"192.168.1.1": {},
	}

	_, subnet30, err := net.ParseCIDR("192.168.1.0/30")
	if err != nil {
		t.Fatalf("failed to parse subnet: %v", err)
	}
	_, subnet6, err := net.ParseCIDR("2001:db8::/64")
	if err != nil {
		t.Fatalf("failed to parse ipv6 subnet: %v", err)
	}

	targets, count := scanner.expandPhaseIPs(envdetect.SubnetPhase{
		Subnets: []net.IPNet{*subnet30, *subnet6},
	}, seen)
	if count != 2 {
		t.Fatalf("expected 2 subnets counted, got %d", count)
	}
	if len(targets) != 1 || targets[0] != "192.168.1.2" {
		t.Fatalf("unexpected targets: %v", targets)
	}
}

func TestShouldSkipPhase(t *testing.T) {
	policy := envdetect.DefaultScanPolicy()
	policy.DialTimeout = time.Second
	scanner := &Scanner{policy: policy}

	ctxShort, cancel := context.WithDeadline(context.Background(), time.Now().Add(500*time.Millisecond))
	defer cancel()

	phaseLowConfidence := envdetect.SubnetPhase{Name: "low", Confidence: 0.2}
	if !scanner.shouldSkipPhase(ctxShort, phaseLowConfidence) {
		t.Fatalf("expected phase to be skipped with short deadline")
	}

	phaseHighConfidence := envdetect.SubnetPhase{Name: "high", Confidence: 0.8}
	if scanner.shouldSkipPhase(ctxShort, phaseHighConfidence) {
		t.Fatalf("expected high confidence phase to run")
	}

	if scanner.shouldSkipPhase(context.Background(), phaseLowConfidence) {
		t.Fatalf("expected phase to run without deadline")
	}
}
