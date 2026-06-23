// Command pulse-mcp is the Model Context Protocol adapter for
// Pulse Intelligence. It exposes the same governed operations
// substrate that Pulse Assistant uses natively, but for
// stdio-speaking external agents like OpenCode, Claude Desktop,
// Claude Code, and other MCP clients. It is deliberately a thin
// translation layer: every MCP tool here is a one-line projection
// of an entry in Pulse's canonical capabilities manifest.
//
// Usage (common mcpServers-style config entry):
//
//	{
//	  "mcpServers": {
//	    "pulse": {
//	      "command": "pulse-mcp",
//	      "args": ["--base-url", "http://localhost:7655"],
//	      "env": { "PULSE_API_TOKEN": "..." }
//	    }
//	  }
//	}
//
// Wire framing: line-delimited JSON-RPC 2.0 on stdio. Logs to
// stderr so the JSON-RPC channel on stdout stays clean.
//
// What it does:
//
//  1. Fetches /api/agent/capabilities from the configured Pulse
//     instance at startup. The manifest is the single source of
//     truth — adding a request/response capability to the Pulse MCP
//     surface tool contract automatically extends the MCP tool
//     surface here, no MCP-side changes required.
//
//  2. Translates each capability into an MCP tool with:
//     - tool name = capability name (snake_case agent identifier)
//     - description = capability description plus manifest metadata
//     (route, scope, action mode, approval policy, shapes, errors)
//     - inputSchema = the manifest-owned JSON Schema when declared,
//     with a path/body fallback for capabilities that have not yet
//     authored a stricter schema.
//
//  3. Handles the MCP JSON-RPC methods agents actually call:
//     initialize, tools/list, tools/call, resources/list,
//     resources/read, prompts/list, and prompts/get. Each tools/call
//     resolves the manifest entry by name, substitutes path
//     params, makes the HTTP request to Pulse with the configured
//     token, and returns the JSON response (or stable error
//     envelope) as a text content block.
//
// What stays out of the request/response tool projection:
//
//   - subscribe_events as a callable tool. SSE streaming doesn't
//     fit the MCP request/response shape, so --emit-notifications
//     bridges it into JSON-RPC notifications when an agent needs
//     real-time push.
//
// MCP resources are also manifest-backed: resources/list uses the
// canonical fleet context capability, and resources/read uses the
// canonical per-resource context capability. The resource URI shape
// and capability projection live in internal/agentcapabilities so
// this adapter does not grow a second inventory model.
//
// MCP prompts are manifest-backed workflow hints over that same
// tool/resource surface. The prompt catalog and prompt rendering live
// in internal/agentcapabilities so this adapter does not grow local
// operational playbooks.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

type agentCapability = agentcapabilities.Capability
type agentCapabilitiesManifest = agentcapabilities.Manifest

type jsonRPCRequest = agentcapabilities.JSONRPCRequest

const (
	pulseMCPServerName    = "pulse-mcp"
	pulseMCPServerVersion = "0.1.0"
)

func main() {
	baseURL := flag.String("base-url", agentcapabilities.DefaultMCPAdapterDefaultBaseURL, "Pulse base URL")
	tokenEnv := flag.String("token-env", agentcapabilities.DefaultMCPAdapterTokenEnv, "Env var holding the Pulse API token")
	emitNotifications := flag.Bool("emit-notifications", false, "Translate Pulse SSE events into JSON-RPC notifications on stdout. Off by default because not every MCP client surfaces server-initiated notifications; enable when wiring an autonomous agent that processes the JSON-RPC stream.")
	flag.Parse()

	log.SetOutput(os.Stderr)
	log.SetPrefix("pulse-mcp ")
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	manifestClient := &http.Client{Timeout: 10 * time.Second}
	manifest, err := agentcapabilities.FetchManifest(context.Background(), manifestClient, *baseURL)
	if err != nil {
		log.Fatalf("could not fetch capabilities manifest from %s: %v", *baseURL, err)
	}
	log.Printf("fetched manifest %s with %d capabilities from %s", manifest.Version, len(manifest.Capabilities), *baseURL)

	token := strings.TrimSpace(os.Getenv(*tokenEnv))
	if token == "" {
		log.Fatal(missingAPITokenMessage(*tokenEnv, manifest))
	}

	server := &mcpServer{
		baseURL:           *baseURL,
		token:             token,
		manifest:          manifest,
		http:              &http.Client{Timeout: 30 * time.Second},
		emitNotifications: *emitNotifications,
	}
	server.serve(os.Stdin, os.Stdout)
}

