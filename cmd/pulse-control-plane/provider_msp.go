package main

import (
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	"github.com/spf13/cobra"
)

func newProviderMSPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider-msp",
		Short: "Operate a provider-hosted MSP control plane",
	}
	cmd.AddCommand(newProviderMSPBootstrapCmd())
	cmd.AddCommand(newProviderMSPPreflightCmd())
	cmd.AddCommand(newProviderMSPProofCmd())
	return cmd
}

func newProviderMSPBootstrapCmd() *cobra.Command {
	var accountID string
	var accountName string
	var ownerEmail string
	var magicLink bool

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Create or update the provider MSP account owner",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudcp.LoadConfig()
			if err != nil {
				return fmt.Errorf("load control plane config: %w", err)
			}
			result, err := cloudcp.BootstrapProviderMSP(cmd.Context(), cfg, cloudcp.ProviderMSPBootstrapOptions{
				AccountID:         accountID,
				AccountName:       accountName,
				OwnerEmail:        ownerEmail,
				GenerateMagicLink: magicLink,
			})
			if err != nil {
				return err
			}
			printProviderMSPBootstrapResult(result)
			return nil
		},
	}
	cmd.Flags().StringVar(&accountID, "account-id", "", "Existing MSP account ID to bootstrap; omitted on first install")
	cmd.Flags().StringVar(&accountName, "account-name", "", "Provider account display name")
	cmd.Flags().StringVar(&ownerEmail, "owner-email", "", "Provider owner email address")
	cmd.Flags().BoolVar(&magicLink, "magic-link", true, "Generate a one-time Pulse Account portal sign-in link")
	_ = cmd.MarkFlagRequired("account-name")
	_ = cmd.MarkFlagRequired("owner-email")
	return cmd
}

func printProviderMSPBootstrapResult(result *cloudcp.ProviderMSPBootstrapResult) {
	if result == nil {
		fmt.Println("provider_msp_bootstrap_ok=false")
		return
	}
	fmt.Println("provider_msp_bootstrap_ok=true")
	fmt.Printf("account_id=%s\n", result.AccountID)
	fmt.Printf("account_name=%s\n", result.AccountName)
	fmt.Printf("owner_user_id=%s\n", result.OwnerUserID)
	fmt.Printf("owner_email=%s\n", result.OwnerEmail)
	fmt.Printf("plan_version=%s\n", result.PlanVersion)
	fmt.Printf("plan_source=%s\n", result.PlanSource)
	fmt.Printf("license_id=%s\n", result.LicenseID)
	fmt.Printf("license_email=%s\n", result.LicenseEmail)
	fmt.Printf("workspace_limit=%d\n", result.WorkspaceLimit)
	if result.MagicLinkURL != "" {
		fmt.Printf("portal_magic_link=%s\n", result.MagicLinkURL)
	}
}
