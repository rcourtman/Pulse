package cloudcp

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	_ "modernc.org/sqlite"
)

const (
	ProviderMSPBackupManifestVersion = "provider-msp-backup/v1"
	providerMSPBackupManifestName    = "manifest.json"
	providerMSPBackupControlPlaneDir = "control-plane"
	providerMSPBackupTenantsDir      = "tenants"
	providerMSPBackupLicenseDir      = "license"
	providerMSPBackupLicenseName     = "provider-msp-license.jwt"
	maxProviderMSPBackupManifestSize = 1024 * 1024
)

// ProviderMSPBackupManifest is the recovery contract stored inside every
// provider-hosted MSP backup archive.
type ProviderMSPBackupManifest struct {
	Version               string         `json:"version"`
	CreatedAt             time.Time      `json:"created_at"`
	ControlPlaneMode      string         `json:"control_plane_mode"`
	Environment           string         `json:"environment"`
	BaseURL               string         `json:"base_url"`
	PlanVersion           string         `json:"plan_version"`
	PlanSource            string         `json:"plan_source"`
	LicenseID             string         `json:"license_id,omitempty"`
	LicenseEmail          string         `json:"license_email,omitempty"`
	LicenseIncluded       bool           `json:"license_included"`
	WorkspaceLimit        int            `json:"workspace_limit"`
	ControlPlaneDir       string         `json:"control_plane_dir"`
	TenantsDir            string         `json:"tenants_dir"`
	RegistryAccountCount  int            `json:"registry_account_count"`
	RegistryTenantCount   int            `json:"registry_tenant_count"`
	RuntimeTenantCount    int            `json:"runtime_tenant_count"`
	RuntimeTenantIDs      []string       `json:"runtime_tenant_ids"`
	RegistryTenantIDs     []string       `json:"registry_tenant_ids"`
	RegistryStateCounts   map[string]int `json:"registry_state_counts"`
	ControlPlaneDBBackups []string       `json:"control_plane_db_backups"`
}

// ProviderMSPBackupCreateResult describes a newly written provider MSP backup.
type ProviderMSPBackupCreateResult struct {
	ArchivePath         string
	Manifest            ProviderMSPBackupManifest
	ControlPlaneEntries int
	TenantEntries       int
	LicenseEntries      int
	BytesWritten        int64
}

// ProviderMSPBackupVerifyResult describes a verified provider MSP backup.
type ProviderMSPBackupVerifyResult struct {
	ArchivePath          string
	Manifest             ProviderMSPBackupManifest
	ControlPlaneEntries  int
	TenantEntries        int
	LicenseEntries       int
	HasTenantRegistryDB  bool
	HasLicenseFile       bool
	RuntimeTenantDirs    []string
	ControlPlaneDBFiles  []string
	VerifiedArchiveBytes int64
}

// DefaultProviderMSPBackupPath returns the compose-friendly default backup
// location under CP_DATA_DIR without placing the archive inside the source trees.
func DefaultProviderMSPBackupPath(cfg *CPConfig, now time.Time) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	name := "provider-msp-backup-" + now.UTC().Format("20060102T150405Z") + ".tar.gz"
	if cfg == nil || strings.TrimSpace(cfg.DataDir) == "" {
		return name
	}
	return filepath.Join(cfg.DataDir, "backups", "provider-msp", name)
}

