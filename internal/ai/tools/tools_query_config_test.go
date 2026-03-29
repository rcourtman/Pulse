package tools

import (
	"context"
	"encoding/json"
	"testing"
)

type stubAppContainerConfigProvider struct {
	calls  []AppContainerConfigRequest
	result *AppContainerConfigResult
	err    error
}

func (s *stubAppContainerConfigProvider) GetConfig(_ context.Context, req AppContainerConfigRequest) (*AppContainerConfigResult, error) {
	s.calls = append(s.calls, req)
	if s.err != nil {
		return nil, s.err
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
