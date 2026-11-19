package truenas

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
)

// Keep each selected disk's valid samples even when another selected graph is
// missing. Host totals still require every selected member at the same instant.
// An unscoped compatibility graph cannot identify any physical disk.
func reportingDiskRates(devices map[string][]TimeSeriesPoint) map[string][]TimeSeriesPoint {
	rates := make(map[string][]TimeSeriesPoint)
	for key, points := range devices {
		identifier := strings.TrimPrefix(key, "disk\x00")
		if identifier != "" && len(points) > 0 {
			rates[identifier] = cloneTimeSeriesPoints(points)
		}
	}
	return rates
}

func latestDeviceRates(series map[string][]TimeSeriesPoint) map[string]TimeSeriesPoint {
	rates := make(map[string]TimeSeriesPoint, len(series))
	for device, points := range series {
		var latest TimeSeriesPoint
		for _, point := range points {
			if point.Timestamp.After(latest.Timestamp) && point.Value >= 0 && !math.IsNaN(point.Value) && !math.IsInf(point.Value, 0) {
				latest = point
			}
		}
		if !latest.Timestamp.IsZero() {
			rates[device] = latest
		}
	}
	return rates
}

func cloneDeviceRates(rates map[string]TimeSeriesPoint) map[string]TimeSeriesPoint {
	if rates == nil {
		return nil
	}
	cloned := make(map[string]TimeSeriesPoint, len(rates))
	for device, point := range rates {
		cloned[device] = point
	}
	return cloned
}

func metricsFromTrueNASDisk(system SystemInfo, disk Disk) *unifiedresources.ResourceMetrics {
	// CORE's reporting catalogue keys disks by the native device name. Do not
	// guess with serials, pool names, disk IDs, suffixes or a different connection.
	device := strings.TrimSpace(disk.Name)
	if device == "" {
		return nil
	}
	metric := func(rates map[string]TimeSeriesPoint) *unifiedresources.MetricValue {
		point, ok := rates[device]
		if !ok || point.Timestamp.IsZero() || point.Value < 0 || math.IsNaN(point.Value) || math.IsInf(point.Value, 0) {
			return nil
		}
		return &unifiedresources.MetricValue{Value: point.Value, Unit: "bytes/s", Source: unifiedresources.SourceTrueNAS, ObservedAt: point.Timestamp}
	}
	read, write := metric(system.DiskReadRates), metric(system.DiskWriteRates)
	if read == nil && write == nil {
		return nil
	}
	return &unifiedresources.ResourceMetrics{DiskRead: read, DiskWrite: write}
}

func reportingDiskRead(values map[string]float64) (float64, bool) {
	value, ok := pickReportingValue(values, "read", "read_bytes", "diskread", "bytes_read", "disk_octets_read")
	return value, ok && value >= 0
}

func reportingDiskWrite(values map[string]float64) (float64, bool) {
	value, ok := pickReportingValue(values, "write", "write_bytes", "diskwrite", "bytes_written", "disk_octets_write")
	return value, ok && value >= 0
}

// The native CORE resolution is normally 10 seconds. Bound the transient
// window for denser or malformed replies; never round the sample timestamp.
const coreDiskRateCellSeconds = 10
const coreDiskRateCellCount = legacyRESTTelemetryWindowSeconds/coreDiskRateCellSeconds + 1

type coreDiskRateFrame struct {
	at                time.Time
	read, write       float64
	hasRead, hasWrite bool
}

type coreDiskRateCells [coreDiskRateCellCount]coreDiskRateFrame

func (c *coreDiskRateCells) add(end int64, at time.Time, values map[string]float64) {
	cell := at.Unix()/coreDiskRateCellSeconds - (end-legacyRESTTelemetryWindowSeconds)/coreDiskRateCellSeconds
	if !at.After(c[cell].at) {
		return
	}
	read, hasRead := reportingDiskRead(values)
	write, hasWrite := reportingDiskWrite(values)
	if hasRead || hasWrite {
		c[cell] = coreDiskRateFrame{at: at, read: read, write: write, hasRead: hasRead, hasWrite: hasWrite}
	}
}

// COREHistoryDirection pairs an actual earlier observation with the exact
// live rate it accompanies. This is a History input, not part of a live DTO.
type COREHistoryDirection struct {
	Tail, Earlier TimeSeriesPoint
}

