package proxmox

import (
	"context"
	"testing"
	"time"
)

type replicationSafetyReader interface {
	GetVMAgentInfo(context.Context, string, int) (map[string]interface{}, error)
	GetVMAgentVersion(context.Context, string, int) (string, error)
	GetVMFSInfo(context.Context, string, int) ([]VMFileSystem, error)
	GetVMNetworkInterfaces(context.Context, string, int) ([]VMNetworkInterface, error)
	GetVMMemoryAvailabilityFromAgent(context.Context, string, int) (LinuxMemoryAvailability, error)
	GetVMMemAvailableFromAgent(context.Context, string, int) (uint64, error)
}

func replicationSafetyReads(c replicationSafetyReader) map[string]func() error {
	ctx := context.Background()
	return map[string]func() error{
		"os":            func() error { _, err := c.GetVMAgentInfo(ctx, "node", 105); return err },
		"version":       func() error { _, err := c.GetVMAgentVersion(ctx, "node", 105); return err },
		"filesystem":    func() error { _, err := c.GetVMFSInfo(ctx, "node", 105); return err },
		"network":       func() error { _, err := c.GetVMNetworkInterfaces(ctx, "node", 105); return err },
		"memory":        func() error { _, err := c.GetVMMemoryAvailabilityFromAgent(ctx, "node", 105); return err },
		"legacy-memory": func() error { _, err := c.GetVMMemAvailableFromAgent(ctx, "node", 105); return err },
	}
}

// A complete server error proves neither that an existing job stopped nor
// that its guest thawed. A subsequent positive status, not an error body or
// an expired process-local fence, must be what restores guest commands.
func TestGuestAgentReplicationHTTP500DefersEveryReader(t *testing.T) {
	for _, kind := range []string{"direct", "cluster"} {
		for _, wasActive := range []bool{false, true} {
			name := kind + "/initial-unknown"
			if wasActive {
				name = kind + "/active-to-unknown"
			}
			t.Run(name, func(t *testing.T) {
				fixture, direct := newReplicationFixture(t, replicationInventoryWithGuest, replicationStatusHTTP500)
				readReplicationInventory(t, direct)
				var client replicationSafetyReader = direct
				if kind == "cluster" {
					client = NewClusterClient("replication-safety", direct.config, []string{direct.config.Host}, nil)
				}
				reads := replicationSafetyReads(client)
				if wasActive {
					fixture.status.Store(replicationStatusRunningFor(time.Minute))
					for name, read := range reads {
						t.Run("active-"+name, func(t *testing.T) {
							if err := read(); GuestAgentDeferredReason(err) != "replication-active" {
								t.Errorf("active job: %v", err)
							}
						})
					}
				}
				fixture.status.Store(replicationStatusHTTP500)
				before := fixture.agentCalls.Load()
				for name, read := range reads {
					t.Run("unknown-"+name, func(t *testing.T) {
						if err := read(); GuestAgentDeferredReason(err) != "lock-unverified" {
							t.Errorf("HTTP 500 admitted a guest read: %v", err)
						}
					})
				}
				if got := fixture.agentCalls.Load(); got != before {
					t.Errorf("HTTP 500 dispatched %d guest commands, want zero", got-before)
				}
				fixture.status.Store(replicationStatusIdle)
				before = fixture.agentCalls.Load()
				for name, read := range reads {
					t.Run("recovered-"+name, func(t *testing.T) {
						if err := read(); err != nil {
							t.Errorf("verified idle did not resume: %v", err)
						}
					})
				}
				if got := fixture.agentCalls.Load() - before; got != 6 {
					t.Errorf("verified idle dispatched %d commands, want six", got)
				}
			})
		}
	}
}
