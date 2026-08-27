//go:build windows

package monitoring

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func replaceDeadManStateFile(from, to string) error {
	fromPath, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return fmt.Errorf("encode dead-man state source path: %w", err)
	}
	toPath, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return fmt.Errorf("encode dead-man state destination path: %w", err)
	}
	if err := windows.MoveFileEx(
		fromPath,
		toPath,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	); err != nil {
		return fmt.Errorf("replace dead-man state with write-through: %w", err)
	}
	return nil
}

func syncDeadManStateDirectory(string) error { return nil }
