package cloudcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/errdefs"
	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestTenantRuntimeRollout_ReconcilesLiveContainerAlreadyOnTargetImage(t *testing.T) {
	tenant := &registry.Tenant{ID: "t-ROLLSYNC", ContainerID: "stale-registry-id"}
	reg := &fakeTenantRuntimeRolloutRegistry{tenant: tenant}
	docker := newFakeTenantRuntimeRolloutDocker()
	docker.addContainer(&cpDocker.RuntimeContainerInfo{
		ID:       "live-container",
		Name:     tenantRuntimeContainerName(tenant.ID),
		ImageRef: "pulse-runtime:target",
		ImageID:  "sha256:target",
		Running:  true,
	})
	docker.health["live-container"] = []bool{true}
	sync := &fakeTenantRuntimeRolloutSynchronizer{}
	clock := newFakeTenantRuntimeRolloutClock()

	service := newTestTenantRuntimeRolloutService(reg, docker, sync, clock)
	result, err := service.Rollout(context.Background(), TenantRuntimeRolloutOptions{
		TenantID: tenant.ID,
		Image:    "pulse-runtime:target",
	})
	if err != nil {
		t.Fatalf("Rollout() error = %v", err)
	}

	if !result.ReconciledOnly {
		t.Fatalf("ReconciledOnly = false, want true")
	}
	if len(sync.snapshots) != 0 {
		t.Fatalf("snapshot count = %d, want 0", len(sync.snapshots))
	}
	if got := reg.updatedTenant.ContainerID; got != "live-container" {
		t.Fatalf("updated tenant container id = %q, want live-container", got)
	}
	if got := reg.updatedTenant.CurrentImageDigest; got != "sha256:target" {
		t.Fatalf("updated tenant image digest = %q, want sha256:target", got)
	}
	if !reg.updatedTenant.HealthCheckOK {
		t.Fatalf("updated tenant health_check_ok = false, want true")
	}
	if len(docker.createCalls) != 0 {
		t.Fatalf("create call count = %d, want 0", len(docker.createCalls))
	}
}

func TestTenantRuntimeRollout_RollsForwardCanonically(t *testing.T) {
	tenant := &registry.Tenant{ID: "t-ROLLFWD", ContainerID: "old-container"}
	reg := &fakeTenantRuntimeRolloutRegistry{tenant: tenant}
	docker := newFakeTenantRuntimeRolloutDocker()
	docker.addContainer(&cpDocker.RuntimeContainerInfo{
		ID:       "old-container",
		Name:     tenantRuntimeContainerName(tenant.ID),
		ImageRef: "pulse-runtime:old",
		ImageID:  "sha256:old",
		Running:  true,
	})
	docker.queueCreate(&cpDocker.RuntimeContainerInfo{
		ID:       "new-container",
		Name:     tenantRuntimeContainerName(tenant.ID),
		ImageRef: "pulse-runtime:new",
		ImageID:  "sha256:new",
		Running:  true,
	}, nil)
	docker.health["new-container"] = []bool{true}
	sync := &fakeTenantRuntimeRolloutSynchronizer{}
	clock := newFakeTenantRuntimeRolloutClock()

	service := newTestTenantRuntimeRolloutService(reg, docker, sync, clock)
	result, err := service.Rollout(context.Background(), TenantRuntimeRolloutOptions{
		TenantID: tenant.ID,
		Image:    "pulse-runtime:new",
		RunID:    "aliasfix",
	})
	if err != nil {
		t.Fatalf("Rollout() error = %v", err)
	}

	if result.ReconciledOnly {
		t.Fatalf("ReconciledOnly = true, want false")
	}
	if got := result.BackupContainerName; got != "pulse-t-ROLLFWD.pre-aliasfix" {
		t.Fatalf("backup container name = %q, want pulse-t-ROLLFWD.pre-aliasfix", got)
	}
	if len(sync.snapshots) != 1 {
		t.Fatalf("snapshot count = %d, want 1", len(sync.snapshots))
	}
	if len(sync.restores) != 0 {
		t.Fatalf("restore count = %d, want 0", len(sync.restores))
	}
	if got := reg.updatedTenant.ContainerID; got != "new-container" {
		t.Fatalf("updated tenant container id = %q, want new-container", got)
	}
	if got := reg.updatedTenant.CurrentImageDigest; got != "sha256:new" {
		t.Fatalf("updated tenant image digest = %q, want sha256:new", got)
	}
	if len(docker.renameCalls) != 1 {
		t.Fatalf("rename count = %d, want 1", len(docker.renameCalls))
	}
	if docker.renameCalls[0].newName != "pulse-t-ROLLFWD.pre-aliasfix" {
		t.Fatalf("rename target = %q, want pulse-t-ROLLFWD.pre-aliasfix", docker.renameCalls[0].newName)
	}
}

