package alerting

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

type historyDurabilityManager interface {
	EventHistoryDurability() eventlog.DurabilityStatus
	AcknowledgeEventHistoryGaps(uint64, string) error
}

func (h *AlertHandlers) historyDurabilityManager(w http.ResponseWriter, r *http.Request) historyDurabilityManager {
	monitor := h.requireMonitor(w, r)
	if monitor == nil {
		return nil
	}
	manager, ok := monitor.GetAlertManager().(historyDurabilityManager)
	if !ok {
		http.Error(w, "event history durability unavailable", http.StatusServiceUnavailable)
		return nil
	}
	return manager
}

// GetHistoryDurability reports completeness separately from the retained rows.
// Read access never acknowledges a gap or treats reopening as recovery.
func (h *AlertHandlers) GetHistoryDurability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	manager := h.historyDurabilityManager(w, r)
	if manager == nil {
		return
	}
	status := manager.EventHistoryDurability()
	if status.Error != "" {
		status.Error = "event history storage or migration is unavailable"
	}
	_ = utils.WriteJSONResponse(w, struct {
		Healthy    bool                      `json:"healthy"`
		Durability eventlog.DurabilityStatus `json:"durability"`
	}{Healthy: status.Healthy(), Durability: status})
}

// AcknowledgeHistoryGaps requires the exact inspected revision and a bounded
// reason. It records acceptance of missing facts; it does not claim recovery,
// delete pending events, or acknowledge any later gap.
func (h *AlertHandlers) AcknowledgeHistoryGaps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	manager := h.historyDurabilityManager(w, r)
	if manager == nil {
		return
	}
	var request struct {
		Revision *uint64 `json:"revision"`
		Reason   string  `json:"reason"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid gap acknowledgement", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "invalid gap acknowledgement", http.StatusBadRequest)
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Revision == nil || request.Reason == "" || len(request.Reason) > 1024 {
		http.Error(w, "revision and a reason of 1 to 1024 bytes are required", http.StatusBadRequest)
		return
	}
	if err := manager.AcknowledgeEventHistoryGaps(*request.Revision, request.Reason); err != nil {
		http.Error(w, "history changed or obligations remain unsettled; inspect durability again", http.StatusConflict)
		return
	}
	_ = utils.WriteJSONResponse(w, map[string]string{"status": "acknowledged", "message": "Missing history facts accepted; no events reconstructed."})
}

// Retained event reads remain available when exact recovery has settled but
// historical completeness is unknown. Existing array responses keep their
// shape and carry an explicit completeness header alongside the status route.
func writeHistoryCompleteness(w http.ResponseWriter, manager AlertManager) {
	source, ok := manager.(historyDurabilityManager)
	if !ok {
		return
	}
	status := source.EventHistoryDurability()
	w.Header().Set("X-Pulse-History-Complete", strconv.FormatBool(status.Healthy()))
	w.Header().Set("X-Pulse-History-Gap-Revision", strconv.FormatUint(status.GapRevision, 10))
}
