package api

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func resolveAuthEnvPath(configPath string) string {
	return filepath.Join(config.ResolveRuntimeDataDir(configPath), ".env")
}

func resolveAuthEnvWritePaths(configPath string, dataPath string) []string {
	paths := []string{resolveAuthEnvPath(configPath)}
	dataEnvPath := resolveAuthEnvPath(dataPath)
	if dataEnvPath != paths[0] {
		paths = append(paths, dataEnvPath)
	}
	return paths
}

func writeAuthEnvFile(configPath string, dataPath string, content []byte) (string, error) {
	paths := resolveAuthEnvWritePaths(configPath, dataPath)
	var lastErr error
	for _, envPath := range paths {
		if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
			lastErr = err
			continue
		}
		if err := os.WriteFile(envPath, content, 0600); err != nil {
			lastErr = err
			continue
		}
		return envPath, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no auth env write path available")
	}
	return "", lastErr
}
