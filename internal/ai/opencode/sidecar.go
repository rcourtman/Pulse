package opencode

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Sidecar manages the OpenCode server process
type Sidecar struct {
	mu sync.RWMutex

	cmd       *exec.Cmd
	port      int
	baseURL   string
	dataDir   string
	mcpURL    string // URL of Pulse's MCP server for OpenCode to connect to
	model     string // Model to use (format: provider/model-name)
	cancelCtx context.CancelFunc

	// API keys for AI providers
	anthropicAPIKey string
	openAIAPIKey    string
	deepSeekAPIKey  string
	geminiAPIKey    string

	started   bool
	healthy   bool
	lastCheck time.Time
}

// SidecarConfig contains configuration for the OpenCode sidecar
type SidecarConfig struct {
	// DataDir is where OpenCode stores its data
	DataDir string
	// MCPURL is the URL of Pulse's MCP server
	MCPURL string
	// Port to run OpenCode on (0 for auto-assign)
	Port int
	// Model to use (format: provider/model-name, e.g. "anthropic/claude-sonnet-4-5")
	Model string
	// API keys for AI providers
	AnthropicAPIKey string
	OpenAIAPIKey    string
	DeepSeekAPIKey  string
	GeminiAPIKey    string
}

// NewSidecar creates a new OpenCode sidecar manager
func NewSidecar(cfg SidecarConfig) *Sidecar {
	return &Sidecar{
		dataDir:         cfg.DataDir,
		mcpURL:          cfg.MCPURL,
		port:            cfg.Port,
		model:           cfg.Model,
		anthropicAPIKey: cfg.AnthropicAPIKey,
		openAIAPIKey:    cfg.OpenAIAPIKey,
		deepSeekAPIKey:  cfg.DeepSeekAPIKey,
		geminiAPIKey:    cfg.GeminiAPIKey,
	}
}

// Start launches the OpenCode server process
func (s *Sidecar) Start(ctx context.Context) error {
	// Acquire lock for initial setup
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}

	// Find a free port if not specified
	if s.port == 0 {
		port, err := findFreePort()
		if err != nil {
			s.mu.Unlock()
			return fmt.Errorf("failed to find free port: %w", err)
		}
		s.port = port
	}

	s.baseURL = fmt.Sprintf("http://127.0.0.1:%d", s.port)

	// Ensure data directory exists
	if s.dataDir != "" {
		if err := os.MkdirAll(s.dataDir, 0755); err != nil {
			s.mu.Unlock()
			return fmt.Errorf("failed to create data directory: %w", err)
		}
	}

	// Create OpenCode config with MCP server connection and model
	if s.dataDir != "" {
		configPath := s.dataDir + "/opencode.json"

		// Build config with optional model
		modelLine := ""
		if s.model != "" {
			modelLine = fmt.Sprintf(`  "model": "%s",
`, s.model)
		}

		mcpConfig := ""
		if s.mcpURL != "" {
			mcpConfig = fmt.Sprintf(`  "mcp": {
    "pulse": {
      "type": "remote",
      "url": "%s",
      "enabled": true
    }
  }`, s.mcpURL)
		}

		// Note: API keys are passed via environment variables (not in config file)
		// OpenCode reads them from ANTHROPIC_API_KEY, OPENAI_API_KEY, etc.
		providerConfig := ""

		config := fmt.Sprintf(`{
  "$schema": "https://opencode.ai/config.json",
%s%s%s
}`, modelLine, providerConfig, mcpConfig)

		if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
			log.Warn().Err(err).Msg("Failed to write OpenCode config")
		} else {
			log.Info().Str("config", configPath).Str("model", s.model).Str("mcpURL", s.mcpURL).Msg("Created OpenCode config")
		}
	}

	// Create a cancellable context for the process
	processCtx, cancel := context.WithCancel(ctx)
	s.cancelCtx = cancel

	// Build command - using npx to run opencode
	s.cmd = exec.CommandContext(processCtx,
		"npx", "-y", "opencode-ai@latest",
		"serve",
		"--port", fmt.Sprintf("%d", s.port),
		"--hostname", "127.0.0.1",
	)

	// Set working directory
	if s.dataDir != "" {
		s.cmd.Dir = s.dataDir
	}

	// Configure environment with API keys
	env := append(os.Environ(),
		fmt.Sprintf("OPENCODE_PORT=%d", s.port),
	)
	if s.anthropicAPIKey != "" {
		env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", s.anthropicAPIKey))
	}
	if s.openAIAPIKey != "" {
		env = append(env, fmt.Sprintf("OPENAI_API_KEY=%s", s.openAIAPIKey))
	}
	if s.deepSeekAPIKey != "" {
		env = append(env, fmt.Sprintf("DEEPSEEK_API_KEY=%s", s.deepSeekAPIKey))
	}
	if s.geminiAPIKey != "" {
		env = append(env, fmt.Sprintf("GEMINI_API_KEY=%s", s.geminiAPIKey))
		// OpenCode also accepts GOOGLE_GENERATIVE_AI_API_KEY - set both to ensure compatibility
		env = append(env, fmt.Sprintf("GOOGLE_GENERATIVE_AI_API_KEY=%s", s.geminiAPIKey))
	}
	s.cmd.Env = env

	// Capture output for debugging
	s.cmd.Stdout = &logWriter{prefix: "opencode", level: "info"}
	s.cmd.Stderr = &logWriter{prefix: "opencode", level: "error"}

	// Start the process
	if err := s.cmd.Start(); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to start opencode: %w", err)
	}

	s.started = true

	// Release lock before waitForReady (which also needs the lock)
	s.mu.Unlock()

	// Start health check goroutine
	go s.healthLoop(ctx)

	// Wait for server to be ready
	if err := s.waitForReady(ctx, 30*time.Second); err != nil {
		log.Error().Err(err).Msg("waitForReady failed")
		s.Stop()
		return fmt.Errorf("opencode failed to become ready: %w", err)
	}

	log.Info().
		Int("port", s.port).
		Str("url", s.baseURL).
		Msg("OpenCode sidecar started")

	return nil
}

