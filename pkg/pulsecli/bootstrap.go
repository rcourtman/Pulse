package pulsecli

import (
	"fmt"
	"os"

	bootstrapstore "github.com/rcourtman/pulse-go-rewrite/internal/bootstrap"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/spf13/cobra"
)

func ShowBootstrapToken(deps *BootstrapDeps) {
	dataPath := config.ResolveRuntimeDataDir("")

	token, tokenPath, _, err := bootstrapstore.Load(dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("╔═══════════════════════════════════════════════════════════════════════╗")
			fmt.Println("║                    NO BOOTSTRAP TOKEN FOUND                           ║")
			fmt.Println("╠═══════════════════════════════════════════════════════════════════════╣")
			fmt.Println("║  Possible reasons:                                                    ║")
			fmt.Println("║  • Initial setup has already been completed                           ║")
			fmt.Println("║  • Authentication is configured (token auto-deleted)                  ║")
			fmt.Println("║  • Server hasn't started yet (token not generated)                    ║")
			fmt.Printf("║  • Token file not found: %-44s║\n", tokenPath)
			fmt.Println("╚═══════════════════════════════════════════════════════════════════════╝")
			bootstrapExit(deps, 1)
			return
		}
		fmt.Printf("Error reading bootstrap token: %v\n", err)
		bootstrapExit(deps, 1)
		return
	}

	fmt.Println("╔═══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║          BOOTSTRAP TOKEN FOR FIRST-TIME SETUP                         ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Token: %-61s ║\n", token)
	fmt.Printf("║  File:  %-61s ║\n", tokenPath)
	fmt.Println("╠═══════════════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Instructions:                                                        ║")
	fmt.Println("║  1. Copy the token above                                              ║")
	fmt.Println("║  2. Open Pulse in your web browser                                    ║")
	fmt.Println("║  3. Paste the token into the unlock screen                            ║")
	fmt.Println("║  4. Complete the admin account setup                                  ║")
	fmt.Println("║                                                                       ║")
	fmt.Println("║  This token will be automatically deleted after successful setup.     ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════════╝")
}

func newBootstrapTokenCmd(bootstrap *BootstrapDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "bootstrap-token",
		Short: "Display the bootstrap setup token",
		Long: `Display the bootstrap setup token required for first-time setup.

This token is generated on first boot and must be entered in the web UI
to unlock the initial setup wizard. The token is automatically deleted
after successful setup completion.`,
		Run: func(cmd *cobra.Command, args []string) {
			ShowBootstrapToken(bootstrap)
		},
	}
}
