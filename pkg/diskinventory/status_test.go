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
