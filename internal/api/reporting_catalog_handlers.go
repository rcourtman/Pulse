package api

import (
	"encoding/json"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// HandleGetReportingCatalog returns the canonical operator-facing reporting
// catalog for the admin settings surface.
func (h *ReportingHandlers) HandleGetReportingCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(reporting.DescribeReportingCatalog())
}
