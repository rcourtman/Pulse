package notifications

import "testing"

func TestClassifyQueueHealthCountsOnlyTerminalOutcomes(t *testing.T) {
	// Pending and sending entries are work in progress, and retries are
	// expected. Treating them as unhealthy would fire on every transient blip.
	health := ClassifyQueueHealth(map[string]int{
		string(QueueStatusPending): 12,
		string(QueueStatusSending): 3,
		string(QueueStatusSent):    100,
	})

	if !health.Healthy {
		t.Error("expected in-flight work alone to stay healthy")
	}
	if health.Status != DeliveryHealthy {
		t.Errorf("Status = %q, want %q", health.Status, DeliveryHealthy)
	}
	if health.AttentionRequired != 0 {
		t.Errorf("AttentionRequired = %d, want 0", health.AttentionRequired)
	}
	if len(health.ReasonCodes) != 0 {
		t.Errorf("ReasonCodes = %v, want none", health.ReasonCodes)
	}
}

func TestClassifyQueueHealthFlagsRetainedFailures(t *testing.T) {
	health := ClassifyQueueHealth(map[string]int{
		string(QueueStatusFailed): 7,
		string(QueueStatusDLQ):    2,
		string(QueueStatusSent):   1,
	})

	if health.Healthy {
		t.Error("expected retained terminal failures to be unhealthy")
	}
	if health.Status != DeliveryDegraded {
		t.Errorf("Status = %q, want %q", health.Status, DeliveryDegraded)
	}
	if health.AttentionRequired != 9 {
		t.Errorf("AttentionRequired = %d, want 9", health.AttentionRequired)
	}
	if health.Failed != 7 || health.DeadLetter != 2 {
		t.Errorf("Failed/DeadLetter = %d/%d, want 7/2", health.Failed, health.DeadLetter)
	}
	if len(health.ReasonCodes) != 2 {
		t.Errorf("ReasonCodes = %v, want both retained-failure reasons", health.ReasonCodes)
	}
}

func TestUnavailableDeliveryHealthIsNotHealthy(t *testing.T) {
	// A queue that cannot be read must not read as success. The entire point of
	// this signal is that silence is the failure mode being guarded against.
	health := UnavailableDeliveryHealth()

	if health.Healthy {
		t.Error("expected an unreadable queue to be treated as not healthy")
	}
	if health.Status != DeliveryUnavailable {
		t.Errorf("Status = %q, want %q", health.Status, DeliveryUnavailable)
	}
	if len(health.ReasonCodes) != 1 || health.ReasonCodes[0] != reasonQueueStatsUnavailable {
		t.Errorf("ReasonCodes = %v, want %q", health.ReasonCodes, reasonQueueStatsUnavailable)
	}
}

func TestDeliveryHealthOnNilManagerIsUnavailable(t *testing.T) {
	var manager *NotificationManager

	if health := manager.DeliveryHealth(); health.Status != DeliveryUnavailable {
		t.Errorf("Status = %q, want %q", health.Status, DeliveryUnavailable)
	}
}
