package agentcapabilities

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
)

const (
	JSONRPCVersion = "2.0"

	JSONRPCErrorParse          = -32700
	JSONRPCErrorInvalidRequest = -32600
	JSONRPCErrorMethodNotFound = -32601
	JSONRPCErrorInvalidParams  = -32602
	JSONRPCErrorInternal       = -32603

	MCPProtocolVersion = "2025-06-18"

	MCPMethodInitialize    = "initialize"
	MCPMethodToolsList     = "tools/list"
	MCPMethodToolsCall     = "tools/call"
	MCPMethodResourcesList = "resources/list"
	MCPMethodResourcesRead = "resources/read"
	MCPMethodPromptsList   = "prompts/list"
	MCPMethodPromptsGet    = "prompts/get"
	MCPMethodPing          = "ping"

	MCPNotificationPrefix = "notifications/"

	MCPPulseNotificationsExperimentalKey = "pulseNotifications"

	MCPResourceContextURIHost  = "resource"
	MCPResourceContextMIMEType = "application/json"
)

// JSONRPCRequest is the shared JSON-RPC 2.0 request envelope used by Pulse's
// MCP transport surfaces. ID stays raw so adapters can echo numeric, string, or
// structured client ids without interpreting them.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse is the shared JSON-RPC 2.0 response envelope used by Pulse's
// MCP transport surfaces.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError is the shared JSON-RPC 2.0 error envelope.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// DecodeJSONRPCRequest parses one JSON-RPC request payload with the stable
// malformed-request message used by Pulse MCP adapters.
func DecodeJSONRPCRequest(raw []byte) (JSONRPCRequest, error) {
	var req JSONRPCRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return JSONRPCRequest{}, fmt.Errorf("malformed JSON-RPC request: %w", err)
	}
	return req, nil
}

// NewJSONRPCParseErrorResponse builds the shared parse-error response for
// request payloads that could not be decoded far enough to recover an id.
func NewJSONRPCParseErrorResponse(err error) JSONRPCResponse {
	message := "malformed JSON-RPC request"
	if err != nil {
		message = err.Error()
	}
	return NewJSONRPCErrorResponse(nil, JSONRPCErrorParse, message, nil)
}

// JSONRPCRequestExpectsResponse reports whether a request has a non-null id.
// JSON-RPC notifications deliberately receive no response.
func JSONRPCRequestExpectsResponse(req JSONRPCRequest) bool {
	id := bytes.TrimSpace(req.ID)
	return len(id) > 0 && !bytes.Equal(id, []byte("null"))
}

// JSONRPCLineDispatcher handles one decoded JSON-RPC request.
type JSONRPCLineDispatcher func(context.Context, JSONRPCRequest) JSONRPCResponse

// JSONRPCResponseWriter writes one JSON-RPC response envelope.
type JSONRPCResponseWriter func(JSONRPCResponse) error

// ServeJSONRPCLines scans a line-delimited JSON-RPC stream, decodes each
// request, writes parse errors with the shared envelope, dispatches valid
// requests, and suppresses responses for JSON-RPC notifications. Transport
// adapters own session policy and response serialization; request framing,
// parse-error mapping, and notification response rules stay here.
func ServeJSONRPCLines(ctx context.Context, reader io.Reader, dispatch JSONRPCLineDispatcher, write JSONRPCResponseWriter) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1<<22)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		req, err := DecodeJSONRPCRequest([]byte(line))
		if err != nil {
			if write != nil {
				if writeErr := write(NewJSONRPCParseErrorResponse(err)); writeErr != nil {
					return writeErr
				}
			}
			continue
		}
		if dispatch == nil {
			continue
		}
		resp := dispatch(ctx, req)
		if !JSONRPCRequestExpectsResponse(req) {
			continue
		}
		if write != nil {
			if err := write(resp); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return err
	}
	return nil
}

// WriteJSONRPCMessage serializes a JSON-RPC envelope or notification with
// stable encoder settings used by Pulse MCP transports.
func WriteJSONRPCMessage(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// MCPServerInfo describes the Pulse MCP server during initialize.
type MCPServerInfo struct {
	Name         string           `json:"name"`
	Version      string           `json:"version"`
	Capabilities *MCPCapabilities `json:"capabilities,omitempty"`
}

// MCPCapabilities describes capabilities advertised during initialize. The
// experimental map is deliberately flexible because MCP clients use
// implementation-specific extension keys for server-initiated notifications.
type MCPCapabilities struct {
	Tools        *MCPToolsCapability     `json:"tools,omitempty"`
	Resources    *MCPResourcesCapability `json:"resources,omitempty"`
	Prompts      *MCPPromptsCapability   `json:"prompts,omitempty"`
	Experimental map[string]any          `json:"experimental,omitempty"`
}

// MCPToolsCapability describes tool support.
type MCPToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCPResourcesCapability describes resource support.
type MCPResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCPPromptsCapability describes prompt support.
type MCPPromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCPPulseNotificationsCapability is Pulse's namespaced initialize extension
// for adapters that translate Pulse Intelligence events into JSON-RPC
// notifications.
type MCPPulseNotificationsCapability struct {
	Kinds []string `json:"kinds"`
}

// MCPInitializeParams are the params for the initialize method.
type MCPInitializeParams struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    MCPCapabilities `json:"capabilities"`
	ClientInfo      MCPClientInfo   `json:"clientInfo"`
}

