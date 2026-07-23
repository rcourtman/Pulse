package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestGuestMetadataHandler(t *testing.T) {
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	handler := NewGuestMetadataHandler(mtp)

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
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	handler := NewHostMetadataHandler(mtp)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/metadata", nil)
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

	req = httptest.NewRequest(http.MethodPut, "/api/agents/metadata/host1", strings.NewReader(`{"customUrl":"http://host.local"}`))
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

	req = httptest.NewRequest(http.MethodDelete, "/api/agents/metadata/host1", nil)
	resp = httptest.NewRecorder()
	handler.HandleDeleteMetadata(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agents/metadata", nil)
	resp = httptest.NewRecorder()
	handler.HandleGetMetadata(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status for agent alias list: %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/agents/metadata/agent1", strings.NewReader(`{"customUrl":"https://agent.local"}`))
	resp = httptest.NewRecorder()
	handler.HandleUpdateMetadata(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status for agent alias update: %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agents/metadata/agent1", nil)
	resp = httptest.NewRecorder()
	handler.HandleGetMetadata(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status for agent alias get: %d", resp.Code)
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &meta); err != nil {
		t.Fatalf("decode host metadata via agent alias: %v", err)
	}
	if meta.CustomURL != "https://agent.local" {
		t.Fatalf("unexpected metadata via agent alias: %+v", meta)
	}
}

func TestMetadataHandlersUseLiveMonitorStoresForImmediateRoundTrips(t *testing.T) {
	fallbackPersistence := config.NewMultiTenantPersistence(t.TempDir())
	defaultPersistence, err := fallbackPersistence.GetPersistence("default")
	if err != nil {
		t.Fatalf("get default persistence: %v", err)
	}

	t.Run("guest", func(t *testing.T) {
		livePath := t.TempDir()
		liveStore := config.NewGuestMetadataStore(livePath, nil)
		defaultPersistence.SetMetadataStores(liveStore, nil, nil)
		handler := NewGuestMetadataHandler(fallbackPersistence)
		resolvedOrgID := ""
		handler.SetStoreResolver(func(ctx context.Context) *config.GuestMetadataStore {
			resolvedOrgID = GetOrgID(ctx)
			return liveStore
		})

		req := httptest.NewRequest(
			http.MethodPut,
			"/api/guests/metadata/app-container:host:name:app",
			strings.NewReader(`{"customUrl":"https://app.internal"}`),
		)
		resp := httptest.NewRecorder()
		handler.HandleUpdateMetadata(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("update status = %d, body = %s", resp.Code, resp.Body.String())
		}
		if meta := liveStore.Get("app-container:host:name:app"); meta == nil || meta.CustomURL != "https://app.internal" {
			t.Fatalf("live guest store metadata = %#v", meta)
		}
		if defaultPersistence.GetGuestMetadataStore() != liveStore {
			t.Fatal("persistence and monitor guest stores do not share one live instance")
		}

		// A config import writes the same file out of band, then Reload must
		// refresh the monitor-owned in-memory store rather than a parallel
		// persistence cache.
		importWriter := config.NewGuestMetadataStore(livePath, nil)
		if err := importWriter.Set("imported", &config.GuestMetadata{
			CustomURL: "https://imported.internal",
		}); err != nil {
			t.Fatalf("write imported guest metadata: %v", err)
		}
		tenantContext := context.WithValue(context.Background(), OrgIDContextKey, "tenant-a")
		if err := handler.Reload(tenantContext); err != nil {
			t.Fatalf("reload live guest metadata: %v", err)
		}
		if resolvedOrgID != "tenant-a" {
			t.Fatalf("reload resolved org %q, want tenant-a", resolvedOrgID)
		}
		if meta := liveStore.Get("imported"); meta == nil || meta.CustomURL != "https://imported.internal" {
			t.Fatalf("reloaded live guest store metadata = %#v", meta)
		}
	})

	t.Run("docker", func(t *testing.T) {
		liveStore := config.NewDockerMetadataStore(t.TempDir(), nil)
		defaultPersistence.SetMetadataStores(nil, liveStore, nil)
		handler := NewDockerMetadataHandler(fallbackPersistence)
		handler.SetStoreResolver(func(context.Context) *config.DockerMetadataStore {
			return liveStore
		})

		req := httptest.NewRequest(
			http.MethodPut,
			"/api/docker/metadata/docker-host:container:runtime-id",
			strings.NewReader(`{"customUrl":"https://container.internal"}`),
		)
		resp := httptest.NewRecorder()
		handler.HandleUpdateMetadata(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("update status = %d, body = %s", resp.Code, resp.Body.String())
		}
		if meta := liveStore.Get("docker-host:container:runtime-id"); meta == nil || meta.CustomURL != "https://container.internal" {
			t.Fatalf("live Docker store metadata = %#v", meta)
		}
		if defaultPersistence.GetDockerMetadataStore() != liveStore {
			t.Fatal("persistence and monitor Docker stores do not share one live instance")
		}
	})

	t.Run("host", func(t *testing.T) {
		liveStore := config.NewHostMetadataStore(t.TempDir(), nil)
		defaultPersistence.SetMetadataStores(nil, nil, liveStore)
		handler := NewHostMetadataHandler(fallbackPersistence)
		handler.SetStoreResolver(func(context.Context) *config.HostMetadataStore {
			return liveStore
		})

		req := httptest.NewRequest(
			http.MethodPut,
			"/api/agents/metadata/agent-live",
			strings.NewReader(`{"customUrl":"https://agent.internal"}`),
		)
		resp := httptest.NewRecorder()
		handler.HandleUpdateMetadata(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("update status = %d, body = %s", resp.Code, resp.Body.String())
		}
		if meta := liveStore.Get("agent-live"); meta == nil || meta.CustomURL != "https://agent.internal" {
			t.Fatalf("live host store metadata = %#v", meta)
		}
		if defaultPersistence.GetHostMetadataStore() != liveStore {
			t.Fatal("persistence and monitor host stores do not share one live instance")
		}
	})
}
