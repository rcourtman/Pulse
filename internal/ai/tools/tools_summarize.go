package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
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
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_summarize",
			Description: `Generate a retrospective summary of one resource or a fleet across a time window. Use this when the operator asks questions like "what's been happening with pve1 this week" or "where should I look across my fleet" — answers grounded in metric stats, alerts, storage state, disk health, and Patrol findings within the window.

Two modes via the 'action' parameter:
  - "resource": summarises a single resource. Required: resource_type, resource_id.
  - "fleet":    summarises a fleet across multiple resources. Required: resource_ids (list).

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
						Description: "For action=resource: canonical resource type (node, vm, system-container, oci-container, app-container, docker-host, storage, agent, k8s, disk, pbs, pmg). For action=fleet: optional default type when resource_ids omit per-entry type.",
					},
					"resource_id": {
						Type:        "string",
						Description: "For action=resource: the resource identifier (e.g. instance:node:vmid).",
					},
					"resource_ids": {
						Type:        "string",
						Description: "For action=fleet: comma-separated list of resource identifiers to include (e.g. \"instance:pve1:101,instance:pve1:102\"). Use pulse_query to enumerate resources of a type first if you need the full set.",
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
			ActionMode:     ToolActionRead,
			ApprovalPolicy: "no approval required; pure read of metrics history and findings store.",
			Summary:        "Returns a retrospective synthesis (observations, recommendations, outliers, period comparison) for one resource or a fleet within a time window.",
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
	if resourceTypeRaw == "" {
		return NewErrorResult(fmt.Errorf("resource_type is required for action=resource")), nil
	}
	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required for action=resource")), nil
	}
	canonicalType := reporting.CanonicalResourceType(resourceTypeRaw)
	if canonicalType == "" {
		return NewErrorResult(fmt.Errorf("unsupported resource_type %q", resourceTypeRaw)), nil
	}

	req := reporting.MetricReportRequest{
		ResourceType: canonicalType,
		ResourceID:   resourceID,
		Start:        start,
		End:          end,
	}
	narrative, err := engine.NarrativeFor(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("narrative generation failed: %w", err)), nil
	}
	if narrative == nil {
		return NewErrorResult(fmt.Errorf("narrative generation produced no result")), nil
	}

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
	if rawIDs == "" {
		return NewErrorResult(fmt.Errorf("resource_ids (comma-separated) is required for action=fleet")), nil
	}
	if canonicalDefault == "" {
		return NewErrorResult(fmt.Errorf("resource_type is required for action=fleet")), nil
	}
	parts := strings.Split(rawIDs, ",")
	if len(parts) > summarizeFleetMaxResources {
		return NewErrorResult(fmt.Errorf("fleet summarize accepts at most %d resources; got %d", summarizeFleetMaxResources, len(parts))), nil
	}
	resources := make([]reporting.MetricReportRequest, 0, len(parts))
	ids := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, raw := range parts {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		resources = append(resources, reporting.MetricReportRequest{
			ResourceType: canonicalDefault,
			ResourceID:   s,
		})
		ids = append(ids, s)
	}
	if len(resources) == 0 {
		return NewErrorResult(fmt.Errorf("resource_ids parsed to zero non-empty identifiers")), nil
	}

	req := reporting.MultiReportRequest{
		Title:     "Fleet summary",
		Start:     start,
		End:       end,
		Resources: resources,
	}
	narrative, err := engine.FleetNarrativeFor(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("fleet narrative generation failed: %w", err)), nil
	}
	if narrative == nil {
		return NewErrorResult(fmt.Errorf("fleet narrative generation produced no result")), nil
	}

	return NewJSONResult(summarizeFleetResponse{
		OK:              true,
		Action:          "fleet",
		ResourceIDs:     ids,
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
