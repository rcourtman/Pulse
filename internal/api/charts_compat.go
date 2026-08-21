package api

import (
	"context"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/chartapi"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

type routerChartMonitorResolver struct {
	router *Router
}

func (r routerChartMonitorResolver) MonitorForContext(ctx context.Context) *monitoring.Monitor {
	if r.router == nil {
		return nil
	}
	return r.router.getTenantMonitor(ctx)
}

func (r *Router) ensureChartService() *chartapi.Service {
	if r.chartService == nil {
		r.chartService = chartapi.NewService(routerChartMonitorResolver{router: r})
	}
	return r.chartService
}

func (r *Router) handleCharts(w http.ResponseWriter, req *http.Request) {
	r.ensureChartService().HandleCharts(w, req)
}

func (r *Router) handleWorkloadCharts(w http.ResponseWriter, req *http.Request) {
	r.ensureChartService().HandleWorkloadCharts(w, req)
}

func (r *Router) handleInfrastructureCharts(w http.ResponseWriter, req *http.Request) {
	r.ensureChartService().HandleInfrastructureCharts(w, req)
}

func (r *Router) handleWorkloadsSummaryCharts(w http.ResponseWriter, req *http.Request) {
	r.ensureChartService().HandleWorkloadsSummaryCharts(w, req)
}

func (r *Router) handleStorageCharts(w http.ResponseWriter, req *http.Request) {
	r.ensureChartService().HandleStorageCharts(w, req)
}

func (r *Router) handleStorageSummaryCharts(w http.ResponseWriter, req *http.Request) {
	r.ensureChartService().HandleStorageSummaryCharts(w, req)
}