func TestTenantRuntimeRollout_RollsBackOnHealthFailure(t *testing.T) {
	tenant := &registry.Tenant{ID: "t-ROLLBACK", ContainerID: "old-container"}
	reg := &fakeTenantRuntimeRolloutRegistry{tenant: tenant}
	docker := newFakeTenantRuntimeRolloutDocker()
	docker.addContainer(&cpDocker.RuntimeContainerInfo{
		ID:       "old-container",
		Name:     tenantRuntimeContainerName(tenant.ID),
		ImageRef: "pulse-runtime:old",
		ImageID:  "sha256:old",
		Running:  true,
	})
	docker.queueCreate(&cpDocker.RuntimeContainerInfo{
		ID:       "new-container",
		Name:     tenantRuntimeContainerName(tenant.ID),
		ImageRef: "pulse-runtime:new",
		ImageID:  "sha256:new",
		Running:  true,
	}, nil)
	docker.health["new-container"] = []bool{false, false, false}
	docker.health["old-container"] = []bool{true}
	sync := &fakeTenantRuntimeRolloutSynchronizer{}
	clock := newFakeTenantRuntimeRolloutClock()

	service := newTestTenantRuntimeRolloutService(reg, docker, sync, clock)
	_, err := service.Rollout(context.Background(), TenantRuntimeRolloutOptions{
		TenantID:      tenant.ID,
		Image:         "pulse-runtime:new",
		RunID:         "rollback",
		HealthTimeout: 5 * time.Second,
	})
	if err == nil {
		t.Fatalf("Rollout() error = nil, want rollback failure")
	}

	if len(sync.snapshots) != 1 {
		t.Fatalf("snapshot count = %d, want 1", len(sync.snapshots))
	}
	if len(sync.restores) != 1 {
		t.Fatalf("restore count = %d, want 1", len(sync.restores))
	}
	if got := reg.updatedTenant.ContainerID; got != "old-container" {
		t.Fatalf("updated tenant container id = %q, want old-container", got)
	}
	if got := reg.updatedTenant.CurrentImageDigest; got != "sha256:old" {
		t.Fatalf("updated tenant image digest = %q, want sha256:old", got)
	}
	if len(docker.removeCalls) == 0 || docker.removeCalls[0] != "new-container" {
		t.Fatalf("remove calls = %v, want new-container removed", docker.removeCalls)
	}
	if got := docker.byName[tenantRuntimeContainerName(tenant.ID)]; got == nil || got.ID != "old-container" {
		t.Fatalf("canonical container after rollback = %#v, want old-container", got)
	}
}

func TestFilesystemTenantRuntimeSynchronizer_SnapshotAndRestore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	srcDir := t.TempDir()
	snapshotDir := t.TempDir()
	restoreDir := t.TempDir()

	mustWrite := func(path, contents string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir parent %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(contents), mode); err != nil {
			t.Fatalf("write file %s: %v", path, err)
		}
	}

	mustWrite(filepath.Join(srcDir, "billing.json"), "{\"valid\":true}\n", 0o600)
	mustWrite(filepath.Join(srcDir, "secrets", "handoff.key"), "handoff-key\n", 0o600)
	if err := os.Symlink(filepath.Join("secrets", "handoff.key"), filepath.Join(srcDir, ".cloud_handoff_key")); err != nil {
		t.Fatalf("symlink .cloud_handoff_key: %v", err)
	}

	mustWrite(filepath.Join(snapshotDir, "stale.txt"), "remove me\n", 0o644)

	syncer := filesystemTenantRuntimeSynchronizer{}
	if err := syncer.Snapshot(ctx, srcDir, snapshotDir); err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(snapshotDir, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("snapshot stale.txt stat err = %v, want not-exist", err)
	}
	billingPath := filepath.Join(snapshotDir, "billing.json")
	billingBytes, err := os.ReadFile(billingPath)
	if err != nil {
		t.Fatalf("read snapshot billing.json: %v", err)
	}
	if got := string(billingBytes); got != "{\"valid\":true}\n" {
		t.Fatalf("snapshot billing.json = %q, want %q", got, "{\"valid\":true}\n")
	}
	if info, err := os.Stat(billingPath); err != nil {
		t.Fatalf("stat snapshot billing.json: %v", err)
	} else if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("snapshot billing.json mode = %o, want 600", got)
	}
	if target, err := os.Readlink(filepath.Join(snapshotDir, ".cloud_handoff_key")); err != nil {
		t.Fatalf("readlink snapshot .cloud_handoff_key: %v", err)
	} else if target != filepath.Join("secrets", "handoff.key") {
		t.Fatalf("snapshot symlink target = %q, want %q", target, filepath.Join("secrets", "handoff.key"))
	}

	mustWrite(filepath.Join(restoreDir, "obsolete.txt"), "old\n", 0o644)
	if err := syncer.Restore(ctx, snapshotDir, restoreDir); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(restoreDir, "obsolete.txt")); !os.IsNotExist(err) {
		t.Fatalf("restore obsolete.txt stat err = %v, want not-exist", err)
	}
	restoredBytes, err := os.ReadFile(filepath.Join(restoreDir, "secrets", "handoff.key"))
	if err != nil {
		t.Fatalf("read restored handoff.key: %v", err)
	}
	if got := string(restoredBytes); got != "handoff-key\n" {
		t.Fatalf("restored handoff.key = %q, want %q", got, "handoff-key\n")
	}
}

