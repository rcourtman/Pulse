package agentcapabilities

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestRequiredCapabilityScopesUseAuthOrderAndDeduplicate(t *testing.T) {
	capabilities := []Capability{
		{Name: "settings_write", Scope: auth.ScopeSettingsWrite},
		{Name: "monitoring_read", Scope: auth.ScopeMonitoringRead},
		{Name: "empty_scope"},
		{Name: "monitoring_write", Scope: auth.ScopeMonitoringWrite},
		{Name: "unknown_one", Scope: "custom:alpha"},
		{Name: "duplicate_monitoring_read", Scope: " " + auth.ScopeMonitoringRead + " "},
		{Name: "ai_execute", Scope: auth.ScopeAIExecute},
		{Name: "unknown_two", Scope: "custom:beta"},
	}

	got := RequiredCapabilityScopes(capabilities)
	want := []string{
		auth.ScopeMonitoringRead,
		auth.ScopeMonitoringWrite,
		auth.ScopeSettingsWrite,
		auth.ScopeAIExecute,
		"custom:alpha",
		"custom:beta",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RequiredCapabilityScopes = %v, want %v", got, want)
	}
}

func TestNormalizeRequiredScopesUseAuthOrderAndDeduplicate(t *testing.T) {
	got := NormalizeRequiredScopes([]string{
		auth.ScopeSettingsWrite,
		auth.ScopeMonitoringRead,
		"",
		" " + auth.ScopeMonitoringRead + " ",
		"custom:alpha",
		auth.ScopeAIExecute,
		"custom:alpha",
		"custom:beta",
	})
	want := []string{
		auth.ScopeMonitoringRead,
		auth.ScopeSettingsWrite,
		auth.ScopeAIExecute,
		"custom:alpha",
		"custom:beta",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeRequiredScopes = %v, want %v", got, want)
	}
}

func TestRequiredCapabilityScopeListReflectsCanonicalManifest(t *testing.T) {
	got := RequiredCapabilityScopeList(CanonicalManifest().Capabilities)
	want := "monitoring:read, monitoring:write, settings:read, settings:write, ai:execute"
	if got != want {
		t.Fatalf("RequiredCapabilityScopeList(CanonicalManifest) = %q, want %q", got, want)
	}
}

func TestManifestRequiredScopeListPrefersManifestRequiredScopes(t *testing.T) {
	manifest := &Manifest{
		RequiredScopes: []string{
			auth.ScopeSettingsWrite,
			auth.ScopeMonitoringRead,
			auth.ScopeMonitoringRead,
			auth.ScopeAIExecute,
		},
		Capabilities: []Capability{
			{Name: "legacy_extra", Scope: auth.ScopeMonitoringWrite},
		},
	}

	got := ManifestRequiredScopeList(manifest)
	want := "monitoring:read, settings:write, ai:execute"
	if got != want {
		t.Fatalf("ManifestRequiredScopeList = %q, want %q", got, want)
	}
}

func TestManifestRequiredScopeListFallsBackToCapabilitiesForLegacyManifest(t *testing.T) {
	manifest := &Manifest{
		Capabilities: []Capability{
			{Name: "settings_write", Scope: auth.ScopeSettingsWrite},
			{Name: "monitoring_read", Scope: auth.ScopeMonitoringRead},
		},
	}

	got := ManifestRequiredScopeList(manifest)
	want := "monitoring:read, settings:write"
	if got != want {
		t.Fatalf("ManifestRequiredScopeList legacy fallback = %q, want %q", got, want)
	}
}
