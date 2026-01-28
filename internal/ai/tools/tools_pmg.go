package tools

import (
	"context"
	"fmt"
)

// registerPMGToolsConsolidated registers the consolidated pulse_pmg tool
func (e *PulseToolExecutor) registerPMGToolsConsolidated() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_pmg",
			Description: `Query Proxmox Mail Gateway status and statistics.

Types:
- status: Instance status and health (nodes, uptime, load)
- mail_stats: Mail flow statistics (counts, spam, virus, bounces)
- queues: Mail queue status (active, deferred, hold)
- spam: Spam quarantine statistics and score distribution

Examples:
- Get status: type="status"
- Get specific instance: type="status", instance="pmg01"
- Get mail stats: type="mail_stats"
- Get queue status: type="queues"
- Get spam stats: type="spam"`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "PMG query type",
						Enum:        []string{"status", "mail_stats", "queues", "spam"},
					},
					"instance": {
						Type:        "string",
						Description: "Optional: specific PMG instance name or ID",
					},
				},
				Required: []string{"type"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executePMG(ctx, args)
		},
	})
}

// executePMG routes to the appropriate PMG handler based on type
func (e *PulseToolExecutor) executePMG(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	pmgType, _ := args["type"].(string)
	switch pmgType {
	case "status":
		return e.executeGetPMGStatus(ctx, args)
	case "mail_stats":
		return e.executeGetMailStats(ctx, args)
	case "queues":
		return e.executeGetMailQueues(ctx, args)
	case "spam":
		return e.executeGetSpamStats(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown type: %s. Use: status, mail_stats, queues, spam", pmgType)), nil
	}
}