// MCPClientInfo describes the MCP client.
type MCPClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPInitializeResult is the result for the initialize method.
type MCPInitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    MCPCapabilities `json:"capabilities"`
	ServerInfo      MCPServerInfo   `json:"serverInfo"`
	Instructions    string          `json:"instructions,omitempty"`
}

// NewMCPToolServerInitializeResult builds the shared initialize response for a
// Pulse MCP server that exposes tools and optionally advertises the Pulse
// Intelligence notification bridge.
func NewMCPToolServerInitializeResult(serverName, serverVersion string, emitPulseNotifications bool) MCPInitializeResult {
	return newMCPToolServerInitializeResult(serverName, serverVersion, emitPulseNotifications, true, false, false, false, SurfaceIDPulseMCP, SurfaceContract{})
}

// NewMCPManifestToolServerInitializeResult builds the shared initialize
// response for a manifest-backed Pulse MCP server. Resource support is
// advertised only when the manifest contains the canonical fleet and
// per-resource context capabilities needed to satisfy resources/list/read.
func NewMCPManifestToolServerInitializeResult(serverName, serverVersion string, emitPulseNotifications bool, manifest Manifest) MCPInitializeResult {
	return NewMCPManifestSurfaceToolServerInitializeResult(serverName, serverVersion, emitPulseNotifications, manifest, SurfaceIDPulseMCP)
}

// NewMCPManifestSurfaceToolServerInitializeResult builds the shared initialize
// response for a manifest-backed MCP server surface. The manifest's published
// surface tool contract decides whether request/response tools may be
// advertised; surface-aware resource and prompt helpers decide whether those
// affordances can be satisfied for this manifest snapshot.
func NewMCPManifestSurfaceToolServerInitializeResult(serverName, serverVersion string, emitPulseNotifications bool, manifest Manifest, surfaceID string) MCPInitializeResult {
	surfaceID = normalizeMCPManifestSurfaceID(surfaceID)
	affordances, _ := ManifestSurfaceAffordances(manifest, surfaceID)
	_, hasSurfaceToolContract := ResolveManifestSurfaceToolContract(manifest, surfaceID)
	return newMCPToolServerInitializeResult(
		serverName,
		serverVersion,
		emitPulseNotifications,
		affordances.Tools && hasSurfaceToolContract,
		MCPManifestResourceProjectionSupported(manifest, surfaceID),
		MCPManifestSurfacePromptProjectionSupported(manifest, surfaceID),
		affordances.CapabilityMetadata,
		surfaceID,
		manifest.SurfaceContract,
	)
}

func newMCPToolServerInitializeResult(serverName, serverVersion string, emitPulseNotifications, exposeTools, exposeResources, exposePrompts, exposeCapabilityMetadata bool, surfaceID string, surfaceContract SurfaceContract) MCPInitializeResult {
	caps := MCPCapabilities{}
	if exposeTools {
		caps.Tools = &MCPToolsCapability{}
	}
	if exposeResources {
		caps.Resources = &MCPResourcesCapability{}
	}
	if exposePrompts {
		caps.Prompts = &MCPPromptsCapability{}
	}
	if emitPulseNotifications {
		caps.Experimental = map[string]any{
			MCPPulseNotificationsExperimentalKey: MCPPulseNotificationsCapability{
				Kinds: AgentActionableEventKinds(),
			},
		}
	}
	return MCPInitializeResult{
		ProtocolVersion: MCPProtocolVersion,
		Capabilities:    caps,
		ServerInfo: MCPServerInfo{
			Name:    serverName,
			Version: serverVersion,
		},
		Instructions: BuildPulseMCPOperatingInstructions(PulseMCPOperatingInstructionOptions{
			SurfaceID:                  surfaceID,
			SupportsTools:              exposeTools,
			SupportsResources:          exposeResources,
			SupportsPrompts:            exposePrompts,
			SupportsCapabilityMetadata: exposeCapabilityMetadata,
			SurfaceContract:            surfaceContract,
		}),
	}
}

// MCPListToolsResult is the tools/list result shape for structured in-process
// registry tools.
type MCPListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// MCPProjectedToolsResult is the tools/list result shape for external-agent
// adapters that project the canonical capabilities manifest.
type MCPProjectedToolsResult struct {
	Tools []ProjectedTool `json:"tools"`
}

// MCPCallToolParams is the protocol-facing compatibility name for the shared
// tool-call params envelope used by MCP tools/call.
type MCPCallToolParams = ToolCallParams

// MCPToolAnnotations is the protocol-facing compatibility name for the shared
// external-agent tool behavior hints projected into MCP tools/list.
type MCPToolAnnotations = ToolBehaviorHints

