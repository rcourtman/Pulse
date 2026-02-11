package infradiscovery

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// mockStateProvider implements StateProvider for testing
type mockStateProvider struct {
	state models.StateSnapshot
}

func (m *mockStateProvider) GetState() models.StateSnapshot {
	return m.state
}

// mockAIAnalyzer implements AIAnalyzer for testing
type mockAIAnalyzer struct {
	responses map[string]string // image -> response
	callCount int
}

func (m *mockAIAnalyzer) AnalyzeForDiscovery(ctx context.Context, prompt string) (string, error) {
	m.callCount++
	// Return a mock response based on what's in the prompt
	// In real tests, we'd parse the prompt to determine which container
	for image, response := range m.responses {
		if containsString(prompt, image) {
			return response, nil
		}
	}
	// Default unknown response
	return `{"service_type": "unknown", "service_name": "Unknown", "category": "unknown", "cli_command": "", "confidence": 0.3, "reasoning": "Could not identify"}`, nil
}

func containsString(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && (s == substr || strings.Contains(s, substr))
}

func TestNewService(t *testing.T) {
	provider := &mockStateProvider{}
	service := NewService(provider, nil, DefaultConfig())

	if service == nil {
		t.Fatal("NewService returned nil")
	}

	if service.interval != 5*time.Minute {
		t.Errorf("interval = %v, want 5m", service.interval)
	}

	if service.cacheExpiry != 1*time.Hour {
		t.Errorf("cacheExpiry = %v, want 1h", service.cacheExpiry)
	}
}

func TestParseAIResponse(t *testing.T) {
	service := &Service{}

	tests := []struct {
		name     string
		response string
		want     *DiscoveryResult
	}{
		{
			name: "valid JSON",
			response: `{
				"service_type": "postgres",
				"service_name": "PostgreSQL",
				"category": "database",
				"cli_command": "docker exec {container} psql -U postgres",
				"confidence": 0.95,
				"reasoning": "Image name contains postgres"
			}`,
			want: &DiscoveryResult{
				ServiceType: "postgres",
				ServiceName: "PostgreSQL",
				Category:    "database",
				CLICommand:  "docker exec {container} psql -U postgres",
				Confidence:  0.95,
				Reasoning:   "Image name contains postgres",
			},
		},
		{
			name:     "JSON in markdown code block",
			response: "```json\n{\"service_type\": \"redis\", \"service_name\": \"Redis\", \"category\": \"cache\", \"cli_command\": \"docker exec {container} redis-cli\", \"confidence\": 0.9, \"reasoning\": \"Redis image\"}\n```",
			want: &DiscoveryResult{
				ServiceType: "redis",
				ServiceName: "Redis",
				Category:    "cache",
				CLICommand:  "docker exec {container} redis-cli",
				Confidence:  0.9,
				Reasoning:   "Redis image",
			},
		},
		{
			name:     "invalid JSON",
			response: "not json at all",
			want:     nil,
		},
		{
			name: "JSON with extra text",
			response: `Here's my analysis:
			{"service_type": "nginx", "service_name": "Nginx", "category": "web", "cli_command": "", "confidence": 0.85, "reasoning": "Web server"}
			That's my answer.`,
			want: &DiscoveryResult{
				ServiceType: "nginx",
				ServiceName: "Nginx",
				Category:    "web",
				CLICommand:  "",
				Confidence:  0.85,
				Reasoning:   "Web server",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.parseAIResponse(tt.response)
			if tt.want == nil {
				if got != nil {
					t.Errorf("parseAIResponse() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("parseAIResponse() = nil, want non-nil")
			}
			if got.ServiceType != tt.want.ServiceType {
				t.Errorf("ServiceType = %q, want %q", got.ServiceType, tt.want.ServiceType)
			}
			if got.ServiceName != tt.want.ServiceName {
				t.Errorf("ServiceName = %q, want %q", got.ServiceName, tt.want.ServiceName)
			}
			if got.Category != tt.want.Category {
				t.Errorf("Category = %q, want %q", got.Category, tt.want.Category)
			}
			if got.CLICommand != tt.want.CLICommand {
				t.Errorf("CLICommand = %q, want %q", got.CLICommand, tt.want.CLICommand)
			}
		})
	}
}

func TestBuildContainerInfo(t *testing.T) {
	service := &Service{}

	container := models.DockerContainer{
		ID:     "abc123",
		Name:   "mydb",
		Image:  "postgres:14",
		Status: "running",
		Ports: []models.DockerContainerPort{
			{PublicPort: 5432, PrivatePort: 5432, Protocol: "tcp"},
		},
		Labels: map[string]string{
			"app": "database",
		},
		Mounts: []models.DockerContainerMount{
			{Destination: "/var/lib/postgresql/data"},
		},
		Networks: []models.DockerContainerNetworkLink{
			{Name: "backend"},
		},
	}

	info := service.buildContainerInfo(container)

	if info.Name != "mydb" {
		t.Errorf("Name = %q, want 'mydb'", info.Name)
	}
	if info.Image != "postgres:14" {
		t.Errorf("Image = %q, want 'postgres:14'", info.Image)
	}
	if len(info.Ports) != 1 {
		t.Errorf("Ports length = %d, want 1", len(info.Ports))
	}
	if info.Ports[0].ContainerPort != 5432 {
		t.Errorf("ContainerPort = %d, want 5432", info.Ports[0].ContainerPort)
	}
	if info.Labels["app"] != "database" {
		t.Errorf("Labels[app] = %q, want 'database'", info.Labels["app"])
	}
	if len(info.Mounts) != 1 || info.Mounts[0] != "/var/lib/postgresql/data" {
		t.Errorf("Mounts = %v, want [/var/lib/postgresql/data]", info.Mounts)
	}
}

func TestRunDiscovery_NoAnalyzer(t *testing.T) {
	provider := &mockStateProvider{
		state: models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					Hostname: "host1",
					Containers: []models.DockerContainer{
						{ID: "1", Name: "test", Image: "test:latest"},
					},
				},
			},
		},
	}

	service := NewService(provider, nil, DefaultConfig())
	// Don't set analyzer

	apps := service.RunDiscovery(context.Background())
	if apps != nil {
		t.Errorf("RunDiscovery() without analyzer should return nil, got %v", apps)
	}
}

