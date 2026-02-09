package servicediscovery

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

func TestFilterSensitiveLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		wantKeys map[string]string // expected values (use "[REDACTED]" for sensitive ones)
	}{
		{
			name:     "nil labels",
			labels:   nil,
			wantKeys: nil,
		},
		{
			name:     "empty labels",
			labels:   map[string]string{},
			wantKeys: map[string]string{},
		},
		{
			name: "safe labels only",
			labels: map[string]string{
				"app":     "myapp",
				"version": "1.0.0",
				"env":     "production",
			},
			wantKeys: map[string]string{
				"app":     "myapp",
				"version": "1.0.0",
				"env":     "production",
			},
		},
		{
			name: "redacts PASSWORD labels",
			labels: map[string]string{
				"app":            "myapp",
				"DB_PASSWORD":    "super-secret",
				"mysql_password": "another-secret",
				"PASSWORD_FILE":  "/secrets/pass",
			},
			wantKeys: map[string]string{
				"app":            "myapp",
				"DB_PASSWORD":    "[REDACTED]",
				"mysql_password": "[REDACTED]",
				"PASSWORD_FILE":  "[REDACTED]",
			},
		},
		{
			name: "redacts SECRET labels",
			labels: map[string]string{
				"app":            "myapp",
				"AWS_SECRET_KEY": "secret123",
				"client_secret":  "xyz",
			},
			wantKeys: map[string]string{
				"app":            "myapp",
				"AWS_SECRET_KEY": "[REDACTED]",
				"client_secret":  "[REDACTED]",
			},
		},
		{
			name: "redacts TOKEN labels",
			labels: map[string]string{
				"app":          "myapp",
				"ACCESS_TOKEN": "tok_123",
				"oauth_token":  "tok_456",
			},
			wantKeys: map[string]string{
				"app":          "myapp",
				"ACCESS_TOKEN": "[REDACTED]",
				"oauth_token":  "[REDACTED]",
			},
		},
		{
			name: "redacts API KEY labels",
			labels: map[string]string{
				"app":            "myapp",
				"API_KEY":        "key123",
				"openai_apikey":  "sk-123",
				"stripe_api_key": "sk_live_123",
			},
			wantKeys: map[string]string{
				"app":            "myapp",
				"API_KEY":        "[REDACTED]",
				"openai_apikey":  "[REDACTED]",
				"stripe_api_key": "[REDACTED]",
			},
		},
		{
			name: "redacts CREDENTIAL labels",
			labels: map[string]string{
				"app":            "myapp",
				"DB_CREDENTIALS": "user:pass",
				"admin_cred":     "admin123",
			},
			wantKeys: map[string]string{
				"app":            "myapp",
				"DB_CREDENTIALS": "[REDACTED]",
				"admin_cred":     "[REDACTED]",
			},
		},
		{
			name: "redacts AUTH labels",
			labels: map[string]string{
				"app":        "myapp",
				"auth_code":  "abc123",
				"BASIC_AUTH": "dXNlcjpwYXNz",
			},
			wantKeys: map[string]string{
				"app":        "myapp",
				"auth_code":  "[REDACTED]",
				"BASIC_AUTH": "[REDACTED]",
			},
		},
		{
			name: "mixed sensitive and safe labels",
			labels: map[string]string{
				"app":             "myapp",
				"version":         "2.0",
				"maintainer":      "team@example.com",
				"DB_PASSWORD":     "secret",
				"API_KEY":         "key123",
				"prometheus_port": "9090",
			},
			wantKeys: map[string]string{
				"app":             "myapp",
				"version":         "2.0",
				"maintainer":      "team@example.com",
				"DB_PASSWORD":     "[REDACTED]",
				"API_KEY":         "[REDACTED]",
				"prometheus_port": "9090",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterSensitiveLabels(tt.labels)

			if tt.wantKeys == nil {
				if got != nil {
					t.Errorf("filterSensitiveLabels() = %v, want nil", got)
				}
				return
			}

			if len(got) != len(tt.wantKeys) {
				t.Errorf("filterSensitiveLabels() returned %d labels, want %d", len(got), len(tt.wantKeys))
			}

			for k, wantV := range tt.wantKeys {
				gotV, ok := got[k]
				if !ok {
					t.Errorf("filterSensitiveLabels() missing key %q", k)
					continue
				}
				if gotV != wantV {
					t.Errorf("filterSensitiveLabels()[%q] = %q, want %q", k, gotV, wantV)
				}
			}
		})
	}
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
	// New format is instructional, should mention the container name and pulse_control
	if !strings.Contains(formatted, "redis") || !strings.Contains(formatted, "docker exec") {
		t.Fatalf("unexpected cli access: %s", formatted)
	}

	service.analysisCache = map[string]*analysisCacheEntry{
		"nginx:latest": {
			result:   &AIAnalysisResponse{ServiceType: "nginx"},
			cachedAt: time.Now(),
		},
	}
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

