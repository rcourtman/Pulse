package cloudcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/lifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

// These fixtures use ordinary sentinel files, never a database, real Docker
// socket, tenant credentials or deployed workload.
type runtimeTransactionFixture struct {
	t       *testing.T
	service *tenantRuntimeRolloutService
	docker  *transactionDocker
	reg     *transactionRegistry
	sync    *transactionSynchronizer
	dataDir string
	events  []string
}

type transactionDocker struct {
	*fakeTenantRuntimeRolloutDocker
	f               *runtimeTransactionFixture
	faults          map[string]error
	ignoreStop      bool
	unknownStop     bool
	stopped         bool
	renameThenError bool
	createError     bool
	lostCreateID    bool
	afterCreate     func()
}

func (d *transactionDocker) call(ctx context.Context, key string) error {
	d.f.events = append(d.f.events, key)
	if err := ctx.Err(); err != nil {
		return err
	}
	return d.faults[key]
}
func (d *transactionDocker) Stop(ctx context.Context, id string) error {
	if err := d.call(ctx, "stop:"+id); err != nil {
		return err
	}
	if id == "old" {
		d.stopped = true
		if d.ignoreStop {
			return nil
		}
	}
	return d.fakeTenantRuntimeRolloutDocker.Stop(ctx, id)
}
func (d *transactionDocker) Start(ctx context.Context, id string) error {
	if err := d.call(ctx, "start:"+id); err != nil {
		return err
	}
	return d.fakeTenantRuntimeRolloutDocker.Start(ctx, id)
}
func (d *transactionDocker) Remove(ctx context.Context, id string) error {
	if err := d.call(ctx, "remove:"+id); err != nil {
		return err
	}
	return d.fakeTenantRuntimeRolloutDocker.Remove(ctx, id)
}
func (d *transactionDocker) Rename(ctx context.Context, id, name string) error {
	key := "rename:" + id + ":" + name
	if err := d.call(ctx, key); err != nil {
		return err
	}
	if err := d.fakeTenantRuntimeRolloutDocker.Rename(ctx, id, name); err != nil {
		return err
	}
	if d.renameThenError && strings.Contains(name, ".pre-") {
		return errors.New("rename response lost after mutation")
	}
	return nil
}
func (d *transactionDocker) Inspect(ctx context.Context, id string) (*cpDocker.RuntimeContainerInfo, error) {
	if err := d.call(ctx, "inspect:"+id); err != nil {
		return nil, err
	}
	if id == "old" && d.stopped && d.unknownStop {
		return nil, errors.New("stopped state unavailable")
	}
	return d.fakeTenantRuntimeRolloutDocker.Inspect(ctx, id)
}
func (d *transactionDocker) HealthCheck(ctx context.Context, id string) (bool, error) {
	if err := d.call(ctx, "health:"+id); err != nil {
		return false, err
	}
	return d.fakeTenantRuntimeRolloutDocker.HealthCheck(ctx, id)
}
func (d *transactionDocker) CreateAndStart(ctx context.Context, tenant, path string) (string, error) {
	if err := d.call(ctx, "create"); err != nil {
		return "", err
	}
	id, err := d.fakeTenantRuntimeRolloutDocker.CreateAndStart(ctx, tenant, path)
	if err != nil {
		return id, err
	}
	if err := os.WriteFile(filepath.Join(path, "state.txt"), []byte("new-attempt"), 0o600); err != nil {
		return id, err
	}
	if d.afterCreate != nil {
		d.afterCreate()
	}
	if d.createError {
		err = errors.New("create/start response failed")
	}
	if d.lostCreateID {
		id = ""
	}
	return id, err
}

type transactionRegistry struct {
	*fakeTenantRuntimeRolloutRegistry
	f          *runtimeTransactionFixture
	attempts   int
	failUpdate int
}

func (r *transactionRegistry) Update(tenant *registry.Tenant) error {
	r.attempts++
	r.f.events = append(r.f.events, "persist:"+tenant.ContainerID)
	if r.attempts == r.failUpdate {
		return errors.New("registry update failed")
	}
	return r.fakeTenantRuntimeRolloutRegistry.Update(tenant)
}

type transactionSynchronizer struct {
	f                   *runtimeTransactionFixture
	snapshotError       bool
	restoreError        bool
	snapshots, restores int
	afterSnapshot       func()
}