func TestRunDiscovery_WithAnalyzer(t *testing.T) {
	provider := &mockStateProvider{
		state: models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					AgentID:  "agent-1",
					Hostname: "docker-host",
					Containers: []models.DockerContainer{
						{ID: "1", Name: "mydb", Image: "postgres:14"},
						{ID: "2", Name: "cache", Image: "redis:7"},
					},
				},
			},
		},
	}

	analyzer := &mockAIAnalyzer{
		responses: map[string]string{
			"postgres:14": `{"service_type": "postgres", "service_name": "PostgreSQL", "category": "database", "cli_command": "docker exec {container} psql -U postgres", "confidence": 0.95, "reasoning": "PostgreSQL database"}`,
			"redis:7":     `{"service_type": "redis", "service_name": "Redis", "category": "cache", "cli_command": "docker exec {container} redis-cli", "confidence": 0.9, "reasoning": "Redis cache"}`,
		},
	}

	service := NewService(provider, nil, DefaultConfig())
	service.SetAIAnalyzer(analyzer)

	apps := service.RunDiscovery(context.Background())

	if len(apps) != 2 {
		t.Fatalf("RunDiscovery() returned %d apps, want 2", len(apps))
	}

	// Check PostgreSQL was detected
	foundPostgres := false
	foundRedis := false
	for _, app := range apps {
		if app.Type == "postgres" {
			foundPostgres = true
			if app.ContainerName != "mydb" {
				t.Errorf("Postgres ContainerName = %q, want 'mydb'", app.ContainerName)
			}
			if app.CLIAccess != "docker exec mydb psql -U postgres" {
				t.Errorf("Postgres CLIAccess = %q, want 'docker exec mydb psql -U postgres'", app.CLIAccess)
			}
		}
		if app.Type == "redis" {
			foundRedis = true
			if app.ContainerName != "cache" {
				t.Errorf("Redis ContainerName = %q, want 'cache'", app.ContainerName)
			}
		}
	}

	if !foundPostgres {
		t.Error("PostgreSQL not detected")
	}
	if !foundRedis {
		t.Error("Redis not detected")
	}
}