// CreateProviderMSPBackup creates a recovery archive for a provider-hosted MSP
// control plane. It backs up the control-plane registry through SQLite's online
// VACUUM INTO path instead of copying live WAL files directly.
func CreateProviderMSPBackup(ctx context.Context, cfg *CPConfig, outputPath string) (*ProviderMSPBackupCreateResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("control plane config is required")
	}
	if !cfg.IsProviderHostedMSP() {
		return nil, fmt.Errorf("provider MSP backup requires CP_CONTROL_PLANE_MODE=%s", ControlPlaneModeProviderHostedMSP)
	}
	if cfg.UsesStripeBilling() {
		return nil, fmt.Errorf("provider MSP backup is unavailable for Stripe-backed control planes")
	}
	if strings.TrimSpace(outputPath) == "" {
		outputPath = DefaultProviderMSPBackupPath(cfg, time.Now().UTC())
	}

	archivePath, err := filepath.Abs(filepath.Clean(outputPath))
	if err != nil {
		return nil, fmt.Errorf("resolve backup archive path: %w", err)
	}
	controlPlaneDir, err := filepath.Abs(filepath.Clean(cfg.ControlPlaneDir()))
	if err != nil {
		return nil, fmt.Errorf("resolve control-plane dir: %w", err)
	}
	tenantsDir, err := filepath.Abs(filepath.Clean(cfg.TenantsDir()))
	if err != nil {
		return nil, fmt.Errorf("resolve tenants dir: %w", err)
	}
	if pathIsInside(archivePath, controlPlaneDir) || pathIsInside(archivePath, tenantsDir) {
		return nil, fmt.Errorf("backup archive must not be written inside %s or %s", cfg.ControlPlaneDir(), cfg.TenantsDir())
	}

	if err := requireDirectory(controlPlaneDir, "control-plane dir"); err != nil {
		return nil, err
	}
	registryDB := filepath.Join(controlPlaneDir, "tenants.db")
	if err := requireRegularFile(registryDB, "tenant registry database"); err != nil {
		return nil, err
	}

	reg, err := registry.NewTenantRegistry(controlPlaneDir)
	if err != nil {
		return nil, fmt.Errorf("open tenant registry: %w", err)
	}
	tenants, listErr := reg.List()
	accounts, accountsErr := reg.ListAccounts()
	closeErr := reg.Close()
	if listErr != nil {
		return nil, fmt.Errorf("list tenant registry rows: %w", listErr)
	}
	if accountsErr != nil {
		return nil, fmt.Errorf("list registry accounts: %w", accountsErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close tenant registry before backup: %w", closeErr)
	}

	manifest, err := buildProviderMSPBackupManifest(cfg, tenants, len(accounts))
	if err != nil {
		return nil, err
	}
	if err := requireProviderMSPRuntimeTenantDirs(tenantsDir, manifest.RuntimeTenantIDs); err != nil {
		return nil, err
	}
	licensePath, licenseIncluded, err := resolveProviderMSPBackupLicensePath(cfg)
	if err != nil {
		return nil, err
	}
	manifest.LicenseIncluded = licenseIncluded

	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return nil, fmt.Errorf("create backup archive parent: %w", err)
	}
	tempDir, err := os.MkdirTemp(filepath.Dir(archivePath), ".provider-msp-backup-*")
	if err != nil {
		return nil, fmt.Errorf("create backup staging dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tmpArchive := archivePath + ".tmp"
	_ = os.Remove(tmpArchive)
	file, err := os.OpenFile(tmpArchive, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create backup archive: %w", err)
	}
	result, err := writeProviderMSPBackupArchive(ctx, file, tempDir, controlPlaneDir, tenantsDir, licensePath, manifest)
	closeArchiveErr := file.Close()
	if err != nil {
		_ = os.Remove(tmpArchive)
		return nil, err
	}
	if closeArchiveErr != nil {
		_ = os.Remove(tmpArchive)
		return nil, fmt.Errorf("close backup archive: %w", closeArchiveErr)
	}
	if err := os.Rename(tmpArchive, archivePath); err != nil {
		_ = os.Remove(tmpArchive)
		return nil, fmt.Errorf("publish backup archive: %w", err)
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		return nil, fmt.Errorf("stat backup archive: %w", err)
	}
	result.ArchivePath = archivePath
	result.BytesWritten = info.Size()
	return result, nil
}

// VerifyProviderMSPBackup validates that a provider MSP backup contains the
// manifest, tenant registry snapshot, required tenant directories, and license
// artifact declared by the manifest.
func VerifyProviderMSPBackup(ctx context.Context, archivePath string) (*ProviderMSPBackupVerifyResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	archivePath = strings.TrimSpace(archivePath)
	if archivePath == "" {
		return nil, fmt.Errorf("backup archive path is required")
	}
	resolvedArchivePath, err := filepath.Abs(filepath.Clean(archivePath))
	if err != nil {
		return nil, fmt.Errorf("resolve backup archive path: %w", err)
	}
	file, err := os.Open(resolvedArchivePath)
	if err != nil {
		return nil, fmt.Errorf("open backup archive: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat backup archive: %w", err)
	}
	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("open backup gzip stream: %w", err)
	}
	defer gz.Close()

	result := &ProviderMSPBackupVerifyResult{
		ArchivePath:          resolvedArchivePath,
		VerifiedArchiveBytes: info.Size(),
	}
	tenantDirs := map[string]bool{}
	dbFiles := map[string]bool{}
	var manifestBytes []byte
	tr := tar.NewReader(gz)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read backup archive: %w", err)
		}
		name, err := cleanProviderMSPArchiveName(header.Name)
		if err != nil {
			return nil, err
		}
		switch {
		case name == providerMSPBackupManifestName:
			manifestBytes, err = readProviderMSPBackupManifestBytes(tr, header.Size)
			if err != nil {
				return nil, err
			}
		case name == providerMSPBackupControlPlaneDir || strings.HasPrefix(name, providerMSPBackupControlPlaneDir+"/"):
			result.ControlPlaneEntries++
			if strings.HasSuffix(name, ".db") {
				dbFiles[name] = true
			}
			if name == providerMSPBackupControlPlaneDir+"/tenants.db" {
				result.HasTenantRegistryDB = true
			}
		case name == providerMSPBackupTenantsDir || strings.HasPrefix(name, providerMSPBackupTenantsDir+"/"):
			result.TenantEntries++
			if tenantID := archiveTenantID(name); tenantID != "" {
				tenantDirs[tenantID] = true
			}
		case name == providerMSPBackupLicenseDir || strings.HasPrefix(name, providerMSPBackupLicenseDir+"/"):
			result.LicenseEntries++
			if name == providerMSPBackupLicenseDir+"/"+providerMSPBackupLicenseName {
				result.HasLicenseFile = true
			}
		default:
			return nil, fmt.Errorf("unexpected top-level backup entry %q", name)
		}
	}
	if len(manifestBytes) == 0 {
		return nil, fmt.Errorf("backup manifest is missing")
	}
	if err := json.Unmarshal(manifestBytes, &result.Manifest); err != nil {
		return nil, fmt.Errorf("parse backup manifest: %w", err)
	}
	if err := validateProviderMSPBackupManifest(result.Manifest); err != nil {
		return nil, err
	}
	if !result.HasTenantRegistryDB {
		return nil, fmt.Errorf("backup is missing control-plane tenant registry snapshot")
	}
	if result.ControlPlaneEntries == 0 {
		return nil, fmt.Errorf("backup is missing control-plane data")
	}
	if result.TenantEntries == 0 {
		return nil, fmt.Errorf("backup is missing tenants directory")
	}
	if result.Manifest.LicenseIncluded && !result.HasLicenseFile {
		return nil, fmt.Errorf("backup manifest declares included MSP license but license file is missing")
	}
	for _, dbFile := range result.Manifest.ControlPlaneDBBackups {
		if !dbFiles[dbFile] {
			return nil, fmt.Errorf("backup manifest declares control-plane DB snapshot %q but it is missing", dbFile)
		}
	}
	for _, tenantID := range result.Manifest.RuntimeTenantIDs {
		if !tenantDirs[tenantID] {
			return nil, fmt.Errorf("backup is missing runtime tenant directory %q", tenantID)
		}
	}
	for tenantID := range tenantDirs {
		result.RuntimeTenantDirs = append(result.RuntimeTenantDirs, tenantID)
	}
	sort.Strings(result.RuntimeTenantDirs)
	for name := range dbFiles {
		result.ControlPlaneDBFiles = append(result.ControlPlaneDBFiles, name)
	}
	sort.Strings(result.ControlPlaneDBFiles)
	return result, nil
}

