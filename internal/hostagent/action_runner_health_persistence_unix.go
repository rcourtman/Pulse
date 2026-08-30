//go:build !windows

package hostagent

import (
	"fmt"
	"os"
)

func replaceActionRunnerHealthFile(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return fmt.Errorf("replace action-runner health marker: %w", err)
	}
	return nil
}

func syncActionRunnerHealthDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open action-runner health directory for sync: %w", err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return fmt.Errorf("sync action-runner health directory: %w", err)
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("close action-runner health directory after sync: %w", err)
	}
	return nil
}
