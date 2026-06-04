package agentcontext

import (
	"fmt"
	"strings"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Fact is a bounded, typed fact in a resource context pack. Values come
// from Pulse-owned runtime state only; raw command output, config files,
// environment variables, and secret-bearing metadata do not belong here.
type Fact struct {
	Label      string     `json:"label"`
	Value      string     `json:"value"`
	Source     string     `json:"source,omitempty"`
	TrustTier  string     `json:"trustTier,omitempty"`
	ObservedAt *time.Time `json:"observedAt,omitempty"`
	Redacted   bool       `json:"redacted,omitempty"`
}

// Redaction records why a context section withheld a field. It gives
// agents and UI inspectors a visible safety boundary without leaking
// the underlying value.
type Redaction struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// Section groups related facts with one provenance/freshness envelope.
// Additive sections make a context pack useful to Assistant and external
// MCP clients without breaking older clients that parse top-level fields.
type Section struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Summary     string      `json:"summary,omitempty"`
	Source      string      `json:"source"`
	TrustTier   string      `json:"trustTier"`
	ObservedAt  *time.Time  `json:"observedAt,omitempty"`
	GeneratedAt time.Time   `json:"generatedAt"`
	Facts       []Fact      `json:"facts"`
	Redactions  []Redaction `json:"redactions,omitempty"`
}

// OperatorState is the context-pack projection of operator intent. It is
// intentionally narrower than the persistence/API shape so this package
// stays independent of HTTP and storage owners.
type OperatorState struct {
	IntentionallyOffline    bool
	NeverAutoRemediate      bool
	MaintenanceWindowActive bool
	MaintenanceStartAt      *time.Time
	MaintenanceEndAt        *time.Time
	Criticality             string
	NotePresent             bool
	SetAt                   time.Time
}

// BuildOptions supplies operational counts owned outside the unified
// resource model.
type BuildOptions struct {
	GeneratedAt          time.Time
	OperatorState        *OperatorState
	ActiveFindingCount   int
	PendingApprovalCount int
	RecentActionCount    int
}

const (
	agentContextTrustPulseAuthored   = "pulse-authored"
	agentContextTrustRuntimeObserved = "runtime-observed"
	agentContextTrustDiscovered      = "discovered"
	agentContextTrustUserTaught      = "user-taught"

	agentContextSourceUnifiedResource = "unified-resource"
	agentContextSourceResourceStore   = "resource-store"
	agentContextSourceOperatorState   = "operator-state"
	agentContextSourcePatrol          = "patrol"
	agentContextSourceActionAudit     = "action-audit"
)

// BuildResourceContextSections returns the canonical resource context
// pack shared by Assistant, the agent HTTP endpoint, and MCP adapters.
func BuildResourceContextSections(resource unified.Resource, store unified.ResourceStore, opts BuildOptions) []Section {
	now := opts.GeneratedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sections := make([]Section, 0, 7)

	sections = appendContextSection(sections, buildAgentIdentityContextSection(resource, now))
	sections = appendContextSection(sections, buildAgentRuntimeContextSection(resource, now))
	sections = appendContextSection(sections, buildAgentTopologyContextSection(resource, now))
	sections = appendContextSection(sections, buildAgentSafetyContextSection(resource, opts, now))
	sections = appendContextSection(sections, buildAgentPolicyContextSection(resource, now))
	if store != nil {
		sections = appendContextSection(sections, buildAgentRecentChangesContextSection(resource, store, now))
	}
	return sections
}