func buildProviderMSPBackupManifest(cfg *CPConfig, tenants []*registry.Tenant, accountCount int) (ProviderMSPBackupManifest, error) {
	workspaceLimit, known := pkglicensing.WorkspaceLimitForPlan(cfg.ProviderMSPPlanVersion)
	if !known {
		return ProviderMSPBackupManifest{}, fmt.Errorf("provider MSP plan %q has no known workspace limit", cfg.ProviderMSPPlanVersion)
	}
	manifest := ProviderMSPBackupManifest{
		Version:               ProviderMSPBackupManifestVersion,
		CreatedAt:             time.Now().UTC(),
		ControlPlaneMode:      string(cfg.ControlPlaneMode),
		Environment:           strings.TrimSpace(cfg.Environment),
		BaseURL:               strings.TrimSpace(cfg.BaseURL),
		PlanVersion:           strings.TrimSpace(cfg.ProviderMSPPlanVersion),
		PlanSource:            providerMSPPlanSourceOrDefault(cfg.ProviderMSPPlanSource),
		LicenseID:             strings.TrimSpace(cfg.ProviderMSPLicenseID),
		LicenseEmail:          strings.ToLower(strings.TrimSpace(cfg.ProviderMSPLicenseEmail)),
		WorkspaceLimit:        workspaceLimit,
		ControlPlaneDir:       providerMSPBackupControlPlaneDir,
		TenantsDir:            providerMSPBackupTenantsDir,
		RegistryAccountCount:  accountCount,
		RegistryStateCounts:   map[string]int{},
		ControlPlaneDBBackups: []string{},
	}
	for _, tenant := range tenants {
		if tenant == nil {
			continue
		}
		tenantID := strings.TrimSpace(tenant.ID)
		if tenantID == "" {
			continue
		}
		manifest.RegistryTenantIDs = append(manifest.RegistryTenantIDs, tenantID)
		manifest.RegistryStateCounts[string(tenant.State)]++
		if providerMSPBackupRequiresRuntimeTenantDir(tenant.State) {
			manifest.RuntimeTenantIDs = append(manifest.RuntimeTenantIDs, tenantID)
		}
	}
	sort.Strings(manifest.RegistryTenantIDs)
	sort.Strings(manifest.RuntimeTenantIDs)
	manifest.RegistryTenantCount = len(manifest.RegistryTenantIDs)
	manifest.RuntimeTenantCount = len(manifest.RuntimeTenantIDs)
	return manifest, nil
}

