package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	replicationInventoryWithGuest = `{"data":[{"id":"105-0","guest":105,"type":"local","target":"other"}]}`
	replicationStatusIdle         = `{"data":[{"id":"105-0","guest":105,"last_try":1,"duration":3.5}]}`
	replicationStatusNone         = `{"data":[]}`
	replicationStatusHTTP500      = "http-500"
	replicationStatusHTTP404      = "http-404"
	replicationStatusHTTP502      = "http-502"
)

func replicationStatusRunningFor(age time.Duration) string {
	return fmt.Sprintf(`{"data":[{"id":"105-0","guest":105,"pid":4242,"last_try":%d}]}`, time.Now().Add(-age).Unix())
}

type replicationFixture struct {
	inventory   atomic.Value
	status      atomic.Value
	statusCalls atomic.Int32
	agentCalls  atomic.Int32
}

func newReplicationFixture(t *testing.T, inventory, status string) (*replicationFixture, *Client) {
	t.Helper()
	fixture := &replicationFixture{}
	fixture.inventory.Store(inventory)
	fixture.status.Store(status)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{}}`)
		case strings.HasSuffix(r.URL.Path, "/cluster/replication"):
			fmt.Fprint(w, fixture.inventory.Load().(string))
		case strings.HasSuffix(r.URL.Path, "/nodes/node/replication"):
			fixture.statusCalls.Add(1)
			if got := r.URL.Query().Get("guest"); got != "105" {
				t.Errorf("replication status guest = %q, want 105", got)
			}
			switch body := fixture.status.Load().(string); body {
			case replicationStatusHTTP500:
				http.Error(w, `{"data":null}`, http.StatusInternalServerError)
			case replicationStatusHTTP404:
				http.NotFound(w, r)
			case replicationStatusHTTP502:
				http.Error(w, `{"data":null}`, http.StatusBadGateway)
			default:
				fmt.Fprint(w, body)
			}
		case strings.Contains(r.URL.Path, "/agent/"):
			fixture.agentCalls.Add(1)
			backupAgentPayload(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return fixture, backupTestClient(t, guestAgentFixtureEndpoint(t, server.URL))
}

func readReplicationInventory(t *testing.T, c *Client) {
	t.Helper()
	if _, err := c.GetReplicationStatus(context.Background()); err != nil {
		t.Fatalf("read replication inventory: %v", err)
	}
}

func TestGuestAgentReplicationBlocksEveryReadAndResumes(t *testing.T) {
	fixture, c := newReplicationFixture(t, replicationInventoryWithGuest, replicationStatusRunningFor(time.Minute))
	readReplicationInventory(t, c)
	for name, read := range backupAgentReads() {
		t.Run(name, func(t *testing.T) {
			err := read(context.Background(), c, 105)
			if got := GuestAgentDeferredReason(err); got != "replication-active" {
				t.Fatalf("deferred reason = %q (err %v), want replication-active", got, err)
			}
		})
	}
	if got := fixture.agentCalls.Load(); got != 0 {
		t.Errorf("agent requests while replicating = %d, want zero", got)
	}
	fixture.status.Store(replicationStatusIdle)
	for name, read := range backupAgentReads() {
		t.Run("resumed-"+name, func(t *testing.T) {
			if err := read(context.Background(), c, 105); err != nil {
				t.Fatal(err)
			}
		})
	}
	if got := fixture.agentCalls.Load(); got != 6 {
		t.Errorf("agent requests after replication = %d, want six", got)
	}
}

func TestGuestAgentLongReplicationTransferDoesNotHoldReadings(t *testing.T) {
	fixture, c := newReplicationFixture(t, replicationInventoryWithGuest, replicationStatusRunningFor(guestReplicationFreezeWindow+time.Minute))
	readReplicationInventory(t, c)
	if _, err := c.GetVMFSInfo(context.Background(), "node", 105); err != nil {
		t.Fatal(err)
	}
	if got := fixture.agentCalls.Load(); got != 1 {
		t.Errorf("agent requests = %d, want one", got)
	}
}

func TestGuestAgentReplicationWithUnusableStartTimeStaysDeferred(t *testing.T) {
	for name, status := range map[string]string{
		"missing":  `{"data":[{"pid":4242}]}`,
		"null":     `{"data":[{"pid":4242,"last_try":null}]}`,
		"unusable": `{"data":[{"pid":4242,"last_try":"soon"}]}`,
		"zero":     `{"data":[{"pid":4242,"last_try":0}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, c := newReplicationFixture(t, replicationInventoryWithGuest, status)
			readReplicationInventory(t, c)
			_, err := c.GetVMFSInfo(context.Background(), "node", 105)
			if got := GuestAgentDeferredReason(err); got != "replication-active" {
				t.Fatalf("deferred reason = %q (err %v), want replication-active", got, err)
			}
		})
	}
}

