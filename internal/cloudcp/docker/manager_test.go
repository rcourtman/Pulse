package docker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/moby/moby/api/types/container"
	mobynetwork "github.com/moby/moby/api/types/network"
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

	env := tenantEnv("t-example", "cloud.pulserelay.pro", "pubkey-123", []string{"172.18.0.0/16", "127.0.0.1/32"}, TenantReportBrandConfig{})
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

func TestCanonicalTenantRuntimeRoutingLowercasesHostedAddressing(t *testing.T) {
	t.Parallel()

	got := CanonicalTenantRuntimeRouting("T-AbCd123", "Cloud.PulseRelay.Pro")
	if got.Host != "t-abcd123.cloud.pulserelay.pro" {
		t.Fatalf("Host = %q, want %q", got.Host, "t-abcd123.cloud.pulserelay.pro")
	}
	if got.PublicURL != "https://t-abcd123.cloud.pulserelay.pro" {
		t.Fatalf("PublicURL = %q, want %q", got.PublicURL, "https://t-abcd123.cloud.pulserelay.pro")
	}
}

func TestTraefikLabelsUseCanonicalLowercaseHost(t *testing.T) {
	t.Parallel()

	labels := TraefikLabels("T-AbCd123", "Cloud.PulseRelay.Pro", 7655)
	got := labels["traefik.http.routers.pulse-T-AbCd123.rule"]
	if got != "Host(`t-abcd123.cloud.pulserelay.pro`)" {
		t.Fatalf("router rule = %q, want %q", got, "Host(`t-abcd123.cloud.pulserelay.pro`)")
	}
}

func TestTraefikLabelsPinTenantRuntimeToIsolatedNetwork(t *testing.T) {
	t.Parallel()

	labels := TraefikLabels("t-acme", "msp.example.com", 7655, "pulse-provider-msp-tenant-t-acme")
	if got := labels["traefik.docker.network"]; got != "pulse-provider-msp-tenant-t-acme" {
		t.Fatalf("traefik.docker.network = %q, want isolated tenant network", got)
	}
}

func TestTenantNetworkNameIsDerivedPerTenant(t *testing.T) {
	t.Parallel()

	mgr := &Manager{cfg: ManagerConfig{
		Network:               "pulse-provider-msp",
		IsolateTenantNetworks: true,
		TenantNetworkPrefix:   "pulse-provider-msp-tenant",
	}}

	gotA := mgr.tenantNetworkName("T-Acme")
	gotB := mgr.tenantNetworkName("T-Beta")
	if gotA != "pulse-provider-msp-tenant-t-acme" {
		t.Fatalf("tenantNetworkName(T-Acme) = %q", gotA)
	}
	if gotB != "pulse-provider-msp-tenant-t-beta" {
		t.Fatalf("tenantNetworkName(T-Beta) = %q", gotB)
	}
	if gotA == gotB {
		t.Fatalf("tenant networks must be distinct, got %q", gotA)
	}
}

func TestHealthCheckPrefersProviderMSPIsolatedTenantNetwork(t *testing.T) {
	t.Parallel()

	mgr := &Manager{cfg: ManagerConfig{
		Network:               "pulse-provider-msp",
		IsolateTenantNetworks: true,
		TenantNetworkPrefix:   "pulse-provider-msp-tenant",
	}}
	inspect := container.InspectResponse{
		Config: &container.Config{
			Labels: map[string]string{"pulse.tenant.id": "t-acme"},
		},
		NetworkSettings: &container.NetworkSettings{
			Networks: map[string]*mobynetwork.EndpointSettings{
				"pulse-provider-msp":               {},
				"pulse-provider-msp-tenant-t-acme": {},
			},
		},
	}

	got := mgr.healthCheckNetworkCandidates(inspect)
	if len(got) < 2 {
		t.Fatalf("healthCheckNetworkCandidates = %v, want tenant and ingress networks", got)
	}
	if got[0] != "pulse-provider-msp-tenant-t-acme" {
		t.Fatalf("first health network = %q, want isolated tenant network; all=%v", got[0], got)
	}
}

