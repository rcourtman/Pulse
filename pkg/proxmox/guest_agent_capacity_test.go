package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// These tests are serial, as are the other guest-guard tests. Restore the
// process-wide state before the package's parallel HTTP tests can start.
func isolateGuestAgentCapacity(t *testing.T) {
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

// Half the protected entries are busy; half retain uncertain completion.
func seedProtectedGuestAgentCapacity(count int) map[guestAgentGuardKey]guestAgentGuardEntry {
	protected := make(map[guestAgentGuardKey]guestAgentGuardEntry, count)
	until := time.Now().Add(time.Hour)
	guestAgentGuards.Lock()
	defer guestAgentGuards.Unlock()
	for i := 0; i < count; i++ {
		key := guestAgentGuardKey{endpoint: "https://protected.capacity.invalid/api2/json", vmid: 1000 + i}
		entry := guestAgentGuardEntry{busy: i%2 == 0, until: until}
		protected[key] = entry
		guestAgentGuards.entries[key] = entry
	}
	return protected
}

func assertProtectedGuestAgentCapacity(t *testing.T, protected map[guestAgentGuardKey]guestAgentGuardEntry) {
	t.Helper()
	guestAgentGuards.Lock()
	defer guestAgentGuards.Unlock()
	if len(guestAgentGuards.entries) > maxGuestAgentGuardEntries {
		t.Errorf("guard bound exceeded: %d", len(guestAgentGuards.entries))
	}
	for key, want := range protected {
		if got, ok := guestAgentGuards.entries[key]; !ok || got != want {
			t.Fatalf("protected busy/cooldown entry evicted or renewed: %+v", key)
		}
	}
}

func TestGuestAgentCapacityReusesExpiredAliasEntries(t *testing.T) {
	for _, expiredAliases := range []int{1, 2} {
		t.Run(fmt.Sprintf("expired-%d", expiredAliases), func(t *testing.T) {
			isolateGuestAgentCapacity(t)
			first, second := "https://first.capacity.invalid/api2/json", "https://second.capacity.invalid/api2/json"
			registerGuestAgentEndpoints("https://first.capacity.invalid", []string{"https://second.capacity.invalid"})
			protected := seedProtectedGuestAgentCapacity(maxGuestAgentGuardEntries - 1 - expiredAliases)
			guestAgentGuards.Lock()
			for _, endpoint := range []string{first, second}[:expiredAliases] {
				guestAgentGuards.entries[guestAgentGuardKey{endpoint: endpoint, vmid: 105}] = guestAgentGuardEntry{until: time.Now().Add(-time.Second)}
			}
			guestAgentGuards.Unlock()
			release, err := acquireGuestAgent(first, 105)
			if err != nil {
				t.Fatalf("expired alias was charged twice against capacity: %v", err)
			}
			if next, err := acquireGuestAgent(second, 105); GuestAgentDeferredReason(err) != "agent-busy" {
				if next != nil {
					next(false)
				}
				t.Errorf("reused capacity lost shared admission: %v", err)
			}
			assertProtectedGuestAgentCapacity(t, protected)
			release(true)
			if next, err := acquireGuestAgent(second, 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
				if next != nil {
					next(false)
				}
				t.Errorf("reused capacity lost uncertainty: %v", err)
			}
			assertProtectedGuestAgentCapacity(t, protected)
		})
	}
}

func TestGuestAgentCapacityReclaimsExpiredBeforeEveryHTTPRead(t *testing.T) {
	for name, read := range backupAgentReads() {
		t.Run(name, func(t *testing.T) {
			isolateGuestAgentCapacity(t)
			var commands, configs atomic.Int32
			var locked atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/config") {
					configs.Add(1)
					if locked.Load() {
						fmt.Fprint(w, `{"data":{"lock":"backup"}}`)
					} else {
						fmt.Fprint(w, `{"data":{}}`)
					}
					return
				}
				commands.Add(1)
				backupAgentPayload(w, r)
			}))
			defer server.Close()
			client := backupTestClient(t, server.URL)
			registerGuestAgentEndpoints(server.URL, []string{"https://alias.capacity.invalid"})
			protected := seedProtectedGuestAgentCapacity(maxGuestAgentGuardEntries - 2)
			guestAgentGuards.Lock()
			guestAgentGuards.entries[guestAgentGuardKey{endpoint: "https://expired.capacity.invalid/api2/json", vmid: 999}] = guestAgentGuardEntry{until: time.Now().Add(-time.Second)}
			guestAgentGuards.Unlock()
			if err := read(context.Background(), client, 105); err != nil {
				t.Fatalf("eligible read postponed until periodic cleanup: %v", err)
			}
			if commands.Load() != 1 || configs.Load() != 2 {
				t.Fatalf("single command and both lock checks required: commands=%d configs=%d", commands.Load(), configs.Load())
			}
			assertProtectedGuestAgentCapacity(t, protected)
			locked.Store(true)
			if err := read(context.Background(), client, 105); GuestAgentDeferredReason(err) != "vm-locked" {
				t.Errorf("capacity recovery bypassed backup admission: %v", err)
			}
			if commands.Load() != 1 || configs.Load() != 3 {
				t.Fatal("backup-locked read sent another command")
			}
			locked.Store(false)
			if err := read(context.Background(), client, 105); err != nil {
				t.Errorf("known-unlocked guest did not resume: %v", err)
			}
			if commands.Load() != 2 || configs.Load() != 5 {
				t.Fatal("resumption did not retain single-attempt/both-lock checks")
			}
			assertProtectedGuestAgentCapacity(t, protected)
		})
	}
}

func TestGuestAgentCapacityNeverEvictsProtectedWork(t *testing.T) {
	for _, slotsLeft := range []int{0, 1} {
		t.Run(fmt.Sprintf("slots-%d", slotsLeft), func(t *testing.T) {
			isolateGuestAgentCapacity(t)
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				backupAgentPayload(w, r)
			}))
			defer server.Close()
			client := backupTestClient(t, server.URL)
			registerGuestAgentEndpoints(server.URL, []string{"https://alias.capacity.invalid"})
			protected := seedProtectedGuestAgentCapacity(maxGuestAgentGuardEntries - slotsLeft)
			for name, read := range backupAgentReads() {
				if err := read(context.Background(), client, 105); GuestAgentDeferredReason(err) != "agent-capacity" {
					t.Errorf("%s evicted protected work instead of deferring: %v", name, err)
				}
			}
			if requests.Load() != 0 {
				t.Fatal("over-capacity guest sent config or agent request")
			}
			guestAgentGuards.Lock()
			unchanged := reflect.DeepEqual(guestAgentGuards.entries, protected)
			guestAgentGuards.Unlock()
			if !unchanged {
				t.Fatal("failed admission left partial alias reservations or changed protected entries")
			}
			assertProtectedGuestAgentCapacity(t, protected)
		})
	}
}