// MCPToolServerHandlers are the adapter-provided actions behind the shared MCP
// JSON-RPC method dispatcher. Transport/session concerns stay in the adapter;
// method semantics, result envelopes, and error mapping stay here.
type MCPToolServerHandlers struct {
	Initialize    func() MCPInitializeResult
	ToolsList     func() MCPProjectedToolsResult
	ToolsCall     func(context.Context, json.RawMessage) (MCPToolResult, error)
	ResourcesList func(context.Context) (MCPListResourcesResult, error)
	ResourcesRead func(context.Context, json.RawMessage) (MCPReadResourceResult, error)
	PromptsList   func() MCPListPromptsResult
	PromptsGet    func(context.Context, json.RawMessage) (MCPGetPromptResult, error)
	OnInitialize  func()
}

// MCPManifestToolServer is the shared manifest-backed MCP tool surface used by
// adapters that expose Pulse Intelligence capabilities as request/response MCP
// tools. Adapters provide transport/session policy; this type owns initialize,
// tools/list, and tools/call semantics over the canonical manifest.
type MCPManifestToolServer struct {
	ServerName                   string
	ServerVersion                string
	SurfaceID                    string
	EmitPulseNotifications       bool
	Client                       HTTPDoer
	BaseURL                      string
	Token                        string
	Manifest                     Manifest
	RecordWorkflowPromptActivity func(context.Context, string)
}

// Handlers returns the shared MCP dispatcher callbacks for this manifest-backed
// tool server.
func (s MCPManifestToolServer) Handlers(onInitialize func()) MCPToolServerHandlers {
	return MCPToolServerHandlers{
		Initialize: func() MCPInitializeResult {
			return s.Initialize()
		},
		ToolsList: func() MCPProjectedToolsResult {
			return s.ToolsList()
		},
		ToolsCall: func(ctx context.Context, raw json.RawMessage) (MCPToolResult, error) {
			return s.ToolsCall(ctx, raw)
		},
		ResourcesList: func(ctx context.Context) (MCPListResourcesResult, error) {
			return s.ResourcesList(ctx)
		},
		ResourcesRead: func(ctx context.Context, raw json.RawMessage) (MCPReadResourceResult, error) {
			return s.ResourcesRead(ctx, raw)
		},
		PromptsList: func() MCPListPromptsResult {
			return s.PromptsList()
		},
		PromptsGet: func(ctx context.Context, raw json.RawMessage) (MCPGetPromptResult, error) {
			return s.PromptsGet(ctx, raw)
		},
		OnInitialize: onInitialize,
	}
}

// Initialize returns the shared Pulse MCP initialize result.
func (s MCPManifestToolServer) Initialize() MCPInitializeResult {
	return NewMCPManifestSurfaceToolServerInitializeResult(s.ServerName, s.ServerVersion, s.EmitPulseNotifications, s.Manifest, s.surfaceID())
}

// ToolsList projects the canonical capabilities manifest into MCP tools.
func (s MCPManifestToolServer) ToolsList() MCPProjectedToolsResult {
	if !s.surfaceAffordances().Tools {
		return MCPProjectedToolsResult{Tools: []ProjectedTool{}}
	}
	return MCPProjectedToolsResult{Tools: ProjectManifestSurfaceTools(s.Manifest, s.surfaceID())}
}

// ToolsCall executes one manifest-backed MCP tool invocation.
func (s MCPManifestToolServer) ToolsCall(ctx context.Context, raw json.RawMessage) (MCPToolResult, error) {
	return ExecuteMCPManifestSurfaceToolHTTP(ctx, s.Client, s.BaseURL, s.Token, s.Manifest, s.surfaceID(), raw)
}

// ResourcesList projects the canonical fleet-context capability into MCP
// resource descriptors.
func (s MCPManifestToolServer) ResourcesList(ctx context.Context) (MCPListResourcesResult, error) {
	return ListMCPManifestSurfaceResourcesHTTP(ctx, s.Client, s.BaseURL, s.Token, s.Manifest, s.surfaceID())
}

// ResourcesRead projects a single MCP resource URI onto the canonical
// per-resource context capability.
func (s MCPManifestToolServer) ResourcesRead(ctx context.Context, raw json.RawMessage) (MCPReadResourceResult, error) {
	return ReadMCPManifestSurfaceResourceHTTP(ctx, s.Client, s.BaseURL, s.Token, s.Manifest, s.surfaceID(), raw)
}

// PromptsList projects manifest-backed Pulse workflow prompts.
func (s MCPManifestToolServer) PromptsList() MCPListPromptsResult {
	if !MCPManifestSurfacePromptProjectionSupported(s.Manifest, s.surfaceID()) {
		return MCPListPromptsResult{Prompts: []MCPPrompt{}}
	}
	return (MCPListPromptsResult{Prompts: ProjectMCPWorkflowPrompts(ManifestPulseWorkflowPrompts(s.Manifest))}).NormalizeCollections()
}

// PromptsGet returns one manifest-backed Pulse workflow prompt.
func (s MCPManifestToolServer) PromptsGet(ctx context.Context, raw json.RawMessage) (MCPGetPromptResult, error) {
	result, err := GetMCPPromptFromManifestSurface(s.Manifest, s.surfaceID(), raw)
	if err != nil {
		return MCPGetPromptResult{}, err
	}
	if s.RecordWorkflowPromptActivity != nil {
		if params, decodeErr := DecodeMCPGetPromptParams(raw); decodeErr == nil {
			s.RecordWorkflowPromptActivity(ctx, params.Name)
		}
	}
	return result, nil
}

