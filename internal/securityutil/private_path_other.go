//go:build !windows

package securityutil

import (
	"fmt"
	"os"
)

// HardenPrivatePath applies an owner-only mode to a file or directory.
func HardenPrivatePath(path string, mode os.FileMode) error {
	before, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect private path before hardening: %w", err)
	}
	if before.Mode()&os.ModeSymlink != 0 || (!before.IsDir() && !before.Mode().IsRegular()) {
		return fmt.Errorf("private path must be a real file or directory")
	}
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("harden private path permissions: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect hardened private path: %w", err)
	}
	return ValidatePrivatePath(path, info)
}

// ValidatePrivatePath rejects paths accessible through group or other mode
// bits. Callers separately validate the expected filesystem object type.
func ValidatePrivatePath(_ string, info os.FileInfo) error {
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("private path grants group or other access")
	}
	return nil
}
