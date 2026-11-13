package config

import "sync"

// persistenceMutex serializes stores opened for the same admitted directory in
// this process. Router, watcher and tenant consumers can own separate instances;
// an import must not race another instance's save during replacement/rollback.
// The zero value is local, for literal fixtures and the private import writer.
// directory is immutable and assigned only after normal path admission.
type persistenceMutex struct {
	directory string
	local     sync.RWMutex
}

type persistenceDirectoryLock struct {
	mu   sync.RWMutex
	refs int // Held locks and waiters, not persistence instances.
}

var persistenceDirectoryLocks = struct {
	sync.Mutex
	directories map[string]*persistenceDirectoryLock
}{directories: make(map[string]*persistenceDirectoryLock)}

func retainPersistenceDirectoryLock(directory string) *persistenceDirectoryLock {
	persistenceDirectoryLocks.Lock()
	defer persistenceDirectoryLocks.Unlock()
	entry := persistenceDirectoryLocks.directories[directory]
	if entry == nil {
		entry = &persistenceDirectoryLock{}
		persistenceDirectoryLocks.directories[directory] = entry
	}
	entry.refs++
	return entry
}

func releasePersistenceDirectoryLock(directory string, read bool) {
	persistenceDirectoryLocks.Lock()
	defer persistenceDirectoryLocks.Unlock()
	entry := persistenceDirectoryLocks.directories[directory]
	// Acquisition never waits on entry.mu while holding the registry lock.
	// Unlock cannot block, and active/waiting references keep this entry alive.
	if read {
		entry.mu.RUnlock()
	} else {
		entry.mu.Unlock()
	}
	entry.refs--
	if entry.refs == 0 {
		delete(persistenceDirectoryLocks.directories, directory)
	}
}

func (m *persistenceMutex) Lock() {
	if m.directory == "" {
		m.local.Lock()
		return
	}
	retainPersistenceDirectoryLock(m.directory).mu.Lock()
}

func (m *persistenceMutex) Unlock() {
	if m.directory == "" {
		m.local.Unlock()
		return
	}
	releasePersistenceDirectoryLock(m.directory, false)
}

func (m *persistenceMutex) RLock() {
	if m.directory == "" {
		m.local.RLock()
		return
	}
	retainPersistenceDirectoryLock(m.directory).mu.RLock()
}

func (m *persistenceMutex) RUnlock() {
	if m.directory == "" {
		m.local.RUnlock()
		return
	}
	releasePersistenceDirectoryLock(m.directory, true)
}
