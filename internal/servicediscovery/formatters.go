package servicediscovery

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
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

// FormatCloudSafeContext returns the operational discovery context that is safe
// to send to a CLOUD model: service identity, the access pattern, and
// operational paths — WITHOUT PII (hostname, IP, bind addresses, alias). It is
// intended for the chat sanitizer to include in cloud-routed summaries so the
// Assistant stays useful on cloud models while genuinely sensitive fields stay
// redacted. Local routing should keep using FormatForAIContext (full context).
//
// Safe by construction: it reads only non-identifying fields and never emits
// Hostname or any address. Port numbers are included without their bind
// addresses. Returns "" when there is no usable identity.
func FormatCloudSafeContext(d *ResourceDiscovery) string {
	if d == nil {
		return ""
	}

	name := strings.TrimSpace(d.ServiceName)
	if name == "" {
		name = strings.TrimSpace(d.ServiceType)
	}
	if name == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Service: " + name)
	if v := strings.TrimSpace(d.ServiceVersion); v != "" {
		sb.WriteString(" (" + v + ")")
	}
	sb.WriteString("\n")

	if d.Category != "" && d.Category != CategoryUnknown {
		sb.WriteString("Category: " + string(d.Category) + "\n")
	}
	if access := strings.TrimSpace(d.CLIAccess); access != "" {
		sb.WriteString("Access: " + access + "\n")
	}
	if len(d.ConfigPaths) > 0 {
		sb.WriteString("Config paths: " + strings.Join(d.ConfigPaths, ", ") + "\n")
	}
	if len(d.DataPaths) > 0 {
		sb.WriteString("Data paths: " + strings.Join(d.DataPaths, ", ") + "\n")
	}
	if len(d.LogPaths) > 0 {
		sb.WriteString("Log paths: " + strings.Join(d.LogPaths, ", ") + "\n")
	}
	if len(d.Ports) > 0 {
		nums := make([]string, 0, len(d.Ports))
		for _, p := range d.Ports {
			if p.Port > 0 {
				nums = append(nums, strconv.Itoa(p.Port))
			}
		}
		if len(nums) > 0 {
			sb.WriteString("Ports: " + strings.Join(nums, ", ") + "\n")
		}
	}

	return strings.TrimSpace(sb.String())
}

