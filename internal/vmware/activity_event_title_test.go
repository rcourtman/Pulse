package vmware

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// An event reads as vCenter's own sentence; the event class is the fallback
// when vCenter sends no message, and it stays in metadata for search.
func TestVMwareEventActivityReadsAsVCenterMessage(t *testing.T) {
	changes := entityActivityChanges("agent-host-101", "vc-1", "host", "host-101", nil, []InventoryEvent{
		{Event: "event-1", Type: "HostConnectedEvent", Message: "Host esxi-01 in Primary DC is connected"},
		{Event: "event-2", Type: "HostProfileAppliedEvent"},
	})
	if len(changes) != 2 {
		t.Fatalf("changes = %+v, want two event changes", changes)
	}

	byEvent := map[string]unifiedresources.ResourceChange{}
	for _, change := range changes {
		byEvent[change.Metadata["vmwareEvent"].(string)] = change
	}

	messaged := byEvent["event-1"]
	if messaged.Reason != "Host esxi-01 in Primary DC is connected" {
		t.Fatalf("messaged event reason = %q, want vCenter's message", messaged.Reason)
	}
	if got := messaged.Metadata[unifiedresources.MetadataActivityTitle]; got != "Host esxi-01 in Primary DC is connected" {
		t.Fatalf("messaged event title = %v, want vCenter's message", got)
	}
	if got := messaged.Metadata["vmwareEventType"]; got != "HostConnectedEvent" {
		t.Fatalf("event class = %v, want it kept in metadata", got)
	}

	if got := byEvent["event-2"].Reason; got != "HostProfileAppliedEvent" {
		t.Fatalf("unmessaged event reason = %q, want the event class", got)
	}
}
