package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestGuestMetadataHandler(t *testing.T) {
	handler := NewGuestMetadataHandler(t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/guests/metadata", nil)
	resp := httptest.NewRecorder()
	handler.HandleGetMetadata(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	var all map[string]config.GuestMetadata
	if err := json.Unmarshal(resp.Body.Bytes(), &all); err != nil {
		t.Fatalf("decode all guests: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected empty metadata, got %v", all)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/guests/metadata/", strings.NewReader(`{}`))
	resp = httptest.NewRecorder()
	handler.HandleUpdateMetadata(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/guests/metadata/100", strings.NewReader(`{"customUrl":"ftp://example.com"}`))
	resp = httptest.NewRecorder()
	handler.HandleUpdateMetadata(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "http:// or https://") {
		t.Fatalf("unexpected error: %s", resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/guests/metadata/100", strings.NewReader(`{"customUrl":"https://example.com","description":"desc"}`))
	resp = httptest.NewRecorder()
	handler.HandleUpdateMetadata(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	var meta config.GuestMetadata
	if err := json.Unmarshal(resp.Body.Bytes(), &meta); err != nil {
		t.Fatalf("decode guest metadata: %v", err)
	}
	if meta.CustomURL != "https://example.com" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/guests/metadata/100", nil)
	resp = httptest.NewRecorder()
	handler.HandleGetMetadata(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &meta); err != nil {
		t.Fatalf("decode guest metadata: %v", err)
	}
	if meta.CustomURL != "https://example.com" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/guests/metadata/100", nil)
	resp = httptest.NewRecorder()
	handler.HandleDeleteMetadata(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
}

func TestHostMetadataHandler(t *testing.T) {
	handler := NewHostMetadataHandler(t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/hosts/metadata", nil)
	resp := httptest.NewRecorder()
	handler.HandleGetMetadata(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	var all map[string]config.HostMetadata
	if err := json.Unmarshal(resp.Body.Bytes(), &all); err != nil {
		t.Fatalf("decode all hosts: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected empty metadata, got %v", all)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/hosts/metadata/host1", strings.NewReader(`{"customUrl":"http://host.local"}`))
	resp = httptest.NewRecorder()
	handler.HandleUpdateMetadata(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	var meta config.HostMetadata
	if err := json.Unmarshal(resp.Body.Bytes(), &meta); err != nil {
		t.Fatalf("decode host metadata: %v", err)
	}
	if meta.CustomURL != "http://host.local" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/hosts/metadata/host1", nil)
	resp = httptest.NewRecorder()
	handler.HandleDeleteMetadata(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
}