func (s *transactionSynchronizer) noWriters() {
	s.f.t.Helper()
	for _, c := range s.f.docker.byID {
		if c.Running || c.Restarting || c.Paused {
			s.f.t.Fatalf("file copy with a live writer: %+v", c)
		}
	}
	_, lease, err := lifecycle.Acquire(context.Background(), filepath.Join(filepath.Dir(s.f.service.tenantsDir), "control-plane"), "t-SAFE")
	if lease != nil {
		_ = lease.Close()
	}
	if !errors.Is(err, lifecycle.ErrBusy) {
		s.f.t.Fatalf("file copy outside lifecycle exclusion: %v", err)
	}
}
func (s *transactionSynchronizer) Snapshot(ctx context.Context, src, dst string) error {
	s.noWriters()
	s.snapshots++
	s.f.events = append(s.f.events, "snapshot")
	if s.snapshotError {
		return errors.New("snapshot failed")
	}
	if err := (filesystemTenantRuntimeSynchronizer{}).Snapshot(ctx, src, dst); err != nil {
		return err
	}
	if s.afterSnapshot != nil {
		s.afterSnapshot()
	}
	return nil
}
func (s *transactionSynchronizer) Restore(ctx context.Context, src, dst string) error {
	s.noWriters()
	s.restores++
	s.f.events = append(s.f.events, "restore")
	if s.restoreError {
		if err := os.WriteFile(filepath.Join(dst, "state.txt"), []byte("partial-restore"), 0o600); err != nil {
			return err
		}
		return errors.New("partial restore failed")
	}
	return (filesystemTenantRuntimeSynchronizer{}).Restore(ctx, src, dst)
}

func newRuntimeTransactionFixture(t *testing.T, missing bool) *runtimeTransactionFixture {
	t.Helper()
	f := &runtimeTransactionFixture{t: t}
	f.reg = &transactionRegistry{fakeTenantRuntimeRolloutRegistry: &fakeTenantRuntimeRolloutRegistry{tenant: &registry.Tenant{ID: "t-SAFE", State: registry.TenantStateActive, ContainerID: "old", CurrentImageDigest: "sha256:old", DesiredImageDigest: "sha256:old"}}, f: f}
	f.docker = &transactionDocker{fakeTenantRuntimeRolloutDocker: newFakeTenantRuntimeRolloutDocker(), f: f, faults: map[string]error{}}
	routing := f.docker.DesiredRuntimeRouting("t-SAFE")
	if !missing {
		f.docker.addContainer(&cpDocker.RuntimeContainerInfo{ID: "old", Name: "pulse-t-SAFE", ImageRef: "runtime:old", ImageID: "sha256:old", Running: true, RouteHost: routing.Host, PublicURL: routing.PublicURL})
	}
	f.docker.queueCreate(&cpDocker.RuntimeContainerInfo{ID: "new", Name: "pulse-t-SAFE", ImageRef: "runtime:new", ImageID: "sha256:new", Running: true, RouteHost: routing.Host, PublicURL: routing.PublicURL}, nil)
	f.sync = &transactionSynchronizer{f: f}
	f.service = newTestTenantRuntimeRolloutService(t, f.reg, f.docker, f.sync, newFakeTenantRuntimeRolloutClock())
	f.dataDir = filepath.Join(f.service.tenantsDir, "t-SAFE")
	if err := os.MkdirAll(f.dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.dataDir, "state.txt"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *runtimeTransactionFixture) rollout(ctx context.Context, prune bool) (*TenantRuntimeRolloutResult, error) {
	return f.service.Rollout(ctx, TenantRuntimeRolloutOptions{TenantID: "t-SAFE", Image: "runtime:new", RunID: "fixture", PrunePrevious: prune})
}
func (f *runtimeTransactionFixture) assertState(path, want string) {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(path, "state.txt"))
	if err != nil || string(data) != want {
		f.t.Fatalf("state %s = %q, %v; want %q", path, data, err, want)
	}
}
func (f *runtimeTransactionFixture) assertHold(want bool) {
	f.t.Helper()
	_, lease, err := lifecycle.Acquire(context.Background(), filepath.Join(filepath.Dir(f.service.tenantsDir), "control-plane"), "t-SAFE")
	if lease != nil {
		_ = lease.Close()
	}
	if want && !errors.Is(err, lifecycle.ErrRecoveryRequired) {
		f.t.Fatalf("want durable hold, got %v", err)
	}
	if !want && err != nil {
		f.t.Fatalf("unexpected lifecycle hold: %v", err)
	}
}

