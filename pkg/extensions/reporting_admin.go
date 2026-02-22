package extensions

import (
	"context"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// ReportingAdminEndpoints defines the enterprise reporting admin endpoint surface.
type ReportingAdminEndpoints interface {
	HandleGenerateReport(http.ResponseWriter, *http.Request)
	HandleGenerateMultiReport(http.ResponseWriter, *http.Request)
}

// WriteReportingErrorFunc writes a structured reporting error response.
type WriteReportingErrorFunc func(http.ResponseWriter, int, string, string, map[string]string)

// ReportingAdminRuntime exposes runtime capabilities needed by reporting admin endpoints.
type ReportingAdminRuntime struct {
	GetEngine           func() reporting.Engine
	GetRequestOrgID     func(context.Context) string
	EnrichReportRequest func(context.Context, string, *reporting.MetricReportRequest, time.Time, time.Time)
	SanitizeFilename    func(string) string
	WriteError          WriteReportingErrorFunc
}

// BindReportingAdminEndpointsFunc allows enterprise modules to bind replacement
// reporting admin endpoints while retaining access to default handlers.
type BindReportingAdminEndpointsFunc func(defaults ReportingAdminEndpoints, runtime ReportingAdminRuntime) ReportingAdminEndpoints
