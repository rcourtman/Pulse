//go:build windows

package hostagent

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func replaceActionRunnerHealthFile(from, to string) error {
	fromPath, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return fmt.Errorf("encode action-runner health source path: %w", err)
	}
	toPath, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return fmt.Errorf("encode action-runner health destination path: %w", err)
	}
	if err := windows.MoveFileEx(
		fromPath,
		toPath,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	); err != nil {
		return fmt.Errorf("replace action-runner health marker with write-through: %w", err)
	}
	return nil
}

func syncActionRunnerHealthDirectory(string) error {
	// MoveFileEx with WRITE_THROUGH is the Windows metadata durability barrier;
	// FlushFileBuffers requires a write-capable handle and is not guaranteed for
	// the read-only directory handles returned by os.Open.
	return nil
}