func TestTenantRuntimeTransactionStoppedCaptureAndCommit(t *testing.T) {
	f := newRuntimeTransactionFixture(t, false)
	result, err := f.rollout(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.ActiveContainerID != "new" || result.PreviousContainerID != "old" || result.BackupContainerName != "pulse-t-SAFE.pre-fixture" {
		t.Fatalf("untruthful result: %+v", result)
	}
	if f.docker.byID["old"].Running || !f.docker.byID["new"].Running || f.reg.tenant.ContainerID != "new" {
		t.Fatal("wrong committed runtime")
	}
	f.assertState(f.dataDir, "new-attempt")
	f.assertState(filepath.Join(filepath.Dir(f.service.tenantsDir), "backups", "rollout", "fixture", "t-SAFE"), "original")
	order := strings.Join(f.events, ",")
	for _, pair := range [][2]string{{"stop:old", "snapshot"}, {"snapshot", "create"}, {"health:new", "persist:new"}} {
		if strings.Index(order, pair[0]) < 0 || strings.Index(order, pair[0]) >= strings.Index(order, pair[1]) {
			t.Fatalf("unsafe ordering %v: %s", pair, order)
		}
	}
	f.assertHold(false)
}

func TestTenantRuntimeTransactionCompensatesFailures(t *testing.T) {
	for _, name := range []string{"snapshot", "snapshot-collision", "rename", "ambiguous-rename", "create-id", "create-lost-id", "health", "persist", "canceled-create"} {
		t.Run(name, func(t *testing.T) {
			f := newRuntimeTransactionFixture(t, false)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			wantRestore := true
			switch name {
			case "snapshot":
				f.sync.snapshotError = true
				wantRestore = false
			case "snapshot-collision":
				path := filepath.Join(filepath.Dir(f.service.tenantsDir), "backups", "rollout", "fixture", "t-SAFE")
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "state.txt"), []byte("older-recovery"), 0o600); err != nil {
					t.Fatal(err)
				}
				wantRestore = false
			case "rename":
				f.docker.faults["rename:old:pulse-t-SAFE.pre-fixture"] = errors.New("rename failed")
				wantRestore = false
			case "ambiguous-rename":
				f.docker.renameThenError = true
				wantRestore = false
			case "create-id":
				f.docker.createError = true
			case "create-lost-id":
				f.docker.createError = true
				f.docker.lostCreateID = true
			case "health":
				f.docker.faults["health:new"] = errors.New("unhealthy replacement")
			case "persist":
				f.reg.failUpdate = 2
			case "canceled-create":
				f.docker.afterCreate = cancel
				f.docker.createError = true
			}
			if _, err := f.rollout(ctx, false); err == nil || !strings.Contains(err.Error(), "rollback completed") {
				t.Fatalf("want truthful compensated error, got %v; events %v", err, f.events)
			}
			if !f.docker.byID["old"].Running || f.docker.byName["pulse-t-SAFE"].ID != "old" {
				t.Fatal("original was not restored")
			}
			if f.reg.tenant.ContainerID != "old" || f.reg.tenant.CurrentImageDigest != "sha256:old" || f.reg.tenant.DesiredImageDigest != "sha256:old" {
				t.Fatalf("rollback kept new registry state: %+v", f.reg.tenant)
			}
			f.assertState(f.dataDir, "original")
			if (f.sync.restores == 1) != wantRestore {
				t.Fatalf("restores=%d, want restore=%t", f.sync.restores, wantRestore)
			}
			if wantRestore {
				if f.docker.byID["new"] != nil {
					t.Fatal("replacement survived rollback")
				}
				order := strings.Join(f.events, ",")
				for _, pair := range [][2]string{{"stop:new", "remove:new"}, {"remove:new", "restore"}, {"restore", "start:old"}} {
					if strings.Index(order, pair[0]) < 0 || strings.Index(order, pair[0]) >= strings.Index(order, pair[1]) {
						t.Fatalf("unsafe rollback order %v: %s", pair, order)
					}
				}
				retained, _ := filepath.Glob(filepath.Join(f.service.tenantsDir, ".t-SAFE.pre-restore-*"))
				if len(retained) != 1 {
					t.Fatalf("post-attempt recovery trees=%v", retained)
				}
				f.assertState(retained[0], "new-attempt")
			}
			if name == "snapshot-collision" {
				f.assertState(filepath.Join(filepath.Dir(f.service.tenantsDir), "backups", "rollout", "fixture", "t-SAFE"), "older-recovery")
			}
			f.assertHold(false)
		})
	}
}

