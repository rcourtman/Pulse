package pulsecli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func GetMockEnvPath(state *State) string {
	dataDir := os.Getenv("PULSE_DATA_DIR")
	if dataDir == "" {
		if info, err := stateMockStat(state, "tmp/dev-config"); err == nil && info.IsDir() {
			dataDir = "tmp/dev-config"
		} else {
			probe := filepath.Join(stateMockEnvDefaultDir(state), "mock.env")
			if _, err := stateMockStat(state, probe); err == nil {
				return probe
			}
			dataDir = stateMockEnvDefaultDir(state)
		}
	}
	return filepath.Join(dataDir, "mock.env")
}

func newMockCmd(state *State) *cobra.Command {
	mockCmd := &cobra.Command{
		Use:   "mock",
		Short: "Manage mock/demo mode for development and demos",
		Long:  `Enable or disable mock mode to run Pulse with simulated data instead of real infrastructure.`,
	}

	mockEnableCmd := &cobra.Command{
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
			if err := setMockMode(state, true); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
				stateExit(state, 1)
				return
			}
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Mock mode enabled")
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Restart Pulse to apply changes:")
			fmt.Fprintln(cmd.OutOrStdout(), "  sudo systemctl restart pulse")
		},
	}

	mockDisableCmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable mock mode and use real infrastructure",
		Long: `Disable mock mode to reconnect to real infrastructure.

This updates the mock.env file and requires a service restart.

Example:
  pulse mock disable
  sudo systemctl restart pulse`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := setMockMode(state, false); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
				stateExit(state, 1)
				return
			}
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Mock mode disabled")
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Restart Pulse to apply changes:")
			fmt.Fprintln(cmd.OutOrStdout(), "  sudo systemctl restart pulse")
		},
	}

	mockStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show current mock mode status",
		Run: func(cmd *cobra.Command, args []string) {
			enabled, config := getMockStatus(state)
			if enabled {
				fmt.Fprintln(cmd.OutOrStdout(), "Mock mode: ENABLED")
				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprintln(cmd.OutOrStdout(), "Configuration:")
				for _, line := range config {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", line)
				}
				return
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Mock mode: DISABLED")
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Run 'pulse mock enable' to enable mock mode")
		},
	}

	mockCmd.AddCommand(mockEnableCmd)
	mockCmd.AddCommand(mockDisableCmd)
	mockCmd.AddCommand(mockStatusCmd)

	return mockCmd
}

func setMockMode(state *State, enable bool) error {
	envPath := GetMockEnvPath(state)
	config := getDefaultMockConfig()

	if data, err := os.ReadFile(envPath); err == nil {
		existing := parseMockEnv(string(data))
		for k, v := range existing {
			if k != "PULSE_MOCK_MODE" {
				config[k] = v
			}
		}
	}

	if enable {
		config["PULSE_MOCK_MODE"] = "true"
	} else {
		config["PULSE_MOCK_MODE"] = "false"
	}

	return writeMockEnv(envPath, config)
}

func getMockStatus(state *State) (bool, []string) {
	envPath := GetMockEnvPath(state)
	data, err := os.ReadFile(envPath)
	if err != nil {
		return false, nil
	}

	config := parseMockEnv(string(data))
	enabled := strings.EqualFold(strings.TrimSpace(config["PULSE_MOCK_MODE"]), "true")
	lines := make([]string, 0, len(config))
	for key, value := range config {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	return enabled, lines
}

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

func writeMockEnv(envPath string, config map[string]string) error {
	var b strings.Builder
	b.WriteString("# Pulse Mock Mode Configuration\n")
	b.WriteString("# Enable with: pulse mock enable\n")
	b.WriteString("# Disable with: pulse mock disable\n\n")

	for _, key := range []string{
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
	} {
		if value, ok := config[key]; ok {
			fmt.Fprintf(&b, "%s=%s\n", key, value)
		}
	}

	for key, value := range config {
		if strings.HasPrefix(key, "PULSE_MOCK_") {
			continue
		}
		fmt.Fprintf(&b, "%s=%s\n", key, value)
	}

	return os.WriteFile(envPath, []byte(b.String()), 0644)
}
