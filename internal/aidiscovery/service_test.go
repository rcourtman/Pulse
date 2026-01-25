package aidiscovery

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type stubAnalyzer struct {
	mu       sync.Mutex
	calls    int
	response string
}

func (s *stubAnalyzer) AnalyzeForDiscovery(ctx context.Context, prompt string) (string, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	return s.response, nil
}

type errorAnalyzer struct{}

func (errorAnalyzer) AnalyzeForDiscovery(ctx context.Context, prompt string) (string, error) {
	return "", context.Canceled
}

type stubStateProvider struct {
	state StateSnapshot
}

func (s stubStateProvider) GetState() StateSnapshot {
	return s.state
}

type panicStateProvider struct{}

func (panicStateProvider) GetState() StateSnapshot {
	panic("boom")
}

func TestService_parseAIResponse_Markdown(t *testing.T) {
	service := &Service{}
	response := "```json\n{\n  \"service_type\": \"nginx\",\n  \"service_name\": \"Nginx\",\n  \"service_version\": \"1.2\",\n  \"category\": \"web_server\",\n  \"cli_access\": \"docker exec {container} bash\",\n  \"facts\": [{\"category\": \"version\", \"key\": \"nginx\", \"value\": \"1.2\", \"source\": \"cmd\", \"confidence\": 0.9}],\n  \"config_paths\": [\"/etc/nginx/nginx.conf\"],\n  \"data_paths\": [\"/var/www\"],\n  \"ports\": [{\"port\": 80, \"protocol\": \"tcp\", \"process\": \"nginx\", \"address\": \"0.0.0.0\"}],\n  \"confidence\": 0.9,\n  \"reasoning\": \"image name\"\n}\n```"

	parsed := service.parseAIResponse(response)
	if parsed == nil {
		t.Fatalf("expected parsed response")
	}
	if parsed.ServiceType != "nginx" || parsed.ServiceName != "Nginx" {
		t.Fatalf("unexpected parsed result: %#v", parsed)
	}
	if len(parsed.Facts) != 1 || parsed.Facts[0].DiscoveredAt.IsZero() {
		t.Fatalf("expected fact timestamp set: %#v", parsed.Facts)
	}

	if service.parseAIResponse("not json") != nil {
		t.Fatalf("expected nil for invalid json")
	}
}

func TestService_analyzeDockerContainer_CacheAndPorts(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil
	service := NewService(store, nil, nil, Config{CacheExpiry: time.Hour})

	analyzer := &stubAnalyzer{
		response: `{"service_type":"nginx","service_name":"Nginx","service_version":"1.2","category":"web_server","cli_access":"docker exec {container} nginx -v","facts":[],"config_paths":[],"data_paths":[],"ports":[],"confidence":0.9,"reasoning":"image"}`,
	}

	container := DockerContainer{
		Name:   "web",
		Image:  "nginx:latest",
		Status: "running",
		Ports: []DockerPort{
			{PublicPort: 8080, PrivatePort: 80, Protocol: "tcp"},
		},
	}
	host := DockerHost{
		AgentID:  "host1",
		Hostname: "host1",
	}

	first := service.analyzeDockerContainer(context.Background(), analyzer, container, host)
	if first == nil {
		t.Fatalf("expected discovery")
	}
	if !strings.Contains(first.CLIAccess, "web") {
		t.Fatalf("expected cli access to include container name, got %s", first.CLIAccess)
	}
	if len(first.Ports) != 1 || first.Ports[0].Port != 80 || first.Ports[0].Address != ":8080" {
		t.Fatalf("unexpected ports: %#v", first.Ports)
	}

	second := service.analyzeDockerContainer(context.Background(), analyzer, container, host)
	if second == nil {
		t.Fatalf("expected cached discovery")
	}

	analyzer.mu.Lock()
	calls := analyzer.calls
	analyzer.mu.Unlock()
	if calls != 1 {
		t.Fatalf("expected analyzer called once, got %d", calls)
	}

	lowAnalyzer := &stubAnalyzer{
		response: `{"service_type":"unknown","service_name":"","service_version":"","category":"unknown","cli_access":"","facts":[],"config_paths":[],"data_paths":[],"ports":[],"confidence":0.4,"reasoning":""}`,
	}
	lowContainer := DockerContainer{Name: "mystery", Image: "unknown:latest"}
	if got := service.analyzeDockerContainer(context.Background(), lowAnalyzer, lowContainer, host); got != nil {
		t.Fatalf("expected low confidence discovery to be skipped")
	}
}

