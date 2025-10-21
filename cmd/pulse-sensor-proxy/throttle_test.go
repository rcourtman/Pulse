package main

import (
    "testing"
    "time"
)

func TestRateLimiterPenalizeMetrics(t *testing.T) {
    metrics := NewProxyMetrics("test")
    rl := newRateLimiter(metrics, nil)
    rl.policy.penaltyDuration = 10 * time.Millisecond

    start := time.Now()
    rl.penalize(peerID{uid: 42}, "invalid_json")
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
            for _, label := range metric.GetLabel() {
                if label.GetName() == "reason" && label.GetValue() == "invalid_json" {
                    found = true
                }
            }
        }
    }
    if !found {
        t.Fatalf("expected limiter penalty metric for invalid_json")
    }
}
