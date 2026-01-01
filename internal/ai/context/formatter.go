package context

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// FormatResourceContext formats a single resource's context for AI consumption
func FormatResourceContext(ctx ResourceContext) string {
	var sb strings.Builder

	// Header with resource identity
	typeLabel := formatResourceType(ctx.ResourceType)
	sb.WriteString(fmt.Sprintf("### %s: %s", typeLabel, ctx.ResourceName))
	if ctx.VMID > 0 {
		sb.WriteString(fmt.Sprintf(" (VMID %d)", ctx.VMID))
	}
	if ctx.Node != "" && ctx.ResourceType != "node" {
		sb.WriteString(fmt.Sprintf(" on %s", ctx.Node))
	}
	sb.WriteString("\n")

	// Current state
	sb.WriteString(fmt.Sprintf("**Status**: %s", ctx.Status))
	if ctx.Uptime > 0 {
		sb.WriteString(fmt.Sprintf(" | **Uptime**: %s", formatDuration(ctx.Uptime)))
	}
	sb.WriteString("\n")

	// Current metrics
	var metrics []string
	if ctx.CurrentCPU >= 0 {
		metrics = append(metrics, fmt.Sprintf("CPU: %.1f%%", ctx.CurrentCPU))
	}
	if ctx.CurrentMemory >= 0 {
		metrics = append(metrics, fmt.Sprintf("Memory: %.1f%%", ctx.CurrentMemory))
	}
	if ctx.CurrentDisk >= 0 {
		metrics = append(metrics, fmt.Sprintf("Disk: %.1f%%", ctx.CurrentDisk))
	}
	if len(metrics) > 0 {
		sb.WriteString("**Current**: " + strings.Join(metrics, " | ") + "\n")
	}

	// Trends section (computed summaries - kept for backwards compatibility)
	if len(ctx.Trends) > 0 {
		var trendLines []string
		for metric, trend := range ctx.Trends {
			if trend.DataPoints < 3 {
				continue // Skip if not enough data
			}
			line := formatTrendLine(metric, trend)
			if line != "" {
				trendLines = append(trendLines, line)
			}
		}
		if len(trendLines) > 0 {
			sb.WriteString("**Trends**: ")
			sb.WriteString(strings.Join(trendLines, " | "))
			sb.WriteString("\n")
		}
	}

	// Raw metric samples - let the LLM interpret patterns directly
	// This is more reliable than pre-computed trends for edge cases
	if len(ctx.MetricSamples) > 0 {
		sb.WriteString("**History (24h sampled, oldest→newest)**: ")
		var sampleLines []string
		for metric, points := range ctx.MetricSamples {
			if len(points) >= 3 {
				line := formatMetricSamples(metric, points)
				if line != "" {
					sampleLines = append(sampleLines, line)
				}
			}
		}
		if len(sampleLines) > 0 {
			sb.WriteString(strings.Join(sampleLines, " | "))
		}
		sb.WriteString("\n")
	}

	// Anomalies (high value - what's unusual)
	if len(ctx.Anomalies) > 0 {
		sb.WriteString("**ANOMALIES**: ")
		var anomalyDescs []string
		for _, a := range ctx.Anomalies {
			anomalyDescs = append(anomalyDescs, a.Description)
		}
		sb.WriteString(strings.Join(anomalyDescs, "; "))
		sb.WriteString("\n")
	}

	// Predictions (proactive value)
	if len(ctx.Predictions) > 0 {
		sb.WriteString("**Predictions**: ")
		var predDescs []string
		for _, p := range ctx.Predictions {
			predDescs = append(predDescs, fmt.Sprintf("%s in ~%.0f days", p.Event, p.DaysUntil))
		}
		sb.WriteString(strings.Join(predDescs, "; "))
		sb.WriteString("\n")
	}

	// User notes (context that only Pulse knows)
	if len(ctx.UserNotes) > 0 {
		sb.WriteString("**User Notes**: ")
		sb.WriteString(strings.Join(ctx.UserNotes, "; "))
		sb.WriteString("\n")
	}

	// Past issues (operational memory)
	if len(ctx.PastIssues) > 0 || ctx.LastRemediation != "" {
		sb.WriteString("**History**: ")
		if ctx.LastRemediation != "" {
			sb.WriteString(ctx.LastRemediation)
		}
		if len(ctx.PastIssues) > 0 {
			sb.WriteString(" Past issues: " + strings.Join(ctx.PastIssues, "; "))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatTrendLine creates a compact trend description
func formatTrendLine(metric string, trend Trend) string {
	if trend.DataPoints < 3 {
		return ""
	}

	metricLabel := strings.Title(metric)

	// Direction with rate
	var directionStr string
	switch trend.Direction {
	case TrendGrowing:
		rate := formatRate(trend.RatePerDay)
		directionStr = fmt.Sprintf("(rising %s)", rate)
	case TrendDeclining:
		rate := formatRate(-trend.RatePerDay) // Make positive for display
		directionStr = fmt.Sprintf("(falling %s)", rate)
	case TrendVolatile:
		directionStr = "(volatile)"
	case TrendStable:
		directionStr = "(stable)"
	default:
		return ""
	}

	// Include range if interesting
	rangeStr := ""
	if trend.Max-trend.Min > 5 { // Only show range if variation is significant
		rangeStr = fmt.Sprintf(" (%.0f-%.0f%%)", trend.Min, trend.Max)
	}

	return fmt.Sprintf("%s: %s%s", metricLabel, directionStr, rangeStr)
}

// formatRate formats a rate value appropriately
func formatRate(ratePerDay float64) string {
	absRate := ratePerDay
	if absRate < 0 {
		absRate = -absRate
	}

	if absRate >= 1 {
		return fmt.Sprintf("%.1f/day", absRate)
	}
	// Convert to per hour if < 1/day
	ratePerHour := absRate / 24
	if ratePerHour >= 0.1 {
		return fmt.Sprintf("%.1f/hr", ratePerHour)
	}
	return "slow"
}

// formatMetricSamples creates a compact representation of sampled values
// Example output: "Disk: 26→26→26→31→31→31" (shows step change visually)
// This lets the LLM interpret patterns directly rather than relying on computed rates
func formatMetricSamples(metric string, points []MetricPoint) string {
	if len(points) < 3 {
		return ""
	}

	metricLabel := strings.Title(metric)

	// Build compact arrow-separated value list
	var values []string
	prevValue := -1.0
	for _, p := range points {
		roundedValue := float64(int(p.Value + 0.5)) // Round to nearest integer
		// Skip consecutive duplicates for compactness
		if roundedValue == prevValue && len(values) > 0 {
			continue
		}
		values = append(values, fmt.Sprintf("%.0f", roundedValue))
		prevValue = roundedValue
	}

	// If all values are the same, just show "stable at X%"
	if len(values) == 1 {
		return fmt.Sprintf("%s: stable at %.0f%%", metricLabel, prevValue)
	}

	// Join with arrows to show progression
	return fmt.Sprintf("%s: %s%%", metricLabel, strings.Join(values, "→"))
}

// DownsampleMetrics takes raw metric points and returns a smaller set for LLM consumption
// It aims for about 10-15 samples across the time range, picking representative values
func DownsampleMetrics(points []MetricPoint, targetSamples int) []MetricPoint {
	if len(points) <= targetSamples {
		return points
	}

	if targetSamples < 3 {
		targetSamples = 3
	}

	// Calculate step size
	step := len(points) / targetSamples
	if step < 1 {
		step = 1
	}

	var sampled []MetricPoint
	for i := 0; i < len(points); i += step {
		sampled = append(sampled, points[i])
	}

	// Always include the last point (current value)
	if len(sampled) > 0 && sampled[len(sampled)-1].Timestamp != points[len(points)-1].Timestamp {
		sampled = append(sampled, points[len(points)-1])
	}

	return sampled
}

// FormatInfrastructureContext formats full infrastructure context for AI
func FormatInfrastructureContext(ctx *InfrastructureContext) string {
	var sb strings.Builder

	sb.WriteString("# Infrastructure State with Historical Context\n\n")
	sb.WriteString(fmt.Sprintf("*Generated at %s | Monitoring %d resources*\n\n",
		ctx.GeneratedAt.Format("2006-01-02 15:04"),
		ctx.TotalResources))

	// Global anomalies first (high priority)
	if len(ctx.Anomalies) > 0 {
		sb.WriteString("## Current Anomalies\n")
		for _, a := range ctx.Anomalies {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", a.Metric, a.Description))
		}
		sb.WriteString("\n")
	}

	// Predictions (proactive insights)
	if len(ctx.Predictions) > 0 {
		sb.WriteString("## Predictions\n")
		for _, p := range ctx.Predictions {
			sb.WriteString(fmt.Sprintf("- **%s** on %s: %s (%.0f days, %.0f%% confidence)\n",
				p.Event, p.ResourceID, p.Basis, p.DaysUntil, p.Confidence*100))
		}
		sb.WriteString("\n")
	}

	// Recent changes (what's different)
	if len(ctx.Changes) > 0 {
		sb.WriteString("## Recent Changes\n")
		for _, c := range ctx.Changes {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", c.ResourceName, c.Description))
		}
		sb.WriteString("\n")
	}

	// Resources by type
	if len(ctx.Nodes) > 0 {
		sb.WriteString("## Proxmox Nodes\n")
		for _, r := range ctx.Nodes {
			sb.WriteString(FormatResourceContext(r))
			sb.WriteString("\n")
		}
	}

	if len(ctx.VMs) > 0 {
		sb.WriteString("## Virtual Machines\n")
		for _, r := range ctx.VMs {
			sb.WriteString(FormatResourceContext(r))
		}
		sb.WriteString("\n")
	}

	if len(ctx.Containers) > 0 {
		sb.WriteString("## LXC/OCI Containers\n")
		for _, r := range ctx.Containers {
			sb.WriteString(FormatResourceContext(r))
		}
		sb.WriteString("\n")
	}

	if len(ctx.Storage) > 0 {
		sb.WriteString("## Storage\n")
		for _, r := range ctx.Storage {
			sb.WriteString(FormatResourceContext(r))
		}
		sb.WriteString("\n")
	}

	if len(ctx.DockerHosts) > 0 {
		sb.WriteString("## Docker Hosts\n")
		for _, r := range ctx.DockerHosts {
			sb.WriteString(FormatResourceContext(r))
		}
		sb.WriteString("\n")
	}

	if len(ctx.Hosts) > 0 {
		sb.WriteString("## Agent Hosts\n")
		for _, r := range ctx.Hosts {
			sb.WriteString(FormatResourceContext(r))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatCompactSummary creates a brief overview suitable for context-limited prompts
func FormatCompactSummary(ctx *InfrastructureContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Infrastructure: %d resources\n", ctx.TotalResources))

	// Count by status
	var healthy, warning, critical int
	countResource := func(resources []ResourceContext) {
		for _, r := range resources {
			switch {
			case len(r.Anomalies) > 0:
				critical++
			case hasGrowingTrend(r):
				warning++
			default:
				healthy++
			}
		}
	}
	countResource(ctx.Nodes)
	countResource(ctx.VMs)
	countResource(ctx.Containers)
	countResource(ctx.Storage)
	countResource(ctx.DockerHosts)
	countResource(ctx.Hosts)

	sb.WriteString(fmt.Sprintf("Health: %d healthy, %d warning, %d critical\n", healthy, warning, critical))

	if len(ctx.Anomalies) > 0 {
		sb.WriteString(fmt.Sprintf("Anomalies: %d active\n", len(ctx.Anomalies)))
	}

	if len(ctx.Predictions) > 0 {
		// Show most urgent prediction
		earliest := ctx.Predictions[0]
		for _, p := range ctx.Predictions[1:] {
			if p.DaysUntil < earliest.DaysUntil {
				earliest = p
			}
		}
		sb.WriteString(fmt.Sprintf("Nearest prediction: %s in %.0f days\n", earliest.Event, earliest.DaysUntil))
	}

	return sb.String()
}

// hasGrowingTrend checks if any metric trend is concerning
func hasGrowingTrend(r ResourceContext) bool {
	for _, t := range r.Trends {
		if t.Direction == TrendGrowing && t.RatePerDay > 1 {
			return true
		}
	}
	return false
}

// formatResourceType converts internal type to display label
func formatResourceType(t string) string {
	switch t {
	case "node":
		return "Node"
	case "vm":
		return "VM"
	case "container":
		return "Container"
	case "oci_container":
		return "OCI Container"
	case "storage":
		return "Storage"
	case "docker_host":
		return "Docker Host"
	case "docker_container":
		return "Docker Container"
	case "host":
		return "Host"
	default:
		return strings.Title(t)
	}
}

// formatDuration formats a duration in human-readable form
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		if mins > 0 {
			return fmt.Sprintf("%dh%dm", hours, mins)
		}
		return fmt.Sprintf("%dh", hours)
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	if hours > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	return fmt.Sprintf("%dd", days)
}

// FormatBackupStatus creates a human-readable backup status
func FormatBackupStatus(lastBackup time.Time) string {
	if lastBackup.IsZero() {
		return "never"
	}
	age := time.Since(lastBackup)
	if age < 24*time.Hour {
		return fmt.Sprintf("%.0fh ago", age.Hours())
	}
	days := age.Hours() / 24
	return fmt.Sprintf("%.0fd ago", days)
}

// FormatNodeForContext creates context for a Proxmox node
func FormatNodeForContext(node models.Node, trends map[string]Trend) ResourceContext {
	// Calculate memory percentage
	memPct := 0.0
	if node.Memory.Total > 0 {
		memPct = float64(node.Memory.Used) / float64(node.Memory.Total) * 100
	}

	ctx := ResourceContext{
		ResourceID:    node.ID,
		ResourceType:  "node",
		ResourceName:  node.Name,
		CurrentCPU:    node.CPU * 100, // Convert from 0-1 to percentage
		CurrentMemory: memPct,
		Status:        node.Status,
		Uptime:        time.Duration(node.Uptime) * time.Second,
		Trends:        trends,
	}

	return ctx
}

// FormatGuestForContext creates context for a VM or container
// Note: cpu is 0-1 ratio from Proxmox API, memUsage and diskUsage are already 0-100 percentages
func FormatGuestForContext(
	id, name, node, guestType, status string,
	vmid int,
	cpu, memUsage, diskUsage float64,
	uptime int64,
	lastBackup time.Time,
	trends map[string]Trend,
) ResourceContext {
	ctx := ResourceContext{
		ResourceID:    id,
		ResourceType:  guestType,
		ResourceName:  name,
		Node:          node,
		VMID:          vmid,
		CurrentCPU:    cpu * 100, // Convert from 0-1 to percentage
		CurrentMemory: memUsage,  // Already 0-100 percentage from Memory.Usage
		CurrentDisk:   diskUsage, // Already 0-100 percentage from Disk.Usage
		Status:        status,
		Uptime:        time.Duration(uptime) * time.Second,
		Trends:        trends,
	}

	return ctx
}

// FormatStorageForContext creates context for storage
func FormatStorageForContext(storage models.Storage, trends map[string]Trend) ResourceContext {
	usagePct := storage.Usage
	if usagePct == 0 && storage.Total > 0 {
		usagePct = float64(storage.Used) / float64(storage.Total) * 100
	}

	ctx := ResourceContext{
		ResourceID:   storage.ID,
		ResourceType: "storage",
		ResourceName: storage.Name,
		Node:         storage.Node,
		CurrentDisk:  usagePct,
		Status:       storage.Status,
		Trends:       trends,
	}

	return ctx
}