func (s MCPManifestToolServer) surfaceAffordances() SurfaceAffordanceContract {
	affordances, _ := ManifestSurfaceAffordances(s.Manifest, s.surfaceID())
	return affordances
}

func (s MCPManifestToolServer) surfaceID() string {
	return normalizeMCPManifestSurfaceID(s.SurfaceID)
}

func normalizeMCPManifestSurfaceID(surfaceID string) string {
	if surfaceID := strings.TrimSpace(surfaceID); surfaceID != "" {
		return surfaceID
	}
	return SurfaceIDPulseMCP
}

// DispatchMCPToolServerRequest routes one JSON-RPC request through the shared
// Pulse MCP tool-server semantics so stdio, HTTP, or future adapters do not
// each own initialize/tools/list/tools/call branching or error-code mapping.
func DispatchMCPToolServerRequest(ctx context.Context, req JSONRPCRequest, handlers MCPToolServerHandlers) JSONRPCResponse {
	switch req.Method {
	case MCPMethodInitialize:
		if handlers.Initialize == nil {
			return NewJSONRPCErrorResponse(req.ID, JSONRPCErrorInternal, "initialize handler unavailable", nil)
		}
		resp := NewJSONRPCResponse(req.ID, handlers.Initialize())
		if handlers.OnInitialize != nil {
			handlers.OnInitialize()
		}
		return resp
	case MCPMethodToolsList:
		if handlers.ToolsList == nil {
			return NewJSONRPCErrorResponse(req.ID, JSONRPCErrorInternal, "tools/list handler unavailable", nil)
		}
		return NewJSONRPCResponse(req.ID, handlers.ToolsList())
	case MCPMethodToolsCall:
		if handlers.ToolsCall == nil {
			return NewJSONRPCErrorResponse(req.ID, JSONRPCErrorInternal, "tools/call handler unavailable", nil)
		}
		result, err := handlers.ToolsCall(ctx, req.Params)
		if err != nil {
			return NewJSONRPCErrorResponse(req.ID, JSONRPCErrorInternal, err.Error(), nil)
		}
		return NewJSONRPCResponse(req.ID, result.NormalizeCollections())
	case MCPMethodResourcesList:
		if handlers.ResourcesList == nil {
			return NewJSONRPCErrorResponse(req.ID, JSONRPCErrorInternal, "resources/list handler unavailable", nil)
		}
		result, err := handlers.ResourcesList(ctx)
		if err != nil {
			return NewJSONRPCErrorResponse(req.ID, JSONRPCErrorInternal, err.Error(), nil)
		}
		return NewJSONRPCResponse(req.ID, result.NormalizeCollections())
	case MCPMethodResourcesRead:
		if handlers.ResourcesRead == nil {
			return NewJSONRPCErrorResponse(req.ID, JSONRPCErrorInternal, "resources/read handler unavailable", nil)
		}
		result, err := handlers.ResourcesRead(ctx, req.Params)
		if err != nil {
			return NewJSONRPCErrorResponse(req.ID, JSONRPCErrorInternal, err.Error(), nil)
		}
		return NewJSONRPCResponse(req.ID, result.NormalizeCollections())
	case MCPMethodPromptsList:
		if handlers.PromptsList == nil {
			return NewJSONRPCErrorResponse(req.ID, JSONRPCErrorInternal, "prompts/list handler unavailable", nil)
		}
		return NewJSONRPCResponse(req.ID, handlers.PromptsList().NormalizeCollections())
	case MCPMethodPromptsGet:
		if handlers.PromptsGet == nil {
			return NewJSONRPCErrorResponse(req.ID, JSONRPCErrorInternal, "prompts/get handler unavailable", nil)
		}
		result, err := handlers.PromptsGet(ctx, req.Params)
		if err != nil {
			return NewJSONRPCErrorResponse(req.ID, JSONRPCErrorInternal, err.Error(), nil)
		}
		return NewJSONRPCResponse(req.ID, result.NormalizeCollections())
	case MCPMethodPing:
		return NewJSONRPCResponse(req.ID, map[string]any{})
	default:
		return NewJSONRPCErrorResponse(req.ID, JSONRPCErrorMethodNotFound, fmt.Sprintf("method not found: %s", req.Method), nil)
	}
}

// DecodeMCPCallToolParams parses raw tools/call params through the shared MCP
// envelope so adapters do not each own JSON error wording.
func DecodeMCPCallToolParams(raw json.RawMessage) (MCPCallToolParams, error) {
	var params MCPCallToolParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return MCPCallToolParams{}, fmt.Errorf("decode tools/call params: %w", err)
	}
	params = NormalizeToolCallParams(params)
	if err := ValidateToolCallParams(params); err != nil {
		return MCPCallToolParams{}, fmt.Errorf("decode tools/call params: %w", err)
	}
	return params, nil
}

