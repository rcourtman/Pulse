package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestSchedulerHealth_EnhancedResponse(t *testing.T) {
	cfg := &config.Config{
		AdaptivePollingEnabled:      true,
		AdaptivePollingBaseInterval: 2 * time.Second,
		PVEInstances: []config.PVEInstance{
			{Name: "pve-a", Host: "https://pve-a:8006"},
		},
		PBSInstances: []config.PBSInstance{
			{Name: "pbs-b", Host: "https://pbs-b:8007"},
		},
	}

	cfg.EnableBackupPolling = false

	monitor, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating monitor: %v", err)
	}

	instanceKey := schedulerKey(InstanceTypePVE, "pve-a")
	monitor.pollStatusMap[instanceKey] = &pollStatus{
		LastSuccess:         time.Now().Add(-30 * time.Second),
		LastErrorAt:         time.Now().Add(-10 * time.Second),
		LastErrorMessage:    "connection timeout",
		LastErrorCategory:   "transient",
		ConsecutiveFailures: 2,
		FirstFailureAt:      time.Now().Add(-20 * time.Second),
	}

	breaker := monitor.ensureBreaker(instanceKey)
	breaker.recordFailure(time.Now())
	monitor.dlqInsightMap[instanceKey] = &dlqInsight{
		Reason:       "max_retry_attempts",
		FirstAttempt: time.Now().Add(-5 * time.Minute),
		LastAttempt:  time.Now().Add(-1 * time.Minute),
		RetryCount:   5,
		NextRetry:    time.Now().Add(4 * time.Minute),
	}

	response := monitor.SchedulerHealth()

	if len(response.Instances) == 0 {
		t.Fatalf("expected instances to be populated")
	}

	found := false
	for _, inst := range response.Instances {
		if inst.Key == instanceKey {
			found = true
			if inst.DisplayName == "" {
				t.Fatalf("expected display name to be set")
			}
			if inst.Connection == "" {
				t.Fatalf("expected connection to be set")
			}
			if inst.PollStatus.ConsecutiveFailures != 2 {
				t.Fatalf("expected consecutive failures to be 2")
			}
			if inst.PollStatus.LastError == nil || inst.PollStatus.LastError.Message == "" {
				t.Fatalf("expected last error details")
			}
			if inst.Breaker.State == "" {
				t.Fatalf("expected breaker state")
			}
			if !inst.DeadLetter.Present || inst.DeadLetter.RetryCount != 5 {
				t.Fatalf("expected dead letter details to be present")
			}
		}
	}

	if !found {
		t.Fatalf("did not find instance %s in response", instanceKey)
	}
}