func TestTenantRuntimeTransactionStopsAtFailedRollbackPrerequisite(t *testing.T) {
	for _, name := range []string{"stop-not-quiescent", "stop-unknown", "new-stop", "remove", "restore-partial", "old-reactivated", "rollback-rename", "rollback-start", "rollback-health"} {
		t.Run(name, func(t *testing.T) {
			f := newRuntimeTransactionFixture(t, false)
			f.docker.faults["health:new"] = errors.New("new health failed")
			wantRestore := false
			switch name {
			case "stop-not-quiescent":
				f.docker.ignoreStop = true
			case "stop-unknown":
				f.docker.unknownStop = true
			case "new-stop":
				f.docker.faults["stop:new"] = errors.New("new stop failed")
			case "remove":
				f.docker.faults["remove:new"] = errors.New("new remove failed")
			case "restore-partial":
				f.sync.restoreError = true
				wantRestore = true
			case "old-reactivated":
				f.docker.afterCreate = func() { f.docker.byID["old"].Running = true }
			case "rollback-rename":
				f.docker.faults["rename:old:pulse-t-SAFE"] = errors.New("rename back failed")
				wantRestore = true
			case "rollback-start":
				f.docker.faults["start:old"] = errors.New("old start failed")
				wantRestore = true
			case "rollback-health":
				f.docker.faults["health:old"] = errors.New("old health failed")
				wantRestore = true
			}
			if _, err := f.rollout(context.Background(), false); err == nil || !strings.Contains(err.Error(), "lifecycle hold retained") {
				t.Fatalf("want retained failure, got %v", err)
			}
			f.assertHold(true)
			if (f.sync.restores == 1) != wantRestore {
				t.Fatalf("restore after failed prerequisite: %d, events %v", f.sync.restores, f.events)
			}
			if name == "restore-partial" || name == "remove" || name == "new-stop" || name == "old-reactivated" || strings.HasPrefix(name, "stop-") || name == "rollback-rename" {
				for _, event := range f.events {
					if event == "start:old" {
						t.Fatalf("started original after unsafe prerequisite: %v", f.events)
					}
				}
			}
			before := len(f.events)
			if _, err := f.rollout(context.Background(), false); !errors.Is(err, lifecycle.ErrRecoveryRequired) || len(f.events) != before {
				t.Fatalf("unfinished transaction replayed: %v", err)
			}
			if name == "rollback-health" && f.reg.tenant.HealthCheckOK {
				t.Fatal("unhealthy rollback falsely marked healthy")
			}
		})
	}
}

func TestTenantRuntimeTransactionMissingRuntimeAndCommittedPruneFailure(t *testing.T) {
	t.Run("missing-runtime-failed-first-boot", func(t *testing.T) {
		f := newRuntimeTransactionFixture(t, true)
		f.docker.createError = true
		if _, err := f.rollout(context.Background(), false); err == nil {
			t.Fatal("expected create failure")
		}
		f.assertState(f.dataDir, "original")
		f.assertHold(false)
		if f.sync.snapshots != 1 || f.sync.restores != 1 || len(f.docker.startCalls) != 0 || f.docker.byID["new"] != nil || f.reg.tenant.HealthCheckOK {
			t.Fatalf("unsafe missing-runtime compensation: %v", f.events)
		}
	})
	t.Run("prune-failed-after-commit", func(t *testing.T) {
		f := newRuntimeTransactionFixture(t, false)
		f.docker.faults["remove:old"] = errors.New("prune failed")
		result, err := f.rollout(context.Background(), true)
		if err == nil || !strings.Contains(err.Error(), "rollout committed to new") || result == nil || result.ActiveContainerID != "new" || f.reg.tenant.ContainerID != "new" {
			t.Fatalf("prune failure hid committed state: %+v %v", result, err)
		}
		f.assertHold(false)
		if f.sync.restores != 0 || f.docker.byID["old"].Running {
			t.Fatal("prune failure rolled back committed runtime")
		}
	})
}

