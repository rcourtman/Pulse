package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/rs/zerolog/log"
)

const (
	ProtocolVersion = "2024-11-05"
	ServerName      = "pulse-mcp"
	ServerVersion   = "1.0.0"
)

// ToolExecutor executes tools on behalf of the MCP server
type ToolExecutor interface {
	ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (CallToolResult, error)
	ListTools() []Tool
}

// Server implements an MCP server over HTTP
type Server struct {
	mu       sync.RWMutex
	executor ToolExecutor
	addr     string
	server   *http.Server
}

// NewServer creates a new MCP server
func NewServer(addr string, executor ToolExecutor) *Server {
	return &Server{
		addr:     addr,
		executor: executor,
	}
}

// Start starts the MCP server
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	log.Info().Str("addr", s.addr).Msg("Starting MCP server")
	return s.server.ListenAndServe()
}

// Stop stops the MCP server
func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// Addr returns the server address
func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, nil, ErrParse, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeError(w, nil, ErrParse, "Failed to parse JSON-RPC request")
		return
	}

	if req.JSONRPC != "2.0" {
		s.writeError(w, req.ID, ErrInvalidRequest, "Invalid JSON-RPC version")
		return
	}

	log.Debug().
		Str("method", req.Method).
		Interface("id", req.ID).
		Msg("MCP request received")

	result, mcpErr := s.handleMethod(r.Context(), req)
	if mcpErr != nil {
		s.writeErrorResponse(w, req.ID, mcpErr)
		return
	}

	s.writeResult(w, req.ID, result)
}

func (s *Server) handleMethod(ctx context.Context, req Request) (interface{}, *Error) {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.Params)
	case "initialized":
		// Client notification that initialization is complete
		return nil, nil
	case "tools/list":
		return s.handleListTools()
	case "tools/call":
		return s.handleCallTool(ctx, req.Params)
	case "resources/list":
		return s.handleListResources()
	case "resources/read":
		return s.handleReadResource(ctx, req.Params)
	case "prompts/list":
		return s.handleListPrompts()
	case "prompts/get":
		return s.handleGetPrompt(req.Params)
	case "ping":
		return map[string]interface{}{}, nil
	default:
		return nil, &Error{
			Code:    ErrMethodNotFound,
			Message: fmt.Sprintf("Method not found: %s", req.Method),
		}
	}
}

func (s *Server) handleInitialize(params json.RawMessage) (*InitializeResult, *Error) {
	var initParams InitializeParams
	if params != nil {
		if err := json.Unmarshal(params, &initParams); err != nil {
			return nil, &Error{
				Code:    ErrInvalidParams,
				Message: "Failed to parse initialize params",
			}
		}
	}

	log.Info().
		Str("client", initParams.ClientInfo.Name).
		Str("clientVersion", initParams.ClientInfo.Version).
		Str("protocolVersion", initParams.ProtocolVersion).
		Msg("MCP client connected")

	return &InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities: Capabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
			Resources: &ResourcesCapability{
				Subscribe:   false,
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    ServerName,
			Version: ServerVersion,
		},
	}, nil
}

func (s *Server) handleListTools() (*ListToolsResult, *Error) {
	s.mu.RLock()
	executor := s.executor
	s.mu.RUnlock()

	if executor == nil {
		return &ListToolsResult{Tools: []Tool{}}, nil
	}

	tools := executor.ListTools()
	return &ListToolsResult{Tools: tools}, nil
}

func (s *Server) handleCallTool(ctx context.Context, params json.RawMessage) (*CallToolResult, *Error) {
	var callParams CallToolParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, &Error{
			Code:    ErrInvalidParams,
			Message: "Failed to parse tool call params",
		}
	}

	s.mu.RLock()
	executor := s.executor
	s.mu.RUnlock()

	if executor == nil {
		return nil, &Error{
			Code:    ErrInternal,
			Message: "No tool executor configured",
		}
	}

	log.Debug().
		Str("tool", callParams.Name).
		Interface("args", callParams.Arguments).
		Msg("Executing tool")

	result, err := executor.ExecuteTool(ctx, callParams.Name, callParams.Arguments)
	if err != nil {
		log.Error().Err(err).Str("tool", callParams.Name).Msg("Tool execution failed")
		return &CallToolResult{
			Content: []Content{NewTextContent(err.Error())},
			IsError: true,
		}, nil
	}

	return &result, nil
}

