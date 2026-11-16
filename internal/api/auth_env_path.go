package api

import (
	"errors"
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

// writeAuthEnvFile publishes complete owner-private credentials, never a partial
// in-place rewrite or a write through a canonical symlink/hard link. The configured
// directory may still be a symlink. Keep preferred/fallback path selection intact.
func writeAuthEnvFile(configPath string, dataPath string, content []byte) (string, error) {
	paths := resolveAuthEnvWritePaths(configPath, dataPath)
	var lastErr error
	for _, envPath := range paths {
		if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
			lastErr = err
			continue
		}
		if err := writePrivateAuthSnapshot(envPath, content, nil); err != nil {
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

func removeAuthEnvFiles(configPath string, dataPath string) error {
	paths := resolveAuthEnvWritePaths(configPath, dataPath)
	var lastErr error
	for _, envPath := range paths {
		if err := os.Remove(envPath); err != nil && !os.IsNotExist(err) {
			lastErr = err
		}
	}
	return lastErr
}

type authEnvFileSnapshot struct {
	path   string
	data   []byte
	exists bool
}

func snapshotAuthEnvFiles(configPath string, dataPath string) ([]authEnvFileSnapshot, error) {
	paths := resolveAuthEnvWritePaths(configPath, dataPath)
	snapshots := make([]authEnvFileSnapshot, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			snapshots = append(snapshots, authEnvFileSnapshot{path: path})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read auth environment file %s: %w", path, err)
		}
		snapshots = append(snapshots, authEnvFileSnapshot{
			path:   path,
			data:   data,
			exists: true,
		})
	}
	return snapshots, nil
}

// A failed reset restores exact bytes with the same private publication boundary
// as a normal save, not the old inode or its potentially wider permissions.
func restoreAuthEnvFiles(snapshots []authEnvFileSnapshot) error {
	var restoreErr error
	for _, snapshot := range snapshots {
		if !snapshot.exists {
			if err := os.Remove(snapshot.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				restoreErr = errors.Join(restoreErr, fmt.Errorf("remove newly created auth environment file %s: %w", snapshot.path, err))
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(snapshot.path), 0o755); err != nil {
			restoreErr = errors.Join(restoreErr, fmt.Errorf("recreate auth environment directory for %s: %w", snapshot.path, err))
			continue
		}
		if err := writePrivateAuthSnapshot(snapshot.path, snapshot.data, nil); err != nil {
			restoreErr = errors.Join(restoreErr, fmt.Errorf("restore auth environment file %s: %w", snapshot.path, err))
		}
	}
	return restoreErr
}
