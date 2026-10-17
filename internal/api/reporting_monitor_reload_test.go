package api

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReportingMonitorReloadUsesReplacementForInventoryAndSchedules(t *testing.T) {
	original, originalPersistence, ctx, _, _ := inventoryScheduleFixture(t)
	replacement, replacementPersistence, _, _, setVMCount := inventoryScheduleFixture(t)
	setVMCount(62)
	router := &Router{reportingHandlers: original}
	router.SetMultiTenantMonitor(replacement.reportMonitorManager())

	snapshot, ok := original.getReportingEnrichmentSnapshot(ctx, "default")
	require.True(t, ok)
	require.Len(t, buildVMInventoryRows(snapshot.Resources), 62)
	persistence, err := original.reportSchedulePersistence(ctx)
	require.NoError(t, err)
	require.Same(t, replacementPersistence, persistence)
	require.NotSame(t, originalPersistence, persistence)

	// Clearing the runtime never falls back to the stopped manager.
	router.SetMultiTenantMonitor(nil)
	_, ok = original.getReportingEnrichmentSnapshot(ctx, "default")
	require.False(t, ok)
	_, err = original.reportSchedulePersistence(ctx)
	require.Error(t, err)
}

func TestReportingMonitorReloadCanOverlapReportingReads(t *testing.T) {
	h, _, _, _, _ := inventoryScheduleFixture(t)
	manager := h.reportMonitorManager()
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		for range 100 {
			h.getReportingEnrichmentSnapshot(context.Background(), "default")
		}
	}()
	for range 100 {
		h.SetMultiTenantMonitor(nil)
		h.SetMultiTenantMonitor(manager)
	}
	readers.Wait()
}
