package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// TemporaryFile is the owned descriptor used to prepare a configuration write.
// FileSystem implementations must create it exclusively with owner-only access.
type TemporaryFile interface {
	Name() string
	Write([]byte) (int, error)
	Chmod(os.FileMode) error
	Close() error
}

func (dfs defaultFileSystem) CreateTemp(dir, pattern string) (TemporaryFile, error) {
	return os.CreateTemp(dir, pattern)
}

// writeConfigFileAtomic publishes only a complete, closed file in the same
// directory. It never reopens the temporary pathname or follows an existing
// destination link. The caller owns directory admission and serialization.
func writeConfigFileAtomic(fs FileSystem, path string, data []byte, perm os.FileMode) (retErr error) {
	f, err := fs.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config file for %s: %w", path, err)
	}
	tmp := f.Name()
	closed, published := false, false
	defer func() {
		if !closed {
			if err := f.Close(); err != nil {
				retErr = errors.Join(retErr, fmt.Errorf("close temp config file %s: %w", tmp, err))
			}
		}
		if !published {
			if err := fs.Remove(tmp); err != nil && !os.IsNotExist(err) {
				retErr = errors.Join(retErr, fmt.Errorf("cleanup temp config file %s: %w", tmp, err))
			}
		}
	}()

	if err := f.Chmod(perm); err != nil {
		return fmt.Errorf("chmod temp config file %s: %w", tmp, err)
	}
	n, err := f.Write(data)
	if err != nil {
		return fmt.Errorf("write temp config file %s: %w", tmp, err)
	}
	if n != len(data) {
		return fmt.Errorf("write temp config file %s: %w", tmp, io.ErrShortWrite)
	}
	closed = true
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp config file %s: %w", tmp, err)
	}
	if err := fs.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename temp config file %s to %s: %w", tmp, path, err)
	}
	published = true
	return nil
}
