package config

import "fmt"

const (
	// Metadata files are local state; bound reads to limit memory pressure from
	// tampered files while still allowing large but legitimate datasets.
	maxGuestMetadataFileSizeBytes  int64 = 16 * 1024 * 1024
	maxHostMetadataFileSizeBytes   int64 = 16 * 1024 * 1024
	maxDockerMetadataFileSizeBytes int64 = 16 * 1024 * 1024
)

// readLimitedRegularFileFS reads from the configured filesystem with a strict
// max-size limit and rejects non-regular files.
func readLimitedRegularFileFS(fs FileSystem, path string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("invalid max file size %d", maxBytes)
	}

	if fs == nil {
		fs = defaultFileSystem{}
	}

	info, err := fs.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to read non-regular file %q", path)
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("file %q exceeds max size %d bytes", path, maxBytes)
	}

	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("file %q exceeds max size %d bytes", path, maxBytes)
	}

	return data, nil
}
