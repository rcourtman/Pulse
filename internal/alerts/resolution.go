package alerts

import (
	"fmt"
	"strings"
)

// AlertResolutionReason says why an alert closed when the close was not a
// recovery. The close still reaches every resolved consumer, so an incident
// opened from the alert does not stay open on the receiving side, but no
// surface may report the resource as healthy.
type AlertResolutionReason string

const (
	// AlertResolutionMovedToAgent means a Pulse agent linked to the alert's
	// resource now owns the same metric, so the agent's own alert settings
	// decide whether the condition is still alerted on.
	AlertResolutionMovedToAgent AlertResolutionReason = "moved_to_agent"
)

// AlertResolution records why an alert closed without its condition being
// observed to clear. A nil resolution is an ordinary recovery.
type AlertResolution struct {
	Reason AlertResolutionReason `json:"reason"`
	// SuccessorResourceID and SuccessorName identify the resource that owns
	// the alert from now on, named the way that resource's own alerts are.
	SuccessorResourceID string `json:"successorResourceId,omitempty"`
	SuccessorName       string `json:"successorName,omitempty"`
}

// Clone returns a copy of the resolution.
func (r *AlertResolution) Clone() *AlertResolution {
	if r == nil {
		return nil
	}
	clone := *r
	return &clone
}

// Outcome is the short form of a close that was not a recovery, for lists
// such as "moved to pve1 (Host Agent)". It is empty for an ordinary recovery.
func (r *AlertResolution) Outcome() string {
	if r == nil {
		return ""
	}
	switch r.Reason {
	case AlertResolutionMovedToAgent:
		successor := strings.TrimSpace(r.SuccessorName)
		if successor == "" {
			successor = "its Pulse agent"
		}
		return "moved to " + successor
	default:
		return ""
	}
}

// Summary is the one-line account of a close that was not a recovery. It is
// empty for an ordinary recovery.
func (r *AlertResolution) Summary() string {
	return r.Describe("Alert")
}

// Describe is Summary with the alert named by the caller, such as
// "Memory alert", so a message about one of several closes says which one.
func (r *AlertResolution) Describe(subject string) string {
	if r == nil {
		return ""
	}
	if subject = strings.TrimSpace(subject); subject == "" {
		subject = "Alert"
	}
	switch r.Reason {
	case AlertResolutionMovedToAgent:
		return fmt.Sprintf("%s %s. This is not a recovery: check the agent for the current reading.", subject, r.Outcome())
	default:
		return ""
	}
}