func providerMSPBackupRequiresRuntimeTenantDir(state registry.TenantState) bool {
	switch state {
	case registry.TenantStateCanceled, registry.TenantStateDeleting, registry.TenantStateDeleted:
		return false
	default:
		return true
	}
}

func resolveProviderMSPBackupLicensePath(cfg *CPConfig) (string, bool, error) {
	if providerMSPPlanSourceOrDefault(cfg.ProviderMSPPlanSource) != ProviderMSPPlanSourceLicenseFile {
		return "", false, nil
	}
	licensePath := strings.TrimSpace(cfg.ProviderMSPLicenseFile)
	if licensePath == "" {
		return "", false, fmt.Errorf("provider MSP backup requires CP_PROVIDER_MSP_LICENSE_FILE when plan_source=%s", ProviderMSPPlanSourceLicenseFile)
	}
	if err := requireRegularFile(licensePath, "provider MSP license file"); err != nil {
		return "", false, err
	}
	return licensePath, true, nil
}

func requireProviderMSPRuntimeTenantDirs(tenantsDir string, tenantIDs []string) error {
	if len(tenantIDs) == 0 {
		return nil
	}
	if err := requireDirectory(tenantsDir, "tenants dir"); err != nil {
		return err
	}
	for _, tenantID := range tenantIDs {
		dir := filepath.Join(tenantsDir, tenantID)
		if err := requireDirectory(dir, "runtime tenant dir "+tenantID); err != nil {
			return err
		}
	}
	return nil
}

