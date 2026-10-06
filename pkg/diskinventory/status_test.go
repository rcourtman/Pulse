package diskinventory

import "testing"

func TestMergeStatusPrefersAvailableEvidencePerField(t *testing.T) {
	existing := &CollectionStatus{
		Serial:      Available("smartctl"),
		Temperature: Unavailable("smartctl", "deadline exceeded"),
	}
	incoming := &CollectionStatus{
		Serial:      Unsupported("provider", "not exposed"),
		Temperature: Available("provider"),
		IO:          Missing("kernel", "counter absent"),
	}

	got := MergeStatus(existing, incoming)
	if got.Serial.State != FieldAvailable || got.Serial.Source != "smartctl" {
		t.Fatalf("available serial evidence was downgraded: %+v", got.Serial)
	}
	if got.Temperature.State != FieldAvailable || got.Temperature.Source != "provider" {
		t.Fatalf("available temperature evidence did not win: %+v", got.Temperature)
	}
	if got.IO.State != FieldMissing {
		t.Fatalf("missing I/O state was not retained: %+v", got.IO)
	}
}

func TestMergeReportedStatusLetsTheSourceWithdrawItsOwnAvailability(t *testing.T) {
	existing := &CollectionStatus{
		Serial:      Available("smartctl"),
		Temperature: Available("smartctl"),
		IO:          Available("kernel_diskstats"),
		Pool:        Available("proxmox_zfs"),
	}
	reported := &CollectionStatus{
		Serial:      Available("smartctl"),
		Temperature: Unavailable("smartctl", "host agent stopped reporting"),
		IO:          Unavailable("kernel_diskstats", "host agent stopped reporting"),
		Pool:        Unavailable("zpool", "command failed"),
	}

	got := MergeReportedStatus(existing, reported)
	if got.Temperature != reported.Temperature || got.IO != reported.IO {
		t.Fatalf("the source's later word on its own fields was ignored: %+v", got)
	}
	if got.Serial != Available("smartctl") {
		t.Fatalf("still-available field changed: %+v", got.Serial)
	}
	if got.Pool != Available("proxmox_zfs") {
		t.Fatalf("another source's available evidence was withdrawn: %+v", got.Pool)
	}

	// A copy that is already weaker never overrides a current report, and the
	// rule is plain MergeStatus when either side is absent.
	if got := MergeReportedStatus(reported, existing); got.Temperature != Available("smartctl") {
		t.Fatalf("current report lost to a weaker copy: %+v", got.Temperature)
	}
	if got := MergeReportedStatus(nil, reported); got.Temperature != reported.Temperature {
		t.Fatalf("nil existing: %+v", got)
	}
	if got := MergeReportedStatus(existing, nil); got.Temperature != existing.Temperature {
		t.Fatalf("nil report: %+v", got)
	}
	if existing.Temperature != Available("smartctl") {
		t.Fatal("MergeReportedStatus mutated its input")
	}
}
