package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClusterResourceCounterPresenceDistinguishesZeroNullAndMissing(t *testing.T) {
	var resource ClusterResource
	if err := json.Unmarshal([]byte(`{
		"type":"qemu",
		"diskread":0,
		"diskwrite":null,
		"netin":42
	}`), &resource); err != nil {
		t.Fatal(err)
	}

	presence := resource.IOCounters.Effective()
	if !presence.DiskRead || !presence.NetworkIn {
		t.Fatalf("explicit zero/value fields were not present: %+v", presence)
	}
	if presence.DiskWrite || presence.NetworkOut {
		t.Fatalf("null/missing fields were incorrectly present: %+v", presence)
	}
}

func TestGuestStatusTypesRetainCounterPresence(t *testing.T) {
	tests := []struct {
		name string
		read func() IOCounterPresence
	}{
		{
			name: "vm listing",
			read: func() IOCounterPresence {
				var value VM
				_ = json.Unmarshal([]byte(`{"diskread":0}`), &value)
				return value.IOCounters
			},
		},
		{
			name: "lxc status",
			read: func() IOCounterPresence {
				var value Container
				_ = json.Unmarshal([]byte(`{"diskread":0}`), &value)
				return value.IOCounters
			},
		},
		{
			name: "qemu status",
			read: func() IOCounterPresence {
				var value VMStatus
				_ = json.Unmarshal([]byte(`{"diskread":0}`), &value)
				return value.IOCounters
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			presence := test.read().Effective()
			if !presence.DiskRead || presence.DiskWrite || presence.NetworkIn || presence.NetworkOut {
				t.Fatalf("presence = %+v, want only diskread", presence)
			}
		})
	}
}

func TestObservationStampUsesOneReceiptTimeForResponse(t *testing.T) {
	observedAt := time.Date(2026, time.July, 24, 8, 30, 0, 0, time.UTC)

	vms := []VM{{VMID: 100}, {VMID: 101}}
	stampVMObservation(vms, observedAt)
	for _, vm := range vms {
		if !vm.ObservedAt.Equal(observedAt) {
			t.Fatalf("VM %d observedAt = %v", vm.VMID, vm.ObservedAt)
		}
	}

	containers := []Container{{VMID: 200}, {VMID: 201}}
	stampContainerObservation(containers, observedAt)
	for _, container := range containers {
		if !container.ObservedAt.Equal(observedAt) {
			t.Fatalf("container %d observedAt = %v", container.VMID, container.ObservedAt)
		}
	}

	resources := []ClusterResource{{VMID: 300}, {VMID: 301}}
	stampClusterResourceObservation(resources, observedAt)
	for _, resource := range resources {
		if !resource.ObservedAt.Equal(observedAt) {
			t.Fatalf("resource %d observedAt = %v", resource.VMID, resource.ObservedAt)
		}
	}
}

func TestInternalCounterMetadataNeverChangesProxmoxWireShape(t *testing.T) {
	payload, err := json.Marshal(ClusterResource{
		VMID:       100,
		DiskRead:   0,
		IOCounters: IOCounterPresence{Explicit: true, DiskRead: true},
		ObservedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ioCounters", "IOCounters", "observedAt", "ObservedAt"} {
		if _, ok := raw[key]; ok {
			t.Fatalf("internal counter metadata %q leaked into JSON", key)
		}
	}
}

func TestBackupLocksRetainCumulativeCounterPresence(t *testing.T) {
	payload := []byte(`{"lock":"backup","diskread":0,"diskwrite":null,"netin":42}`)
	var listing VM
	var resource ClusterResource
	var status VMStatus
	for _, dst := range []any{&listing, &resource, &status} {
		if err := json.Unmarshal(payload, dst); err != nil {
			t.Fatal(err)
		}
	}
	for name, value := range map[string]struct {
		lock     string
		presence IOCounterPresence
	}{"listing": {listing.Lock, listing.IOCounters}, "resource": {resource.Lock, resource.IOCounters}, "status": {status.Lock, status.IOCounters}} {
		t.Run(name, func(t *testing.T) {
			p := value.presence.Effective()
			if value.lock != "backup" || !p.DiskRead || !p.NetworkIn || p.DiskWrite || p.NetworkOut {
				t.Fatalf("lock decoding corrupted presence: %#v", value)
			}
		})
	}
}

func TestUnverifiedVMConfigPreservesStatusCounterObservations(t *testing.T) {
	var commands atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			w.Header().Set("Content-Length", "1000")
			fmt.Fprint(w, `{"data":{}}`)
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			fmt.Fprint(w, `{"data":{"status":"running","cpu":0.25,"diskread":0,"diskwrite":null,"netin":42}}`)
		default:
			commands.Add(1)
			backupAgentPayload(w, r)
		}
	}))
	defer server.Close()
	c := backupTestClient(t, server.URL)
	if _, err := c.GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "lock-unverified" {
		t.Errorf("incomplete config did not defer the guest command: %v", err)
	}
	status, err := c.GetVMStatus(context.Background(), "node", 105)
	if err != nil {
		t.Fatal(err)
	}
	p := status.IOCounters.Effective()
	if status.CPU != 0.25 || status.NetIn != 42 || status.DiskRead != 0 || status.ObservedAt.IsZero() || !p.DiskRead || !p.NetworkIn || p.DiskWrite || p.NetworkOut {
		t.Fatalf("unverified guest lock suppressed/changed current PVE counters: %+v", status)
	}
	if got := commands.Load(); got != 0 {
		t.Errorf("guest commands with incomplete config = %d, want zero", got)
	}
}
