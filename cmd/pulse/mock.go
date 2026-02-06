package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var mockCmd = &cobra.Command{
	Use:   "mock",
	Short: "Manage mock/demo mode for development and demos",
	Long:  `Enable or disable mock mode to run Pulse with simulated data instead of real infrastructure.`,
}

var mockEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable mock mode with simulated infrastructure data",
	Long: `Enable mock mode to run Pulse with simulated data.

This creates/updates the mock.env file and requires a service restart.
Mock mode is useful for:
  - Demos without real infrastructure
  - Development and testing
  - Showcasing AI patrol features

Example:
  pulse mock enable
  sudo systemctl restart pulse`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := setMockMode(true); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			osExit(1)
			return
		}
		fmt.Println("✓ Mock mode enabled")
		fmt.Println("")
		fmt.Println("Restart Pulse to apply changes:")
		fmt.Println("  sudo systemctl restart pulse")
	},
}

var mockDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable mock mode and use real infrastructure",
	Long: `Disable mock mode to reconnect to real infrastructure.

This updates the mock.env file and requires a service restart.

Example:
  pulse mock disable
  sudo systemctl restart pulse`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := setMockMode(false); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			osExit(1)
			return
		}
		fmt.Println("✓ Mock mode disabled")
		fmt.Println("")
		fmt.Println("Restart Pulse to apply changes:")
		fmt.Println("  sudo systemctl restart pulse")
	},
}

var mockStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current mock mode status",
	Run: func(cmd *cobra.Command, args []string) {
		enabled, config := getMockStatus()
		if enabled {
			fmt.Println("Mock mode: ENABLED")
			fmt.Println("")
			fmt.Println("Configuration:")
			for _, line := range config {
				fmt.Printf("  %s\n", line)
			}
		} else {
			fmt.Println("Mock mode: DISABLED")
			fmt.Println("")
			fmt.Println("Run 'pulse mock enable' to enable mock mode")
		}
	},
}

func init() {
	mockCmd.AddCommand(mockEnableCmd)
	mockCmd.AddCommand(mockDisableCmd)
	mockCmd.AddCommand(mockStatusCmd)
	rootCmd.AddCommand(mockCmd)
}

var mockEnvDefaultDir = "/opt/pulse"
var mockEnvStat = os.Stat

// getMockEnvPath returns the path to mock.env
func getMockEnvPath() string {
	// Check PULSE_DATA_DIR first, then fall back to /opt/pulse
	dataDir := os.Getenv("PULSE_DATA_DIR")
	if dataDir == "" {
		// Check if we're in a packaged install (running from mockEnvDefaultDir)
		probe := filepath.Join(mockEnvDefaultDir, "mock.env")
		if _, err := mockEnvStat(probe); err == nil {
			return probe
		}
		dataDir = mockEnvDefaultDir
	}
	return filepath.Join(dataDir, "mock.env")
}

// setMockMode enables or disables mock mode by updating mock.env
func setMockMode(enable bool) error {
	envPath := getMockEnvPath()

	// Read existing config or create default
	config := getDefaultMockConfig()

	// Try to preserve existing config
	if data, err := os.ReadFile(envPath); err == nil {
		existing := parseMockEnv(string(data))
		for k, v := range existing {
			if k != "PULSE_MOCK_MODE" {
				config[k] = v
			}
		}
	}

	// Set the mode
	if enable {
		config["PULSE_MOCK_MODE"] = "true"
	} else {
		config["PULSE_MOCK_MODE"] = "false"
	}

	// Write the file
	return writeMockEnv(envPath, config)
}

// getDefaultMockConfig returns the default mock configuration
func getDefaultMockConfig() map[string]string {
	return map[string]string{
		"PULSE_MOCK_MODE":              "false",
		"PULSE_MOCK_NODES":             "7",
		"PULSE_MOCK_VMS_PER_NODE":      "5",
		"PULSE_MOCK_LXCS_PER_NODE":     "8",
		"PULSE_MOCK_DOCKER_HOSTS":      "3",
		"PULSE_MOCK_DOCKER_CONTAINERS": "12",
		"PULSE_MOCK_GENERIC_HOSTS":     "4",
		"PULSE_MOCK_K8S_CLUSTERS":      "2",
		"PULSE_MOCK_K8S_NODES":         "4",
		"PULSE_MOCK_K8S_PODS":          "30",
		"PULSE_MOCK_K8S_DEPLOYMENTS":   "12",
		"PULSE_MOCK_RANDOM_METRICS":    "true",
		"PULSE_MOCK_STOPPED_PERCENT":   "20",
	}
}

// parseMockEnv parses a mock.env file into a map
func parseMockEnv(content string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// writeMockEnv writes the mock.env file
func writeMockEnv(path string, config map[string]string) error {
	// Order keys for consistent output
	keys := []string{
		"PULSE_MOCK_MODE",
		"PULSE_MOCK_NODES",
		"PULSE_MOCK_VMS_PER_NODE",
		"PULSE_MOCK_LXCS_PER_NODE",
		"PULSE_MOCK_DOCKER_HOSTS",
		"PULSE_MOCK_DOCKER_CONTAINERS",
		"PULSE_MOCK_GENERIC_HOSTS",
		"PULSE_MOCK_K8S_CLUSTERS",
		"PULSE_MOCK_K8S_NODES",
		"PULSE_MOCK_K8S_PODS",
		"PULSE_MOCK_K8S_DEPLOYMENTS",
		"PULSE_MOCK_RANDOM_METRICS",
		"PULSE_MOCK_STOPPED_PERCENT",
	}

	var lines []string
	lines = append(lines, "# Pulse Mock Mode Configuration")
	lines = append(lines, "# Enable with: pulse mock enable")
	lines = append(lines, "# Disable with: pulse mock disable")
	lines = append(lines, "")

	for _, key := range keys {
		if val, ok := config[key]; ok {
			lines = append(lines, fmt.Sprintf("%s=%s", key, val))
		}
	}

	// Add any extra keys not in our ordered list
	for k, v := range config {
		found := false
		for _, key := range keys {
			if k == key {
				found = true
				break
			}
		}
		if !found {
			lines = append(lines, fmt.Sprintf("%s=%s", k, v))
		}
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0644)
}

// getMockStatus returns the current mock mode status and config
func getMockStatus() (enabled bool, config []string) {
	envPath := getMockEnvPath()

	data, err := os.ReadFile(envPath)
	if err != nil {
		return false, nil
	}

	parsed := parseMockEnv(string(data))
	enabled = parsed["PULSE_MOCK_MODE"] == "true"

	if enabled {
		config = []string{
			fmt.Sprintf("Nodes: %s", parsed["PULSE_MOCK_NODES"]),
			fmt.Sprintf("VMs per node: %s", parsed["PULSE_MOCK_VMS_PER_NODE"]),
			fmt.Sprintf("Containers per node: %s", parsed["PULSE_MOCK_LXCS_PER_NODE"]),
			fmt.Sprintf("Docker hosts: %s", parsed["PULSE_MOCK_DOCKER_HOSTS"]),
			fmt.Sprintf("K8s clusters: %s", parsed["PULSE_MOCK_K8S_CLUSTERS"]),
		}
	}

	return enabled, config
}
