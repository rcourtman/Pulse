//go:build !windows

package monitoring

import (
	"fmt"
	"os"
)

func replaceDeadManStateFile(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return fmt.Errorf("replace dead-man state: %w", err)
	}
	return nil
}

func syncDeadManStateDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open dead-man state directory for sync: %w", err)
	}
	if err := dir.Sync(); err != nil {
		_ = dir.Close()
		return fmt.Errorf("sync dead-man state directory: %w", err)
	}
	if err := dir.Close(); err != nil {
		return fmt.Errorf("close dead-man state directory: %w", err)
	}
	return nil
}
