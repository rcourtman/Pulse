package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type stubAppContainerConfigProvider struct {
	calls  []AppContainerConfigRequest
	result *AppContainerConfigResult
	err    error
	empty  bool
}

func (s *stubAppContainerConfigProvider) GetConfig(_ context.Context, req AppContainerConfigRequest) (*AppContainerConfigResult, error) {
	s.calls = append(s.calls, req)
	if s.err != nil {
		return nil, s.err
	}
	if s.empty {
		return nil, nil
	}
	if s.result == nil {
		return &AppContainerConfigResult{
			ResourceID:     req.ResourceID,
			ProviderUID:    req.ProviderUID,
			Name:           req.Name,
			Host:           req.Host,
			Platform:       req.Platform,
			Status:         "running",
			ContainerCount: 1,
			UsedHostIPs:    []string{},
			Images:         []string{},
			Ports:          []PortInfo{},
			Networks:       []NetworkInfo{},
			Mounts:         []MountInfo{},
			Containers:     []AppContainerConfigContainer{},
		}, nil
	}
	result := *s.result
	return &result, nil
}

func TestAppContainerConfigObservationContract(t *testing.T) {
	t.Setenv("PULSE_STRICT_RESOLUTION", "true")
	for _, tc := range []struct {
		name, session, reason, failure             string
		missing, docker, noProvider, noHost, empty bool
		noInventory, noReadState                   bool
	}{
		{name: "without session"},
		{name: "empty session", session: "empty"},
		{name: "discovered session", session: "discovered"},
		{name: "stale placement", session: "stale"},
		{name: "unsupported adapter", docker: true, reason: "unsupported_adapter"},
		{name: "unavailable provider", noProvider: true, reason: "provider_unavailable"},
		{name: "missing placement", noHost: true, reason: "resource_context_unavailable"},
		{name: "empty provider response", empty: true, reason: "empty_provider_response"},
		{name: "provider failed", failure: "provider read failed"},
		{name: "resource absent", missing: true},
		{name: "inventory unavailable", noInventory: true, failure: "inventory is unavailable"},
		{name: "read state unavailable", noReadState: true, failure: "state is unavailable"},
		{name: "query denied", session: "denied", failure: "not permitted"},
		{name: "canonical query denied through prefix", session: "canonical-denied", failure: "not permitted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := newTrueNASUnifiedQueryProvider(t)
			resource, _, found := findCanonicalAppContainerResource(registry, "nextcloud")
			if !found {
				t.Fatal("missing canonical fixture")
			}
			if tc.docker {
				resource.TrueNAS = nil
				resource.Tags = nil
			}
			if tc.noHost {
				resource.ParentName = ""
				resource.Identity.Hostnames = nil
			}
			provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{resource}}
			config := &stubAppContainerConfigProvider{empty: tc.empty}
			if tc.failure == "provider read failed" {
				config.err = errors.New(tc.failure)
			}
			cfg := ExecutorConfig{UnifiedResourceProvider: provider, ReadState: registry.ResourceRegistry}
			if tc.noInventory {
				cfg.UnifiedResourceProvider = nil
			}
			if tc.noReadState {
				cfg.ReadState = nil
			}
			if !tc.noProvider {
				cfg.AppContainerConfigProvider = config
			}
			executor := NewPulseToolExecutor(cfg)
			ref := "Nextcloud"
			if tc.session != "" {
				resolved := &mockResolvedContext{resources: map[string]ResolvedResourceInfo{}, aliases: map[string]ResolvedResourceInfo{}}
				executor.SetResolvedContext(resolved)
				switch tc.session {
				case "discovered":
					reg, ok := resolvedAppContainerRegistration(resource)
					if !ok {
						t.Fatal("fixture registration unavailable")
					}
					resolved.AddResolvedResource(reg)
				case "stale", "denied", "canonical-denied":
					cached := &mockResource{resourceID: resource.ID, kind: "app-container", adapter: "docker", targetHost: "stale-host", providerUID: "stale-id", allowedActions: []string{"query"}}
					if tc.session != "stale" {
						cached.allowedActions = []string{"logs"}
					}
					resolved.resources[resource.ID] = cached
					if tc.session == "canonical-denied" {
						ref = "next"
					} else {
						resolved.aliases[ref] = cached
					}
				}
			}
			if tc.missing {
				ref = "absent-container"
			}
			// Inventory get succeeds independently of optional session state.
			if tc.session == "" && !tc.missing && !tc.noInventory && !tc.noReadState {
				got, err := executor.executeGetResource(context.Background(), map[string]interface{}{"resource_type": "app-container", "resource_id": ref})
				if err != nil || got.IsError || strings.Contains(got.Content[0].Text, "not_found") {
					t.Fatalf("canonical get failed: %+v %v", got, err)
				}
			}
			args := map[string]interface{}{"action": "config", "resource_type": "app-container", "resource_id": ref}
			result, err := executor.executeQuery(context.Background(), args)
			if err != nil {
				t.Fatal(err)
			}
			evidence, err := json.Marshal(map[string]interface{}{"case": tc.name, "input": args, "result": result})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("CONFIG_EVIDENCE %s", evidence)
			wantError := tc.failure != "" || tc.reason != ""
			if result.IsError != wantError {
				t.Fatalf("read error bit=%v, want %v: %+v", result.IsError, wantError, result)
			}
			if tc.failure != "" {
				if !result.IsError || !strings.Contains(result.Content[0].Text, tc.failure) {
					t.Fatalf("expected %q failure, got %+v", tc.failure, result)
				}
			} else {
				var response map[string]interface{}
				if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
					t.Fatal(err)
				}
				switch {
				case tc.missing:
					if response["error"] != "not_found" {
						t.Fatalf("expected true absence, got %+v", response)
					}
				case tc.reason != "":
					if response["available"] != false || response["reason"] != tc.reason || response["resource_id"] != resource.ID {
						t.Fatalf("unavailable configuration lost identity or reason: %+v", response)
					}
				default:
					if response["id"] != appContainerProviderID(resource) || response["host"] != canonicalAppContainerHost(resource) || response["platform"] != "truenas" {
						t.Fatalf("incorrect config identity: %+v", response)
					}
				}
			}
			wantCalls := 1
			if tc.missing || tc.docker || tc.noProvider || tc.noHost || tc.noInventory || tc.noReadState || strings.Contains(tc.session, "denied") {
				wantCalls = 0
			}
			if len(config.calls) != wantCalls {
				t.Fatalf("provider calls=%d, want %d", len(config.calls), wantCalls)
			}
			if wantCalls == 1 {
				call := config.calls[0]
				if call.ResourceID != resource.ID || call.ProviderUID != appContainerProviderID(resource) || call.Host != canonicalAppContainerHost(resource) || call.Platform != "truenas" {
					t.Fatalf("request used session identity instead of canonical inventory: %+v", call)
				}
			}
			if tc.session == "stale" {
				cached, ok := executor.resolvedContext.GetResolvedResourceByID(resource.ID)
				if !ok || strings.Join(cached.GetAllowedActions(), ",") != "query" {
					t.Fatal("read expanded existing session action authority")
				}
			}
		})
	}
}