// FormatSectionsForModelContext renders context sections as model-only
// briefing text. It is intentionally compact and provenance-rich so the
// chat runtime can attach it to the provider request without storing it
// in the visible user message.
func FormatSectionsForModelContext(resource unified.Resource, sections []Section) string {
	if len(sections) == 0 {
		return ""
	}

	policy := resource.Policy
	label := unified.ResourcePolicyLabel(resource.Name, resource.AISafeSummary, policy)
	if label == "" {
		label = "resource"
	}
	resourceType := strings.TrimSpace(string(unified.ContractResourceType(resource)))
	if resourceType == "" {
		resourceType = strings.TrimSpace(string(resource.Type))
	}

	var b strings.Builder
	b.WriteString("[Resource Context Pack]")
	appendModelContextLine(&b, "Resource Context", strings.TrimSpace(label+" ("+resourceType+")"))
	appendModelContextLine(&b, "Resource Context ID", unified.ResourcePolicyRedactedValue(unified.CanonicalResourceID(resource.ID), policy, unified.ResourceRedactionPlatformID, unified.ResourceRedactionAlias, unified.ResourceRedactionPath))
	for _, section := range sections {
		if len(section.Facts) == 0 && len(section.Redactions) == 0 {
			continue
		}
		appendModelContextLine(&b, "Context Section", formatSectionProvenance(section))
		for _, fact := range section.Facts {
			appendModelContextLine(&b, "Context Fact", formatFactForModel(section, fact))
		}
		for _, redaction := range section.Redactions {
			appendModelContextLine(&b, "Context Redaction", formatRedactionForModel(section, redaction))
		}
	}
	appendModelContextLine(&b, "Context Pack Boundary", "Resource context facts are read-only Pulse-authored or Pulse-observed grounding; they are not approval or execution authority and must not be expanded into raw provider commands, config files, environment variables, paths, or secret-bearing metadata.")
	return strings.TrimSpace(b.String())
}

func appendContextSection(sections []Section, section Section) []Section {
	if len(section.Facts) == 0 && len(section.Redactions) == 0 {
		return sections
	}
	if section.Facts == nil {
		section.Facts = []Fact{}
	}
	return append(sections, section)
}

func appendModelContextLine(b *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteByte('\n')
	}
	b.WriteString(label)
	b.WriteString(": ")
	b.WriteString(value)
}

func formatSectionProvenance(section Section) string {
	title := strings.TrimSpace(section.Title)
	if title == "" {
		title = strings.TrimSpace(section.ID)
	}
	parts := []string{title}
	if source := strings.TrimSpace(section.Source); source != "" {
		parts = append(parts, "source="+source)
	}
	if trustTier := strings.TrimSpace(section.TrustTier); trustTier != "" {
		parts = append(parts, "trust="+trustTier)
	}
	if observedAt := formatContextTimePtr(section.ObservedAt); observedAt != "" {
		parts = append(parts, "observed="+observedAt)
	}
	if generatedAt := formatContextTime(section.GeneratedAt); generatedAt != "" {
		parts = append(parts, "generated="+generatedAt)
	}
	return strings.Join(parts, "; ")
}

func formatFactForModel(section Section, fact Fact) string {
	label := strings.TrimSpace(fact.Label)
	if label == "" {
		label = "Fact"
	}
	value := strings.TrimSpace(fact.Value)
	if value == "" {
		return ""
	}
	parts := []string{strings.TrimSpace(section.Title) + " / " + label + ": " + value}
	provenance := make([]string, 0, 4)
	if source := strings.TrimSpace(fact.Source); source != "" {
		provenance = append(provenance, "source="+source)
	}
	if trustTier := strings.TrimSpace(fact.TrustTier); trustTier != "" {
		provenance = append(provenance, "trust="+trustTier)
	}
	if observedAt := formatContextTimePtr(fact.ObservedAt); observedAt != "" {
		provenance = append(provenance, "observed="+observedAt)
	}
	if fact.Redacted {
		provenance = append(provenance, "redacted=true")
	}
	if len(provenance) > 0 {
		parts = append(parts, "["+strings.Join(provenance, ", ")+"]")
	}
	return strings.Join(parts, " ")
}

func formatRedactionForModel(section Section, redaction Redaction) string {
	field := strings.TrimSpace(redaction.Field)
	if field == "" {
		field = "field"
	}
	reason := strings.TrimSpace(redaction.Reason)
	if reason == "" {
		return strings.TrimSpace(section.Title) + " / " + field
	}
	return strings.TrimSpace(section.Title) + " / " + field + ": " + reason
}

func formatContextTimePtr(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatContextTime(*value)
}

func formatContextTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func newAgentContextSection(id, title, source, trustTier string, generatedAt time.Time, observedAt *time.Time) Section {
	return Section{
		ID:          id,
		Title:       title,
		Source:      source,
		TrustTier:   trustTier,
		ObservedAt:  observedAt,
		GeneratedAt: generatedAt,
		Facts:       []Fact{},
	}
}

func agentContextFact(label, value, source, trustTier string, observedAt *time.Time) Fact {
	value = strings.TrimSpace(value)
	fact := Fact{
		Label:      strings.TrimSpace(label),
		Value:      value,
		Source:     source,
		TrustTier:  trustTier,
		ObservedAt: observedAt,
	}
	if value == unified.ResourcePolicyRedactedLabel {
		fact.Redacted = true
	}
	return fact
}

func addAgentContextFact(facts *[]Fact, label, value, source, trustTier string, observedAt *time.Time) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	*facts = append(*facts, agentContextFact(label, value, source, trustTier, observedAt))
}

func addAgentContextCountFact(facts *[]Fact, label string, value int, source, trustTier string, observedAt *time.Time) {
	if value <= 0 {
		return
	}
	addAgentContextFact(facts, label, fmt.Sprintf("%d", value), source, trustTier, observedAt)
}

func buildAgentIdentityContextSection(resource unified.Resource, now time.Time) Section {
	observedAt := agentResourceObservedAt(resource)
	section := newAgentContextSection("identity", "Identity", agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, now, observedAt)
	policy := resource.Policy

	displayName := unified.ResourcePolicyLabel(resource.Name, resource.AISafeSummary, policy)
	addAgentContextFact(&section.Facts, "Display name", displayName, agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
	addAgentContextFact(&section.Facts, "Canonical ID", unified.ResourcePolicyRedactedValue(unified.CanonicalResourceID(resource.ID), policy, unified.ResourceRedactionPlatformID, unified.ResourceRedactionAlias), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
	addAgentContextFact(&section.Facts, "Resource type", string(resource.Type), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
	addAgentContextFact(&section.Facts, "Status", string(resource.Status), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(&section.Facts, "Technology", resource.Technology, agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
	if !resource.LastSeen.IsZero() {
		addAgentContextFact(&section.Facts, "Last seen", resource.LastSeen.UTC().Format(time.RFC3339), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	}
	if len(resource.Sources) > 0 {
		addAgentContextFact(&section.Facts, "Sources", joinDataSources(resource.Sources), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
	}
	for source, status := range resource.SourceStatus {
		label := fmt.Sprintf("%s source", source)
		value := strings.TrimSpace(status.Status)
		if !status.LastSeen.IsZero() {
			value = strings.TrimSpace(value + " lastSeen=" + status.LastSeen.UTC().Format(time.RFC3339))
		}
		addAgentContextFact(&section.Facts, label, value, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	}

	if len(unified.ResourcePolicySummaryLines(resource.Policy)) > 0 {
		section.Redactions = append(section.Redactions, agentResourcePolicyRedactions(resource.Policy)...)
	}

	return section
}

func buildAgentRuntimeContextSection(resource unified.Resource, now time.Time) Section {
	observedAt := agentResourceObservedAt(resource)
	section := newAgentContextSection("runtime", "Runtime and Discovery", agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, now, observedAt)
	policy := resource.Policy

	if resource.DiscoveryTarget != nil {
		addAgentContextFact(&section.Facts, "Discovery type", resource.DiscoveryTarget.ResourceType, agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
		addAgentContextFact(&section.Facts, "Discovery agent", unified.ResourcePolicyRedactedValue(resource.DiscoveryTarget.AgentID, policy, unified.ResourceRedactionPlatformID, unified.ResourceRedactionAlias), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
		addAgentContextFact(&section.Facts, "Discovery resource", unified.ResourcePolicyRedactedValue(resource.DiscoveryTarget.ResourceID, policy, unified.ResourceRedactionPlatformID, unified.ResourceRedactionAlias), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
		addAgentContextFact(&section.Facts, "Discovery hostname", unified.ResourcePolicyRedactedValue(resource.DiscoveryTarget.Hostname, policy, unified.ResourceRedactionHostname), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
	}

	addMetricsFacts(&section.Facts, resource, observedAt)
	addAgentDataFacts(&section.Facts, resource, observedAt)
	addProxmoxFacts(&section.Facts, resource, observedAt)
	addDockerFacts(&section.Facts, resource, observedAt)
	addKubernetesFacts(&section.Facts, resource, observedAt)
	addStorageFacts(&section.Facts, resource, observedAt)
	addTrueNASFacts(&section.Facts, resource, observedAt)

	return section
}

func buildAgentTopologyContextSection(resource unified.Resource, now time.Time) Section {
	observedAt := agentResourceObservedAt(resource)
	section := newAgentContextSection("topology", "Topology", agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, now, observedAt)
	policy := resource.Policy

	if resource.ParentID != nil {
		addAgentContextFact(&section.Facts, "Parent ID", unified.ResourcePolicyRedactedValue(unified.CanonicalResourceID(*resource.ParentID), policy, unified.ResourceRedactionPlatformID, unified.ResourceRedactionAlias), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
	}
	addAgentContextFact(&section.Facts, "Parent name", unified.ResourcePolicyRedactedValue(resource.ParentName, policy, unified.ResourceRedactionHostname, unified.ResourceRedactionAlias), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
	addAgentContextCountFact(&section.Facts, "Child resources", resource.ChildCount, agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)

	relationships := unified.ResourceRelationshipsWithCanonicalParent(resource)
	if len(relationships) > 0 {
		addAgentContextCountFact(&section.Facts, "Relationship count", len(relationships), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
		for i, rel := range relationships {
			if i >= 5 {
				addAgentContextFact(&section.Facts, "Relationship limit", fmt.Sprintf("%d additional relationships omitted from context pack", len(relationships)-i), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
				break
			}
			value := formatAgentRelationshipFact(rel, policy)
			relObservedAt := timePtrIfSet(rel.LastSeenAt)
			if relObservedAt == nil {
				relObservedAt = timePtrIfSet(rel.ObservedAt)
			}
			addAgentContextFact(&section.Facts, fmt.Sprintf("Relationship %d", i+1), value, agentContextSourceUnifiedResource, agentContextTrustDiscovered, relObservedAt)
		}
	}

	return section
}

func buildAgentSafetyContextSection(resource unified.Resource, opts BuildOptions, now time.Time) Section {
	observedAt := agentResourceObservedAt(resource)
	section := newAgentContextSection("safety", "Safety and Operations", agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, now, observedAt)

	if opts.OperatorState != nil {
		addAgentContextFact(&section.Facts, "Operator state", formatOperatorStateFact(*opts.OperatorState), agentContextSourceOperatorState, agentContextTrustUserTaught, timePtrIfSet(opts.OperatorState.SetAt))
	}
	addAgentContextCountFact(&section.Facts, "Active findings", opts.ActiveFindingCount, agentContextSourcePatrol, agentContextTrustRuntimeObserved, &now)
	addAgentContextCountFact(&section.Facts, "Pending approvals", opts.PendingApprovalCount, agentContextSourceActionAudit, agentContextTrustRuntimeObserved, &now)
	addAgentContextCountFact(&section.Facts, "Recent actions", opts.RecentActionCount, agentContextSourceActionAudit, agentContextTrustRuntimeObserved, &now)
	if len(resource.Capabilities) > 0 {
		addAgentContextCountFact(&section.Facts, "Governed capabilities", len(resource.Capabilities), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
		for i, capability := range resource.Capabilities {
			if i >= 5 {
				addAgentContextFact(&section.Facts, "Capability limit", fmt.Sprintf("%d additional capabilities omitted from context pack", len(resource.Capabilities)-i), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
				break
			}
			addAgentContextFact(&section.Facts, fmt.Sprintf("Capability %d", i+1), formatCapabilityFact(capability), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
		}
	}
	if len(resource.Incidents) > 0 {
		addAgentContextCountFact(&section.Facts, "Incidents", len(resource.Incidents), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
		for i, incident := range resource.Incidents {
			if i >= 3 {
				break
			}
			incidentObservedAt := timePtrIfSet(incident.StartedAt)
			addAgentContextFact(&section.Facts, fmt.Sprintf("Incident %d", i+1), unified.ResourcePolicyRedactedText(incident.Summary, resource), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, incidentObservedAt)
		}
	}

	return section
}

func buildAgentPolicyContextSection(resource unified.Resource, now time.Time) Section {
	observedAt := agentResourceObservedAt(resource)
	section := newAgentContextSection("policy", "Data Policy", agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, now, observedAt)
	for _, line := range unified.ResourcePolicySummaryLines(resource.Policy) {
		addAgentContextFact(&section.Facts, "Policy", line, agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
	}
	if resource.AISafeSummary != "" && unified.ResourcePolicyUsesAISafeSummary(resource.AISafeSummary, resource.Policy) {
		addAgentContextFact(&section.Facts, "AI-safe summary", strings.TrimSpace(resource.AISafeSummary), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
	}
	section.Redactions = append(section.Redactions, agentResourcePolicyRedactions(resource.Policy)...)
	return section
}

func buildAgentRecentChangesContextSection(resource unified.Resource, store unified.ResourceStore, now time.Time) Section {
	section := newAgentContextSection("recent_changes", "Recent Changes", agentContextSourceResourceStore, agentContextTrustRuntimeObserved, now, &now)
	changes, err := store.GetRecentChanges(unified.CanonicalResourceID(resource.ID), now.Add(-7*24*time.Hour), 5)
	if err != nil || len(changes) == 0 {
		return section
	}
	addAgentContextCountFact(&section.Facts, "Included changes", len(changes), agentContextSourceResourceStore, agentContextTrustRuntimeObserved, &now)
	for i, change := range changes {
		observedAt := timePtrIfSet(change.ObservedAt)
		addAgentContextFact(&section.Facts, fmt.Sprintf("Change %d", i+1), formatAgentResourceChangeFact(change, resource), agentContextSourceResourceStore, agentContextTrustRuntimeObserved, observedAt)
	}
	return section
}

func addMetricsFacts(facts *[]Fact, resource unified.Resource, observedAt *time.Time) {
	if resource.Metrics == nil {
		return
	}
	addMetricFact(facts, "CPU", resource.Metrics.CPU, observedAt)
	addMetricFact(facts, "Memory", resource.Metrics.Memory, observedAt)
	addMetricFact(facts, "Disk", resource.Metrics.Disk, observedAt)
	addMetricFact(facts, "Network in", resource.Metrics.NetIn, observedAt)
	addMetricFact(facts, "Network out", resource.Metrics.NetOut, observedAt)
}

func addMetricFact(facts *[]Fact, label string, metric *unified.MetricValue, observedAt *time.Time) {
	if metric == nil {
		return
	}
	value := ""
	switch {
	case metric.Percent > 0:
		value = fmt.Sprintf("%.1f%%", metric.Percent)
	case metric.Value != 0:
		value = fmt.Sprintf("%.1f %s", metric.Value, strings.TrimSpace(metric.Unit))
	case metric.Used != nil && metric.Total != nil && *metric.Total > 0:
		value = fmt.Sprintf("%d/%d", *metric.Used, *metric.Total)
	}
	addAgentContextFact(facts, label, value, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
}

func addAgentDataFacts(facts *[]Fact, resource unified.Resource, observedAt *time.Time) {
	if resource.Agent == nil {
		return
	}
	policy := resource.Policy
	addAgentContextFact(facts, "Agent hostname", unified.ResourcePolicyRedactedValue(resource.Agent.Hostname, policy, unified.ResourceRedactionHostname), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Agent platform", resource.Agent.Platform, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Agent OS", strings.TrimSpace(resource.Agent.OSName+" "+resource.Agent.OSVersion), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Agent version", resource.Agent.AgentVersion, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Commands enabled", fmt.Sprintf("%t", resource.Agent.CommandsEnabled), agentContextSourceUnifiedResource, agentContextTrustPulseAuthored, observedAt)
}

func addProxmoxFacts(facts *[]Fact, resource unified.Resource, observedAt *time.Time) {
	if resource.Proxmox == nil {
		return
	}
	policy := resource.Policy
	addAgentContextFact(facts, "Proxmox node", unified.ResourcePolicyRedactedValue(resource.Proxmox.NodeName, policy, unified.ResourceRedactionHostname, unified.ResourceRedactionAlias), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	if resource.Proxmox.VMID > 0 {
		addAgentContextFact(facts, "Proxmox VMID", unified.ResourcePolicyRedactedValue(fmt.Sprintf("%d", resource.Proxmox.VMID), policy, unified.ResourceRedactionPlatformID), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	}
	addAgentContextFact(facts, "Container type", resource.Proxmox.ContainerType, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "OS", strings.TrimSpace(resource.Proxmox.OSName+" "+resource.Proxmox.OSVersion), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	if resource.Proxmox.HasDocker {
		addAgentContextFact(facts, "Nested Docker", "detected", agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, timePtrIfSetValue(resource.Proxmox.DockerCheckedAt))
	}
	addAgentContextFact(facts, "Proxmox lock", resource.Proxmox.Lock, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
}

func addDockerFacts(facts *[]Fact, resource unified.Resource, observedAt *time.Time) {
	if resource.Docker == nil {
		return
	}
	policy := resource.Policy
	addAgentContextFact(facts, "Docker runtime", strings.TrimSpace(resource.Docker.Runtime+" "+resource.Docker.RuntimeVersion), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Docker version", resource.Docker.DockerVersion, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextCountFact(facts, "Containers", resource.Docker.ContainerCount, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextCountFact(facts, "Images", resource.Docker.ImageCount, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Container state", resource.Docker.ContainerState, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Container health", resource.Docker.Health, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	if resource.Docker.RestartCount > 0 {
		addAgentContextFact(facts, "Restart count", fmt.Sprintf("%d", resource.Docker.RestartCount), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	}
	addAgentContextFact(facts, "Image", unified.ResourcePolicyRedactedValue(resource.Docker.Image, policy, unified.ResourceRedactionHostname, unified.ResourceRedactionAlias), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Ports", formatDockerPorts(resource.Docker.Ports), agentContextSourceUnifiedResource, agentContextTrustDiscovered, observedAt)
	addAgentContextCountFact(facts, "Networks", len(resource.Docker.Networks), agentContextSourceUnifiedResource, agentContextTrustDiscovered, observedAt)
	addAgentContextCountFact(facts, "Mounts", len(resource.Docker.Mounts), agentContextSourceUnifiedResource, agentContextTrustDiscovered, observedAt)
	addAgentContextFact(facts, "Service stack", resource.Docker.Stack, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	if resource.Docker.DesiredTasks > 0 || resource.Docker.RunningTasks > 0 {
		addAgentContextFact(facts, "Service tasks", fmt.Sprintf("%d running / %d desired", resource.Docker.RunningTasks, resource.Docker.DesiredTasks), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	}
	addAgentContextFact(facts, "Endpoint ports", formatDockerServicePorts(resource.Docker.EndpointPorts), agentContextSourceUnifiedResource, agentContextTrustDiscovered, observedAt)
	if resource.Docker.UpdateStatus != nil {
		addAgentContextFact(facts, "Container image update", fmt.Sprintf("available=%t", resource.Docker.UpdateStatus.UpdateAvailable), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, timePtrIfSet(resource.Docker.UpdateStatus.LastChecked))
	}
}

func addKubernetesFacts(facts *[]Fact, resource unified.Resource, observedAt *time.Time) {
	if resource.Kubernetes == nil {
		return
	}
	addAgentContextFact(facts, "Namespace", resource.Kubernetes.Namespace, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Kubernetes status", resource.Kubernetes.Phase, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Service type", resource.Kubernetes.ServiceType, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Service ports", formatKubernetesServicePorts(resource.Kubernetes.ServicePorts), agentContextSourceUnifiedResource, agentContextTrustDiscovered, observedAt)
	addAgentContextCountFact(facts, "Pod containers", len(resource.Kubernetes.PodContainers), agentContextSourceUnifiedResource, agentContextTrustDiscovered, observedAt)
}

func addStorageFacts(facts *[]Fact, resource unified.Resource, observedAt *time.Time) {
	if resource.Storage == nil {
		return
	}
	policy := resource.Policy
	addAgentContextFact(facts, "Storage type", resource.Storage.Type, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Storage platform", resource.Storage.Platform, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Storage path", unified.ResourcePolicyRedactedValue(resource.Storage.Path, policy, unified.ResourceRedactionPath), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Protection", resource.Storage.Protection, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextFact(facts, "Risk summary", unified.ResourcePolicyRedactedText(resource.Storage.RiskSummary, resource), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextCountFact(facts, "Consumers", resource.Storage.ConsumerCount, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
}

func addTrueNASFacts(facts *[]Fact, resource unified.Resource, observedAt *time.Time) {
	if resource.TrueNAS == nil {
		return
	}
	addAgentContextFact(facts, "TrueNAS version", resource.TrueNAS.Version, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	addAgentContextCountFact(facts, "TrueNAS services", len(resource.TrueNAS.Services), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
	if resource.TrueNAS.App != nil {
		addAgentContextFact(facts, "TrueNAS app state", resource.TrueNAS.App.State, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
		addAgentContextFact(facts, "TrueNAS app version", strings.TrimSpace(resource.TrueNAS.App.Version+" "+resource.TrueNAS.App.HumanVersion), agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
		addAgentContextCountFact(facts, "TrueNAS app containers", resource.TrueNAS.App.ContainerCount, agentContextSourceUnifiedResource, agentContextTrustRuntimeObserved, observedAt)
		addAgentContextCountFact(facts, "TrueNAS app ports", len(resource.TrueNAS.App.UsedPorts), agentContextSourceUnifiedResource, agentContextTrustDiscovered, observedAt)
	}
}

func formatAgentRelationshipFact(rel unified.ResourceRelationship, policy *unified.ResourcePolicy) string {
	source := unified.ResourcePolicyRedactedValue(unified.CanonicalResourceID(rel.SourceID), policy, unified.ResourceRedactionPlatformID, unified.ResourceRedactionAlias)
	target := unified.ResourcePolicyRedactedValue(unified.CanonicalResourceID(rel.TargetID), policy, unified.ResourceRedactionPlatformID, unified.ResourceRedactionAlias)
	state := "historical"
	if rel.Active {
		state = "active"
	}
	parts := []string{fmt.Sprintf("%s %s -> %s", rel.Type, source, target), state}
	if rel.Discoverer != "" {
		parts = append(parts, "discoverer="+rel.Discoverer)
	}
	if rel.Confidence > 0 {
		parts = append(parts, fmt.Sprintf("confidence=%.0f%%", rel.Confidence*100))
	}
	if len(rel.Metadata) > 0 {
		parts = append(parts, "metadata present")
	}
	return strings.Join(parts, "; ")
}

func formatOperatorStateFact(state OperatorState) string {
	parts := []string{}
	if state.IntentionallyOffline {
		parts = append(parts, "intentionally offline")
	}
	if state.NeverAutoRemediate {
		parts = append(parts, "never auto-remediate")
	}
	if state.MaintenanceWindowActive {
		parts = append(parts, "maintenance active")
	} else if state.MaintenanceStartAt != nil || state.MaintenanceEndAt != nil {
		parts = append(parts, "maintenance scheduled")
	}
	if state.Criticality != "" {
		parts = append(parts, "criticality="+state.Criticality)
	}
	if state.NotePresent {
		parts = append(parts, "note present")
	}
	if len(parts) == 0 {
		return "operator state recorded"
	}
	return strings.Join(parts, "; ")
}

func formatCapabilityFact(capability unified.ResourceCapability) string {
	parts := []string{capability.Name}
	if capability.MinimumApprovalLevel != "" {
		parts = append(parts, "approval="+string(capability.MinimumApprovalLevel))
	}
	if capability.Platform != "" {
		parts = append(parts, "platform="+capability.Platform)
	}
	sensitiveParams := 0
	for _, param := range capability.Params {
		if param.IsSensitive {
			sensitiveParams++
		}
	}
	if sensitiveParams > 0 {
		parts = append(parts, fmt.Sprintf("%d sensitive params hidden", sensitiveParams))
	}
	return strings.Join(parts, "; ")
}

func formatAgentResourceChangeFact(change unified.ResourceChange, resource unified.Resource) string {
	parts := []string{string(change.Kind)}
	if change.From != "" || change.To != "" {
		transition := strings.TrimSpace(change.From + " -> " + change.To)
		parts = append(parts, unified.ResourcePolicyRedactedText(transition, resource))
	}
	if change.Reason != "" {
		parts = append(parts, "reason="+unified.ResourcePolicyRedactedText(change.Reason, resource))
	}
	if change.SourceType != "" {
		parts = append(parts, "source="+string(change.SourceType))
	}
	if change.SourceAdapter != "" {
		parts = append(parts, "adapter="+string(change.SourceAdapter))
	}
	if change.Confidence != "" {
		parts = append(parts, "confidence="+string(change.Confidence))
	}
	if len(change.RelatedResources) > 0 {
		parts = append(parts, fmt.Sprintf("%d related resources", len(change.RelatedResources)))
	}
	return strings.Join(parts, "; ")
}

func agentResourceObservedAt(resource unified.Resource) *time.Time {
	if !resource.UpdatedAt.IsZero() {
		return timePtrIfSet(resource.UpdatedAt)
	}
	return timePtrIfSet(resource.LastSeen)
}

func timePtrIfSet(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	utc := t.UTC()
	return &utc
}

func timePtrIfSetValue(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	return timePtrIfSet(*t)
}

func joinDataSources(sources []unified.DataSource) string {
	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		if strings.TrimSpace(string(source)) != "" {
			parts = append(parts, string(source))
		}
	}
	return strings.Join(parts, ", ")
}

func agentResourcePolicyRedactions(policy *unified.ResourcePolicy) []Redaction {
	labels := unified.ResourcePolicyRedactionLabels(policy)
	if len(labels) == 0 {
		return nil
	}
	return []Redaction{
		{
			Field:  "resource identifiers",
			Reason: "canonical resource policy redacts " + strings.Join(labels, ", "),
		},
	}
}

func formatDockerPorts(ports []unified.DockerPortMeta) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, 0, min(len(ports), 5))
	for i, port := range ports {
		if i >= 5 {
			parts = append(parts, fmt.Sprintf("%d more", len(ports)-i))
			break
		}
		if port.PublicPort > 0 {
			parts = append(parts, fmt.Sprintf("%d:%d/%s", port.PublicPort, port.PrivatePort, port.Protocol))
			continue
		}
		parts = append(parts, fmt.Sprintf("%d/%s", port.PrivatePort, port.Protocol))
	}
	return strings.Join(parts, ", ")
}

func formatDockerServicePorts(ports []unified.DockerServicePortMeta) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, 0, min(len(ports), 5))
	for i, port := range ports {
		if i >= 5 {
			parts = append(parts, fmt.Sprintf("%d more", len(ports)-i))
			break
		}
		if port.PublishedPort > 0 {
			parts = append(parts, fmt.Sprintf("%d:%d/%s", port.PublishedPort, port.TargetPort, port.Protocol))
			continue
		}
		parts = append(parts, fmt.Sprintf("%d/%s", port.TargetPort, port.Protocol))
	}
	return strings.Join(parts, ", ")
}

func formatKubernetesServicePorts(ports []unified.K8sServicePort) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, 0, min(len(ports), 5))
	for i, port := range ports {
		if i >= 5 {
			parts = append(parts, fmt.Sprintf("%d more", len(ports)-i))
			break
		}
		parts = append(parts, fmt.Sprintf("%d:%s/%s", port.Port, port.TargetPort, port.Protocol))
	}
	return strings.Join(parts, ", ")
}
