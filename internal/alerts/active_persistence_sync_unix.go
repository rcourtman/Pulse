//go:build !windows

package alerts

import (
	"fmt"
	"os"
)

func syncActiveAlertsDirectory(path string) error {
	dirHandle, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open alerts directory for sync: %w", err)
	}
	if err := dirHandle.Sync(); err != nil {
		_ = dirHandle.Close()
		return fmt.Errorf("failed to sync alerts directory: %w", err)
	}
	if err := dirHandle.Close(); err != nil {
		return fmt.Errorf("failed to close alerts directory after sync: %w", err)
	}
	return nil
}

func replaceActiveAlertsFile(from, to string) error {
	return os.Rename(from, to)
}