func newTestTenantRuntimeRolloutService(
	reg tenantRuntimeRolloutRegistry,
	docker tenantRuntimeRolloutDocker,
	sync tenantRuntimeRolloutSynchronizer,
	clock *fakeTenantRuntimeRolloutClock,
) *tenantRuntimeRolloutService {
	return &tenantRuntimeRolloutService{
		registry:      reg,
		docker:        docker,
		tenantsDir:    tTempDirForRolloutService(),
		synchronizer:  sync,
		now:           clock.Now,
		sleep:         clock.Sleep,
		healthTimeout: 5 * time.Second,
		healthPoll:    1 * time.Second,
	}
}

func tTempDirForRolloutService() string {
	return "/tmp/pulse-rollout-tests"
}

type fakeTenantRuntimeRolloutRegistry struct {
	tenant        *registry.Tenant
	updatedTenant registry.Tenant
	updateCount   int
}

func (f *fakeTenantRuntimeRolloutRegistry) Get(tenantID string) (*registry.Tenant, error) {
	if f.tenant == nil || f.tenant.ID != tenantID {
		return nil, nil
	}
	copy := *f.tenant
	return &copy, nil
}

func (f *fakeTenantRuntimeRolloutRegistry) Update(t *registry.Tenant) error {
	if t == nil {
		return fmt.Errorf("tenant is nil")
	}
	f.updatedTenant = *t
	f.updateCount++
	f.tenant = &f.updatedTenant
	return nil
}

type fakeTenantRuntimeRolloutSynchronizer struct {
	snapshots [][2]string
	restores  [][2]string
}

func (f *fakeTenantRuntimeRolloutSynchronizer) Snapshot(_ context.Context, srcDir, snapshotDir string) error {
	f.snapshots = append(f.snapshots, [2]string{srcDir, snapshotDir})
	return nil
}

func (f *fakeTenantRuntimeRolloutSynchronizer) Restore(_ context.Context, snapshotDir, dstDir string) error {
	f.restores = append(f.restores, [2]string{snapshotDir, dstDir})
	return nil
}

type fakeTenantRuntimeRolloutDocker struct {
	byID        map[string]*cpDocker.RuntimeContainerInfo
	byName      map[string]*cpDocker.RuntimeContainerInfo
	createQueue []fakeTenantRuntimeCreateResult
	createCalls []fakeTenantRuntimeCreateCall
	stopCalls   []string
	startCalls  []string
	removeCalls []string
	renameCalls []fakeTenantRuntimeRenameCall
	health      map[string][]bool
}

type fakeTenantRuntimeCreateResult struct {
	info *cpDocker.RuntimeContainerInfo
	err  error
}

type fakeTenantRuntimeCreateCall struct {
	tenantID      string
	tenantDataDir string
}

type fakeTenantRuntimeRenameCall struct {
	containerIDOrName string
	newName           string
}

func newFakeTenantRuntimeRolloutDocker() *fakeTenantRuntimeRolloutDocker {
	return &fakeTenantRuntimeRolloutDocker{
		byID:   make(map[string]*cpDocker.RuntimeContainerInfo),
		byName: make(map[string]*cpDocker.RuntimeContainerInfo),
		health: make(map[string][]bool),
	}
}