func missingAPITokenMessage(tokenEnv string, manifest *agentCapabilitiesManifest) string {
	scopeList := agentcapabilities.ManifestRequiredScopeList(manifest)
	if scopeList == "" {
		return fmt.Sprintf("env var %s is empty; pulse-mcp needs an API token for the manifest tools you enable", tokenEnv)
	}
	return fmt.Sprintf("env var %s is empty; pulse-mcp needs an API token. Mint one with the manifest scopes for the tools you enable (current manifest scopes: %s)", tokenEnv, scopeList)
}

// mcpServer holds the per-process state: the configured Pulse base
// URL and token, the manifest fetched at startup, and the HTTP
// client used to call Pulse.
type mcpServer struct {
	baseURL           string
	token             string
	manifest          *agentCapabilitiesManifest
	http              agentcapabilities.HTTPDoer
	mu                sync.Mutex // guards stdout writes
	emitNotifications bool
	// notificationsOnce ensures the SSE consumer goroutine starts
	// at most once per process — `initialize` may be called more
	// than once if a client reconnects, but we only need one
	// consumer per stdio session.
	notificationsOnce sync.Once
	// out is the writer used for both responses and notifications.
	// Captured the first time `serve` is called so the SSE consumer
	// can write notifications to the same channel without an extra
	// argument-passing dance.
	out io.Writer
}

type pulseMCPAgentSurfaceHTTPDoer struct {
	next agentcapabilities.HTTPDoer
}

func (d pulseMCPAgentSurfaceHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	if req != nil {
		req = req.Clone(req.Context())
		req.Header = req.Header.Clone()
		req.Header.Set(agentcapabilities.AgentSurfaceHeader, agentcapabilities.AgentSurfacePulseMCP)
	}
	next := d.next
	if next == nil {
		next = http.DefaultClient
	}
	return next.Do(req)
}

// serve is the stdio loop: read line-delimited JSON-RPC requests
// from `in`, dispatch, write responses to `out`. Each request is on
// its own line; blank lines are ignored; EOF stops the server.
func (s *mcpServer) serve(in io.Reader, out io.Writer) {
	s.out = out
	if err := agentcapabilities.ServeJSONRPCLines(
		context.Background(),
		in,
		func(ctx context.Context, req agentcapabilities.JSONRPCRequest) agentcapabilities.JSONRPCResponse {
			return s.dispatch(ctx, &req)
		},
		func(resp agentcapabilities.JSONRPCResponse) error {
			return s.writeJSON(out, resp)
		},
	); err != nil {
		log.Printf("stdio JSON-RPC: %v", err)
	}
}

// dispatch delegates MCP method semantics to the shared Pulse Intelligence
// boundary. This binary owns session policy and stdout serialization only.
func (s *mcpServer) dispatch(ctx context.Context, req *jsonRPCRequest) agentcapabilities.JSONRPCResponse {
	return agentcapabilities.DispatchMCPToolServerRequest(ctx, *req, s.manifestToolServer().Handlers(func() {
		// Start the SSE-to-notifications bridge once per process,
		// only if the operator opted in. Spawned after the
		// initialize response is queued so the client sees the
		// handshake reply before any notification arrives.
		if s.emitNotifications {
			s.notificationsOnce.Do(func() {
				go s.consumeSSEEvents(context.Background())
			})
		}
	}))
}

