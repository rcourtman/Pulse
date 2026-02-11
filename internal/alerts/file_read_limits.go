package alerts

import (
	"fmt"
	"io"
	"os"
)

const (
	// Persisted alert files are local state files; bound reads to limit memory pressure
	// from tampered files while still allowing large but legitimate datasets.
	maxActiveAlertsFileSizeBytes int64 = 16 * 1024 * 1024
	maxAlertHistoryFileSizeBytes int64 = 64 * 1024 * 1024
)

// readLimitedRegularFile reads a file with a strict max-size limit and rejects
// non-regular files to avoid unbounded reads from special files or symlinks.
func readLimitedRegularFile(path string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("invalid max file size %d", maxBytes)
	}

	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to read non-regular file %q", path)
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("file %q exceeds max size %d bytes", path, maxBytes)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("file %q exceeds max size %d bytes", path, maxBytes)
	}

	return data, nil
}
