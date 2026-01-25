package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func newDockerMetadataHandler(t *testing.T) *DockerMetadataHandler {
	t.Helper()
	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("GetPersistence: %v", err)
	}
	return NewDockerMetadataHandler(mtp)
}

func TestDockerMetadataHandlers_ContainerMetadata(t *testing.T) {
	handler := newDockerMetadataHandler(t)

	t.Run("get-all-empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/docker/metadata", nil)
		rec := httptest.NewRecorder()

		handler.HandleGetMetadata(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(payload) != 0 {
			t.Fatalf("expected empty map, got %v", payload)
		}
	})

	t.Run("update-invalid-url", func(t *testing.T) {
		meta := config.DockerMetadata{CustomURL: "ftp://example.com"}
		body, _ := json.Marshal(meta)
		req := httptest.NewRequest(http.MethodPut, "/api/docker/metadata/host1:container:abc", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handler.HandleUpdateMetadata(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("update-get-delete", func(t *testing.T) {
		meta := config.DockerMetadata{
			CustomURL:   "https://example.com",
			Description: "test container",
			Tags:        []string{"app"},
		}
		body, _ := json.Marshal(meta)
		req := httptest.NewRequest(http.MethodPut, "/api/docker/metadata/host1:container:abc", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handler.HandleUpdateMetadata(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}

		getReq := httptest.NewRequest(http.MethodGet, "/api/docker/metadata/host1:container:abc", nil)
		getRec := httptest.NewRecorder()
		handler.HandleGetMetadata(getRec, getReq)
		if getRec.Code != http.StatusOK {
			t.Fatalf("get status = %d, want 200", getRec.Code)
		}
		var got config.DockerMetadata
		if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode get response: %v", err)
		}
		if got.CustomURL != "https://example.com" {
			t.Fatalf("custom_url = %q, want https://example.com", got.CustomURL)
		}

		delReq := httptest.NewRequest(http.MethodDelete, "/api/docker/metadata/host1:container:abc", nil)
		delRec := httptest.NewRecorder()
		handler.HandleDeleteMetadata(delRec, delReq)
		if delRec.Code != http.StatusNoContent {
			t.Fatalf("delete status = %d, want 204", delRec.Code)
		}

		getReq = httptest.NewRequest(http.MethodGet, "/api/docker/metadata/host1:container:abc", nil)
		getRec = httptest.NewRecorder()
		handler.HandleGetMetadata(getRec, getReq)
		if getRec.Code != http.StatusOK {
			t.Fatalf("get status = %d, want 200", getRec.Code)
		}
		var empty config.DockerMetadata
		if err := json.Unmarshal(getRec.Body.Bytes(), &empty); err != nil {
			t.Fatalf("decode get response: %v", err)
		}
		if empty.ID != "host1:container:abc" {
			t.Fatalf("expected empty metadata with ID, got %q", empty.ID)
		}
	})
}

func TestDockerMetadataHandlers_HostMetadata(t *testing.T) {
	handler := newDockerMetadataHandler(t)

	t.Run("update-and-get-host", func(t *testing.T) {
		meta := config.DockerHostMetadata{
			CustomDisplayName: "Host A",
			CustomURL:         "https://portainer.local",
			Notes:             []string{"note1"},
		}
		body, _ := json.Marshal(meta)
		req := httptest.NewRequest(http.MethodPut, "/api/docker/hosts/metadata/host-1", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handler.HandleUpdateHostMetadata(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}

		getReq := httptest.NewRequest(http.MethodGet, "/api/docker/hosts/metadata/host-1", nil)
		getRec := httptest.NewRecorder()
		handler.HandleGetHostMetadata(getRec, getReq)
		if getRec.Code != http.StatusOK {
			t.Fatalf("get status = %d, want 200", getRec.Code)
		}
		var got config.DockerHostMetadata
		if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode get response: %v", err)
		}
		if got.CustomDisplayName != "Host A" {
			t.Fatalf("custom_display_name = %q, want Host A", got.CustomDisplayName)
		}
	})

	t.Run("merge-host-metadata", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/docker/hosts/metadata/host-1", bytes.NewReader([]byte(`{}`)))
		rec := httptest.NewRecorder()

		handler.HandleUpdateHostMetadata(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}

		getReq := httptest.NewRequest(http.MethodGet, "/api/docker/hosts/metadata/host-1", nil)
		getRec := httptest.NewRecorder()
		handler.HandleGetHostMetadata(getRec, getReq)
		if getRec.Code != http.StatusOK {
			t.Fatalf("get status = %d, want 200", getRec.Code)
		}
		var got config.DockerHostMetadata
		if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode get response: %v", err)
		}
		if got.CustomDisplayName != "Host A" {
			t.Fatalf("expected merged display name, got %q", got.CustomDisplayName)
		}
	})

	t.Run("delete-host-metadata", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/docker/hosts/metadata/host-1", nil)
		rec := httptest.NewRecorder()

		handler.HandleDeleteHostMetadata(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
	})
}