// COREHistoryDisk identifies the inventory generation and canonical target
// that can use the observation. Read and write can be available independently.
type COREHistoryDisk struct {
	TargetID    string
	InventoryAt time.Time
	Read, Write COREHistoryDirection
}

func (p *Provider) COREHistoryDisks() []COREHistoryDisk {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.coreDiskHistory)
}

func coreHistoryObservations(connectionID string, previous, current *FixtureSnapshot) []COREHistoryDisk {
	if len(current.System.coreDiskRateWindow) == 0 || previous == nil || previous.CollectedAt.IsZero() ||
		!current.CollectedAt.After(previous.CollectedAt) || current.System.MachineID != previous.System.MachineID {
		return nil
	}

	// Only the same unique hardware disk at the same native device name in
	// both inventories can use intra-poll history. In particular, a new
	// device or a placeholder serial cannot borrow a previous occupant's I/O.
	prior, currentDisks := coreHistoryInventory(previous.Disks), coreHistoryInventory(current.Disks)
	scope := systemSourceID(connectionID, current.System.Hostname)
	var history []COREHistoryDisk
	for name, disk := range currentDisks {
		if prior[name].Serial != disk.Serial {
			continue
		}
		cells, ok := current.System.coreDiskRateWindow[name]
		if !ok {
			continue
		}
		read, write := distinguishCOREHistory(cells, current.System.DiskReadRates[name], current.System.DiskWriteRates[name], previous.CollectedAt)
		observation := COREHistoryDisk{
			TargetID: trueNASDiskMetricResourceID(scope, disk), InventoryAt: current.CollectedAt,
			Read: read, Write: write,
		}
		if !observation.Read.Earlier.Timestamp.IsZero() || !observation.Write.Earlier.Timestamp.IsZero() {
			history = append(history, observation)
		}
	}
	return history
}

func coreHistoryInventory(disks []Disk) map[string]Disk {
	byName := make(map[string]Disk, len(disks))
	names, serials := make(map[string]int, len(disks)), make(map[string]int, len(disks))
	for _, disk := range disks {
		disk.Name, disk.Serial = strings.TrimSpace(disk.Name), strings.TrimSpace(disk.Serial)
		names[disk.Name]++
		serials[strings.ToLower(disk.Serial)]++
		if disk.Name != "" && diskinventory.IsUsableHardwareID(disk.Serial) {
			byName[disk.Name] = disk
		}
	}
	for name, disk := range byName {
		if names[name] != 1 || serials[strings.ToLower(disk.Serial)] != 1 {
			delete(byName, name)
		}
	}
	return byName
}

// Choose one actual earlier frame, not a different instant for each rate.
// If both directions are available, prefer a complete differing frame and
// include its same-time companion even when that value matches the tail.
// Otherwise the chart reports a manufactured gap in a measured direction.
// We do not fill a genuinely absent rate, or look behind the prior inventory.
func distinguishCOREHistory(cells coreDiskRateCells, readTail, writeTail TimeSeriesPoint, after time.Time) (COREHistoryDirection, COREHistoryDirection) {
	read, write := COREHistoryDirection{Tail: readTail}, COREHistoryDirection{Tail: writeTail}
	var bestDistance float64
	bestSize := 0
	for _, frame := range cells {
		if !frame.at.After(after) {
			continue
		}
		hasRead := frame.hasRead && frame.at.Before(readTail.Timestamp)
		hasWrite := frame.hasWrite && frame.at.Before(writeTail.Timestamp)
		if (frame.hasRead && !readTail.Timestamp.IsZero() && !hasRead) || (frame.hasWrite && !writeTail.Timestamp.IsZero() && !hasWrite) {
			continue // a known companion must not be dropped to make room
		}
		size, distance := 0, 0.0
		if hasRead {
			size++
			distance = math.Abs(frame.read - readTail.Value)
		}
		if hasWrite {
			size++
			distance = max(distance, math.Abs(frame.write-writeTail.Value))
		}
		if distance == 0 || size < bestSize || (size == bestSize && distance < bestDistance) {
			continue
		}
		bestSize, bestDistance = size, distance
		read.Earlier, write.Earlier = TimeSeriesPoint{}, TimeSeriesPoint{}
		if hasRead {
			read.Earlier = TimeSeriesPoint{Timestamp: frame.at, Value: frame.read}
		}
		if hasWrite {
			write.Earlier = TimeSeriesPoint{Timestamp: frame.at, Value: frame.write}
		}
	}
	return read, write
}
