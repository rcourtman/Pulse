// Package agentstate reads the small authority files used at agent startup.
package agentstate

import (
	"errors"
	"io"
	"os"
)

// MaxFileBytes bounds a complete saved token or identity, including whitespace.
// It is deliberately much larger than the credentials and IDs Pulse issues.
const MaxFileBytes = 64 * 1024

var (
	ErrNotRegular = errors.New("agent state must be a regular file")
	ErrTooLarge   = errors.New("agent state exceeds the 64 KiB file limit")
)

// ReadFile follows file/directory links, including projected Kubernetes secrets
// and relocated state, but never reads a stream or more than MaxFileBytes+1.
// It does not impose a deadline on underlying filesystem operations.
func ReadFile(path string) ([]byte, error) {
	// Reject known devices/streams before opening them. Check the opened handle
	// again below: a file or projected-secret link may change between the calls.
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrNotRegular
	}
	if info.Size() > MaxFileBytes {
		return nil, ErrTooLarge
	}
	f, err := openStateFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readOpenedFile(f)
}

func readOpenedFile(f *os.File) ([]byte, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrNotRegular
	}
	return readBounded(f, info.Size())
}

func readBounded(r io.Reader, size int64) ([]byte, error) {
	if size > MaxFileBytes {
		return nil, ErrTooLarge
	}
	// Stat alone cannot bound a file which grows during the read, or one whose
	// reported size is smaller than the bytes it serves. Never accept a prefix.
	data, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileBytes {
		return nil, ErrTooLarge
	}
	return data, nil
}
