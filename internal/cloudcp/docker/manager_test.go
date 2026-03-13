package docker

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestTenantImmutableOwnershipPaths(t *testing.T) {
	t.Parallel()

	got := tenantImmutableOwnershipPaths()
	want := []string{
		"/etc/pulse/billing.json",
		"/etc/pulse/secrets/handoff.key",
		"/etc/pulse/.cloud_handoff_key",
	}
	if len(got) != len(want) {
		t.Fatalf("len(paths) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paths[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTenantMountsKeepImmutableFilesReadOnly(t *testing.T) {
	t.Parallel()

	tenantDataDir := filepath.Join("/tmp", "tenant-data")
	mounts := tenantMounts(tenantDataDir)

	if len(mounts) != 4 {
		t.Fatalf("len(mounts) = %d, want 4", len(mounts))
	}

	if mounts[0].Target != "/etc/pulse" {
		t.Fatalf("mounts[0].Target = %q, want %q", mounts[0].Target, "/etc/pulse")
	}
	if mounts[0].ReadOnly {
		t.Fatalf("mounts[0].ReadOnly = true, want false")
	}

	checkMount := func(index int, wantTarget, wantSource string) {
		t.Helper()
		if mounts[index].Target != wantTarget {
			t.Fatalf("mounts[%d].Target = %q, want %q", index, mounts[index].Target, wantTarget)
		}
		if mounts[index].Source != wantSource {
			t.Fatalf("mounts[%d].Source = %q, want %q", index, mounts[index].Source, wantSource)
		}
		if !mounts[index].ReadOnly {
			t.Fatalf("mounts[%d].ReadOnly = false, want true", index)
		}
	}

	checkMount(1, "/etc/pulse/billing.json", filepath.Join(tenantDataDir, "billing.json"))
	checkMount(2, "/etc/pulse/secrets/handoff.key", filepath.Join(tenantDataDir, "secrets", "handoff.key"))
	checkMount(3, "/etc/pulse/.cloud_handoff_key", filepath.Join(tenantDataDir, ".cloud_handoff_key"))
}

func TestTenantEnvIncludesImmutableOwnershipContract(t *testing.T) {
	t.Parallel()

	env := tenantEnv()
	want := map[string]bool{
		"PULSE_DATA_DIR=/etc/pulse": true,
		"PULSE_HOSTED_MODE=true":    true,
		"PUID=1000":                 true,
		"PGID=1000":                 true,
		immutableOwnershipPathsEnv + "=/etc/pulse/billing.json:/etc/pulse/secrets/handoff.key:/etc/pulse/.cloud_handoff_key": true,
	}
	if len(env) != len(want) {
		t.Fatalf("len(env) = %d, want %d", len(env), len(want))
	}
	for _, item := range env {
		if !want[item] {
			t.Fatalf("unexpected env item %q", item)
		}
	}
}

func TestPrepareImmutableMountSourcesAlignsOwnershipAndPermissions(t *testing.T) {
	t.Parallel()

	tenantDataDir := t.TempDir()
	secretsDir := filepath.Join(tenantDataDir, "secrets")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatalf("mkdir secrets: %v", err)
	}

	paths := []string{
		filepath.Join(tenantDataDir, "billing.json"),
		filepath.Join(secretsDir, "handoff.key"),
		filepath.Join(tenantDataDir, ".cloud_handoff_key"),
	}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("secret"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	uid := os.Getuid()
	gid := os.Getgid()
	if err := prepareImmutableMountSources(tenantDataDir, uid, gid); err != nil {
		t.Fatalf("prepareImmutableMountSources: %v", err)
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s perms = %o, want %o", path, info.Mode().Perm(), 0o600)
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatalf("%s stat type %T, want *syscall.Stat_t", path, info.Sys())
		}
		if int(stat.Uid) != uid {
			t.Fatalf("%s uid = %d, want %d", path, stat.Uid, uid)
		}
		if int(stat.Gid) != gid {
			t.Fatalf("%s gid = %d, want %d", path, stat.Gid, gid)
		}
	}
}
