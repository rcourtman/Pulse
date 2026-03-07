package pulsecli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/server"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const maxConfigImportBytes int64 = 16 << 20 // 16 MiB

type State struct {
	ExportFile        *string
	ImportFile        *string
	Passphrase        *string
	ForceImport       *bool
	ExitFunc          *func(int)
	ReadPassword      *func(int) ([]byte, error)
	MockEnvDefaultDir *string
	MockEnvStat       *func(string) (os.FileInfo, error)
}

type Options struct {
	Use             string
	Short           string
	Long            string
	Version         string
	VersionTemplate string
	RunE            func(context.Context) error
	VersionPrinter  func(io.Writer)
	State           *State
}

func NewRootCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:          opts.Use,
		Short:        opts.Short,
		Long:         opts.Long,
		SilenceUsage: true,
		Version:      opts.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.RunE == nil {
				return nil
			}
			return opts.RunE(cmd.Context())
		},
	}

	if opts.VersionTemplate != "" {
		cmd.SetVersionTemplate(opts.VersionTemplate)
	}

	cmd.AddCommand(newVersionCmd(opts))
	cmd.AddCommand(newConfigCmd(opts.State))
	cmd.AddCommand(newBootstrapTokenCmd(opts.State))
	cmd.AddCommand(newMockCmd(opts.State))

	return cmd
}

func ResetFlags(state *State) {
	if state == nil {
		return
	}
	if state.ExportFile != nil {
		*state.ExportFile = ""
	}
	if state.ImportFile != nil {
		*state.ImportFile = ""
	}
	if state.Passphrase != nil {
		*state.Passphrase = ""
	}
	if state.ForceImport != nil {
		*state.ForceImport = false
	}
}

func ReadBoundedRegularFile(path string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("invalid max bytes %d", maxBytes)
	}

	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("path is not a regular file")
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", maxBytes)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", maxBytes)
	}

	return data, nil
}

func ReadBoundedHTTPBody(reader io.Reader, declaredLength, maxBytes int64, source string) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("invalid max bytes %d", maxBytes)
	}
	if source == "" {
		source = "response body"
	}
	if declaredLength > maxBytes {
		return nil, fmt.Errorf("%s exceeds %d bytes", source, maxBytes)
	}

	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%s exceeds %d bytes", source, maxBytes)
	}

	return data, nil
}

func GetPassphrase(state *State, prompt string, confirm bool) string {
	if pass := os.Getenv("PULSE_PASSPHRASE"); pass != "" {
		return pass
	}

	if state != nil && state.Passphrase != nil && *state.Passphrase != "" {
		return *state.Passphrase
	}

	fmt.Print(prompt)
	bytePassword, err := stateReadPassword(state, int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return ""
	}

	pass := string(bytePassword)
	if !confirm {
		return pass
	}

	fmt.Print("Confirm passphrase: ")
	bytePassword2, err := stateReadPassword(state, int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return ""
	}
	if string(bytePassword2) != pass {
		fmt.Println("Passphrases do not match")
		return ""
	}

	return pass
}