func writeProviderMSPBackupArchive(ctx context.Context, output io.Writer, tempDir, controlPlaneDir, tenantsDir, licensePath string, manifest ProviderMSPBackupManifest) (*ProviderMSPBackupCreateResult, error) {
	gz := gzip.NewWriter(output)
	tw := tar.NewWriter(gz)
	result := &ProviderMSPBackupCreateResult{
		Manifest: manifest,
	}
	controlPlaneEntries, dbBackups, err := addProviderMSPBackupDir(ctx, tw, tempDir, controlPlaneDir, providerMSPBackupControlPlaneDir, true, manifest.CreatedAt)
	if err != nil {
		return nil, err
	}
	result.ControlPlaneEntries = controlPlaneEntries
	manifest.ControlPlaneDBBackups = dbBackups
	tenantEntries, _, err := addProviderMSPBackupDir(ctx, tw, tempDir, tenantsDir, providerMSPBackupTenantsDir, false, manifest.CreatedAt)
	if err != nil {
		return nil, err
	}
	result.TenantEntries = tenantEntries
	if strings.TrimSpace(licensePath) != "" {
		if err := writeProviderMSPBackupRootDir(tw, providerMSPBackupLicenseDir, manifest.CreatedAt); err != nil {
			return nil, err
		}
		result.LicenseEntries++
		if err := addProviderMSPBackupFile(tw, licensePath, providerMSPBackupLicenseDir+"/"+providerMSPBackupLicenseName, 0); err != nil {
			return nil, err
		}
		result.LicenseEntries++
	}
	result.Manifest = manifest
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode backup manifest: %w", err)
	}
	manifestBytes = append(manifestBytes, '\n')
	if err := writeProviderMSPBackupBytes(tw, providerMSPBackupManifestName, manifestBytes, 0o600, manifest.CreatedAt); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close backup tar stream: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("close backup gzip stream: %w", err)
	}
	return result, nil
}

func addProviderMSPBackupDir(ctx context.Context, tw *tar.Writer, tempDir, sourceDir, archiveRoot string, snapshotSQLite bool, createdAt time.Time) (int, []string, error) {
	if strings.TrimSpace(sourceDir) == "" {
		return 0, nil, fmt.Errorf("source dir is required for %s", archiveRoot)
	}
	entries := 0
	dbBackups := []string{}
	if err := writeProviderMSPBackupRootDir(tw, archiveRoot, createdAt); err != nil {
		return 0, nil, err
	}
	entries++
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return entries, dbBackups, nil
	} else if err != nil {
		return 0, nil, fmt.Errorf("stat %s: %w", sourceDir, err)
	}
	err := filepath.WalkDir(sourceDir, func(filePath string, entry os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return fmt.Errorf("walk %s: %w", filePath, walkErr)
		}
		rel, err := filepath.Rel(sourceDir, filePath)
		if err != nil {
			return fmt.Errorf("resolve backup relative path: %w", err)
		}
		if rel == "." {
			return nil
		}
		archiveName := filepath.ToSlash(filepath.Join(archiveRoot, rel))
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat backup source %s: %w", filePath, err)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if err := addProviderMSPBackupSymlink(tw, filePath, archiveName, info); err != nil {
				return err
			}
			entries++
			return nil
		}
		if entry.IsDir() {
			if err := addProviderMSPBackupHeader(tw, info, archiveName, ""); err != nil {
				return err
			}
			entries++
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported backup source file type at %s", filePath)
		}
		if snapshotSQLite && isProviderMSPSQLiteSidecar(rel) {
			return nil
		}
		if snapshotSQLite && strings.EqualFold(filepath.Ext(filePath), ".db") {
			snapshotPath := filepath.Join(tempDir, archiveRoot, rel)
			if err := snapshotSQLiteDatabase(filePath, snapshotPath); err != nil {
				return err
			}
			if err := addProviderMSPBackupFile(tw, snapshotPath, archiveName, info.Mode().Perm()); err != nil {
				return err
			}
			dbBackups = append(dbBackups, archiveName)
			entries++
			return nil
		}
		if err := addProviderMSPBackupFile(tw, filePath, archiveName, 0); err != nil {
			return err
		}
		entries++
		return nil
	})
	sort.Strings(dbBackups)
	if err != nil {
		return 0, nil, err
	}
	return entries, dbBackups, nil
}

