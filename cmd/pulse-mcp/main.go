// Command pulse-mcp is a minimal MCP (Model Context Protocol)
// adapter that wraps Pulse's agent surface for stdio-speaking
// clients like Claude Desktop and Claude Code. It is the
// translation layer the substrate was designed to make cheap:
// every MCP tool here is a one-line projection of an entry in
// Pulse's hand-authored capabilities manifest.
//
// Usage (typical Claude Desktop config entry):
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
//     truth — adding a capability there automatically extends the
//     MCP tool surface here, no MCP-side changes required.
//
//  2. Translates each capability into an MCP tool with:
//     - tool name = capability name (snake_case agent identifier)
//     - description = capability description
//     - inputSchema = derived from path placeholders + body shape:
//     {resourceId} segments become required string properties;
//     non-GET tools accept a free-form `body` object.
//
//  3. Handles the MCP JSON-RPC methods Claude actually calls:
//     initialize, tools/list, tools/call. Each tools/call
//     resolves the manifest entry by name, substitutes path
//     params, makes the HTTP request to Pulse with the configured
//     token, and returns the JSON response (or stable error
//     envelope) as a text content block.
//
// What it does not do (yet):
//
//   - subscribe_events. SSE streaming doesn't fit the MCP tool
//     shape; it would be an MCP "notification" or a long-running
//     tool. Future slices can layer this on; agents that need
//     real-time push consume the SSE stream directly.
//
//   - Resource URIs. MCP supports `resources/list`/`resources/read`
//     in addition to tools, but for Pulse the tool-only model is
//     sufficient and keeps the adapter small. A future slice can
//     project resource-context bundles as MCP resources if the
//     UX value is clear.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// agentCapability mirrors Pulse's manifest wire shape — defined
// inline so the adapter depends on nothing in the pulse module.
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

// jsonRPCRequest is the JSON-RPC 2.0 request envelope. Method is
// the MCP method name (e.g. "tools/list"); params is method-
// specific. ID is null for notifications, otherwise echoed back on
// the response.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MCP tool shape. inputSchema is JSON Schema (draft-07-ish) that
// the agent uses to validate before calling.
type mcpTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// pathPlaceholderRE matches `{paramName}` in capability paths.
var pathPlaceholderRE = regexp.MustCompile(`\{([a-zA-Z][a-zA-Z0-9]*)\}`)

func main() {
	baseURL := flag.String("base-url", "http://localhost:7655", "Pulse base URL")
	tokenEnv := flag.String("token-env", "PULSE_API_TOKEN", "Env var holding the Pulse API token")
	flag.Parse()

	log.SetOutput(os.Stderr)
	log.SetPrefix("pulse-mcp ")
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	token := strings.TrimSpace(os.Getenv(*tokenEnv))
	if token == "" {
		log.Fatalf("env var %s is empty; pulse-mcp needs an API token with monitoring:read scope (and monitoring:write for set/clear operator-state)", *tokenEnv)
	}

	manifest, err := fetchManifest(*baseURL)
	if err != nil {
		log.Fatalf("could not fetch capabilities manifest from %s: %v", *baseURL, err)
	}
	log.Printf("fetched manifest %s with %d capabilities from %s", manifest.Version, len(manifest.Capabilities), *baseURL)

	server := &mcpServer{
		baseURL:  *baseURL,
		token:    token,
		manifest: manifest,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
	server.serve(os.Stdin, os.Stdout)
}

// mcpServer holds the per-process state: the configured Pulse base
// URL and token, the manifest fetched at startup, and the HTTP
// client used to call Pulse.
type mcpServer struct {
	baseURL  string
	token    string
	manifest *agentCapabilitiesManifest
	http     *http.Client
	mu       sync.Mutex // guards stdout writes
}

// serve is the stdio loop: read line-delimited JSON-RPC requests
// from `in`, dispatch, write responses to `out`. Each request is on
// its own line; blank lines are ignored; EOF stops the server.
func (s *mcpServer) serve(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 1<<22) // up to 4 MB per message
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.writeJSON(out, jsonRPCResponse{
				JSONRPC: "2.0",
				Error: &jsonRPCError{
					Code:    -32700, // Parse error
					Message: fmt.Sprintf("malformed JSON-RPC request: %v", err),
				},
			})
			continue
		}
		resp := s.dispatch(context.Background(), &req)
		// Notifications (id is null/absent) get no response.
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue
		}
		s.writeJSON(out, resp)
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		log.Printf("stdio scanner: %v", err)
	}
}

// dispatch routes one JSON-RPC request to the right handler. The
// MCP methods we support are minimal: initialize, tools/list,
// tools/call. Anything else gets method-not-found per JSON-RPC.
func (s *mcpServer) dispatch(ctx context.Context, req *jsonRPCRequest) jsonRPCResponse {
	resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = s.handleInitialize()
	case "tools/list":
		resp.Result = s.handleToolsList()
	case "tools/call":
		result, err := s.handleToolsCall(ctx, req.Params)
		if err != nil {
			resp.Error = &jsonRPCError{Code: -32603, Message: err.Error()}
		} else {
			resp.Result = result
		}
	case "ping":
		resp.Result = map[string]any{}
	default:
		resp.Error = &jsonRPCError{
			Code:    -32601,
			Message: fmt.Sprintf("method not found: %s", req.Method),
		}
	}
	return resp
}

// handleInitialize returns the MCP server's capabilities. Tools
// are the only category we expose; resources, prompts, and
// sampling are intentionally not advertised.
func (s *mcpServer) handleInitialize() any {
	return map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "pulse-mcp",
			"version": "0.1.0",
		},
	}
}

