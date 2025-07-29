package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// ConfigLoader handles loading configuration from multiple sources
type ConfigLoader struct {
	settings     *Settings
	configPaths  []string
	envPrefix    string
	cliArgs      map[string]interface{}
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{
		settings:  DefaultSettings(),
		envPrefix: "PULSE_",
		configPaths: []string{
			"/etc/pulse/pulse.yml",
			"/etc/pulse/pulse.yaml",
			"/etc/pulse/pulse.json",
			"./pulse.yml",
			"./pulse.yaml",
			"./pulse.json",
		},
		cliArgs: make(map[string]interface{}),
	}
}

// LoadConfig loads configuration from all sources in order of precedence
func (cl *ConfigLoader) LoadConfig() (*Settings, error) {
	// 1. Start with defaults (already set in NewConfigLoader)
	
	// 2. Load from config file
	if err := cl.loadFromFile(); err != nil {
		log.Warn().Err(err).Msg("Failed to load config file, using defaults")
	}

	// 3. Load from environment variables
	cl.loadFromEnv()

	// 4. Parse and apply CLI arguments
	cl.parseCliArgs()
	cl.applyCliArgs()

	// 5. Validate final configuration
	if err := cl.settings.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cl.settings, nil
}

// SetConfigPath adds a custom config path to search
func (cl *ConfigLoader) SetConfigPath(path string) {
	cl.configPaths = append([]string{path}, cl.configPaths...)
}

// loadFromFile loads configuration from the first found config file
func (cl *ConfigLoader) loadFromFile() error {
	var configPath string
	
	// Find the first existing config file
	for _, path := range cl.configPaths {
		if _, err := os.Stat(path); err == nil {
			configPath = path
			break
		}
	}

	if configPath == "" {
		return fmt.Errorf("no config file found")
	}

	log.Info().Str("path", configPath).Msg("Loading configuration file")

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse based on extension
	ext := strings.ToLower(filepath.Ext(configPath))
	switch ext {
	case ".yml", ".yaml":
		if err := yaml.Unmarshal(data, cl.settings); err != nil {
			return fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, cl.settings); err != nil {
			return fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}

	return nil
}

// loadFromEnv loads configuration from environment variables
func (cl *ConfigLoader) loadFromEnv() {
	// Server settings
	if val := os.Getenv(cl.envPrefix + "SERVER_BACKEND_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			cl.settings.Server.Backend.Port = port
		}
	}
	if val := os.Getenv(cl.envPrefix + "SERVER_BACKEND_HOST"); val != "" {
		cl.settings.Server.Backend.Host = val
	}
	if val := os.Getenv(cl.envPrefix + "SERVER_FRONTEND_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			cl.settings.Server.Frontend.Port = port
		}
	}
	if val := os.Getenv(cl.envPrefix + "SERVER_FRONTEND_HOST"); val != "" {
		cl.settings.Server.Frontend.Host = val
	}

	// Monitoring settings
	if val := os.Getenv(cl.envPrefix + "MONITORING_POLLING_INTERVAL"); val != "" {
		if interval, err := strconv.Atoi(val); err == nil {
			cl.settings.Monitoring.PollingInterval = interval
		}
	}
	if val := os.Getenv(cl.envPrefix + "MONITORING_CONCURRENT_POLLING"); val != "" {
		cl.settings.Monitoring.ConcurrentPolling = strings.ToLower(val) == "true"
	}
	if val := os.Getenv(cl.envPrefix + "MONITORING_BACKUP_POLLING_CYCLES"); val != "" {
		if cycles, err := strconv.Atoi(val); err == nil {
			cl.settings.Monitoring.BackupPollingCycles = cycles
		}
	}

	// Logging settings
	if val := os.Getenv(cl.envPrefix + "LOG_LEVEL"); val != "" {
		cl.settings.Logging.Level = strings.ToLower(val)
	}
	if val := os.Getenv(cl.envPrefix + "LOG_FILE"); val != "" {
		cl.settings.Logging.File = val
	}

	// Security settings
	if val := os.Getenv(cl.envPrefix + "API_TOKEN"); val != "" {
		cl.settings.Security.APIToken = val
	}
	if val := os.Getenv(cl.envPrefix + "ALLOWED_ORIGINS"); val != "" {
		cl.settings.Security.AllowedOrigins = strings.Split(val, ",")
	}
}

