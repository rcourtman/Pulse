package relay

import (
	"fmt"
	"regexp"
	"strings"
)

// Notification type constants
const (
	PushTypePatrolFinding   = "patrol_finding"
	PushTypePatrolCritical  = "patrol_critical"
	PushTypeApprovalRequest = "approval_request"
	PushTypeFixCompleted    = "fix_completed"
)

// Priority constants
const (
	PushPriorityNormal = "normal"
	PushPriorityHigh   = "high"
)

// Action type constants
const (
	PushActionViewFinding   = "view_finding"
	PushActionApproveFix    = "approve_fix"
	PushActionViewFixResult = "view_fix_result"
)

// Title and body length limits for push payloads visible to Apple/Google.
const (
	maxPushTitleLen = 100
	maxPushBodyLen  = 200
)

// truncate returns s truncated to maxLen, appending "..." if shortened.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// ipv4Pattern matches IPv4 addresses regardless of surrounding punctuation
// (e.g. "192.168.1.10", "(192.168.1.10)", "[10.0.0.1]:8080").
// Each octet is constrained to 0-255.
var ipv4Pattern = regexp.MustCompile(
	`(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)` +
		`(?:\.(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}` +
		`(?::\d+)?`, // optional :port
)

// resourceNamePattern matches common Proxmox-style resource identifiers such as
// "node-1", "pve-cluster02", "vm-100", "ct-200", as well as hostnames and FQDNs
// that appear in patrol finding titles. These are replaced with a generic
// placeholder so push payloads (visible to Apple/Google) don't leak infra details.
var resourceNamePattern = regexp.MustCompile(
	// Proxmox-style identifiers: node/pve/vm/ct/qemu/lxc followed by separator and ID
	`\b(?:node|pve|vm|ct|qemu|lxc)[-_/]\S+` +
		`|` +
		// FQDN-like: word.word.tld (at least 3 dot-separated labels)
		`\b[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?){2,}\b`,
)

// sanitizeTitle removes infrastructure identifiers (hostnames, IPs, resource
// names) from a title so the push payload doesn't leak infra context.
func sanitizeTitle(title string) string {
	// Replace IPv4 addresses (handles any surrounding punctuation)
	title = ipv4Pattern.ReplaceAllString(title, "[resource]")

	// Replace resource name patterns
	title = resourceNamePattern.ReplaceAllString(title, "[resource]")

	// Collapse consecutive placeholders (possibly separated by whitespace only)
	for strings.Contains(title, "[resource] [resource]") {
		title = strings.ReplaceAll(title, "[resource] [resource]", "[resource]")
	}

	return title
}

// NewPatrolFindingNotification creates a push notification for a new patrol finding.
func NewPatrolFindingNotification(findingID, severity, category, title string) PushNotificationPayload {
	notifType := PushTypePatrolFinding
	priority := PushPriorityNormal
	if severity == "critical" {
		notifType = PushTypePatrolCritical
		priority = PushPriorityHigh
	}

	body := fmt.Sprintf("New %s %s finding detected", severity, category)

	return PushNotificationPayload{
		Type:       notifType,
		Priority:   priority,
		Title:      truncate(sanitizeTitle(title), maxPushTitleLen),
		Body:       truncate(body, maxPushBodyLen),
		ActionType: PushActionViewFinding,
		ActionID:   findingID,
		Category:   category,
		Severity:   severity,
	}
}

// NewApprovalRequestNotification creates a push notification for a fix needing approval.
func NewApprovalRequestNotification(approvalID, findingTitle, riskLevel string) PushNotificationPayload {
	body := "A proposed fix requires your approval"
	if riskLevel != "" {
		body = fmt.Sprintf("A %s-risk fix requires your approval", riskLevel)
	}

	return PushNotificationPayload{
		Type:       PushTypeApprovalRequest,
		Priority:   PushPriorityHigh,
		Title:      truncate(sanitizeTitle(findingTitle), maxPushTitleLen),
		Body:       truncate(body, maxPushBodyLen),
		ActionType: PushActionApproveFix,
		ActionID:   approvalID,
	}
}

// NewFixCompletedNotification creates a push notification for a completed fix.
func NewFixCompletedNotification(findingID, title string, success bool) PushNotificationPayload {
	body := "Fix applied successfully"
	if !success {
		body = "Fix attempt failed — review needed"
	}

	return PushNotificationPayload{
		Type:       PushTypeFixCompleted,
		Priority:   PushPriorityNormal,
		Title:      truncate(sanitizeTitle(title), maxPushTitleLen),
		Body:       truncate(body, maxPushBodyLen),
		ActionType: PushActionViewFixResult,
		ActionID:   findingID,
	}
}
