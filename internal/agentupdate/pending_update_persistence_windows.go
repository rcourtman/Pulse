//go:build windows

package agentupdate

import (
	"fmt"
	"os"

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

// syncUpdateFile flushes a closed file's data. FlushFileBuffers needs a
// handle with write access; Windows has no executable mode bit to persist.
func syncUpdateFile(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func syncUpdateDirectory(string) error {
	// MoveFileEx with WRITE_THROUGH is the Windows replacement durability
	// barrier. Windows does not expose a portable directory fsync equivalent.
	return nil
}
