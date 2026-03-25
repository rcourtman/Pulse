package reporting

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"time"
)

// VMInventoryData captures the current-state VM inventory export payload.
type VMInventoryData struct {
	GeneratedAt time.Time
	Rows        []VMInventoryRow
}

// VMInventoryRow captures a single VM's current inventory state.
type VMInventoryRow struct {
	ResourceID           string
	Instance             string
	Node                 string
	Pool                 string
	VMID                 int
	Name                 string
	Status               string
	CPUCores             int
	MemoryAllocatedBytes int64
	DiskAllocatedBytes   int64
	DiskUsedBytes        int64
	DiskStatusReason     string
}

var vmInventoryCSVHeaders = []string{
	"Resource ID",
	"Instance",
	"Node",
	"Pool",
	"VMID",
	"VM Name",
	"Status",
	"CPU Cores",
	"Memory Allocated Bytes",
	"Disk Allocated Bytes",
	"Disk Used Bytes",
	"Disk Status Reason",
}

// GenerateVMInventoryCSV renders a spreadsheet-friendly CSV export for the
// current VM inventory state.
func GenerateVMInventoryCSV(data VMInventoryData) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if err := w.Write(vmInventoryCSVHeaders); err != nil {
		return nil, fmt.Errorf("write VM inventory CSV header: %w", err)
	}

	rows := append([]VMInventoryRow(nil), data.Rows...)
	sort.SliceStable(rows, func(i, j int) bool {
		left := rows[i]
		right := rows[j]
		switch {
		case left.Instance != right.Instance:
			return left.Instance < right.Instance
		case left.Node != right.Node:
			return left.Node < right.Node
		case left.Name != right.Name:
			return left.Name < right.Name
		case left.VMID != right.VMID:
			return left.VMID < right.VMID
		default:
			return left.ResourceID < right.ResourceID
		}
	})

	for _, row := range rows {
		record := []string{
			row.ResourceID,
			row.Instance,
			row.Node,
			row.Pool,
			strconv.Itoa(row.VMID),
			row.Name,
			row.Status,
			strconv.Itoa(row.CPUCores),
			strconv.FormatInt(row.MemoryAllocatedBytes, 10),
			strconv.FormatInt(row.DiskAllocatedBytes, 10),
			strconv.FormatInt(row.DiskUsedBytes, 10),
			row.DiskStatusReason,
		}
		if err := w.Write(record); err != nil {
			return nil, fmt.Errorf("write VM inventory CSV row for %q: %w", row.ResourceID, err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flush VM inventory CSV: %w", err)
	}

	return buf.Bytes(), nil
}
