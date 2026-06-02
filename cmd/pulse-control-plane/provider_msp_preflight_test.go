package main

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
)

func TestProviderMSPCommandExposesPreflight(t *testing.T) {
	cmd := newProviderMSPCmd()
	for _, child := range cmd.Commands() {
		if child.Name() == "preflight" {
			return
		}
	}
	t.Fatal("provider-msp preflight command is not registered")
}

func TestProviderMSPPreflightRequiresLicenseSourceByDefault(t *testing.T) {
	docker := &fakeProviderMSPPreflightDocker{
		report: &cpDocker.RuntimePrerequisiteReport{
			OK:              true,
			DockerReachable: true,
			NetworkName:     "pulse-provider-msp",
			NetworkOK:       true,
			NetworkID:       "network-test",
			ImageRef:        "pulse:test",
			ImageID:         "sha256:test",
			ImageAvailable:  true,
		},
	}
	report, err := runProviderMSPPreflightWithDependencies(
		context.Background(),
		testProviderMSPPreflightConfig(t, cloudcp.ProviderMSPPlanSourceEnvFallback),
		providerMSPPreflightOptions{},
		fakeProviderMSPPreflightDependencies(docker, &cloudcp.StorageGuardrailReport{Enabled: false, OK: true}),
	)

	if err == nil {
		t.Fatal("expected preflight to reject environment-fallback MSP plan source")
	}
	if report == nil || report.OK {
		t.Fatalf("report.OK = %v, want false", report != nil && report.OK)
	}
	if got := strings.Join(report.Failures, "; "); !strings.Contains(got, cloudcp.ProviderMSPPlanSourceLicenseFile) {
		t.Fatalf("failures = %q, want license-file source failure", got)
	}
	if !report.RegistryReady {
		t.Fatalf("registry readiness should still be checked: %#v", report)
	}
	if docker.pullImage != true {
		t.Fatalf("PullImage = %t, want true by default", docker.pullImage)
	}
}

func TestProviderMSPPreflightPassesWithLicenseDockerAndStorage(t *testing.T) {
	docker := &fakeProviderMSPPreflightDocker{
		report: &cpDocker.RuntimePrerequisiteReport{
			OK:              true,
			DockerReachable: true,
			NetworkName:     "pulse-provider-msp",
			NetworkOK:       true,
			NetworkID:       "network-test",
			ImageRef:        "pulse:test",
			ImageID:         "sha256:test",
			ImageAvailable:  true,
		},
	}
	storage := &cloudcp.StorageGuardrailReport{
		Enabled: true,
		OK:      true,
		BuildCache: cloudcp.StorageBuildCacheReport{
			OK:       true,
			MaxBytes: 1024,
		},
		Filesystems: []cloudcp.StorageFilesystemReport{{
			Name:              "data",
			Path:              "/tmp/pulse-data",
			AvailableBytes:    2048,
			MinAvailableBytes: 1,
			OK:                true,
		}},
	}

	report, err := runProviderMSPPreflightWithDependencies(
		context.Background(),
		testProviderMSPPreflightConfig(t, cloudcp.ProviderMSPPlanSourceLicenseFile),
		providerMSPPreflightOptions{},
		fakeProviderMSPPreflightDependencies(docker, storage),
	)

	if err != nil {
		t.Fatalf("runProviderMSPPreflightWithDependencies: %v", err)
	}
	if !report.OK {
		t.Fatalf("report.OK = false, failures = %v", report.Failures)
	}
	if report.WorkspaceLimit != 15 {
		t.Fatalf("WorkspaceLimit = %d, want 15", report.WorkspaceLimit)
	}
	if report.PlanSource != cloudcp.ProviderMSPPlanSourceLicenseFile {
		t.Fatalf("PlanSource = %q, want %q", report.PlanSource, cloudcp.ProviderMSPPlanSourceLicenseFile)
	}
	if !report.RegistryReady || report.Docker == nil || !report.Docker.ImageAvailable || report.Storage == nil || !report.Storage.OK {
		t.Fatalf("readiness report incomplete: %#v", report)
	}
	if docker.pullImage != true {
		t.Fatalf("PullImage = %t, want true so fresh installs prepare the runtime image", docker.pullImage)
	}
}

