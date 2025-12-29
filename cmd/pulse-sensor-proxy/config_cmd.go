package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"
)

var (
	// Config command flags
	configPathFlag       string
	allowedNodesPathFlag string
	mergeNodesFlag       []string
	replaceMode          bool

	// New flags for extended commands
	controlPlaneURL     string
	controlPlaneToken   string
	controlPlaneRefresh int

	httpEnabled   bool
	httpAddr      string
	httpAuthToken string
	httpTLSCert   string
	httpTLSKey    string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage sensor proxy configuration",
	Long:  `Atomic configuration management for pulse-sensor-proxy`,
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration files",
	Long:  `Parse and validate config.yaml and allowed_nodes.yaml files`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := configPathFlag
		if cfgPath == "" {
			cfgPath = defaultConfigPath
		}

		// Use loadConfig to parse config properly and get the actual allowed_nodes_file path
		cfg, err := loadConfig(cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Config validation failed: %v\n", err)
			return err
		}

		// Determine allowed_nodes path from config (honors allowed_nodes_file setting)
		allowedNodesPath := allowedNodesPathFlag
		if allowedNodesPath == "" {
			if cfg.AllowedNodesFile != "" {
				allowedNodesPath = cfg.AllowedNodesFile
			} else {
				// Default fallback
				allowedNodesPath = filepath.Join(filepath.Dir(cfgPath), "allowed_nodes.yaml")
			}
		}

		// Check if allowed_nodes.yaml exists and validate it
		// Empty lists are allowed (admin may clear for security or cluster relies on IPC validation)
		if _, err := os.Stat(allowedNodesPath); err == nil {
			if err := validateAllowedNodesFile(allowedNodesPath); err != nil {
				fmt.Fprintf(os.Stderr, "Allowed nodes validation failed: %v\n", err)
				return err
			}
		}

		fmt.Println("Configuration valid")
		return nil
	},
}

var setAllowedNodesCmd = &cobra.Command{
	Use:   "set-allowed-nodes",
	Short: "Atomically update allowed_nodes.yaml",
	Long: `Merge or replace allowed nodes with atomic writes and file locking.

Examples:
  # Merge new nodes into existing list
  pulse-sensor-proxy config set-allowed-nodes --merge 192.168.0.1 --merge node1.local

  # Replace entire list
  pulse-sensor-proxy config set-allowed-nodes --replace --merge 192.168.0.1 --merge 192.168.0.2
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		allowedNodesPath := allowedNodesPathFlag
		if allowedNodesPath == "" {
			// Default to /etc/pulse-sensor-proxy/allowed_nodes.yaml
			allowedNodesPath = "/etc/pulse-sensor-proxy/allowed_nodes.yaml"
		}

		// Allow zero nodes when using --replace (admin may want to clear the list)
		if len(mergeNodesFlag) == 0 && !replaceMode {
			return fmt.Errorf("no nodes specified (use --merge flag, or --replace to clear)")
		}

		if err := setAllowedNodes(allowedNodesPath, mergeNodesFlag, replaceMode); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to update allowed nodes: %v\n", err)
			return err
		}

		if replaceMode {
			fmt.Printf("Replaced allowed nodes with %d entries\n", len(mergeNodesFlag))
		} else {
			fmt.Printf("Merged %d nodes into allowed nodes list\n", len(mergeNodesFlag))
		}
		return nil
	},
}

var migrateToFileCmd = &cobra.Command{
	Use:   "migrate-to-file",
	Short: "Migrate inline allowed_nodes to file-based configuration",
	Long: `Atomically migrates inline allowed_nodes block from config.yaml to allowed_nodes.yaml.

This command:
- Extracts nodes from inline allowed_nodes block in config.yaml
- Removes the inline block from config.yaml
- Ensures allowed_nodes_file is set in config.yaml
- Writes nodes to allowed_nodes.yaml
- All operations are atomic with file locking

Safe to run multiple times (idempotent).
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := configPathFlag
		if cfgPath == "" {
			cfgPath = defaultConfigPath
		}

		allowedNodesPath := allowedNodesPathFlag
		if allowedNodesPath == "" {
			allowedNodesPath = filepath.Join(filepath.Dir(cfgPath), "allowed_nodes.yaml")
		}

		migrated, err := migrateInlineToFile(cfgPath, allowedNodesPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
			return err
		}

		if migrated {
			fmt.Println("Migration complete: inline allowed_nodes moved to file")
		}
		// Silent success when nothing to migrate (idempotent)
		return nil
	},
}

