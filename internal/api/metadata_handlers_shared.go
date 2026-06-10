package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

// handleMetadataGetRequest serves the GET flow shared by the guest and docker
// metadata handlers: the bare route returns the full metadata map (an empty
// object instead of null), a suffixed route returns that resource's metadata
// (a zero record instead of a 404).
func handleMetadataGetRequest[M any](
	w http.ResponseWriter,
	r *http.Request,
	routePrefix string,
	getAll func(ctx context.Context) map[string]*M,
	get func(ctx context.Context, id string) *M,
	emptyRecord func(id string) *M,
) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	// Handle both the bare route and its trailing-slash form.
	if path == routePrefix || path == routePrefix+"/" {
		// Get all metadata
		w.Header().Set("Content-Type", "application/json")
		allMeta := getAll(r.Context())
		if allMeta == nil {
			// Return empty object instead of null
			json.NewEncoder(w).Encode(make(map[string]*M))
		} else {
			json.NewEncoder(w).Encode(allMeta)
		}
		return
	}

	// Get specific resource ID from path
	resourceID := strings.TrimPrefix(path, routePrefix+"/")

	w.Header().Set("Content-Type", "application/json")

	if resourceID != "" {
		meta := get(r.Context(), resourceID)
		if meta == nil {
			// Return empty metadata instead of 404
			json.NewEncoder(w).Encode(emptyRecord(resourceID))
		} else {
			json.NewEncoder(w).Encode(meta)
		}
	} else {
		// This shouldn't happen with current routing, but handle it anyway
		http.Error(w, "Invalid request path", http.StatusBadRequest)
	}
}

// handleMetadataUpdateRequest serves the PUT/POST flow shared by the guest
// and docker metadata handlers: decode a bounded body, validate the custom
// URL, persist, and echo the stored record.
func handleMetadataUpdateRequest[M any](
	w http.ResponseWriter,
	r *http.Request,
	routePrefix string,
	idRequiredMsg string,
	idLogKey string,
	saveErrLogMsg string,
	updatedLogMsg string,
	customURL func(*M) string,
	set func(ctx context.Context, id string, meta *M) error,
) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resourceID := strings.TrimPrefix(r.URL.Path, routePrefix+"/")
	if resourceID == "" || resourceID == "metadata" {
		http.Error(w, idRequiredMsg, http.StatusBadRequest)
		return
	}

	// Limit request body to 16KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)

	var meta M
	if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate URL if provided
	if errMsg := validateCustomURL(customURL(&meta)); errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	if err := set(r.Context(), resourceID, &meta); err != nil {
		log.Error().Err(err).Str(idLogKey, resourceID).Msg(saveErrLogMsg)
		http.Error(w, metadataSaveErrorMessage(err), http.StatusInternalServerError)
		return
	}

	log.Info().Str(idLogKey, resourceID).Str("url", customURL(&meta)).Msg(updatedLogMsg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&meta)
}