func TestFilesystemTenantRuntimeRecoveryPreservesDestinationOnCopyFailure(t *testing.T) {
	for _, name := range []string{"missing", "canceled", "unsupported", "overlap", "root-symlink"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "snapshot")
			dst := filepath.Join(root, "tenant")
			for _, path := range []string{src, dst} {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "state.txt"), []byte(path), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			ctx := context.Background()
			switch name {
			case "missing":
				src = filepath.Join(root, "absent")
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "unsupported":
				if err := syscall.Mkfifo(filepath.Join(src, "fifo"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "overlap":
				src = dst
			case "root-symlink":
				link := filepath.Join(root, "link")
				if err := os.Symlink(dst, link); err != nil {
					t.Fatal(err)
				}
				src = link
			}
			if err := (filesystemTenantRuntimeSynchronizer{}).Restore(ctx, src, dst); err == nil {
				t.Fatal("unsafe restore accepted")
			}
			data, err := os.ReadFile(filepath.Join(dst, "state.txt"))
			if err != nil || string(data) != dst {
				t.Fatalf("failed copy damaged destination: %q %v", data, err)
			}
		})
	}
}

func TestTenantRuntimeTransactionRejectsInactiveTenantsAndUnsafeRunPaths(t *testing.T) {
	for _, state := range []registry.TenantState{registry.TenantStateDeleted, registry.TenantStateDeleting, registry.TenantStateSuspended, registry.TenantStateCanceled, "unknown"} {
		t.Run(string(state), func(t *testing.T) {
			f := newRuntimeTransactionFixture(t, false)
			f.reg.tenant.State = state
			if _, err := f.rollout(context.Background(), false); err == nil {
				t.Fatal("inactive runtime started")
			}
			if len(f.events) != 0 {
				t.Fatalf("inactive runtime touched: %v", f.events)
			}
		})
	}
	for _, raw := range []string{".", "..", "../outside"} {
		got := sanitizeTenantRuntimeRolloutRunID(raw, time.Unix(1, 0))
		if got == "." || got == ".." || filepath.Base(got) != got {
			t.Fatal(fmt.Sprintf("unsafe run path %q", got))
		}
	}
}

func TestTenantRuntimeTransactionDoesNotLeaveFailedRecoveryGreen(t *testing.T) {
	f := newRuntimeTransactionFixture(t, false)
	previousCheck := f.service.now().Add(-time.Hour)
	observedStop := f.service.now()
	f.reg.tenant.HealthCheckOK = true
	f.reg.tenant.LastHealthCheck = &previousCheck
	f.sync.restoreError = true
	f.docker.faults["health:new"] = errors.New("new runtime unhealthy")
	f.sync.afterSnapshot = func() {
		current, err := f.service.registry.Get("t-SAFE")
		if err != nil {
			t.Fatal(err)
		}
		if current.HealthCheckOK || current.LastHealthCheck == nil || !current.LastHealthCheck.Equal(observedStop) {
			t.Fatalf("stopped runtime retained green or misdated its observed stop: %+v", current)
		}
	}
	if _, err := f.rollout(context.Background(), false); err == nil {
		t.Fatal("expected partial restore failure")
	}
	current, err := f.service.registry.Get("t-SAFE")
	if err != nil {
		t.Fatal(err)
	}
	if current.HealthCheckOK || current.ContainerID != "old" || current.LastHealthCheck == nil || !current.LastHealthCheck.Equal(observedStop) {
		t.Fatalf("incomplete recovery falsely healthy: %+v", current)
	}
	f.assertHold(true)
}