// handleToolsList projects each manifest capability into an MCP
// tool. subscribe_events is filtered out — SSE streaming doesn't
// fit the request/response tool shape.
func (s *mcpServer) handleToolsList() any {
	tools := make([]mcpTool, 0, len(s.manifest.Capabilities))
	for _, cap := range s.manifest.Capabilities {
		if cap.Name == "subscribe_events" {
			continue
		}
		schema := buildInputSchema(cap)
		tools = append(tools, mcpTool{
			Name:        cap.Name,
			Description: cap.Description,
			InputSchema: schema,
		})
	}
	return map[string]any{"tools": tools}
}

// handleToolsCall executes one tool invocation. params is shaped
// `{"name": "...", "arguments": {...}}`. The tool name resolves
// to a manifest capability; arguments fill path placeholders and
// (for non-GET tools) the request body.
func (s *mcpServer) handleToolsCall(ctx context.Context, raw json.RawMessage) (any, error) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("decode tools/call params: %w", err)
	}
	cap, ok := s.findCapability(params.Name)
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", params.Name)
	}

	url, err := substitutePathParams(cap.Path, params.Arguments)
	if err != nil {
		return nil, fmt.Errorf("substitute path params: %w", err)
	}

	var body io.Reader
	if cap.Method != http.MethodGet && cap.Method != http.MethodDelete {
		// Non-GET/DELETE tools accept a `body` argument that's
		// JSON-encoded as the request body. If absent, we send an
		// empty object — that's fine for the finding-action
		// capabilities that just need `{ "finding_id": "..." }`.
		bodyArg, ok := params.Arguments["body"]
		if !ok {
			// Some capabilities take their body fields at the top
			// level (no nested "body" key). Treat the whole
			// arguments object as the body, minus any consumed
			// path-placeholder keys.
			pathParams := pathParamSet(cap.Path)
			cleaned := map[string]any{}
			for k, v := range params.Arguments {
				if !pathParams[k] {
					cleaned[k] = v
				}
			}
			bodyArg = cleaned
		}
		buf, err := json.Marshal(bodyArg)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		body = bytes.NewReader(buf)
	}

	httpReq, err := http.NewRequestWithContext(ctx, cap.Method, s.baseURL+url, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("X-API-Token", s.token)
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call Pulse: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Build the MCP content result. The substrate's stable error
	// envelope ({"error": "code", "message": "..."}) is preserved
	// verbatim — agents on the MCP side branch on the same code
	// they would branching on the HTTP response.
	text := string(respBody)
	isError := resp.StatusCode < 200 || resp.StatusCode >= 300
	return map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": text,
			},
		},
		"isError": isError,
	}, nil
}

func (s *mcpServer) findCapability(name string) (agentCapability, bool) {
	for _, c := range s.manifest.Capabilities {
		if c.Name == name {
			return c, true
		}
	}
	return agentCapability{}, false
}

func (s *mcpServer) writeJSON(out io.Writer, v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// buildInputSchema generates a permissive JSON Schema for a
// capability. Path placeholders become required string properties;
// non-GET/DELETE capabilities also accept a free-form `body`
// object. The schema is permissive on purpose — the manifest is
// the source of truth for what the underlying endpoint accepts;
// MCP just needs enough shape so the agent knows which fields to
// pass.
func buildInputSchema(cap agentCapability) json.RawMessage {
	properties := map[string]any{}
	required := []string{}
	for _, m := range pathPlaceholderRE.FindAllStringSubmatch(cap.Path, -1) {
		name := m[1]
		properties[name] = map[string]any{
			"type":        "string",
			"description": "Canonical " + name + " (e.g. \"vm:101\", \"container:web-1\")",
		}
		required = append(required, name)
	}
	if cap.Method != http.MethodGet && cap.Method != http.MethodDelete {
		desc := "Request body fields"
		if cap.RequestBodyShape != "" {
			desc = "Request body fields. Shape hint: " + cap.RequestBodyShape
		}
		properties["body"] = map[string]any{
			"type":                 "object",
			"description":          desc,
			"additionalProperties": true,
		}
	}
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": true,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	out, _ := json.Marshal(schema)
	return out
}

// substitutePathParams replaces `{name}` segments in a capability's
// path with the corresponding argument value. Missing args for
// declared placeholders are an error so the agent gets a stable
// failure rather than an HTTP 404 on a malformed URL.
func substitutePathParams(path string, args map[string]any) (string, error) {
	var missing []string
	out := pathPlaceholderRE.ReplaceAllStringFunc(path, func(match string) string {
		name := match[1 : len(match)-1] // strip { and }
		v, ok := args[name]
		if !ok {
			missing = append(missing, name)
			return match
		}
		s, ok := v.(string)
		if !ok {
			missing = append(missing, name+" (not a string)")
			return match
		}
		return s
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("missing path argument(s): %s", strings.Join(missing, ", "))
	}
	return out, nil
}

// pathParamSet returns the set of placeholder names declared in a
// path. Used to filter path args out of the request body when a
// caller passes everything at the top level.
func pathParamSet(path string) map[string]bool {
	set := map[string]bool{}
	for _, m := range pathPlaceholderRE.FindAllStringSubmatch(path, -1) {
		set[m[1]] = true
	}
	return set
}

// fetchManifest pulls the capabilities manifest from Pulse. This
// is the only call the adapter makes before its first tool
// invocation; the manifest is not cached or refreshed during the
// process lifetime — restart pulse-mcp to pick up new
// capabilities.
func fetchManifest(baseURL string) (*agentCapabilitiesManifest, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/agent/capabilities", nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest GET returned %d", resp.StatusCode)
	}
	var m agentCapabilitiesManifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	return &m, nil
}