var addSubnetCmd = &cobra.Command{
	Use:   "add-subnet [subnet]",
	Short: "Add an allowed source subnet",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		subnet := args[0]
		cfgPath := configPathFlag
		if cfgPath == "" {
			cfgPath = defaultConfigPath
		}

		return updateConfigMap(cfgPath, func(config map[string]interface{}) error {
			var subnets []string
			if existing, ok := config["allowed_source_subnets"]; ok {
				if list, ok := existing.([]interface{}); ok {
					for _, item := range list {
						if s, ok := item.(string); ok {
							subnets = append(subnets, s)
						}
					}
				}
			}

			// Check if already exists
			for _, s := range subnets {
				if s == subnet {
					return nil // Already exists
				}
			}

			subnets = append(subnets, subnet)
			config["allowed_source_subnets"] = subnets
			return nil
		})
	},
}

var setControlPlaneCmd = &cobra.Command{
	Use:   "set-control-plane",
	Short: "Configure Pulse control plane connection",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := configPathFlag
		if cfgPath == "" {
			cfgPath = defaultConfigPath
		}

		return updateConfigMap(cfgPath, func(config map[string]interface{}) error {
			cp := make(map[string]interface{})
			if existing, ok := config["pulse_control_plane"]; ok {
				if m, ok := existing.(map[string]interface{}); ok {
					cp = m
				}
			}

			if controlPlaneURL != "" {
				cp["url"] = controlPlaneURL
			}
			if controlPlaneToken != "" {
				cp["token_file"] = controlPlaneToken
			}
			if controlPlaneRefresh > 0 {
				cp["refresh_interval"] = controlPlaneRefresh
			}

			config["pulse_control_plane"] = cp
			return nil
		})
	},
}

var setHTTPCmd = &cobra.Command{
	Use:   "set-http",
	Short: "Configure HTTP mode settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := configPathFlag
		if cfgPath == "" {
			cfgPath = defaultConfigPath
		}

		return updateConfigMap(cfgPath, func(config map[string]interface{}) error {
			if cmd.Flags().Changed("enabled") {
				config["http_enabled"] = httpEnabled
			}
			if httpAddr != "" {
				config["http_listen_addr"] = httpAddr
			}
			if httpAuthToken != "" {
				config["http_auth_token"] = httpAuthToken
			}
			if httpTLSCert != "" {
				config["http_tls_cert"] = httpTLSCert
			}
			if httpTLSKey != "" {
				config["http_tls_key"] = httpTLSKey
			}
			return nil
		})
	},
}

func init() {
	// Add subcommands to config command
	configCmd.AddCommand(validateCmd)
	configCmd.AddCommand(setAllowedNodesCmd)
	configCmd.AddCommand(migrateToFileCmd)
	configCmd.AddCommand(addSubnetCmd)
	configCmd.AddCommand(setControlPlaneCmd)
	configCmd.AddCommand(setHTTPCmd)

	// Validate command flags
	validateCmd.Flags().StringVar(&configPathFlag, "config", "", "Path to config.yaml (default: /etc/pulse-sensor-proxy/config.yaml)")
	validateCmd.Flags().StringVar(&allowedNodesPathFlag, "allowed-nodes", "", "Path to allowed_nodes.yaml (default: same dir as config)")

	// Set-allowed-nodes command flags
	setAllowedNodesCmd.Flags().StringVar(&allowedNodesPathFlag, "allowed-nodes", "", "Path to allowed_nodes.yaml (default: /etc/pulse-sensor-proxy/allowed_nodes.yaml)")
	setAllowedNodesCmd.Flags().StringSliceVar(&mergeNodesFlag, "merge", []string{}, "Node to merge (can be specified multiple times)")
	setAllowedNodesCmd.Flags().BoolVar(&replaceMode, "replace", false, "Replace entire list instead of merging")

	// Migrate-to-file command flags
	migrateToFileCmd.Flags().StringVar(&configPathFlag, "config", "", "Path to config.yaml (default: /etc/pulse-sensor-proxy/config.yaml)")
	migrateToFileCmd.Flags().StringVar(&allowedNodesPathFlag, "allowed-nodes", "", "Path to allowed_nodes.yaml (default: same dir as config)")

	// Add-subnet command flags
	addSubnetCmd.Flags().StringVar(&configPathFlag, "config", "", "Path to config.yaml")

	// Set-control-plane command flags
	setControlPlaneCmd.Flags().StringVar(&configPathFlag, "config", "", "Path to config.yaml")
	setControlPlaneCmd.Flags().StringVar(&controlPlaneURL, "url", "", "Control plane URL")
	setControlPlaneCmd.Flags().StringVar(&controlPlaneToken, "token-file", "", "Path to token file")
	setControlPlaneCmd.Flags().IntVar(&controlPlaneRefresh, "refresh", 0, "Refresh interval in seconds")

	// Set-http command flags
	setHTTPCmd.Flags().StringVar(&configPathFlag, "config", "", "Path to config.yaml")
	setHTTPCmd.Flags().BoolVar(&httpEnabled, "enabled", false, "Enable HTTP mode")
	setHTTPCmd.Flags().StringVar(&httpAddr, "listen-addr", "", "HTTP listen address")
	setHTTPCmd.Flags().StringVar(&httpAuthToken, "auth-token", "", "HTTP auth token")
	setHTTPCmd.Flags().StringVar(&httpTLSCert, "tls-cert", "", "TLS certificate path")
	setHTTPCmd.Flags().StringVar(&httpTLSKey, "tls-key", "", "TLS key path")

	// Add config command to root
	rootCmd.AddCommand(configCmd)
}

