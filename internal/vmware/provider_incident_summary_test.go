package vmware

import "testing"

// An incident summary is the alert message and the reason vSphere's Health
// column shows beside the resource name, so it says what vCenter flagged and
// nothing the row already shows: no managed object ID, no alarm colour.
func TestVMwareIncidentSummariesLeadWithWhatVCenterFlagged(t *testing.T) {
	network := InventoryNetwork{
		Network:       "network-302",
		Name:          "Edge Stateful",
		OverallStatus: "yellow",
		TriggeredAlarms: []InventoryAlarm{
			{Alarm: "alarm-41", Name: "Network packet loss above threshold", OverallStatus: "yellow"},
			{Alarm: "alarm-42", OverallStatus: "red"},
			{Name: "Cleared alarm", OverallStatus: "green"},
		},
	}
	incidents := networkIncidents(network)
	if len(incidents) != 2 {
		t.Fatalf("network incidents = %+v, want the yellow and red alarms only", incidents)
	}
	if got := incidents[0].Summary; got != "Network packet loss above threshold" {
		t.Fatalf("named alarm summary = %q, want the alarm's own name", got)
	}
	if got := incidents[1].Summary; got != "alarm-42" {
		t.Fatalf("unnamed alarm summary = %q, want its alarm ID", got)
	}
	if got := vmwareAlarmIncidentSummary(InventoryAlarm{OverallStatus: "red"}); got != "vCenter alarm" {
		t.Fatalf("anonymous alarm summary = %q, want vCenter alarm", got)
	}

	datastore := InventoryDatastore{Datastore: "datastore-204", Name: "edge-cold-iscsi", OverallStatus: "yellow"}
	incidents = datastoreIncidents(datastore)
	if len(incidents) != 1 || incidents[0].Code != "vmware_health_state" {
		t.Fatalf("datastore incidents = %+v, want one overall status incident", incidents)
	}
	if got := incidents[0].Summary; got != "vCenter health is yellow" {
		t.Fatalf("overall status summary = %q, want vCenter health is yellow", got)
	}
}
