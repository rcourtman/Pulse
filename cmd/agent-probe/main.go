// Command agent-probe is a worked example of an external agent
// consuming Pulse's agent-paradigm read substrate. It is not a
// production tool — it exists as the smallest legible reference
// implementation so anyone building MCP servers, Claude Code
// integrations, or custom agents on top of Pulse can see the
// discovery → triage → depth → push flow as one short program.
//
// What it does, in order:
//
//  1. Fetches /api/agent/capabilities (unauthenticated) and prints
//     the declared capabilities. This is "discovery" — the agent
//     learns what's available from the wire, not from documentation.
//
//  2. Resolves the get_fleet_context capability from the manifest
//     and calls it (authenticated). This is "triage" — one read for
//     "where do I focus?".
//
//  3. Picks a focus resource: critical findings first, then warning,
//     then first in the list. Calls get_resource_context for it.
//     This is "depth" — the situated picture for the chosen target.
//
//  4. Subscribes to /api/agent/events (SSE). Prints the next event
//     it sees and exits. This is "push" — proof the doorbell wires
//     up correctly. A short timeout guards against an idle stream.
//
// Hard constraints honored on purpose:
//
//   - No internal/ai or API handler imports. The only Pulse package
//     reused is the tiny agentcapabilities wire/projection package, so this
//     in-repo reference client cannot drift from the manifest shape or path
//     projection rules. A real external agent can define the same JSON shape
//     from the manifest or a generated client.
//   - Branches on stable error codes from the manifest, never on
//     human-readable error messages.
//   - Resolves paths from the manifest rather than hardcoding them.
//     If discovery moves a path, this probe follows automatically.
//
// Run it against a local instance:
//
//	go run ./cmd/agent-probe \
//	    --base-url http://localhost:7655 \
//	    --token "<api-token>" \
//	    --event-timeout 5s
//
// The token needs the monitoring:read scope. Discovery itself is
// public, but the rest of the substrate is gated.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

// fleetResource is the per-resource thin rollup the fleet endpoint
// returns. We only depend on the fields the probe actually needs to
// pick a focus and print one-line summaries.
type fleetResource struct {
	CanonicalID             string `json:"canonicalId"`
	ResourceType            string `json:"resourceType"`
	ResourceName            string `json:"resourceName"`
	IntentionallyOffline    bool   `json:"intentionallyOffline"`
	NeverAutoRemediate      bool   `json:"neverAutoRemediate"`
	MaintenanceWindowActive bool   `json:"maintenanceWindowActive"`
	Findings                struct {
		Total    int `json:"total"`
		Critical int `json:"critical"`
		Warning  int `json:"warning"`
		Info     int `json:"info"`
	} `json:"findings"`
	PendingApprovalCount int `json:"pendingApprovalCount"`
}

type fleetContext struct {
	Resources   []fleetResource `json:"resources"`
	GeneratedAt time.Time       `json:"generatedAt"`
}

