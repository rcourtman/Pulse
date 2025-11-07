package main

import (
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
