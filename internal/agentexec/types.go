package agentexec

import (
	"time"
)

// MessageType identifies the type of WebSocket message
type MessageType string

const (
	// Agent -> Server messages
	MsgTypeAgentRegister MessageType = "agent_register"
	MsgTypeAgentPing     MessageType = "agent_ping"
	MsgTypeCommandResult MessageType = "command_result"

	// Server -> Agent messages
	MsgTypeRegistered MessageType = "registered"
	MsgTypePong       MessageType = "pong"
	MsgTypeExecuteCmd MessageType = "execute_command"
	MsgTypeReadFile   MessageType = "read_file"
)

// Message is the envelope for all WebSocket messages
type Message struct {
	Type      MessageType `json:"type"`
	ID        string      `json:"id,omitempty"` // Unique message ID for request/response correlation
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload,omitempty"`
}

// AgentRegisterPayload is sent by agent on connection
type AgentRegisterPayload struct {
	AgentID  string   `json:"agent_id"`
	Hostname string   `json:"hostname"`
	Version  string   `json:"version"`
	Platform string   `json:"platform"` // "linux", "windows", "darwin"
	Tags     []string `json:"tags,omitempty"`
	Token    string   `json:"token"` // API token for authentication
}

// RegisteredPayload is sent by server after successful registration
type RegisteredPayload struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ExecuteCommandPayload is sent by server to request command execution
type ExecuteCommandPayload struct {
	RequestID  string `json:"request_id"`
	Command    string `json:"command"`
	TargetType string `json:"target_type"`         // "host", "container", "vm"
	TargetID   string `json:"target_id,omitempty"` // VMID for container/VM
	Timeout    int    `json:"timeout,omitempty"`   // seconds, 0 = default
}

// ReadFilePayload is sent by server to request file content
type ReadFilePayload struct {
	RequestID  string `json:"request_id"`
	Path       string `json:"path"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id,omitempty"`
	MaxBytes   int64  `json:"max_bytes,omitempty"` // 0 = default (1MB)
}

// CommandResultPayload is sent by agent with command execution result
type CommandResultPayload struct {
	RequestID string `json:"request_id"`
	Success   bool   `json:"success"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
	ExitCode  int    `json:"exit_code"`
	Error     string `json:"error,omitempty"`
	Duration  int64  `json:"duration_ms"`
}

// ConnectedAgent represents an agent connected via WebSocket
type ConnectedAgent struct {
	AgentID     string
	Hostname    string
	Version     string
	Platform    string
	Tags        []string
	ConnectedAt time.Time
}
