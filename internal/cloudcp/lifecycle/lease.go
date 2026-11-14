// Package lifecycle serializes control-plane operations that own a tenant's
// runtime or data. The lock is outside the tenant tree so restoring that tree
// cannot replace the lock inode or erase an unfinished recovery hold.
package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

var (
	ErrBusy             = errors.New("tenant runtime lifecycle is busy")
	ErrRecoveryRequired = errors.New("tenant runtime recovery is unfinished; retained recovery material must be reconciled before retry")
)

// Recovery records identities, never tenant contents or credentials. Presence
// alone is a hold, including an unreadable or partially written record.
type Recovery struct {
	TenantID            string `json:"tenant_id"`
	PreviousContainerID string `json:"previous_container_id,omitempty"`
	NewContainerID      string `json:"new_container_id,omitempty"`
	SnapshotDir         string `json:"snapshot_dir"`
	BackupContainerName string `json:"backup_container_name,omitempty"`
	Phase               string `json:"phase"`
}

type contextKey struct{}

// Lease is process-independent exclusion. Closing it never clears a recovery
// hold: only the transaction that verified completion may call Complete.
type Lease struct {
	file     *os.File
	path     string
	active   atomic.Bool
	marked   bool
	record   Recovery
	borrowed bool
	owner    *Lease
}

func ValidTenantID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

// Acquire does not queue: monitors skip a busy tenant and mutating callers
// return an error without touching its runtime or data. Nested lifecycle calls
// may borrow the same active lease only through the owning internal context.
func Acquire(ctx context.Context, controlPlaneDir, tenantID string) (context.Context, *Lease, error) {
	if err := ctx.Err(); err != nil {
		return ctx, nil, err
	}
	if strings.TrimSpace(controlPlaneDir) == "" || !ValidTenantID(tenantID) {
		return ctx, nil, fmt.Errorf("control-plane directory and valid tenant id are required")
	}
	dir, err := filepath.Abs(filepath.Join(controlPlaneDir, "runtime-lifecycle"))
	if err != nil {
		return ctx, nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ctx, nil, err
	}
	if err := syncDirectory(filepath.Dir(dir)); err != nil {
		return ctx, nil, err
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return ctx, nil, err
	}
	path := filepath.Join(dir, tenantID+".lock")
	if owner, ok := ctx.Value(contextKey{}).(*Lease); ok && owner.path == path && owner.active.Load() {
		return ctx, &Lease{borrowed: true, owner: owner}, nil
	}
	f, err := openAndLock(path)
	if err != nil {
		return ctx, nil, fmt.Errorf("%w: %s: %v", ErrBusy, tenantID, err)
	}
	l := &Lease{file: f, path: path}
	l.active.Store(true)
	if _, err := os.Lstat(l.recoveryPath()); !os.IsNotExist(err) {
		_ = l.Close()
		return ctx, nil, fmt.Errorf("%w: %s", ErrRecoveryRequired, l.recoveryPath())
	}
	return context.WithValue(ctx, contextKey{}, l), l, nil
}

func (l *Lease) recoveryPath() string { return strings.TrimSuffix(l.path, ".lock") + ".recovery.json" }

// MarkRecovery is durably recorded before any destructive operation. Updates
// retain the earlier complete record until the replacement is synced.
func (l *Lease) MarkRecovery(record Recovery) error {
	if l != nil && l.borrowed {
		return l.owner.MarkRecovery(record)
	}
	if l == nil || !l.active.Load() || l.borrowed {
		return fmt.Errorf("an owning lifecycle lease is required")
	}
	l.record = record
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if !l.marked {
		f, err := os.OpenFile(l.recoveryPath(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		l.marked = true // Even a failed write must leave a hold.
		_, writeErr := f.Write(data)
		syncErr := f.Sync()
		closeErr := f.Close()
		return errors.Join(writeErr, syncErr, closeErr, syncDirectory(filepath.Dir(l.path)))
	}
	f, err := os.CreateTemp(filepath.Dir(l.path), ".recovery-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), l.recoveryPath()); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(l.path))
}

func (l *Lease) Complete() error {
	if l != nil && l.borrowed {
		return l.owner.Complete()
	}
	if l == nil || !l.active.Load() || l.borrowed {
		return fmt.Errorf("an owning lifecycle lease is required")
	}
	if !l.marked {
		return nil
	}
	if err := os.Remove(l.recoveryPath()); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Dir(l.path)); err != nil {
		return errors.Join(err, l.MarkRecovery(l.record))
	}
	l.marked = false
	return nil
}

func (l *Lease) Close() error {
	if l == nil || l.borrowed || !l.active.Swap(false) {
		return nil
	}
	// Never unlink the lock: a waiter can already have opened its inode.
	return l.file.Close()
}

func syncDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
