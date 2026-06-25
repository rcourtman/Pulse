package agentcapabilities

import (
	"strings"
	"testing"
)

func TestRequiredCapabilityScopeMarkdownListUsesManifestScopeSummary(t *testing.T) {
	got := RequiredCapabilityScopeMarkdownList([]Capability{
		{Name: "settings_write", Scope: "settings:write"},
		{Name: "monitoring_read", Scope: "monitoring:read"},
		{Name: "ai_execute", Scope: "ai:execute"},
		{Name: "duplicate_monitoring_read", Scope: "monitoring:read"},
	})
	want := "`monitoring:read`, `settings:write`, and `ai:execute`"
	if got != want {
		t.Fatalf("scope markdown list = %q, want %q", got, want)
	}
}

func TestManifestRequiredScopeMarkdownListPrefersRequiredScopes(t *testing.T) {
	got := ManifestRequiredScopeMarkdownList(Manifest{
		RequiredScopes: []string{
			"settings:write",
			"monitoring:read",
			"ai:execute",
			"monitoring:read",
		},
		Capabilities: []Capability{
			{Name: "legacy_extra", Scope: "monitoring:write"},
		},
	})
	want := "`monitoring:read`, `settings:write`, and `ai:execute`"
	if got != want {
		t.Fatalf("manifest scope markdown list = %q, want %q", got, want)
	}
}

func TestManifestRequiredScopeMarkdownListFallsBackForLegacyManifest(t *testing.T) {
	got := ManifestRequiredScopeMarkdownList(Manifest{
		Capabilities: []Capability{
			{Name: "settings_write", Scope: "settings:write"},
			{Name: "monitoring_read", Scope: "monitoring:read"},
		},
	})
	want := "`monitoring:read` and `settings:write`"
	if got != want {
		t.Fatalf("legacy manifest scope markdown list = %q, want %q", got, want)
	}
}

func TestMCPClientConfigMarkdownProjectsManifestAdapterContract(t *testing.T) {
	got := MCPClientConfigMarkdown(CanonicalManifest().MCPAdapter)

	for _, expected := range []string{
		"server name `pulse`, command `pulse-mcp`, base URL flag `--base-url`, default URL `http://localhost:7655`, and token environment variable `PULSE_API_TOKEN`",
		"currently declared config families: `OpenCode`, `Claude-style clients`, and `custom MCP clients`",
		"#### OpenCode",
		`"command": ["pulse-mcp", "--base-url", "http://localhost:7655"]`,
		`"PULSE_API_TOKEN": "your-token-here"`,
		"#### Claude-style clients",
		`"mcpServers": {`,
		`"args": ["--base-url", "http://localhost:7655"]`,
		"#### custom MCP clients",
		"Use command `pulse-mcp`, pass `--base-url http://localhost:7655`, set `PULSE_API_TOKEN` to the API token, and keep the server name `pulse`",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("MCP client config markdown missing %q:\n%s", expected, got)
		}
	}
}

func TestMCPClientConfigMarkdownFallsBackForLegacyManifest(t *testing.T) {
	got := MCPClientConfigMarkdown(MCPAdapterContract{})

	for _, expected := range []string{
		"command `pulse-mcp`",
		"base URL flag `--base-url`",
		"token environment variable `PULSE_API_TOKEN`",
		"#### OpenCode",
		"#### Claude-style clients",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("legacy MCP client config markdown missing %q:\n%s", expected, got)
		}
	}
}

func TestPulseIntelligenceOverviewMarkdownProjectsManifestSurfaceContract(t *testing.T) {
	got := PulseIntelligenceOverviewMarkdown(CanonicalManifest().SurfaceContract)

	for _, expected := range []string{
		"shared **Pulse Intelligence Core**: Canonical context, governed actions, safety gates, approval state, action audit, and verification shared by Pulse Assistant, Pulse MCP, and Pulse Patrol.",
		"Patrol as the primary built-in operator and Assistant plus MCP as access paths",
		"1. **Pulse Patrol**: Patrol is the first-party operations surface: it checks infrastructure, investigates issues, follows the chosen Patrol mode before acting, verifies outcomes, and records what happened.",
		"2. **Pulse Assistant**: The contextual explanation, approval, and handoff surface for Patrol findings, governed actions, verification, and operator questions. Affordances: tools and interactive questions.",
		"3. **Pulse MCP**: The external-agent adapter that projects canonical Pulse Intelligence capabilities as MCP tools. Affordances: tools, resources, prompts, and capability metadata.",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("overview markdown missing %q:\n%s", expected, got)
		}
	}
}

