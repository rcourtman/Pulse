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
//   - Standard library only. No internal/ai imports, no Pulse types
//     reused. A real external agent has no privileged access.
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
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// agentCapability mirrors the api package's AgentCapability wire
// shape — defined inline so this program depends on nothing in the
// pulse module. If the manifest's shape evolves, this struct
// follows; the JSON tags are the contract.
type agentCapability struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Category         string   `json:"category"`
	Method           string   `json:"method"`
	Path             string   `json:"path"`
	Scope            string   `json:"scope"`
	ResponseShape    string   `json:"responseShape,omitempty"`
	ErrorCodes       []string `json:"errorCodes,omitempty"`
	RequestBodyShape string   `json:"requestBodyShape,omitempty"`
}

type agentCapabilitiesManifest struct {
	Version      string            `json:"version"`
	Capabilities []agentCapability `json:"capabilities"`
}

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

// errorEnvelope is the shared shape every agent-surface endpoint
// uses on failure. The "error" field is a stable snake_case code
// (e.g. resource_not_found, operator_state_not_set); "message" is
// human text agents may surface but must not branch on.
type errorEnvelope struct {
	Error   string `json:"error"`
	Message string `json:"message"`
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
	manifest, err := fetchManifest(client, *baseURL)
	if err != nil {
		exitf("discovery failed: %v", err)
	}
	fmt.Printf("discovered manifest %s with %d capabilities\n", manifest.Version, len(manifest.Capabilities))
	for _, cap := range manifest.Capabilities {
		fmt.Printf("  %-22s %s %s  (%s)\n", cap.Name, cap.Method, cap.Path, cap.Scope)
	}

	byName := map[string]agentCapability{}
	for _, c := range manifest.Capabilities {
		byName[c.Name] = c
	}

	fleetCap, ok := byName["get_fleet_context"]
	if !ok {
		exitf("manifest missing required capability get_fleet_context")
	}
	contextCap, ok := byName["get_resource_context"]
	if !ok {
		exitf("manifest missing required capability get_resource_context")
	}
	streamCap, ok := byName["subscribe_events"]
	if !ok {
		exitf("manifest missing required capability subscribe_events")
	}

	// --- 2. Triage. Authenticated. ---
	fleet, err := fetchFleet(client, *baseURL, fleetCap.Path, *token)
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
		depthPath := strings.Replace(contextCap.Path, "{resourceId}", focus.CanonicalID, 1)
		body, err := fetchAuthenticatedRaw(client, *baseURL+depthPath, *token)
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
	if err := readOneSSEEvent(ctx, *baseURL+streamCap.Path, *token); err != nil {
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

func fetchManifest(client *http.Client, baseURL string) (*agentCapabilitiesManifest, error) {
	resp, err := client.Get(baseURL + "/api/agent/capabilities")
	if err != nil {
		return nil, fmt.Errorf("GET capabilities: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET capabilities: status %d", resp.StatusCode)
	}
	var manifest agentCapabilitiesManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	return &manifest, nil
}

func fetchFleet(client *http.Client, baseURL, path, token string) (*fleetContext, error) {
	body, err := fetchAuthenticatedRaw(client, baseURL+path, token)
	if err != nil {
		return nil, err
	}
	var fleet fleetContext
	if err := json.Unmarshal(body, &fleet); err != nil {
		return nil, fmt.Errorf("decode fleet: %w", err)
	}
	return &fleet, nil
}

// fetchAuthenticatedRaw is the shared GET helper for any
// authenticated agent-surface endpoint. Branches on the stable
// error envelope when a non-2xx comes back so the caller sees the
// agent-stable code, not just an HTTP status.
func fetchAuthenticatedRaw(client *http.Client, fullURL, token string) ([]byte, error) {
	parsed, err := url.Parse(fullURL)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", fullURL, err)
	}
	req, err := http.NewRequest(http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-API-Token", token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", parsed.Path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return body, nil
	}
	// Non-2xx: try to surface the stable error code from the
	// envelope. Unauthenticated rejection comes through as plain
	// text from the auth middleware (not the envelope), so fall back
	// to the body verbatim if the JSON decode fails.
	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Error != "" {
		return nil, fmt.Errorf("GET %s: %d %s (%s)", parsed.Path, resp.StatusCode, env.Error, env.Message)
	}
	return nil, fmt.Errorf("GET %s: %d %s", parsed.Path, resp.StatusCode, strings.TrimSpace(string(body)))
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
// non-keepalive event payload, prints it, and returns. SSE is line-
// based with empty lines as record separators; this implementation
// is deliberately minimal — a few hundred bytes of stdlib code is
// enough to consume the substrate's push channel.
func readOneSSEEvent(ctx context.Context, fullURL, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Token", token)
	req.Header.Set("Accept", "text/event-stream")
	// No timeout on this client — context cancels the read.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("subscribe: status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)

	var event, data string
	skippedConnected := false
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case line == "":
			// End of event record. The first event the server
			// always emits is "stream.connected" — skip it once so
			// the probe surfaces the first *real* event (or a
			// heartbeat).
			if event == "stream.connected" && !skippedConnected {
				skippedConnected = true
				event, data = "", ""
				continue
			}
			if event != "" || data != "" {
				fmt.Printf("  event: %s\n  data: %s\n", event, data)
				return nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return ctx.Err()
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "agent-probe: "+format+"\n", args...)
	os.Exit(1)
}
