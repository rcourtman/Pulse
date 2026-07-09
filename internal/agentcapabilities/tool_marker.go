package agentcapabilities

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// ToolMarkerApprovalRequiredPrefix is the legacy-compatible marker used
	// when an Assistant tool needs operator approval before execution.
	ToolMarkerApprovalRequiredPrefix = ErrCodeApprovalRequired + ":"

	// ToolMarkerPolicyBlockedPrefix is the legacy-compatible marker used when
	// an Assistant tool is blocked by command policy.
	ToolMarkerPolicyBlockedPrefix = ErrCodePolicyBlocked + ":"

	// ToolMarkerApprovalRequiredType is the stable JSON payload type carried by
	// approval-required tool markers.
	ToolMarkerApprovalRequiredType = "approval_required"

	// ToolMarkerPolicyBlockedType is the stable JSON payload type carried by
	// policy-blocked tool markers.
	ToolMarkerPolicyBlockedType = "policy_blocked"
)

// ApprovalPlanData is the shared user-facing subset of a governed action plan
// carried inside approval-required tool markers.
type ApprovalPlanData struct {
	ActionID          string `json:"action_id,omitempty"`
	RequestID         string `json:"request_id,omitempty"`
	Summary           string `json:"summary,omitempty"`
	RequiresApproval  bool   `json:"requires_approval"`
	ApprovalPolicy    string `json:"approval_policy,omitempty"`
	BlastRadius       string `json:"blast_radius,omitempty"`
	RollbackAvailable bool   `json:"rollback_available"`
	PlanHash          string `json:"plan_hash,omitempty"`
	ExpiresAt         string `json:"expires_at,omitempty"`
}