// Stop shuts down the OpenCode server process
func (s *Sidecar) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	if s.cancelCtx != nil {
		s.cancelCtx()
	}

	if s.cmd != nil && s.cmd.Process != nil {
		// Give it a chance to shut down gracefully
		done := make(chan error, 1)
		go func() {
			done <- s.cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited
		case <-time.After(5 * time.Second):
			// Force kill
			_ = s.cmd.Process.Kill()
		}
	}

	s.started = false
	s.healthy = false
	log.Info().Msg("OpenCode sidecar stopped")

	return nil
}

// UpdateModel updates the model configuration
// Call this before Restart to use a new model
func (s *Sidecar) UpdateModel(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.model = model
}

// writeConfig writes the opencode.json config file
// This is called during Start and can be called separately for config updates
func (s *Sidecar) writeConfig() error {
	if s.dataDir == "" {
		return nil
	}

	configPath := s.dataDir + "/opencode.json"

	// Build config with optional model
	modelLine := ""
	if s.model != "" {
		modelLine = fmt.Sprintf(`  "model": "%s",
`, s.model)
	}

	mcpConfig := ""
	if s.mcpURL != "" {
		mcpConfig = fmt.Sprintf(`  "mcp": {
    "pulse": {
      "type": "remote",
      "url": "%s",
      "enabled": true
    }
  }`, s.mcpURL)
	}

	config := fmt.Sprintf(`{
  "$schema": "https://opencode.ai/config.json",
%s%s
}`, modelLine, mcpConfig)

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write OpenCode config: %w", err)
	}

	log.Info().Str("config", configPath).Str("model", s.model).Msg("Updated OpenCode config")
	return nil
}

// Restart stops and starts the sidecar
func (s *Sidecar) Restart(ctx context.Context) error {
	if err := s.Stop(); err != nil {
		log.Warn().Err(err).Msg("Error stopping sidecar during restart")
	}
	return s.Start(ctx)
}

// BaseURL returns the base URL of the OpenCode server
func (s *Sidecar) BaseURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.baseURL
}

// Port returns the port the OpenCode server is running on
func (s *Sidecar) Port() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.port
}

// IsHealthy returns whether the sidecar is healthy
func (s *Sidecar) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.healthy
}

// IsStarted returns whether the sidecar has been started
func (s *Sidecar) IsStarted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started
}

// waitForReady polls the health endpoint until the server is ready
func (s *Sidecar) waitForReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if s.checkHealth() {
			s.mu.Lock()
			s.healthy = true
			s.mu.Unlock()
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for opencode to be ready")
}

// checkHealth performs a health check against the OpenCode server
func (s *Sidecar) checkHealth() bool {
	client := newHTTPClient(5 * time.Second)
	resp, err := client.Get(s.baseURL + "/global/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

// healthLoop continuously monitors the sidecar health
func (s *Sidecar) healthLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			healthy := s.checkHealth()

			s.mu.Lock()
			wasHealthy := s.healthy
			s.healthy = healthy
			s.lastCheck = time.Now()
			s.mu.Unlock()

			if wasHealthy && !healthy {
				log.Warn().Msg("OpenCode sidecar became unhealthy, attempting restart")
				if err := s.Restart(ctx); err != nil {
					log.Error().Err(err).Msg("Failed to restart OpenCode sidecar")
				}
			}
		}
	}
}

// findFreePort finds an available TCP port
func findFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// logWriter writes process output to zerolog
type logWriter struct {
	prefix string
	level  string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	switch w.level {
	case "error":
		log.Error().Str("source", w.prefix).Msg(msg)
	default:
		log.Debug().Str("source", w.prefix).Msg(msg)
	}
	return len(p), nil
}