func TestFilesystemTenantRuntimeRestoreRetainsBothCompleteTreesOnTransitionFailure(t *testing.T) {
	for _, failure := range []string{"retain", "install", "install-and-undo"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "snapshot")
			dst := filepath.Join(root, "tenant")
			for _, entry := range []struct{ path, data string }{{src, "snapshot"}, {dst, "post-attempt"}} {
				if err := os.Mkdir(entry.path, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(entry.path, "state.txt"), []byte(entry.data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			syncer := filesystemTenantRuntimeSynchronizer{renameTree: func(from, to string) error {
				calls++
				if failure == "retain" && calls == 1 || failure != "retain" && calls == 2 || failure == "install-and-undo" && calls == 3 {
					return errors.New("inert rename failure")
				}
				return os.Rename(from, to)
			}}
			if err := syncer.Restore(context.Background(), src, dst); err == nil {
				t.Fatal("expected transition failure")
			}
			data, err := os.ReadFile(filepath.Join(src, "state.txt"))
			if err != nil || string(data) != "snapshot" {
				t.Fatalf("original snapshot lost: %q %v", data, err)
			}
			stages, _ := filepath.Glob(filepath.Join(root, ".tenant.restore-*"))
			if len(stages) != 1 {
				t.Fatalf("complete staged recovery copy lost: %v", stages)
			}
			data, err = os.ReadFile(filepath.Join(stages[0], "state.txt"))
			if err != nil || string(data) != "snapshot" {
				t.Fatalf("staged recovery lost: %q %v", data, err)
			}
			original := dst
			if failure == "install-and-undo" {
				retained, _ := filepath.Glob(filepath.Join(root, ".tenant.pre-restore-*"))
				if len(retained) != 1 {
					t.Fatalf("original recovery tree lost: %v", retained)
				}
				original = retained[0]
			}
			data, err = os.ReadFile(filepath.Join(original, "state.txt"))
			if err != nil || string(data) != "post-attempt" {
				t.Fatalf("original destination lost: %q %v", data, err)
			}
		})
	}
}

func (r *transactionRegistry) UpdateRuntimeState(t *registry.Tenant) error {
	current, err := r.Get(t.ID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("tenant missing")
	}
	current.ContainerID = t.ContainerID
	current.CurrentImageDigest = t.CurrentImageDigest
	current.DesiredImageDigest = t.DesiredImageDigest
	current.LastHealthCheck = t.LastHealthCheck
	current.HealthCheckOK = t.HealthCheckOK
	return r.Update(current)
}

func TestTenantRuntimeTransactionPreservesConcurrentWithdrawal(t *testing.T) {
	for _, healthy := range []bool{false, true} {
		t.Run(fmt.Sprint(healthy), func(t *testing.T) {
			f := newRuntimeTransactionFixture(t, false)
			f.docker.afterCreate = func() { f.reg.tenant.State = registry.TenantStateCanceled }
			if !healthy {
				f.docker.faults["health:new"] = errors.New("new health failed")
			}
			_, err := f.rollout(context.Background(), false)
			if f.reg.tenant.State != registry.TenantStateCanceled {
				t.Fatal("runtime persistence recreated withdrawn lifecycle state")
			}
			if !healthy {
				if err == nil {
					t.Fatal("expected held recovery")
				}
				f.assertHold(true)
				if f.docker.byID["old"].Running {
					t.Fatal("rollback reopened old state after authority withdrawal")
				}
				for _, event := range f.events {
					if event == "start:old" {
						t.Fatal("withdrawn old state started")
					}
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				f.assertHold(false)
			}
		})
	}
}

func TestTenantRuntimeTransactionDoesNotCommitPausedOrRestartingRuntime(t *testing.T) {
	for _, name := range []string{"paused", "restarting"} {
		t.Run(name, func(t *testing.T) {
			f := newRuntimeTransactionFixture(t, false)
			f.docker.afterCreate = func() {
				if name == "paused" {
					f.docker.byID["new"].Paused = true
				} else {
					f.docker.byID["new"].Restarting = true
				}
			}
			if _, err := f.rollout(context.Background(), false); err == nil {
				t.Fatal("unavailable runtime committed as healthy")
			}
			if f.reg.tenant.ContainerID != "old" || f.reg.tenant.HealthCheckOK {
				t.Fatal("unavailable replacement persisted as healthy")
			}
			if f.docker.byID["old"].Running {
				t.Fatal("unconfirmed replacement quiescence reopened original")
			}
			f.assertHold(true)
		})
	}
}
