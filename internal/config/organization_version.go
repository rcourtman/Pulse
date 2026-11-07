package config

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
)

// ErrOrganizationChanged means another write (or deletion) superseded the
// loaded snapshot. Callers must read and authorize a new intentional operation,
// not automatically retry or merge old grants into the current organization.
var ErrOrganizationChanged = errors.New("organization changed since it was read")

func organizationVersion(path string, data []byte) string {
	h := sha256.New()
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

type organizationFileLock struct {
	mu   sync.Mutex
	refs int
}

var organizationFileLocks = struct {
	sync.Mutex
	files map[string]*organizationFileLock
}{files: make(map[string]*organizationFileLock)}

// lockOrganizationFile serializes comparison/publication and deletion across
// persistence instances in this process. Entries include waiters and retire
// when unused, so removed tenants do not leave an unbounded lock registry.
// Locks never cover request-body reads, provider work or response transmission.
func lockOrganizationFile(path string) func() {
	organizationFileLocks.Lock()
	entry := organizationFileLocks.files[path]
	if entry == nil {
		entry = &organizationFileLock{}
		organizationFileLocks.files[path] = entry
	}
	entry.refs++
	organizationFileLocks.Unlock()
	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		organizationFileLocks.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(organizationFileLocks.files, path)
		}
		organizationFileLocks.Unlock()
	}
}
