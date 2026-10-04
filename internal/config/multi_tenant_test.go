package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMultiTenantPersistence_InvalidOrgIDsRejected(t *testing.T) {
	baseDir := t.TempDir()
	mtp := NewMultiTenantPersistence(baseDir)

	invalidIDs := []string{
		"",
		".",
		"..",
		"../bad",
		"bad/..",
		"bad/../evil",
		"bad org",
		"bad\torg",
		"bad\norg",
		"bad\\org",
		"bad:org",
		strings.Repeat("a", 65),
	}

	for _, orgID := range invalidIDs {
		if _, err := mtp.GetPersistence(orgID); err == nil {
			t.Fatalf("expected error for orgID %q", orgID)
		}
		if mtp.OrgExists(orgID) {
			t.Fatalf("OrgExists should be false for orgID %q", orgID)
		}
	}

	if _, err := os.Stat(filepath.Join(baseDir, "orgs")); err == nil {
		t.Fatalf("unexpected orgs directory created for invalid org IDs")
	}
}

func TestMultiTenantPersistence_OrgIDLengthBoundaries(t *testing.T) {
	baseDir := t.TempDir()
	mtp := NewMultiTenantPersistence(baseDir)

	maxLenID := strings.Repeat("a", 64)
	if _, err := mtp.GetPersistence(maxLenID); err != nil {
		t.Fatalf("expected max length org ID to be accepted: %v", err)
	}

	if _, err := mtp.GetPersistence(strings.Repeat("b", 65)); err == nil {
		t.Fatalf("expected org ID longer than 64 chars to be rejected")
	}
}

func TestMultiTenantPersistence_GetPersistence_CreatesOrgDir(t *testing.T) {
	baseDir := t.TempDir()
	mtp := NewMultiTenantPersistence(baseDir)

	if _, err := mtp.GetPersistence("acme"); err != nil {
		t.Fatalf("GetPersistence(acme) failed: %v", err)
	}

	orgDir := filepath.Join(baseDir, "orgs", "acme")
	info, err := os.Stat(orgDir)
	if err != nil {
		t.Fatalf("expected org dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected org dir to be a directory")
	}
}

func TestMultiTenantPersistence_CanonicalizesBaseDataDir(t *testing.T) {
	root := t.TempDir()
	rawBaseDir := filepath.Join(root, "tenants", "..", "tenants")

	mtp := NewMultiTenantPersistence("  " + rawBaseDir + "  ")

	expectedBaseDir := filepath.Clean(rawBaseDir)
	if mtp.BaseDataDir() != expectedBaseDir {
		t.Fatalf("BaseDataDir() = %q, want %q", mtp.BaseDataDir(), expectedBaseDir)
	}

	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("GetPersistence(default) failed: %v", err)
	}
	if _, err := mtp.GetPersistence("acme"); err != nil {
		t.Fatalf("GetPersistence(acme) failed: %v", err)
	}

	defaultInfo, err := os.Stat(expectedBaseDir)
	if err != nil {
		t.Fatalf("expected canonical base dir to exist: %v", err)
	}
	if !defaultInfo.IsDir() {
		t.Fatalf("expected canonical base dir to be a directory")
	}

	orgDir := filepath.Join(expectedBaseDir, "orgs", "acme")
	orgInfo, err := os.Stat(orgDir)
	if err != nil {
		t.Fatalf("expected canonical org dir to exist: %v", err)
	}
	if !orgInfo.IsDir() {
		t.Fatalf("expected canonical org dir to be a directory")
	}
}

func TestResolveTenantOrgDirRejectsBlankBaseDir(t *testing.T) {
	originalDefaultDataDir := defaultDataDir
	defaultDataDir = " \t "
	t.Cleanup(func() { defaultDataDir = originalDefaultDataDir })
	t.Setenv("PULSE_DATA_DIR", " \t ")

	if _, err := resolveTenantOrgDir(" \t ", "default"); err == nil {
		t.Fatal("expected blank base dir to be rejected")
	}
}

func TestMultiTenantPersistence_ListOrganizationIDsUsesStorageKeysOnly(t *testing.T) {
	root := t.TempDir()
	mtp := NewMultiTenantPersistence(root)
	for orgID, body := range map[string]string{
		"default": `{"id":"metadata-redirect"}`,
		"alpha":   `{broken`,
		"zeta":    `{}`,
	} {
		persistence, err := mtp.GetPersistence(orgID)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(persistence.GetConfigDir(), "org.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	orgsDir := filepath.Join(root, "orgs")
	for _, invalid := range []string{"bad name", "bad:org"} {
		if err := os.Mkdir(filepath.Join(orgsDir, invalid), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(orgsDir, "stray-file"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(orgsDir, "linked-tenant")); err != nil {
		t.Fatal(err)
	}
	got, err := mtp.ListOrganizationIDs()
	if err != nil || !reflect.DeepEqual(got, []string{"alpha", "default", "zeta"}) {
		t.Fatalf("storage IDs = %v, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(orgsDir, "metadata-redirect")); !os.IsNotExist(err) {
		t.Fatal("enumeration created a tenant from a metadata ID")
	}
	// Metadata-dependent consumers, including account/authentication flows,
	// retain strict decoding: the new storage-key API is not authorization.
	if orgs, err := mtp.ListOrganizations(); err == nil || orgs != nil {
		t.Fatal("metadata list must still fail closed on malformed org.json")
	}
	body, err := os.ReadFile(filepath.Join(orgsDir, "alpha", "org.json"))
	if err != nil || string(body) != "{broken" {
		t.Fatal("enumeration repaired or overwrote metadata")
	}
}

func TestMultiTenantPersistence_ListOrganizationIDsMissingDirectoryAndFailure(t *testing.T) {
	t.Run("missing_directory_has_default_without_writes", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "absent")
		mtp := NewMultiTenantPersistence(root)
		got, err := mtp.ListOrganizationIDs()
		if err != nil || !reflect.DeepEqual(got, []string{"default"}) {
			t.Fatalf("missing directory IDs = %v, %v", got, err)
		}
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatal("ID enumeration created persistence or metadata")
		}
	})
	t.Run("directory_failure_returns_no_partial_inventory", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "orgs"), []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := NewMultiTenantPersistence(root).ListOrganizationIDs()
		if err == nil || got != nil {
			t.Fatalf("broken organization directory returned partial inventory: %v, %v", got, err)
		}
	})
}
