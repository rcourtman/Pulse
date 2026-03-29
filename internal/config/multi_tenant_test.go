package config

import (
	"os"
	"path/filepath"
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
