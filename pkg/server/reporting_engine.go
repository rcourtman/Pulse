package server

import (
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// newOrgScopedReportEngine builds the standard report engine. Each report
// reads the metrics store of the organization it was requested in, resolved
// per request through the current multi-tenant monitor so it follows monitor
// reloads (which close and recreate the stores). The engine refuses a request
// that names no organization rather than falling back to the default one.
func newOrgScopedReportEngine(multiTenantMonitor func() *monitoring.MultiTenantMonitor) *reporting.ReportEngine {
	return reporting.NewReportEngine(reporting.EngineConfig{
		MetricsStoreForOrg: func(orgID string) (*metrics.Store, error) {
			mtMonitor := multiTenantMonitor()
			if mtMonitor == nil {
				return nil, fmt.Errorf("monitoring is not running")
			}
			monitor, err := mtMonitor.GetMonitor(orgID)
			if err != nil {
				return nil, err
			}
			return monitor.GetMetricsStore(), nil
		},
	})
}
