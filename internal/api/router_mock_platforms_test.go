package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

func TestRouterMockMode_SeedsTrueNASAndVMwareSupplementalResources(t *testing.T) {
	previous := mock.IsMockEnabled()
	mock.SetEnabled(true)
	t.Cleanup(func() {
		mock.SetEnabled(previous)
	})

	cfg := &config.Config{DataPath: t.TempDir()}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	t.Cleanup(func() {
		router.shutdownBackgroundWorkers()
	})

	for _, tc := range []struct {
		name   string
		source string
		want   unified.DataSource
	}{
		{name: "truenas", source: "truenas", want: unified.SourceTrueNAS},
		{name: "vmware", source: "vmware-vsphere", want: unified.SourceVMware},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/resources?source="+tc.source, nil)
			rec := httptest.NewRecorder()

			router.resourceHandlers.HandleListResources(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp ResourcesResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode resources response: %v", err)
			}
			if len(resp.Data) == 0 {
				t.Fatalf("expected mock %s resources, got none", tc.source)
			}
			foundSource := false
			for _, resource := range resp.Data {
				for _, source := range resource.Sources {
					if source == tc.want {
						foundSource = true
						break
					}
				}
				if foundSource {
					break
				}
			}
			if !foundSource {
				t.Fatalf("expected at least one resource with source %q, got %#v", tc.want, resp.Data)
			}
		})
	}
}

func TestRouterMockMode_RestoresPlatformFeatureFlagsAfterDisable(t *testing.T) {
	t.Setenv(truenas.FeatureTrueNAS, "false")
	t.Setenv(vmware.FeatureVMware, "false")

	previousTrueNAS := truenas.IsFeatureEnabled()
	previousVMware := vmware.IsFeatureEnabled()
	truenas.ResetFeatureEnabledFromEnv()
	vmware.ResetFeatureEnabledFromEnv()
	t.Cleanup(func() {
		truenas.SetFeatureEnabled(previousTrueNAS)
		vmware.SetFeatureEnabled(previousVMware)
	})

	cfg := &config.Config{DataPath: t.TempDir()}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	t.Cleanup(func() {
		router.shutdownBackgroundWorkers()
	})

	router.syncPlatformSupplementalProviders(true)
	if !truenas.IsFeatureEnabled() {
		t.Fatal("expected mock mode to force-enable TrueNAS feature flag")
	}
	if !vmware.IsFeatureEnabled() {
		t.Fatal("expected mock mode to force-enable VMware feature flag")
	}

	router.syncPlatformSupplementalProviders(false)
	if truenas.IsFeatureEnabled() {
		t.Fatal("expected disabling mock mode to restore TrueNAS feature flag from env")
	}
	if vmware.IsFeatureEnabled() {
		t.Fatal("expected disabling mock mode to restore VMware feature flag from env")
	}
}