func snapshotSQLiteDatabase(sourcePath, destPath string) error {
	if err := requireRegularFile(sourcePath, "sqlite database"); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create sqlite snapshot parent: %w", err)
	}
	_ = os.Remove(destPath)
	dsn := sourcePath + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"foreign_keys(ON)",
		},
	}.Encode()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open sqlite database for backup: %w", err)
	}
	defer db.Close()
	if _, err := db.Exec("VACUUM INTO ?", destPath); err != nil {
		return fmt.Errorf("snapshot sqlite database %s: %w", filepath.Base(sourcePath), err)
	}
	return nil
}

func addProviderMSPBackupFile(tw *tar.Writer, sourcePath, archiveName string, modeOverride os.FileMode) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat backup file %s: %w", sourcePath, err)
	}
	if modeOverride != 0 {
		info = providerMSPFileInfoWithMode{FileInfo: info, mode: modeOverride}
	}
	file, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open backup file %s: %w", sourcePath, err)
	}
	defer file.Close()
	if err := addProviderMSPBackupHeader(tw, info, archiveName, ""); err != nil {
		return err
	}
	if _, err := io.Copy(tw, file); err != nil {
		return fmt.Errorf("write backup file %s: %w", archiveName, err)
	}
	return nil
}

func addProviderMSPBackupSymlink(tw *tar.Writer, sourcePath, archiveName string, info os.FileInfo) error {
	target, err := os.Readlink(sourcePath)
	if err != nil {
		return fmt.Errorf("read backup symlink %s: %w", sourcePath, err)
	}
	return addProviderMSPBackupHeader(tw, info, archiveName, target)
}

func addProviderMSPBackupHeader(tw *tar.Writer, info os.FileInfo, archiveName, linkTarget string) error {
	header, err := tar.FileInfoHeader(info, linkTarget)
	if err != nil {
		return fmt.Errorf("create backup tar header %s: %w", archiveName, err)
	}
	header.Name = archiveName
	header.Uid = 0
	header.Gid = 0
	header.Uname = ""
	header.Gname = ""
	if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
		header.Name += "/"
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write backup tar header %s: %w", archiveName, err)
	}
	return nil
}

func writeProviderMSPBackupRootDir(tw *tar.Writer, archiveName string, modTime time.Time) error {
	name := strings.TrimSuffix(archiveName, "/") + "/"
	header := &tar.Header{
		Name:     name,
		Mode:     0o755,
		Typeflag: tar.TypeDir,
		ModTime:  modTime,
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write backup tar header %s: %w", archiveName, err)
	}
	return nil
}

func writeProviderMSPBackupBytes(tw *tar.Writer, archiveName string, content []byte, mode int64, modTime time.Time) error {
	header := &tar.Header{
		Name:     archiveName,
		Mode:     mode,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
		ModTime:  modTime,
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write backup tar header %s: %w", archiveName, err)
	}
	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("write backup entry %s: %w", archiveName, err)
	}
	return nil
}

