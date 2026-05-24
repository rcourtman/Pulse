package platformsupport

import "testing"

func TestManifestGeneratedSourceMetadataIsStable(t *testing.T) {
	if ManifestSourcePath != "docs/release-control/v6/internal/PLATFORM_SUPPORT_MANIFEST.json" {
		t.Fatalf("manifest source path = %q", ManifestSourcePath)
	}
	if ManifestSchemaVersion != 2 {
		t.Fatalf("manifest schema version = %d, want 2", ManifestSchemaVersion)
	}
	if len(ManifestSourceSHA256) != 64 {
		t.Fatalf("manifest source sha length = %d, want 64", len(ManifestSourceSHA256))
	}
}

func TestAgentHostProfileResolverUsesManifestTokens(t *testing.T) {
	profile, ok := AgentHostProfileForIdentity(" Unraid ")
	if !ok {
		t.Fatal("expected Unraid identity token to resolve")
	}
	if profile.ID != "unraid" {
		t.Fatalf("profile id = %q, want unraid", profile.ID)
	}
	if profile.RuntimePlatform != "linux" {
		t.Fatalf("runtime platform = %q, want linux", profile.RuntimePlatform)
	}
	if profile, ok := AgentHostProfileForIdentity("unraid-os"); !ok || profile.ID != "unraid" {
		t.Fatalf("expected raw os-release Unraid identity token to resolve, got %+v ok=%v", profile, ok)
	}

	profile.HostIdentityTokens[0] = "mutated"
	profile, ok = AgentHostProfileForIdentity("unraid")
	if !ok {
		t.Fatal("expected resolver data to be immutable across callers")
	}
	if profile.HostIdentityTokens[0] != "unraid" {
		t.Fatalf("identity tokens mutated globally: %+v", profile.HostIdentityTokens)
	}
}

func TestNormalizeRuntimePlatformForAgentHostProfile(t *testing.T) {
	if got := NormalizeRuntimePlatformForAgentHostProfile("unraid", ""); got != "linux" {
		t.Fatalf("empty reported platform = %q, want linux", got)
	}
	if got := NormalizeRuntimePlatformForAgentHostProfile("unraid", "unraid"); got != "linux" {
		t.Fatalf("legacy reported platform = %q, want linux", got)
	}
	if got := NormalizeRuntimePlatformForAgentHostProfile("unraid", "linux"); got != "linux" {
		t.Fatalf("canonical reported platform = %q, want linux", got)
	}
	if got := NormalizeRuntimePlatformForAgentHostProfile("unraid", "slackware"); got != "linux" {
		t.Fatalf("profiled distro platform = %q, want linux", got)
	}
	if got := NormalizeRuntimePlatformForAgentHostProfile("unknown", "unraid"); got != "unraid" {
		t.Fatalf("unknown host profile platform = %q, want original value", got)
	}
}
