package docker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestTenantImmutableOwnershipPaths(t *testing.T) {
	t.Parallel()

	got := tenantImmutableOwnershipPaths()
	want := []string{
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

	if len(mounts) != 3 {
		t.Fatalf("len(mounts) = %d, want 3", len(mounts))
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

	for _, mounted := range mounts {
		if mounted.Target == "/etc/pulse/billing.json" {
			t.Fatalf("billing.json should be writable through the tenant data mount, got dedicated mount %+v", mounted)
		}
	}

	checkMount(1, "/etc/pulse/secrets/handoff.key", filepath.Join(tenantDataDir, "secrets", "handoff.key"))
	checkMount(2, "/etc/pulse/.cloud_handoff_key", filepath.Join(tenantDataDir, ".cloud_handoff_key"))
}

func TestTenantEnvIncludesImmutableOwnershipContract(t *testing.T) {
	t.Parallel()

	env := tenantEnv("t-example", "cloud.pulserelay.pro", "pubkey-123", []string{"172.18.0.0/16", "127.0.0.1/32"})
	want := map[string]bool{
		"PULSE_DATA_DIR=/etc/pulse":       true,
		"PULSE_HOSTED_MODE=true":          true,
		"PULSE_TENANT_ID=t-example":       true,
		"PULSE_MULTI_TENANT_ENABLED=true": true,
		"PUID=1000":                       true,
		"PGID=1000":                       true,
		"PULSE_PUBLIC_URL=https://t-example.cloud.pulserelay.pro":                                    true,
		"PULSE_TRIAL_ACTIVATION_PUBLIC_KEY=pubkey-123":                                               true,
		"PULSE_TRUSTED_PROXY_CIDRS=172.18.0.0/16,127.0.0.1/32":                                       true,
		immutableOwnershipPathsEnv + "=/etc/pulse/secrets/handoff.key:/etc/pulse/.cloud_handoff_key": true,
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

func TestTenantEnvOmitsPublicURLWithoutTenantContext(t *testing.T) {
	t.Parallel()

	env := tenantEnv("", "", "", nil)
	sawTenantID := false
	for _, item := range env {
		if strings.HasPrefix(item, "PULSE_PUBLIC_URL=") {
			t.Fatalf("unexpected public URL env item %q", item)
		}
		if item == "PULSE_TENANT_ID=" {
			sawTenantID = true
		}
	}
	if !sawTenantID {
		t.Fatalf("expected explicit empty tenant id env item, got %v", env)
	}
}

func TestCanonicalTrustedProxyCIDR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "172.18.0.0/16", want: "172.18.0.0/16"},
		{input: "172.18.12.34/16", want: "172.18.0.0/16"},
		{input: "127.0.0.1", want: "127.0.0.1/32"},
		{input: "2001:db8::1", want: "2001:db8::1/128"},
		{input: "not-a-cidr", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := canonicalTrustedProxyCIDR(tc.input); got != tc.want {
				t.Fatalf("canonicalTrustedProxyCIDR(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestPrepareTenantRuntimeMountSourcesAlignsOwnershipAndPermissions(t *testing.T) {
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
	if err := prepareTenantRuntimeMountSources(tenantDataDir, uid, gid); err != nil {
		t.Fatalf("prepareTenantRuntimeMountSources: %v", err)
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

func TestCreateAndStartFailsBeforeImmutablePrepWhenDockerUnavailable(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///tmp/pulse-missing-docker.sock")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")

	mgr, err := NewManager(ManagerConfig{
		Image:      "pulse:test",
		Network:    "bridge",
		BaseDomain: "cloud.example.com",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	tenantDataDir := t.TempDir()
	secretsDir := filepath.Join(tenantDataDir, "secrets")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatalf("mkdir secrets: %v", err)
	}
	for _, path := range []string{
		filepath.Join(tenantDataDir, "billing.json"),
		filepath.Join(secretsDir, "handoff.key"),
		filepath.Join(tenantDataDir, ".cloud_handoff_key"),
	} {
		if err := os.WriteFile(path, []byte("secret"), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	_, err = mgr.CreateAndStart(context.Background(), "t-unavailable", tenantDataDir)
	if err == nil {
		t.Fatal("expected docker daemon error")
	}
	if !strings.Contains(err.Error(), "ping docker daemon") {
		t.Fatalf("CreateAndStart error = %v, want ping docker daemon failure", err)
	}
	if strings.Contains(err.Error(), "prepare tenant runtime mounts") {
		t.Fatalf("CreateAndStart error = %v, want daemon failure before mount preparation", err)
	}
}
