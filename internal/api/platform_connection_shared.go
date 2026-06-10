package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// decodeOptionalInstanceRequest decodes an optional JSON body onto a copy of
// base. The results are (instance, decoded, ok): an empty body returns base
// with decoded=false, a malformed body writes a 400 response and returns
// ok=false. Shared by the TrueNAS and VMware connection handlers.
func decodeOptionalInstanceRequest[T any](w http.ResponseWriter, r *http.Request, base T) (T, bool, bool) {
	var zero T
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return zero, false, false
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return base, false, true
	}

	instance := base
	if err := json.Unmarshal(body, &instance); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return zero, false, false
	}

	return instance, true, true
}

// platformConnectionUpdateSpec wires the per-platform pieces of the shared
// connection-update flow. The flow itself — locate by trimmed ID, decode with
// the stored record as fallback, normalize, preserve masked secrets,
// validate, persist, respond redacted — is platform-invariant policy;
// single-sourcing it means a new platform cannot forget masked-secret
// preservation.
type platformConnectionUpdateSpec[T any] struct {
	notFoundCode string
	saveFailCode string
	saveFailMsg  string
	id           func(T) string
	decode       func(http.ResponseWriter, *http.Request, T) (T, bool)
	setID        func(*T, string)
	normalize    func(*T)
	preserve     func(updated *T, stored T)
	validate     func(T) error
	redacted     func(T) any
	save         func([]T) error
}

// updatePlatformConnection runs the shared platform connection update flow
// over the loaded instances for the resolved connection ID.
func updatePlatformConnection[T any](w http.ResponseWriter, r *http.Request, connectionID string, instances []T, spec platformConnectionUpdateSpec[T]) {
	index := -1
	for i := range instances {
		if strings.TrimSpace(spec.id(instances[i])) == connectionID {
			index = i
			break
		}
	}
	if index < 0 {
		writeErrorResponse(w, http.StatusNotFound, spec.notFoundCode, "Connection not found", nil)
		return
	}

	instance, ok := spec.decode(w, r, instances[index])
	if !ok {
		return
	}
	spec.setID(&instance, connectionID)
	spec.normalize(&instance)
	spec.preserve(&instance, instances[index])
	if err := spec.validate(instance); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}
	instances[index] = instance
	if err := spec.save(instances); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, spec.saveFailCode, spec.saveFailMsg, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, spec.redacted(instance))
}

// platformConnectionItemHandlers carries the per-platform handlers for the
// "/api/<platform>/connections/" item routes.
type platformConnectionItemHandlers struct {
	test    http.HandlerFunc
	preview http.HandlerFunc
	delete  http.HandlerFunc
	update  http.HandlerFunc
}

// platformConnectionItemRoute builds the shared mux handler for platform
// connection item routes: POST .../test, POST .../preview, DELETE, PUT — all
// admin + settings:write gated. handlers is resolved per request so late
// handler wiring and nil-service checks keep working.
func (r *Router) platformConnectionItemRoute(unavailableCode, unavailableMsg string, handlers func() *platformConnectionItemHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		h := handlers()
		if h == nil {
			writeErrorResponse(w, http.StatusServiceUnavailable, unavailableCode, unavailableMsg, nil)
			return
		}

		if req.Method == http.MethodPost && strings.HasSuffix(strings.Trim(req.URL.Path, "/"), "/test") {
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, h.test))(w, req)
		} else if req.Method == http.MethodPost && strings.HasSuffix(strings.Trim(req.URL.Path, "/"), "/preview") {
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, h.preview))(w, req)
		} else if req.Method == http.MethodDelete {
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, h.delete))(w, req)
		} else if req.Method == http.MethodPut {
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, h.update))(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
