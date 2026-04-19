package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func ptrTime(t time.Time) *time.Time { return &t }

func healthEntry(lastSuccess *time.Time, errMessage, errCategory string, breakerState string) monitoring.InstanceHealth {
	ps := monitoring.InstancePollStatus{LastSuccess: lastSuccess}
	if errMessage != "" {
		ps.LastError = &monitoring.ErrorDetail{
			At:       time.Now(),
			Message:  errMessage,
			Category: errCategory,
		}
	}
	return monitoring.InstanceHealth{
		PollStatus: ps,
		Breaker:    monitoring.InstanceBreaker{State: breakerState},
	}
}

func TestDeriveConnectionState_Paused(t *testing.T) {
	state, reason, _, _ := deriveConnectionState(false, monitoring.InstanceHealth{}, time.Now())
	if state != ConnectionStatePaused {
		t.Fatalf("got %q, want %q", state, ConnectionStatePaused)
	}
	if reason == "" {
		t.Fatal("expected non-empty reason for paused state")
	}
}

func TestDeriveConnectionState_Pending(t *testing.T) {
	state, _, lastSeen, lastError := deriveConnectionState(true, monitoring.InstanceHealth{}, time.Now())
	if state != ConnectionStatePending {
		t.Fatalf("got %q, want %q", state, ConnectionStatePending)
	}
	if lastSeen != nil || lastError != nil {
		t.Fatal("expected nil lastSeen and lastError in pending state")
	}
}

func TestDeriveConnectionState_Unauthorized(t *testing.T) {
	now := time.Now()
	h := healthEntry(ptrTime(now.Add(-30*time.Second)), "401 Unauthorized: token invalid", "auth", "closed")
	state, reason, _, err := deriveConnectionState(true, h, now)
	if state != ConnectionStateUnauthorized {
		t.Fatalf("got %q, want %q", state, ConnectionStateUnauthorized)
	}
	if reason == "" {
		t.Fatal("expected reason to include error message")
	}
	if err == nil || err.Message != "401 Unauthorized: token invalid" {
		t.Fatalf("unexpected lastError: %+v", err)
	}
}

func TestDeriveConnectionState_Unreachable(t *testing.T) {
	now := time.Now()
	h := healthEntry(ptrTime(now.Add(-30*time.Second)), "connection refused", "network", "open")
	state, _, _, _ := deriveConnectionState(true, h, now)
	if state != ConnectionStateUnreachable {
		t.Fatalf("got %q, want %q", state, ConnectionStateUnreachable)
	}
}

func TestDeriveConnectionState_Stale(t *testing.T) {
	now := time.Now()
	stale := now.Add(-5 * time.Minute)
	h := healthEntry(ptrTime(stale), "", "", "closed")
	state, reason, _, _ := deriveConnectionState(true, h, now)
	if state != ConnectionStateStale {
		t.Fatalf("got %q, want %q", state, ConnectionStateStale)
	}
	if reason == "" {
		t.Fatal("expected reason to describe staleness")
	}
}

func TestDeriveConnectionState_Active(t *testing.T) {
	now := time.Now()
	h := healthEntry(ptrTime(now.Add(-10*time.Second)), "", "", "closed")
	state, _, _, _ := deriveConnectionState(true, h, now)
	if state != ConnectionStateActive {
		t.Fatalf("got %q, want %q", state, ConnectionStateActive)
	}
}

func TestBuildConnections_SortsByTypeThenName(t *testing.T) {
	now := time.Now()
	in := aggregatorInputs{
		pveInstances: []config.PVEInstance{
			{Name: "beta", Host: "https://b.lan:8006", MonitorVMs: true},
			{Name: "alpha", Host: "https://a.lan:8006", MonitorVMs: true},
		},
		pbsInstances: []config.PBSInstance{
			{Name: "backups", Host: "https://bkp.lan:8007"},
		},
		now: now,
	}
	got := buildConnections(in)
	if len(got) != 3 {
		t.Fatalf("expected 3 connections, got %d", len(got))
	}
	if got[0].Type != ConnectionTypePBS {
		t.Fatalf("expected PBS first, got %q", got[0].Type)
	}
	if got[1].Name != "alpha" || got[2].Name != "beta" {
		t.Fatalf("PVE order wrong: %q, %q", got[1].Name, got[2].Name)
	}
}