func main() {
	baseURL := flag.String("base-url", "http://localhost:7655", "Pulse base URL")
	token := flag.String("token", "", "API token with monitoring:read scope (required for triage and depth steps)")
	eventTimeout := flag.Duration("event-timeout", 5*time.Second, "How long to wait for the first SSE event before giving up")
	flag.Parse()

	if strings.TrimSpace(*token) == "" {
		fmt.Fprintln(os.Stderr, "agent-probe: --token is required for triage/depth/push steps; discovery alone works without it")
		os.Exit(2)
	}

	client := &http.Client{Timeout: 15 * time.Second}

	// --- 1. Discovery. Public, no token needed. ---
	manifest, err := agentcapabilities.FetchManifest(context.Background(), client, *baseURL)
	if err != nil {
		exitf("discovery failed: %v", err)
	}
	fmt.Printf("discovered manifest %s with %d capabilities\n", manifest.Version, len(manifest.Capabilities))
	for _, cap := range manifest.Capabilities {
		fmt.Printf("  %-22s %s %s  (%s)\n", cap.Name, cap.Method, cap.Path, cap.Scope)
	}

	streamCap, ok := agentcapabilities.FindCapability(manifest.Capabilities, agentcapabilities.EventSubscriptionCapabilityName)
	if !ok {
		exitf("manifest missing required capability subscribe_events")
	}

	// --- 2. Triage. Authenticated. ---
	fleet, err := fetchFleet(context.Background(), client, *baseURL, manifest.Capabilities, *token)
	if err != nil {
		exitf("triage failed: %v", err)
	}
	fmt.Printf("\nfleet sweep — %d resources at %s\n", len(fleet.Resources), fleet.GeneratedAt.Format(time.RFC3339))
	for _, r := range fleet.Resources {
		flags := []string{}
		if r.IntentionallyOffline {
			flags = append(flags, "offline")
		}
		if r.NeverAutoRemediate {
			flags = append(flags, "locked")
		}
		if r.MaintenanceWindowActive {
			flags = append(flags, "maintenance")
		}
		flagStr := ""
		if len(flags) > 0 {
			flagStr = " [" + strings.Join(flags, ",") + "]"
		}
		fmt.Printf("  %-30s %-12s findings: %d (c=%d w=%d i=%d)  pending: %d%s\n",
			r.CanonicalID, r.ResourceType,
			r.Findings.Total, r.Findings.Critical, r.Findings.Warning, r.Findings.Info,
			r.PendingApprovalCount, flagStr)
	}

	focus := pickFocus(fleet.Resources)
	if focus == nil {
		fmt.Println("\nno resources visible to this token; skipping depth step")
	} else {
		// --- 3. Depth. ---
		body, err := agentcapabilities.CallRequestResponseCapabilityHTTPBodyByName(
			context.Background(),
			client,
			*baseURL,
			*token,
			manifest.Capabilities,
			agentcapabilities.ResourceContextCapabilityName,
			map[string]any{
				"resourceId": focus.CanonicalID,
			},
		)
		if err != nil {
			exitf("depth failed for %s: %v", focus.CanonicalID, err)
		}
		fmt.Printf("\nresource-context for %s:\n", focus.CanonicalID)
		// Pretty-print the body so a reader sees the substrate's
		// shape without us redefining every type. Real agents would
		// decode against typed structs.
		var pretty map[string]any
		if err := json.Unmarshal(body, &pretty); err == nil {
			out, _ := json.MarshalIndent(pretty, "  ", "  ")
			fmt.Println("  " + string(out))
		} else {
			fmt.Println(string(body))
		}
	}

	// --- 4. Push. ---
	fmt.Printf("\nsubscribing to %s (waiting up to %s for the first event)\n",
		streamCap.Path, *eventTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), *eventTimeout)
	defer cancel()
	if err := readOneSSEEvent(ctx, *baseURL, streamCap.Path, *token); err != nil {
		// Timeout is the boring case (no events fired); not a hard
		// failure for a probe.
		if strings.Contains(err.Error(), "deadline exceeded") {
			fmt.Println("  (no event in window — stream is healthy if the connect succeeded; idle is normal)")
		} else {
			exitf("push failed: %v", err)
		}
	}
	fmt.Println("\nagent-probe done — discovery, triage, depth, and push all walked the substrate cleanly.")
}

func fetchFleet(ctx context.Context, client *http.Client, baseURL string, capabilities []agentcapabilities.Capability, token string) (*fleetContext, error) {
	body, err := agentcapabilities.CallRequestResponseCapabilityHTTPBodyByName(
		ctx,
		client,
		baseURL,
		token,
		capabilities,
		agentcapabilities.FleetContextCapabilityName,
		nil,
	)
	if err != nil {
		return nil, err
	}
	var fleet fleetContext
	if err := json.Unmarshal(body, &fleet); err != nil {
		return nil, fmt.Errorf("decode fleet: %w", err)
	}
	return &fleet, nil
}

// pickFocus chooses the most "interesting" resource from a fleet
// sweep. The triage rule is lexicographic — severity dominates
// count: any resource with a critical finding outranks every
// resource with only warnings, regardless of warning count; same
// for warning over info, and findings over pending approvals. The
// rule is intentionally simple so a reader can predict what the
// probe will pick. Real agents will have richer policies; this is
// legible default behavior for a probe.
func pickFocus(resources []fleetResource) *fleetResource {
	if len(resources) == 0 {
		return nil
	}
	best := 0
	for i := 1; i < len(resources); i++ {
		if focusLess(resources[best], resources[i]) {
			best = i
		}
	}
	return &resources[best]
}

// focusLess returns true if `a` is less interesting than `b` under
// the probe's lex ordering. Comparison cascades: critical count,
// then warning count, then info count, then pending approvals.
func focusLess(a, b fleetResource) bool {
	if a.Findings.Critical != b.Findings.Critical {
		return a.Findings.Critical < b.Findings.Critical
	}
	if a.Findings.Warning != b.Findings.Warning {
		return a.Findings.Warning < b.Findings.Warning
	}
	if a.Findings.Info != b.Findings.Info {
		return a.Findings.Info < b.Findings.Info
	}
	return a.PendingApprovalCount < b.PendingApprovalCount
}

// readOneSSEEvent opens the SSE stream, reads up to the first
// non-keepalive event payload through the shared Pulse Intelligence
// SSE parser, prints it, and returns.
func readOneSSEEvent(ctx context.Context, baseURL, path, token string) error {
	seen := false
	if err := agentcapabilities.StreamAgentActionableSSERecords(ctx, nil, baseURL, path, token, func(record agentcapabilities.SSERecord) bool {
		fmt.Printf("  event: %s\n  data: %s\n", record.Event, record.Data)
		seen = true
		return false
	}); err != nil {
		return err
	}
	if seen {
		return nil
	}
	return ctx.Err()
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "agent-probe: "+format+"\n", args...)
	os.Exit(1)
}
