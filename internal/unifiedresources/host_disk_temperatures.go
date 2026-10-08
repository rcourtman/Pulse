package unifiedresources

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
	"github.com/rcourtman/pulse-go-rewrite/pkg/fsfilters"
)

// HostDiskTemperatureReading is the temperature one physical disk of a host
// agent shows, taken from the disk resource the registry builds for it.
type HostDiskTemperatureReading struct {
	// Device names the disk on the host: a SMART row's device label, or the
	// kernel block device token of an Unraid inventory row.
	Device string
	// MetricID is the disk's history key (HostSMARTDiskMetricID or
	// HostUnraidDiskMetricID).
	MetricID    string
	Model       string
	DiskType    string
	Temperature int
	Collection  *diskinventory.CollectionStatus
	// Standby reports a SMART row the agent marked as in standby.
	Standby bool
	// UnraidOnly reports a disk built from the host's Unraid inventory row
	// alone, because no SMART row of the host carries its disk key.
	UnraidOnly bool
}

// Collected reports whether the reading is a temperature collected now: not
// a disk in standby, and not a reading its collection state marks as kept
// from earlier (diskinventory.TemperatureCollected).
func (r HostDiskTemperatureReading) Collected() bool {
	return !r.Standby && diskinventory.TemperatureCollected(r.Temperature, r.Collection)
}

// HostDiskTemperatureReadings returns the temperature each physical disk of a
// host agent shows, in report order: one reading per SMART row the registry
// ingests, then one per Unraid inventory row whose disk key no SMART row of
// the host carries. The registry merges a SMART row and an Unraid row with
// one key into one disk whatever device labels they use, as a controller
// member ("0 [megaraid,0]") and its Unraid device ("sda") do. A SMART row
// without a reading of its own shows the one its Unraid row reports
// (HostSMARTDiskTemperature), and a disk only the Unraid inventory lists, such
// as a --disk-exclude match, shows that row's (HostUnraidDiskTemperature).
// When smartctl and Unraid report different usable serials for one device the
// registry shows both disks, so both are listed. The readings come from the
// adapters the registry builds the disks with, so a consumer judging them
// judges the temperature, collection state and disk type each row's disk
// shows.
func HostDiskTemperatureReadings(host models.Host) []HostDiskTemperatureReading {
	readings := make([]HostDiskTemperatureReading, 0, len(host.Sensors.SMART))
	smartKeys := make(map[string]struct{}, len(host.Sensors.SMART))
	for _, disk := range host.Sensors.SMART {
		// The registry ingests no disk for a virtual block device.
		if fsfilters.IsVirtualBlockDevice(disk.Device) {
			continue
		}
		key := HostSMARTDiskMetricID(host, disk)
		if key != "" {
			smartKeys[key] = struct{}{}
		}
		resource, _ := resourceFromHostSMARTDisk(host, disk)
		reading := hostDiskTemperatureReading(disk.Device, key, resource)
		reading.Standby = disk.Standby
		readings = append(readings, reading)
	}
	if host.Unraid == nil {
		return readings
	}
	for _, disk := range host.Unraid.Disks {
		// HostUnraidDiskMetricID is empty for a row without a device, which
		// the registry ingests no disk for.
		key := HostUnraidDiskMetricID(host, disk)
		if key == "" {
			continue
		}
		if _, ok := smartKeys[key]; ok {
			continue
		}
		resource, _ := resourceFromHostUnraidPhysicalDisk(host, disk)
		reading := hostDiskTemperatureReading(normalizePhysicalDiskDeviceToken(disk.Device), key, resource)
		reading.UnraidOnly = true
		readings = append(readings, reading)
	}
	return readings
}

func hostDiskTemperatureReading(device, metricID string, resource Resource) HostDiskTemperatureReading {
	reading := HostDiskTemperatureReading{Device: device, MetricID: metricID}
	if disk := resource.PhysicalDisk; disk != nil {
		reading.Model = disk.Model
		reading.DiskType = disk.DiskType
		reading.Temperature = disk.Temperature
		reading.Collection = diskinventory.CloneStatus(disk.Collection)
	}
	return reading
}
