package agentexec

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// MessageType identifies the type of WebSocket message
type MessageType string

const (
	// Agent -> Server messages
	MsgTypeAgentRegister            MessageType = "agent_register"
	MsgTypeAgentPing                MessageType = "agent_ping"
	MsgTypeCommandResult            MessageType = "command_result"
	MsgTypeHostStorageCleanupResult MessageType = "host_storage_cleanup_result"
	MsgTypeHostUpdateResult         MessageType = "host_update_result"

	// Server -> Agent messages
	MsgTypeRegistered         MessageType = "registered"
	MsgTypePong               MessageType = "pong"
	MsgTypeExecuteCmd         MessageType = "execute_command"
	MsgTypeHostStorageCleanup MessageType = "host_storage_cleanup"
	MsgTypeReadFile           MessageType = "read_file"
	MsgTypeHostUpdate         MessageType = "host_update"
	MsgTypeDeployPreflight    MessageType = "deploy_preflight"
	MsgTypeDeployInstall      MessageType = "deploy_install"
	MsgTypeDeployCancelJob    MessageType = "deploy_cancel"

	// Agent -> Server messages (deploy)
	MsgTypeDeployProgress MessageType = "deploy_progress"
)

// Message is the envelope for all WebSocket messages
type Message struct {
	Type      MessageType     `json:"type"`
	ID        string          `json:"id,omitempty"` // Unique message ID for request/response correlation
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// NewMessage creates a message envelope with a safely marshaled payload.
func NewMessage(messageType MessageType, id string, payload any) (Message, error) {
	msg := Message{
		Type:      messageType,
		ID:        id,
		Timestamp: time.Now(),
	}
	if err := msg.SetPayload(payload); err != nil {
		return Message{}, err
	}
	return msg, nil
}

// SetPayload marshals and sets the payload for this message.
func (m *Message) SetPayload(payload any) error {
	if payload == nil {
		m.Payload = nil
		return nil
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	m.Payload = encoded
	return nil
}

// DecodePayload unmarshals the message payload into target.
func (m Message) DecodePayload(target any) error {
	if len(m.Payload) == 0 {
		return errors.New("message payload is empty")
	}
	return json.Unmarshal(m.Payload, target)
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
	RequestID     string                `json:"request_id"`
	Command       string                `json:"command"`
	ApprovalID    string                `json:"approval_id,omitempty"`
	ApprovalGrant *CommandApprovalGrant `json:"approval_grant,omitempty"`
	TargetType    string                `json:"target_type"`         // "agent", "container", "vm"
	TargetID      string                `json:"target_id,omitempty"` // VMID for container/VM
	Timeout       int                   `json:"timeout,omitempty"`   // seconds, 0 = default
	// Trusted marks a payload as originating from a Pulse-internal subsystem
	// whose command catalog is hardcoded and vetted (e.g. servicediscovery
	// deep scans wrap read-only inspections in `docker exec`). When set, the
	// server's command policy approval gate is bypassed for this command.
	// This field must only be set by trusted internal call sites — never
	// from a deserialised HTTP body or any user-driven path.
	Trusted bool `json:"trusted,omitempty"`

	authorization *commandAuthorizationContext
}

type commandAuthorizationContext struct {
	OrgID    string
	ActionID string
}

// BindCommandAuthorization attaches server-owned approval scope to a command.
// The scope is deliberately not serializable, so provider arguments, HTTP
// payloads, relay callers, and agent wire messages cannot manufacture it.
func (p *ExecuteCommandPayload) BindCommandAuthorization(orgID, actionID string) {
	if p == nil {
		return
	}
	p.authorization = &commandAuthorizationContext{
		OrgID:    strings.TrimSpace(orgID),
		ActionID: strings.TrimSpace(actionID),
	}
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

const HostUpdateOperationInstall = "install_os_updates"

// HostUpdatePayload is the closed, typed host-package operation sent to a
// Unified Agent. It intentionally has no command or package-name fields: the
// agent owns the package-manager command catalog and always updates the whole
// bounded set returned by its own preflight simulation.
type HostUpdatePayload struct {
	RequestID             string `json:"request_id"`
	ActionID              string `json:"action_id"`
	Operation             string `json:"operation"`
	ExpectedInventoryHash string `json:"expected_inventory_hash"`
	Timeout               int    `json:"timeout,omitempty"`
}

type HostPackageUpdate struct {
	Name             string `json:"name"`
	InstalledVersion string `json:"installed_version,omitempty"`
	AvailableVersion string `json:"available_version,omitempty"`
}

type HostPackageUpdateSnapshot struct {
	Supported      bool                `json:"supported"`
	Manager        string              `json:"manager,omitempty"`
	InventoryHash  string              `json:"inventory_hash,omitempty"`
	PendingCount   int                 `json:"pending_count"`
	Packages       []HostPackageUpdate `json:"packages,omitempty"`
	CheckedAt      time.Time           `json:"checked_at,omitempty"`
	RebootRequired bool                `json:"reboot_required,omitempty"`
	Error          string              `json:"error,omitempty"`
}

const (
	HostUpdateVerificationVerified     = "verified"
	HostUpdateVerificationFailed       = "failed"
	HostUpdateVerificationInconclusive = "inconclusive"
)

// HostUpdateResultPayload carries execution and read-after-write evidence
// independently. Success means the package-manager mutation completed;
// Verification states whether the postcondition was actually observed.
type HostUpdateResultPayload struct {
	RequestID    string                    `json:"request_id"`
	Success      bool                      `json:"success"`
	Before       HostPackageUpdateSnapshot `json:"before"`
	After        HostPackageUpdateSnapshot `json:"after"`
	Verification string                    `json:"verification"`
	Error        string                    `json:"error,omitempty"`
	Duration     int64                     `json:"duration_ms"`
}

const HostStorageCleanupOperationPackageCache = "clean_package_cache"

const HostStorageCleanupMaxReportedBytes int64 = 1 << 60

// HostStorageCleanupPayload is the closed storage-pressure operation sent to
// a Unified Agent. The model cannot select paths or commands: the agent owns
// the single APT package-cache target and command catalog.
type HostStorageCleanupPayload struct {
	RequestID           string `json:"request_id"`
	ActionID            string `json:"action_id"`
	Operation           string `json:"operation"`
	ExpectedFingerprint string `json:"expected_fingerprint"`
	Timeout             int    `json:"timeout,omitempty"`
}

type HostStorageCleanupSnapshot struct {
	Supported        bool      `json:"supported"`
	Provider         string    `json:"provider,omitempty"`
	Fingerprint      string    `json:"fingerprint,omitempty"`
	ReclaimableBytes int64     `json:"reclaimable_bytes"`
	CheckedAt        time.Time `json:"checked_at,omitempty"`
	Error            string    `json:"error,omitempty"`
}

const (
	HostStorageCleanupVerificationVerified     = "verified"
	HostStorageCleanupVerificationFailed       = "failed"
	HostStorageCleanupVerificationInconclusive = "inconclusive"
)

// HostStorageCleanupResultPayload separates mutation success from the
// read-after-write observation of reclaimed cache bytes.
type HostStorageCleanupResultPayload struct {
	RequestID      string                     `json:"request_id"`
	Success        bool                       `json:"success"`
	Before         HostStorageCleanupSnapshot `json:"before"`
	After          HostStorageCleanupSnapshot `json:"after"`
	ReclaimedBytes int64                      `json:"reclaimed_bytes"`
	Verification   string                     `json:"verification"`
	Error          string                     `json:"error,omitempty"`
	Duration       int64                      `json:"duration_ms"`
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

// --- Deploy protocol payloads ---

const MaxDeployParallel = 10

// NormalizeDeployMaxParallel applies the shared server/agent concurrency
// contract before the value reaches worker or channel allocation boundaries.
func NormalizeDeployMaxParallel(value, defaultValue int) int {
	if value <= 0 {
		value = defaultValue
	}
	if value > MaxDeployParallel {
		return MaxDeployParallel
	}
	return value
}

// DeployPreflightPayload is sent by the server to request SSH preflight checks
// against cluster peer nodes.
type DeployPreflightPayload struct {
	RequestID   string                  `json:"request_id"`
	JobID       string                  `json:"job_id"`
	Targets     []DeployPreflightTarget `json:"targets"`
	PulseURL    string                  `json:"pulse_url"`              // URL peers must reach
	MaxParallel int                     `json:"max_parallel,omitempty"` // 0 = sequential
	Timeout     int                     `json:"timeout,omitempty"`      // per-target seconds, 0 = 120
}

// DeployPreflightTarget identifies a single node to preflight-check via SSH.
type DeployPreflightTarget struct {
	TargetID string `json:"target_id"` // deploy.Target.ID
	NodeName string `json:"node_name"` // Proxmox node name
	NodeIP   string `json:"node_ip"`   // IP to SSH into
}

// DeployInstallPayload is sent by the server to request agent installation
// on cluster peer nodes via SSH.
type DeployInstallPayload struct {
	RequestID   string                `json:"request_id"`
	JobID       string                `json:"job_id"`
	Targets     []DeployInstallTarget `json:"targets"`
	PulseURL    string                `json:"pulse_url"`
	MaxParallel int                   `json:"max_parallel,omitempty"`
	Timeout     int                   `json:"timeout,omitempty"` // per-target seconds, 0 = 300
}

// DeployInstallTarget identifies a single node for agent installation via SSH.
type DeployInstallTarget struct {
	TargetID       string `json:"target_id"` // deploy.Target.ID
	NodeName       string `json:"node_name"`
	NodeIP         string `json:"node_ip"`
	Arch           string `json:"arch"`            // from preflight: amd64, arm64
	BootstrapToken string `json:"bootstrap_token"` // per-target enrollment token
}

// DeployCancelPayload is sent by the server to cancel an in-flight deploy job.
type DeployCancelPayload struct {
	RequestID string `json:"request_id"`
	JobID     string `json:"job_id"`
}

// DeployProgressPayload is sent by the agent to stream progress events for
// preflight checks and install operations. Multiple progress messages are
// sent per request, with the final message having Final=true.
type DeployProgressPayload struct {
	RequestID string              `json:"request_id"`
	JobID     string              `json:"job_id"`
	TargetID  string              `json:"target_id,omitempty"` // empty for job-level events
	Phase     DeployProgressPhase `json:"phase"`
	Status    DeployStepStatus    `json:"status"`
	Message   string              `json:"message"`
	Data      string              `json:"data,omitempty"` // JSON blob for structured results
	Final     bool                `json:"final"`          // true = last message for this request
}

// DeployProgressPhase identifies which stage of the deploy lifecycle this
// progress event belongs to.
type DeployProgressPhase string

const (
	DeployPhasePreflightSSH      DeployProgressPhase = "preflight_ssh"
	DeployPhasePreflightArch     DeployProgressPhase = "preflight_arch"
	DeployPhasePreflightReach    DeployProgressPhase = "preflight_reachability"
	DeployPhasePreflightAgent    DeployProgressPhase = "preflight_existing_agent"
	DeployPhasePreflightComplete DeployProgressPhase = "preflight_complete"
	DeployPhaseInstallTransfer   DeployProgressPhase = "install_transfer"
	DeployPhaseInstallExecute    DeployProgressPhase = "install_execute"
	DeployPhaseInstallEnrollWait DeployProgressPhase = "install_enroll_wait"
	DeployPhaseInstallComplete   DeployProgressPhase = "install_complete"
	DeployPhaseCanceled          DeployProgressPhase = "canceled"
	DeployPhaseJobComplete       DeployProgressPhase = "job_complete"
)

// DeployStepStatus is the outcome of a single progress step.
type DeployStepStatus string

const (
	DeployStepStarted DeployStepStatus = "started"
	DeployStepOK      DeployStepStatus = "ok"
	DeployStepFailed  DeployStepStatus = "failed"
	DeployStepSkipped DeployStepStatus = "skipped"
)

// PreflightResultData is the structured data for a completed preflight check.
// Serialized as JSON in DeployProgressPayload.Data.
type PreflightResultData struct {
	Arch           string `json:"arch,omitempty"`
	HasAgent       bool   `json:"has_agent"`
	PulseReachable bool   `json:"pulse_reachable"`
	SSHReachable   bool   `json:"ssh_reachable"`
	ErrorDetail    string `json:"error_detail,omitempty"`
}

// InstallResultData is the structured data for a completed install step.
// Serialized as JSON in DeployProgressPayload.Data.
type InstallResultData struct {
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output,omitempty"`
}