// ExecuteMCPManifestSurfaceToolHTTP projects one MCP tools/call request onto
// the neutral manifest-backed tool execution helper after applying the same
// manifest-owned surface affordance and surface tool allowlist used by
// tools/list.
func ExecuteMCPManifestSurfaceToolHTTP(ctx context.Context, client HTTPDoer, baseURL, token string, manifest Manifest, surfaceID string, raw json.RawMessage) (MCPToolResult, error) {
	params, err := DecodeMCPCallToolParams(raw)
	if err != nil {
		return MCPToolResult{}, err
	}
	surfaceID = normalizeMCPManifestSurfaceID(surfaceID)
	affordances, _ := ManifestSurfaceAffordances(manifest, surfaceID)
	if !affordances.Tools {
		return MCPToolResult{}, fmt.Errorf("MCP tools are not enabled for surface %s", surfaceID)
	}
	return ExecuteCapabilityToolHTTP(ctx, client, baseURL, token, ManifestSurfaceToolCapabilities(manifest, surfaceID), params)
}

// MCPResource describes an available MCP resource.
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// MCPListResourcesResult is the resources/list result.
type MCPListResourcesResult struct {
	Resources []MCPResource `json:"resources"`
}

// NormalizeCollections returns a detached resources/list result with stable
// empty collections.
func (r MCPListResourcesResult) NormalizeCollections() MCPListResourcesResult {
	r.Resources = append([]MCPResource(nil), r.Resources...)
	if r.Resources == nil {
		r.Resources = []MCPResource{}
	}
	return r
}

// MCPReadResourceParams are the params for resources/read.
type MCPReadResourceParams struct {
	URI string `json:"uri"`
}

// MCPReadResourceResult is the resources/read result.
type MCPReadResourceResult struct {
	Contents []MCPResourceContent `json:"contents"`
}

// NormalizeCollections returns a detached resources/read result with stable
// empty collections.
func (r MCPReadResourceResult) NormalizeCollections() MCPReadResourceResult {
	r.Contents = append([]MCPResourceContent(nil), r.Contents...)
	if r.Contents == nil {
		r.Contents = []MCPResourceContent{}
	}
	return r
}

// MCPResourceContent is one resource content block.
type MCPResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

type mcpFleetContextPayload struct {
	Resources []mcpFleetResourcePayload `json:"resources"`
}

type mcpFleetResourcePayload struct {
	CanonicalID          string `json:"canonicalId"`
	ResourceType         string `json:"resourceType"`
	ResourceName         string `json:"resourceName"`
	Technology           string `json:"technology,omitempty"`
	PendingApprovalCount int    `json:"pendingApprovalCount,omitempty"`
	Findings             struct {
		Total    int `json:"total"`
		Critical int `json:"critical"`
		Warning  int `json:"warning"`
		Info     int `json:"info"`
	} `json:"findings"`
}

// MCPManifestResourceProjectionSupported reports whether one manifest surface
// can satisfy MCP resources/list and resources/read through the canonical Pulse
// Intelligence context capabilities.
func MCPManifestResourceProjectionSupported(manifest Manifest, surfaceID string) bool {
	return len(ManifestSurfaceResourceCapabilities(manifest, surfaceID)) == 2
}

// ManifestSurfaceResourceCapabilities returns the canonical context
// capabilities allowed to back MCP resources for one manifest surface.
func ManifestSurfaceResourceCapabilities(manifest Manifest, surfaceID string) []Capability {
	surfaceID = normalizeMCPManifestSurfaceID(surfaceID)
	affordances, _ := ManifestSurfaceAffordances(manifest, surfaceID)
	if !affordances.Resources {
		return []Capability{}
	}
	fleet, err := ResolveRequestResponseCapability(manifest.Capabilities, FleetContextCapabilityName)
	if err != nil {
		return []Capability{}
	}
	resource, err := ResolveRequestResponseCapability(manifest.Capabilities, ResourceContextCapabilityName)
	if err != nil {
		return []Capability{}
	}
	return []Capability{fleet, resource}
}

// MCPResourceURI returns the stable MCP resource URI for a canonical Pulse
// resource id.
func MCPResourceURI(resourceID string) string {
	return "pulse://" + MCPResourceContextURIHost + "/" + url.PathEscape(strings.TrimSpace(resourceID))
}

// ResourceIDFromMCPResourceURI decodes a Pulse MCP resource URI into the
// canonical resource id passed to get_resource_context.
func ResourceIDFromMCPResourceURI(rawURI string) (string, error) {
	rawURI = strings.TrimSpace(rawURI)
	if rawURI == "" {
		return "", fmt.Errorf("resource URI is required")
	}
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return "", fmt.Errorf("parse resource URI: %w", err)
	}
	if parsed.Scheme != "pulse" || parsed.Host != MCPResourceContextURIHost {
		return "", fmt.Errorf("unsupported resource URI: %s", rawURI)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("resource URI must not include query or fragment: %s", rawURI)
	}
	encodedID := strings.TrimPrefix(parsed.EscapedPath(), "/")
	if strings.TrimSpace(encodedID) == "" {
		return "", fmt.Errorf("resource URI is missing a resource id")
	}
	resourceID, err := url.PathUnescape(encodedID)
	if err != nil {
		return "", fmt.Errorf("decode resource URI: %w", err)
	}
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return "", fmt.Errorf("resource URI is missing a resource id")
	}
	return resourceID, nil
}

