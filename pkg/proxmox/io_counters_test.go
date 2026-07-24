package proxmox

import (
	"encoding/json"
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
