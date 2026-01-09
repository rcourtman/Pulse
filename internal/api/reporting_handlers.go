package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// ReportingHandlers handles reporting-related requests
type ReportingHandlers struct{}

// NewReportingHandlers creates a new ReportingHandlers
func NewReportingHandlers() *ReportingHandlers {
	return &ReportingHandlers{}
}

// HandleGenerateReport generates a report
func (h *ReportingHandlers) HandleGenerateReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	engine := reporting.GetEngine()
	if engine == nil {
		http.Error(w, "Reporting engine not initialized", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	format := reporting.ReportFormat(q.Get("format"))
	if format == "" {
		format = reporting.FormatPDF
	}

	resourceType := q.Get("resourceType")
	resourceID := q.Get("resourceId")
	if resourceType == "" || resourceID == "" {
		http.Error(w, "resourceType and resourceId are required", http.StatusBadRequest)
		return
	}

	metricType := q.Get("metricType")

	// Parse range
	end := time.Now()
	if q.Get("end") != "" {
		if t, err := time.Parse(time.RFC3339, q.Get("end")); err == nil {
			end = t
		}
	}

	start := end.Add(-24 * time.Hour)
	if q.Get("start") != "" {
		if t, err := time.Parse(time.RFC3339, q.Get("start")); err == nil {
			start = t
		}
	}

	req := reporting.MetricReportRequest{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		MetricType:   metricType,
		Start:        start,
		End:          end,
		Format:       format,
		Title:        q.Get("title"),
	}

	data, contentType, err := engine.Generate(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate report: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	// Suggest a filename
	filename := fmt.Sprintf("report-%s-%s.%s", resourceID, time.Now().Format("20060102"), format)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Write(data)
}