func TestCaching(t *testing.T) {
	provider := &mockStateProvider{
		state: models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					AgentID:  "agent-1",
					Hostname: "host1",
					Containers: []models.DockerContainer{
						{ID: "1", Name: "db1", Image: "postgres:14"},
						{ID: "2", Name: "db2", Image: "postgres:14"}, // Same image
					},
				},
			},
		},
	}

	analyzer := &mockAIAnalyzer{
		responses: map[string]string{
			"postgres:14": `{"service_type": "postgres", "service_name": "PostgreSQL", "category": "database", "cli_command": "docker exec {container} psql", "confidence": 0.95, "reasoning": "PostgreSQL"}`,
		},
	}

	service := NewService(provider, nil, DefaultConfig())
	service.SetAIAnalyzer(analyzer)

	// First run
	service.RunDiscovery(context.Background())

	// Should have called AI once (cached for second container with same image)
	if analyzer.callCount != 1 {
		t.Errorf("First run: analyzer called %d times, want 1 (caching)", analyzer.callCount)
	}

	// Second run should use cache
	analyzer.callCount = 0
	service.RunDiscovery(context.Background())

	if analyzer.callCount != 0 {
		t.Errorf("Second run: analyzer called %d times, want 0 (should use cache)", analyzer.callCount)
	}

	// Clear cache and run again
	service.ClearCache()
	service.RunDiscovery(context.Background())

	if analyzer.callCount != 1 {
		t.Errorf("After cache clear: analyzer called %d times, want 1", analyzer.callCount)
	}
}

func TestGetStatus(t *testing.T) {
	provider := &mockStateProvider{}
	service := NewService(provider, nil, DefaultConfig())

	status := service.GetStatus()

	if status["running"] != false {
		t.Errorf("status['running'] = %v, want false", status["running"])
	}
	if status["ai_analyzer_set"] != false {
		t.Errorf("status['ai_analyzer_set'] = %v, want false", status["ai_analyzer_set"])
	}

	// Set analyzer
	service.SetAIAnalyzer(&mockAIAnalyzer{})
	status = service.GetStatus()

	if status["ai_analyzer_set"] != true {
		t.Errorf("status['ai_analyzer_set'] = %v, want true after setting analyzer", status["ai_analyzer_set"])
	}
}

func TestLowConfidenceFiltering(t *testing.T) {
	provider := &mockStateProvider{
		state: models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					AgentID:  "agent-1",
					Hostname: "host1",
					Containers: []models.DockerContainer{
						{ID: "1", Name: "mystery", Image: "custom/unknown:latest"},
					},
				},
			},
		},
	}

	analyzer := &mockAIAnalyzer{
		responses: map[string]string{
			"custom/unknown:latest": `{"service_type": "unknown", "service_name": "Unknown Service", "category": "unknown", "cli_command": "", "confidence": 0.3, "reasoning": "Cannot identify"}`,
		},
	}

	service := NewService(provider, nil, DefaultConfig())
	service.SetAIAnalyzer(analyzer)

	apps := service.RunDiscovery(context.Background())

	// Low confidence results should be filtered out
	if len(apps) != 0 {
		t.Errorf("RunDiscovery() returned %d apps, want 0 (low confidence should be filtered)", len(apps))
	}
}

func TestParseAIResponse_NormalizesUnsafeFields(t *testing.T) {
	service := &Service{}

	response := `{
		"service_type": "Postgres Admin",
		"service_name": "PostgreSQL\u0000\nCluster",
		"category": "Data Base",
		"cli_command": "docker exec {container} psql; rm -rf /",
		"confidence": 4.2,
		"reasoning": "line1\nline2"
	}`

	got := service.parseAIResponse(response)
	if got == nil {
		t.Fatal("parseAIResponse() = nil, want non-nil")
	}

	if got.ServiceType != "unknown" {
		t.Fatalf("ServiceType = %q, want unknown", got.ServiceType)
	}
	if got.ServiceName != "PostgreSQL Cluster" {
		t.Fatalf("ServiceName = %q, want sanitized value", got.ServiceName)
	}
	if got.Category != "unknown" {
		t.Fatalf("Category = %q, want unknown", got.Category)
	}
	if got.CLICommand != "" {
		t.Fatalf("CLICommand = %q, want empty for unsafe command", got.CLICommand)
	}
	if got.Confidence != 1 {
		t.Fatalf("Confidence = %v, want 1", got.Confidence)
	}
	if got.Reasoning != "line1 line2" {
		t.Fatalf("Reasoning = %q, want normalized single-line text", got.Reasoning)
	}
}

