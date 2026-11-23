package pulsecli

import (
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// ClientDeps bundles optional dependency injections for the standalone `pulse`
// client binary (cmd/pulse-cli) and shared remote subcommands on `cmd/pulse`.
type ClientDeps struct {
	Auth    *AuthDeps
	Connect *ConnectDeps
	Doctor  *DoctorDeps
	MCP     *MCPDeps
	Remote  *RemoteDeps
	Fleet   *FleetDeps
	Actions *ActionsDeps
}

// NormalizeClientArgs applies the argv[0] compatibility shim: when the client
// binary is invoked via a `pulse-mcp` symlink or executable name (including
// `pulse-mcp.exe` or `pulse-mcp-*`), dispatch directly to the built-in `mcp`
// subcommand without duplicating `mcp` if already specified.
func NormalizeClientArgs(argv0 string, args []string) []string {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(argv0)))
	if !strings.HasPrefix(base, "pulse-mcp") {
		return args
	}
	if len(args) > 0 && args[0] == "mcp" {
		return args
	}
	out := make([]string, 0, len(args)+1)
	out = append(out, "mcp")
	out = append(out, args...)
	return out
}

// NewClientRootCommand constructs the root Cobra command for the standalone
// cross-platform `pulse` workstation CLI (`cmd/pulse-cli`). Bare `pulse` prints
// help; all operational subcommands talk to a remote Pulse instance over HTTP
// or run the built-in `pulse mcp` stdio adapter.
func NewClientRootCommand(command CommandSpec, deps ClientDeps) *cobra.Command {
	if strings.TrimSpace(command.Use) == "" {
		command.Use = "pulse"
	}
	if strings.TrimSpace(command.Short) == "" {
		command.Short = "Connect external agents and inspect your Pulse fleet"
	}
	if strings.TrimSpace(command.Long) == "" {
		command.Long = "Pulse CLI connects coding agents (Claude Code, Codex, OpenCode, Claude Desktop) to Pulse over MCP and provides scriptable human/JSON terminal commands backed by /api/agent/capabilities."
	}

	cmd := &cobra.Command{
		Use:          command.Use,
		Short:        command.Short,
		Long:         command.Long,
		SilenceUsage: true,
		Version:      command.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	if command.VersionTemplate != "" {
		cmd.SetVersionTemplate(command.VersionTemplate)
	}

	addClientPersistentFlags(cmd)
	cmd.AddCommand(newVersionCmd(command))
	registerRemoteCommands(cmd, deps)
	return cmd
}

func addClientPersistentFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String("api-url", "", "Pulse server URL (overrides saved profile, PULSE_API_URL takes precedence)")
	cmd.PersistentFlags().String("profile", "", "Saved auth profile name (defaults to active default profile)")
	cmd.PersistentFlags().Bool("json", false, "Emit JSON output (automatic when stdout is not a TTY)")
}

func registerRemoteCommands(cmd *cobra.Command, deps ClientDeps) {
	cmd.AddCommand(NewAuthCommand(deps.Auth))
	cmd.AddCommand(NewConnectCommand(deps.Connect))
	cmd.AddCommand(NewDoctorCommand(deps.Doctor))
	cmd.AddCommand(NewMCPCommand(deps.MCP))
	cmd.AddCommand(newFleetCmd(deps.Fleet))
	cmd.AddCommand(newResourceCmd(deps.Remote))
	cmd.AddCommand(newChangesCmd(deps.Remote))
	cmd.AddCommand(newMetricsCmd(deps.Remote))
	cmd.AddCommand(newEventsCmd(deps.Remote))
	cmd.AddCommand(newFindingsCmd(deps.Remote))
	cmd.AddCommand(newOperatorStateCmd(deps.Remote))
	cmd.AddCommand(newActionsCmd(deps.Actions))
	hasConfigCmd := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "config" {
			hasConfigCmd = true
			break
		}
	}
	if !hasConfigCmd {
		cmd.AddCommand(newDeclarativeConfigCmd(deps.Remote))
	}
	cmd.AddCommand(newCallCmd(deps.Remote))
}