func ShowBootstrapToken(state *State) {
	dataPath := os.Getenv("PULSE_DATA_DIR")
	if dataPath == "" {
		if os.Getenv("PULSE_DOCKER") == "true" {
			dataPath = "/data"
		} else {
			dataPath = "/etc/pulse"
		}
	}

	tokenPath := filepath.Join(dataPath, ".bootstrap_token")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("╔═══════════════════════════════════════════════════════════════════════╗")
			fmt.Println("║                    NO BOOTSTRAP TOKEN FOUND                           ║")
			fmt.Println("╠═══════════════════════════════════════════════════════════════════════╣")
			fmt.Println("║  Possible reasons:                                                    ║")
			fmt.Println("║  • Initial setup has already been completed                           ║")
			fmt.Println("║  • Authentication is configured (token auto-deleted)                  ║")
			fmt.Println("║  • Server hasn't started yet (token not generated)                    ║")
			fmt.Printf("║  • Token file not found: %-44s║\n", tokenPath)
			fmt.Println("╚═══════════════════════════════════════════════════════════════════════╝")
			stateExit(state, 1)
			return
		}
		fmt.Printf("Error reading bootstrap token: %v\n", err)
		stateExit(state, 1)
		return
	}

	token := strings.TrimSpace(string(data))
	if token == "" {
		fmt.Println("Error: Bootstrap token file is empty")
		stateExit(state, 1)
		return
	}

	fmt.Println("╔═══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║          BOOTSTRAP TOKEN FOR FIRST-TIME SETUP                         ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Token: %-61s ║\n", token)
	fmt.Printf("║  File:  %-61s ║\n", tokenPath)
	fmt.Println("╠═══════════════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Instructions:                                                        ║")
	fmt.Println("║  1. Copy the token above                                              ║")
	fmt.Println("║  2. Open Pulse in your web browser                                    ║")
	fmt.Println("║  3. Paste the token into the unlock screen                            ║")
	fmt.Println("║  4. Complete the admin account setup                                  ║")
	fmt.Println("║                                                                       ║")
	fmt.Println("║  This token will be automatically deleted after successful setup.     ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════════╝")
}

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

func newVersionCmd(opts Options) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			if opts.VersionPrinter != nil {
				opts.VersionPrinter(cmd.OutOrStdout())
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", opts.Version)
		},
	}
}

func newBootstrapTokenCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "bootstrap-token",
		Short: "Display the bootstrap setup token",
		Long: `Display the bootstrap setup token required for first-time setup.

This token is generated on first boot and must be entered in the web UI
to unlock the initial setup wizard. The token is automatically deleted
after successful setup completion.`,
		Run: func(cmd *cobra.Command, args []string) {
			ShowBootstrapToken(state)
		},
	}
}

