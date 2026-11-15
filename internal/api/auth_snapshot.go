package api

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// authSnapshotWriteFunc keeps the actual descriptor write independently
// exercisable by per-store fixtures, without replacing snapshot publication or
// sharing a mutable global hook. Runtime stores use the default file.Write.
type authSnapshotWriteFunc func(*os.File, []byte) (int, error)

// writePrivateAuthSnapshot publishes already-encoded authority using a new
// owner-private regular file. Never reuse path+".tmp": WriteFile would follow a
// stale symlink or keep an old file's wider mode. The caller owns serialization
// and directory preparation; this helper owns only its exclusive descriptor and
// temporary pathname. It does not promise directory-fsync/power-loss durability.
func writePrivateAuthSnapshot(path string, data []byte, write authSnapshotWriteFunc) (retErr error) {
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create private snapshot: %w", err)
	}
	closed, published := false, false
	defer func() {
		if !closed {
			retErr = errors.Join(retErr, file.Close())
		}
		if !published {
			if err := os.Remove(file.Name()); err != nil && !os.IsNotExist(err) {
				retErr = errors.Join(retErr, fmt.Errorf("clean up private snapshot: %w", err))
			}
		}
	}()
	if write == nil {
		write = func(file *os.File, data []byte) (int, error) { return file.Write(data) }
	}
	if n, err := write(file, data); err != nil {
		return fmt.Errorf("write private snapshot: %w", err)
	} else if n != len(data) {
		return fmt.Errorf("write private snapshot: %w", io.ErrShortWrite)
	}
	closed = true
	if err := file.Close(); err != nil {
		return fmt.Errorf("close private snapshot: %w", err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("publish private snapshot: %w", err)
	}
	published = true
	return nil
}
