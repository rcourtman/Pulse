// Package config manages Pulse configuration from multiple sources.
//
// Configuration File Separation:
//   - .env: Authentication credentials ONLY (PULSE_AUTH_USER, PULSE_AUTH_PASS, API_TOKEN)
//   - system.json: Application settings (polling interval, timeouts, update settings, etc.)
//   - nodes.enc: Encrypted node credentials (PVE/PBS passwords and tokens)
//
// This separation ensures security, clarity, and proper access control.
// See docs/CONFIGURATION.md for detailed documentation.
package config

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/rcourtman/pulse-go-rewrite/internal/auth"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// IsPasswordHashed checks if a string looks like a bcrypt hash
func IsPasswordHashed(password string) bool {
	// Bcrypt hashes start with $2a$, $2b$, or $2y$ and are 60 characters long
	// We check for >= 55 to catch truncated hashes and warn users
	if !strings.HasPrefix(password, "$2") {
		return false
	}

	length := len(password)
	if length == 60 {
		return true // Perfect bcrypt hash
	}

	// Warn about truncated or invalid hashes
	if length >= 55 && length < 60 {
		log.Error().
			Int("length", length).
			Str("hash_start", password[:20]+"...").
			Msg("Bcrypt hash appears truncated! Should be 60 characters. Password will be treated as plaintext.")
		return false // Treat as plaintext to force user to fix it
	}

	return false
}