func TestExecuteGetResourceConfig_TrueNASAppUsesNativeConfigProvider(t *testing.T) {
	provider := newTrueNASUnifiedQueryProvider(t)
	resolved := &mockResolvedContext{
		resources: make(map[string]ResolvedResourceInfo),
		aliases:   make(map[string]ResolvedResourceInfo),
	}
	configProvider := &stubAppContainerConfigProvider{
		result: &AppContainerConfigResult{
			ResourceID:            "app-container:truenas-main:nextcloud",
			ProviderUID:           "nextcloud",
			Name:                  "Nextcloud",
			Host:                  "truenas-main",
			Platform:              "truenas",
			Status:                "running",
			Version:               "1.0.3",
			HumanVersion:          "29.0.7",
			Notes:                 "Team cloud and file sync",
			UpgradeAvailable:      true,
			ImageUpdatesAvailable: true,
			ContainerCount:        2,
			UsedHostIPs:           []string{"0.0.0.0"},
			Images: []string{
				"docker.io/library/nextcloud:29.0.7",
				"docker.io/library/redis:7.2",
			},
			Ports: []PortInfo{{
				Private:  443,
				Public:   30443,
				Protocol: "tcp",
				IP:       "0.0.0.0",
			}},
			Networks: []NetworkInfo{{
				Name: "ix-nextcloud_default",
			}},
			Mounts: []MountInfo{{
				Source:      "/mnt/tank/apps/nextcloud",
				Destination: "/var/www/html",
				ReadWrite:   true,
			}},
			Containers: []AppContainerConfigContainer{{
				ID:      "nextcloud-web-1",
				Service: "nextcloud",
				Image:   "docker.io/library/nextcloud:29.0.7",
				State:   "running",
				Ports: []PortInfo{{
					Private:  443,
					Public:   30443,
					Protocol: "tcp",
					IP:       "0.0.0.0",
				}},
				Mounts: []MountInfo{{
					Source:      "/mnt/tank/apps/nextcloud",
					Destination: "/var/www/html",
					ReadWrite:   true,
				}},
			}},
		},
	}

	executor := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider:    provider,
		ReadState:                  provider.ResourceRegistry,
		AppContainerConfigProvider: configProvider,
	})
	executor.SetResolvedContext(resolved)

	if _, err := executor.executeGetResource(context.Background(), map[string]interface{}{
		"resource_type": "app-container",
		"resource_id":   "nextcloud",
	}); err != nil {
		t.Fatalf("seed resolved context: unexpected error: %v", err)
	}

	result, err := executor.executeGetResourceConfig(context.Background(), map[string]interface{}{
		"resource_type": "app-container",
		"resource_id":   "Nextcloud",
	})
	if err != nil {
		t.Fatalf("executeGetResourceConfig(app-container): unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got %+v", result)
	}

	var response AppContainerConfigResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("decode app-container config response: %v", err)
	}
	if response.Type != "app-container" || response.ID != "nextcloud" || response.Name != "Nextcloud" {
		t.Fatalf("unexpected config identity: %+v", response)
	}
	if response.Platform != "truenas" || response.Host != "truenas-main" || response.Status != "running" {
		t.Fatalf("unexpected config placement/state: %+v", response)
	}
	if response.Policy == nil {
		t.Fatal("expected governed policy metadata on app config response")
	}
	if response.AISafeSummary == "" {
		t.Fatal("expected aiSafeSummary on app config response")
	}
	if len(response.Containers) != 1 || response.Containers[0].Service != "nextcloud" {
		t.Fatalf("unexpected app container config shape: %+v", response.Containers)
	}
	if len(configProvider.calls) != 1 {
		t.Fatalf("expected one native config call, got %+v", configProvider.calls)
	}
	call := configProvider.calls[0]
	if call.OrgID != "default" || call.ProviderUID != "nextcloud" || call.Host != "truenas-main" || call.Platform != "truenas" {
		t.Fatalf("unexpected native app config request: %+v", call)
	}
	if call.ResourceID == "" {
		t.Fatalf("expected canonical resource id in native app config request, got %+v", call)
	}
}
