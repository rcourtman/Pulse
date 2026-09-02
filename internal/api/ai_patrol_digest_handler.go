package api

import (
	"context"
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

	digest, _ := h.BuildPatrolDigest(r.Context(), days)
	if err := utils.WriteJSONResponse(w, digest); err != nil {
		log.Error().Err(err).Msg("Failed to write patrol digest response")
	}
}

// BuildPatrolDigest assembles the digest inputs for the tenant in ctx and
// returns the rollup. The boolean reports whether a Patrol service backed the
// numbers; a false result is the zero-valued shape a client can still render as
// "Patrol has not run", while scheduled emails treat it as "not available".
func (h *AISettingsHandler) BuildPatrolDigest(ctx context.Context, days int) (ai.PatrolDigest, bool) {
	days = ai.NormalizePatrolDigestDays(days)
	input := ai.PatrolDigestInput{
		Now:                time.Now().UTC(),
		Days:               days,
		Mode:               config.PatrolAutonomyMonitor,
		RunHistoryCapacity: ai.MaxPatrolRunHistory,
	}
	available := false
	if aiService := h.GetAIService(ctx); aiService != nil {
		input.Mode = aiService.GetEffectivePatrolAutonomyLevel()
		if patrol := aiService.GetPatrolService(); patrol != nil {
			available = true
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
	input.Actions = h.patrolDigestActions(ctx)
	return ai.BuildPatrolDigest(input), available
}

// patrolDigestActions reads the canonical action audits the digest summarises.
// A missing store degrades to an empty action line rather than failing the
// whole digest: runs, findings, and spend are still worth showing.
func (h *AISettingsHandler) patrolDigestActions(ctx context.Context) []unifiedresources.ActionAuditRecord {
	if h == nil {
		return nil
	}
	h.stateMu.RLock()
	provider := h.resourceStoreProvider
	h.stateMu.RUnlock()
	if provider == nil {
		return nil
	}
	store, err := provider(GetOrgID(ctx))
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
