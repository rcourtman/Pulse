package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
	"github.com/rs/zerolog/log"
)

// registerSummarizeTools registers the pulse_summarize tool which
// exposes the reporting synthesis engine to chat sessions as a
// retrospective question-answering capability. The tool wraps the
// engine's NarrativeFor and FleetNarrativeFor entry points so
// operators can ask "what's hot on pve1 this week" or "where should
// I look across my fleet" without round-tripping through report
// generation. v1 returns heuristic narrative (the same deterministic
// observations the report PDF carries when AI is unconfigured); a
// follow-up commit will thread the per-tenant AI narrator through
// the chat session so this tool can return AI-generated synthesis
// in the same shape.
func (e *PulseToolExecutor) registerSummarizeTools() {
	e.registry.registerBuiltin(RegisteredTool{
		Definition: Tool{
			Name: agentcapabilities.PulseSummarizeToolName,
			Description: `Generate a retrospective summary of one resource or a fleet across a time window. Use this when the operator asks questions like "what's been happening with pve1 this week" or "where should I look across my fleet" — answers grounded in metric stats, alerts, storage state, disk health, and Patrol findings within the window.

Two modes via the 'action' parameter:
  - "resource": summarises a single resource. Required: resource_id (ID or name); resource_type only when the ID is not a known resource.
  - "fleet":    summarises a fleet across multiple resources. resource_ids is optional — omit it and the tool enumerates the known fleet itself (infrastructure first, bounded). Never ask the operator for resource IDs.

Time window defaults to the last 7 days; supported ranges: 24h, 7d, 30d.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"action": {
						Type:        "string",
						Description: "Summary scope: resource (single) or fleet (multi-resource).",
						Enum:        []string{"resource", "fleet"},
					},
					"resource_type": {
						Type:        "string",
						Description: "For action=resource: canonical resource type (node, vm, system-container, oci-container, app-container, docker-host, storage, agent, k8s, disk, pbs, pmg). For action=fleet: optional filter (when enumerating) or default type for unrecognized resource_ids entries.",
					},
					"resource_id": {
						Type:        "string",
						Description: "For action=resource: the resource identifier or name (e.g. instance:node:vmid, or a host name).",
					},
					"resource_ids": {
						Type:        "string",
						Description: "For action=fleet: optional comma-separated list of resource identifiers or names to include (e.g. \"instance:pve1:101,instance:pve1:102\"). Omit to summarize every known resource (bounded).",
					},
					"range": {
						Type:        "string",
						Description: "Time window: 24h, 7d, or 30d. Defaults to 7d.",
						Enum:        []string{"24h", "7d", "30d"},
					},
				},
				Required: []string{"action"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeSummarize(ctx, args)
		},
		Governance: ToolGovernance{
			ActionMode:      ToolActionRead,
			ApprovalPolicy:  ToolApprovalScopeOnly,
			ApprovalSummary: "no approval required; pure read of metrics history and findings store.",
			Summary:         "Returns a retrospective synthesis (observations, recommendations, outliers, period comparison) for one resource or a fleet within a time window.",
		},
	})
}

// summarizeRangeWindow maps the operator-facing range token to the
// reporting catalog's supported windows. Unknown values fall back to
// the catalog default rather than erroring — the model is more
// forgiving than the API handler is required to be.
func summarizeRangeWindow(raw string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "24h":
		return 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	case "7d", "":
		return 7 * 24 * time.Hour
	default:
		// Unknown values are normalized to the catalog default rather
		// than rejected — the chat model is the caller, and forcing it
		// to retry over a typo is worse UX than silently coercing to
		// the standard window.
		return reporting.DescribePerformanceReport().DefaultRangeDuration()
	}
}

func (e *PulseToolExecutor) executeSummarize(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	engine := reporting.GetEngine()
	if engine == nil {
		return NewErrorResult(fmt.Errorf("reporting engine not initialized")), nil
	}

	action, _ := args["action"].(string)
	action = strings.TrimSpace(strings.ToLower(action))

	rangeRaw, _ := args["range"].(string)
	window := summarizeRangeWindow(rangeRaw)
	end := time.Now()
	start := end.Add(-window)

	switch action {
	case "resource":
		return e.summarizeResource(ctx, engine, args, start, end)
	case "fleet":
		return e.summarizeFleet(ctx, engine, args, start, end)
	case "":
		return NewErrorResult(fmt.Errorf("'action' is required: use 'resource' or 'fleet'")), nil
	default:
		return NewErrorResult(fmt.Errorf("unknown action %q: use 'resource' or 'fleet'", action)), nil
	}
}

type summarizeResourceResponse struct {
	OK              bool                        `json:"ok"`
	Action          string                      `json:"action"`
	ResourceType    string                      `json:"resource_type"`
	ResourceID      string                      `json:"resource_id"`
	WindowStart     time.Time                   `json:"window_start"`
	WindowEnd       time.Time                   `json:"window_end"`
	NarrativeSource string                      `json:"narrative_source"`
	HealthStatus    string                      `json:"health_status,omitempty"`
	HealthMessage   string                      `json:"health_message,omitempty"`
	Observations    []reporting.NarrativeBullet `json:"observations,omitempty"`
	Recommendations []string                    `json:"recommendations,omitempty"`
	Disclaimer      string                      `json:"disclaimer,omitempty"`
}

func (e *PulseToolExecutor) summarizeResource(
	_ context.Context,
	engine reporting.Engine,
	args map[string]interface{},
	start, end time.Time,
) (CallToolResult, error) {
	resourceTypeRaw, _ := args["resource_type"].(string)
	resourceID, _ := args["resource_id"].(string)
	resourceTypeRaw = strings.TrimSpace(resourceTypeRaw)
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required for action=resource")), nil
	}

	// Resolve the reference against the unified registry first: models pass
	// canonical IDs and plain names, both of which need translation onto the
	// metrics-store ID space before the engine can find any data points.
	var req reporting.MetricReportRequest
	canonicalType := ""
	if cand, ok := e.buildSummarizeCandidateIndex().lookup(resourceID); ok {
		canonicalType = cand.reportType
		req = reporting.MetricReportRequest{
			ResourceType:      cand.reportType,
			ResourceID:        cand.id,
			MetricsResourceID: cand.metricsID,
		}
		resourceID = cand.id
		if cand.name != "" {
			req.Resource = &reporting.ResourceInfo{Name: cand.name, Status: cand.status}
		}
	} else {
		if resourceTypeRaw == "" {
			return NewErrorResult(fmt.Errorf("resource_type is required for action=resource when resource_id %q does not match a known resource", resourceID)), nil
		}
		canonicalType = reporting.CanonicalResourceType(resourceTypeRaw)
		if canonicalType == "" {
			return NewErrorResult(fmt.Errorf("unsupported resource_type %q", resourceTypeRaw)), nil
		}
		req = reporting.MetricReportRequest{
			ResourceType: canonicalType,
			ResourceID:   resourceID,
		}
	}
	req.Start = start
	req.End = end
	req.Narrator = e.reportNarrator
	req.FindingsProvider = e.reportFindingsProvider
	narrative, err := engine.NarrativeFor(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("narrative generation failed: %w", err)), nil
	}
	if narrative == nil {
		return NewErrorResult(fmt.Errorf("narrative generation produced no result")), nil
	}

	// Telemetry: structured event line per summarize invocation so
	// chat-side reporting usage can be audited alongside report-PDF
	// generation. Same event-name convention as the API handlers.
	log.Info().
		Str("event", "reporting.summarize.invoked").
		Str("org_id", e.orgID).
		Str("action", "resource").
		Str("resource_type", canonicalType).
		Str("narrative_source", narrative.Source).
		Bool("ai_configured", e.reportNarrator != nil).
		Bool("findings_configured", e.reportFindingsProvider != nil).
		Time("window_start", start).
		Time("window_end", end).
		Msg("Reporting: pulse_summarize invoked")

	return NewJSONResult(summarizeResourceResponse{
		OK:              true,
		Action:          "resource",
		ResourceType:    canonicalType,
		ResourceID:      resourceID,
		WindowStart:     start,
		WindowEnd:       end,
		NarrativeSource: narrative.Source,
		HealthStatus:    narrative.HealthStatus,
		HealthMessage:   narrative.HealthMessage,
		Observations:    narrative.Observations,
		Recommendations: narrative.Recommendations,
		Disclaimer:      narrative.Disclaimer,
	}), nil
}

type summarizeFleetResponse struct {
	OK              bool                        `json:"ok"`
	Action          string                      `json:"action"`
	ResourceIDs     []string                    `json:"resource_ids"`
	Resources       []summarizeFleetEntry       `json:"resources,omitempty"`
	Enumerated      bool                        `json:"enumerated,omitempty"`
	Note            string                      `json:"note,omitempty"`
	WindowStart     time.Time                   `json:"window_start"`
	WindowEnd       time.Time                   `json:"window_end"`
	NarrativeSource string                      `json:"narrative_source"`
	HealthStatus    string                      `json:"health_status,omitempty"`
	HealthMessage   string                      `json:"health_message,omitempty"`
	Outliers        []reporting.FleetOutlier    `json:"outliers,omitempty"`
	Patterns        []reporting.NarrativeBullet `json:"patterns,omitempty"`
	Recommendations []string                    `json:"recommendations,omitempty"`
	Disclaimer      string                      `json:"disclaimer,omitempty"`
}

// summarizeFleetEntry names one fleet member in the response so the model can
// narrate outliers with human-readable identities instead of opaque IDs.
type summarizeFleetEntry struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// summarizeMetricsTargetResolver is the optional capability the unified
// resource provider exposes for translating canonical resource IDs into
// metrics-store query targets. The production provider (the monitor's
// unified adapter) implements it; lightweight fixtures may not.
type summarizeMetricsTargetResolver interface {
	MetricsTargetForResource(resourceID string) *unifiedresources.MetricsTarget
}

// summarizeFleetCandidate is a unified resource resolved into the reporting
// engine's request shape: a reporting resource type, the canonical resource
// ID (which keys Patrol findings and recovery points), and the metrics-store
// query ID when the resource's metrics target differs from the canonical ID.
type summarizeFleetCandidate struct {
	reportType string
	id         string
	name       string
	status     string
	metricsID  string
}

// summarizeFleetEnumerationOrder lists the unified resource types included in
// a self-enumerated fleet summary, highest-signal first: infrastructure
// parents, then guests, then storage. The order decides what survives the
// summarizeFleetMaxResources cap.
var summarizeFleetEnumerationOrder = []unifiedresources.ResourceType{
	unifiedresources.ResourceTypeAgent,
	unifiedresources.ResourceTypePBS,
	unifiedresources.ResourceTypePMG,
	unifiedresources.ResourceTypeK8sCluster,
	unifiedresources.ResourceTypeVM,
	unifiedresources.ResourceTypeSystemContainer,
	unifiedresources.ResourceTypeAppContainer,
	unifiedresources.ResourceTypeStorage,
}

// summarizeReportTypeForResource maps a unified resource to the reporting
// engine's canonical resource-type vocabulary. Host-flavored resources
// (unified type "agent") are classified by their richest data source the same
// way the platform pages type them: agent-backed hosts report as "agent",
// pure Proxmox nodes as "node", pure Docker hosts as "docker-host".
func summarizeReportTypeForResource(res unifiedresources.Resource) string {
	switch unifiedresources.CanonicalResourceType(res.Type) {
	case unifiedresources.ResourceTypeAgent:
		switch {
		case res.Agent != nil:
			return "agent"
		case res.Proxmox != nil:
			return "node"
		case res.Docker != nil:
			return "docker-host"
		default:
			return "agent"
		}
	case unifiedresources.ResourceTypeVM:
		return "vm"
	case unifiedresources.ResourceTypeSystemContainer:
		return "system-container"
	case unifiedresources.ResourceTypeAppContainer:
		return "app-container"
	case unifiedresources.ResourceTypeK8sCluster:
		return "k8s"
	case unifiedresources.ResourceTypeStorage:
		return "storage"
	case unifiedresources.ResourceTypePBS:
		return "pbs"
	case unifiedresources.ResourceTypePMG:
		return "pmg"
	case unifiedresources.ResourceTypePhysicalDisk:
		return "disk"
	case unifiedresources.ResourceTypePod:
		return "pod"
	default:
		return ""
	}
}

// summarizeMetricsTarget resolves the metrics-store target for a unified
// resource, asking the provider's on-demand resolver first (registry targets
// are computed lazily; the struct field is only populated by fixtures).
func (e *PulseToolExecutor) summarizeMetricsTarget(res unifiedresources.Resource) *unifiedresources.MetricsTarget {
	if resolver, ok := e.unifiedResourceProvider.(summarizeMetricsTargetResolver); ok && resolver != nil {
		if target := resolver.MetricsTargetForResource(res.ID); target != nil {
			return target
		}
	}
	return res.MetricsTarget
}

// summarizeFleetCandidateFor projects a unified resource into a fleet
// candidate, mirroring the API report path's resolveReportSubject: the
// canonical ID stays the request ResourceID (findings and recovery points key
// on it) while the metrics target rides MetricsResourceID. When the resolved
// target's type is one reporting understands, it wins over the static
// classification — merged host resources advertise the store family their
// metrics are actually written under. Pure Proxmox nodes are the documented
// exception: their metrics live under the "node" store type while the target
// labels the agent family, so the node classification is kept there.
func (e *PulseToolExecutor) summarizeFleetCandidateFor(res unifiedresources.Resource) (summarizeFleetCandidate, bool) {
	id := strings.TrimSpace(res.ID)
	reportType := summarizeReportTypeForResource(res)
	if id == "" || reportType == "" {
		return summarizeFleetCandidate{}, false
	}
	cand := summarizeFleetCandidate{
		reportType: reportType,
		id:         id,
		name:       strings.TrimSpace(res.Name),
		status:     string(res.Status),
	}
	if target := e.summarizeMetricsTarget(res); target != nil {
		cand.metricsID = strings.TrimSpace(target.ResourceID)
		if reportType != "node" {
			if canonical := reporting.CanonicalResourceType(target.ResourceType); canonical != "" {
				cand.reportType = canonical
			}
		}
	}
	return cand, true
}

// enumerateSummarizeFleet walks the unified resource provider in priority
// order and returns deduped fleet candidates, optionally filtered to one
// reporting type. Returns nil when no provider is wired.
func (e *PulseToolExecutor) enumerateSummarizeFleet(filterType string) []summarizeFleetCandidate {
	if e.unifiedResourceProvider == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []summarizeFleetCandidate
	for _, resourceType := range summarizeFleetEnumerationOrder {
		for _, res := range e.unifiedResourceProvider.GetByType(resourceType) {
			cand, ok := e.summarizeFleetCandidateFor(res)
			if !ok {
				continue
			}
			if filterType != "" && cand.reportType != filterType {
				continue
			}
			if _, dup := seen[cand.id]; dup {
				continue
			}
			seen[cand.id] = struct{}{}
			out = append(out, cand)
		}
	}
	return out
}

// summarizeCandidateIndex indexes every enumerable resource by canonical ID
// and, when unambiguous, by lowercased name — operators and models refer to
// resources by name far more readily than by internal identifier. ID matches
// always win over name matches.
type summarizeCandidateIndex struct {
	byID   map[string]summarizeFleetCandidate
	byName map[string]summarizeFleetCandidate
}

func (idx *summarizeCandidateIndex) lookup(ref string) (summarizeFleetCandidate, bool) {
	if idx == nil {
		return summarizeFleetCandidate{}, false
	}
	if cand, ok := idx.byID[ref]; ok {
		return cand, true
	}
	cand, ok := idx.byName[strings.ToLower(ref)]
	return cand, ok
}

func (e *PulseToolExecutor) buildSummarizeCandidateIndex() *summarizeCandidateIndex {
	candidates := e.enumerateSummarizeFleet("")
	if len(candidates) == 0 {
		return nil
	}
	idx := &summarizeCandidateIndex{
		byID:   make(map[string]summarizeFleetCandidate, len(candidates)),
		byName: make(map[string]summarizeFleetCandidate, len(candidates)),
	}
	ambiguous := make(map[string]struct{})
	for _, cand := range candidates {
		idx.byID[cand.id] = cand
		name := strings.ToLower(cand.name)
		if name == "" {
			continue
		}
		if _, dup := idx.byName[name]; dup {
			ambiguous[name] = struct{}{}
			continue
		}
		idx.byName[name] = cand
	}
	for name := range ambiguous {
		delete(idx.byName, name)
	}
	return idx
}

// summarizeFleetMaxResources caps fleet inputs so a single tool call
// can't query unbounded resources. Matches the reporting catalog's
// MultiResourceMax so the tool's contract aligns with the API's.
const summarizeFleetMaxResources = 50

func (e *PulseToolExecutor) summarizeFleet(
	_ context.Context,
	engine reporting.Engine,
	args map[string]interface{},
	start, end time.Time,
) (CallToolResult, error) {
	defaultType, _ := args["resource_type"].(string)
	defaultType = strings.TrimSpace(defaultType)
	canonicalDefault := ""
	if defaultType != "" {
		canonicalDefault = reporting.CanonicalResourceType(defaultType)
		if canonicalDefault == "" {
			return NewErrorResult(fmt.Errorf("unsupported resource_type %q", defaultType)), nil
		}
	}

	rawIDs, _ := args["resource_ids"].(string)
	rawIDs = strings.TrimSpace(rawIDs)

	var candidates []summarizeFleetCandidate
	enumerated := false
	note := ""
	if rawIDs == "" {
		// No explicit list: enumerate the known fleet ourselves. The
		// natural first-session question ("how is my machine doing?")
		// must succeed without the model knowing any internal IDs.
		candidates = e.enumerateSummarizeFleet(canonicalDefault)
		if len(candidates) == 0 {
			if canonicalDefault != "" {
				return NewErrorResult(fmt.Errorf("no monitored resources of type %q found to summarize; retry without resource_type to enumerate the whole fleet — do not ask the operator for resource IDs", canonicalDefault)), nil
			}
			return NewErrorResult(fmt.Errorf("no monitored resources found to summarize: Pulse is not monitoring anything yet, so tell the user to connect a node or agent first — do not ask the operator for resource IDs")), nil
		}
		enumerated = true
		if len(candidates) > summarizeFleetMaxResources {
			note = fmt.Sprintf("fleet truncated to the first %d of %d known resources (infrastructure first)", summarizeFleetMaxResources, len(candidates))
			candidates = candidates[:summarizeFleetMaxResources]
		}
	} else {
		parts := strings.Split(rawIDs, ",")
		if len(parts) > summarizeFleetMaxResources {
			return NewErrorResult(fmt.Errorf("fleet summarize accepts at most %d resources; got %d", summarizeFleetMaxResources, len(parts))), nil
		}
		index := e.buildSummarizeCandidateIndex()
		seen := make(map[string]struct{}, len(parts))
		var unresolved []string
		for _, raw := range parts {
			s := strings.TrimSpace(raw)
			if s == "" {
				continue
			}
			cand, ok := index.lookup(s)
			if !ok {
				if canonicalDefault == "" {
					unresolved = append(unresolved, s)
					continue
				}
				// Unrecognized reference with an explicit default type:
				// pass it through untranslated — the caller may be
				// addressing the metrics store's native ID space.
				cand = summarizeFleetCandidate{reportType: canonicalDefault, id: s}
			}
			if _, dup := seen[cand.id]; dup {
				continue
			}
			seen[cand.id] = struct{}{}
			candidates = append(candidates, cand)
		}
		if len(unresolved) > 0 {
			return NewErrorResult(fmt.Errorf("could not resolve %s to known resources; retry with action=fleet and no resource_ids to enumerate the fleet automatically, or add resource_type for identifiers from the metrics ID space — do not ask the operator for resource IDs", strings.Join(unresolved, ", "))), nil
		}
		if len(candidates) == 0 {
			return NewErrorResult(fmt.Errorf("resource_ids parsed to zero non-empty identifiers")), nil
		}
	}

	resources := make([]reporting.MetricReportRequest, 0, len(candidates))
	ids := make([]string, 0, len(candidates))
	entries := make([]summarizeFleetEntry, 0, len(candidates))
	for _, cand := range candidates {
		req := reporting.MetricReportRequest{
			ResourceType:      cand.reportType,
			ResourceID:        cand.id,
			MetricsResourceID: cand.metricsID,
		}
		if cand.name != "" {
			req.Resource = &reporting.ResourceInfo{Name: cand.name, Status: cand.status}
		}
		resources = append(resources, req)
		ids = append(ids, cand.id)
		entries = append(entries, summarizeFleetEntry{ID: cand.id, Type: cand.reportType, Name: cand.name})
	}

	req := reporting.MultiReportRequest{
		Title:            "Fleet summary",
		Start:            start,
		End:              end,
		Resources:        resources,
		FleetNarrator:    e.reportFleetNarrator,
		Narrator:         e.reportNarrator,
		FindingsProvider: e.reportFindingsProvider,
	}
	narrative, err := engine.FleetNarrativeFor(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("fleet narrative generation failed: %w", err)), nil
	}
	if narrative == nil {
		return NewErrorResult(fmt.Errorf("fleet narrative generation produced no result")), nil
	}

	log.Info().
		Str("event", "reporting.summarize.invoked").
		Str("org_id", e.orgID).
		Str("action", "fleet").
		Str("resource_type", canonicalDefault).
		Bool("enumerated", enumerated).
		Int("resource_count", len(ids)).
		Str("narrative_source", narrative.Source).
		Bool("ai_configured", e.reportFleetNarrator != nil).
		Bool("findings_configured", e.reportFindingsProvider != nil).
		Time("window_start", start).
		Time("window_end", end).
		Msg("Reporting: pulse_summarize invoked")

	return NewJSONResult(summarizeFleetResponse{
		OK:              true,
		Action:          "fleet",
		ResourceIDs:     ids,
		Resources:       entries,
		Enumerated:      enumerated,
		Note:            note,
		WindowStart:     start,
		WindowEnd:       end,
		NarrativeSource: narrative.Source,
		HealthStatus:    narrative.HealthStatus,
		HealthMessage:   narrative.HealthMessage,
		Outliers:        narrative.Outliers,
		Patterns:        narrative.Patterns,
		Recommendations: narrative.Recommendations,
		Disclaimer:      narrative.Disclaimer,
	}), nil
}