// updateConfigMap safely updates the config file using a map representation
func updateConfigMap(path string, updateFn func(map[string]interface{}) error) error {
	lockPath := path + ".lock"
	return withLockedFile(lockPath, func(f *os.File) error {
		// Read current config
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				data = []byte("{}\n")
			} else {
				return fmt.Errorf("failed to read config: %w", err)
			}
		}

		// Sanitize duplicate blocks before parsing
		_, data = sanitizeDuplicateAllowedNodesBlocks(path, data)

		var config map[string]interface{}
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}

		if config == nil {
			config = make(map[string]interface{})
		}

		// Apply updates
		if err := updateFn(config); err != nil {
			return err
		}

		// Write back
		newData, err := yaml.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		// Preserve header comment if possible (simple heuristic)
		header := "# Managed by pulse-sensor-proxy config CLI\n"
		finalData := []byte(header + string(newData))

		return atomicWriteFile(path, finalData, 0644)
	})
}

// validateAllowedNodesFile parses and validates the allowed_nodes.yaml file
func validateAllowedNodesFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read allowed_nodes file: %w", err)
	}

	// Parse YAML - can be either a dict with allowed_nodes key or a list
	var result interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("failed to parse allowed_nodes YAML: %w", err)
	}

	// Extract nodes (empty list is valid - admin may clear for security)
	// Clusters can also rely on IPC-based validation instead of static allowlists
	var nodes []string
	switch v := result.(type) {
	case map[string]interface{}:
		if nodeList, ok := v["allowed_nodes"]; ok {
			if list, ok := nodeList.([]interface{}); ok {
				for _, item := range list {
					if s, ok := item.(string); ok && s != "" {
						nodes = append(nodes, s)
					}
				}
			}
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				nodes = append(nodes, s)
			}
		}
	case nil:
		// Empty file is valid
		return nil
	}

	// Empty list is allowed - don't enforce minimum
	return nil
}

// setAllowedNodes atomically updates the allowed_nodes.yaml file with file locking
func setAllowedNodes(path string, newNodes []string, replace bool) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Use a separate lock file that persists across renames
	lockPath := path + ".lock"
	return withLockedFile(lockPath, func(f *os.File) error {
		var existing []string

		// Read existing nodes if not in replace mode
		if !replace {
			if data, err := os.ReadFile(path); err == nil {
				existing = extractNodesFromYAML(data)
			}
		}

		// Merge and deduplicate
		merged := normalizeNodes(append(existing, newNodes...))

		// Allow empty lists (admin may want to clear the file)
		// Serialize to YAML
		output := map[string]interface{}{
			"allowed_nodes": merged,
		}
		data, err := yaml.Marshal(output)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}

		// Add header comment
		header := "# Managed by pulse-sensor-proxy config CLI\n# Do not edit manually while service is running\n"
		finalData := []byte(header + string(data))

		// Write atomically while holding lock
		return atomicWriteFile(path, finalData, 0644)
	})
}

// withLockedFile opens a lock file with exclusive locking and runs a callback
//
// IMPORTANT: If future commands need to modify multiple files, use consistent lock ordering
// to avoid deadlocks (e.g., always lock config.yaml.lock before allowed_nodes.yaml.lock)
func withLockedFile(lockPath string, fn func(f *os.File) error) error {
	// Open or create the lock file (never deleted, persists across renames)
	// Use 0600 to prevent unprivileged users from holding LOCK_EX and DoS'ing the installer
	f, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}
	defer f.Close()

	// Ensure correct permissions even if file already exists with broader perms
	if err := f.Chmod(0600); err != nil {
		return fmt.Errorf("failed to set lock file permissions: %w", err)
	}

	// Acquire exclusive lock
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("failed to acquire file lock: %w", err)
	}
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN) //nolint:errcheck

	// Run callback while holding lock
	return fn(f)
}

