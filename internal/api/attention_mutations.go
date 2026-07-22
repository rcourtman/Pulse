package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const (
	maxAttentionSuppressionDuration = 30 * 24 * time.Hour
	maxAttentionMutationBodyBytes   = 4096
)

type attentionMutationKind string

const (
	attentionMutationAcknowledge   attentionMutationKind = "acknowledge"
	attentionMutationUnacknowledge attentionMutationKind = "unacknowledge"
	attentionMutationSuppress      attentionMutationKind = "suppress"
	attentionMutationUnsuppress    attentionMutationKind = "unsuppress"
)

type attentionSuppressionRequest struct {
	Reason    string    `json:"reason"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func parseAttentionMutationPath(
	path string,
) (string, attentionMutationKind, bool) {
	path = strings.Trim(path, "/")
	for _, kind := range []attentionMutationKind{
		attentionMutationAcknowledge,
		attentionMutationUnacknowledge,
		attentionMutationSuppress,
		attentionMutationUnsuppress,
	} {
		suffix := "/" + string(kind)
		if !strings.HasSuffix(path, suffix) {
			continue
		}
		itemID, err := url.PathUnescape(strings.TrimSuffix(path, suffix))
		itemID = strings.TrimSpace(itemID)
		return itemID, kind, err == nil && itemID != ""
	}
	return "", "", false
}

func isAttentionMutationPath(path string) bool {
	_, _, ok := parseAttentionMutationPath(path)
	return ok
}

func (h *AttentionHandlers) handleAttentionMutation(
	w http.ResponseWriter,
	r *http.Request,
	path string,
) {
	if !ensureRelayMobileRuntimeRoute(w, r, relayMobileRouteAttentionMutation) {
		return
	}
	itemID, kind, ok := parseAttentionMutationPath(path)
	if !ok {
		writeJSONError(
			w,
			http.StatusNotFound,
			"attention_mutation_not_found",
			"Attention mutation was not found.",
		)
		return
	}
	if h == nil || h.getMonitor == nil {
		writeAttentionUnavailable(w, nil)
		return
	}
	monitor := h.getMonitor(r.Context())
	if monitor == nil || monitor.GetAlertManager() == nil {
		writeAttentionUnavailable(w, nil)
		return
	}
	manager := monitor.GetAlertManager()
	actor := strings.TrimSpace(auth.GetUser(r.Context()))
	if actor == "" {
		actor = "unknown"
	}

	var err error
	switch kind {
	case attentionMutationAcknowledge:
		err = manager.AcknowledgeAlert(itemID, actor)
	case attentionMutationUnacknowledge:
		err = manager.UnacknowledgeAlert(itemID)
	case attentionMutationSuppress:
		var request attentionSuppressionRequest
		r.Body = http.MaxBytesReader(w, r.Body, maxAttentionMutationBodyBytes)
		if decodeErr := json.NewDecoder(r.Body).Decode(&request); decodeErr != nil {
			writeJSONError(
				w,
				http.StatusBadRequest,
				"invalid_attention_suppression",
				"Suppression requires a reason and expiry.",
			)
			return
		}
		now := time.Now().UTC()
		if strings.TrimSpace(request.Reason) == "" ||
			request.ExpiresAt.IsZero() ||
			!request.ExpiresAt.After(now) ||
			request.ExpiresAt.After(now.Add(maxAttentionSuppressionDuration)) {
			writeJSONError(
				w,
				http.StatusBadRequest,
				"invalid_attention_suppression",
				"Suppression requires a reason and an expiry within 30 days.",
			)
			return
		}
		expiresAt := request.ExpiresAt.UTC()
		err = manager.SuppressOperationalAlert(
			itemID,
			actor,
			request.Reason,
			&expiresAt,
		)
	case attentionMutationUnsuppress:
		err = manager.UnsuppressOperationalAlert(itemID)
	}
	if err != nil {
		writeJSONError(
			w,
			http.StatusNotFound,
			"attention_item_not_found",
			"Attention item was not found.",
		)
		return
	}
	monitor.SyncAlertState()
	if err := utils.WriteJSONResponse(w, map[string]bool{"success": true}); err != nil {
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"attention_mutation_encode_failed",
			"Failed to encode attention mutation result.",
		)
	}
}
