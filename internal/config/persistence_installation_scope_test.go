package config

import "testing"

func TestConfigPersistenceSharedInstallationDataDir_DefaultAndTenantPaths(t *testing.T) {
	baseDir := t.TempDir()
	mtp := NewMultiTenantPersistence(baseDir)

	defaultPersistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("GetPersistence(default): %v", err)
	}
	if got := defaultPersistence.SharedInstallationDataDir(); got != mtp.BaseDataDir() {
		t.Fatalf("default SharedInstallationDataDir() = %q, want %q", got, mtp.BaseDataDir())
	}

	tenantPersistence, err := mtp.GetPersistence("acme")
	if err != nil {
		t.Fatalf("GetPersistence(acme): %v", err)
	}
	if got := tenantPersistence.SharedInstallationDataDir(); got != mtp.BaseDataDir() {
		t.Fatalf("tenant SharedInstallationDataDir() = %q, want %q", got, mtp.BaseDataDir())
	}
}