func TestParseAIResponse_RejectsOversizedPayload(t *testing.T) {
	service := &Service{}
	tooLarge := strings.Repeat("a", maxAIResponseLength+1)
	if got := service.parseAIResponse(tooLarge); got != nil {
		t.Fatalf("parseAIResponse() = %v, want nil for oversized payload", got)
	}
}

func TestRunDiscovery_ShellQuotesUnsafeContainerNameInCLI(t *testing.T) {
	provider := &mockStateProvider{
		state: models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					AgentID:  "agent-1",
					Hostname: "docker-host",
					Containers: []models.DockerContainer{
						{ID: "1", Name: "db;rm -rf /", Image: "postgres:14"},
					},
				},
			},
		},
	}

	analyzer := &mockAIAnalyzer{
		responses: map[string]string{
			"postgres:14": `{"service_type": "postgres", "service_name": "PostgreSQL", "category": "database", "cli_command": "docker exec {container} psql -U postgres", "confidence": 0.95, "reasoning": "PostgreSQL database"}`,
		},
	}

	service := NewService(provider, nil, DefaultConfig())
	service.SetAIAnalyzer(analyzer)

	apps := service.RunDiscovery(context.Background())
	if len(apps) != 1 {
		t.Fatalf("RunDiscovery() returned %d apps, want 1", len(apps))
	}
	if apps[0].CLIAccess != "docker exec 'db;rm -rf /' psql -U postgres" {
		t.Fatalf("CLIAccess = %q, expected shell-quoted unsafe container name", apps[0].CLIAccess)
	}
}

func TestBuildContainerInfo_SanitizesAndBoundsMetadata(t *testing.T) {
	service := &Service{}
	container := models.DockerContainer{
		Name:   "web\nnode",
		Image:  "nginx:latest",
		Status: "running\r\nok",
		Ports: []models.DockerContainerPort{
			{PublicPort: 8080, PrivatePort: 80, Protocol: "TCP"},
			{PublicPort: 1234, PrivatePort: 0, Protocol: "udp"},
			{PublicPort: 65536, PrivatePort: 70000, Protocol: "icmp"},
		},
		Labels: map[string]string{
			"app\nname": "nginx\tfrontend",
			"\n":        "drop-me",
		},
		Mounts: []models.DockerContainerMount{
			{Destination: "/var/log/nginx\n"},
			{Destination: ""},
		},
		Networks: []models.DockerContainerNetworkLink{
			{Name: "front\tend"},
			{Name: ""},
		},
	}

	info := service.buildContainerInfo(container)
	if info.Name != "web node" {
		t.Fatalf("Name = %q, want sanitized value", info.Name)
	}
	if info.Status != "running ok" {
		t.Fatalf("Status = %q, want sanitized value", info.Status)
	}
	if len(info.Ports) != 1 {
		t.Fatalf("Ports len = %d, want 1 valid port", len(info.Ports))
	}
	if info.Ports[0].Protocol != "tcp" {
		t.Fatalf("Protocol = %q, want tcp", info.Ports[0].Protocol)
	}
	if info.Labels["app name"] != "nginx frontend" {
		t.Fatalf("Labels[app name] = %q, want sanitized label", info.Labels["app name"])
	}
	if _, exists := info.Labels[""]; exists {
		t.Fatal("expected empty/invalid label key to be dropped")
	}
	if len(info.Mounts) != 1 || info.Mounts[0] != "/var/log/nginx" {
		t.Fatalf("Mounts = %#v, want sanitized mount destination", info.Mounts)
	}
	if len(info.Networks) != 1 || info.Networks[0] != "front end" {
		t.Fatalf("Networks = %#v, want sanitized network name", info.Networks)
	}
}