func newConfigCmd(state *State) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management commands",
		Long:  `Manage Pulse configuration settings`,
	}

	configInfoCmd := &cobra.Command{
		Use:   "info",
		Short: "Show configuration information",
		Long:  `Display information about Pulse configuration`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "Pulse Configuration Information")
			fmt.Fprintln(cmd.OutOrStdout(), "==============================")
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Configuration is managed through the web UI.")
			fmt.Fprintln(cmd.OutOrStdout(), "Settings are stored in encrypted files at /etc/pulse/")
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Configuration files:")
			fmt.Fprintln(cmd.OutOrStdout(), "  - nodes.enc      : Encrypted Proxmox node configurations")
			fmt.Fprintln(cmd.OutOrStdout(), "  - email.enc      : Encrypted email settings")
			fmt.Fprintln(cmd.OutOrStdout(), "  - system.json    : System settings (polling interval, etc)")
			fmt.Fprintln(cmd.OutOrStdout(), "  - alerts.json    : Alert rules and thresholds")
			fmt.Fprintln(cmd.OutOrStdout(), "  - webhooks.enc   : Webhook configurations")
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "To configure Pulse, use the Settings tab in the web UI.")
			return nil
		},
	}

	configExportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export configuration with encryption",
		Long:  `Export all Pulse configuration to an encrypted file`,
		Example: `  # Export with interactive passphrase prompt
  pulse config export -o pulse-config.enc

  # Export with passphrase from environment variable
  PULSE_PASSPHRASE=mysecret pulse config export -o pulse-config.enc`,
		RunE: func(cmd *cobra.Command, args []string) error {
			pass := GetPassphrase(state, "Enter passphrase for encryption: ", false)
			if pass == "" {
				return fmt.Errorf("passphrase is required")
			}

			configPath := os.Getenv("PULSE_DATA_DIR")
			if configPath == "" {
				configPath = "/etc/pulse"
			}

			persistence := config.NewConfigPersistence(configPath)
			exportedData, err := persistence.ExportConfig(pass)
			if err != nil {
				return fmt.Errorf("failed to export configuration: %w", err)
			}

			if state != nil && state.ExportFile != nil && *state.ExportFile != "" {
				if err := os.WriteFile(*state.ExportFile, []byte(exportedData), 0600); err != nil {
					return fmt.Errorf("failed to write export file: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Configuration exported to %s\n", *state.ExportFile)
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), exportedData)
			return nil
		},
	}

	configImportCmd := &cobra.Command{
		Use:   "import",
		Short: "Import configuration from encrypted export",
		Long:  `Import Pulse configuration from an encrypted export file`,
		Example: `  # Import with interactive passphrase prompt
  pulse config import -i pulse-config.enc

  # Import with passphrase from environment variable
  PULSE_PASSPHRASE=mysecret pulse config import -i pulse-config.enc

  # Force import without confirmation
  pulse config import -i pulse-config.enc --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if state == nil || state.ImportFile == nil || *state.ImportFile == "" {
				return fmt.Errorf("import file is required (use -i flag)")
			}

			data, err := ReadBoundedRegularFile(*state.ImportFile, maxConfigImportBytes)
			if err != nil {
				return fmt.Errorf("failed to read import file: %w", err)
			}

			pass := GetPassphrase(state, "Enter passphrase for decryption: ", false)
			if pass == "" {
				return fmt.Errorf("passphrase is required")
			}

			forceImport := state != nil && state.ForceImport != nil && *state.ForceImport
			if !forceImport {
				fmt.Fprintln(cmd.OutOrStdout(), "WARNING: This will overwrite all existing configuration!")
				fmt.Fprint(cmd.OutOrStdout(), "Continue? (yes/no): ")
				reader := bufio.NewReader(os.Stdin)
				response, _ := reader.ReadString('\n')
				response = strings.TrimSpace(strings.ToLower(response))
				if response != "yes" && response != "y" {
					fmt.Fprintln(cmd.OutOrStdout(), "Import cancelled")
					return nil
				}
			}

			configPath := os.Getenv("PULSE_DATA_DIR")
			if configPath == "" {
				configPath = "/etc/pulse"
			}

			persistence := config.NewConfigPersistence(configPath)
			if err := persistence.ImportConfig(string(data), pass); err != nil {
				return fmt.Errorf("failed to import configuration: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Configuration imported successfully")
			fmt.Fprintln(cmd.OutOrStdout(), "Please restart Pulse for changes to take effect:")
			fmt.Fprintln(cmd.OutOrStdout(), "  sudo systemctl restart pulse")
			return nil
		},
	}

	configAutoImportCmd := &cobra.Command{
		Use:    "auto-import",
		Hidden: true,
		Short:  "Auto-import configuration on startup",
		Long:   `Automatically import configuration from URL or file on first startup`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configURL := os.Getenv("PULSE_INIT_CONFIG_URL")
			configData := os.Getenv("PULSE_INIT_CONFIG_DATA")
			configPass := os.Getenv("PULSE_INIT_CONFIG_PASSPHRASE")

			if configURL == "" && configData == "" {
				return nil
			}

			if configPass == "" {
				return fmt.Errorf("PULSE_INIT_CONFIG_PASSPHRASE is required for auto-import")
			}

			var encryptedData string
			if configURL != "" {
				parsedURL, err := url.Parse(configURL)
				if err != nil {
					return fmt.Errorf("invalid PULSE_INIT_CONFIG_URL: %w", err)
				}
				if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
					return fmt.Errorf("unsupported URL scheme %q for PULSE_INIT_CONFIG_URL", parsedURL.Scheme)
				}

				client := &http.Client{Timeout: 15 * time.Second}
				resp, err := client.Get(configURL)
				if err != nil {
					return fmt.Errorf("failed to fetch configuration from URL: %w", err)
				}

				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					if closeErr := resp.Body.Close(); closeErr != nil {
						return errors.Join(
							fmt.Errorf("failed to fetch configuration from URL: %s", resp.Status),
							fmt.Errorf("failed to close configuration response body: %w", closeErr),
						)
					}
					return fmt.Errorf("failed to fetch configuration from URL: %s", resp.Status)
				}

				body, err := ReadBoundedHTTPBody(resp.Body, resp.ContentLength, maxConfigImportBytes, "configuration response")
				if err != nil {
					if closeErr := resp.Body.Close(); closeErr != nil {
						return errors.Join(
							fmt.Errorf("failed to read configuration response: %w", err),
							fmt.Errorf("failed to close configuration response body: %w", closeErr),
						)
					}
					return fmt.Errorf("failed to read configuration response: %w", err)
				}
				if closeErr := resp.Body.Close(); closeErr != nil {
					return fmt.Errorf("failed to close configuration response body: %w", closeErr)
				}
				if len(body) == 0 {
					return fmt.Errorf("configuration response from URL was empty")
				}

				payload, err := server.NormalizeImportPayload(body)
				if err != nil {
					return fmt.Errorf("failed to normalize imported configuration payload from URL: %w", err)
				}
				encryptedData = payload
			} else {
				payload, err := server.NormalizeImportPayload([]byte(configData))
				if err != nil {
					return fmt.Errorf("failed to normalize imported configuration payload from PULSE_INIT_CONFIG_DATA: %w", err)
				}
				encryptedData = payload
			}

			configPath := os.Getenv("PULSE_DATA_DIR")
			if configPath == "" {
				configPath = "/etc/pulse"
			}

			persistence := config.NewConfigPersistence(configPath)
			if err := persistence.ImportConfig(encryptedData, configPass); err != nil {
				return fmt.Errorf("failed to auto-import configuration: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Configuration auto-imported successfully")
			return nil
		},
	}

	configCmd.AddCommand(configInfoCmd)
	configCmd.AddCommand(configExportCmd)
	configCmd.AddCommand(configImportCmd)
	configCmd.AddCommand(configAutoImportCmd)

	if state != nil && state.ExportFile != nil && state.Passphrase != nil {
		configExportCmd.Flags().StringVarP(state.ExportFile, "output", "o", "", "Output file for encrypted configuration")
		configExportCmd.Flags().StringVarP(state.Passphrase, "passphrase", "p", "", "Passphrase for encryption (or use PULSE_PASSPHRASE env var)")
	}
	if state != nil && state.ImportFile != nil && state.Passphrase != nil && state.ForceImport != nil {
		configImportCmd.Flags().StringVarP(state.ImportFile, "input", "i", "", "Input file with encrypted configuration")
		configImportCmd.Flags().StringVarP(state.Passphrase, "passphrase", "p", "", "Passphrase for decryption (or use PULSE_PASSPHRASE env var)")
		configImportCmd.Flags().BoolVarP(state.ForceImport, "force", "f", false, "Force import without confirmation")
	}

	return configCmd
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

func stateExit(state *State, code int) {
	if state != nil && state.ExitFunc != nil && *state.ExitFunc != nil {
		(*state.ExitFunc)(code)
		return
	}
	os.Exit(code)
}

func stateReadPassword(state *State, fd int) ([]byte, error) {
	if state != nil && state.ReadPassword != nil && *state.ReadPassword != nil {
		return (*state.ReadPassword)(fd)
	}
	return term.ReadPassword(fd)
}

func stateMockEnvDefaultDir(state *State) string {
	if state != nil && state.MockEnvDefaultDir != nil && *state.MockEnvDefaultDir != "" {
		return *state.MockEnvDefaultDir
	}
	return "/opt/pulse"
}

func stateMockStat(state *State, path string) (os.FileInfo, error) {
	if state != nil && state.MockEnvStat != nil && *state.MockEnvStat != nil {
		return (*state.MockEnvStat)(path)
	}
	return os.Stat(path)
}