// DecodeMCPReadResourceParams parses and validates resources/read params.
func DecodeMCPReadResourceParams(raw json.RawMessage) (MCPReadResourceParams, error) {
	var params MCPReadResourceParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return MCPReadResourceParams{}, fmt.Errorf("decode resources/read params: %w", err)
	}
	params.URI = strings.TrimSpace(params.URI)
	if params.URI == "" {
		return MCPReadResourceParams{}, fmt.Errorf("decode resources/read params: resource URI is required")
	}
	if _, err := ResourceIDFromMCPResourceURI(params.URI); err != nil {
		return MCPReadResourceParams{}, fmt.Errorf("decode resources/read params: %w", err)
	}
	return params, nil
}

// ListMCPManifestSurfaceResourcesHTTP projects the canonical fleet-context
// response into MCP resource descriptors for one manifest-owned surface. The
// fleet context remains the source of truth; this helper only adapts identity
// fields into the MCP resources/list envelope.
func ListMCPManifestSurfaceResourcesHTTP(ctx context.Context, client HTTPDoer, baseURL, token string, manifest Manifest, surfaceID string) (MCPListResourcesResult, error) {
	capabilities, err := resolveMCPManifestSurfaceResourceCapabilities(manifest, surfaceID)
	if err != nil {
		return MCPListResourcesResult{}, err
	}
	body, err := CallRequestResponseCapabilityHTTPBodyByName(ctx, client, baseURL, token, capabilities, FleetContextCapabilityName, map[string]any{})
	if err != nil {
		return MCPListResourcesResult{}, err
	}
	var fleet mcpFleetContextPayload
	if err := json.Unmarshal(body, &fleet); err != nil {
		return MCPListResourcesResult{}, fmt.Errorf("decode %s response: %w", FleetContextCapabilityName, err)
	}
	resources := make([]MCPResource, 0, len(fleet.Resources))
	for _, resource := range fleet.Resources {
		resourceID := strings.TrimSpace(resource.CanonicalID)
		if resourceID == "" {
			continue
		}
		resources = append(resources, MCPResource{
			URI:         MCPResourceURI(resourceID),
			Name:        mcpResourceDisplayName(resource),
			Description: mcpResourceDescription(resource),
			MimeType:    MCPResourceContextMIMEType,
		})
	}
	return (MCPListResourcesResult{Resources: resources}).NormalizeCollections(), nil
}

// ReadMCPManifestSurfaceResourceHTTP projects one MCP resource URI onto the
// canonical get_resource_context capability for one manifest-owned surface and
// returns the JSON bundle as resource content.
func ReadMCPManifestSurfaceResourceHTTP(ctx context.Context, client HTTPDoer, baseURL, token string, manifest Manifest, surfaceID string, raw json.RawMessage) (MCPReadResourceResult, error) {
	params, err := DecodeMCPReadResourceParams(raw)
	if err != nil {
		return MCPReadResourceResult{}, err
	}
	capabilities, err := resolveMCPManifestSurfaceResourceCapabilities(manifest, surfaceID)
	if err != nil {
		return MCPReadResourceResult{}, err
	}
	resourceID, err := ResourceIDFromMCPResourceURI(params.URI)
	if err != nil {
		return MCPReadResourceResult{}, err
	}
	body, err := CallRequestResponseCapabilityHTTPBodyByName(ctx, client, baseURL, token, capabilities, ResourceContextCapabilityName, map[string]any{
		ResourceIDArgumentName: resourceID,
	})
	if err != nil {
		return MCPReadResourceResult{}, err
	}
	return (MCPReadResourceResult{Contents: []MCPResourceContent{{
		URI:      MCPResourceURI(resourceID),
		MimeType: MCPResourceContextMIMEType,
		Text:     string(body),
	}}}).NormalizeCollections(), nil
}

func resolveMCPManifestSurfaceResourceCapabilities(manifest Manifest, surfaceID string) ([]Capability, error) {
	surfaceID = normalizeMCPManifestSurfaceID(surfaceID)
	affordances, _ := ManifestSurfaceAffordances(manifest, surfaceID)
	if !affordances.Resources {
		return nil, fmt.Errorf("MCP resources are not enabled for surface %s", surfaceID)
	}
	capabilities := ManifestSurfaceResourceCapabilities(manifest, surfaceID)
	if len(capabilities) != 2 {
		return nil, fmt.Errorf("MCP resources require %s and %s capabilities", FleetContextCapabilityName, ResourceContextCapabilityName)
	}
	return capabilities, nil
}

func mcpResourceDisplayName(resource mcpFleetResourcePayload) string {
	if name := strings.TrimSpace(resource.ResourceName); name != "" {
		return name
	}
	return strings.TrimSpace(resource.CanonicalID)
}