// parseCliArgs parses command-line arguments
func (cl *ConfigLoader) parseCliArgs() {
	// Check if flags have already been parsed (to avoid re-parsing in API calls)
	if flag.Parsed() {
		// If already parsed, try to get values from existing flags
		flag.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "backend-port":
				if port, err := strconv.Atoi(f.Value.String()); err == nil && port > 0 {
					cl.cliArgs["backend-port"] = port
				}
			case "backend-host":
				if f.Value.String() != "" {
					cl.cliArgs["backend-host"] = f.Value.String()
				}
			case "frontend-port":
				if port, err := strconv.Atoi(f.Value.String()); err == nil && port > 0 {
					cl.cliArgs["frontend-port"] = port
				}
			case "frontend-host":
				if f.Value.String() != "" {
					cl.cliArgs["frontend-host"] = f.Value.String()
				}
			case "log-level":
				if f.Value.String() != "" {
					cl.cliArgs["log-level"] = f.Value.String()
				}
			case "log-file":
				if f.Value.String() != "" {
					cl.cliArgs["log-file"] = f.Value.String()
				}
			}
		})
		return
	}

	// Define flags only if not already parsed
	backendPort := flag.Int("backend-port", 0, "Backend server port")
	backendHost := flag.String("backend-host", "", "Backend server host")
	frontendPort := flag.Int("frontend-port", 0, "Frontend server port")
	frontendHost := flag.String("frontend-host", "", "Frontend server host")
	configPath := flag.String("config", "", "Path to configuration file")
	logLevel := flag.String("log-level", "", "Log level (debug, info, warn, error)")
	logFile := flag.String("log-file", "", "Path to log file")

	// Custom usage
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment variables:\n")
		fmt.Fprintf(os.Stderr, "  PULSE_SERVER_BACKEND_PORT     Backend port (default: 3000)\n")
		fmt.Fprintf(os.Stderr, "  PULSE_SERVER_FRONTEND_PORT    Frontend port (default: 7655)\n")
		fmt.Fprintf(os.Stderr, "  PULSE_LOG_LEVEL              Log level (default: info)\n")
		fmt.Fprintf(os.Stderr, "  ... and more (see documentation)\n")
	}

	// Parse
	flag.Parse()

	// Store non-zero values
	if *backendPort > 0 {
		cl.cliArgs["backend-port"] = *backendPort
	}
	if *backendHost != "" {
		cl.cliArgs["backend-host"] = *backendHost
	}
	if *frontendPort > 0 {
		cl.cliArgs["frontend-port"] = *frontendPort
	}
	if *frontendHost != "" {
		cl.cliArgs["frontend-host"] = *frontendHost
	}
	if *configPath != "" {
		cl.SetConfigPath(*configPath)
	}
	if *logLevel != "" {
		cl.cliArgs["log-level"] = *logLevel
	}
	if *logFile != "" {
		cl.cliArgs["log-file"] = *logFile
	}
}

// applyCliArgs applies CLI arguments (highest priority)
func (cl *ConfigLoader) applyCliArgs() {
	if port, ok := cl.cliArgs["backend-port"].(int); ok {
		cl.settings.Server.Backend.Port = port
	}
	if host, ok := cl.cliArgs["backend-host"].(string); ok {
		cl.settings.Server.Backend.Host = host
	}
	if port, ok := cl.cliArgs["frontend-port"].(int); ok {
		cl.settings.Server.Frontend.Port = port
	}
	if host, ok := cl.cliArgs["frontend-host"].(string); ok {
		cl.settings.Server.Frontend.Host = host
	}
	if level, ok := cl.cliArgs["log-level"].(string); ok {
		cl.settings.Logging.Level = level
	}
	if file, ok := cl.cliArgs["log-file"].(string); ok {
		cl.settings.Logging.File = file
	}
}

// GetEffectiveConfig returns a summary of the effective configuration and its sources
func (cl *ConfigLoader) GetEffectiveConfig() map[string]interface{} {
	return map[string]interface{}{
		"server": map[string]interface{}{
			"backend": map[string]interface{}{
				"port": cl.settings.Server.Backend.Port,
				"host": cl.settings.Server.Backend.Host,
			},
			"frontend": map[string]interface{}{
				"port": cl.settings.Server.Frontend.Port,
				"host": cl.settings.Server.Frontend.Host,
			},
		},
		"monitoring": map[string]interface{}{
			"pollingInterval":     cl.settings.Monitoring.PollingInterval,
			"backupPollingCycles": cl.settings.Monitoring.BackupPollingCycles,
		},
		"logging": map[string]interface{}{
			"level": cl.settings.Logging.Level,
			"file":  cl.settings.Logging.File,
		},
	}
}