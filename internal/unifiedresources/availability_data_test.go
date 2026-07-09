package unifiedresources

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAvailabilityDataJSONOmitsUnknownProbeTimes(t *testing.T) {
	payload, err := json.Marshal(AvailabilityData{TargetID: "router"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	wire := string(payload)
	if strings.Contains(wire, "lastChecked") || strings.Contains(wire, "lastSuccess") {
		t.Fatalf("unknown probe times must be absent from JSON, got %s", wire)
	}
}

func TestAvailabilityDataJSONIncludesKnownProbeTimes(t *testing.T) {
	checkedAt := time.Date(2026, time.July, 9, 12, 0, 0, 0, time.UTC)
	payload, err := json.Marshal(AvailabilityData{
		TargetID:    "router",
		LastChecked: &checkedAt,
		LastSuccess: &checkedAt,
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	wire := string(payload)
	if !strings.Contains(wire, "lastChecked") || !strings.Contains(wire, "lastSuccess") {
		t.Fatalf("known probe times must be present in JSON, got %s", wire)
	}
}
