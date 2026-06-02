package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	"github.com/spf13/cobra"
)

func newProviderMSPBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create and verify provider-hosted MSP recovery archives",
	}
	cmd.AddCommand(newProviderMSPBackupCreateCmd())
	cmd.AddCommand(newProviderMSPBackupRestoreCmd())
	cmd.AddCommand(newProviderMSPBackupVerifyCmd())
	return cmd
}

func newProviderMSPBackupCreateCmd() *cobra.Command {
	var outputPath string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a provider-hosted MSP recovery archive",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudcp.LoadConfig()
			if err != nil {
				return fmt.Errorf("load control plane config: %w", err)
			}
			result, err := cloudcp.CreateProviderMSPBackup(cmd.Context(), cfg, outputPath)
			if err != nil {
				return err
			}
			printProviderMSPBackupCreateResult(result)
			return nil
		},
	}
	cmd.Flags().StringVar(&outputPath, "output", "", "Backup archive path (default: <CP_DATA_DIR>/backups/provider-msp/provider-msp-backup-<timestamp>.tar.gz)")
	return cmd
}

func newProviderMSPBackupRestoreCmd() *cobra.Command {
	var archivePath string
	var targetDataDir string
	var licenseOutputPath string
	var replaceExisting bool
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "restore [archive]",
		Short: "Restore a provider-hosted MSP recovery archive",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				if strings.TrimSpace(archivePath) != "" && archivePath != args[0] {
					return fmt.Errorf("archive path provided both as --archive and positional argument")
				}
				archivePath = args[0]
			}
			if strings.TrimSpace(targetDataDir) == "" {
				targetDataDir = providerMSPRestoreDefaultDataDir()
			}
			result, err := cloudcp.RestoreProviderMSPBackup(cmd.Context(), cloudcp.ProviderMSPBackupRestoreOptions{
				ArchivePath:       archivePath,
				TargetDataDir:     targetDataDir,
				LicenseOutputPath: licenseOutputPath,
				ReplaceExisting:   replaceExisting,
				DryRun:            dryRun,
			})
			if err != nil {
				return err
			}
			printProviderMSPBackupRestoreResult(result)
			return nil
		},
	}
	cmd.Flags().StringVar(&archivePath, "archive", "", "Backup archive path to restore")
	cmd.Flags().StringVar(&targetDataDir, "target-data-dir", "", "Target CP_DATA_DIR to restore into (default: CP_DATA_DIR or /data)")
	cmd.Flags().StringVar(&licenseOutputPath, "license-output", "", "Where to restore the provider MSP license file (default: <target-data-dir>/provider-msp-license.jwt)")
	cmd.Flags().BoolVar(&replaceExisting, "replace", false, "Replace existing restored control-plane/tenant state after the control plane has been stopped")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Verify and report restore targets without writing files")
	return cmd
}

func newProviderMSPBackupVerifyCmd() *cobra.Command {
	var archivePath string
	cmd := &cobra.Command{
		Use:   "verify [archive]",
		Short: "Verify a provider-hosted MSP recovery archive",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				if strings.TrimSpace(archivePath) != "" && archivePath != args[0] {
					return fmt.Errorf("archive path provided both as --archive and positional argument")
				}
				archivePath = args[0]
			}
			result, err := cloudcp.VerifyProviderMSPBackup(cmd.Context(), archivePath)
			if err != nil {
				return err
			}
			printProviderMSPBackupVerifyResult(result)
			return nil
		},
	}
	cmd.Flags().StringVar(&archivePath, "archive", "", "Backup archive path to verify")
	return cmd
}

func providerMSPRestoreDefaultDataDir() string {
	if value := strings.TrimSpace(os.Getenv("CP_DATA_DIR")); value != "" {
		return value
	}
	return "/data"
}

func printProviderMSPBackupCreateResult(result *cloudcp.ProviderMSPBackupCreateResult) {
	if result == nil {
		fmt.Println("provider_msp_backup_created=false")
		return
	}
	fmt.Println("provider_msp_backup_created=true")
	fmt.Printf("archive_path=%s\n", result.ArchivePath)
	fmt.Printf("archive_bytes=%d\n", result.BytesWritten)
	printProviderMSPBackupManifest(result.Manifest)
	fmt.Printf("control_plane_entries=%d\n", result.ControlPlaneEntries)
	fmt.Printf("tenant_entries=%d\n", result.TenantEntries)
	fmt.Printf("license_entries=%d\n", result.LicenseEntries)
}

