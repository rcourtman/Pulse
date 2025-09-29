package config

import (
	"fmt"
	"net"
	"strconv"
)

// Settings represents the complete application configuration
type Settings struct {
	Server     ServerSettings     `json:"server" yaml:"server" mapstructure:"server"`
	Monitoring MonitoringSettings `json:"monitoring" yaml:"monitoring" mapstructure:"monitoring"`
	Logging    LoggingSettings    `json:"logging" yaml:"logging" mapstructure:"logging"`
	Security   SecuritySettings   `json:"security" yaml:"security" mapstructure:"security"`
}

// ServerSettings contains all server-related configuration
type ServerSettings struct {
	Backend  PortSettings `json:"backend" yaml:"backend" mapstructure:"backend"`
	Frontend PortSettings `json:"frontend" yaml:"frontend" mapstructure:"frontend"`
}

// PortSettings defines configuration for a network service
type PortSettings struct {
	Port int    `json:"port" yaml:"port" mapstructure:"port"`
	Host string `json:"host" yaml:"host" mapstructure:"host"`
}

// MonitoringSettings contains monitoring-related configuration
type MonitoringSettings struct {
	PollingInterval      int  `json:"pollingInterval" yaml:"pollingInterval" mapstructure:"pollingInterval"` // milliseconds
	ConcurrentPolling    bool `json:"concurrentPolling" yaml:"concurrentPolling" mapstructure:"concurrentPolling"`
	BackupPollingCycles  int  `json:"backupPollingCycles" yaml:"backupPollingCycles" mapstructure:"backupPollingCycles"` // How often to poll backups
	MetricsRetentionDays int  `json:"metricsRetentionDays" yaml:"metricsRetentionDays" mapstructure:"metricsRetentionDays"`
}

// LoggingSettings contains logging configuration
type LoggingSettings struct {
	Level      string `json:"level" yaml:"level" mapstructure:"level"`       // debug, info, warn, error
	File       string `json:"file" yaml:"file" mapstructure:"file"`          // log file path
	MaxSize    int    `json:"maxSize" yaml:"maxSize" mapstructure:"maxSize"` // MB
	MaxBackups int    `json:"maxBackups" yaml:"maxBackups" mapstructure:"maxBackups"`
	MaxAge     int    `json:"maxAge" yaml:"maxAge" mapstructure:"maxAge"` // days
	Compress   bool   `json:"compress" yaml:"compress" mapstructure:"compress"`
}

// SecuritySettings contains security-related configuration
type SecuritySettings struct {
	APIToken             string   `json:"apiToken" yaml:"apiToken" mapstructure:"apiToken"`
	AllowedOrigins       []string `json:"allowedOrigins" yaml:"allowedOrigins" mapstructure:"allowedOrigins"`
	IframeEmbedding      string   `json:"iframeEmbedding" yaml:"iframeEmbedding" mapstructure:"iframeEmbedding"` // DENY, SAMEORIGIN, or URL
	EnableAuthentication bool     `json:"enableAuthentication" yaml:"enableAuthentication" mapstructure:"enableAuthentication"`
}

// DefaultSettings returns the default configuration
func DefaultSettings() *Settings {
	return &Settings{
		Server: ServerSettings{
			Backend: PortSettings{
				Port: 3000,
				Host: "0.0.0.0",
			},
			Frontend: PortSettings{
				Port: 7655,
				Host: "0.0.0.0",
			},
		},
		Monitoring: MonitoringSettings{
			PollingInterval:      5000, // 5 seconds
			ConcurrentPolling:    true,
			BackupPollingCycles:  10, // Poll backups every 10 cycles
			MetricsRetentionDays: 7,
		},
		Logging: LoggingSettings{
			Level:      "info",
			File:       "/opt/pulse/pulse.log",
			MaxSize:    100, // 100MB
			MaxBackups: 5,
			MaxAge:     30, // 30 days
			Compress:   true,
		},
		Security: SecuritySettings{
			AllowedOrigins:       []string{"*"},
			IframeEmbedding:      "SAMEORIGIN",
			EnableAuthentication: false,
		},
	}
}

// Validate checks if the settings are valid
func (s *Settings) Validate() error {
	// Validate backend port
	if err := validatePort(s.Server.Backend.Port); err != nil {
		return fmt.Errorf("invalid backend port: %w", err)
	}

	// Validate frontend port
	if err := validatePort(s.Server.Frontend.Port); err != nil {
		return fmt.Errorf("invalid frontend port: %w", err)
	}

	// Ensure ports are different
	if s.Server.Backend.Port == s.Server.Frontend.Port {
		return fmt.Errorf("backend and frontend ports must be different")
	}

	// Validate hosts
	if err := validateHost(s.Server.Backend.Host); err != nil {
		return fmt.Errorf("invalid backend host: %w", err)
	}

	if err := validateHost(s.Server.Frontend.Host); err != nil {
		return fmt.Errorf("invalid frontend host: %w", err)
	}

	// Validate monitoring settings
	if s.Monitoring.PollingInterval < 1000 {
		return fmt.Errorf("polling interval must be at least 1000ms (1 second)")
	}

	if s.Monitoring.BackupPollingCycles < 1 {
		return fmt.Errorf("backup polling cycles must be at least 1")
	}

	// Validate logging level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[s.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", s.Logging.Level)
	}

	return nil
}

// validatePort checks if a port number is valid
func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	// Warn about privileged ports
	if port < 1024 {
		// This is just a warning in the validation
		// The actual binding will fail if not root
		return nil
	}

	return nil
}

// validateHost checks if a host string is valid
func validateHost(host string) error {
	if host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	// Special cases
	if host == "0.0.0.0" || host == "::" || host == "localhost" {
		return nil
	}

	// Try to parse as IP
	if net.ParseIP(host) != nil {
		return nil
	}

	// Try to parse as hostname (basic check)
	if len(host) > 0 && len(host) <= 253 {
		return nil
	}

	return fmt.Errorf("invalid host: %s", host)
}

// IsPortAvailable checks if a port is available for binding
func IsPortAvailable(host string, port int) bool {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}
