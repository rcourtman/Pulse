//go:build windows

package agentupdate

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func replacePendingUpdateFile(from, to string) error {
	fromPath, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return fmt.Errorf("encode pending update source path: %w", err)
	}
	toPath, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return fmt.Errorf("encode pending update destination path: %w", err)
	}
	if err := windows.MoveFileEx(
		fromPath,
		toPath,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	); err != nil {
		return fmt.Errorf("replace pending update handoff with write-through: %w", err)
	}
	return nil
}

func syncUpdateDirectory(string) error {
	// MoveFileEx with WRITE_THROUGH is the Windows replacement durability
	// barrier. Windows does not expose a portable directory fsync equivalent.
	return nil
}