func (s *Server) handleListResources() (*ListResourcesResult, *Error) {
	// Pulse exposes infrastructure state as a resource
	return &ListResourcesResult{
		Resources: []Resource{
			{
				URI:         "pulse://infrastructure/state",
				Name:        "Infrastructure State",
				Description: "Current state of all monitored infrastructure",
				MimeType:    "application/json",
			},
			{
				URI:         "pulse://infrastructure/alerts",
				Name:        "Active Alerts",
				Description: "Currently active alerts",
				MimeType:    "application/json",
			},
		},
	}, nil
}

func (s *Server) handleReadResource(ctx context.Context, params json.RawMessage) (*ReadResourceResult, *Error) {
	var readParams ReadResourceParams
	if err := json.Unmarshal(params, &readParams); err != nil {
		return nil, &Error{
			Code:    ErrInvalidParams,
			Message: "Failed to parse resource read params",
		}
	}

	// TODO: Implement resource reading via executor
	// For now, return empty result
	return &ReadResourceResult{
		Contents: []ResourceContent{
			{
				URI:      readParams.URI,
				MimeType: "application/json",
				Text:     "{}",
			},
		},
	}, nil
}

func (s *Server) handleListPrompts() (*ListPromptsResult, *Error) {
	// Pulse provides some built-in prompts
	return &ListPromptsResult{
		Prompts: []Prompt{
			{
				Name:        "analyze_infrastructure",
				Description: "Analyze the current infrastructure state and identify issues",
			},
			{
				Name:        "investigate_alert",
				Description: "Investigate a specific alert",
				Arguments: []PromptArgument{
					{Name: "alert_id", Description: "The alert ID to investigate", Required: true},
				},
			},
		},
	}, nil
}

func (s *Server) handleGetPrompt(params json.RawMessage) (*GetPromptResult, *Error) {
	var getParams GetPromptParams
	if err := json.Unmarshal(params, &getParams); err != nil {
		return nil, &Error{
			Code:    ErrInvalidParams,
			Message: "Failed to parse prompt get params",
		}
	}

	switch getParams.Name {
	case "analyze_infrastructure":
		return &GetPromptResult{
			Description: "Analyze current infrastructure",
			Messages: []PromptMessage{
				{
					Role:    "user",
					Content: NewTextContent("Analyze the current infrastructure state. Look for any issues, capacity concerns, or optimization opportunities."),
				},
			},
		}, nil
	case "investigate_alert":
		alertID := getParams.Arguments["alert_id"]
		return &GetPromptResult{
			Description: fmt.Sprintf("Investigate alert %s", alertID),
			Messages: []PromptMessage{
				{
					Role:    "user",
					Content: NewTextContent(fmt.Sprintf("Investigate alert %s. Identify the root cause and suggest remediation steps.", alertID)),
				},
			},
		}, nil
	default:
		return nil, &Error{
			Code:    ErrInvalidParams,
			Message: fmt.Sprintf("Unknown prompt: %s", getParams.Name),
		}
	}
}

func (s *Server) writeResult(w http.ResponseWriter, id interface{}, result interface{}) {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		s.writeError(w, id, ErrInternal, "Failed to marshal result")
		return
	}

	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultJSON,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) writeError(w http.ResponseWriter, id interface{}, code int, message string) {
	s.writeErrorResponse(w, id, &Error{Code: code, Message: message})
}

func (s *Server) writeErrorResponse(w http.ResponseWriter, id interface{}, err *Error) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   err,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// SetExecutor updates the tool executor
func (s *Server) SetExecutor(executor ToolExecutor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.executor = executor
}
