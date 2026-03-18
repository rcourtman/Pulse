package unifiedresources

// ActionApprovalLevel describes what kind of confirmation is required to execute a capability.
type ActionApprovalLevel string

const (
	ApprovalNone        ActionApprovalLevel = "none"         // Safe to auto-execute without human
	ApprovalDryRun      ActionApprovalLevel = "dry_run_only" // AI can only plan/dry-run this action
	ApprovalAdmin       ActionApprovalLevel = "admin"        // Requires an administrator confirmation
	ApprovalMultiFactor ActionApprovalLevel = "mfa"          // Requires strict explicit 2FA
)

// CapabilityType represents a standard unified intent or a vendor-specific escape hatch.
type CapabilityType string

const (
	CapabilityTypeCommon CapabilityType = "common"
	CapabilityTypeNative CapabilityType = "native"
)

// CapabilityParam defines the input schema for a capability.
type CapabilityParam struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"` // e.g., "string", "boolean", "int"
	Required     bool     `json:"required"`
	Enum         []string `json:"enum,omitempty"`
	Pattern      string   `json:"pattern,omitempty"` // Regex validation
	DefaultValue any      `json:"defaultValue,omitempty"`
	IsSensitive  bool     `json:"isSensitive"` // True = masked from persistent audit logs
	Description  string   `json:"description,omitempty"`
}

// ResourceCapability defines a bounded safe action that can be performed against a resource.
type ResourceCapability struct {
	Name                 string              `json:"name"`
	Type                 CapabilityType      `json:"type"`
	Description          string              `json:"description"`
	MinimumApprovalLevel ActionApprovalLevel `json:"minimumApprovalLevel"` // Policy may escalate this
	Platform             string              `json:"platform,omitempty"`
	InternalHandler      string              `json:"-"` // DO NOT expose execution plumbing to public surfaces
	Params               []CapabilityParam   `json:"params,omitempty"`
}
