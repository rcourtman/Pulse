package api

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

type attentionEvidenceResponse struct {
	Evidence  operationaltrust.EvidenceEnvelope  `json:"evidence"`
	Freshness operationaltrust.EvidenceFreshness `json:"freshness"`
	Retained  bool                               `json:"retained"`
}

func parseAttentionEvidencePath(path string) (string, string, bool) {
	path = strings.Trim(path, "/")
	boundary := strings.LastIndex(path, "/evidence/")
	if boundary <= 0 {
		return "", "", false
	}
	itemID, itemErr := url.PathUnescape(path[:boundary])
	evidenceID, evidenceErr := url.PathUnescape(path[boundary+len("/evidence/"):])
	itemID = strings.TrimSpace(itemID)
	evidenceID = strings.TrimSpace(evidenceID)
	return itemID, evidenceID, itemErr == nil && evidenceErr == nil &&
		itemID != "" && evidenceID != ""
}

func (h *AttentionHandlers) handleAttentionEvidence(
	w http.ResponseWriter,
	r *http.Request,
	path string,
) {
	itemID, evidenceID, ok := parseAttentionEvidencePath(path)
	if !ok {
		writeErrorResponse(
			w,
			http.StatusBadRequest,
			"invalid_attention_evidence_id",
			"Attention evidence reference is invalid.",
			nil,
		)
		return
	}
	projection, err := h.project(r, true)
	if err != nil {
		writeAttentionUnavailable(w, err)
		return
	}
	var detail *ai.AttentionItemDetail
	for index := range projection.Details {
		if projection.Details[index].Item.ID == itemID {
			detail = &projection.Details[index]
			break
		}
	}
	if detail == nil {
		writeErrorResponse(
			w,
			http.StatusNotFound,
			"attention_item_not_found",
			"Attention item was not found.",
			nil,
		)
		return
	}
	for _, evidence := range detail.Evidence {
		if evidence.ID != evidenceID {
			continue
		}
		response := attentionEvidenceResponse{
			Evidence:  evidence.Clone(),
			Freshness: evidence.FreshnessAt(time.Now().UTC()),
			Retained:  true,
		}
		if err := utils.WriteJSONResponse(w, response); err != nil {
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"attention_evidence_encode_failed",
				"Failed to encode attention evidence.",
			)
		}
		return
	}
	for _, referencedID := range detail.OperationalRecord.EvidenceIDs {
		if referencedID == evidenceID {
			writeErrorResponse(
				w,
				http.StatusGone,
				"attention_evidence_detail_expired",
				"The retained evidence summary is still linked, but its bounded detail has expired.",
				map[string]string{
					"evidenceId": evidenceID,
					"retained":   "false",
				},
			)
			return
		}
	}
	writeErrorResponse(
		w,
		http.StatusNotFound,
		"attention_evidence_not_found",
		"Evidence was not found for this attention item.",
		nil,
	)
}
