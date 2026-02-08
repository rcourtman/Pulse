package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/assert"
)

func TestHandleGetResource(t *testing.T) {
	// Setup
	handlers := NewResourceHandlers()

	// Add a dummy resource
	res := unifiedresources.LegacyResource{
		ID:   "test-node-1",
		Type: unifiedresources.LegacyResourceTypeNode,
		Name: "Test Node",
	}
	handlers.Store().Upsert(res)

	// Test Case 1: Resource Found
	req := httptest.NewRequest("GET", "/api/resources/test-node-1", nil)
	w := httptest.NewRecorder()

	// We need to route this correctly or call the handler directly with context
	// Handler expects path param parsing usually handled by router/mux.
	// But HandleGetResource implementation says:
	// path := strings.TrimPrefix(r.URL.Path, "/api/resources/")
	// So we can call it directly.

	handlers.HandleGetResource(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var returned unifiedresources.LegacyResource
	err := json.NewDecoder(w.Body).Decode(&returned)
	assert.NoError(t, err)
	assert.Equal(t, "test-node-1", returned.ID)

	// Test Case 2: Resource Not Found
	req = httptest.NewRequest("GET", "/api/resources/non-existent", nil)
	w = httptest.NewRecorder()

	handlers.HandleGetResource(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleGetResources(t *testing.T) {
	handlers := NewResourceHandlers()

	handlers.Store().Upsert(unifiedresources.LegacyResource{ID: "vm-1", Type: unifiedresources.LegacyResourceTypeVM, Status: unifiedresources.LegacyStatusRunning})
	handlers.Store().Upsert(unifiedresources.LegacyResource{ID: "node-1", Type: unifiedresources.LegacyResourceTypeNode, Status: unifiedresources.LegacyStatusOnline})

	// Case 1: List All
	req := httptest.NewRequest("GET", "/api/resources", nil)
	w := httptest.NewRecorder()
	handlers.HandleGetResources(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ResourcesResponse
	json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, 2, resp.Count)

	// Case 2: Filter by Type
	req = httptest.NewRequest("GET", "/api/resources?type=vm", nil)
	w = httptest.NewRecorder()
	handlers.HandleGetResources(w, req)
	json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, 1, resp.Count)
	assert.Equal(t, "vm-1", resp.Resources[0].ID)
}

func TestHandleGetResourceStats(t *testing.T) {
	handlers := NewResourceHandlers()
	handlers.Store().Upsert(unifiedresources.LegacyResource{ID: "1", Type: unifiedresources.LegacyResourceTypeVM})

	req := httptest.NewRequest("GET", "/api/resources/stats", nil)
	w := httptest.NewRecorder()
	handlers.HandleGetResourceStats(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var stats unifiedresources.LegacyStoreStats
	json.NewDecoder(w.Body).Decode(&stats)
	assert.Equal(t, 1, stats.TotalResources)
}
