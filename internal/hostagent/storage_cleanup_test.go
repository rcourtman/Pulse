package hostagent

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

func TestScanAPTPackageCacheCountsOnlyRegularDebArchives(t *testing.T) {
	dir := t.TempDir()
	partial := filepath.Join(dir, "partial")
	if err := os.MkdirAll(partial, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "one.deb"), []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(partial, "two.DEB"), []byte("1234567"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lock"), []byte("not reclaimable"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "one.deb"), filepath.Join(dir, "alias.deb")); err != nil {
		t.Fatal(err)
	}

	snapshot, err := scanAPTPackageCacheAt(dir)
	if err != nil {
		t.Fatalf("scan cache: %v", err)
	}
	if snapshot.ReclaimableBytes != 12 {
		t.Fatalf("reclaimable bytes = %d, want 12", snapshot.ReclaimableBytes)
	}
	if !strings.HasPrefix(snapshot.Fingerprint, "sha256:") || len(snapshot.Fingerprint) != 71 {
		t.Fatalf("unexpected fingerprint %q", snapshot.Fingerprint)
	}
	again, err := scanAPTPackageCacheAt(dir)
	if err != nil || again.Fingerprint != snapshot.Fingerprint {
		t.Fatalf("cache fingerprint is not deterministic: first=%q second=%q err=%v", snapshot.Fingerprint, again.Fingerprint, err)
	}
}

func TestStorageCleanupManagerApplyUsesClosedAPTCatalogAndVerifiesBytes(t *testing.T) {
	before := agentexec.HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: "sha256:" + strings.Repeat("a", 64), ReclaimableBytes: 400, CheckedAt: time.Now().UTC()}
	after := agentexec.HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: "sha256:" + strings.Repeat("b", 64), ReclaimableBytes: 40, CheckedAt: time.Now().UTC()}
	snapshots := []agentexec.HostStorageCleanupSnapshot{before, after}
	var calls [][]string
	manager := newStorageCleanupManager("linux")
	manager.lookPath = func(string) (string, error) { return "/usr/bin/apt-get", nil }
	manager.scan = func() (agentexec.HostStorageCleanupSnapshot, error) {
		snapshot := snapshots[0]
		snapshots = snapshots[1:]
		return snapshot, nil
	}
	manager.run = func(_ context.Context, env []string, name string, args ...string) packageUpdateCommandResult {
		calls = append(calls, append([]string{name}, args...))
		if !reflect.DeepEqual(env, []string{"DEBIAN_FRONTEND=noninteractive"}) {
			t.Fatalf("unexpected environment: %#v", env)
		}
		return packageUpdateCommandResult{}
	}

	result := manager.Apply(context.Background(), agentexec.HostStorageCleanupPayload{
		RequestID:           "cleanup-1",
		Operation:           agentexec.HostStorageCleanupOperationPackageCache,
		ExpectedFingerprint: before.Fingerprint,
	})
	if !result.Success || result.Verification != agentexec.HostStorageCleanupVerificationVerified || result.ReclaimedBytes != 360 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !reflect.DeepEqual(calls, [][]string{{"apt-get", "clean"}}) {
		t.Fatalf("unexpected command catalog: %#v", calls)
	}
}

func TestStorageCleanupManagerRefusesFingerprintDriftBeforeMutation(t *testing.T) {
	manager := newStorageCleanupManager("linux")
	manager.lookPath = func(string) (string, error) { return "/usr/bin/apt-get", nil }
	manager.scan = func() (agentexec.HostStorageCleanupSnapshot, error) {
		return agentexec.HostStorageCleanupSnapshot{Fingerprint: "sha256:" + strings.Repeat("b", 64), ReclaimableBytes: 400}, nil
	}
	manager.run = func(context.Context, []string, string, ...string) packageUpdateCommandResult {
		t.Fatal("cleanup command must not run after fingerprint drift")
		return packageUpdateCommandResult{}
	}

	result := manager.Apply(context.Background(), agentexec.HostStorageCleanupPayload{
		RequestID:           "cleanup-drift",
		Operation:           agentexec.HostStorageCleanupOperationPackageCache,
		ExpectedFingerprint: "sha256:" + strings.Repeat("a", 64),
	})
	if result.Success || result.Verification != agentexec.HostStorageCleanupVerificationInconclusive || !strings.Contains(result.Error, "replan") {
		t.Fatalf("unexpected drift result: %#v", result)
	}
}
