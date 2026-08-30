//go:build !windows

package agentupdate

import "os"

func replacePendingUpdateFile(from, to string) error {
	return os.Rename(from, to)
}

func syncUpdateDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
