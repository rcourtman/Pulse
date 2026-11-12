// Package agentstate reads the small authority files used at agent startup.
package agentstate

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// MaxFileBytes bounds a complete saved token or identity, including whitespace.
// It is deliberately much larger than the credentials and IDs Pulse issues.
const MaxFileBytes = 64 * 1024

var (
	ErrNotRegular = errors.New("agent state must be a regular file")
	ErrTooLarge   = errors.New("agent state exceeds the 64 KiB file limit")
	// ErrUnavailable is deliberately not os.ErrNotExist: a saved entry whose
	// target disappeared is not permission to start with replacement authority.
	ErrUnavailable = errors.New("saved agent state is unavailable")
)

// ReadFile follows file/directory links, including projected Kubernetes secrets
// and relocated state, but never reads a stream or more than MaxFileBytes+1.
// It does not impose a deadline on underlying filesystem operations.
func ReadFile(path string) ([]byte, error) {
	// Reject known devices/streams before opening them. Check the opened handle
	// again below: a file or projected-secret link may change between the calls.
	info, err := os.Stat(path)
	if err != nil {
		return nil, classifyMissingFile(path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, ErrNotRegular
	}
	if info.Size() > MaxFileBytes {
		return nil, ErrTooLarge
	}
	f, err := openStateFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Stat already observed saved state. Losing it before open is not
			// a first start, even if the link or file has now vanished entirely.
			return nil, ErrUnavailable
		}
		return nil, err
	}
	defer f.Close()
	return readOpenedFile(f)
}

// classifyMissingFile keeps real first-run absence distinct from an existing
// file/directory link with a missing target. Walk only until the nearest extant
// entry; a missing child beneath a live directory link is still ordinary absence.
// This is a conservative startup check, not a filesystem snapshot or I/O deadline.
func classifyMissingFile(path string, err error) error {
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	entry := trimTrailingSeparators(path)
	for current := entry; ; {
		info, checkErr := os.Lstat(current)
		if checkErr == nil {
			if current == entry {
				// The entry exists, or reappeared after the failed Stat. Do not
				// infer absence from the earlier missing-target observation.
				return ErrUnavailable
			}
			if info.Mode()&os.ModeSymlink != 0 {
				if _, targetErr := os.Stat(current); targetErr != nil {
					return ErrUnavailable
				}
			}
			return err
		}
		if !errors.Is(checkErr, os.ErrNotExist) {
			return ErrUnavailable // Absence could not be confirmed.
		}
		// Do not Clean or use Dir here: collapsing link/.. would bypass an
		// unavailable link that the operating system must actually traverse.
		parent, _ := filepath.Split(current)
		parent = trimTrailingSeparators(parent)
		if parent == "" {
			parent = "."
		}
		if parent == current {
			return err
		}
		current = parent
	}
}

func trimTrailingSeparators(path string) string {
	// Keep a volume's root separator (also the Unix root), without changing
	// relative components or the meaning of paths through directory links.
	rootLength := len(filepath.VolumeName(path)) + 1
	for len(path) > rootLength && os.IsPathSeparator(path[len(path)-1]) {
		path = path[:len(path)-1]
	}
	return path
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
