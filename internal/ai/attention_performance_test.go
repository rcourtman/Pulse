package ai

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestAttentionProjectionLargeCardinalityKeepsResponseBounded(t *testing.T) {
	const cardinality = 10_000
	const guests = 2_000
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	// Every alert names a guest by its Proxmox reference, so the projection
	// resolves each subject against a populated inventory.
	snapshot := models.StateSnapshot{VMs: make([]models.VM, guests)}
	for index := range snapshot.VMs {
		vmid := 100 + index
		snapshot.VMs[index] = models.VM{ID: fmt.Sprintf("pve1:node1:%d", vmid), VMID: vmid, Name: fmt.Sprintf("guest-%d", vmid),
			Node: "node1", Instance: "pve1", Status: "running", Type: "qemu", LastSeen: now}
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	active := make([]alerts.Alert, cardinality)
	for index := range active {
		active[index] = attentionTestAlert(
			fmt.Sprintf("record-%05d", index),
			operationaltrust.OperationalOpen,
			operationaltrust.SeverityWarning,
			now.Add(-time.Duration(index)*time.Second),
			now,
		)
		reference := snapshot.VMs[index%guests].ID
		active[index].ResourceID = reference
		active[index].OperationalRecord.SubjectResourceID = reference
	}

	startedAt := time.Now()
	projection := ProjectAttentionItems(active, nil, nil, registry, now)
	elapsed := time.Since(startedAt)
	if projection.Summary.ActiveCount != cardinality {
		t.Fatalf(
			"ActiveCount = %d, want %d",
			projection.Summary.ActiveCount,
			cardinality,
		)
	}
	for _, detail := range projection.Details {
		if !strings.HasPrefix(detail.Item.SubjectResourceID, "vm-") {
			t.Fatalf("subject %q was not resolved to its unified guest", detail.Item.SubjectResourceID)
		}
	}
	page, err := PaginateAttentionDetails(projection.Details, 1, MaxAttentionPageSize)
	if err != nil {
		t.Fatalf("PaginateAttentionDetails(): %v", err)
	}
	if len(page) != MaxAttentionPageSize {
		t.Fatalf("page size = %d, want %d", len(page), MaxAttentionPageSize)
	}
	// This is deliberately generous enough for shared CI while still catching
	// accidental quadratic projection or an external request per item.
	if elapsed > 5*time.Second {
		t.Fatalf("10k attention projection took %s, want <= 5s", elapsed)
	}
}
