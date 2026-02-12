package monitoring

import "testing"

type fakeStateBroadcaster struct {
	globalBroadcasts int
	tenantBroadcasts []string
}

func (f *fakeStateBroadcaster) BroadcastState(_ interface{}) {
	f.globalBroadcasts++
}

func (f *fakeStateBroadcaster) BroadcastStateToTenant(orgID string, _ interface{}) {
	f.tenantBroadcasts = append(f.tenantBroadcasts, orgID)
}

func TestMonitorBroadcastStateUsesTenantChannelForDefaultOrg(t *testing.T) {
	m := &Monitor{}
	m.SetOrgID("default")

	broadcaster := &fakeStateBroadcaster{}
	m.broadcastState(broadcaster, map[string]string{"kind": "state"})

	if broadcaster.globalBroadcasts != 0 {
		t.Fatalf("expected no global broadcasts, got %d", broadcaster.globalBroadcasts)
	}
	if len(broadcaster.tenantBroadcasts) != 1 || broadcaster.tenantBroadcasts[0] != "default" {
		t.Fatalf("expected tenant broadcast to default org, got %+v", broadcaster.tenantBroadcasts)
	}
}

func TestMonitorBroadcastStateUsesGlobalChannelForLegacyMonitor(t *testing.T) {
	m := &Monitor{}

	broadcaster := &fakeStateBroadcaster{}
	m.broadcastState(broadcaster, map[string]string{"kind": "state"})

	if broadcaster.globalBroadcasts != 1 {
		t.Fatalf("expected one global broadcast, got %d", broadcaster.globalBroadcasts)
	}
	if len(broadcaster.tenantBroadcasts) != 0 {
		t.Fatalf("expected no tenant broadcasts, got %+v", broadcaster.tenantBroadcasts)
	}
}

func TestMonitorSetOrgIDTrimsWhitespace(t *testing.T) {
	m := &Monitor{}
	m.SetOrgID("  org-a  ")

	if got := m.GetOrgID(); got != "org-a" {
		t.Fatalf("expected trimmed org ID org-a, got %q", got)
	}
}
