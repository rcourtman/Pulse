package aidiscovery

import (
	"fmt"
	"strings"
	"time"
)

// FormatForAIContext formats discoveries for inclusion in AI prompts.
// This provides context about resources for Patrol, Investigation, and Chat.
func FormatForAIContext(discoveries []*ResourceDiscovery) string {
	if len(discoveries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Infrastructure Discovery\n\n")
	sb.WriteString("The following has been discovered about the affected resources:\n\n")

	for _, d := range discoveries {
		sb.WriteString(formatSingleDiscovery(d))
		sb.WriteString("\n")
	}

	sb.WriteString("\n**IMPORTANT:** Use the CLI access methods shown above. For example:\n")
	sb.WriteString("- For LXC containers, use `pct exec <vmid> -- <command>`\n")
	sb.WriteString("- For VMs with guest agent, use `qm guest exec <vmid> -- <command>`\n")
	sb.WriteString("- For Docker containers, use `docker exec <container> <command>`\n")

	return sb.String()
}

// FormatSingleForAIContext formats a single discovery for AI context.
func FormatSingleForAIContext(d *ResourceDiscovery) string {
	if d == nil {
		return ""
	}
	return formatSingleDiscovery(d)
}

// formatSingleDiscovery formats a single discovery entry.
func formatSingleDiscovery(d *ResourceDiscovery) string {
	var sb strings.Builder

	// Header with service info
	sb.WriteString(fmt.Sprintf("### %s (%s)\n", d.ServiceName, d.ID))
	sb.WriteString(fmt.Sprintf("- **Type:** %s\n", d.ResourceType))
	sb.WriteString(fmt.Sprintf("- **Host:** %s\n", d.Hostname))

	if d.ServiceVersion != "" {
		sb.WriteString(fmt.Sprintf("- **Version:** %s\n", d.ServiceVersion))
	}

	if d.Category != "" && d.Category != CategoryUnknown {
		sb.WriteString(fmt.Sprintf("- **Category:** %s\n", d.Category))
	}

	// CLI access (most important for remediation)
	if d.CLIAccess != "" {
		sb.WriteString(fmt.Sprintf("- **CLI Access:** `%s`\n", d.CLIAccess))
	}

	// Config and data paths
	if len(d.ConfigPaths) > 0 {
		sb.WriteString(fmt.Sprintf("- **Config Paths:** %s\n", strings.Join(d.ConfigPaths, ", ")))
	}
	if len(d.DataPaths) > 0 {
		sb.WriteString(fmt.Sprintf("- **Data Paths:** %s\n", strings.Join(d.DataPaths, ", ")))
	}

	// Ports
	if len(d.Ports) > 0 {
		var ports []string
		for _, p := range d.Ports {
			ports = append(ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
		}
		sb.WriteString(fmt.Sprintf("- **Ports:** %s\n", strings.Join(ports, ", ")))
	}

	// Important facts
	importantFacts := filterImportantFacts(d.Facts)
	if len(importantFacts) > 0 {
		sb.WriteString("- **Key Facts:**\n")
		for _, f := range importantFacts {
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", f.Key, f.Value))
		}
	}

	// User notes (critical for context)
	if d.UserNotes != "" {
		sb.WriteString(fmt.Sprintf("- **User Notes:** %s\n", d.UserNotes))
	}

	return sb.String()
}

// filterImportantFacts returns the most relevant facts for AI context.
func filterImportantFacts(facts []DiscoveryFact) []DiscoveryFact {
	var important []DiscoveryFact

	// Priority categories
	priorityCategories := map[FactCategory]bool{
		FactCategoryHardware:   true, // GPU, TPU
		FactCategoryDependency: true, // MQTT, database connections
		FactCategorySecurity:   true, // Auth info
		FactCategoryVersion:    true, // Version info
	}

	for _, f := range facts {
		if priorityCategories[f.Category] && f.Confidence >= 0.7 {
			important = append(important, f)
		}
	}

	// Limit to top 5 facts
	if len(important) > 5 {
		important = important[:5]
	}

	return important
}

// FormatDiscoverySummary formats a summary of all discoveries.
func FormatDiscoverySummary(discoveries []*ResourceDiscovery) string {
	if len(discoveries) == 0 {
		return "No infrastructure discovery data available."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Infrastructure Discovery Summary (%d resources):\n\n", len(discoveries)))

	// Group by resource type
	byType := make(map[ResourceType][]*ResourceDiscovery)
	for _, d := range discoveries {
		byType[d.ResourceType] = append(byType[d.ResourceType], d)
	}

	for rt, ds := range byType {
		sb.WriteString(fmt.Sprintf("**%s** (%d):\n", rt, len(ds)))
		for _, d := range ds {
			confidence := ""
			if d.Confidence >= 0.9 {
				confidence = " [high confidence]"
			} else if d.Confidence >= 0.7 {
				confidence = " [medium confidence]"
			}
			sb.WriteString(fmt.Sprintf("  - %s: %s%s\n", d.ResourceID, d.ServiceName, confidence))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatForRemediation formats discovery specifically for remediation context.
func FormatForRemediation(d *ResourceDiscovery) string {
	if d == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Resource Context for Remediation\n\n")

	sb.WriteString(fmt.Sprintf("**Resource:** %s (%s)\n", d.ServiceName, d.ID))
	sb.WriteString(fmt.Sprintf("**Type:** %s on %s\n\n", d.ResourceType, d.Hostname))

	// CLI access is most critical
	if d.CLIAccess != "" {
		sb.WriteString("### How to Execute Commands\n")
		sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", d.CLIAccess))
	}

	// Service-specific info
	if d.ServiceType != "" {
		sb.WriteString(fmt.Sprintf("**Service:** %s", d.ServiceType))
		if d.ServiceVersion != "" {
			sb.WriteString(fmt.Sprintf(" v%s", d.ServiceVersion))
		}
		sb.WriteString("\n\n")
	}

	// Config paths for potential fixes
	if len(d.ConfigPaths) > 0 {
		sb.WriteString("### Configuration Files\n")
		for _, p := range d.ConfigPaths {
			sb.WriteString(fmt.Sprintf("- `%s`\n", p))
		}
		sb.WriteString("\n")
	}

	// User notes may contain important context
	if d.UserNotes != "" {
		sb.WriteString("### User Notes\n")
		sb.WriteString(d.UserNotes)
		sb.WriteString("\n\n")
	}

	// Hardware info for special considerations
	for _, f := range d.Facts {
		if f.Category == FactCategoryHardware {
			sb.WriteString(fmt.Sprintf("**Hardware:** %s = %s\n", f.Key, f.Value))
		}
	}

	return sb.String()
}

// FormatDiscoveryAge returns a human-readable age string.
func FormatDiscoveryAge(d *ResourceDiscovery) string {
	if d == nil || d.UpdatedAt.IsZero() {
		return "unknown"
	}

	age := time.Since(d.UpdatedAt)
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		mins := int(age.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case age < 24*time.Hour:
		hours := int(age.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(age.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

// GetCLIExample returns an example CLI command for the resource.
func GetCLIExample(d *ResourceDiscovery, exampleCmd string) string {
	if d == nil || d.CLIAccess == "" {
		return ""
	}

	// Replace the placeholder with the example command
	cli := d.CLIAccess
	cli = strings.ReplaceAll(cli, "...", exampleCmd)
	cli = strings.ReplaceAll(cli, "{command}", exampleCmd)

	return cli
}

// FormatFactsTable formats facts as a simple table.
func FormatFactsTable(facts []DiscoveryFact) string {
	if len(facts) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("| Category | Key | Value |\n")
	sb.WriteString("|----------|-----|-------|\n")

	for _, f := range facts {
		value := f.Value
		if len(value) > 50 {
			value = value[:47] + "..."
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", f.Category, f.Key, value))
	}

	return sb.String()
}

// BuildResourceContextForPatrol builds context for Patrol findings.
func BuildResourceContextForPatrol(store *Store, resourceIDs []string) string {
	if store == nil || len(resourceIDs) == 0 {
		return ""
	}

	discoveries, err := store.GetMultiple(resourceIDs)
	if err != nil || len(discoveries) == 0 {
		return ""
	}

	return FormatForAIContext(discoveries)
}

// ToJSON converts a discovery to a JSON-friendly map.
func ToJSON(d *ResourceDiscovery) map[string]any {
	if d == nil {
		return nil
	}

	facts := make([]map[string]any, 0, len(d.Facts))
	for _, f := range d.Facts {
		facts = append(facts, map[string]any{
			"category":   f.Category,
			"key":        f.Key,
			"value":      f.Value,
			"source":     f.Source,
			"confidence": f.Confidence,
		})
	}

	ports := make([]map[string]any, 0, len(d.Ports))
	for _, p := range d.Ports {
		ports = append(ports, map[string]any{
			"port":     p.Port,
			"protocol": p.Protocol,
			"process":  p.Process,
			"address":  p.Address,
		})
	}

	return map[string]any{
		"id":              d.ID,
		"resource_type":   d.ResourceType,
		"resource_id":     d.ResourceID,
		"host_id":         d.HostID,
		"hostname":        d.Hostname,
		"service_type":    d.ServiceType,
		"service_name":    d.ServiceName,
		"service_version": d.ServiceVersion,
		"category":        d.Category,
		"cli_access":      d.CLIAccess,
		"facts":           facts,
		"config_paths":    d.ConfigPaths,
		"data_paths":      d.DataPaths,
		"ports":           ports,
		"user_notes":      d.UserNotes,
		"confidence":      d.Confidence,
		"ai_reasoning":    d.AIReasoning,
		"discovered_at":   d.DiscoveredAt,
		"updated_at":      d.UpdatedAt,
		"scan_duration":   d.ScanDuration,
	}
}
