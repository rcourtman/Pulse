package chat

import (
	"fmt"
	"strings"
	"time"
)

// SessionState represents the current state of a chat session's workflow.
// This FSM enforces structural guarantees that prevent prompt steering from
// creeping back and ensures contributors can't accidentally bypass safety checks.
type SessionState string

const (
	// StateResolving - no validated target yet, must discover resources first
	StateResolving SessionState = "RESOLVING"

	// StateReading - read tools allowed, can query and explore
	StateReading SessionState = "READING"

	// StateWriting - write tools allowed (strict gating applies)
	StateWriting SessionState = "WRITING"

	// StateVerifying - must run at least one read after a write before final answer
	StateVerifying SessionState = "VERIFYING"
)

// ToolKind classifies tool calls for FSM state transitions
type ToolKind int

const (
	// ToolKindResolve - discovery/query tools that find resources
	ToolKindResolve ToolKind = iota

	// ToolKindRead - read-only tools (logs, metrics, status, config)
	ToolKindRead

	// ToolKindWrite - mutating tools (restart, stop, start, delete, file write)
	ToolKindWrite

	// ToolKindUserInput - interactive tools that request user input (does not advance FSM state)
	ToolKindUserInput
)

func (k ToolKind) String() string {
	switch k {
	case ToolKindResolve:
		return "resolve"
	case ToolKindRead:
		return "read"
	case ToolKindWrite:
		return "write"
	case ToolKindUserInput:
		return "user_input"
	default:
		return "unknown"
	}
}

// SessionFSM tracks the workflow state for a chat session.
// This is stored alongside ResolvedContext in the session.
type SessionFSM struct {
	State SessionState `json:"state"`

	// WroteThisEpisode tracks whether we performed a write in this "episode"
	WroteThisEpisode bool `json:"wrote_this_episode"`

	// ReadAfterWrite tracks whether we performed a read *after* the last write
	ReadAfterWrite bool `json:"read_after_write"`

	// LastWriteTool records the last write tool for debugging/telemetry
	LastWriteTool string `json:"last_write_tool,omitempty"`

	// LastWriteAt records when the last write happened
	LastWriteAt time.Time `json:"last_write_at,omitempty"`

	// LastReadTool records the last read tool (for verification tracking)
	LastReadTool string `json:"last_read_tool,omitempty"`

	// LastReadAt records when the last read happened
	LastReadAt time.Time `json:"last_read_at,omitempty"`

	// PendingRecoveries tracks blocked operations awaiting recovery
	// Key is recovery_id (UUID), cleaned up after TTL
	PendingRecoveries map[string]*PendingRecovery `json:"-"`
}

// PendingRecovery tracks a blocked operation that may be retried after recovery
type PendingRecovery struct {
	RecoveryID string    `json:"recovery_id"`
	ErrorCode  string    `json:"error_code"` // FSM_BLOCKED, STRICT_RESOLUTION
	Tool       string    `json:"tool"`       // original tool that was blocked
	CreatedAt  time.Time `json:"created_at"`
	Attempts   int       `json:"attempts"` // number of recovery attempts
}

// RecoveryTTL is how long we track pending recoveries before cleanup
const RecoveryTTL = 10 * time.Minute

// NewSessionFSM creates a new FSM in the initial RESOLVING state
func NewSessionFSM() *SessionFSM {
	return &SessionFSM{
		State:             StateResolving,
		PendingRecoveries: make(map[string]*PendingRecovery),
	}
}

// TrackPendingRecovery records a blocked operation that may be recovered.
// Returns the recovery_id for correlation.
func (fsm *SessionFSM) TrackPendingRecovery(errorCode, tool string) string {
	fsm.cleanupExpiredRecoveries()

	recoveryID := fmt.Sprintf("%s-%d", tool, time.Now().UnixNano())
	fsm.PendingRecoveries[recoveryID] = &PendingRecovery{
		RecoveryID: recoveryID,
		ErrorCode:  errorCode,
		Tool:       tool,
		CreatedAt:  time.Now(),
		Attempts:   1,
	}
	return recoveryID
}

// CheckRecoverySuccess checks if a successful tool call resolves a pending recovery.
// Returns the PendingRecovery if found (for metrics), nil otherwise.
// The recovery is removed from tracking after this call.
func (fsm *SessionFSM) CheckRecoverySuccess(tool string) *PendingRecovery {
	fsm.cleanupExpiredRecoveries()

	// Look for any pending recovery for this tool
	for id, pr := range fsm.PendingRecoveries {
		if pr.Tool == tool {
			delete(fsm.PendingRecoveries, id)
			return pr
		}
	}
	return nil
}

