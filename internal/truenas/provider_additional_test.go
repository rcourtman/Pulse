package truenas

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestRecordsParentPoolFallsBackToDatasetName(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() { SetFeatureEnabled(previous) })

	provider := NewProvider(FixtureSnapshot{
		CollectedAt: time.Unix(1707400000, 0).UTC(),
		System: SystemInfo{
			Hostname: "truenas-main",
			Healthy:  true,
		},
		Pools: []Pool{{
			ID:         "1",
			Name:       "tank",
			Status:     "ONLINE",
			TotalBytes: 1000,
			UsedBytes:  300,
			FreeBytes:  700,
		}},
		Datasets: []Dataset{{
			ID:         "tank/apps",
			Name:       "tank/apps",
			Pool:       "",
			UsedBytes:  120,
			AvailBytes: 80,
			Mounted:    true,
			ReadOnly:   false,
		}},
	})

	records := provider.Records()
	if len(records) == 0 {
		t.Fatal("expected records")
	}

	for _, record := range records {
		if record.Resource.Type != unifiedresources.ResourceTypeStorage {
			continue
		}
		if record.Resource.Name != "tank/apps" {
			continue
		}
		if record.ParentSourceID != "system:truenas-main/pool:tank" {
			t.Fatalf("expected dataset parent source id system:truenas-main/pool:tank, got %q", record.ParentSourceID)
		}
		return
	}

	t.Fatal("dataset record tank/apps not found")
}

func TestStatusFromSystemAndParseBoolBranches(t *testing.T) {
	if got := statusFromSystem(SystemInfo{Healthy: true}); got != unifiedresources.StatusOnline {
		t.Fatalf("statusFromSystem(healthy) = %q, want %q", got, unifiedresources.StatusOnline)
	}
	if got := statusFromSystem(SystemInfo{Healthy: false}); got != unifiedresources.StatusWarning {
		t.Fatalf("statusFromSystem(unhealthy) = %q, want %q", got, unifiedresources.StatusWarning)
	}

	if !parseFeatureEnabled("") {
		t.Fatal("expected parseFeatureEnabled to default empty values to true")
	}
	if !parseFeatureEnabled(" \t ") {
		t.Fatal("expected parseFeatureEnabled to default blank values to true")
	}
	if parseFeatureEnabled("off") {
		t.Fatal("expected parseFeatureEnabled to treat explicit off as false")
	}
	if !parseBool("  YeS ") {
		t.Fatal("expected parseBool to treat yes as true")
	}
	if parseBool("off") {
		t.Fatal("expected parseBool to treat off as false")
	}
}

func TestSystemStatusPromotesHealthySystemWhenStorageRiskExists(t *testing.T) {
	risk := &unifiedresources.StorageRisk{Level: "warning"}
	if got := systemStatus(SystemInfo{Healthy: true}, risk, nil); got != unifiedresources.StatusWarning {
		t.Fatalf("systemStatus(healthy, warning risk) = %q, want %q", got, unifiedresources.StatusWarning)
	}
	if got := systemStatus(SystemInfo{Healthy: false}, risk, nil); got != unifiedresources.StatusWarning {
		t.Fatalf("systemStatus(unhealthy, warning risk) = %q, want %q", got, unifiedresources.StatusWarning)
	}
	if got := systemStatus(SystemInfo{Healthy: true}, nil, nil); got != unifiedresources.StatusOnline {
		t.Fatalf("systemStatus(healthy, nil risk) = %q, want %q", got, unifiedresources.StatusOnline)
	}
}

// API-backed TrueNAS systems have no pulse-agent SMART sweep, so the provider
// must surface API-reported disk temperatures as SMART sensor entries for the
// host Thermals card (#1573).
func TestSensorMetaIncludesDiskTemperatures(t *testing.T) {
	system := SystemInfo{
		TemperatureCelsius: map[string]float64{"cpu_package": 55},
	}
	disks := []Disk{
		{Name: "ada0", Model: "WDC", Serial: "S1", Transport: "ata", SizeBytes: 100, Temperature: 34, Pool: "tank", Status: "ONLINE"},
		{Name: "ada1", Model: "WDC", Serial: "S2", Transport: "ata", SizeBytes: 100, Temperature: 0, Pool: "tank"},
	}

	sensors := sensorMetaFromTrueNASSystem(system, disks)
	if sensors == nil {
		t.Fatal("expected sensors")
	}
	if len(sensors.SMART) != 1 {
		t.Fatalf("expected only disks with a reading as SMART entries, got %+v", sensors.SMART)
	}
	entry := sensors.SMART[0]
	if entry.Device != "ada0" || entry.Temperature != 34 || entry.Pool != "tank" || entry.Health != "PASSED" {
		t.Fatalf("unexpected SMART entry: %+v", entry)
	}

	// Disk-only systems (no CPU sensor payload) still get a sensors object.
	diskOnly := sensorMetaFromTrueNASSystem(SystemInfo{}, disks)
	if diskOnly == nil || len(diskOnly.SMART) != 1 || len(diskOnly.TemperatureCelsius) != 0 {
		t.Fatalf("expected disk-only sensors, got %+v", diskOnly)
	}

	if sensorMetaFromTrueNASSystem(SystemInfo{}, nil) != nil {
		t.Fatal("expected nil sensors when there is nothing to report")
	}
}