func (f *fakeTenantRuntimeRolloutDocker) addContainer(info *cpDocker.RuntimeContainerInfo) {
	copy := *info
	f.byID[copy.ID] = &copy
	f.byName[copy.Name] = &copy
}

func (f *fakeTenantRuntimeRolloutDocker) queueCreate(info *cpDocker.RuntimeContainerInfo, err error) {
	f.createQueue = append(f.createQueue, fakeTenantRuntimeCreateResult{info: info, err: err})
}

func (f *fakeTenantRuntimeRolloutDocker) CreateAndStart(_ context.Context, tenantID, tenantDataDir string) (string, error) {
	f.createCalls = append(f.createCalls, fakeTenantRuntimeCreateCall{tenantID: tenantID, tenantDataDir: tenantDataDir})
	if len(f.createQueue) == 0 {
		return "", fmt.Errorf("unexpected CreateAndStart call")
	}
	next := f.createQueue[0]
	f.createQueue = f.createQueue[1:]
	if next.err != nil {
		return "", next.err
	}
	f.addContainer(next.info)
	return next.info.ID, nil
}

func (f *fakeTenantRuntimeRolloutDocker) HealthCheck(_ context.Context, containerID string) (bool, error) {
	if sequence, ok := f.health[containerID]; ok {
		if len(sequence) == 0 {
			return false, nil
		}
		next := sequence[0]
		f.health[containerID] = sequence[1:]
		return next, nil
	}
	info, ok := f.byID[containerID]
	if !ok {
		return false, errdefs.NotFound(fmt.Errorf("missing container %s", containerID))
	}
	return info.Running, nil
}

func (f *fakeTenantRuntimeRolloutDocker) Inspect(_ context.Context, containerIDOrName string) (*cpDocker.RuntimeContainerInfo, error) {
	if info, ok := f.byID[containerIDOrName]; ok {
		copy := *info
		return &copy, nil
	}
	if info, ok := f.byName[containerIDOrName]; ok {
		copy := *info
		return &copy, nil
	}
	return nil, errdefs.NotFound(fmt.Errorf("missing container %s", containerIDOrName))
}

func (f *fakeTenantRuntimeRolloutDocker) Remove(_ context.Context, containerID string) error {
	f.removeCalls = append(f.removeCalls, containerID)
	info, ok := f.byID[containerID]
	if !ok {
		if infoByName, okByName := f.byName[containerID]; okByName {
			info = infoByName
			ok = true
		}
	}
	if !ok {
		return errdefs.NotFound(fmt.Errorf("missing container %s", containerID))
	}
	delete(f.byID, info.ID)
	delete(f.byName, info.Name)
	return nil
}

func (f *fakeTenantRuntimeRolloutDocker) Rename(_ context.Context, containerIDOrName, newName string) error {
	f.renameCalls = append(f.renameCalls, fakeTenantRuntimeRenameCall{containerIDOrName: containerIDOrName, newName: newName})
	info, err := f.Inspect(context.Background(), containerIDOrName)
	if err != nil {
		return err
	}
	stored := f.byID[info.ID]
	delete(f.byName, stored.Name)
	stored.Name = newName
	f.byName[newName] = stored
	return nil
}

func (f *fakeTenantRuntimeRolloutDocker) Start(_ context.Context, containerID string) error {
	f.startCalls = append(f.startCalls, containerID)
	info, err := f.Inspect(context.Background(), containerID)
	if err != nil {
		return err
	}
	f.byID[info.ID].Running = true
	return nil
}

func (f *fakeTenantRuntimeRolloutDocker) Stop(_ context.Context, containerID string) error {
	f.stopCalls = append(f.stopCalls, containerID)
	info, err := f.Inspect(context.Background(), containerID)
	if err != nil {
		return err
	}
	f.byID[info.ID].Running = false
	return nil
}

type fakeTenantRuntimeRolloutClock struct {
	now time.Time
}

func newFakeTenantRuntimeRolloutClock() *fakeTenantRuntimeRolloutClock {
	return &fakeTenantRuntimeRolloutClock{now: time.Unix(1_700_000_000, 0).UTC()}
}

func (c *fakeTenantRuntimeRolloutClock) Now() time.Time {
	return c.now
}

func (c *fakeTenantRuntimeRolloutClock) Sleep(duration time.Duration) {
	c.now = c.now.Add(duration)
}
