package pulsecli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type configExportFile interface {
	io.WriteCloser
	Name() string
	Chmod(os.FileMode) error
	Sync() error
}

// Keep fault injection local to a write, not in process-wide filesystem hooks.
type configExportFileOps struct {
	lstat      func(string) (os.FileInfo, error)
	createTemp func(string, string) (configExportFile, error)
	rename     func(string, string) error
	remove     func(string) error
}

func writeConfigExportFile(path string, data []byte) error {
	return publishConfigExportFile(path, data, configExportFileOps{
		lstat: os.Lstat,
		createTemp: func(dir, pattern string) (configExportFile, error) {
			return os.CreateTemp(dir, pattern)
		},
		rename: os.Rename,
		remove: os.Remove,
	})
}

// Publish only a complete private export; never truncate the previous backup or
// write through its final symlink/hard link. The containing directory is chosen
// by the operator. This is not a power-loss or hostile-directory guarantee.
func publishConfigExportFile(path string, data []byte, ops configExportFileOps) (retErr error) {
	if info, err := ops.lstat(path); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("inspect export destination: %w", err)
		}
	} else if !info.Mode().IsRegular() {
		return fmt.Errorf("export destination is not a regular file (symlinks are not supported)")
	}

	f, err := ops.createTemp(filepath.Dir(path), ".pulse-config-export-*")
	if err != nil {
		return fmt.Errorf("create private export file: %w", err)
	}
	closed, published := false, false
	defer func() {
		if !closed {
			if err := f.Close(); err != nil {
				retErr = errors.Join(retErr, fmt.Errorf("close private export file: %w", err))
			}
		}
		if !published {
			if err := ops.remove(f.Name()); err != nil && !os.IsNotExist(err) {
				retErr = errors.Join(retErr, fmt.Errorf("clean up private export file: %w", err))
			}
		}
	}()

	if err := f.Chmod(0o600); err != nil {
		return fmt.Errorf("set private export permissions: %w", err)
	}
	if n, err := f.Write(data); err != nil {
		return fmt.Errorf("write private export file: %w", err)
	} else if n != len(data) {
		return fmt.Errorf("write private export file: %w", io.ErrShortWrite)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync private export file: %w", err)
	}
	closed = true
	if err := f.Close(); err != nil {
		return fmt.Errorf("close private export file: %w", err)
	}
	if err := ops.rename(f.Name(), path); err != nil {
		return fmt.Errorf("publish private export file: %w", err)
	}
	published = true
	return nil
}
