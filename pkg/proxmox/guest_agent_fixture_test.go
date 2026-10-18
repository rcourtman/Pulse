package proxmox

import (
	"testing"
	"time"
)

// Serial guest-agent fixtures own one registry for their whole scenario.
// Keep admission shared across clients and aliases within that fixture, then
// restore prior state before the package's parallel HTTP tests can start.
func isolateGuestAgentRegistry(t *testing.T) {
	t.Helper()
	guestAgentGuards.Lock()
	aliases, entries, next := guestAgentGuards.aliases, guestAgentGuards.entries, guestAgentGuards.nextCleanup
	guestAgentGuards.aliases = make(map[string][]string)
	guestAgentGuards.entries = make(map[guestAgentGuardKey]guestAgentGuardEntry)
	guestAgentGuards.nextCleanup = time.Now().Add(time.Hour)
	guestAgentGuards.Unlock()
	t.Cleanup(func() {
		guestAgentGuards.Lock()
		guestAgentGuards.aliases, guestAgentGuards.entries, guestAgentGuards.nextCleanup = aliases, entries, next
		guestAgentGuards.Unlock()
	})
}