func printProviderMSPBackupRestoreResult(result *cloudcp.ProviderMSPBackupRestoreResult) {
	if result == nil {
		fmt.Println("provider_msp_backup_restored=false")
		return
	}
	fmt.Printf("provider_msp_backup_restored=%t\n", !result.DryRun)
	fmt.Printf("provider_msp_backup_restore_dry_run=%t\n", result.DryRun)
	fmt.Printf("archive_path=%s\n", result.ArchivePath)
	fmt.Printf("archive_bytes=%d\n", result.VerifiedArchiveBytes)
	fmt.Printf("target_data_dir=%s\n", result.TargetDataDir)
	fmt.Printf("control_plane_dir=%s\n", result.ControlPlaneDir)
	fmt.Printf("tenants_dir=%s\n", result.TenantsDir)
	fmt.Printf("license_output_path=%s\n", result.LicenseOutputPath)
	fmt.Printf("replace_existing=%t\n", result.ReplaceExisting)
	printProviderMSPBackupManifest(result.Manifest)
	fmt.Printf("control_plane_entries_restored=%d\n", result.ControlPlaneEntriesRestored)
	fmt.Printf("tenant_entries_restored=%d\n", result.TenantEntriesRestored)
	fmt.Printf("license_entries_restored=%d\n", result.LicenseEntriesRestored)
	fmt.Printf("restored_registry_tenant_count=%d\n", result.RestoredRegistryTenantCount)
	for _, tenantID := range result.RestoredRuntimeTenantIDs {
		fmt.Printf("restored_runtime_tenant_id=%s\n", tenantID)
	}
}

func printProviderMSPBackupVerifyResult(result *cloudcp.ProviderMSPBackupVerifyResult) {
	if result == nil {
		fmt.Println("provider_msp_backup_verified=false")
		return
	}
	fmt.Println("provider_msp_backup_verified=true")
	fmt.Printf("archive_path=%s\n", result.ArchivePath)
	fmt.Printf("archive_bytes=%d\n", result.VerifiedArchiveBytes)
	printProviderMSPBackupManifest(result.Manifest)
	fmt.Printf("control_plane_entries=%d\n", result.ControlPlaneEntries)
	fmt.Printf("tenant_entries=%d\n", result.TenantEntries)
	fmt.Printf("license_entries=%d\n", result.LicenseEntries)
	fmt.Printf("tenant_registry_db_present=%t\n", result.HasTenantRegistryDB)
	fmt.Printf("license_file_present=%t\n", result.HasLicenseFile)
	for _, dbFile := range result.ControlPlaneDBFiles {
		fmt.Printf("control_plane_db_backup=%s\n", dbFile)
	}
	for _, tenantID := range result.RuntimeTenantDirs {
		fmt.Printf("runtime_tenant_dir=%s\n", tenantID)
	}
}

func printProviderMSPBackupManifest(manifest cloudcp.ProviderMSPBackupManifest) {
	fmt.Printf("manifest_version=%s\n", manifest.Version)
	fmt.Printf("created_at=%s\n", manifest.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Printf("control_plane_mode=%s\n", manifest.ControlPlaneMode)
	fmt.Printf("environment=%s\n", manifest.Environment)
	fmt.Printf("base_url=%s\n", manifest.BaseURL)
	fmt.Printf("plan_version=%s\n", manifest.PlanVersion)
	fmt.Printf("plan_source=%s\n", manifest.PlanSource)
	fmt.Printf("license_id=%s\n", manifest.LicenseID)
	fmt.Printf("license_email=%s\n", manifest.LicenseEmail)
	fmt.Printf("license_included=%t\n", manifest.LicenseIncluded)
	fmt.Printf("workspace_limit=%d\n", manifest.WorkspaceLimit)
	fmt.Printf("registry_account_count=%d\n", manifest.RegistryAccountCount)
	fmt.Printf("registry_tenant_count=%d\n", manifest.RegistryTenantCount)
	fmt.Printf("runtime_tenant_count=%d\n", manifest.RuntimeTenantCount)
	for _, tenantID := range manifest.RuntimeTenantIDs {
		fmt.Printf("runtime_tenant_id=%s\n", tenantID)
	}
	for _, dbFile := range manifest.ControlPlaneDBBackups {
		fmt.Printf("manifest_db_backup=%s\n", dbFile)
	}
}