func TestPulseIntelligenceOverviewMarkdownFallsBackForLegacyManifest(t *testing.T) {
	got := PulseIntelligenceOverviewMarkdown(SurfaceContract{})

	for _, expected := range []string{
		"shared **Pulse Intelligence Core**: Canonical context, governed actions, safety gates, approval state, action audit, and verification.",
		"zero supported operator-facing surfaces",
		"Supported operator-facing surfaces are declared by the agent capabilities manifest.",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("legacy overview markdown missing %q:\n%s", expected, got)
		}
	}
}

func TestMCPSurfaceContractMarkdownProjectsManifestSurfaceContract(t *testing.T) {
	got := MCPSurfaceContractMarkdown(CanonicalManifest().SurfaceContract)

	for _, expected := range []string{
		"**Pulse Intelligence Core**: Canonical context, governed actions, safety gates, approval state, action audit, and verification shared by Pulse Assistant, Pulse MCP, and Pulse Patrol.",
		"**Pulse Patrol**: Patrol is the first-party operations surface: it checks infrastructure, investigates issues, follows the chosen Patrol mode before acting, verifies outcomes, and records what happened.",
		"**Pulse Assistant** (the native Pulse surface): The contextual explanation, approval, and handoff surface for Patrol findings, governed actions, verification, and operator questions. Affordances: tools and interactive questions.",
		"**Pulse MCP**: The external-agent adapter that projects canonical Pulse Intelligence capabilities as MCP tools. Affordances: tools, resources, prompts, and capability metadata.",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("surface contract markdown missing %q:\n%s", expected, got)
		}
	}
}

func TestMCPSurfaceContractMarkdownFallsBackForLegacyManifest(t *testing.T) {
	got := MCPSurfaceContractMarkdown(SurfaceContract{})

	for _, expected := range []string{
		"**Pulse Intelligence Core**: The shared context, governed tool, safety gate, approval, audit, and verification substrate.",
		"**Pulse MCP**: The external-agent adapter over the same manifest-backed Pulse Intelligence capabilities.",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("legacy surface contract markdown missing %q:\n%s", expected, got)
		}
	}
}