// atomicWriteFile writes data to a file atomically using temp file + rename
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	// Create temp file in same directory
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Clean up temp file on error
	defer func() {
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
	}()

	// Write data
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Sync to disk
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close temp file
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set permissions
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Mark temp file as successfully moved (don't delete in defer)
	tmpPath = ""

	// Sync directory to ensure rename is persisted
	dirFile, err := os.Open(dir)
	if err == nil {
		dirFile.Sync() //nolint:errcheck
		dirFile.Close()
	}

	return nil
}

// extractNodesFromYAML extracts node list from YAML data
func extractNodesFromYAML(data []byte) []string {
	var result interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil
	}

	var nodes []string
	switch v := result.(type) {
	case map[string]interface{}:
		if nodeList, ok := v["allowed_nodes"]; ok {
			if list, ok := nodeList.([]interface{}); ok {
				for _, item := range list {
					if s, ok := item.(string); ok && s != "" {
						nodes = append(nodes, s)
					}
				}
			}
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				nodes = append(nodes, s)
			}
		}
	}

	return nodes
}

// migrateInlineToFile atomically migrates inline allowed_nodes from config.yaml to allowed_nodes.yaml
// Returns (true, nil) if migration was performed, (false, nil) if nothing to migrate, or (false, err) on error.
func migrateInlineToFile(configPath, allowedNodesPath string) (bool, error) {
	configLockPath := configPath + ".lock"
	allowedNodesLockPath := allowedNodesPath + ".lock"

	var migrated bool

	// Lock both files in consistent order to prevent deadlocks
	// Always lock config.yaml before allowed_nodes.yaml
	err := withLockedFile(configLockPath, func(configLock *os.File) error {
		return withLockedFile(allowedNodesLockPath, func(allowedNodesLock *os.File) error {
			// Read current config
			configData, err := os.ReadFile(configPath)
			if err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}

			// Sanitize duplicate blocks before parsing
			_, configData = sanitizeDuplicateAllowedNodesBlocks(configPath, configData)

			// Parse config to extract inline nodes
			var config map[string]interface{}
			if err := yaml.Unmarshal(configData, &config); err != nil {
				return fmt.Errorf("failed to parse config: %w", err)
			}

			// Check if inline allowed_nodes exists - this determines if migration is needed
			_, hasInlineNodes := config["allowed_nodes"]
			_, hasFileRef := config["allowed_nodes_file"]

			// If already using file mode and no inline nodes, nothing to migrate
			if !hasInlineNodes && hasFileRef {
				return nil
			}

			// Extract inline nodes (if any)
			var inlineNodes []string
			if allowedNodes, ok := config["allowed_nodes"]; ok {
				if nodeList, ok := allowedNodes.([]interface{}); ok {
					for _, node := range nodeList {
						if s, ok := node.(string); ok && s != "" {
							inlineNodes = append(inlineNodes, s)
						}
					}
				}
			}

			// Mark that migration is being performed
			migrated = hasInlineNodes

			// Remove inline allowed_nodes block from config
			delete(config, "allowed_nodes")

			// Ensure allowed_nodes_file is set
			if _, exists := config["allowed_nodes_file"]; !exists {
				config["allowed_nodes_file"] = allowedNodesPath
			}

			// Write updated config atomically
			newConfigData, err := yaml.Marshal(config)
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}

			if err := atomicWriteFile(configPath, newConfigData, 0644); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			// Merge inline nodes with existing file nodes (if any)
			var existingNodes []string
			if data, err := os.ReadFile(allowedNodesPath); err == nil {
				existingNodes = extractNodesFromYAML(data)
			}

			// Combine and deduplicate
			allNodes := normalizeNodes(append(existingNodes, inlineNodes...))

			// Write allowed_nodes.yaml atomically
			output := map[string]interface{}{
				"allowed_nodes": allNodes,
			}
			allowedNodesData, err := yaml.Marshal(output)
			if err != nil {
				return fmt.Errorf("failed to marshal allowed_nodes: %w", err)
			}

			header := "# Managed by pulse-sensor-proxy config CLI\n# Do not edit manually while service is running\n"
			finalData := []byte(header + string(allowedNodesData))

			return atomicWriteFile(allowedNodesPath, finalData, 0644)
		})
	})
	return migrated, err
}
