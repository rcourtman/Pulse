package main

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterPenalizeMetrics(t *testing.T) {
	metrics := NewProxyMetrics("test")
	rl := newRateLimiter(metrics, nil, nil, nil)
	rl.policy.penaltyDuration = 10 * time.Millisecond

	start := time.Now()
	rl.penalize("uid:42", "invalid_json")
	if time.Since(start) < rl.policy.penaltyDuration {
		t.Fatalf("expected penalize to sleep at least %v", rl.policy.penaltyDuration)
	}

	mf, err := metrics.registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	found := false
	for _, fam := range mf {
		if fam.GetName() != "pulse_proxy_limiter_penalties_total" {
			continue
		}
		for _, metric := range fam.GetMetric() {
			if metric.GetCounter().GetValue() == 0 {
				continue
			}
			var reasonLabel, peerLabel string
			for _, label := range metric.GetLabel() {
				switch label.GetName() {
				case "reason":
					reasonLabel = label.GetValue()
				case "peer":
					peerLabel = label.GetValue()
				}
			}
			if reasonLabel == "invalid_json" && peerLabel == "uid:42" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected limiter penalty metric for invalid_json and peer uid:42")
	}
}

func TestIdentifyPeerRangeVsUID(t *testing.T) {
	uidRanges := []idRange{{start: 100000, length: 65536}}
	gidRanges := []idRange{{start: 100000, length: 65536}}
	rl := newRateLimiter(nil, nil, uidRanges, gidRanges)

	containerCred := &peerCredentials{uid: 110000, gid: 110000}
	containerPeer := rl.identifyPeer(containerCred)
	if containerPeer.uidRange == nil {
		t.Fatalf("expected container peer to map to UID range")
	}
	if got := containerPeer.String(); got != "range:100000-165535" {
		t.Fatalf("unexpected container peer label: %s", got)
	}

	hostCred := &peerCredentials{uid: 1000, gid: 1000}
	hostPeer := rl.identifyPeer(hostCred)
	if hostPeer.uidRange != nil {
		t.Fatalf("expected host peer to use UID label")
	}
	if got := hostPeer.String(); got != "uid:1000" {
		t.Fatalf("unexpected host peer label: %s", got)
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := newRateLimiter(nil, nil, nil, nil)
	rl.policy.perPeerConcurrency = 1
	rl.policy.globalConcurrency = 10
	rl.policy.perPeerBurst = 10

	id := peerID{uid: 1000}
	release1, _, allowed1 := rl.allow(id)
	if !allowed1 {
		t.Fatal("expected first request to be allowed")
	}

	// per-peer concurrency hit
	_, reason, allowed2 := rl.allow(id)
	if allowed2 {
		t.Fatal("expected second request to be rejected")
	}
	if reason != "peer_concurrency" {
		t.Errorf("expected reason peer_concurrency, got %s", reason)
	}

	release1()

	// Now allowed again
	_, _, allowed3 := rl.allow(id)
	if !allowed3 {
		t.Fatal("expected third request to be allowed")
	}
}

func TestRateLimiter_GlobalConcurrency(t *testing.T) {
	rl := newRateLimiter(nil, nil, nil, nil)
	rl.globalSem = make(chan struct{}, 1) // Force global limit to 1

	id1 := peerID{uid: 1001}
	id2 := peerID{uid: 1002}

	release1, _, _ := rl.allow(id1)

	_, reason, allowed := rl.allow(id2)
	if allowed {
		t.Fatal("expected id2 to be rejected due to global concurrency")
	}
	if reason != "global_concurrency" {
		t.Errorf("expected global_concurrency, got %s", reason)
	}

	release1()
}

func TestNodeGate(t *testing.T) {
	g := newNodeGate()

	release1 := g.acquire("node1")

	acquired2 := make(chan bool)
	go func() {
		release2 := g.acquire("node1")
		acquired2 <- true
		release2()
	}()

	select {
	case <-acquired2:
		t.Fatal("should not have acquired node1 while held")
	case <-time.After(20 * time.Millisecond):
		// Good
	}

	release1()

	select {
	case <-acquired2:
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Fatal("should have acquired node1 after release")
	}
}

func TestNodeGate_AcquireContext(t *testing.T) {
	g := newNodeGate()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release1, _ := g.acquireContext(ctx, "node1")

	// Try to acquire again with cancelled context
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	_, err := g.acquireContext(ctx2, "node1")
	if err == nil {
		t.Error("expected error for cancelled context")
	}

	release1()
}

func TestRateLimiter_NewLimit(t *testing.T) {
	metrics := NewProxyMetrics("test")
	cfg := &RateLimitConfig{
		PerPeerIntervalMs: 100,
		PerPeerBurst:      5,
	}
	rl := newRateLimiter(metrics, cfg, nil, nil)
	if rl.policy.perPeerBurst != 5 {
		t.Errorf("expected burst 5, got %d", rl.policy.perPeerBurst)
	}
	rl.shutdown()
}

func TestIdentifyPeer_EdgeCases(t *testing.T) {
	rl := newRateLimiter(nil, nil, nil, nil)
	if id := rl.identifyPeer(nil); id.uid != 0 {
		t.Error("expected 0 UID for nil creds")
	}

	var nilRl *rateLimiter
	id := nilRl.identifyPeer(&peerCredentials{uid: 123})
	if id.uid != 123 {
		t.Error("expected 123 UID for nil limiter")
	}
}
