package monitoring

import "testing"

func TestReplicationDeferralIsTreatedAsADeferredDiskRead(t *testing.T) {
	for _, reason := range []string{"replication-active", "prev-replication-active"} {
		if !guestAgentDiskDeferred(reason) {
			t.Errorf("guestAgentDiskDeferred(%q) = false, want true", reason)
		}
	}
}
