package main

import (
	"fmt"
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
