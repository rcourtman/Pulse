package api

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	"github.com/stretchr/testify/require"
)

func TestBackupHealthRecoveryStoreSurvivesMonitorReplacement(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	manager := recoverymanager.New(persistence)
	router := &Router{config: &config.Config{}, recoveryHandlers: NewRecoveryHandlers(manager)}
	assertBound := func(m *monitoring.Monitor) {
		t.Helper()
		// Observe the dependency without adding a production getter solely for
		// a wiring test. The live reporting proof also exercises actual writes.
		bound := reflect.ValueOf(m).Elem().FieldByName("recoveryManager").Pointer()
		require.Equal(t, reflect.ValueOf(manager).Pointer(), bound)
	}
	standalone := newReportingMonitorForTest(t, models.NewState(), nil)
	router.SetMonitor(standalone)
	assertBound(standalone)

	firstMonitor := newReportingMonitorForTest(t, models.NewState(), nil)
	first := newReportingMTMForTest(t, firstMonitor)
	router.SetMultiTenantMonitor(first)
	m, err := first.GetMonitor("default")
	require.NoError(t, err)
	assertBound(m)

	replacementMonitor := newReportingMonitorForTest(t, models.NewState(), nil)
	replacement := newReportingMTMForTest(t, replacementMonitor)
	router.SetMultiTenantMonitor(replacement)
	m, err = replacement.GetMonitor("default")
	require.NoError(t, err)
	assertBound(m)
}