// cleanupExpiredRecoveries removes recoveries older than RecoveryTTL
func (fsm *SessionFSM) cleanupExpiredRecoveries() {
	if fsm.PendingRecoveries == nil {
		fsm.PendingRecoveries = make(map[string]*PendingRecovery)
		return
	}

	cutoff := time.Now().Add(-RecoveryTTL)
	for id, pr := range fsm.PendingRecoveries {
		if pr.CreatedAt.Before(cutoff) {
			delete(fsm.PendingRecoveries, id)
		}
	}
}

// CanExecuteTool checks if the current state allows executing a tool of the given kind.
// Returns an error describing why the tool is blocked, or nil if allowed.
func (fsm *SessionFSM) CanExecuteTool(kind ToolKind, toolName string) error {
	switch fsm.State {
	case StateResolving:
		// In RESOLVING, only allow resolve/read tools (must discover before writing)
		if kind == ToolKindWrite {
			return &FSMBlockedError{
				State:       fsm.State,
				ToolName:    toolName,
				ToolKind:    kind,
				Reason:      "No resources have been discovered yet. Use pulse_query to discover resources before performing write operations.",
				Recoverable: true,
			}
		}
		return nil

	case StateReading:
		// In READING, all tools are allowed
		return nil

	case StateWriting:
		// In WRITING, all tools are allowed (this state is transitional)
		return nil

	case StateVerifying:
		// In VERIFYING, only allow read/resolve tools until verification is complete
		if kind == ToolKindWrite {
			return &FSMBlockedError{
				State:       fsm.State,
				ToolName:    toolName,
				ToolKind:    kind,
				Reason:      "Must verify the previous write operation before performing another write. Use a read tool (logs, status, query) to check the result first.",
				Recoverable: true,
			}
		}
		return nil
	}

	return nil
}

// CanFinalAnswer checks if the current state allows producing a final answer.
// Returns an error if the model should continue with tool calls instead.
func (fsm *SessionFSM) CanFinalAnswer() error {
	if fsm.State == StateVerifying && !fsm.ReadAfterWrite {
		return &FSMBlockedError{
			State:       fsm.State,
			Reason:      "Must verify the write operation before providing a final answer. Use a read tool to check the result.",
			Recoverable: true,
		}
	}
	return nil
}

// OnToolSuccess transitions the FSM state after a successful tool execution.
// Call this after a tool completes successfully.
func (fsm *SessionFSM) OnToolSuccess(kind ToolKind, toolName string) {
	now := time.Now()

	switch kind {
	case ToolKindResolve:
		// Discovery counts as a read - enables reading state
		if fsm.State == StateResolving {
			fsm.State = StateReading
		}
		fsm.LastReadTool = toolName
		fsm.LastReadAt = now
		// Resolve also counts as "read after write" for verification
		if fsm.State == StateVerifying {
			fsm.ReadAfterWrite = true
		}

	case ToolKindRead:
		// Read transitions from RESOLVING to READING
		if fsm.State == StateResolving {
			fsm.State = StateReading
		}
		fsm.LastReadTool = toolName
		fsm.LastReadAt = now
		// Read after write clears the verification requirement
		if fsm.State == StateVerifying {
			fsm.ReadAfterWrite = true
		}

	case ToolKindWrite:
		// Write transitions to VERIFYING state
		fsm.State = StateVerifying
		fsm.WroteThisEpisode = true
		fsm.ReadAfterWrite = false
		fsm.LastWriteTool = toolName
		fsm.LastWriteAt = now

	case ToolKindUserInput:
		// Interactive user input does not advance state (it is neither discovery nor verification).
		return
	}
}

// CompleteVerification transitions from VERIFYING to READING after successful verification.
// Call this after ReadAfterWrite becomes true and you want to allow new writes.
func (fsm *SessionFSM) CompleteVerification() {
	if fsm.State == StateVerifying && fsm.ReadAfterWrite {
		fsm.State = StateReading
		fsm.ReadAfterWrite = false // Reset for next verification cycle
		// Note: WroteThisEpisode stays true - it tracks "wrote at all this session"
		// not "wrote in current verification cycle"
	}
}

// Reset resets the FSM to initial state (e.g., for session clear)
func (fsm *SessionFSM) Reset() {
	fsm.State = StateResolving
	fsm.WroteThisEpisode = false
	fsm.ReadAfterWrite = false
	fsm.LastWriteTool = ""
	fsm.LastWriteAt = time.Time{}
	fsm.LastReadTool = ""
	fsm.LastReadAt = time.Time{}
}

// ResetKeepProgress resets verification tracking but keeps the "active" state
// Use this for context clear with keepPinned=true
func (fsm *SessionFSM) ResetKeepProgress() {
	if fsm.State == StateVerifying {
		fsm.State = StateReading
	}
	fsm.WroteThisEpisode = false
	fsm.ReadAfterWrite = false
}