func TestProviderMSPPreflightSkipImagePullUsesInspectOnly(t *testing.T) {
	docker := &fakeProviderMSPPreflightDocker{
		report: &cpDocker.RuntimePrerequisiteReport{
			OK:              true,
			DockerReachable: true,
			NetworkName:     "pulse-provider-msp",
			NetworkOK:       true,
			ImageRef:        "pulse:test",
			ImageAvailable:  true,
		},
	}
	_, err := runProviderMSPPreflightWithDependencies(
		context.Background(),
		testProviderMSPPreflightConfig(t, cloudcp.ProviderMSPPlanSourceLicenseFile),
		providerMSPPreflightOptions{SkipImagePull: true},
		fakeProviderMSPPreflightDependencies(docker, &cloudcp.StorageGuardrailReport{Enabled: false, OK: true}),
	)
	if err != nil {
		t.Fatalf("runProviderMSPPreflightWithDependencies: %v", err)
	}
	if docker.pullImage {
		t.Fatal("PullImage = true, want false with --skip-image-pull")
	}
}

func testProviderMSPPreflightConfig(t *testing.T, planSource string) *cloudcp.CPConfig {
	t.Helper()
	dir := t.TempDir()
	return &cloudcp.CPConfig{
		DataDir:                         dir,
		Environment:                     "production",
		ControlPlaneMode:                cloudcp.ControlPlaneModeProviderHostedMSP,
		BaseURL:                         "https://msp.example.com",
		PulseImage:                      "pulse:test",
		DockerNetwork:                   "pulse-provider-msp",
		TenantMemoryLimit:               512 * 1024 * 1024,
		TenantCPUShares:                 256,
		TenantLogMaxSize:                "10m",
		TenantLogMaxFile:                3,
		StorageGuardrailsEnabled:        true,
		StorageRootPath:                 dir,
		StorageDataPath:                 dir,
		StorageDockerPath:               dir,
		StorageMinRootAvailableBytes:    1,
		StorageMinDataAvailableBytes:    1,
		StorageMinDockerAvailableBytes:  1,
		StorageMaxDockerBuildCacheBytes: 1024,
		ProviderMSPPlanVersion:          "msp_growth",
		ProviderMSPPlanSource:           planSource,
		ProviderMSPLicenseID:            "lic_provider_msp_test",
		ProviderMSPLicenseEmail:         "provider@example.com",
	}
}

func fakeProviderMSPPreflightDependencies(docker *fakeProviderMSPPreflightDocker, storage *cloudcp.StorageGuardrailReport) providerMSPPreflightDependencies {
	return providerMSPPreflightDependencies{
		OpenRegistry: func(*cloudcp.CPConfig) (providerMSPPreflightRegistry, error) {
			return &fakeProviderMSPPreflightRegistry{}, nil
		},
		NewDocker: func(*cloudcp.CPConfig) (providerMSPPreflightDocker, error) {
			return docker, nil
		},
		CheckStorage: func(context.Context, *cloudcp.CPConfig, cloudcp.StorageDockerUsageProvider) (*cloudcp.StorageGuardrailReport, error) {
			return storage, nil
		},
	}
}

type fakeProviderMSPPreflightRegistry struct{}

func (f *fakeProviderMSPPreflightRegistry) Ping() error  { return nil }
func (f *fakeProviderMSPPreflightRegistry) Close() error { return nil }

type fakeProviderMSPPreflightDocker struct {
	report    *cpDocker.RuntimePrerequisiteReport
	pullImage bool
}

func (f *fakeProviderMSPPreflightDocker) CheckRuntimePrerequisites(_ context.Context, opts cpDocker.RuntimePrerequisiteOptions) (*cpDocker.RuntimePrerequisiteReport, error) {
	f.pullImage = opts.PullImage
	return f.report, nil
}

func (f *fakeProviderMSPPreflightDocker) DiskUsage(context.Context) (*cpDocker.DiskUsageSnapshot, error) {
	return &cpDocker.DiskUsageSnapshot{}, nil
}

func (f *fakeProviderMSPPreflightDocker) Close() error { return nil }