func validateProviderMSPBackupManifest(manifest ProviderMSPBackupManifest) error {
	if manifest.Version != ProviderMSPBackupManifestVersion {
		return fmt.Errorf("unsupported provider MSP backup manifest version %q", manifest.Version)
	}
	if manifest.ControlPlaneMode != string(ControlPlaneModeProviderHostedMSP) {
		return fmt.Errorf("backup control_plane_mode = %q, want %q", manifest.ControlPlaneMode, ControlPlaneModeProviderHostedMSP)
	}
	if strings.TrimSpace(manifest.PlanVersion) == "" {
		return fmt.Errorf("backup manifest is missing plan_version")
	}
	if strings.TrimSpace(manifest.PlanSource) == "" {
		return fmt.Errorf("backup manifest is missing plan_source")
	}
	if manifest.ControlPlaneDir != providerMSPBackupControlPlaneDir {
		return fmt.Errorf("backup control_plane_dir = %q, want %q", manifest.ControlPlaneDir, providerMSPBackupControlPlaneDir)
	}
	if manifest.TenantsDir != providerMSPBackupTenantsDir {
		return fmt.Errorf("backup tenants_dir = %q, want %q", manifest.TenantsDir, providerMSPBackupTenantsDir)
	}
	if manifest.RegistryTenantCount != len(manifest.RegistryTenantIDs) {
		return fmt.Errorf("backup registry_tenant_count = %d, want %d ids", manifest.RegistryTenantCount, len(manifest.RegistryTenantIDs))
	}
	if manifest.RuntimeTenantCount != len(manifest.RuntimeTenantIDs) {
		return fmt.Errorf("backup runtime_tenant_count = %d, want %d ids", manifest.RuntimeTenantCount, len(manifest.RuntimeTenantIDs))
	}
	return nil
}

func readProviderMSPBackupManifestBytes(reader io.Reader, size int64) ([]byte, error) {
	if size < 0 || size > maxProviderMSPBackupManifestSize {
		return nil, fmt.Errorf("backup manifest size %d is outside allowed range", size)
	}
	limited := io.LimitReader(reader, maxProviderMSPBackupManifestSize+1)
	content, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read backup manifest: %w", err)
	}
	if len(content) > maxProviderMSPBackupManifestSize {
		return nil, fmt.Errorf("backup manifest exceeds %d bytes", maxProviderMSPBackupManifestSize)
	}
	return content, nil
}

func cleanProviderMSPArchiveName(raw string) (string, error) {
	name := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if name == "" {
		return "", fmt.Errorf("backup archive contains empty entry name")
	}
	name = strings.TrimPrefix(name, "./")
	cleaned := path.Clean(name)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
		return "", fmt.Errorf("backup archive contains unsafe entry name %q", raw)
	}
	return cleaned, nil
}

func archiveTenantID(name string) string {
	if name == providerMSPBackupTenantsDir || !strings.HasPrefix(name, providerMSPBackupTenantsDir+"/") {
		return ""
	}
	rest := strings.TrimPrefix(name, providerMSPBackupTenantsDir+"/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func isProviderMSPSQLiteSidecar(rel string) bool {
	name := strings.ToLower(filepath.Base(rel))
	return strings.HasSuffix(name, ".db-wal") || strings.HasSuffix(name, ".db-shm")
}

func pathIsInside(candidate, parent string) bool {
	candidate = filepath.Clean(candidate)
	parent = filepath.Clean(parent)
	rel, err := filepath.Rel(parent, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func requireDirectory(dir, label string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("%s unavailable: %w", label, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s %q is not a directory", label, dir)
	}
	return nil
}

func requireRegularFile(filePath, label string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("%s unavailable: %w", label, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s %q is not a regular file", label, filePath)
	}
	return nil
}

type providerMSPFileInfoWithMode struct {
	os.FileInfo
	mode os.FileMode
}

func (i providerMSPFileInfoWithMode) Mode() os.FileMode {
	return i.mode
}