// Config holds all application configuration
// NOTE: The envconfig tags are legacy and not used - configuration is loaded from encrypted JSON files
type Config struct {
	// Server settings
	BackendHost  string `envconfig:"BACKEND_HOST" default:"0.0.0.0"`
	BackendPort  int    `envconfig:"BACKEND_PORT" default:"3000"`
	FrontendHost string `envconfig:"FRONTEND_HOST" default:"0.0.0.0"`
	FrontendPort int    `envconfig:"FRONTEND_PORT" default:"7655"`
	ConfigPath   string `envconfig:"CONFIG_PATH" default:"/etc/pulse"`
	DataPath     string `envconfig:"DATA_PATH" default:"/var/lib/pulse"`
	PublicURL    string `envconfig:"PULSE_PUBLIC_URL" default:""` // Full URL to access Pulse (e.g., http://192.168.1.100:7655)

	// Proxmox VE connections
	PVEInstances []PVEInstance

	// Proxmox Backup Server connections
	PBSInstances []PBSInstance

	// Proxmox Mail Gateway connections
	PMGInstances []PMGInstance

	// Monitoring settings
	// Note: PVE polling is hardcoded to 10s since Proxmox cluster/resources endpoint only updates every 10s
	PBSPollingInterval   time.Duration `envconfig:"PBS_POLLING_INTERVAL"` // PBS polling interval (60s default)
	PMGPollingInterval   time.Duration `envconfig:"PMG_POLLING_INTERVAL"` // PMG polling interval (60s default)
	ConcurrentPolling    bool          `envconfig:"CONCURRENT_POLLING" default:"true"`
	ConnectionTimeout    time.Duration `envconfig:"CONNECTION_TIMEOUT" default:"45s"` // Increased for slow storage operations
	MetricsRetentionDays int           `envconfig:"METRICS_RETENTION_DAYS" default:"7"`
	BackupPollingCycles  int           `envconfig:"BACKUP_POLLING_CYCLES" default:"10"`
	WebhookBatchDelay    time.Duration `envconfig:"WEBHOOK_BATCH_DELAY" default:"10s"`

	// Logging settings
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`
	LogFile     string `envconfig:"LOG_FILE" default:""`
	LogMaxSize  int    `envconfig:"LOG_MAX_SIZE" default:"100"` // MB
	LogMaxAge   int    `envconfig:"LOG_MAX_AGE" default:"30"`   // days
	LogCompress bool   `envconfig:"LOG_COMPRESS" default:"true"`

	// Security settings
	APIToken             string `envconfig:"API_TOKEN"`
	APITokenEnabled      bool   `envconfig:"API_TOKEN_ENABLED" default:"false"`
	AuthUser             string `envconfig:"PULSE_AUTH_USER"`
	AuthPass             string `envconfig:"PULSE_AUTH_PASS"`
	DisableAuth          bool   `envconfig:"DISABLE_AUTH" default:"false"`
	DemoMode             bool   `envconfig:"DEMO_MODE" default:"false"` // Read-only demo mode
	AllowedOrigins       string `envconfig:"ALLOWED_ORIGINS" default:"*"`
	IframeEmbeddingAllow string `envconfig:"IFRAME_EMBEDDING_ALLOW" default:"SAMEORIGIN"`

	// Proxy authentication settings
	ProxyAuthSecret        string `envconfig:"PROXY_AUTH_SECRET"`
	ProxyAuthUserHeader    string `envconfig:"PROXY_AUTH_USER_HEADER"`
	ProxyAuthRoleHeader    string `envconfig:"PROXY_AUTH_ROLE_HEADER"`
	ProxyAuthRoleSeparator string `envconfig:"PROXY_AUTH_ROLE_SEPARATOR" default:"|"`
	ProxyAuthAdminRole     string `envconfig:"PROXY_AUTH_ADMIN_ROLE" default:"admin"`
	ProxyAuthLogoutURL     string `envconfig:"PROXY_AUTH_LOGOUT_URL"`

	// OIDC configuration
	OIDC *OIDCConfig `json:"-"`
	// HTTPS/TLS settings
	HTTPSEnabled bool   `envconfig:"HTTPS_ENABLED" default:"false"`
	TLSCertFile  string `envconfig:"TLS_CERT_FILE" default:""`
	TLSKeyFile   string `envconfig:"TLS_KEY_FILE" default:""`

	// Update settings
	UpdateChannel           string        `envconfig:"UPDATE_CHANNEL" default:"stable"`
	AutoUpdateEnabled       bool          `envconfig:"AUTO_UPDATE_ENABLED" default:"false"`
	AutoUpdateCheckInterval time.Duration `envconfig:"AUTO_UPDATE_CHECK_INTERVAL" default:"24h"`
	AutoUpdateTime          string        `envconfig:"AUTO_UPDATE_TIME" default:"03:00"`

	// Discovery settings
	DiscoveryEnabled bool   `envconfig:"DISCOVERY_ENABLED" default:"true"`
	DiscoverySubnet  string `envconfig:"DISCOVERY_SUBNET" default:"auto"`

	// Deprecated - for backward compatibility
	Port  int  `envconfig:"PORT"` // Maps to BackendPort
	Debug bool `envconfig:"DEBUG" default:"false"`

	// Track which settings are overridden by environment variables
	EnvOverrides map[string]bool `json:"-"`
}

// PVEInstance represents a Proxmox VE connection
type PVEInstance struct {
	Name                       string
	Host                       string // Primary endpoint (user-provided)
	User                       string
	Password                   string
	TokenName                  string
	TokenValue                 string
	Fingerprint                string
	VerifySSL                  bool
	MonitorVMs                 bool
	MonitorContainers          bool
	MonitorStorage             bool
	MonitorBackups             bool
	MonitorPhysicalDisks       *bool // Monitor physical disks (nil = enabled by default, can be explicitly disabled)
	PhysicalDiskPollingMinutes int   // How often to poll physical disks (0 = use default)

	// Cluster support
	IsCluster        bool              // True if this is a cluster
	ClusterName      string            // Cluster name if applicable
	ClusterEndpoints []ClusterEndpoint // All discovered cluster nodes
}

// ClusterEndpoint represents a single node in a cluster
type ClusterEndpoint struct {
	NodeID   string    // Node ID in cluster
	NodeName string    // Node name
	Host     string    // Full URL (e.g., https://node1.lan:8006)
	IP       string    // IP address
	Online   bool      // Current online status
	LastSeen time.Time // Last successful connection
}

// PBSInstance represents a Proxmox Backup Server connection
type PBSInstance struct {
	Name               string
	Host               string
	User               string
	Password           string
	TokenName          string
	TokenValue         string
	Fingerprint        string
	VerifySSL          bool
	MonitorBackups     bool
	MonitorDatastores  bool
	MonitorSyncJobs    bool
	MonitorVerifyJobs  bool
	MonitorPruneJobs   bool
	MonitorGarbageJobs bool
}

// PMGInstance represents a Proxmox Mail Gateway connection
type PMGInstance struct {
	Name        string
	Host        string
	User        string
	Password    string
	TokenName   string
	TokenValue  string
	Fingerprint string
	VerifySSL   bool

	MonitorMailStats   bool
	MonitorQueues      bool
	MonitorQuarantine  bool
	MonitorDomainStats bool
}

// Global persistence instance for saving
var globalPersistence *ConfigPersistence

// Load reads configuration from encrypted persistence files
func Load() (*Config, error) {
	// Get data directory from environment
	dataDir := "/etc/pulse"
	if dir := os.Getenv("PULSE_DATA_DIR"); dir != "" {
		dataDir = dir
	}

	// Load .env file if it exists (for deployment overrides)
	envFile := filepath.Join(dataDir, ".env")
	if _, err := os.Stat(envFile); err == nil {
		if err := godotenv.Load(envFile); err != nil {
			log.Warn().Err(err).Str("file", envFile).Msg("Failed to load .env file")
		} else {
			log.Info().Str("file", envFile).Msg("Loaded .env file for deployment overrides")
		}
	}

	// Also try loading from current directory for development
	if err := godotenv.Load(); err == nil {
		log.Info().Msg("Loaded configuration from .env in current directory")
	}

	// Load mock.env and mock.env.local for mock mode configuration
	mockEnv := "mock.env"
	mockEnvLocal := "mock.env.local"
	if _, err := os.Stat(mockEnv); err == nil {
		if err := godotenv.Load(mockEnv); err != nil {
			log.Warn().Err(err).Str("file", mockEnv).Msg("Failed to load mock.env file")
		} else {
			log.Info().Str("file", mockEnv).Msg("Loaded mock mode configuration")
		}
	}
	if _, err := os.Stat(mockEnvLocal); err == nil {
		if err := godotenv.Load(mockEnvLocal); err != nil {
			log.Warn().Err(err).Str("file", mockEnvLocal).Msg("Failed to load mock.env.local file")
		} else {
			log.Info().Str("file", mockEnvLocal).Msg("Loaded local mock mode overrides")
		}
	}

	// Initialize config with defaults
	cfg := &Config{
		BackendHost:          "0.0.0.0",
		BackendPort:          3000,
		FrontendHost:         "0.0.0.0",
		FrontendPort:         7655,
		ConfigPath:           dataDir,
		DataPath:             dataDir,
		ConcurrentPolling:    true,
		ConnectionTimeout:    60 * time.Second,
		MetricsRetentionDays: 7,
		BackupPollingCycles:  10,
		WebhookBatchDelay:    10 * time.Second,
		LogLevel:             "info",
		LogMaxSize:           100,
		LogMaxAge:            30,
		LogCompress:          true,
		AllowedOrigins:       "", // Empty means no CORS headers (same-origin only)
		IframeEmbeddingAllow: "SAMEORIGIN",
		PBSPollingInterval:   60 * time.Second, // Default PBS polling (slower)
		PMGPollingInterval:   60 * time.Second, // Default PMG polling (aggregated stats)
		DiscoveryEnabled:     true,
		DiscoverySubnet:      "auto",
		EnvOverrides:         make(map[string]bool),
		OIDC:                 NewOIDCConfig(),
	}

	// Initialize persistence
	persistence := NewConfigPersistence(dataDir)
	if persistence != nil {
		// Store global persistence for saving
		globalPersistence = persistence
		// Load nodes configuration
		if nodesConfig, err := persistence.LoadNodesConfig(); err == nil && nodesConfig != nil {
			cfg.PVEInstances = nodesConfig.PVEInstances
			cfg.PBSInstances = nodesConfig.PBSInstances
			cfg.PMGInstances = nodesConfig.PMGInstances
			log.Info().
				Int("pve", len(cfg.PVEInstances)).
				Int("pbs", len(cfg.PBSInstances)).
				Int("pmg", len(cfg.PMGInstances)).
				Msg("Loaded nodes configuration")
		} else if err != nil {
			log.Warn().Err(err).Msg("Failed to load nodes configuration")
		}

		// Load system configuration
		if systemSettings, err := persistence.LoadSystemSettings(); err == nil && systemSettings != nil {
			// Load PBS polling interval if configured
			if systemSettings.PBSPollingInterval > 0 {
				cfg.PBSPollingInterval = time.Duration(systemSettings.PBSPollingInterval) * time.Second
			}
			if systemSettings.PMGPollingInterval > 0 {
				cfg.PMGPollingInterval = time.Duration(systemSettings.PMGPollingInterval) * time.Second
			}

			if systemSettings.UpdateChannel != "" {
				cfg.UpdateChannel = systemSettings.UpdateChannel
			}
			cfg.AutoUpdateEnabled = systemSettings.AutoUpdateEnabled
			if systemSettings.AutoUpdateCheckInterval > 0 {
				cfg.AutoUpdateCheckInterval = time.Duration(systemSettings.AutoUpdateCheckInterval) * time.Hour
			}
			if systemSettings.AutoUpdateTime != "" {
				cfg.AutoUpdateTime = systemSettings.AutoUpdateTime
			}
			if systemSettings.AllowedOrigins != "" {
				cfg.AllowedOrigins = systemSettings.AllowedOrigins
			}
			if systemSettings.ConnectionTimeout > 0 {
				cfg.ConnectionTimeout = time.Duration(systemSettings.ConnectionTimeout) * time.Second
			}
			if systemSettings.LogLevel != "" {
				cfg.LogLevel = systemSettings.LogLevel
			}
			// Always load DiscoveryEnabled even if false
			cfg.DiscoveryEnabled = systemSettings.DiscoveryEnabled
			if systemSettings.DiscoverySubnet != "" {
				cfg.DiscoverySubnet = systemSettings.DiscoverySubnet
			}
			// APIToken no longer loaded from system.json - only from .env
			log.Info().
				Str("updateChannel", cfg.UpdateChannel).
				Str("logLevel", cfg.LogLevel).
				Msg("Loaded system configuration")
		} else {
			// No system.json exists - create default one
			log.Info().Msg("No system.json found, creating default")
			defaultSettings := SystemSettings{
				ConnectionTimeout: int(cfg.ConnectionTimeout.Seconds()),
				AutoUpdateEnabled: false,
			}
			if err := persistence.SaveSystemSettings(defaultSettings); err != nil {
				log.Warn().Err(err).Msg("Failed to create default system.json")
			}
		}

		if oidcSettings, err := persistence.LoadOIDCConfig(); err == nil && oidcSettings != nil {
			cfg.OIDC = oidcSettings
		} else if err != nil {
			log.Warn().Err(err).Msg("Failed to load OIDC configuration")
		}
	}

	// Ensure PBS polling interval has default if not set
	// Note: PVE polling is hardcoded to 10s in monitor.go
	if cfg.PBSPollingInterval == 0 {
		cfg.PBSPollingInterval = 60 * time.Second
	}
	if cfg.PMGPollingInterval == 0 {
		cfg.PMGPollingInterval = 60 * time.Second
	}

	// Limited environment variable support
	// NOTE: Node configuration is NOT done via env vars - use the web UI instead

	// Support both FRONTEND_PORT (preferred) and PORT (legacy) env vars
	if frontendPort := os.Getenv("FRONTEND_PORT"); frontendPort != "" {
		if p, err := strconv.Atoi(frontendPort); err == nil {
			cfg.FrontendPort = p
			log.Info().Int("port", p).Msg("Overriding frontend port from FRONTEND_PORT env var")
		}
	} else if port := os.Getenv("PORT"); port != "" {
		// Fall back to PORT for backwards compatibility
		if p, err := strconv.Atoi(port); err == nil {
			cfg.FrontendPort = p
			log.Info().Int("port", p).Msg("Overriding frontend port from PORT env var (legacy)")
		}
	}
	if apiToken := os.Getenv("API_TOKEN"); apiToken != "" {
		// Auto-hash plain text tokens for security
		if !auth.IsAPITokenHashed(apiToken) {
			// Plain text token - hash it immediately
			cfg.APIToken = auth.HashAPIToken(apiToken)
			log.Info().Msg("Auto-hashed plain text API token from environment variable")
		} else {
			// Already hashed
			cfg.APIToken = apiToken
			log.Debug().Msg("Loaded pre-hashed API token from env var")
		}
	}
	// Check if API token is enabled
	if apiTokenEnabled := os.Getenv("API_TOKEN_ENABLED"); apiTokenEnabled != "" {
		cfg.APITokenEnabled = apiTokenEnabled == "true" || apiTokenEnabled == "1"
		log.Debug().Bool("enabled", cfg.APITokenEnabled).Msg("API token enabled status from env var")
	} else if cfg.APIToken != "" {
		// If token exists but no explicit enabled flag, assume enabled for backwards compatibility
		cfg.APITokenEnabled = true
		log.Debug().Msg("API token exists without explicit enabled flag, assuming enabled for backwards compatibility")
	}
	// Check if auth is disabled
	disableAuthEnv := os.Getenv("DISABLE_AUTH")
	log.Debug().Str("DISABLE_AUTH_ENV", disableAuthEnv).Msg("Checking DISABLE_AUTH environment variable")
	if disableAuthEnv != "" {
		cfg.DisableAuth = disableAuthEnv == "true" || disableAuthEnv == "1"
		log.Debug().Bool("DisableAuth", cfg.DisableAuth).Msg("DisableAuth set from environment")
		if cfg.DisableAuth {
			log.Warn().Msg("⚠️  AUTHENTICATION DISABLED - Pulse is running without authentication!")
		}
	} else {
		log.Debug().Bool("DisableAuth", cfg.DisableAuth).Msg("DISABLE_AUTH not set, DisableAuth remains")
	}

	// Check if demo mode is enabled
	demoModeEnv := os.Getenv("DEMO_MODE")
	if demoModeEnv != "" {
		cfg.DemoMode = demoModeEnv == "true" || demoModeEnv == "1"
		if cfg.DemoMode {
			log.Warn().Msg("🎭 DEMO MODE - All modifications disabled (read-only)")
		}
	}

	// Load proxy authentication settings
	if proxyAuthSecret := os.Getenv("PROXY_AUTH_SECRET"); proxyAuthSecret != "" {
		cfg.ProxyAuthSecret = proxyAuthSecret
		log.Info().Msg("Proxy authentication secret configured")

		// Load other proxy auth settings
		if userHeader := os.Getenv("PROXY_AUTH_USER_HEADER"); userHeader != "" {
			cfg.ProxyAuthUserHeader = userHeader
			log.Info().Str("header", userHeader).Msg("Proxy auth user header configured")
		}
		if roleHeader := os.Getenv("PROXY_AUTH_ROLE_HEADER"); roleHeader != "" {
			cfg.ProxyAuthRoleHeader = roleHeader
			log.Info().Str("header", roleHeader).Msg("Proxy auth role header configured")
		}
		if roleSeparator := os.Getenv("PROXY_AUTH_ROLE_SEPARATOR"); roleSeparator != "" {
			cfg.ProxyAuthRoleSeparator = roleSeparator
			log.Info().Str("separator", roleSeparator).Msg("Proxy auth role separator configured")
		}
		if adminRole := os.Getenv("PROXY_AUTH_ADMIN_ROLE"); adminRole != "" {
			cfg.ProxyAuthAdminRole = adminRole
			log.Info().Str("role", adminRole).Msg("Proxy auth admin role configured")
		}
		if logoutURL := os.Getenv("PROXY_AUTH_LOGOUT_URL"); logoutURL != "" {
			cfg.ProxyAuthLogoutURL = logoutURL
			log.Info().Str("url", logoutURL).Msg("Proxy auth logout URL configured")
		}
	}

	oidcEnv := make(map[string]string)
	if val := os.Getenv("OIDC_ENABLED"); val != "" {
		oidcEnv["OIDC_ENABLED"] = val
	}
	if val := os.Getenv("OIDC_ISSUER_URL"); val != "" {
		oidcEnv["OIDC_ISSUER_URL"] = val
	}
	if val := os.Getenv("OIDC_CLIENT_ID"); val != "" {
		oidcEnv["OIDC_CLIENT_ID"] = val
	}
	if val := os.Getenv("OIDC_CLIENT_SECRET"); val != "" {
		oidcEnv["OIDC_CLIENT_SECRET"] = val
	}
	if val := os.Getenv("OIDC_REDIRECT_URL"); val != "" {
		oidcEnv["OIDC_REDIRECT_URL"] = val
	}
	if val := os.Getenv("OIDC_LOGOUT_URL"); val != "" {
		oidcEnv["OIDC_LOGOUT_URL"] = val
	}
	if val := os.Getenv("OIDC_SCOPES"); val != "" {
		oidcEnv["OIDC_SCOPES"] = val
	}
	if val := os.Getenv("OIDC_USERNAME_CLAIM"); val != "" {
		oidcEnv["OIDC_USERNAME_CLAIM"] = val
	}
	if val := os.Getenv("OIDC_EMAIL_CLAIM"); val != "" {
		oidcEnv["OIDC_EMAIL_CLAIM"] = val
	}
	if val := os.Getenv("OIDC_GROUPS_CLAIM"); val != "" {
		oidcEnv["OIDC_GROUPS_CLAIM"] = val
	}
	if val := os.Getenv("OIDC_ALLOWED_GROUPS"); val != "" {
		oidcEnv["OIDC_ALLOWED_GROUPS"] = val
	}
	if val := os.Getenv("OIDC_ALLOWED_DOMAINS"); val != "" {
		oidcEnv["OIDC_ALLOWED_DOMAINS"] = val
	}
	if val := os.Getenv("OIDC_ALLOWED_EMAILS"); val != "" {
		oidcEnv["OIDC_ALLOWED_EMAILS"] = val
	}
	if len(oidcEnv) > 0 {
		cfg.OIDC.MergeFromEnv(oidcEnv)
	}
	if authUser := os.Getenv("PULSE_AUTH_USER"); authUser != "" {
		cfg.AuthUser = authUser
		log.Info().Msg("Overriding auth user from env var")
	}
	if authPass := os.Getenv("PULSE_AUTH_PASS"); authPass != "" {
		// Auto-hash plain text passwords for security
		if !IsPasswordHashed(authPass) {
			// Plain text password - hash it immediately
			hashedPass, err := auth.HashPassword(authPass)
			if err != nil {
				log.Error().Err(err).Msg("Failed to hash password from environment variable")
				// Fall back to plain text if hashing fails (shouldn't happen)
				cfg.AuthPass = authPass
			} else {
				cfg.AuthPass = hashedPass
				log.Info().Msg("Auto-hashed plain text password from environment variable")
			}
		} else {
			// Already hashed - validate it's complete
			if len(authPass) != 60 {
				log.Error().Int("length", len(authPass)).Msg("Bcrypt hash appears truncated! Expected 60 characters. Authentication may fail.")
				log.Error().Msg("Ensure the full hash is enclosed in single quotes in your .env file or Docker environment")
			}
			cfg.AuthPass = authPass
			log.Debug().Msg("Loaded pre-hashed password from env var")
		}
	}

	// HTTPS/TLS configuration from environment
	if httpsEnabled := os.Getenv("HTTPS_ENABLED"); httpsEnabled != "" {
		cfg.HTTPSEnabled = httpsEnabled == "true" || httpsEnabled == "1"
		log.Debug().Bool("enabled", cfg.HTTPSEnabled).Msg("HTTPS enabled status from env var")
	}
	if tlsCertFile := os.Getenv("TLS_CERT_FILE"); tlsCertFile != "" {
		cfg.TLSCertFile = tlsCertFile
		log.Debug().Str("cert_file", tlsCertFile).Msg("TLS cert file from env var")
	}
	if tlsKeyFile := os.Getenv("TLS_KEY_FILE"); tlsKeyFile != "" {
		cfg.TLSKeyFile = tlsKeyFile
		log.Debug().Str("key_file", tlsKeyFile).Msg("TLS key file from env var")
	}

	// REMOVED: Update channel, auto-update, connection timeout, and allowed origins env vars
	// These settings now ONLY come from system.json to prevent confusion
	// Only keeping essential deployment/infrastructure env vars

	// Normalize PVE user fields for password authentication
	for i := range cfg.PVEInstances {
		if cfg.PVEInstances[i].TokenName == "" && cfg.PVEInstances[i].TokenValue == "" && cfg.PVEInstances[i].User != "" && !strings.Contains(cfg.PVEInstances[i].User, "@") {
			cfg.PVEInstances[i].User = cfg.PVEInstances[i].User + "@pam"
		}
	}

	if cfg.AllowedOrigins == "" {
		// If not configured and we're in development mode (different ports for frontend/backend)
		// allow localhost for development convenience
		if os.Getenv("NODE_ENV") == "development" || os.Getenv("PULSE_DEV") == "true" {
			cfg.AllowedOrigins = "http://localhost:5173,http://localhost:7655"
			log.Info().Msg("Development mode: allowing localhost origins")
		}
	}
	// Support env vars for important settings (override system.json)
	// NOTE: Environment variables always take precedence over UI/system.json settings
	if discoveryEnabled := os.Getenv("DISCOVERY_ENABLED"); discoveryEnabled != "" {
		cfg.DiscoveryEnabled = discoveryEnabled == "true" || discoveryEnabled == "1"
		cfg.EnvOverrides["discoveryEnabled"] = true
		log.Info().Bool("enabled", cfg.DiscoveryEnabled).Msg("Discovery enabled overridden by DISCOVERY_ENABLED env var")
	}
	if discoverySubnet := os.Getenv("DISCOVERY_SUBNET"); discoverySubnet != "" {
		cfg.DiscoverySubnet = discoverySubnet
		cfg.EnvOverrides["discoverySubnet"] = true
		log.Info().Str("subnet", discoverySubnet).Msg("Discovery subnet overridden by DISCOVERY_SUBNET env var")
	}
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
		cfg.EnvOverrides["logLevel"] = true
		log.Info().Str("level", logLevel).Msg("Log level overridden by LOG_LEVEL env var")
	}
	if connectionTimeout := os.Getenv("CONNECTION_TIMEOUT"); connectionTimeout != "" {
		if d, err := time.ParseDuration(connectionTimeout + "s"); err == nil {
			cfg.ConnectionTimeout = d
			cfg.EnvOverrides["connectionTimeout"] = true
			log.Info().Dur("timeout", d).Msg("Connection timeout overridden by CONNECTION_TIMEOUT env var")
		} else if d, err := time.ParseDuration(connectionTimeout); err == nil {
			cfg.ConnectionTimeout = d
			cfg.EnvOverrides["connectionTimeout"] = true
			log.Info().Dur("timeout", d).Msg("Connection timeout overridden by CONNECTION_TIMEOUT env var")
		}
	}
	if allowedOrigins := os.Getenv("ALLOWED_ORIGINS"); allowedOrigins != "" {
		cfg.AllowedOrigins = allowedOrigins
		cfg.EnvOverrides["allowedOrigins"] = true
		log.Info().Str("origins", allowedOrigins).Msg("Allowed origins overridden by ALLOWED_ORIGINS env var")
	}
	if publicURL := os.Getenv("PULSE_PUBLIC_URL"); publicURL != "" {
		cfg.PublicURL = publicURL
		log.Info().Str("url", publicURL).Msg("Public URL configured from PULSE_PUBLIC_URL env var")
	} else {
		// Try to auto-detect public URL if not explicitly configured
		if detectedURL := detectPublicURL(cfg.FrontendPort); detectedURL != "" {
			cfg.PublicURL = detectedURL
			log.Info().Str("url", detectedURL).Msg("Auto-detected public URL for webhook notifications")
		}
	}

	cfg.OIDC.ApplyDefaults(cfg.PublicURL)

	// Set log level
	switch cfg.LogLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel) // Default to info level
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// SaveConfig saves the configuration back to encrypted files
func SaveConfig(cfg *Config) error {
	if globalPersistence == nil {
		return fmt.Errorf("config persistence not initialized")
	}

	// Save nodes configuration
	if err := globalPersistence.SaveNodesConfig(cfg.PVEInstances, cfg.PBSInstances, cfg.PMGInstances); err != nil {
		return fmt.Errorf("failed to save nodes config: %w", err)
	}

	// Save system configuration
	systemSettings := SystemSettings{
		// Note: PVE polling is hardcoded to 10s
		UpdateChannel:           cfg.UpdateChannel,
		AutoUpdateEnabled:       cfg.AutoUpdateEnabled,
		AutoUpdateCheckInterval: int(cfg.AutoUpdateCheckInterval.Hours()),
		AutoUpdateTime:          cfg.AutoUpdateTime,
		AllowedOrigins:          cfg.AllowedOrigins,
		ConnectionTimeout:       int(cfg.ConnectionTimeout.Seconds()),
		LogLevel:                cfg.LogLevel,
		DiscoveryEnabled:        cfg.DiscoveryEnabled,
		DiscoverySubnet:         cfg.DiscoverySubnet,
		// APIToken removed - now handled via .env only
	}
	if err := globalPersistence.SaveSystemSettings(systemSettings); err != nil {
		return fmt.Errorf("failed to save system config: %w", err)
	}

	return nil
}

// SaveOIDCConfig persists OIDC settings using the shared config persistence layer.
func SaveOIDCConfig(settings *OIDCConfig) error {
	if globalPersistence == nil {
		return fmt.Errorf("config persistence not initialized")
	}
	if settings == nil {
		return fmt.Errorf("oidc settings cannot be nil")
	}

	clone := settings.Clone()
	if clone == nil {
		return fmt.Errorf("failed to clone oidc settings")
	}

	return globalPersistence.SaveOIDCConfig(*clone)
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate server settings
	if c.BackendPort <= 0 || c.BackendPort > 65535 {
		return fmt.Errorf("invalid backend port: %d", c.BackendPort)
	}
	if c.FrontendPort <= 0 || c.FrontendPort > 65535 {
		return fmt.Errorf("invalid frontend port: %d", c.FrontendPort)
	}

	// Validate monitoring settings
	// Note: PVE polling is hardcoded to 10s
	if c.ConnectionTimeout < time.Second {
		return fmt.Errorf("connection timeout must be at least 1 second")
	}

	// Validate PVE instances
	for i, pve := range c.PVEInstances {
		if pve.Host == "" {
			return fmt.Errorf("PVE instance %d: host is required", i+1)
		}
		if !strings.HasPrefix(pve.Host, "http://") && !strings.HasPrefix(pve.Host, "https://") {
			return fmt.Errorf("PVE instance %d: host must start with http:// or https://", i+1)
		}
		// Must have either password or token
		if pve.Password == "" && (pve.TokenName == "" || pve.TokenValue == "") {
			return fmt.Errorf("PVE instance %d: either password or token authentication is required", i+1)
		}
	}

	// Validate and auto-fix PBS instances
	validPBS := []PBSInstance{}
	for i, pbs := range c.PBSInstances {
		if pbs.Host == "" {
			log.Warn().Int("instance", i+1).Msg("PBS instance missing host, skipping")
			continue
		}
		// Auto-fix missing protocol
		if !strings.HasPrefix(pbs.Host, "http://") && !strings.HasPrefix(pbs.Host, "https://") {
			pbs.Host = "https://" + pbs.Host
			log.Info().Str("host", pbs.Host).Msg("PBS host auto-corrected to include https://")
		}
		// Check authentication
		if pbs.Password == "" && (pbs.TokenName == "" || pbs.TokenValue == "") {
			log.Warn().Int("instance", i+1).Str("host", pbs.Host).Msg("PBS instance missing authentication, skipping")
			continue
		}
		validPBS = append(validPBS, pbs)
	}
	c.PBSInstances = validPBS

	if err := c.OIDC.Validate(); err != nil {
		return err
	}

	return nil
}

// detectPublicURL attempts to automatically detect the public URL for Pulse
func detectPublicURL(port int) string {
	// When running inside Docker we can't reliably determine an externally reachable address.
	// Returning an empty string avoids surfacing container-only IPs (e.g., 172.x) in notifications.
	if _, err := os.Stat("/.dockerenv"); err == nil {
		log.Info().Msg("Docker environment detected - skipping public URL auto-detect. Set PULSE_PUBLIC_URL to expose external links.")
		return ""
	}

	// Method 1: Check if we're in a Proxmox container (most common deployment)
	if _, err := os.Stat("/etc/pve"); err == nil {
		// We're likely in a ProxmoxVE container
		// Try to get the container's IP from hostname -I
		if output, err := exec.Command("hostname", "-I").Output(); err == nil {
			ips := strings.Fields(string(output))
			for _, ip := range ips {
				// Skip localhost and IPv6
				if !strings.HasPrefix(ip, "127.") && !strings.Contains(ip, ":") {
					return fmt.Sprintf("http://%s:%d", ip, port)
				}
			}
		}
	}

	// Method 2: Try to get the primary network interface IP
	if ip := getOutboundIP(); ip != "" {
		return fmt.Sprintf("http://%s:%d", ip, port)
	}

	// Method 3: Get all non-loopback IPs and use the first private one
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ip := ipnet.IP.String()
					// Prefer private IPs (RFC1918)
					if strings.HasPrefix(ip, "192.168.") ||
						strings.HasPrefix(ip, "10.") ||
						strings.HasPrefix(ip, "172.") {
						return fmt.Sprintf("http://%s:%d", ip, port)
					}
				}
			}
		}
		// If no private IP found, use the first public one
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return fmt.Sprintf("http://%s:%d", ipnet.IP.String(), port)
				}
			}
		}
	}

	return ""
}

// getOutboundIP gets the preferred outbound IP of this machine
func getOutboundIP() string {
	// Try to connect to a public DNS server (doesn't actually connect, just resolves the route)
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// Try Cloudflare DNS as fallback
		conn, err = net.Dial("udp", "1.1.1.1:80")
		if err != nil {
			return ""
		}
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