// FSMBlockedError is returned when the FSM blocks an action
type FSMBlockedError struct {
	State       SessionState
	ToolName    string
	ToolKind    ToolKind
	Reason      string
	Recoverable bool
}

func (e *FSMBlockedError) Error() string {
	if e.ToolName != "" {
		return fmt.Sprintf("FSM blocked tool '%s' (%s) in state %s: %s", e.ToolName, e.ToolKind, e.State, e.Reason)
	}
	return fmt.Sprintf("FSM blocked in state %s: %s", e.State, e.Reason)
}

// Code returns the error code for tool responses
func (e *FSMBlockedError) Code() string {
	return "FSM_BLOCKED"
}

// classifyToolByName classifies a tool by its name and action parameters.
// This is the centralized classification that new tools must be added to.
func classifyToolByName(toolName string, args map[string]interface{}) ToolKind {
	// Get action if present
	action, _ := args["action"].(string)
	actionLower := strings.ToLower(action)
	operation, _ := args["operation"].(string)
	operationLower := strings.ToLower(operation)

	switch toolName {
	case "pulse_question":
		return ToolKindUserInput

	// === Query/Discovery tools (Resolve) ===
	case "pulse_query":
		// query actions: search, get, config, topology, list, health
		return ToolKindResolve

	case "pulse_discovery":
		return ToolKindResolve

	// === Read-only tools (Read) ===
	case "pulse_metrics":
		return ToolKindRead

	case "pulse_alerts":
		// Most alert operations are read-only
		switch actionLower {
		case "resolve", "dismiss":
			return ToolKindWrite // These modify alert state
		default:
			return ToolKindRead
		}

	case "pulse_storage":
		return ToolKindRead

	case "pulse_kubernetes":
		return ToolKindRead

	case "pulse_knowledge":
		// knowledge operations: remember is write, recall is read
		switch actionLower {
		case "remember", "note", "save":
			return ToolKindWrite
		default:
			return ToolKindRead
		}

	case "pulse_pmg":
		return ToolKindRead

	case "pulse_read":
		// pulse_read is ALWAYS read-only - enforced at the tool layer
		// This tool never triggers VERIFYING state, even when running commands
		return ToolKindRead

	// === Control tools (Write) ===
	case "pulse_control":
		// pulse_control is always a write (guest control, run command)
		return ToolKindWrite

	case "pulse_docker":
		// Docker operations depend on action
		switch actionLower {
		case "control":
			return ToolKindWrite
		case "update", "check_updates", "trigger_update":
			return ToolKindWrite
		default:
			// services, tasks, swarm, list - read operations
			return ToolKindRead
		}

	case "pulse_file_edit":
		// File operations depend on action
		switch actionLower {
		case "read":
			return ToolKindRead
		case "write", "append":
			return ToolKindWrite
		default:
			return ToolKindRead
		}

	// === Legacy tool names (for backwards compatibility) ===
	case "pulse_run_command":
		return ToolKindWrite

	case "pulse_control_guest":
		return ToolKindWrite

	case "pulse_control_docker":
		return ToolKindWrite

	case "pulse_search_resources", "pulse_get_resource", "pulse_get_topology",
		"pulse_list_infrastructure", "pulse_get_connection_health":
		return ToolKindResolve

	case "pulse_get_docker_logs", "pulse_get_performance_metrics",
		"pulse_get_temperatures", "pulse_get_baselines", "pulse_get_patterns":
		return ToolKindRead

	// === Patrol tools ===
	case "patrol_get_findings":
		return ToolKindRead // Reading existing findings doesn't require discovery
	case "patrol_report_finding", "patrol_resolve_finding":
		return ToolKindWrite
	}

	// Check if the action/operation parameter indicates a write
	writeActions := map[string]bool{
		"start": true, "stop": true, "restart": true, "delete": true,
		"shutdown": true, "reboot": true, "write": true, "append": true,
		"update": true, "trigger": true, "resolve": true, "dismiss": true,
		"control": true,
	}
	if writeActions[actionLower] || writeActions[operationLower] {
		return ToolKindWrite
	}

	// Check if the action/operation parameter indicates a read
	readActions := map[string]bool{
		"get": true, "list": true, "search": true, "query": true,
		"read": true, "logs": true, "status": true, "health": true,
		"describe": true, "inspect": true, "show": true,
	}
	if readActions[actionLower] || readActions[operationLower] {
		return ToolKindRead
	}

	// Default to WRITE for unknown tools (security-safe: requires discovery first,
	// verification after). This ensures new tools don't accidentally bypass FSM gates.
	return ToolKindWrite
}

// ClassifyToolCall classifies a tool call for FSM state transitions.
// This is the exported function that the agentic loop should use.
func ClassifyToolCall(toolName string, args map[string]interface{}) ToolKind {
	return classifyToolByName(toolName, args)
}