func TestService_DiscoverResource_RecentAndNoAnalyzer(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil
	service := NewService(store, nil, nil, DefaultConfig())

	req := DiscoveryRequest{
		ResourceType: ResourceTypeDocker,
		ResourceID:   "nginx",
		HostID:       "host1",
		Hostname:     "host1",
	}
	discovery := &ResourceDiscovery{
		ID:           MakeResourceID(req.ResourceType, req.HostID, req.ResourceID),
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		HostID:       req.HostID,
		Hostname:     req.Hostname,
		ServiceName:  "Existing",
	}
	if err := store.Save(discovery); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	found, err := service.DiscoverResource(context.Background(), req)
	if err != nil {
		t.Fatalf("DiscoverResource error: %v", err)
	}
	if found == nil || found.ServiceName != "Existing" {
		t.Fatalf("unexpected discovery: %#v", found)
	}

	_, err = service.DiscoverResource(context.Background(), DiscoveryRequest{
		ResourceType: ResourceTypeVM,
		ResourceID:   "101",
		HostID:       "node1",
		Hostname:     "node1",
		Force:        true,
	})
	if err == nil || !strings.Contains(err.Error(), "AI analyzer") {
		t.Fatalf("expected analyzer error, got %v", err)
	}

	service.SetAIAnalyzer(errorAnalyzer{})
	_, err = service.DiscoverResource(context.Background(), DiscoveryRequest{
		ResourceType: ResourceTypeVM,
		ResourceID:   "102",
		HostID:       "node1",
		Hostname:     "node1",
		Force:        true,
	})
	if err == nil || !strings.Contains(err.Error(), "AI analysis failed") {
		t.Fatalf("expected analysis error, got %v", err)
	}

	service.SetAIAnalyzer(&stubAnalyzer{response: "not json"})
	_, err = service.DiscoverResource(context.Background(), DiscoveryRequest{
		ResourceType: ResourceTypeVM,
		ResourceID:   "103",
		HostID:       "node1",
		Hostname:     "node1",
		Force:        true,
	})
	if err == nil || !strings.Contains(err.Error(), "failed to parse") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestService_getResourceMetadata(t *testing.T) {
	state := StateSnapshot{
		VMs: []VM{
			{VMID: 101, Name: "vm1", Node: "node1", Status: "running"},
		},
		Containers: []Container{
			{VMID: 201, Name: "lxc1", Node: "node2", Status: "stopped"},
		},
		DockerHosts: []DockerHost{
			{
				AgentID:  "agent1",
				Hostname: "dock1",
				Containers: []DockerContainer{
					{Name: "redis", Image: "redis:latest", Status: "running", Labels: map[string]string{"tier": "cache"}},
				},
			},
		},
	}

	service := NewService(nil, nil, stubStateProvider{state: state}, DefaultConfig())

	vmMeta := service.getResourceMetadata(DiscoveryRequest{
		ResourceType: ResourceTypeVM,
		ResourceID:   "101",
		HostID:       "node1",
	})
	if vmMeta["name"] != "vm1" || vmMeta["vmid"] != 101 {
		t.Fatalf("unexpected vm metadata: %#v", vmMeta)
	}

	lxcMeta := service.getResourceMetadata(DiscoveryRequest{
		ResourceType: ResourceTypeLXC,
		ResourceID:   "201",
		HostID:       "node2",
	})
	if lxcMeta["name"] != "lxc1" || lxcMeta["status"] != "stopped" {
		t.Fatalf("unexpected lxc metadata: %#v", lxcMeta)
	}

	dockerMeta := service.getResourceMetadata(DiscoveryRequest{
		ResourceType: ResourceTypeDocker,
		ResourceID:   "redis",
		HostID:       "agent1",
	})
	if dockerMeta["image"] != "redis:latest" || dockerMeta["status"] != "running" {
		t.Fatalf("unexpected docker metadata: %#v", dockerMeta)
	}

	dockerByHost := service.getResourceMetadata(DiscoveryRequest{
		ResourceType: ResourceTypeDocker,
		ResourceID:   "redis",
		HostID:       "dock1",
	})
	if dockerByHost["image"] != "redis:latest" {
		t.Fatalf("unexpected docker hostname metadata: %#v", dockerByHost)
	}
}

func TestService_formatCLIAccessAndStatus(t *testing.T) {
	service := NewService(nil, nil, nil, DefaultConfig())
	formatted := service.formatCLIAccess(ResourceTypeDocker, "redis", "")
	if !strings.Contains(formatted, "redis") || !strings.Contains(formatted, "...") {
		t.Fatalf("unexpected cli access: %s", formatted)
	}

	service.analysisCache = map[string]*AIAnalysisResponse{"nginx:latest": {ServiceType: "nginx"}}
	service.running = true
	status := service.GetStatus()
	if status["running"] != true || status["cache_size"] != 1 {
		t.Fatalf("unexpected status: %#v", status)
	}

	service.ClearCache()
	if len(service.analysisCache) != 0 {
		t.Fatalf("expected cache cleared")
	}
}

func TestService_DefaultsAndSetAnalyzer(t *testing.T) {
	service := NewService(nil, nil, nil, Config{})
	if service.interval == 0 || service.cacheExpiry == 0 {
		t.Fatalf("expected defaults for interval and cache expiry")
	}

	analyzer := &stubAnalyzer{response: `{}`}
	service.SetAIAnalyzer(analyzer)
	if service.aiAnalyzer == nil {
		t.Fatalf("expected analyzer set")
	}
	if service.GetProgress("missing") != nil {
		t.Fatalf("expected nil progress without scanner")
	}
	if service.getResourceMetadata(DiscoveryRequest{}) != nil {
		t.Fatalf("expected nil metadata without state provider")
	}
}

func TestService_RunBackgroundDiscoveryAndWrappers(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil
	state := StateSnapshot{
		DockerHosts: []DockerHost{
			{
				AgentID:  "host1",
				Hostname: "host1",
				Containers: []DockerContainer{
					{Name: "web", Image: "nginx:latest", Status: "running"},
				},
			},
		},
	}
	service := NewService(store, nil, stubStateProvider{state: state}, DefaultConfig())
	service.SetAIAnalyzer(&stubAnalyzer{
		response: `{"service_type":"nginx","service_name":"Nginx","service_version":"1.2","category":"web_server","cli_access":"docker exec {container} nginx -v","facts":[],"config_paths":[],"data_paths":[],"ports":[],"confidence":0.9,"reasoning":"image"}`,
	})

	service.runBackgroundDiscovery(context.Background())
	id := MakeResourceID(ResourceTypeDocker, "host1", "web")

	if got, err := service.GetDiscovery(id); err != nil || got == nil {
		t.Fatalf("GetDiscovery error: %v", err)
	}
	if got, err := service.GetDiscoveryByResource(ResourceTypeDocker, "host1", "web"); err != nil || got == nil {
		t.Fatalf("GetDiscoveryByResource error: %v", err)
	}

	if list, err := service.ListDiscoveries(); err != nil || len(list) != 1 {
		t.Fatalf("ListDiscoveries unexpected: %v len=%d", err, len(list))
	}
	if list, err := service.ListDiscoveriesByType(ResourceTypeDocker); err != nil || len(list) != 1 {
		t.Fatalf("ListDiscoveriesByType unexpected: %v len=%d", err, len(list))
	}
	if list, err := service.ListDiscoveriesByHost("host1"); err != nil || len(list) != 1 {
		t.Fatalf("ListDiscoveriesByHost unexpected: %v len=%d", err, len(list))
	}

	if err := service.UpdateNotes(id, "note", map[string]string{"k": "v"}); err != nil {
		t.Fatalf("UpdateNotes error: %v", err)
	}
	updated, err := service.GetDiscovery(id)
	if err != nil || updated.UserNotes != "note" {
		t.Fatalf("expected updated notes: %#v err=%v", updated, err)
	}

	scanner := NewDeepScanner(&stubExecutor{})
	scanner.progress[id] = &DiscoveryProgress{ResourceID: id}
	service.scanner = scanner
	if service.GetProgress(id) == nil {
		t.Fatalf("expected progress")
	}

	if err := service.DeleteDiscovery(id); err != nil {
		t.Fatalf("DeleteDiscovery error: %v", err)
	}

	service.stateProvider = nil
	service.runBackgroundDiscovery(context.Background())
}

func TestService_PromptsAndDiscoveryLoop(t *testing.T) {
	service := NewService(nil, nil, nil, DefaultConfig())

	container := DockerContainer{
		Name:   "web",
		Image:  "nginx:latest",
		Status: "running",
		Ports: []DockerPort{
			{PublicPort: 8080, PrivatePort: 80, Protocol: "tcp"},
		},
		Labels: map[string]string{"app": "nginx"},
		Mounts: []DockerMount{{Destination: "/etc/nginx"}},
	}
	host := DockerHost{Hostname: "host1"}
	prompt := service.buildMetadataAnalysisPrompt(container, host)
	if !strings.Contains(prompt, "\"ports\"") || !strings.Contains(prompt, "\"labels\"") || !strings.Contains(prompt, "\"mounts\"") {
		t.Fatalf("unexpected metadata prompt: %s", prompt)
	}

	longOutput := strings.Repeat("a", 2100)
	deepPrompt := service.buildDeepAnalysisPrompt(AIAnalysisRequest{
		ResourceType: ResourceTypeDocker,
		ResourceID:   "web",
		HostID:       "host1",
		Hostname:     "host1",
		Metadata:     map[string]any{"image": "nginx"},
		CommandOutputs: map[string]string{
			"ps": longOutput,
		},
	})
	if !strings.Contains(deepPrompt, "(truncated)") || !strings.Contains(deepPrompt, "Metadata:") {
		t.Fatalf("unexpected deep prompt")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service.initialDelay = time.Millisecond
	service.Start(ctx)
	service.Start(ctx)
	service.Stop()

	service.stopCh = make(chan struct{})
	close(service.stopCh)
	service.discoveryLoop(context.Background())

	service.initialDelay = 0
	service.stopCh = make(chan struct{})
	close(service.stopCh)
	service.discoveryLoop(context.Background())
}

func TestService_DiscoveryLoop_StopAndCancel(t *testing.T) {
	state := StateSnapshot{
		DockerHosts: []DockerHost{
			{
				AgentID:  "host1",
				Hostname: "host1",
				Containers: []DockerContainer{
					{Name: "web", Image: "nginx:latest", Status: "running"},
				},
			},
		},
	}

	runLoop := func(stopWithCancel bool) {
		store, err := NewStore(t.TempDir())
		if err != nil {
			t.Fatalf("NewStore error: %v", err)
		}
		store.crypto = nil

		service := NewService(store, nil, stubStateProvider{state: state}, DefaultConfig())
		analyzer := &stubAnalyzer{
			response: `{"service_type":"nginx","service_name":"Nginx","service_version":"1.2","category":"web_server","cli_access":"docker exec {container} nginx -v","facts":[],"config_paths":[],"data_paths":[],"ports":[],"confidence":0.9,"reasoning":"image"}`,
		}
		service.SetAIAnalyzer(analyzer)
		service.initialDelay = time.Millisecond
		service.interval = time.Millisecond
		service.cacheExpiry = time.Nanosecond

		done := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			service.discoveryLoop(ctx)
			close(done)
		}()

		time.Sleep(5 * time.Millisecond)
		if stopWithCancel {
			cancel()
		} else {
			close(service.stopCh)
		}

		select {
		case <-done:
		case <-time.After(50 * time.Millisecond):
			t.Fatalf("discoveryLoop did not stop")
		}

		analyzer.mu.Lock()
		calls := analyzer.calls
		analyzer.mu.Unlock()
		if calls < 2 {
			t.Fatalf("expected multiple discoveries, got %d", calls)
		}
	}

	runLoop(false)
	runLoop(true)
}

func TestService_DiscoverDockerContainersSkips(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	service := NewService(store, nil, nil, DefaultConfig())
	service.discoverDockerContainers(context.Background(), []DockerHost{{AgentID: "host1"}})

	service.SetAIAnalyzer(&stubAnalyzer{
		response: `{"service_type":"nginx","service_name":"Nginx","service_version":"1.2","category":"web_server","cli_access":"docker exec {container} nginx -v","facts":[],"config_paths":[],"data_paths":[],"ports":[],"confidence":0.9,"reasoning":"image"}`,
	})

	id := MakeResourceID(ResourceTypeDocker, "host1", "web")
	if err := store.Save(&ResourceDiscovery{ID: id, ResourceType: ResourceTypeDocker}); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	service.cacheExpiry = time.Hour
	service.discoverDockerContainers(context.Background(), []DockerHost{
		{AgentID: "host1", Containers: []DockerContainer{{Name: "web", Image: "nginx:latest"}}},
	})

	badAnalyzer := &stubAnalyzer{response: "not json"}
	if got := service.analyzeDockerContainer(context.Background(), badAnalyzer, DockerContainer{Name: "bad", Image: "bad"}, DockerHost{AgentID: "host1"}); got != nil {
		t.Fatalf("expected nil for bad analysis")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	analyzer := &stubAnalyzer{response: `{"service_type":"nginx","service_name":"Nginx","service_version":"1.2","category":"web_server","cli_access":"docker exec {container} nginx -v","facts":[],"config_paths":[],"data_paths":[],"ports":[],"confidence":0.9,"reasoning":"image"}`}
	service.SetAIAnalyzer(analyzer)
	service.discoverDockerContainers(canceled, []DockerHost{
		{AgentID: "host1", Containers: []DockerContainer{{Name: "web2", Image: "nginx:latest"}}},
	})
	analyzer.mu.Lock()
	calls := analyzer.calls
	analyzer.mu.Unlock()
	if calls != 0 {
		t.Fatalf("expected analyzer not called on canceled context")
	}

	errAnalyzer := errorAnalyzer{}
	if got := service.analyzeDockerContainer(context.Background(), errAnalyzer, DockerContainer{Name: "err", Image: "err"}, DockerHost{AgentID: "host1"}); got != nil {
		t.Fatalf("expected nil when analyzer returns error")
	}

	storePath := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(storePath, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	service.store.dataDir = storePath
	service.discoverDockerContainers(context.Background(), []DockerHost{
		{AgentID: "host1", Containers: []DockerContainer{{Name: "web3", Image: "nginx:latest"}}},
	})
}

func TestService_RunBackgroundDiscoveryRecover(t *testing.T) {
	service := NewService(nil, nil, panicStateProvider{}, DefaultConfig())
	service.runBackgroundDiscovery(context.Background())
}

func TestService_DiscoverResource_SaveError(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	badPath := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(badPath, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	store.dataDir = badPath

	service := NewService(store, nil, nil, DefaultConfig())
	service.SetAIAnalyzer(&stubAnalyzer{
		response: `{"service_type":"nginx","service_name":"Nginx","service_version":"1.2","category":"web_server","cli_access":"docker exec {container} nginx -v","facts":[],"config_paths":[],"data_paths":[],"ports":[],"confidence":0.9,"reasoning":"image"}`,
	})

	_, err = service.DiscoverResource(context.Background(), DiscoveryRequest{
		ResourceType: ResourceTypeDocker,
		ResourceID:   "web",
		HostID:       "host1",
		Hostname:     "host1",
		Force:        true,
	})
	if err == nil || !strings.Contains(err.Error(), "failed to save discovery") {
		t.Fatalf("expected save error, got %v", err)
	}
}

func TestService_DiscoverResource_ScanError(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	scanner := NewDeepScanner(nil)
	service := NewService(store, scanner, nil, DefaultConfig())
	service.SetAIAnalyzer(&stubAnalyzer{
		response: `{"service_type":"nginx","service_name":"Nginx","service_version":"1.2","category":"web_server","cli_access":"docker exec {container} nginx -v","facts":[],"config_paths":[],"data_paths":[],"ports":[],"confidence":0.9,"reasoning":"image"}`,
	})

	_, err = service.DiscoverResource(context.Background(), DiscoveryRequest{
		ResourceType: ResourceTypeDocker,
		ResourceID:   "web",
		HostID:       "host1",
		Hostname:     "host1",
		Force:        true,
	})
	if err != nil {
		t.Fatalf("expected scan error to be tolerated, got %v", err)
	}
}

func TestService_DiscoveryLoop_ContextDoneAtStart(t *testing.T) {
	service := NewService(nil, nil, nil, DefaultConfig())
	service.initialDelay = time.Hour
	service.stopCh = make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	service.discoveryLoop(ctx)
}

func TestService_DiscoverResource_WithScanResult(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	exec := &stubExecutor{
		agents: []ConnectedAgent{{AgentID: "host1", Hostname: "host1"}},
	}
	scanner := NewDeepScanner(exec)
	scanner.maxParallel = 1

	state := StateSnapshot{
		DockerHosts: []DockerHost{
			{
				AgentID:  "host1",
				Hostname: "host1",
				Containers: []DockerContainer{
					{Name: "web", Image: "nginx:latest", Status: "running"},
				},
			},
		},
	}

	service := NewService(store, scanner, stubStateProvider{state: state}, DefaultConfig())
	service.SetAIAnalyzer(&stubAnalyzer{
		response: `{"service_type":"nginx","service_name":"Nginx","service_version":"1.2","category":"web_server","cli_access":"docker exec {container} nginx -v","facts":[],"config_paths":[],"data_paths":[],"ports":[{"port":80,"protocol":"tcp","process":"nginx","address":"0.0.0.0"}],"confidence":0.9,"reasoning":"image"}`,
	})

	existing := &ResourceDiscovery{
		ID:           MakeResourceID(ResourceTypeDocker, "host1", "web"),
		ResourceType: ResourceTypeDocker,
		ResourceID:   "web",
		HostID:       "host1",
		Hostname:     "host1",
		UserNotes:    "keep",
		UserSecrets:  map[string]string{"token": "secret"},
		DiscoveredAt: time.Now().Add(-2 * time.Hour),
	}
	if err := store.Save(existing); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	found, err := service.DiscoverResource(context.Background(), DiscoveryRequest{
		ResourceType: ResourceTypeDocker,
		ResourceID:   "web",
		HostID:       "host1",
		Hostname:     "host1",
		Force:        true,
	})
	if err != nil {
		t.Fatalf("DiscoverResource error: %v", err)
	}
	if found.UserNotes != "keep" || found.UserSecrets["token"] != "secret" {
		t.Fatalf("expected user fields preserved: %#v", found)
	}
	if len(found.RawCommandOutput) == 0 {
		t.Fatalf("expected raw command output")
	}
	if found.DiscoveredAt.After(existing.DiscoveredAt) {
		t.Fatalf("expected older discovered_at preserved")
	}
}