func mcpResourceDescription(resource mcpFleetResourcePayload) string {
	parts := []string{}
	if resourceType := strings.TrimSpace(resource.ResourceType); resourceType != "" {
		parts = append(parts, resourceType)
	}
	if technology := strings.TrimSpace(resource.Technology); technology != "" {
		parts = append(parts, technology)
	}
	if resource.Findings.Total > 0 {
		parts = append(parts, fmt.Sprintf("findings: %d total (%d critical, %d warning, %d info)", resource.Findings.Total, resource.Findings.Critical, resource.Findings.Warning, resource.Findings.Info))
	}
	if resource.PendingApprovalCount > 0 {
		parts = append(parts, fmt.Sprintf("pending approvals: %d", resource.PendingApprovalCount))
	}
	return strings.Join(parts, "; ")
}

// MCPPrompt describes an available prompt template.
type MCPPrompt struct {
	Name        string              `json:"name"`
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Arguments   []MCPPromptArgument `json:"arguments,omitempty"`
}

// NormalizeCollections returns a prompt with detached argument metadata.
func (p MCPPrompt) NormalizeCollections() MCPPrompt {
	p.Name = strings.TrimSpace(p.Name)
	p.Title = strings.TrimSpace(p.Title)
	p.Description = strings.TrimSpace(p.Description)
	p.Arguments = append([]MCPPromptArgument(nil), p.Arguments...)
	if p.Arguments == nil {
		p.Arguments = []MCPPromptArgument{}
	}
	return p
}

// MCPPromptArgument describes an argument in a prompt template.
type MCPPromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// MCPListPromptsResult is the prompts/list result.
type MCPListPromptsResult struct {
	Prompts []MCPPrompt `json:"prompts"`
}

// NormalizeCollections returns a detached prompts/list result.
func (r MCPListPromptsResult) NormalizeCollections() MCPListPromptsResult {
	r.Prompts = append([]MCPPrompt(nil), r.Prompts...)
	if r.Prompts == nil {
		r.Prompts = []MCPPrompt{}
	}
	for i := range r.Prompts {
		r.Prompts[i] = r.Prompts[i].NormalizeCollections()
	}
	return r
}

// MCPGetPromptParams are the params for prompts/get.
type MCPGetPromptParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// NormalizeCollections returns detached prompt params with stable maps.
func (p MCPGetPromptParams) NormalizeCollections() MCPGetPromptParams {
	p.Name = strings.TrimSpace(p.Name)
	if p.Arguments == nil {
		p.Arguments = map[string]string{}
		return p
	}
	args := make(map[string]string, len(p.Arguments))
	for k, v := range p.Arguments {
		args[k] = v
	}
	p.Arguments = args
	return p
}

// MCPGetPromptResult is the prompts/get result.
type MCPGetPromptResult struct {
	Description string             `json:"description,omitempty"`
	Messages    []MCPPromptMessage `json:"messages"`
}

// NormalizeCollections returns a detached prompt result.
func (r MCPGetPromptResult) NormalizeCollections() MCPGetPromptResult {
	r.Messages = append([]MCPPromptMessage(nil), r.Messages...)
	if r.Messages == nil {
		r.Messages = []MCPPromptMessage{}
	}
	return r
}

// MCPPromptMessage is a message in a prompt result.
type MCPPromptMessage struct {
	Role    string     `json:"role"`
	Content MCPContent `json:"content"`
}

// MCPManifestPromptProjectionSupported reports whether a manifest exposes
// workflow prompts for external agents.
func MCPManifestPromptProjectionSupported(manifest Manifest) bool {
	return mcpManifestPromptProjectionSupported(manifest)
}

// MCPManifestSurfacePromptProjectionSupported reports whether one manifest
// surface can satisfy MCP prompts/list and prompts/get from the manifest-owned
// Pulse workflow prompt catalogue.
func MCPManifestSurfacePromptProjectionSupported(manifest Manifest, surfaceID string) bool {
	surfaceID = normalizeMCPManifestSurfaceID(surfaceID)
	affordances, _ := ManifestSurfaceAffordances(manifest, surfaceID)
	return affordances.Prompts && mcpManifestPromptProjectionSupported(manifest)
}

func mcpManifestPromptProjectionSupported(manifest Manifest) bool {
	for _, prompt := range ManifestPulseWorkflowPrompts(manifest) {
		if strings.TrimSpace(prompt.Name) != "" {
			return true
		}
	}
	return false
}

// ProjectMCPWorkflowPrompts projects manifest-owned Pulse Intelligence workflow
// starters into MCP prompt templates.
func ProjectMCPWorkflowPrompts(workflowPrompts []PulseWorkflowPrompt) []MCPPrompt {
	prompts := make([]MCPPrompt, 0, len(workflowPrompts))
	for _, workflowPrompt := range workflowPrompts {
		prompts = append(prompts, MCPPrompt{
			Name:        workflowPrompt.Name,
			Title:       workflowPrompt.Label,
			Description: workflowPrompt.Description,
			Arguments:   projectPulseWorkflowPromptArgumentsToMCP(workflowPrompt.Arguments),
		})
	}
	for i := range prompts {
		prompts[i] = prompts[i].NormalizeCollections()
	}
	return prompts
}

