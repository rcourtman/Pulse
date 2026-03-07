package pulsecli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/server"
	"github.com/spf13/cobra"
)

const maxConfigImportBytes int64 = 16 << 20 // 16 MiB

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
