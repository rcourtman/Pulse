// patrol_prober.go implements GuestProber using the agentexec system to run
// ping commands on connected host agents.
package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

// agentExecProber implements GuestProber using the agent execution server.
type agentExecProber struct {
	server *agentexec.Server
}

// NewAgentExecProber creates a GuestProber that runs ping commands via connected agents.
func NewAgentExecProber(server *agentexec.Server) GuestProber {
	return &agentExecProber{server: server}
}

// GetAgentForHost returns the agent ID for a given hostname, if connected.
func (p *agentExecProber) GetAgentForHost(hostname string) (string, bool) {
	if p.server == nil {
		return "", false
	}
	return p.server.GetAgentForHost(hostname)
}

// PingGuests pings a list of IPs from a specific agent and returns results.
// It composes a batch shell command that runs all pings concurrently as background
// subshells, so total wall time is ~1 second regardless of guest count.
func (p *agentExecProber) PingGuests(ctx context.Context, agentID string, ips []string) (map[string]PingResult, error) {
	if p.server == nil {
		return nil, fmt.Errorf("agent exec server not available")
	}
	if len(ips) == 0 {
		return map[string]PingResult{}, nil
	}

	// Build batch ping command: all pings run as background subshells
	// Each outputs REACH:<ip>:UP or REACH:<ip>:DOWN
	var sb strings.Builder
	sb.WriteString("for ip in")
	for _, ip := range ips {
		sb.WriteString(" ")
		sb.WriteString(ip)
	}
	sb.WriteString("; do (ping -c1 -W1 \"$ip\" >/dev/null 2>&1 && echo \"REACH:$ip:UP\" || echo \"REACH:$ip:DOWN\") & done; wait")

	result, err := p.server.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		RequestID:  uuid.New().String(),
		Command:    sb.String(),
		TargetType: "host",
		Timeout:    5, // seconds — generous for parallel pings
	})
	if err != nil {
		return nil, fmt.Errorf("ping command failed: %w", err)
	}

	return parsePingOutput(result.Stdout), nil
}

// parsePingOutput parses the output of the batch ping command.
// Expected lines: "REACH:<ip>:UP" or "REACH:<ip>:DOWN"
func parsePingOutput(output string) map[string]PingResult {
	results := make(map[string]PingResult)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "REACH:") {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		ip := parts[1]
		up := parts[2] == "UP"
		results[ip] = PingResult{Reachable: up}
	}
	return results
}
