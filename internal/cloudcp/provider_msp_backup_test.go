package cloudcp

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestProviderMSPBackupCreateVerifyIncludesRecoveryContract(t *testing.T) {
	cfg := testProviderMSPBackupConfig(t)
	reg := seedProviderMSPBackupRegistry(t, cfg)
	if err := reg.Close(); err != nil {
		t.Fatalf("close registry: %v", err)
	}
	magicLinks, err := cpauth.NewService(cfg.ControlPlaneDir())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	magicLinks.Close()
	if err := os.MkdirAll(filepath.Join(cfg.TenantsDir(), "t-ACTIVE001"), 0o755); err != nil {
		t.Fatalf("create tenant dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.TenantsDir(), "t-ACTIVE001", "runtime.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("write tenant runtime file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ControlPlaneDir(), "operator-note.txt"), []byte("keep me"), 0o600); err != nil {
		t.Fatalf("write control-plane note: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ControlPlaneDir(), "tenants.db-wal"), []byte("stale wal sidecar"), 0o600); err != nil {
		t.Fatalf("write wal sidecar: %v", err)
	}

	archivePath := filepath.Join(t.TempDir(), "provider-msp-backup.tar.gz")
	result, err := CreateProviderMSPBackup(context.Background(), cfg, archivePath)
	if err != nil {
		t.Fatalf("CreateProviderMSPBackup: %v", err)
	}
	if result.Manifest.Version != ProviderMSPBackupManifestVersion {
		t.Fatalf("manifest version = %q", result.Manifest.Version)
	}
	if !result.Manifest.LicenseIncluded {
		t.Fatal("expected provider MSP license to be included in recovery archive")
	}
	if result.Manifest.RegistryAccountCount != 1 || result.Manifest.RegistryTenantCount != 2 || result.Manifest.RuntimeTenantCount != 1 {
		t.Fatalf("manifest counts = accounts %d registry tenants %d runtime tenants %d", result.Manifest.RegistryAccountCount, result.Manifest.RegistryTenantCount, result.Manifest.RuntimeTenantCount)
	}
	if !providerMSPBackupTestContainsString(result.Manifest.ControlPlaneDBBackups, "control-plane/tenants.db") {
		t.Fatalf("manifest db backups = %#v, want tenant registry", result.Manifest.ControlPlaneDBBackups)
	}
	if !providerMSPBackupTestContainsString(result.Manifest.ControlPlaneDBBackups, "control-plane/cp_magic_links.db") {
		t.Fatalf("manifest db backups = %#v, want magic-link db", result.Manifest.ControlPlaneDBBackups)
	}

	verify, err := VerifyProviderMSPBackup(context.Background(), archivePath)
	if err != nil {
		t.Fatalf("VerifyProviderMSPBackup: %v", err)
	}
	if !verify.HasTenantRegistryDB || !verify.HasLicenseFile {
		t.Fatalf("verify flags = registry %t license %t", verify.HasTenantRegistryDB, verify.HasLicenseFile)
	}
	if len(verify.RuntimeTenantDirs) != 1 || verify.RuntimeTenantDirs[0] != "t-ACTIVE001" {
		t.Fatalf("runtime tenant dirs = %#v", verify.RuntimeTenantDirs)
	}

	entries := readProviderMSPBackupEntries(t, archivePath)
	for _, want := range []string{
		"manifest.json",
		"control-plane/tenants.db",
		"control-plane/cp_magic_links.db",
		"control-plane/operator-note.txt",
		"tenants/t-ACTIVE001/runtime.json",
		"license/provider-msp-license.jwt",
	} {
		if _, ok := entries[want]; !ok {
			t.Fatalf("backup missing %q; entries=%#v", want, sortedEntryNames(entries))
		}
	}
	if _, ok := entries["control-plane/tenants.db-wal"]; ok {
		t.Fatalf("backup included raw sqlite WAL sidecar")
	}
	assertProviderMSPBackupRegistrySnapshotCount(t, entries["control-plane/tenants.db"], 2)
}

func TestProviderMSPBackupRejectsArchiveInsideSourceTrees(t *testing.T) {
	cfg := testProviderMSPBackupConfig(t)
	reg := seedProviderMSPBackupRegistry(t, cfg)
	if err := reg.Close(); err != nil {
		t.Fatalf("close registry: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.TenantsDir(), "t-ACTIVE001"), 0o755); err != nil {
		t.Fatalf("create tenant dir: %v", err)
	}

	_, err := CreateProviderMSPBackup(context.Background(), cfg, filepath.Join(cfg.ControlPlaneDir(), "backup.tar.gz"))
	if err == nil {
		t.Fatal("expected archive inside control-plane dir to be rejected")
	}
	if !strings.Contains(err.Error(), "must not be written inside") {
		t.Fatalf("error = %v", err)
	}
}

func TestProviderMSPBackupRequiresRuntimeTenantDirs(t *testing.T) {
	cfg := testProviderMSPBackupConfig(t)
	reg := seedProviderMSPBackupRegistry(t, cfg)
	if err := reg.Close(); err != nil {
		t.Fatalf("close registry: %v", err)
	}
	if err := os.MkdirAll(cfg.TenantsDir(), 0o755); err != nil {
		t.Fatalf("create tenants dir: %v", err)
	}

	_, err := CreateProviderMSPBackup(context.Background(), cfg, filepath.Join(t.TempDir(), "backup.tar.gz"))
	if err == nil {
		t.Fatal("expected missing active tenant runtime dir to fail")
	}
	if !strings.Contains(err.Error(), "runtime tenant dir t-ACTIVE001") {
		t.Fatalf("error = %v", err)
	}
}

func TestProviderMSPBackupRestoreRecoversStateAndLicense(t *testing.T) {
	_, archivePath := createProviderMSPBackupArchiveForRestoreTest(t)
	targetDataDir := filepath.Join(t.TempDir(), "restored-data")

	result, err := RestoreProviderMSPBackup(context.Background(), ProviderMSPBackupRestoreOptions{
		ArchivePath:   archivePath,
		TargetDataDir: targetDataDir,
	})
	if err != nil {
		t.Fatalf("RestoreProviderMSPBackup: %v", err)
	}
	if result.DryRun {
		t.Fatal("restore result unexpectedly marked as dry-run")
	}
	if result.RestoredRegistryTenantCount != 2 {
		t.Fatalf("RestoredRegistryTenantCount = %d, want 2", result.RestoredRegistryTenantCount)
	}
	if len(result.RestoredRuntimeTenantIDs) != 1 || result.RestoredRuntimeTenantIDs[0] != "t-ACTIVE001" {
		t.Fatalf("RestoredRuntimeTenantIDs = %#v", result.RestoredRuntimeTenantIDs)
	}
	assertProviderMSPBackupRestoredTenantCount(t, filepath.Join(targetDataDir, "control-plane", "tenants.db"), 2)
	if got, err := os.ReadFile(filepath.Join(targetDataDir, "tenants", "t-ACTIVE001", "runtime.json")); err != nil {
		t.Fatalf("read restored tenant runtime file: %v", err)
	} else if string(got) != `{"ok":true}` {
		t.Fatalf("restored tenant runtime file = %q", got)
	}
	if got, err := os.ReadFile(filepath.Join(targetDataDir, "provider-msp-license.jwt")); err != nil {
		t.Fatalf("read restored license: %v", err)
	} else if string(got) != "signed-provider-msp-license" {
		t.Fatalf("restored license = %q", got)
	}
	if _, err := os.Stat(filepath.Join(targetDataDir, "tenants", "t-DELETED001")); !os.IsNotExist(err) {
		t.Fatalf("deleted tenant runtime dir should not be restored, stat err=%v", err)
	}
}

func TestProviderMSPBackupRestoreDryRunDoesNotWriteTarget(t *testing.T) {
	_, archivePath := createProviderMSPBackupArchiveForRestoreTest(t)
	targetDataDir := filepath.Join(t.TempDir(), "restore-dry-run")

	result, err := RestoreProviderMSPBackup(context.Background(), ProviderMSPBackupRestoreOptions{
		ArchivePath:   archivePath,
		TargetDataDir: targetDataDir,
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("RestoreProviderMSPBackup dry-run: %v", err)
	}
	if !result.DryRun {
		t.Fatal("restore result should be marked dry-run")
	}
	if _, err := os.Stat(filepath.Join(targetDataDir, "control-plane")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create control-plane dir, stat err=%v", err)
	}
}

func TestProviderMSPBackupRestoreRequiresReplaceForExistingState(t *testing.T) {
	_, archivePath := createProviderMSPBackupArchiveForRestoreTest(t)
	targetDataDir := filepath.Join(t.TempDir(), "restore-replace")
	stalePath := filepath.Join(targetDataDir, "control-plane", "stale.txt")
	if err := os.MkdirAll(filepath.Dir(stalePath), 0o755); err != nil {
		t.Fatalf("create stale control-plane dir: %v", err)
	}
	if err := os.WriteFile(stalePath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	_, err := RestoreProviderMSPBackup(context.Background(), ProviderMSPBackupRestoreOptions{
		ArchivePath:   archivePath,
		TargetDataDir: targetDataDir,
	})
	if err == nil {
		t.Fatal("expected restore to reject existing provider MSP state")
	}
	if !strings.Contains(err.Error(), "replace enabled") {
		t.Fatalf("error = %v", err)
	}

	result, err := RestoreProviderMSPBackup(context.Background(), ProviderMSPBackupRestoreOptions{
		ArchivePath:       archivePath,
		TargetDataDir:     targetDataDir,
		ReplaceExisting:   true,
		LicenseOutputPath: filepath.Join(targetDataDir, "provider-msp-license.jwt"),
	})
	if err != nil {
		t.Fatalf("RestoreProviderMSPBackup replace: %v", err)
	}
	if !result.ReplaceExisting {
		t.Fatal("restore result should record replace mode")
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("replace should remove stale file, stat err=%v", err)
	}
	assertProviderMSPBackupRestoredTenantCount(t, filepath.Join(targetDataDir, "control-plane", "tenants.db"), 2)
}

// restoreProviderMSPArchiveFile writes archive-supplied bytes straight to disk,
// and the tar header's declared size cannot bound that write: the header comes
// from the archive, so a gzip bomb, a sparse entry declaring a huge logical
// size, or a corrupt stream can all deliver far more than it claims. These
// tests pin the bound to the bytes actually copied.
func TestRestoreProviderMSPArchiveFileRejectsDeclaredOversize(t *testing.T) {
	destPath := filepath.Join(t.TempDir(), "oversized.db")
	budget := &providerMSPRestoreBudget{entryCap: 64, totalCap: 1 << 20, remaining: 1 << 20}
	header := &tar.Header{Name: "control-plane/oversized.db", Typeflag: tar.TypeReg, Size: 65, Mode: 0o600}

	err := restoreProviderMSPArchiveFile(strings.NewReader("ignored"), header, destPath, budget)
	if err == nil {
		t.Fatal("expected an oversized declared entry size to be rejected")
	}
	if !strings.Contains(err.Error(), "per-entry limit") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Fatalf("rejected entry should not touch the disk, stat err=%v", statErr)
	}
}

func TestRestoreProviderMSPArchiveFileBoundsUnderdeclaredEntry(t *testing.T) {
	destPath := filepath.Join(t.TempDir(), "bomb.db")
	budget := &providerMSPRestoreBudget{entryCap: 64, totalCap: 1 << 20, remaining: 1 << 20}
	// The header understates the payload the way a bomb does: 8 bytes declared,
	// 4 KiB delivered. Trusting the declaration writes all 4 KiB.
	header := &tar.Header{Name: "control-plane/bomb.db", Typeflag: tar.TypeReg, Size: 8, Mode: 0o600}

	err := restoreProviderMSPArchiveFile(strings.NewReader(strings.Repeat("x", 4096)), header, destPath, budget)
	if err == nil {
		t.Fatal("expected an entry over the per-entry limit to fail the restore")
	}
	if !strings.Contains(err.Error(), "per-entry limit") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Fatalf("failed entry should not leave a partial file, stat err=%v", statErr)
	}
	if budget.remaining != 1<<20 {
		t.Fatalf("failed entry should not consume budget, remaining = %d", budget.remaining)
	}
}

func TestRestoreProviderMSPArchiveFileBoundsTotalRestore(t *testing.T) {
	dir := t.TempDir()
	budget := &providerMSPRestoreBudget{entryCap: 1 << 20, totalCap: 24, remaining: 24}
	header := func(name string) *tar.Header {
		return &tar.Header{Name: name, Typeflag: tar.TypeReg, Size: 16, Mode: 0o600}
	}

	firstPath := filepath.Join(dir, "first.db")
	if err := restoreProviderMSPArchiveFile(strings.NewReader(strings.Repeat("a", 16)), header("control-plane/first.db"), firstPath, budget); err != nil {
		t.Fatalf("first entry is within budget: %v", err)
	}
	if budget.remaining != 8 {
		t.Fatalf("remaining after first entry = %d, want 8", budget.remaining)
	}

	// Each entry is under the per-entry cap; only the running total catches it.
	secondPath := filepath.Join(dir, "second.db")
	err := restoreProviderMSPArchiveFile(strings.NewReader(strings.Repeat("b", 16)), header("control-plane/second.db"), secondPath, budget)
	if err == nil {
		t.Fatal("expected the cumulative restore limit to fail the second entry")
	}
	if !strings.Contains(err.Error(), "total restore limit") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(secondPath); !os.IsNotExist(statErr) {
		t.Fatalf("failed entry should not leave a partial file, stat err=%v", statErr)
	}
	if got, readErr := os.ReadFile(firstPath); readErr != nil || string(got) != strings.Repeat("a", 16) {
		t.Fatalf("first entry = %q, err=%v", got, readErr)
	}
}

// A restore that fails partway has already removed whatever it replaced, so
// leaving the half-written tree behind hands the operator a control plane that
// looks bootable and forces the retry to use replace.
func TestProviderMSPBackupRestoreRollsBackPartialRestore(t *testing.T) {
	archivePath := writeProviderMSPBackupArchiveFailingMidExtract(t)
	targetDataDir := filepath.Join(t.TempDir(), "restore-rollback")

	_, err := RestoreProviderMSPBackup(context.Background(), ProviderMSPBackupRestoreOptions{
		ArchivePath:   archivePath,
		TargetDataDir: targetDataDir,
	})
	if err == nil {
		t.Fatal("expected the unsupported entry to fail the restore")
	}
	if !strings.Contains(err.Error(), "unsupported backup restore entry type") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error should report the rollback, got %v", err)
	}
	for _, dir := range []string{providerMSPBackupControlPlaneDir, providerMSPBackupTenantsDir} {
		if _, statErr := os.Stat(filepath.Join(targetDataDir, dir)); !os.IsNotExist(statErr) {
			t.Fatalf("failed restore should not leave %s behind, stat err=%v", dir, statErr)
		}
	}
}

// writeProviderMSPBackupArchiveFailingMidExtract builds an archive that passes
// verification and then fails during extraction, after a real file has already
// landed in the target.
func writeProviderMSPBackupArchiveFailingMidExtract(t *testing.T) string {
	t.Helper()
	manifestBytes, err := json.Marshal(ProviderMSPBackupManifest{
		Version:               ProviderMSPBackupManifestVersion,
		CreatedAt:             time.Now().UTC(),
		ControlPlaneMode:      string(ControlPlaneModeProviderHostedMSP),
		Environment:           "production",
		BaseURL:               "https://msp.example.com",
		PlanVersion:           "msp_growth",
		PlanSource:            ProviderMSPPlanSourceLicenseFile,
		ControlPlaneDir:       providerMSPBackupControlPlaneDir,
		TenantsDir:            providerMSPBackupTenantsDir,
		ControlPlaneDBBackups: []string{providerMSPBackupControlPlaneDir + "/tenants.db"},
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	archivePath := filepath.Join(t.TempDir(), "partial-restore.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)

	writeHeader := func(header *tar.Header) {
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("write header %s: %v", header.Name, err)
		}
	}
	writeFile := func(name string, content []byte) {
		writeHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o600, Size: int64(len(content))})
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("write entry %s: %v", name, err)
		}
	}

	writeFile(providerMSPBackupManifestName, manifestBytes)
	writeHeader(&tar.Header{Name: providerMSPBackupControlPlaneDir + "/", Typeflag: tar.TypeDir, Mode: 0o755})
	writeFile(providerMSPBackupControlPlaneDir+"/tenants.db", []byte("restored-before-the-failure"))
	writeHeader(&tar.Header{Name: providerMSPBackupTenantsDir + "/", Typeflag: tar.TypeDir, Mode: 0o755})
	writeHeader(&tar.Header{Name: providerMSPBackupTenantsDir + "/t-ACTIVE001/", Typeflag: tar.TypeDir, Mode: 0o755})
	// Restore only handles regular files and directories, so this aborts the
	// extraction with tenants.db already written.
	writeHeader(&tar.Header{Name: providerMSPBackupControlPlaneDir + "/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0o777})

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return archivePath
}

func TestProviderMSPBackupManifestAcceptsMSPControlPlaneModes(t *testing.T) {
	base := ProviderMSPBackupManifest{
		Version:         ProviderMSPBackupManifestVersion,
		PlanVersion:     "msp_growth",
		PlanSource:      ProviderMSPPlanSourceLicenseFile,
		ControlPlaneDir: providerMSPBackupControlPlaneDir,
		TenantsDir:      providerMSPBackupTenantsDir,
	}

	for _, mode := range []ControlPlaneMode{ControlPlaneModeProviderHostedMSP, ControlPlaneModePulseHostedMSP} {
		t.Run(string(mode), func(t *testing.T) {
			manifest := base
			manifest.ControlPlaneMode = string(mode)
			if err := validateProviderMSPBackupManifest(manifest); err != nil {
				t.Fatalf("validateProviderMSPBackupManifest: %v", err)
			}
		})
	}
}

func testProviderMSPBackupConfig(t *testing.T) *CPConfig {
	t.Helper()
	root := t.TempDir()
	licenseFile := filepath.Join(root, "provider-msp-license.jwt")
	if err := os.WriteFile(licenseFile, []byte("signed-provider-msp-license"), 0o600); err != nil {
		t.Fatalf("write license: %v", err)
	}
	return &CPConfig{
		DataDir:                 filepath.Join(root, "data"),
		Environment:             "production",
		ControlPlaneMode:        ControlPlaneModeProviderHostedMSP,
		BaseURL:                 "https://msp.example.com",
		ProviderMSPPlanVersion:  "msp_growth",
		ProviderMSPPlanSource:   ProviderMSPPlanSourceLicenseFile,
		ProviderMSPLicenseFile:  licenseFile,
		ProviderMSPLicenseID:    "lic_provider_msp_backup",
		ProviderMSPLicenseEmail: "provider@example.com",
	}
}

func seedProviderMSPBackupRegistry(t *testing.T, cfg *CPConfig) *registry.TenantRegistry {
	t.Helper()
	reg, err := registry.NewTenantRegistry(cfg.ControlPlaneDir())
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	if err := reg.CreateAccount(&registry.Account{
		ID:          "acct_backup",
		Kind:        registry.AccountKindMSP,
		DisplayName: "Backup MSP",
	}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if err := reg.Create(&registry.Tenant{
		ID:          "t-ACTIVE001",
		AccountID:   "acct_backup",
		Email:       "client-a@example.com",
		DisplayName: "Client A",
		State:       registry.TenantStateActive,
	}); err != nil {
		t.Fatalf("Create active tenant: %v", err)
	}
	if err := reg.Create(&registry.Tenant{
		ID:          "t-DELETED001",
		AccountID:   "acct_backup",
		Email:       "client-old@example.com",
		DisplayName: "Deleted Client",
		State:       registry.TenantStateDeleted,
	}); err != nil {
		t.Fatalf("Create deleted tenant: %v", err)
	}
	return reg
}

func createProviderMSPBackupArchiveForRestoreTest(t *testing.T) (*CPConfig, string) {
	t.Helper()
	cfg := testProviderMSPBackupConfig(t)
	reg := seedProviderMSPBackupRegistry(t, cfg)
	if err := reg.Close(); err != nil {
		t.Fatalf("close registry: %v", err)
	}
	magicLinks, err := cpauth.NewService(cfg.ControlPlaneDir())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	magicLinks.Close()
	if err := os.MkdirAll(filepath.Join(cfg.TenantsDir(), "t-ACTIVE001"), 0o755); err != nil {
		t.Fatalf("create active tenant dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.TenantsDir(), "t-ACTIVE001", "runtime.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("write active tenant runtime file: %v", err)
	}
	archivePath := filepath.Join(t.TempDir(), "provider-msp-backup.tar.gz")
	if _, err := CreateProviderMSPBackup(context.Background(), cfg, archivePath); err != nil {
		t.Fatalf("CreateProviderMSPBackup: %v", err)
	}
	return cfg, archivePath
}

func readProviderMSPBackupEntries(t *testing.T, archivePath string) map[string][]byte {
	t.Helper()
	file, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("open gzip: %v", err)
	}
	defer gz.Close()
	entries := map[string][]byte{}
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		name := strings.TrimSuffix(header.Name, "/")
		if header.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("read %s: %v", header.Name, err)
			}
			entries[name] = content
		} else {
			entries[name] = nil
		}
	}
	return entries
}

func assertProviderMSPBackupRegistrySnapshotCount(t *testing.T, dbBytes []byte, want int) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "tenants.db")
	if err := os.WriteFile(dbPath, dbBytes, 0o600); err != nil {
		t.Fatalf("write snapshot db: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open snapshot db: %v", err)
	}
	defer db.Close()
	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tenants`).Scan(&got); err != nil {
		t.Fatalf("query snapshot db: %v", err)
	}
	if got != want {
		t.Fatalf("snapshot tenant count = %d, want %d", got, want)
	}
}

func assertProviderMSPBackupRestoredTenantCount(t *testing.T, dbPath string, want int) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open restored db: %v", err)
	}
	defer db.Close()
	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tenants`).Scan(&got); err != nil {
		t.Fatalf("query restored db: %v", err)
	}
	if got != want {
		t.Fatalf("restored tenant count = %d, want %d", got, want)
	}
}

func sortedEntryNames(entries map[string][]byte) []string {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func providerMSPBackupTestContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
