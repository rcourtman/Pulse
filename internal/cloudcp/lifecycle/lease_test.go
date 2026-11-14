package lifecycle

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTenantLifecycleProcessProbe(t *testing.T) {
	dir := os.Getenv("PULSE_TEST_LIFECYCLE_DIR")
	if dir == "" {
		t.Skip("subprocess helper")
	}
	_, lease, err := Acquire(context.Background(), dir, "t-one")
	switch os.Getenv("PULSE_TEST_LIFECYCLE_EXPECT") {
	case "busy":
		if !errors.Is(err, ErrBusy) {
			t.Fatalf("want cross-process busy, got %v", err)
		}
	case "held":
		if !errors.Is(err, ErrRecoveryRequired) {
			t.Fatalf("want retained recovery hold, got %v", err)
		}
	case "free":
		if err != nil {
			t.Fatal(err)
		}
		_ = lease.Close()
	default:
		t.Fatal("missing expected result")
	}
}

func TestTenantLifecycleCrossProcessExclusionAndRetainedHold(t *testing.T) {
	dir := t.TempDir()
	probe := func(expect string) {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^TestTenantLifecycleProcessProbe$")
		cmd.Env = append(os.Environ(), "PULSE_TEST_LIFECYCLE_DIR="+dir, "PULSE_TEST_LIFECYCLE_EXPECT="+expect)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", expect, err, output)
		}
	}
	ctx, lease, err := Acquire(context.Background(), dir, "t-one")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	probe("busy")
	_, nested, err := Acquire(ctx, dir, "t-one")
	if err != nil {
		t.Fatal(err)
	}
	_ = nested.Close()
	probe("busy") // A nested lifecycle call must not release the owner.
	_, other, err := Acquire(context.Background(), dir, "t-other")
	if err != nil {
		t.Fatalf("unrelated tenant blocked: %v", err)
	}
	_ = other.Close()
	if err := lease.MarkRecovery(Recovery{TenantID: "t-one", SnapshotDir: "fixture-snapshot", Phase: "stopping"}); err != nil {
		t.Fatal(err)
	}
	if err := lease.MarkRecovery(Recovery{TenantID: "t-one", SnapshotDir: "fixture-snapshot", NewContainerID: "fixture-new", Phase: "rollback"}); err != nil {
		t.Fatal(err)
	}
	_ = lease.Close() // Simulate process exit, not transaction completion.
	probe("held")
	// Only this fixture, not a production route, reconciles its inert record.
	if err := os.Remove(filepath.Join(dir, "runtime-lifecycle", "t-one.recovery.json")); err != nil {
		t.Fatal(err)
	}
	probe("free")
	_, lease2, err := Acquire(context.Background(), dir, "t-one")
	if err != nil {
		t.Fatal(err)
	}
	defer lease2.Close()
	if err := lease2.MarkRecovery(Recovery{TenantID: "t-one", Phase: "verified"}); err != nil {
		t.Fatal(err)
	}
	if err := lease2.Complete(); err != nil {
		t.Fatal(err)
	}
	_ = lease2.Close()
	probe("free")
}

func TestTenantLifecycleFailsClosedForCancellationInvalidAndCorruptHolds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Acquire(ctx, t.TempDir(), "t-one"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, id := range []string{"", ".", "..", "../tenant", "tenant/other", "tenant\\other"} {
		if _, _, err := Acquire(context.Background(), t.TempDir(), id); err == nil {
			t.Fatalf("accepted %q", id)
		}
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "runtime-lifecycle"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "runtime-lifecycle", "t-one.recovery.json"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Acquire(context.Background(), dir, "t-one"); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("corrupt record did not hold: %v", err)
	}
}
