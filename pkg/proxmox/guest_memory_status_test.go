package proxmox

import (
	"encoding/json"
	"testing"
)

// These tests also run unchanged against the parent: the wire distinction is
// the requirement, rather than the new implementation's private presence bit.
func testVMMemInfoAvailabilityRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name, wire string
		present    bool
		available  uint64
	}{
		{"measured-zero", `{"total":8192,"available":0,"free":1024,"cached":4096}`, true, 0},
		{"positive", `{"total":8192,"available":6144}`, true, 6144},
		{"missing", `{"total":8192,"free":1024,"cached":4096}`, false, 0},
		{"null-is-not-zero", `{"total":8192,"available":null,"free":1024,"cached":4096}`, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sample VMMemInfo
			if err := json.Unmarshal([]byte(tc.wire), &sample); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 3; i++ {
				encoded, err := json.Marshal(sample)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(encoded, &fields); err != nil {
					t.Fatal(err)
				}
				_, present := fields["available"]
				if present != tc.present || sample.Available != tc.available || sample.Total != 8192 {
					t.Errorf("round trip %d: availability presence/value/capacity lost: %s", i, encoded)
				}
				if err := json.Unmarshal(encoded, &sample); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func testVMMemInfoAvailabilityRejectsInvalidNumbers(t *testing.T) {
	for _, value := range []string{`-1`, `0.5`, `"0"`, `true`, `{}`, `[]`, `18446744073709551616`} {
		t.Run(value, func(t *testing.T) {
			var sample VMMemInfo
			if err := json.Unmarshal([]byte(`{"total":8192,"available":`+value+`}`), &sample); err == nil {
				t.Fatal("invalid availability became a valid sample")
			}
		})
	}
}

func testVMMemInfoAvailabilityPresenceDoesNotLeakAcrossDecode(t *testing.T) {
	var sample VMMemInfo
	for _, wire := range []string{`{"total":8192,"available":0}`, `{"total":4096}`, `{"total":2048,"available":null}`} {
		if err := json.Unmarshal([]byte(wire), &sample); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(sample)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		json.Unmarshal(encoded, &fields)
		_, present := fields["available"]
		want := wire == `{"total":8192,"available":0}`
		if present != want {
			t.Errorf("presence survived unrelated sample: %s", encoded)
		}
	}
}
