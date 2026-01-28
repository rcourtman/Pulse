package tools

import (
	"context"
	"fmt"
)

// registerControlToolsConsolidated registers the consolidated pulse_control tool
func (e *PulseToolExecutor) registerControlToolsConsolidated() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_control",
			Description: `Control Proxmox VMs/LXC containers or execute WRITE commands on infrastructure.

IMPORTANT: For READ operations (grep, cat, tail, logs, ps, status checks), use pulse_read instead.
This tool is for WRITE operations that modify state.

Types:
- guest: Start, stop, restart, shutdown, or delete VMs and LXC containers
- command: Execute commands that MODIFY state (restart services, write files, etc.)

USE pulse_control FOR:
- Guest control: start/stop/restart/delete VMs and LXCs
- Service management: systemctl restart, service start/stop
- Package management: apt install, yum update
- File modification: echo > file, sed -i, rm, mv, cp

DO NOT use pulse_control for:
- Reading logs → use pulse_read action=exec or action=logs
- Checking status → use pulse_read action=exec
- Reading files → use pulse_read action=file
- Finding files → use pulse_read action=find

Examples:
- Restart VM: type="guest", guest_id="101", action="restart"
- Restart service: type="command", command="systemctl restart nginx", target_host="webserver"

For Docker container control, use pulse_docker.
Note: Delete requires the guest to be stopped first.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "Control type: guest or command",
						Enum:        []string{"guest", "command"},
					},
					"guest_id": {
						Type:        "string",
						Description: "For guest: VMID or name",
					},
					"action": {
						Type:        "string",
						Description: "For guest: start, stop, shutdown, restart, delete",
						Enum:        []string{"start", "stop", "shutdown", "restart", "delete"},
					},
					"command": {
						Type:        "string",
						Description: "For command type: the shell command to execute",
					},
					"target_host": {
						Type:        "string",
						Description: "For command type: hostname to run command on",
					},
					"run_on_host": {
						Type:        "boolean",
						Description: "For command type: run on host (default true)",
					},
					"force": {
						Type:        "boolean",
						Description: "For guest stop: force stop without graceful shutdown",
					},
				},
				Required: []string{"type"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeControl(ctx, args)
		},
		RequireControl: true,
	})
}

// executeControl routes to the appropriate control handler based on type
// Handler functions are implemented in tools_control.go
func (e *PulseToolExecutor) executeControl(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	controlType, _ := args["type"].(string)
	switch controlType {
	case "guest":
		return e.executeControlGuest(ctx, args)
	case "command":
		return e.executeRunCommand(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown type: %s. Use: guest, command", controlType)), nil
	}
}