func TestTenantEnvLowercasesPublicURLHost(t *testing.T) {
	t.Parallel()

	env := tenantEnv("T-AbCd123", "Cloud.PulseRelay.Pro", "", nil, TenantReportBrandConfig{})
	want := "PULSE_PUBLIC_URL=https://t-abcd123.cloud.pulserelay.pro"
	for _, item := range env {
		if item == want {
			return
		}
	}
	t.Fatalf("tenantEnv() missing %q in %v", want, env)
}

func TestTenantEnvOmitsPublicURLWithoutTenantContext(t *testing.T) {
	t.Parallel()

	env := tenantEnv("", "", "", nil, TenantReportBrandConfig{})
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

func TestTenantEnvIncludesProviderReportBrandDefaults(t *testing.T) {
	t.Parallel()

	env := tenantEnv("t-acme", "msp.example.com", "", nil, TenantReportBrandConfig{
		DisplayName: "Acme Managed IT",
		LogoBase64:  "iVBORw0KGgo=",
		LogoFormat:  "png",
	})
	want := map[string]bool{
		"PULSE_REPORT_PROVIDER_BRAND_DISPLAY_NAME=Acme Managed IT": true,
		"PULSE_REPORT_PROVIDER_BRAND_LOGO_BASE64=iVBORw0KGgo=":     true,
		"PULSE_REPORT_PROVIDER_BRAND_LOGO_FORMAT=png":              true,
	}
	for _, item := range env {
		delete(want, item)
	}
	if len(want) > 0 {
		t.Fatalf("tenantEnv() missing report brand env items: %v; env=%v", want, env)
	}
}

func TestTenantRuntimeLogConfigBoundsJSONLogs(t *testing.T) {
	t.Parallel()

	got := tenantRuntimeLogConfig("", 0)
	if got.Type != "json-file" {
		t.Fatalf("LogConfig.Type = %q, want json-file", got.Type)
	}
	if got.Config["max-size"] != defaultTenantLogMaxSize {
		t.Fatalf("max-size = %q, want %q", got.Config["max-size"], defaultTenantLogMaxSize)
	}
	if got.Config["max-file"] != "3" {
		t.Fatalf("max-file = %q, want 3", got.Config["max-file"])
	}

	custom := tenantRuntimeLogConfig("25m", 4)
	if custom.Config["max-size"] != "25m" {
		t.Fatalf("custom max-size = %q, want 25m", custom.Config["max-size"])
	}
	if custom.Config["max-file"] != "4" {
		t.Fatalf("custom max-file = %q, want 4", custom.Config["max-file"])
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

func TestCheckRuntimePrerequisitesReportsMissingDockerDaemon(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///tmp/pulse-missing-docker-preflight.sock")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")

	mgr, err := NewManager(ManagerConfig{
		Image:      "pulse:test",
		Network:    "pulse-provider-msp",
		BaseDomain: "msp.example.com",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	report, err := mgr.CheckRuntimePrerequisites(context.Background(), RuntimePrerequisiteOptions{PullImage: false})
	if err != nil {
		t.Fatalf("CheckRuntimePrerequisites: %v", err)
	}
	if report == nil {
		t.Fatal("report is nil")
	}
	if report.OK {
		t.Fatalf("report.OK = true, want false")
	}
	if report.DockerReachable {
		t.Fatal("DockerReachable = true, want false")
	}
	if report.NetworkName != "pulse-provider-msp" {
		t.Fatalf("NetworkName = %q, want pulse-provider-msp", report.NetworkName)
	}
	if report.ImageRef != "pulse:test" {
		t.Fatalf("ImageRef = %q, want pulse:test", report.ImageRef)
	}
	if got := strings.Join(report.Failures, "; "); !strings.Contains(got, "ping docker daemon") {
		t.Fatalf("failures = %q, want docker daemon ping failure", got)
	}
}