// formatSingleDiscovery formats a single discovery entry.
func formatSingleDiscovery(d *ResourceDiscovery) string {
	var sb strings.Builder

	// Header with service info
	sb.WriteString(fmt.Sprintf("### %s (%s)\n", d.ServiceName, d.ID))
	sb.WriteString(fmt.Sprintf("- **Type:** %s\n", d.ResourceType))
	sb.WriteString(fmt.Sprintf("- **Target:** %s\n", firstNonEmpty(d.Hostname, d.TargetID)))

	if d.ServiceVersion != "" {
		sb.WriteString(fmt.Sprintf("- **Version:** %s\n", d.ServiceVersion))
	}

	if d.SuggestedURL != "" {
		sb.WriteString(fmt.Sprintf("- **Suggested Web URL:** %s\n", d.SuggestedURL))
	}

	if d.Category != "" && d.Category != CategoryUnknown {
		sb.WriteString(fmt.Sprintf("- **Category:** %s\n", d.Category))
	}

	// CLI access (most important for remediation)
	if d.CLIAccess != "" {
		sb.WriteString(fmt.Sprintf("- **CLI Access:** `%s`\n", d.CLIAccess))
	}

	// Config, data, and log paths
	if len(d.ConfigPaths) > 0 {
		sb.WriteString(fmt.Sprintf("- **Config Paths:** %s\n", strings.Join(d.ConfigPaths, ", ")))
	}
	if len(d.DataPaths) > 0 {
		sb.WriteString(fmt.Sprintf("- **Data Paths:** %s\n", strings.Join(d.DataPaths, ", ")))
	}
	if len(d.LogPaths) > 0 {
		sb.WriteString(fmt.Sprintf("- **Log Paths:** %s\n", strings.Join(d.LogPaths, ", ")))
	}

	// Docker bind mounts — where the container's paths actually live on the
	// host. A container path like /config is meaningless for editing or backing
	// up persistent files without its host source, so surface the mapping so the
	// assistant can act on the real files, not the ephemeral container view.
	if len(d.DockerMounts) > 0 {
		sb.WriteString("- **Docker Mounts (host -> container):**\n")
		for _, m := range d.DockerMounts {
			line := fmt.Sprintf("  - %s -> %s", m.Source, m.Destination)
			if m.ReadOnly {
				line += " (read-only)"
			}
			sb.WriteString(line + "\n")
		}
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
	// Priority ranking — facts the assistant needs to understand and act on the
	// workload, ordered most-actionable first so the cap never drops a fact that
	// helps it DO something (restart, auth) in favour of an informational one.
	// Service + storage were previously dropped entirely; a service fact (e.g.
	// the systemd unit) is exactly how you restart/reload a workload, and a
	// storage fact (the backing dataset/disk) is where its data lives — neither
	// is redundant with the path/CLI sections.
	priorityRank := map[FactCategory]int{
		FactCategoryService:    0, // how to restart/reload — most actionable
		FactCategorySecurity:   1, // auth/access
		FactCategoryDependency: 2, // MQTT, database connections
		FactCategoryHardware:   3, // GPU, TPU
		FactCategoryVersion:    4, // version info
		FactCategoryStorage:    5, // backing dataset/disk
	}

	var important []DiscoveryFact
	for _, f := range facts {
		if _, ok := priorityRank[f.Category]; ok && f.Confidence >= 0.7 {
			important = append(important, f)
		}
	}

	// Stable sort by actionability so the most useful facts survive the cap;
	// within a category the analyzer's original order is preserved.
	sort.SliceStable(important, func(i, j int) bool {
		return priorityRank[important[i].Category] < priorityRank[important[j].Category]
	})

	// Cap to keep the pack concise (the analyzer already caps total facts at 12).
	const maxFacts = 8
	if len(important) > maxFacts {
		important = important[:maxFacts]
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

// FormatScopeHint returns a compact, single-line discovery hint for scoped patrols.
func FormatScopeHint(discoveries []*ResourceDiscovery) string {
	if len(discoveries) == 0 {
		return ""
	}
	primary := discoveries[0]
	summary := formatScopeDiscoverySummary(primary)
	if summary == "" {
		return ""
	}
	if len(discoveries) > 1 {
		summary = fmt.Sprintf("%s (+%d more)", summary, len(discoveries)-1)
	}
	return "Discovery: " + summary
}

func formatScopeDiscoverySummary(d *ResourceDiscovery) string {
	if d == nil {
		return ""
	}
	name := firstNonEmpty(d.ServiceName, d.ServiceType, d.ResourceID, d.ID)
	if name == "" {
		return ""
	}
	base := name
	if d.ServiceVersion != "" && !strings.Contains(strings.ToLower(base), strings.ToLower(d.ServiceVersion)) {
		version := d.ServiceVersion
		if !strings.HasPrefix(strings.ToLower(version), "v") {
			version = "v" + version
		}
		base = fmt.Sprintf("%s %s", base, version)
	}

	target := firstNonEmpty(d.Hostname, d.TargetID)
	meta := strings.TrimSpace(string(d.ResourceType))
	if target != "" {
		if meta != "" {
			meta = fmt.Sprintf("%s on %s", meta, target)
		} else {
			meta = target
		}
	}
	if meta != "" {
		base = fmt.Sprintf("%s (%s)", base, meta)
	}

	parts := []string{base}
	if cli := shortenScopeCLI(d.CLIAccess); cli != "" {
		parts = append(parts, "cli: "+cli)
	}
	if ports := formatScopePorts(d.Ports); ports != "" {
		parts = append(parts, "ports: "+ports)
	}

	return strings.Join(parts, "; ")
}

func shortenScopeCLI(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	compact := strings.Join(strings.Fields(trimmed), " ")
	return truncateScopeText(compact, 64)
}

func formatScopePorts(ports []PortInfo) string {
	if len(ports) == 0 {
		return ""
	}
	maxPorts := 3
	if len(ports) < maxPorts {
		maxPorts = len(ports)
	}
	parts := make([]string, 0, maxPorts)
	for i := 0; i < maxPorts; i++ {
		p := ports[i]
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		parts = append(parts, fmt.Sprintf("%d/%s", p.Port, proto))
	}
	if len(ports) > maxPorts {
		parts = append(parts, fmt.Sprintf("+%d more", len(ports)-maxPorts))
	}
	return strings.Join(parts, ", ")
}

func truncateScopeText(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// FormatForRemediation formats discovery specifically for remediation context.
func FormatForRemediation(d *ResourceDiscovery) string {
	if d == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Resource Context for Remediation\n\n")

	sb.WriteString(fmt.Sprintf("**Resource:** %s (%s)\n", d.ServiceName, d.ID))
	sb.WriteString(fmt.Sprintf("**Type:** %s on %s\n\n", d.ResourceType, firstNonEmpty(d.Hostname, d.TargetID)))

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

	// Service control — how to restart/reload the workload, the core remediation
	// action. Previously remediation context surfaced only hardware facts, so it
	// never told you how to actually restart the thing it was helping you fix.
	var serviceFacts []DiscoveryFact
	for _, f := range d.Facts {
		if f.Category == FactCategoryService {
			serviceFacts = append(serviceFacts, f)
		}
	}
	if len(serviceFacts) > 0 {
		sb.WriteString("### Service Control\n")
		for _, f := range serviceFacts {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", f.Key, f.Value))
		}
		sb.WriteString("\n")
	}

	// Config paths for potential fixes
	if len(d.ConfigPaths) > 0 {
		sb.WriteString("### Configuration Files\n")
		for _, p := range d.ConfigPaths {
			sb.WriteString(fmt.Sprintf("- `%s`\n", p))
		}
		sb.WriteString("\n")
	}

	// Data directories — where persistent data lives (backup targets, disk-full
	// triage, restore points). FormatForAIContext surfaces these; remediation
	// must too.
	if len(d.DataPaths) > 0 {
		sb.WriteString("### Data Directories\n")
		for _, p := range d.DataPaths {
			sb.WriteString(fmt.Sprintf("- `%s`\n", p))
		}
		sb.WriteString("\n")
	}

	// Bind mounts — edit or back up persistent files at the HOST source; the
	// container only sees the destination path, so without this a fix written
	// "to /config" would not survive a container recreate.
	if len(d.DockerMounts) > 0 {
		sb.WriteString("### Bind Mounts (host -> container)\n")
		for _, m := range d.DockerMounts {
			line := fmt.Sprintf("- `%s` -> `%s`", m.Source, m.Destination)
			if m.ReadOnly {
				line += " (read-only)"
			}
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}

	// Log paths for troubleshooting
	if len(d.LogPaths) > 0 {
		sb.WriteString("### Log Files\n")
		for _, p := range d.LogPaths {
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

	// Other facts needed to diagnose — dependencies (upstreams, brokers, DBs),
	// auth/security posture, backing storage, special hardware. Previously only
	// hardware surfaced here, so a 502's upstream or a broker's auth never
	// reached the fix context. Use the same ranked filter as the chat pack;
	// service facts are already shown prominently under Service Control.
	var diagnosticFacts []DiscoveryFact
	for _, f := range filterImportantFacts(d.Facts) {
		if f.Category == FactCategoryService {
			continue
		}
		diagnosticFacts = append(diagnosticFacts, f)
	}
	if len(diagnosticFacts) > 0 {
		sb.WriteString("### Relevant Facts\n")
		for _, f := range diagnosticFacts {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", f.Key, f.Value))
		}
		sb.WriteString("\n")
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

// FilterDiscoveriesByResourceIDs returns discoveries that match any of the given resource IDs.
// This is used to scope discovery context for targeted patrol runs.
func FilterDiscoveriesByResourceIDs(discoveries []*ResourceDiscovery, resourceIDs []string) []*ResourceDiscovery {
	if len(discoveries) == 0 {
		return nil
	}
	if len(resourceIDs) == 0 {
		return discoveries
	}

	tokens := buildResourceIDTokenSet(resourceIDs)
	if len(tokens) == 0 {
		return nil
	}

	filtered := make([]*ResourceDiscovery, 0, len(discoveries))
	for _, d := range discoveries {
		if discoveryMatchesTokens(d, tokens) {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

func buildResourceIDTokenSet(resourceIDs []string) map[string]struct{} {
	tokens := make(map[string]struct{})
	for _, id := range resourceIDs {
		addResourceIDTokens(tokens, id)
	}
	return tokens
}

func addResourceIDTokens(tokens map[string]struct{}, resourceID string) {
	trimmed := strings.TrimSpace(resourceID)
	if trimmed == "" {
		return
	}

	utils.AddToken(tokens, trimmed)

	if last := utils.LastSegment(trimmed, '/'); last != "" {
		utils.AddToken(tokens, last)
	}
	if last := utils.LastSegment(trimmed, ':'); last != "" {
		utils.AddToken(tokens, last)
	}

	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "vm-") {
		utils.AddToken(tokens, trimmed[3:])
	}
	if strings.HasPrefix(lower, "ct-") {
		utils.AddToken(tokens, trimmed[3:])
	}
	if strings.HasPrefix(lower, "lxc-") {
		utils.AddToken(tokens, trimmed[4:])
	}

	if strings.Contains(lower, "qemu/") || strings.Contains(lower, "lxc/") || strings.HasPrefix(lower, "vm-") || strings.HasPrefix(lower, "ct-") {
		if digits := utils.TrailingDigits(trimmed); digits != "" {
			utils.AddToken(tokens, digits)
		}
	}

	// docker:host/container -> host + container tokens
	if strings.Contains(trimmed, ":") {
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) == 2 {
			rest := parts[1]
			if slash := strings.Index(rest, "/"); slash >= 0 {
				host := strings.TrimSpace(rest[:slash])
				container := strings.TrimSpace(rest[slash+1:])
				utils.AddToken(tokens, host)
				utils.AddToken(tokens, container)
			}
		}
	}
}

func discoveryMatchesTokens(d *ResourceDiscovery, tokens map[string]struct{}) bool {
	if d == nil {
		return false
	}

	candidates := discoveryTokens(d)
	for _, candidate := range candidates {
		if _, ok := tokens[candidate]; ok {
			return true
		}
	}
	return false
}

func discoveryTokens(d *ResourceDiscovery) []string {
	var tokens []string
	targetID := strings.TrimSpace(d.TargetID)

	add := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		tokens = append(tokens, strings.ToLower(trimmed))
	}

	add(d.ResourceID)
	add(d.ID)
	add(targetID)
	if targetID != "" {
		add("agent:" + targetID)
	}

	switch d.ResourceType {
	case ResourceTypeVM:
		add("qemu/" + d.ResourceID)
		add("vm/" + d.ResourceID)
		add("vm-" + d.ResourceID)
	case ResourceTypeSystemContainer:
		add("lxc/" + d.ResourceID)
		add("ct/" + d.ResourceID)
		add("ct-" + d.ResourceID)
		add("system-container/" + d.ResourceID)
	case ResourceTypeDocker:
		if targetID != "" {
			add("docker:" + targetID)
			add("docker:" + targetID + "/" + d.ResourceID)
		}
	case ResourceTypeAgent:
		add("agent:" + d.ResourceID)
	case ResourceTypeK8s:
		add("k8s/" + d.ResourceID)
		add("kubernetes/" + d.ResourceID)
	}

	return tokens
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
		"id":                          d.ID,
		"resource_type":               d.ResourceType,
		"resource_id":                 d.ResourceID,
		"target_id":                   strings.TrimSpace(d.TargetID),
		"agent_id":                    d.AgentID,
		"hostname":                    d.Hostname,
		"service_type":                d.ServiceType,
		"service_name":                d.ServiceName,
		"service_version":             d.ServiceVersion,
		"category":                    d.Category,
		"cli_access":                  d.CLIAccess,
		"facts":                       facts,
		"config_paths":                d.ConfigPaths,
		"data_paths":                  d.DataPaths,
		"log_paths":                   d.LogPaths,
		"ports":                       ports,
		"user_notes":                  d.UserNotes,
		"confidence":                  d.Confidence,
		"ai_reasoning":                d.AIReasoning,
		"discovered_at":               d.DiscoveredAt,
		"updated_at":                  d.UpdatedAt,
		"scan_duration":               d.ScanDuration,
		"suggested_url":               d.SuggestedURL,
		"suggested_url_source_code":   d.SuggestedURLSourceCode,
		"suggested_url_source_detail": d.SuggestedURLSourceDetail,
		"suggested_url_diagnostic":    d.SuggestedURLDiagnostic,
	}
}