func TestMCPToolCapabilityInventoryMarkdownProjectsManifestTools(t *testing.T) {
	got := MCPToolCapabilityInventoryMarkdown(Manifest{
		SurfaceContract: CanonicalManifest().SurfaceContract,
		SurfaceToolContracts: []SurfaceToolContract{{
			SurfaceID: SurfaceIDPulseMCP,
			ToolNames: []string{
				FleetContextCapabilityName,
				SetOperatorStateCapabilityName,
				EventSubscriptionCapabilityName,
			},
		}},
		Capabilities: []Capability{
			{
				Name:           FleetContextCapabilityName,
				Title:          "Fleet triage",
				Description:    "Return a fleet triage rollup.",
				Category:       "context",
				Method:         "GET",
				Path:           "/api/agent/fleet-context",
				Scope:          "monitoring:read",
				ActionMode:     ActionModeRead,
				ApprovalPolicy: ApprovalPolicyScopeOnly,
			},
			{
				Name:           ResourceContextCapabilityName,
				Description:    "Raw manifest capability omitted from Pulse MCP surface.",
				Category:       "context",
				Method:         "GET",
				Path:           ResourceContextCapabilityPath,
				Scope:          "monitoring:read",
				ActionMode:     ActionModeRead,
				ApprovalPolicy: ApprovalPolicyScopeOnly,
			},
			{
				Name:           EventSubscriptionCapabilityName,
				Description:    "Stream events.",
				Category:       "context",
				Method:         "GET",
				Path:           "/api/agent/events",
				Scope:          "monitoring:read",
				ActionMode:     ActionModeRead,
				ApprovalPolicy: ApprovalPolicyScopeOnly,
			},
			{
				Name:           SetOperatorStateCapabilityName,
				Title:          "Set operator state",
				Description:    "Replace the operator-set state for a resource.",
				Category:       "operator-state",
				Method:         "PUT",
				Path:           OperatorStateCapabilityPath,
				Scope:          "monitoring:write",
				ActionMode:     ActionModeWrite,
				ApprovalPolicy: ApprovalPolicyScopeOnly,
			},
		},
	})
	want := strings.Join([]string{
		"**Context (read-only):**",
		"",
		"- `get_fleet_context` (Fleet triage, `GET /api/agent/fleet-context`, scope `monitoring:read`, mode `read`, approval `scope_only`): Return a fleet triage rollup.",
		"",
		"**Operator state (per-resource intent):**",
		"",
		"- `set_operator_state` (Set operator state, `PUT /api/resources/{resourceId}/operator-state`, scope `monitoring:write`, mode `write`, approval `scope_only`): Replace the operator-set state for a resource.",
	}, "\n")
	if got != want {
		t.Fatalf("MCP tool inventory markdown:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, EventSubscriptionCapabilityName) {
		t.Fatal("streaming subscribe_events capability must not render as a request/response MCP tool")
	}
	if strings.Contains(got, ResourceContextCapabilityName) {
		t.Fatal("raw manifest capability omitted from Pulse MCP surface contract must not render as an MCP tool")
	}
}

func TestMCPToolCapabilityInventoryMarkdownHandlesEmptyManifest(t *testing.T) {
	got := MCPToolCapabilityInventoryMarkdown(Manifest{})
	want := "No request/response MCP tools are currently advertised by the manifest."
	if got != want {
		t.Fatalf("empty inventory = %q, want %q", got, want)
	}
}

func TestMCPPromptInventoryMarkdownProjectsManifestPrompts(t *testing.T) {
	got := MCPPromptInventoryMarkdown(Manifest{
		SurfaceContract: CanonicalManifest().SurfaceContract,
		Capabilities: []Capability{
			{Name: FleetContextCapabilityName},
			{Name: ResourceContextCapabilityName},
			{Name: ListFindingsCapabilityName},
		},
		WorkflowPrompts: []PulseWorkflowPrompt{
			{
				Name:        PulseWorkflowPromptTriageFleet,
				Label:       "Triage fleet",
				Description: "Triage the Pulse fleet using the canonical fleet context capability, then choose where deeper investigation is warranted.",
			},
			{
				Name:        PulseWorkflowPromptOperationsLoop,
				Label:       "Ask Patrol to handle an issue",
				Description: "Have Patrol investigate active findings, follow the configured Patrol mode, take approved actions, verify the outcome, and record what happened.",
			},
			{
				Name:        PulseWorkflowPromptInvestigateResource,
				Label:       "Investigate resource",
				Description: "Investigate one Pulse resource using the canonical resource context capability and resource URI projection.",
				Arguments: []PulseWorkflowPromptArgument{{
					Name:     ResourceIDArgumentName,
					Required: true,
				}},
			},
			{
				Name:        PulseWorkflowPromptReviewFinding,
				Label:       "Review finding",
				Description: "Review one Patrol finding and propose the safest governed next step.",
				Arguments: []PulseWorkflowPromptArgument{{
					Name:     FindingIDArgumentName,
					Required: true,
				}},
			},
		},
	})
	want := strings.Join([]string{
		"- `pulse_triage_fleet` (Triage fleet): Triage the Pulse fleet using the canonical fleet context capability, then choose where deeper investigation is warranted.",
		"- `pulse_operations_loop` (Ask Patrol to handle an issue): Have Patrol investigate active findings, follow the configured Patrol mode, take approved actions, verify the outcome, and record what happened.",
		"- `pulse_investigate_resource` (Investigate resource; required argument `resourceId`): Investigate one Pulse resource using the canonical resource context capability and resource URI projection.",
		"- `pulse_review_finding` (Review finding; required argument `finding_id`): Review one Patrol finding and propose the safest governed next step.",
	}, "\n")
	if got != want {
		t.Fatalf("MCP prompt inventory markdown:\n%s\nwant:\n%s", got, want)
	}
}

func TestMCPPromptInventoryMarkdownHonorsSurfacePromptAffordance(t *testing.T) {
	got := MCPPromptInventoryMarkdown(Manifest{
		SurfaceContract: SurfaceContract{
			OperatorSurfaces: []OperatorSurfaceContract{{
				ID:          SurfaceIDPulseMCP,
				Label:       "Pulse MCP",
				Affordances: SurfaceAffordanceContract{Tools: true},
			}},
		},
		WorkflowPrompts: []PulseWorkflowPrompt{{
			Name:        PulseWorkflowPromptTriageFleet,
			Label:       "Triage fleet",
			Description: "Triage the Pulse fleet.",
		}},
	})
	want := "No MCP workflow prompts are currently advertised by the manifest."
	if got != want {
		t.Fatalf("prompt-disabled inventory = %q, want %q", got, want)
	}
}

func TestMCPPromptInventoryMarkdownHandlesExplicitEmptyPromptCatalogue(t *testing.T) {
	got := MCPPromptInventoryMarkdown(Manifest{
		SurfaceContract: CanonicalManifest().SurfaceContract,
		Capabilities: []Capability{
			{Name: FleetContextCapabilityName},
			{Name: ResourceContextCapabilityName},
			{Name: ListFindingsCapabilityName},
		},
		WorkflowPrompts: []PulseWorkflowPrompt{},
	})
	want := "No MCP workflow prompts are currently advertised by the manifest."
	if got != want {
		t.Fatalf("empty prompt inventory = %q, want %q", got, want)
	}
}

func TestMCPErrorCodeInventoryMarkdownProjectsManifestCodes(t *testing.T) {
	got := MCPErrorCodeInventoryMarkdown(Manifest{
		SurfaceContract: CanonicalManifest().SurfaceContract,
		SurfaceToolContracts: []SurfaceToolContract{{
			SurfaceID: SurfaceIDPulseMCP,
			ToolNames: []string{
				ResourceContextCapabilityName,
				PlanActionCapabilityName,
				EventSubscriptionCapabilityName,
			},
		}},
		Capabilities: []Capability{
			{
				Name:       FleetContextCapabilityName,
				ErrorCodes: []string{"not_documented_for_this_surface"},
			},
			{
				Name:       ResourceContextCapabilityName,
				ErrorCodes: []string{"resource_not_found"},
			},
			{
				Name:       EventSubscriptionCapabilityName,
				ErrorCodes: []string{"sse_unavailable"},
			},
			{
				Name:       PlanActionCapabilityName,
				ErrorCodes: []string{"invalid_action_request", "resource_not_found", "invalid_action_request", ""},
			},
		},
	})
	want := strings.Join([]string{
		"- `get_resource_context`: `resource_not_found`",
		"- `plan_action`: `invalid_action_request` and `resource_not_found`",
	}, "\n")
	if got != want {
		t.Fatalf("MCP error-code inventory markdown:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, FleetContextCapabilityName) {
		t.Fatal("raw manifest capability omitted from Pulse MCP surface contract must not render MCP error codes")
	}
	if strings.Contains(got, EventSubscriptionCapabilityName) {
		t.Fatal("streaming subscribe_events capability must not render MCP tool error codes")
	}
}

func TestMCPErrorCodeInventoryMarkdownHandlesNoErrorCodes(t *testing.T) {
	got := MCPErrorCodeInventoryMarkdown(Manifest{
		SurfaceContract: CanonicalManifest().SurfaceContract,
		SurfaceToolContracts: []SurfaceToolContract{{
			SurfaceID: SurfaceIDPulseMCP,
			ToolNames: []string{FleetContextCapabilityName},
		}},
		Capabilities: []Capability{{Name: FleetContextCapabilityName}},
	})
	want := "No capability-specific stable error codes are currently advertised by the manifest."
	if got != want {
		t.Fatalf("empty error-code inventory = %q, want %q", got, want)
	}
}
