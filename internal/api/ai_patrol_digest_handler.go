package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// patrolDigestActionScanLimit bounds the action-audit read behind the digest.
// Patrol-origin actions are a small fraction of the audit table and a week
// of them fits comfortably; the store caps larger requests at 100 anyway.
const patrolDigestActionScanLimit = 500

var patrolDigestActionStates = []unifiedresources.ActionState{
	unifiedresources.ActionStatePlanned,
	unifiedresources.ActionStatePending,
	unifiedresources.ActionStateApproved,
	unifiedresources.ActionStateRejected,
	unifiedresources.ActionStateExpired,
	unifiedresources.ActionStateExecuting,
	unifiedresources.ActionStateCompleted,
	unifiedresources.ActionStateFailed,
}

// HandleGetPatrolDigest returns the "what Patrol did for you" rollup for the
// requested window (GET /api/ai/patrol/digest?days=7). It reads only records
// Pulse already retains; see docs/PATROL_WEEKLY_DIGEST.md.
func (h *AISettingsHandler) HandleGetPatrolDigest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	days := ai.PatrolDigestDefaultDays
	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > ai.PatrolDigestMaxDays {
			writeErrorResponse(
				w,
				http.StatusBadRequest,
				"invalid_patrol_digest_days",
				"Digest window must be between 1 and 30 days.",
				map[string]string{"days": strconv.Itoa(ai.PatrolDigestMaxDays)},
			)
			return
		}
		days = parsed
	}

	input := ai.PatrolDigestInput{
		Now:                time.Now().UTC(),
		Days:               days,
		Mode:               config.PatrolAutonomyMonitor,
		RunHistoryCapacity: ai.MaxPatrolRunHistory,
	}

	if aiService := h.GetAIService(r.Context()); aiService != nil {
		input.Mode = aiService.GetEffectivePatrolAutonomyLevel()
		if patrol := aiService.GetPatrolService(); patrol != nil {
			input.Runs = patrol.GetRunHistory(ai.MaxPatrolRunHistory)
			if store := patrol.GetFindings(); store != nil {
				input.Findings = store.GetAll(nil)
			}
		}
		if costStore := aiService.CostStore(); costStore != nil {
			// One extra day so a window that started mid-day is fully covered;
			// BuildPatrolDigest trims to the exact window.
			input.Usage = costStore.ListEvents(days + 1)
		}
	}

	input.Actions = h.patrolDigestActions(r)

	if err := utils.WriteJSONResponse(w, ai.BuildPatrolDigest(input)); err != nil {
		log.Error().Err(err).Msg("Failed to write patrol digest response")
	}
}

// patrolDigestActions reads the canonical action audits the digest summarises.
// A missing store degrades to an empty action line rather than failing the
// whole digest: runs, findings, and spend are still worth showing.
func (h *AISettingsHandler) patrolDigestActions(r *http.Request) []unifiedresources.ActionAuditRecord {
	if h == nil {
		return nil
	}
	h.stateMu.RLock()
	provider := h.resourceStoreProvider
	h.stateMu.RUnlock()
	if provider == nil {
		return nil
	}
	store, err := provider(GetOrgID(r.Context()))
	if err != nil || store == nil {
		if err != nil {
			log.Debug().Err(err).Msg("Failed to resolve resource store for Patrol digest actions")
		}
		return nil
	}
	records, err := store.GetActionAuditsByStates(patrolDigestActionStates, patrolDigestActionScanLimit)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to read action audits for Patrol digest")
		return nil
	}
	return records
}
