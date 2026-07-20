package unifiedresources

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAvailabilityDataPreservesThreeStateUDPWireEvidence(t *testing.T) {
	resource := Resource{
		ID:   "availability:syslog",
		Type: ResourceTypeNetworkEndpoint,
		Availability: &AvailabilityData{
			Protocol:     "udp",
			ProbeOutcome: "indeterminate",
			UDPMode:      "open_or_filtered",
			Available:    false,
		},
	}

	payload, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	wire := string(payload)
	for _, expected := range []string{
		`"protocol":"udp"`,
		`"probeOutcome":"indeterminate"`,
		`"udpMode":"open_or_filtered"`,
	} {
		if !strings.Contains(wire, expected) {
			t.Fatalf("wire payload %s does not contain %s", wire, expected)
		}
	}
	if strings.Contains(wire, `"available":true`) {
		t.Fatalf("indeterminate UDP evidence must not claim availability: %s", wire)
	}
}
