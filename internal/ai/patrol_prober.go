// patrol_prober.go implements GuestProber using the agentexec system to run
// ping commands on connected host agents.
package ai

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

const maxConcurrentGuestPings = 16

type guestPingProbeResult struct {
	ip        string
	reachable bool
	err       error
}

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
// It sends one validated read-only ping command per target so the agent-exec
// policy can auto-approve the command without permitting compound shell input.
func (p *agentExecProber) PingGuests(ctx context.Context, agentID string, ips []string) (map[string]PingResult, error) {
	if p.server == nil {
		return nil, fmt.Errorf("agent exec server not available")
	}
	if len(ips) == 0 {
		return map[string]PingResult{}, nil
	}

	targets := make([]string, 0, len(ips))
	for _, rawIP := range ips {
		ip, err := canonicalPingIP(rawIP)
		if err != nil {
			return nil, err
		}
		targets = append(targets, ip)
	}

	results := make(map[string]PingResult, len(targets))
	workerLimit := maxConcurrentGuestPings
	if len(targets) < workerLimit {
		workerLimit = len(targets)
	}
	sem := make(chan struct{}, workerLimit)
	resultCh := make(chan guestPingProbeResult, len(targets))

	var wg sync.WaitGroup
	for _, ip := range targets {
		ip := ip
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				resultCh <- guestPingProbeResult{ip: ip, err: ctx.Err()}
				return
			}

			result, err := p.server.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
				RequestID:  uuid.New().String(),
				Command:    pingCommandForIP(ip),
				TargetType: "agent",
				Timeout:    3,
			})
			if err != nil {
				resultCh <- guestPingProbeResult{ip: ip, err: fmt.Errorf("ping %s failed: %w", ip, err)}
				return
			}
			if result == nil {
				resultCh <- guestPingProbeResult{ip: ip, err: fmt.Errorf("ping %s returned no result", ip)}
				return
			}

			resultCh <- guestPingProbeResult{ip: ip, reachable: result.ExitCode == 0}
		}()
	}

	wg.Wait()
	close(resultCh)

	var firstErr error
	for item := range resultCh {
		if item.err != nil {
			if firstErr == nil {
				firstErr = item.err
			}
			continue
		}
		results[item.ip] = PingResult{Reachable: item.reachable}
	}
	if firstErr != nil {
		return nil, firstErr
	}

	return results, nil
}

func canonicalPingIP(rawIP string) (string, error) {
	rawIP = strings.TrimSpace(rawIP)
	addr, err := netip.ParseAddr(rawIP)
	if err != nil {
		return "", fmt.Errorf("invalid ping target %q: expected an IP address", rawIP)
	}
	return addr.Unmap().String(), nil
}

func pingCommandForIP(ip string) string {
	return "ping -c 1 -W 1 " + ip
}
