package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
	"github.com/rs/zerolog/log"
)

type workloadHistoryActivityRequest struct {
	Activity string `json:"activity"`
}

// HandleWorkloadHistoryActivity accepts one closed, content-free adoption
// milestone. The browser deduplicates each value per session; this boundary
// rejects everything except the four declared values and persists only counts.
func (r *Router) HandleWorkloadHistoryActivity(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload workloadHistoryActivityRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	var discard json.RawMessage
	if err := dec.Decode(&discard); err != io.EOF {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	activity := strings.TrimSpace(payload.Activity)
	switch activity {
	case config.WorkloadHistoryActivityPreview,
		config.WorkloadHistoryActivityScrub,
		config.WorkloadHistoryActivityRangeChange,
		config.WorkloadHistoryActivityDetailsSelected:
	default:
		http.Error(w, "Unknown workload history activity", http.StatusBadRequest)
		return
	}

	persistence := r.persistenceForOrg(req.Context())
	if persistence != nil {
		if err := persistence.RecordWorkloadHistoryActivity(activity, time.Now().UTC()); err != nil {
			log.Debug().Err(err).Str("activity", activity).Msg("Failed to record workload history activity")
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ApplyWorkloadHistoryTelemetrySnapshot aggregates the bounded, content-free
// counters across local organizations for the existing daily telemetry ping.
func (r *Router) ApplyWorkloadHistoryTelemetrySnapshot(s *telemetry.Snapshot, now time.Time) {
	if r == nil || s == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	since := now.UTC().Add(-telemetry.PulseIntelligenceTelemetryWindow)
	for _, orgID := range r.pulseIntelligenceTelemetryOrgIDs() {
		persistence := r.persistence
		if orgID != "default" && r.multiTenant != nil {
			var err error
			persistence, err = r.multiTenant.GetPersistence(orgID)
			if err != nil {
				continue
			}
		}
		if persistence == nil {
			continue
		}
		tally, err := persistence.LoadWorkloadHistoryActivityTally()
		if err != nil || tally == nil {
			continue
		}
		s.WorkloadHistoryPreviewSessions30d += tally.PreviewsSince(since)
		s.WorkloadHistoryScrubSessions30d += tally.ScrubsSince(since)
		s.WorkloadHistoryRangeChangeSessions30d += tally.RangeChangesSince(since)
		s.WorkloadHistoryDetailsSelectionSessions30d += tally.DetailsSelectedSince(since)
	}
}