func projectPulseWorkflowPromptArgumentsToMCP(args []PulseWorkflowPromptArgument) []MCPPromptArgument {
	out := make([]MCPPromptArgument, 0, len(args))
	for _, arg := range args {
		out = append(out, MCPPromptArgument{
			Name:        arg.Name,
			Description: arg.Description,
			Required:    arg.Required,
		})
	}
	return out
}

// DecodeMCPGetPromptParams parses and validates prompts/get params.
func DecodeMCPGetPromptParams(raw json.RawMessage) (MCPGetPromptParams, error) {
	var params MCPGetPromptParams
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &params); err != nil {
			return MCPGetPromptParams{}, fmt.Errorf("decode prompts/get params: %w", err)
		}
	}
	params = params.NormalizeCollections()
	if params.Name == "" {
		return MCPGetPromptParams{}, fmt.Errorf("decode prompts/get params: prompt name is required")
	}
	return params, nil
}

// GetMCPPromptFromManifest returns one manifest-owned workflow prompt result.
func GetMCPPromptFromManifest(manifest Manifest, raw json.RawMessage) (MCPGetPromptResult, error) {
	params, err := DecodeMCPGetPromptParams(raw)
	if err != nil {
		return MCPGetPromptResult{}, err
	}
	return BuildMCPPromptFromManifest(manifest, params)
}

// GetMCPPromptFromManifestSurface returns one manifest-owned workflow prompt
// result after applying the target MCP surface's prompt affordance and manifest
// prompt catalogue gate.
func GetMCPPromptFromManifestSurface(manifest Manifest, surfaceID string, raw json.RawMessage) (MCPGetPromptResult, error) {
	surfaceID = normalizeMCPManifestSurfaceID(surfaceID)
	if !MCPManifestSurfacePromptProjectionSupported(manifest, surfaceID) {
		return MCPGetPromptResult{}, fmt.Errorf("MCP prompts are not enabled for surface %s", surfaceID)
	}
	return GetMCPPromptFromManifest(manifest, raw)
}

// BuildMCPPromptFromManifest renders one manifest-declared workflow prompt for
// the MCP prompt envelope.
func BuildMCPPromptFromManifest(manifest Manifest, params MCPGetPromptParams) (MCPGetPromptResult, error) {
	params = params.NormalizeCollections()
	if !pulseWorkflowPromptDeclared(ManifestPulseWorkflowPrompts(manifest), params.Name) {
		return MCPGetPromptResult{}, fmt.Errorf("unknown prompt: %s", params.Name)
	}
	result, err := BuildPulseWorkflowPromptFromManifestWithOptions(manifest, PulseWorkflowPromptParams{
		Name:      params.Name,
		Arguments: params.Arguments,
	}, PulseWorkflowPromptRenderOptions{
		ResourceContextInstruction: func(resourceID string) string {
			return fmt.Sprintf("Prefer resources/read on %s when resources are available, otherwise call %s with %s %q", MCPResourceURI(resourceID), ResourceContextCapabilityName, ResourceIDArgumentName, resourceID)
		},
	})
	if err != nil {
		return MCPGetPromptResult{}, err
	}
	return newMCPUserPrompt(result.Description, result.Text), nil
}

func newMCPUserPrompt(description, text string) MCPGetPromptResult {
	return (MCPGetPromptResult{
		Description: description,
		Messages: []MCPPromptMessage{{
			Role:    "user",
			Content: NewToolTextContent(text),
		}},
	}).NormalizeCollections()
}

// NewJSONRPCResponse builds a success response with the shared protocol marker.
func NewJSONRPCResponse(id json.RawMessage, result any) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: id, Result: result}
}

// NewJSONRPCErrorResponse builds an error response with the shared protocol
// marker and stable JSON-RPC code.
func NewJSONRPCErrorResponse(id json.RawMessage, code int, message string, data any) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

// NewJSONRPCNotification builds a JSON-RPC notification. Notifications must not
// carry an id or clients will wait for a response.
func NewJSONRPCNotification(method string, params json.RawMessage) JSONRPCRequest {
	return JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  params,
	}
}

// MCPNotificationMethod projects a Pulse event kind onto the MCP notification
// method namespace.
func MCPNotificationMethod(kind string) string {
	return MCPNotificationPrefix + kind
}

// NewMCPEventNotification projects a Pulse Intelligence SSE event onto the MCP
// notification channel. Empty records and stream-local transport events do not
// produce client-facing notifications.
func NewMCPEventNotification(kind string, params json.RawMessage) (JSONRPCRequest, bool) {
	if !IsActionableSSERecord(SSERecord{Event: kind, Data: string(params)}) {
		return JSONRPCRequest{}, false
	}
	return NewJSONRPCNotification(MCPNotificationMethod(kind), params), true
}

// MCPContent is the protocol-facing compatibility name for the shared tool
// result content block used by MCP tools/call.
type MCPContent = ToolContent

// MCPToolResult is the protocol-facing compatibility name for the shared tool
// result envelope used by MCP tools/call.
type MCPToolResult = ToolResult

func (e JSONRPCError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("json-rpc error %d", e.Code)
	}
	return e.Message
}
