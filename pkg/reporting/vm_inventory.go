package reporting

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"time"
)

// InventoryExportColumnDefinition describes one stable column in a current-state
// inventory export contract.
type InventoryExportColumnDefinition struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// VMInventoryExportDefinition describes the operator-facing contract for the VM
// inventory export surface.
type VMInventoryExportDefinition struct {
	ID             string                            `json:"id"`
	Title          string                            `json:"title"`
	Description    string                            `json:"description"`
	Format         ReportFormat                      `json:"format"`
	ExportEndpoint string                            `json:"exportEndpoint"`
	FilenamePrefix string                            `json:"filenamePrefix"`
	Columns        []InventoryExportColumnDefinition `json:"columns"`
}

// SupportsFormat reports whether the inventory export allows the given output
// format.
func (d VMInventoryExportDefinition) SupportsFormat(format ReportFormat) bool {
	return format == d.Format
}

// InvalidFormatError returns the canonical validation message for unsupported
// VM inventory formats.
func (d VMInventoryExportDefinition) InvalidFormatError() string {
	return invalidFormatErrorMessage([]ReportFormat{d.Format})
}

// AttachmentFilename returns the canonical attachment filename for a VM
// inventory export.
func (d VMInventoryExportDefinition) AttachmentFilename(generatedAt time.Time) string {
	return fmt.Sprintf("%s-%s.%s", d.FilenamePrefix, reportingDateStamp(generatedAt), d.Format)
}

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

// DescribeVMInventoryExport returns the canonical operator-facing definition for
// the current-state VM inventory CSV surface.
func DescribeVMInventoryExport() VMInventoryExportDefinition {
	return VMInventoryExportDefinition{
		ID:             "vm_inventory",
		Title:          "VM Inventory Export",
		Description:    "Export the current fleet-wide VM inventory as CSV using the canonical runtime model. Includes VM identity, placement, CPU, memory allocation, disk allocation, and disk usage columns.",
		Format:         FormatCSV,
		ExportEndpoint: "/api/admin/reports/inventory/vms/export",
		FilenamePrefix: "vm-inventory",
		Columns: []InventoryExportColumnDefinition{
			{Key: "resource_id", Label: "Resource ID", Description: "Canonical Pulse resource ID for the VM."},
			{Key: "instance", Label: "Instance", Description: "Configured Proxmox instance or cluster name."},
			{Key: "node", Label: "Node", Description: "Proxmox node currently hosting the VM."},
			{Key: "pool", Label: "Pool", Description: "Canonical Proxmox pool membership when the platform reports one."},
			{Key: "vmid", Label: "VMID", Description: "Numeric Proxmox VM identifier."},
			{Key: "vm_name", Label: "VM Name", Description: "Current VM display name from the runtime model."},
			{Key: "status", Label: "Status", Description: "Canonical runtime status for the VM."},
			{Key: "cpu_cores", Label: "CPU Cores", Description: "Allocated virtual CPU core count."},
			{Key: "memory_allocated_bytes", Label: "Memory Allocated Bytes", Description: "Configured memory allocation in bytes."},
			{Key: "disk_allocated_bytes", Label: "Disk Allocated Bytes", Description: "Total allocated disk capacity in bytes across the VM."},
			{Key: "disk_used_bytes", Label: "Disk Used Bytes", Description: "Current used disk bytes from the canonical runtime disk view."},
			{Key: "disk_status_reason", Label: "Disk Status Reason", Description: "Reason disk usage is partial or unavailable when the runtime cannot provide a full guest view."},
		},
	}
}

func vmInventoryCSVHeaders() []string {
	definition := DescribeVMInventoryExport()
	headers := make([]string, 0, len(definition.Columns))
	for _, column := range definition.Columns {
		headers = append(headers, column.Label)
	}
	return headers
}

// GenerateVMInventoryCSV renders a spreadsheet-friendly CSV export for the
// current VM inventory state.
func GenerateVMInventoryCSV(data VMInventoryData) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if err := w.Write(vmInventoryCSVHeaders()); err != nil {
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
