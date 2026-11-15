package hostagent

import (
	"context"
	"fmt"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/platformsupport"
)

// packageManagerLease is the single host-local authority for every APT-backed
// inspection and mutation. Keeping this above the individual managers prevents
// telemetry refresh from racing an update or cache cleanup.
type packageManagerLease struct {
	once sync.Once
	ch   chan struct{}
}

// supportsAPTPlatform resolves distro-specific agent identity through the
// canonical runtime-platform contract. Agents intentionally preserve values
// such as "debian" and "ubuntu" for display, so an exact "linux" comparison
// incorrectly disables the production APT managers on their primary hosts.
func supportsAPTPlatform(platform string) bool {
	return platformsupport.AgentCommandPlatform(platform) == platformsupport.RuntimePlatformLinux
}

func newPackageManagerLease() *packageManagerLease {
	lease := &packageManagerLease{}
	lease.init()
	return lease
}

func configurePackageManagers(platform string, updates *packageUpdateManager, cleanup *storageCleanupManager) (packageUpdateReporter, *storageCleanupManager) {
	lease := newPackageManagerLease()
	var reporter packageUpdateReporter
	if updates == nil {
		reporter = newPackageUpdateInspector(platform, lease)
	} else {
		updates.lease = lease
		reporter = updates
	}
	if cleanup == nil {
		cleanup = newStorageCleanupManager(platform, lease)
	} else {
		cleanup.lease = lease
	}
	return reporter, cleanup
}

func (l *packageManagerLease) init() {
	l.once.Do(func() {
		l.ch = make(chan struct{}, 1)
		l.ch <- struct{}{}
	})
}

func (l *packageManagerLease) acquire(ctx context.Context) (func(), error) {
	if l == nil {
		return nil, fmt.Errorf("shared package manager lease is required")
	}
	l.init()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-l.ch:
		return func() { l.ch <- struct{}{} }, nil
	}
}
