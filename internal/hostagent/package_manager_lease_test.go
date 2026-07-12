package hostagent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

func TestPackageManagerLeaseNilFailsClosed(t *testing.T) {
	if release, err := (*packageManagerLease)(nil).acquire(context.Background()); err == nil || release != nil {
		t.Fatalf("nil lease acquire returned a release function=%t err=%v", release != nil, err)
	}
	manager := newPackageUpdateManager("linux", nil)
	called := false
	manager.run = func(context.Context, []string, string, ...string) packageUpdateCommandResult {
		called = true
		return packageUpdateCommandResult{}
	}
	snapshot := manager.Snapshot(context.Background(), true)
	if called || snapshot.Error == "" {
		t.Fatalf("nil lease did not fail closed: called=%t snapshot=%#v", called, snapshot)
	}
}

func TestSupportsAPTPlatformUsesCanonicalRuntimeIdentity(t *testing.T) {
	for _, platform := range []string{"linux", "debian", "ubuntu", "debian gnu/linux"} {
		if !supportsAPTPlatform(platform) {
			t.Fatalf("APT platform %q was rejected", platform)
		}
	}
	for _, platform := range []string{"windows", "macos", "freebsd"} {
		if supportsAPTPlatform(platform) {
			t.Fatalf("non-APT platform %q was accepted", platform)
		}
	}
}

func TestConfigurePackageManagersInjectsOneSharedLeaseAndSerializesRefresh(t *testing.T) {
	updates, cleanup := configurePackageManagers("linux", nil, nil)
	if updates.lease == nil || cleanup.lease == nil || updates.lease != cleanup.lease {
		t.Fatalf("managers do not share one lease: update=%p cleanup=%p", updates.lease, cleanup.lease)
	}
	updates.lookPath = func(string) (string, error) { return "/usr/bin/apt-get", nil }
	cleanup.lookPath = func(string) (string, error) { return "/usr/bin/apt-get", nil }
	entered := make(chan struct{})
	releaseUpdate := make(chan struct{})
	updates.run = func(context.Context, []string, string, ...string) packageUpdateCommandResult {
		close(entered)
		<-releaseUpdate
		return packageUpdateCommandResult{}
	}
	var cleanupEntered atomic.Bool
	cleanup.scan = func() (agentexec.HostStorageCleanupSnapshot, error) {
		cleanupEntered.Store(true)
		return agentexec.HostStorageCleanupSnapshot{}, nil
	}
	doneUpdate := make(chan struct{})
	go func() { updates.Snapshot(context.Background(), true); close(doneUpdate) }()
	<-entered
	doneCleanup := make(chan struct{})
	go func() { cleanup.Snapshot(context.Background(), true); close(doneCleanup) }()
	time.Sleep(25 * time.Millisecond)
	if cleanupEntered.Load() {
		t.Fatal("cleanup refresh entered while update refresh held the shared lease")
	}
	close(releaseUpdate)
	<-doneUpdate
	select {
	case <-doneCleanup:
	case <-time.After(time.Second):
		t.Fatal("cleanup refresh did not proceed after shared lease release")
	}
	if !cleanupEntered.Load() {
		t.Fatal("cleanup refresh never entered")
	}
}

func TestConfigurePackageManagersRebindsInjectedManagersToOneAuthority(t *testing.T) {
	updates := newPackageUpdateManager("linux", nil)
	cleanup := newStorageCleanupManager("linux", nil)
	gotUpdates, gotCleanup := configurePackageManagers("linux", updates, cleanup)
	if gotUpdates != updates || gotCleanup != cleanup || gotUpdates.lease == nil || gotUpdates.lease != gotCleanup.lease {
		t.Fatal("runtime composition did not inject one shared package manager authority")
	}
}