func (s *mcpServer) manifestToolServer() agentcapabilities.MCPManifestToolServer {
	var manifest agentcapabilities.Manifest
	if s.manifest != nil {
		manifest = *s.manifest
	}
	return agentcapabilities.MCPManifestToolServer{
		ServerName:                   pulseMCPServerName,
		ServerVersion:                pulseMCPServerVersion,
		SurfaceID:                    agentcapabilities.SurfaceIDPulseMCP,
		EmitPulseNotifications:       s.emitNotifications,
		Client:                       s.pulseMCPHTTPClient(),
		BaseURL:                      s.baseURL,
		Token:                        s.token,
		Manifest:                     manifest,
		RecordWorkflowPromptActivity: s.recordWorkflowPromptActivity,
	}
}

func (s *mcpServer) pulseMCPHTTPClient() agentcapabilities.HTTPDoer {
	if s == nil {
		return pulseMCPAgentSurfaceHTTPDoer{}
	}
	return pulseMCPAgentSurfaceHTTPDoer{next: s.http}
}

func (s *mcpServer) recordWorkflowPromptActivity(ctx context.Context, promptName string) {
	promptName = strings.TrimSpace(promptName)
	if s == nil || promptName == "" || strings.TrimSpace(s.baseURL) == "" || strings.TrimSpace(s.token) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	body, err := json.Marshal(map[string]string{"name": promptName})
	if err != nil {
		log.Printf("workflow prompt activity: marshal: %v", err)
		return
	}
	req, err := agentcapabilities.NewAgentHTTPRequest(
		ctx,
		http.MethodPost,
		s.baseURL,
		agentcapabilities.AgentWorkflowPromptActivityPath,
		s.token,
		bytes.NewReader(body),
	)
	if err != nil {
		log.Printf("workflow prompt activity: %v", err)
		return
	}
	resp, err := s.pulseMCPHTTPClient().Do(req)
	if err != nil {
		log.Printf("workflow prompt activity: %v", err)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("workflow prompt activity: status %d", resp.StatusCode)
	}
}

func (s *mcpServer) writeJSON(out io.Writer, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := agentcapabilities.WriteJSONRPCMessage(out, v); err != nil {
		log.Printf("encode response: %v", err)
		return err
	}
	return nil
}

// consumeSSEEvents opens a long-lived SSE connection to Pulse's
// /api/agent/events stream and translates each non-keepalive event
// into a JSON-RPC notification on stdout. Notifications use the
// MCP convention `notifications/<event-kind>` (e.g.
// `notifications/finding.created`) so MCP clients that route on
// method names can dispatch directly.
//
// The function is meant to run for the lifetime of the stdio
// session; it returns only on context cancellation or unrecoverable
// stream errors. On a recoverable error (network blip, server
// restart), it backs off briefly and reconnects so the bridge
// stays up across the kinds of stream stalls the MCP server's
// idle-tolerance budget already accepts on the substrate side.
func (s *mcpServer) consumeSSEEvents(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		if err := s.streamSSEOnce(ctx, agentcapabilities.AgentEventsPath); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("SSE bridge: %v (reconnecting in %s)", err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		// Clean exit (e.g. server closed the stream); reset the
		// backoff so the next reconnect is immediate.
		backoff = time.Second
	}
}

// streamSSEOnce opens one SSE connection and reads events until
// the connection drops or the context is cancelled. The shared
// Pulse Intelligence core owns SSE-to-MCP notification projection;
// the adapter only serializes projected notifications to stdout.
func (s *mcpServer) streamSSEOnce(ctx context.Context, path string) error {
	// Bypass the finite s.http timeout: the SSE stream is long-lived and ctx is
	// the right cancellation signal. The wrapper still marks the adapter surface.
	return agentcapabilities.StreamMCPEventNotifications(ctx, pulseMCPAgentSurfaceHTTPDoer{}, s.baseURL, path, s.token, func(notification agentcapabilities.JSONRPCRequest) error {
		return s.writeJSON(s.out, notification)
	})
}