// ApprovalContextConfidenceData describes how strongly a proposed action was
// bound to a concrete target before approval.
type ApprovalContextConfidenceData struct {
	Level    string   `json:"level,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
}

// ApprovalPreflightData describes the pre-execution dry-run and verification
// boundary carried inside approval-required tool markers.
type ApprovalPreflightData struct {
	Target            string   `json:"target,omitempty"`
	CurrentState      string   `json:"current_state,omitempty"`
	IntendedChange    string   `json:"intended_change,omitempty"`
	DryRunAvailable   bool     `json:"dry_run_available"`
	DryRunSummary     string   `json:"dry_run_summary,omitempty"`
	SafetyChecks      []string `json:"safety_checks,omitempty"`
	VerificationSteps []string `json:"verification_steps,omitempty"`
	GeneratedAt       string   `json:"generated_at,omitempty"`
}

// ApprovalRequiredToolMarkerData is the typed payload carried by
// APPROVAL_REQUIRED markers. It lives beside the marker formatter so native
// Assistant, external-agent adapters, and evaluation tools parse the same
// approval contract instead of each defining a local anonymous shape.
type ApprovalRequiredToolMarkerData struct {
	Type              string                         `json:"type,omitempty"`
	DoNotRetry        bool                           `json:"do_not_retry,omitempty"`
	ApprovalID        string                         `json:"approval_id,omitempty"`
	Command           string                         `json:"command,omitempty"`
	ToolID            string                         `json:"tool_id,omitempty"`
	Risk              string                         `json:"risk,omitempty"`
	Description       string                         `json:"description,omitempty"`
	Reason            string                         `json:"reason,omitempty"`
	HowToApprove      string                         `json:"how_to_approve,omitempty"`
	TargetHost        string                         `json:"target_host,omitempty"`
	Host              string                         `json:"host,omitempty"`
	DockerHost        string                         `json:"docker_host,omitempty"`
	ResourceHost      string                         `json:"resource_host,omitempty"`
	Cluster           string                         `json:"cluster,omitempty"`
	TargetName        string                         `json:"target_name,omitempty"`
	TargetType        string                         `json:"target_type,omitempty"`
	TargetID          string                         `json:"target_id,omitempty"`
	AuditID           string                         `json:"audit_id,omitempty"`
	Plan              *ApprovalPlanData              `json:"plan,omitempty"`
	ContextConfidence *ApprovalContextConfidenceData `json:"context_confidence,omitempty"`
	Preflight         *ApprovalPreflightData         `json:"preflight,omitempty"`
}

// TargetHint returns the best human-facing target label carried by an approval
// marker without making chat surfaces duplicate the same field-priority list.
func (d ApprovalRequiredToolMarkerData) TargetHint() string {
	return firstNonEmptyString(
		d.TargetHost,
		d.Host,
		d.DockerHost,
		d.ResourceHost,
		d.Cluster,
		d.TargetName,
	)
}

// DescriptionText returns the best human-facing reason carried by an approval
// marker.
func (d ApprovalRequiredToolMarkerData) DescriptionText() string {
	return firstNonEmptyString(d.Description, d.Reason)
}

// ApprovalRequiredToolMarker builds the common approval-required marker for a
// command-oriented tool. Callers with richer payload fields can use
// FormatApprovalRequiredToolMarker directly.
func ApprovalRequiredToolMarker(command, toolID, reason, approvalID, howToApprove string) string {
	payload := map[string]any{
		"command":        command,
		"tool_id":        toolID,
		"reason":         reason,
		"how_to_approve": howToApprove,
	}
	if approvalID = strings.TrimSpace(approvalID); approvalID != "" {
		payload["approval_id"] = approvalID
	}
	return FormatApprovalRequiredToolMarker(payload)
}

// PolicyBlockedToolMarker builds the common policy-blocked marker for a
// command-oriented tool. Callers with richer payload fields can use
// FormatPolicyBlockedToolMarker directly.
func PolicyBlockedToolMarker(command, reason string) string {
	return FormatPolicyBlockedToolMarker(map[string]any{
		"command": command,
		"reason":  reason,
	})
}

// FormatApprovalRequiredToolMarker formats a caller-provided payload with the
// shared approval marker prefix and required machine-readable fields.
func FormatApprovalRequiredToolMarker(payload map[string]any) string {
	return formatToolMarker(ToolMarkerApprovalRequiredPrefix, ToolMarkerApprovalRequiredType, payload, "command", "reason")
}

// FormatPolicyBlockedToolMarker formats a caller-provided payload with the
// shared policy marker prefix and required machine-readable fields.
func FormatPolicyBlockedToolMarker(payload map[string]any) string {
	return formatToolMarker(ToolMarkerPolicyBlockedPrefix, ToolMarkerPolicyBlockedType, payload, "reason", "command")
}

// HasApprovalRequiredToolMarker reports whether content starts with the shared
// approval-required marker prefix. It intentionally accepts both
// "APPROVAL_REQUIRED:{...}" and "APPROVAL_REQUIRED: {...}" legacy forms.
func HasApprovalRequiredToolMarker(content string) bool {
	return strings.HasPrefix(content, ToolMarkerApprovalRequiredPrefix)
}

// HasPolicyBlockedToolMarker reports whether content starts with the shared
// policy-blocked marker prefix.
func HasPolicyBlockedToolMarker(content string) bool {
	return strings.HasPrefix(content, ToolMarkerPolicyBlockedPrefix)
}

// ApprovalRequiredToolMarkerPayloadJSON returns the marker payload bytes when
// content carries the shared approval-required prefix.
func ApprovalRequiredToolMarkerPayloadJSON(content string) ([]byte, bool) {
	return toolMarkerPayloadJSON(content, ToolMarkerApprovalRequiredPrefix)
}

// ParseApprovalRequiredToolMarkerPayload parses a shared approval-required
// marker into a generic payload map and verifies the stable payload type.
func ParseApprovalRequiredToolMarkerPayload(content string) (map[string]any, bool) {
	return parseToolMarkerPayload(content, ToolMarkerApprovalRequiredPrefix, ToolMarkerApprovalRequiredType)
}

// ParseApprovalRequiredToolMarkerData parses an approval-required marker into
// the shared typed payload contract.
func ParseApprovalRequiredToolMarkerData(content string) (ApprovalRequiredToolMarkerData, bool) {
	raw, ok := ApprovalRequiredToolMarkerPayloadJSON(content)
	if !ok {
		return ApprovalRequiredToolMarkerData{}, false
	}
	var data ApprovalRequiredToolMarkerData
	if err := json.Unmarshal(raw, &data); err != nil {
		return ApprovalRequiredToolMarkerData{}, false
	}
	if data.Type != ToolMarkerApprovalRequiredType {
		return ApprovalRequiredToolMarkerData{}, false
	}
	return data, true
}

// ParsePolicyBlockedToolMarkerPayload parses a shared policy-blocked marker
// into a generic payload map and verifies the stable payload type.
func ParsePolicyBlockedToolMarkerPayload(content string) (map[string]any, bool) {
	return parseToolMarkerPayload(content, ToolMarkerPolicyBlockedPrefix, ToolMarkerPolicyBlockedType)
}

func formatToolMarker(prefix, payloadType string, payload map[string]any, fallbackKeys ...string) string {
	prepared := map[string]any{}
	for k, v := range payload {
		prepared[k] = v
	}
	prepared["type"] = payloadType
	prepared["do_not_retry"] = true

	body, err := json.Marshal(prepared)
	if err != nil {
		fallback := firstStringPayloadValue(prepared, fallbackKeys...)
		if fallback == "" {
			fallback = payloadType
		}
		return fmt.Sprintf("%s %s", prefix, fallback)
	}
	return fmt.Sprintf("%s %s", prefix, body)
}

func toolMarkerPayloadJSON(content, prefix string) ([]byte, bool) {
	if !strings.HasPrefix(content, prefix) {
		return nil, false
	}
	payload := strings.TrimSpace(strings.TrimPrefix(content, prefix))
	if payload == "" {
		return nil, false
	}
	return []byte(payload), true
}

func parseToolMarkerPayload(content, prefix, payloadType string) (map[string]any, bool) {
	raw, ok := toolMarkerPayloadJSON(content, prefix)
	if !ok {
		return nil, false
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false
	}
	if payload["type"] != payloadType {
		return nil, false
	}
	return payload, true
}

func firstStringPayloadValue(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
