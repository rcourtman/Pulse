package config

import (
	"errors"
	"fmt"
)

const (
	// Metadata files are local state; bound reads to limit memory pressure from
	// tampered files while still allowing large but legitimate datasets.
	maxGuestMetadataFileSizeBytes  int64 = 16 * 1024 * 1024
	maxHostMetadataFileSizeBytes   int64 = 16 * 1024 * 1024
	maxDockerMetadataFileSizeBytes int64 = 16 * 1024 * 1024
	maxHostContinuityFileSizeBytes int64 = 16 * 1024 * 1024

	// host_metadata.json holds two things: annotations, which every signed-in
	// user may write for any host ID, and each host's command policy. Writes
	// through HostMetadataStore.Set stop growing the file here, so that they
	// cannot use up the room SetCommandsEnabled needs for a policy change.
	maxHostAnnotationsFileSizeBytes = maxHostMetadataFileSizeBytes / 2
)

// ErrMetadataFileTooLarge is returned when a metadata change is refused
// because the file holding it would outgrow what its loader reads back.
var ErrMetadataFileTooLarge = errors.New("metadata file size limit reached")

// checkMetadataFileSize refuses a metadata snapshot its loader would not read
// back. A store that cannot read its file starts empty and replaces the file
// at its next write, so one oversized write would cost every record in it.
func checkMetadataFileSize(fileName string, data []byte, maxBytes int64) error {
	if int64(len(data)) > maxBytes {
		return fmt.Errorf("%w: %s would be %d bytes, over the %d byte limit", ErrMetadataFileTooLarge, fileName, len(data), maxBytes)
	}
	return nil
}

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