func TestService_FingerprintCollectionAndDiscoveryWrappers(t *testing.T) {
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

	// First, collect fingerprints (no AI calls)
	service.collectFingerprints(context.Background())

	// Verify fingerprint was collected (key format is type:host:id)
	fp, err := store.GetFingerprint("docker:host1:web")
	if err != nil {
		t.Fatalf("GetFingerprint error: %v", err)
	}
	if fp == nil {
		t.Fatalf("expected fingerprint to be collected")
	}

	// Now trigger on-demand discovery (this makes AI call)
	id := MakeResourceID(ResourceTypeDocker, "host1", "web")
	discovery, err := service.DiscoverResource(context.Background(), DiscoveryRequest{
		ResourceType: ResourceTypeDocker,
		ResourceID:   "web",
		HostID:       "host1",
		Hostname:     "host1",
	})
	if err != nil {
		t.Fatalf("DiscoverResource error: %v", err)
	}
	if discovery == nil {
		t.Fatalf("expected discovery result")
	}

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
	service.collectFingerprints(context.Background())
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

func TestService_FingerprintLoop_StopAndCancel(t *testing.T) {
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
		// Analyzer should be called by automatic refresh for changed/new resources.
		analyzer := &stubAnalyzer{
			response: `{"service_type":"nginx","service_name":"Nginx","service_version":"1.2","category":"web_server","cli_access":"docker exec {container} nginx -v","facts":[],"config_paths":[],"data_paths":[],"ports":[],"confidence":0.9,"reasoning":"image"}`,
		}
		service.SetAIAnalyzer(analyzer)
		service.initialDelay = time.Millisecond
		service.interval = time.Millisecond
		service.cacheExpiry = time.Nanosecond

		done := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel() // Always cancel to prevent context leak
		go func() {
			service.discoveryLoop(ctx)
			close(done)
		}()

		// Wait for at least one automatic refresh cycle to run.
		calls := 0
		deadline := time.Now().Add(200 * time.Millisecond)
		for time.Now().Before(deadline) {
			analyzer.mu.Lock()
			calls = analyzer.calls
			analyzer.mu.Unlock()
			if calls > 0 {
				break
			}
			time.Sleep(2 * time.Millisecond)
		}
		if calls == 0 {
			t.Fatalf("expected automatic discovery refresh to invoke AI analyzer")
		}

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

		// Verify fingerprints were collected.
		// Key format is type:host:id
		fp, err := store.GetFingerprint("docker:host1:web")
		if err != nil {
			t.Fatalf("GetFingerprint error: %v", err)
		}
		if fp == nil {
			t.Fatalf("expected fingerprint to be collected")
		}

		discovery, err := store.Get("docker:host1:web")
		if err != nil {
			t.Fatalf("Get discovery error: %v", err)
		}
		if discovery == nil {
			t.Fatalf("expected automatic discovery refresh to persist discovery data")
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

func TestService_CollectFingerprintsRecover(t *testing.T) {
	service := NewService(nil, nil, panicStateProvider{}, DefaultConfig())
	service.collectFingerprints(context.Background())
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

func TestParseDockerMounts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []DockerBindMount
	}{
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "no_docker_mounts marker",
			input:    "no_docker_mounts",
			expected: nil,
		},
		{
			name:     "only done marker",
			input:    "docker_mounts_done",
			expected: nil,
		},
		{
			name:  "single container with bind mount",
			input: "CONTAINER:homepage\n/home/user/homepage/config|/app/config|bind\ndocker_mounts_done",
			expected: []DockerBindMount{
				{ContainerName: "homepage", Source: "/home/user/homepage/config", Destination: "/app/config", Type: "bind"},
			},
		},
		{
			name:  "single container with volume",
			input: "CONTAINER:nginx\nnginx_data|/usr/share/nginx/html|volume\ndocker_mounts_done",
			expected: []DockerBindMount{
				{ContainerName: "nginx", Source: "nginx_data", Destination: "/usr/share/nginx/html", Type: "volume"},
			},
		},
		{
			name:  "multiple containers",
			input: "CONTAINER:homepage\n/home/user/config|/app/config|bind\nCONTAINER:watchtower\n/var/run/docker.sock|/var/run/docker.sock|bind\ndocker_mounts_done",
			expected: []DockerBindMount{
				{ContainerName: "homepage", Source: "/home/user/config", Destination: "/app/config", Type: "bind"},
				{ContainerName: "watchtower", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock", Type: "bind"},
			},
		},
		{
			name:  "container with multiple mounts",
			input: "CONTAINER:jellyfin\n/media/movies|/movies|bind\n/media/tv|/tv|bind\n/config/jellyfin|/config|bind\ndocker_mounts_done",
			expected: []DockerBindMount{
				{ContainerName: "jellyfin", Source: "/media/movies", Destination: "/movies", Type: "bind"},
				{ContainerName: "jellyfin", Source: "/media/tv", Destination: "/tv", Type: "bind"},
				{ContainerName: "jellyfin", Source: "/config/jellyfin", Destination: "/config", Type: "bind"},
			},
		},
		{
			name:     "container with no mounts",
			input:    "CONTAINER:alpine\ndocker_mounts_done",
			expected: nil,
		},
		{
			name:  "filters out tmpfs",
			input: "CONTAINER:app\n/data|/data|bind\n||tmpfs\ndocker_mounts_done",
			expected: []DockerBindMount{
				{ContainerName: "app", Source: "/data", Destination: "/data", Type: "bind"},
			},
		},
		{
			name:  "mount without type defaults to included",
			input: "CONTAINER:app\n/config|/app/config\ndocker_mounts_done",
			expected: []DockerBindMount{
				{ContainerName: "app", Source: "/config", Destination: "/app/config", Type: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDockerMounts(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d mounts, got %d: %#v", len(tt.expected), len(result), result)
			}
			for i := range tt.expected {
				if result[i].ContainerName != tt.expected[i].ContainerName {
					t.Errorf("mount %d: expected container %q, got %q", i, tt.expected[i].ContainerName, result[i].ContainerName)
				}
				if result[i].Source != tt.expected[i].Source {
					t.Errorf("mount %d: expected source %q, got %q", i, tt.expected[i].Source, result[i].Source)
				}
				if result[i].Destination != tt.expected[i].Destination {
					t.Errorf("mount %d: expected destination %q, got %q", i, tt.expected[i].Destination, result[i].Destination)
				}
				if result[i].Type != tt.expected[i].Type {
					t.Errorf("mount %d: expected type %q, got %q", i, tt.expected[i].Type, result[i].Type)
				}
			}
		})
	}
}

func TestService_Redirection(t *testing.T) {
	// Setup store
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	// Setup state with a linked PVE node
	state := StateSnapshot{
		Nodes: []Node{
			{
				ID:                "pve-id-1",
				Name:              "pve1",
				LinkedHostAgentID: "agent-pve1",
			},
		},
		Hosts: []Host{
			{
				ID:       "agent-pve1",
				Hostname: "pve1-host",
			},
		},
	}

	// Setup service
	service := NewService(store, nil, stubStateProvider{state: state}, DefaultConfig())
	service.SetAIAnalyzer(&stubAnalyzer{
		response: `{"service_type":"proxmox","service_name":"Proxmox VE","service_version":"8.0","category":"virtualizer","cli_access":"ssh root@pve1","facts":[],"config_paths":[],"data_paths":[],"ports":[],"confidence":0.9,"reasoning":"test"}`,
	})

	ctx := context.Background()

	// 1. Test DiscoverResource redirection
	// Trigger discovery for the PVE node "pve1"
	req := DiscoveryRequest{
		ResourceType: ResourceTypeHost,
		HostID:       "pve1",
		ResourceID:   "pve1",
		Force:        true,
	}

	discovery, err := service.DiscoverResource(ctx, req)
	if err != nil {
		t.Fatalf("DiscoverResource error: %v", err)
	}

	// The discovery should be associated with the AGENT ID, not the NODE ID
	expectedID := MakeResourceID(ResourceTypeHost, "agent-pve1", "agent-pve1") // Host resources usually have HostID == ResourceID
	if discovery.ID != expectedID {
		t.Errorf("DiscoverResource ID mismatch. Got %s, want %s (should have redirected to agent ID)", discovery.ID, expectedID)
	}
	if discovery.HostID != "agent-pve1" {
		t.Errorf("DiscoverResource HostID mismatch. Got %s, want agent-pve1", discovery.HostID)
	}

	// 2. Test GetDiscoveryByResource redirection (standard case)
	// Try to get discovery using the PVE node name
	got, err := service.GetDiscoveryByResource(ResourceTypeHost, "pve1", "pve1")
	if err != nil {
		t.Fatalf("GetDiscoveryByResource error: %v", err)
	}
	if got == nil {
		t.Fatalf("GetDiscoveryByResource returned nil")
	}

	// It should return the discovery we just created (which is under agent-pve1)
	if got.ID != expectedID {
		t.Errorf("GetDiscoveryByResource returned wrong discovery. Got ID %s, want %s", got.ID, expectedID)
	}

	// 3. Test GetDiscoveryByResource fallback
	// Create a "legacy" discovery that only exists under the node ID
	legacyID := MakeResourceID(ResourceTypeHost, "pve1", "pve1")
	legacyDiscovery := &ResourceDiscovery{
		ID:           legacyID,
		ResourceType: ResourceTypeHost,
		HostID:       "pve1",
		ResourceID:   "pve1",
		ServiceName:  "Legacy PVE",
	}
	if err := store.Save(legacyDiscovery); err != nil {
		t.Fatalf("Failed to save legacy discovery: %v", err)
	}

	// Temporarily remove the "agent" discovery to force fallback
	if err := store.Delete(expectedID); err != nil {
		t.Fatalf("Failed to delete agent discovery: %v", err)
	}

	// Try to get "pve1" again. It should redirect to agent-pve1 (not found), then fallback to pve1 (found)
	gotLegacy, err := service.GetDiscoveryByResource(ResourceTypeHost, "pve1", "pve1")
	if err != nil {
		t.Fatalf("GetDiscoveryByResource fallback error: %v", err)
	}
	if gotLegacy == nil {
		t.Fatalf("GetDiscoveryByResource fallback returned nil")
	}
	if gotLegacy.ID != legacyID {
		t.Errorf("GetDiscoveryByResource fallback returned wrong ID. Got %s, want %s", gotLegacy.ID, legacyID)
	}

	// 4. Test Deduplication
	// Restore the agent discovery, so we have BOTH legacy (pve1) and agent (agent-pve1)
	if err := store.Save(discovery); err != nil {
		t.Fatalf("Failed to restore agent discovery: %v", err)
	}

	list, err := service.ListDiscoveries()
	if err != nil {
		t.Fatalf("ListDiscoveries error: %v", err)
	}

	// We expect deduplication to remove the legacy pve1 entry because agent-pve1 exists
	foundLegacy := false
	foundAgent := false
	for _, d := range list {
		if d.ID == legacyID {
			foundLegacy = true
		}
		if d.ID == expectedID {
			foundAgent = true
		}
	}

	if foundLegacy {
		t.Errorf("Deduplication failed: Legacy PVE node discovery should have been filtered out")
	}
	if !foundAgent {
		t.Errorf("Deduplication failed: Agent discovery should be present")
	}
}