func TestGuestAgentGuestWithoutReplicationJobSkipsStatusRead(t *testing.T) {
	for name, inventory := range map[string]string{
		"other guest": `{"data":[{"id":"200-0","guest":200}]}`,
		"no jobs":     `{"data":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			fixture, c := newReplicationFixture(t, inventory, replicationStatusRunningFor(time.Minute))
			readReplicationInventory(t, c)
			if _, err := c.GetVMFSInfo(context.Background(), "node", 105); err != nil {
				t.Fatal(err)
			}
			if got := fixture.statusCalls.Load(); got != 0 {
				t.Errorf("status reads = %d, want zero", got)
			}
		})
	}
}

func TestGuestAgentBeforeFirstInventoryDoesNotCheckReplication(t *testing.T) {
	fixture, c := newReplicationFixture(t, replicationInventoryWithGuest, replicationStatusRunningFor(time.Minute))
	if _, err := c.GetVMFSInfo(context.Background(), "node", 105); err != nil {
		t.Fatal(err)
	}
	if got := fixture.statusCalls.Load(); got != 0 {
		t.Errorf("status reads = %d, want zero", got)
	}
}

func TestGuestAgentReplicationInventoryOnlyAddsGuests(t *testing.T) {
	fixture, c := newReplicationFixture(t, `{"data":[]}`, replicationStatusRunningFor(time.Minute))
	readReplicationInventory(t, c)
	if _, err := c.GetVMFSInfo(context.Background(), "node", 105); err != nil {
		t.Fatal(err)
	}

	fixture.inventory.Store(replicationInventoryWithGuest)
	readReplicationInventory(t, c)
	_, err := c.GetVMFSInfo(context.Background(), "node", 105)
	if got := GuestAgentDeferredReason(err); got != "replication-active" {
		t.Fatalf("added job: deferred reason = %q (err %v), want replication-active", got, err)
	}

	fixture.inventory.Store(`{"data":null}`)
	if _, err := c.GetReplicationStatus(context.Background()); err == nil {
		t.Fatal("incomplete inventory was accepted")
	}
	_, err = c.GetVMFSInfo(context.Background(), "node", 105)
	if got := GuestAgentDeferredReason(err); got != "replication-active" {
		t.Fatalf("failed inventory: deferred reason = %q (err %v), want replication-active", got, err)
	}

	// Another token with a narrower VM.Audit scope can list nothing for this guest.
	fixture.inventory.Store(`{"data":[]}`)
	readReplicationInventory(t, c)
	_, err = c.GetVMFSInfo(context.Background(), "node", 105)
	if got := GuestAgentDeferredReason(err); got != "replication-active" {
		t.Fatalf("narrower inventory: deferred reason = %q (err %v), want replication-active", got, err)
	}

	// A job that really was removed answers with nothing running.
	fixture.status.Store(replicationStatusNone)
	if _, err := c.GetVMFSInfo(context.Background(), "node", 105); err != nil {
		t.Fatalf("removed job: %v", err)
	}
}

func TestGuestAgentReplicationInventoryCoversClusterEndpoints(t *testing.T) {
	registerGuestAgentEndpoints("https://replication-a.test:8006", []string{"https://replication-b.test:8006"})
	recordGuestReplicationJobs("https://replication-a.test:8006/api2/json", []ReplicationJob{{GuestID: 105}})
	if !guestHasReplicationJob("https://replication-b.test:8006/api2/json", 105) {
		t.Error("other cluster endpoint did not see the job")
	}
	if guestHasReplicationJob("https://replication-b.test:8006/api2/json", 106) {
		t.Error("guest without a job was reported as having one")
	}
	if guestHasReplicationJob("https://replication-unrelated.test:8006/api2/json", 105) {
		t.Error("unrelated endpoint saw another cluster's job")
	}
}

func TestGuestAgentUnknownReplicationStateFailsClosed(t *testing.T) {
	cases := map[string]string{
		"bad gateway":   replicationStatusHTTP502,
		"garbage":       `not json`,
		"null data":     `{"data":null}`,
		"object data":   `{"data":{}}`,
		"extra value":   `{"data":[]}{"data":[]}`,
		"duplicate pid": `{"data":[{"pid":1,"pid":0}]}`,
		"unusable pid":  `{"data":[{"pid":"running"}]}`,
	}
	for name, status := range cases {
		t.Run(name, func(t *testing.T) {
			fixture, c := newReplicationFixture(t, replicationInventoryWithGuest, status)
			readReplicationInventory(t, c)
			_, err := c.GetVMFSInfo(context.Background(), "node", 105)
			if got := GuestAgentDeferredReason(err); got != "lock-unverified" {
				t.Fatalf("deferred reason = %q (err %v), want lock-unverified", got, err)
			}
			if got := fixture.agentCalls.Load(); got != 0 {
				t.Errorf("agent requests = %d, want zero", got)
			}
		})
	}
}

func TestGuestAgentReplicationStatusFailureProxmoxReportsDoesNotDeferReads(t *testing.T) {
	for name, status := range map[string]string{
		"no quorum":     replicationStatusHTTP500,
		"missing route": replicationStatusHTTP404,
	} {
		t.Run(name, func(t *testing.T) {
			fixture, c := newReplicationFixture(t, replicationInventoryWithGuest, status)
			readReplicationInventory(t, c)
			if _, err := c.GetVMFSInfo(context.Background(), "node", 105); err != nil {
				t.Fatal(err)
			}
			if got := fixture.agentCalls.Load(); got != 1 {
				t.Errorf("agent requests = %d, want one", got)
			}
		})
	}
}
