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
	configPathFlag         string
	allowedNodesPathFlag   string
	mergeNodesFlag         []string
	replaceMode            bool
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

func init() {
	// Add subcommands to config command
	configCmd.AddCommand(validateCmd)
	configCmd.AddCommand(setAllowedNodesCmd)

	// Validate command flags
	validateCmd.Flags().StringVar(&configPathFlag, "config", "", "Path to config.yaml (default: /etc/pulse-sensor-proxy/config.yaml)")
	validateCmd.Flags().StringVar(&allowedNodesPathFlag, "allowed-nodes", "", "Path to allowed_nodes.yaml (default: same dir as config)")

	// Set-allowed-nodes command flags
	setAllowedNodesCmd.Flags().StringVar(&allowedNodesPathFlag, "allowed-nodes", "", "Path to allowed_nodes.yaml (default: /etc/pulse-sensor-proxy/allowed_nodes.yaml)")
	setAllowedNodesCmd.Flags().StringSliceVar(&mergeNodesFlag, "merge", []string{}, "Node to merge (can be specified multiple times)")
	setAllowedNodesCmd.Flags().BoolVar(&replaceMode, "replace", false, "Replace entire list instead of merging")

	// Add config command to root
	rootCmd.AddCommand(configCmd)
}

// validateConfigFile parses and validates the main config file
func validateConfigFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Check for duplicate allowed_nodes blocks (the issue we're fixing)
	sanitized, cleanData := sanitizeDuplicateAllowedNodesBlocks("", data)
	if sanitized {
		return fmt.Errorf("config contains duplicate allowed_nodes blocks (would auto-fix on service start)")
	}

	// Parse YAML
	cfg := &Config{}
	if err := yaml.Unmarshal(cleanData, cfg); err != nil {
		return fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Validate required fields
	if cfg.ReadTimeout <= 0 {
		return fmt.Errorf("read_timeout must be positive")
	}
	if cfg.WriteTimeout <= 0 {
		return fmt.Errorf("write_timeout must be positive")
	}

	return nil
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
