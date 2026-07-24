package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMetricPoint_ZeroValue(t *testing.T) {
	var point MetricPoint

	// Zero value should have zero time
	if !point.Timestamp.IsZero() {
		t.Error("Zero MetricPoint should have zero timestamp")
	}
	if point.Value != 0 {
		t.Errorf("Zero MetricPoint Value = %v, want 0", point.Value)
	}
}

func TestIOMetrics_Fields(t *testing.T) {
	now := time.Now()
	metrics := IOMetrics{
		DiskRead:   1000,
		DiskWrite:  2000,
		NetworkIn:  3000,
		NetworkOut: 4000,
		Timestamp:  now,
		ObservedAt: IOCounterObservationTimes{DiskRead: now},
	}

	if metrics.DiskRead != 1000 {
		t.Errorf("DiskRead = %v, want 1000", metrics.DiskRead)
	}
	if metrics.DiskWrite != 2000 {
		t.Errorf("DiskWrite = %v, want 2000", metrics.DiskWrite)
	}
	if metrics.NetworkIn != 3000 {
		t.Errorf("NetworkIn = %v, want 3000", metrics.NetworkIn)
	}
	if metrics.NetworkOut != 4000 {
		t.Errorf("NetworkOut = %v, want 4000", metrics.NetworkOut)
	}
	if !metrics.Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", metrics.Timestamp, now)
	}
}

func TestIOMetrics_ZeroValue(t *testing.T) {
	var metrics IOMetrics

	if metrics.DiskRead != 0 {
		t.Error("Zero IOMetrics should have zero DiskRead")
	}
	if metrics.DiskWrite != 0 {
		t.Error("Zero IOMetrics should have zero DiskWrite")
	}
	if metrics.NetworkIn != 0 {
		t.Error("Zero IOMetrics should have zero NetworkIn")
	}
	if metrics.NetworkOut != 0 {
		t.Error("Zero IOMetrics should have zero NetworkOut")
	}
	if !metrics.Timestamp.IsZero() {
		t.Error("Zero IOMetrics should have zero Timestamp")
	}
}

func TestIOMetrics_JSONSerializationUsesCamelCaseFields(t *testing.T) {
	now := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	metrics := IOMetrics{
		DiskRead:   1000,
		DiskWrite:  2000,
		NetworkIn:  3000,
		NetworkOut: 4000,
		Timestamp:  now,
	}

	payload, err := json.Marshal(metrics)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded["diskRead"] != float64(1000) {
		t.Errorf("diskRead = %v, want 1000", decoded["diskRead"])
	}
	if decoded["diskWrite"] != float64(2000) {
		t.Errorf("diskWrite = %v, want 2000", decoded["diskWrite"])
	}
	if decoded["networkIn"] != float64(3000) {
		t.Errorf("networkIn = %v, want 3000", decoded["networkIn"])
	}
	if decoded["networkOut"] != float64(4000) {
		t.Errorf("networkOut = %v, want 4000", decoded["networkOut"])
	}
	if decoded["timestamp"] != now.Format(time.RFC3339Nano) {
		t.Errorf("timestamp = %v, want %v", decoded["timestamp"], now.Format(time.RFC3339Nano))
	}

	if _, ok := decoded["DiskRead"]; ok {
		t.Error("expected DiskRead key to be absent")
	}
	if _, ok := decoded["DiskWrite"]; ok {
		t.Error("expected DiskWrite key to be absent")
	}
	if _, ok := decoded["NetworkIn"]; ok {
		t.Error("expected NetworkIn key to be absent")
	}
	if _, ok := decoded["NetworkOut"]; ok {
		t.Error("expected NetworkOut key to be absent")
	}
	if _, ok := decoded["Timestamp"]; ok {
		t.Error("expected Timestamp key to be absent")
	}
	if _, ok := decoded["ObservedAt"]; ok {
		t.Error("expected per-counter observation times to be absent")
	}
}

func TestIORateValidityLegacyFallbackTreatsOnlyNonZeroRatesAsKnown(t *testing.T) {
	validity := (IORateValidity{}).EffectiveForRates(1024, 0, 2048, 0)
	if !validity.DiskRead || validity.DiskWrite || !validity.NetworkIn || validity.NetworkOut {
		t.Fatalf("legacy inferred validity = %+v", validity)
	}

	explicitIdle := IORateValidity{
		Explicit:   true,
		DiskRead:   true,
		DiskWrite:  true,
		NetworkIn:  true,
		NetworkOut: true,
	}.EffectiveForRates(0, 0, 0, 0)
	if !explicitIdle.DiskRead || !explicitIdle.DiskWrite || !explicitIdle.NetworkIn || !explicitIdle.NetworkOut {
		t.Fatalf("explicit idle validity was not preserved: %+v", explicitIdle)
	}
}

func TestUnknownGuestRatesRemainNumericOnAPIAndWebsocketShapes(t *testing.T) {
	vm := VM{
		ID:        "site:node:100",
		DiskRead:  0,
		DiskWrite: 0,
		IORateValidity: IORateValidity{
			Explicit: true,
		},
	}
	modelPayload, err := json.Marshal(vm)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(modelPayload), "IORateValidity") || strings.Contains(string(modelPayload), "ioRateValidity") {
		t.Fatalf("internal validity leaked into guest model JSON: %s", modelPayload)
	}

	payload, err := json.Marshal(vm.ToFrontend())
	if err != nil {
		t.Fatal(err)
	}
	wire := string(payload)
	for _, field := range []string{`"diskRead":0`, `"diskWrite":0`, `"networkIn":0`, `"networkOut":0`} {
		if !strings.Contains(wire, field) {
			t.Fatalf("wire payload %s does not contain numeric field %s", wire, field)
		}
	}
	if strings.Contains(wire, "null") {
		t.Fatalf("guest wire payload contains unstable null: %s", wire)
	}
	if strings.Contains(wire, "IORateValidity") {
		t.Fatalf("internal validity leaked into wire payload: %s", wire)
	}
}

func TestNativePoolHealthEvidenceNormalizesWithoutChangingIdentity(t *testing.T) {
	pool := (ZFSPool{
		Name:  "tank",
		State: "DEGRADED",
		ScanDetails: &ZFSScan{
			Function:   "RESILVER",
			State:      "SCANNING",
			Percentage: 42.5,
		},
		Devices: []ZFSDevice{{
			GUID:    "leaf-guid",
			Path:    "/dev/disk/by-partuuid/example",
			State:   "UNAVAIL",
			Missing: true,
		}},
	}).NormalizeCollections()
	if pool.Name != "tank" || pool.ScanDetails == nil || len(pool.Devices) != 1 {
		t.Fatalf("normalized ZFS evidence = %+v", pool)
	}

	cluster := (CephCluster{
		FSID:   "cluster-fsid",
		Health: "HEALTH_WARN",
		HealthChecks: []CephHealthCheck{{
			Code:     "OSD_DOWN",
			Severity: "HEALTH_WARN",
			Summary:  "one OSD is down",
		}},
	}).NormalizeCollections()
	if cluster.FSID != "cluster-fsid" || len(cluster.HealthChecks) != 1 || cluster.HealthChecks[0].Code != "OSD_DOWN" {
		t.Fatalf("normalized Ceph evidence = %+v", cluster)
	}
}
