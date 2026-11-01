//go:build !windows

package agentupdate

import "os"

func replacePendingUpdateFile(from, to string) error {
	return os.Rename(from, to)
}

// syncUpdateFile flushes a closed file's data and inode metadata, including
// a mode set after it was written; fsync applies to the file, not the
// descriptor, so a read-only one suffices.
func syncUpdateFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func syncUpdateDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