func TestBuildConnections_PVEPausedRespectsDisabled(t *testing.T) {
	in := aggregatorInputs{
		pveInstances: []config.PVEInstance{{Name: "pve1", Host: "https://pve1.lan:8006", Disabled: true}},
		now:          time.Now(),
	}
	got := buildConnections(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(got))
	}
	if got[0].State != ConnectionStatePaused {
		t.Fatalf("expected paused, got %q", got[0].State)
	}
	if got[0].Enabled {
		t.Fatal("expected Enabled=false for Disabled PVE instance")
	}
}

func TestBuildConnections_AgentStateFromLastSeen(t *testing.T) {
	now := time.Now()
	in := aggregatorInputs{
		hosts: []models.Host{
			{ID: "fresh", Hostname: "h1", LastSeen: now.Add(-10 * time.Second)},
			{ID: "stale", Hostname: "h2", LastSeen: now.Add(-5 * time.Minute)},
			{ID: "never", Hostname: "h3"},
		},
		now: now,
	}
	got := buildConnections(in)
	byID := map[string]Connection{}
	for _, c := range got {
		byID[c.ID] = c
	}
	if byID["agent:fresh"].State != ConnectionStateActive {
		t.Fatalf("fresh agent: got %q, want active", byID["agent:fresh"].State)
	}
	if byID["agent:stale"].State != ConnectionStateStale {
		t.Fatalf("stale agent: got %q, want stale", byID["agent:stale"].State)
	}
	if byID["agent:never"].State != ConnectionStatePending {
		t.Fatalf("never-reported agent: got %q, want pending", byID["agent:never"].State)
	}
	for _, c := range got {
		if c.Capabilities.SupportsPause || c.Capabilities.SupportsScope {
			t.Fatalf("agents must not advertise pause/scope capabilities: %+v", c)
		}
	}
}

func TestBuildConnections_VMwareAndTrueNASEnabledFlag(t *testing.T) {
	in := aggregatorInputs{
		vmwareInstances:  []config.VMwareVCenterInstance{{ID: "vc1", Name: "vc", Host: "vc.lan", Enabled: false}},
		truenasInstances: []config.TrueNASInstance{{ID: "tn1", Name: "tn", Host: "tn.lan", Enabled: true, UseHTTPS: true}},
		now:              time.Now(),
	}
	got := buildConnections(in)
	var vmw, tn Connection
	for _, c := range got {
		switch c.Type {
		case ConnectionTypeVMware:
			vmw = c
		case ConnectionTypeTrueNAS:
			tn = c
		}
	}
	if vmw.State != ConnectionStatePaused || vmw.Enabled {
		t.Fatalf("vmware with Enabled=false should be paused, got state=%q enabled=%v", vmw.State, vmw.Enabled)
	}
	if tn.State != ConnectionStatePending {
		t.Fatalf("truenas with no health yet should be pending, got %q", tn.State)
	}
	if !tn.Enabled {
		t.Fatal("truenas with Enabled=true should surface enabled=true")
	}
}

func TestBuildConnections_UsesHealthLookup(t *testing.T) {
	now := time.Now()
	ls := now.Add(-15 * time.Second)
	in := aggregatorInputs{
		pveInstances: []config.PVEInstance{{Name: "pve1", Host: "https://pve1.lan:8006"}},
		instanceHealth: map[string]monitoring.InstanceHealth{
			"pve::pve1": healthEntry(&ls, "", "", "closed"),
		},
		now: now,
	}
	got := buildConnections(in)
	if got[0].State != ConnectionStateActive {
		t.Fatalf("expected active, got %q", got[0].State)
	}
	if got[0].LastSeen == nil || !got[0].LastSeen.Equal(ls) {
		t.Fatalf("lastSeen not propagated: %+v", got[0].LastSeen)
	}
}
