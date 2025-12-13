// Package ai provides AI-powered infrastructure investigation and remediation.
package ai

import (
	"fmt"
	"strings"
	"time"
)

// AlertInfo contains information about an alert for AI context
type AlertInfo struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`          // cpu, memory, disk, offline, etc.
	Level        string    `json:"level"`         // warning, critical
	ResourceID   string    `json:"resource_id"`   // unique resource identifier
	ResourceName string    `json:"resource_name"` // human-readable name
	ResourceType string    `json:"resource_type"` // guest, node, storage, docker, etc.
	Node         string    `json:"node"`          // PVE node (if applicable)
	Instance     string    `json:"instance"`      // Proxmox instance name
	Message      string    `json:"message"`       // Alert description
	Value        float64   `json:"value"`         // Current metric value
	Threshold    float64   `json:"threshold"`     // Threshold that was exceeded
	StartTime    time.Time `json:"start_time"`    // When alert started
	Duration     string    `json:"duration"`      // Human-readable duration
	Acknowledged bool      `json:"acknowledged"`  // Whether alert has been acked
}

// ResolvedAlertInfo contains information about a recently resolved alert
type ResolvedAlertInfo struct {
	AlertInfo
	ResolvedTime time.Time `json:"resolved_time"`
	Duration     string    `json:"total_duration"` // How long the alert lasted
}

// AlertProvider provides access to the current alert state
type AlertProvider interface {
	// GetActiveAlerts returns all currently active alerts
	GetActiveAlerts() []AlertInfo

	// GetRecentlyResolved returns alerts resolved in the last N minutes
	GetRecentlyResolved(minutes int) []ResolvedAlertInfo

	// GetAlertsByResource returns active alerts for a specific resource
	GetAlertsByResource(resourceID string) []AlertInfo

	// GetAlertHistory returns historical alerts for a resource
	GetAlertHistory(resourceID string, limit int) []ResolvedAlertInfo
}

// SetAlertProvider sets the alert provider for AI context
func (s *Service) SetAlertProvider(ap AlertProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alertProvider = ap
}

// buildAlertContext generates AI context from current alerts
func (s *Service) buildAlertContext() string {
	s.mu.RLock()
	ap := s.alertProvider
	s.mu.RUnlock()

	if ap == nil {
		return ""
	}

	activeAlerts := ap.GetActiveAlerts()
	recentlyResolved := ap.GetRecentlyResolved(30) // Last 30 minutes

	if len(activeAlerts) == 0 && len(recentlyResolved) == 0 {
		return ""
	}

	var sections []string
	sections = append(sections, "\n## Alert Status")

	// Active alerts
	if len(activeAlerts) > 0 {
		sections = append(sections, "\n### Active Alerts")
		sections = append(sections, fmt.Sprintf("There are **%d active alert(s)** that may need attention:\n", len(activeAlerts)))

		// Group by severity
		var critical, warning []AlertInfo
		for _, a := range activeAlerts {
			if a.Level == "critical" {
				critical = append(critical, a)
			} else {
				warning = append(warning, a)
			}
		}

		if len(critical) > 0 {
			sections = append(sections, "**Critical:**")
			for _, a := range critical {
				sections = append(sections, formatAlertForAI(a))
			}
		}

		if len(warning) > 0 {
			sections = append(sections, "**Warning:**")
			for _, a := range warning {
				sections = append(sections, formatAlertForAI(a))
			}
		}
	} else {
		sections = append(sections, "\n### No Active Alerts")
		sections = append(sections, "All systems are operating within normal thresholds.")
	}

	// Recently resolved
	if len(recentlyResolved) > 0 {
		sections = append(sections, fmt.Sprintf("\n### Recently Resolved (%d)", len(recentlyResolved)))
		sections = append(sections, "These alerts were resolved in the last 30 minutes:")
		// Show up to 5 most recent
		limit := 5
		if len(recentlyResolved) < limit {
			limit = len(recentlyResolved)
		}
		for i := 0; i < limit; i++ {
			a := recentlyResolved[i]
			sections = append(sections, fmt.Sprintf("- **%s** on %s: %s (lasted %s, resolved %s ago)",
				a.Type, a.ResourceName, a.Message, a.Duration,
				formatTimeAgo(a.ResolvedTime)))
		}
		if len(recentlyResolved) > limit {
			sections = append(sections, fmt.Sprintf("  ... and %d more", len(recentlyResolved)-limit))
		}
	}

	return strings.Join(sections, "\n")
}

// buildTargetAlertContext builds alert context for a specific target
func (s *Service) buildTargetAlertContext(resourceID string) string {
	s.mu.RLock()
	ap := s.alertProvider
	s.mu.RUnlock()

	if ap == nil || resourceID == "" {
		return ""
	}

	alerts := ap.GetAlertsByResource(resourceID)
	if len(alerts) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "\n### Active Alerts for This Resource")
	for _, a := range alerts {
		lines = append(lines, formatAlertForAI(a))
	}

	return strings.Join(lines, "\n")
}

// formatAlertForAI formats an alert for inclusion in AI context
func formatAlertForAI(a AlertInfo) string {
	ackedNote := ""
	if a.Acknowledged {
		ackedNote = " [ACKNOWLEDGED]"
	}

	nodeInfo := ""
	if a.Node != "" {
		nodeInfo = fmt.Sprintf(" on node %s", a.Node)
	}

	return fmt.Sprintf("- **%s** %s: %s (current: %.1f%%, threshold: %.1f%%) - active for %s%s%s",
		strings.ToUpper(a.Level), a.Type, a.ResourceName,
		a.Value, a.Threshold, a.Duration, nodeInfo, ackedNote)
}

// formatTimeAgo returns a human-readable time-ago string
func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	} else if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", mins)
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

// AlertInvestigationRequest represents a request to investigate an alert
type AlertInvestigationRequest struct {
	AlertID      string `json:"alert_id"`
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	ResourceType string `json:"resource_type"` // guest, node, storage, docker
	AlertType    string `json:"alert_type"`    // cpu, memory, disk, offline, etc.
	Level        string `json:"level"`         // warning, critical
	Value        float64 `json:"value"`
	Threshold    float64 `json:"threshold"`
	Message      string `json:"message"`
	Duration     string `json:"duration"` // How long the alert has been active
	Node         string `json:"node,omitempty"`
	VMID         int    `json:"vmid,omitempty"`
}

// GenerateAlertInvestigationPrompt creates a focused prompt for alert investigation
func GenerateAlertInvestigationPrompt(req AlertInvestigationRequest) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("Investigate this %s alert:\n\n", strings.ToUpper(req.Level)))
	prompt.WriteString(fmt.Sprintf("**Resource:** %s (%s)\n", req.ResourceName, req.ResourceType))
	prompt.WriteString(fmt.Sprintf("**Alert Type:** %s\n", req.AlertType))
	prompt.WriteString(fmt.Sprintf("**Current Value:** %.1f%%\n", req.Value))
	prompt.WriteString(fmt.Sprintf("**Threshold:** %.1f%%\n", req.Threshold))
	prompt.WriteString(fmt.Sprintf("**Duration:** %s\n", req.Duration))

	if req.Node != "" {
		prompt.WriteString(fmt.Sprintf("**Node:** %s\n", req.Node))
	}

	prompt.WriteString("\n**Action Required:**\n")
	prompt.WriteString("1. Identify the root cause of this alert\n")
	prompt.WriteString("2. Check related metrics and system state\n")
	prompt.WriteString("3. Suggest specific remediation steps\n")
	prompt.WriteString("4. If safe, execute diagnostic commands to gather more info\n")

	return prompt.String()
}
