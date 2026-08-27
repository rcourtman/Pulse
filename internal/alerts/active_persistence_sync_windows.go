//go:build windows

package alerts

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func syncActiveAlertsDirectory(path string) error {
	// MoveFileEx with WRITE_THROUGH is the Windows metadata durability barrier;
	// FlushFileBuffers is documented for file handles with GENERIC_WRITE and is
	// not one of the operations guaranteed for directory handles.
	return nil
}

func replaceActiveAlertsFile(from, to string) error {
	fromPath, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return fmt.Errorf("encode active alerts source path: %w", err)
	}
	toPath, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return fmt.Errorf("encode active alerts destination path: %w", err)
	}
	if err := windows.MoveFileEx(
		fromPath,
		toPath,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	); err != nil {
		return fmt.Errorf("replace active alerts file with write-through: %w", err)
	}
	return nil
}
