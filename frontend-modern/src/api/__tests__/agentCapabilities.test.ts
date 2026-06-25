import { describe, expect, it, vi } from 'vitest';
import {
  AGENT_CAPABILITIES_PATH,
  AGENT_OPERATIONS_LOOP_STATUS_PATH,
  AGENT_SURFACE_ID_PULSE_MCP,
  fetchAgentCapabilitiesManifest,
  fetchAgentOperationsLoopStatus,
  formatAgentMCPServersConfig,
  formatAgentOpenCodeMCPConfig,
  getAgentCapabilityErrorCodeSummaries,
  getAgentMCPOperationsLoopReadiness,
  getAgentManifestSurfaceToolContract,
  getAgentManifestSurfaceToolContracts,
  getAgentMCPClientExamples,
  getAgentMCPConfigFamilyByShape,
  getAgentSurfaceToolPosturePresentation,
  getAgentSurfaceContractEntries,
  getAgentWorkflowPrompts,
  groupAgentCapabilitiesByManifestCategories,
  normalizeAgentMCPAdapter,
  normalizeAgentSurfaceToolContract,
  type AgentCapability,
  type AgentCapabilitiesManifest,
  type AgentSurfaceToolContract,
} from '../agentCapabilities';
import generatedAgentCapabilitiesSource from '@/api/generated/agentCapabilities.ts?raw';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

const capability = (name: string, category: string): AgentCapability => ({
  name,
  category,
  description: `${name} description`,
  method: 'GET',
  path: `/api/${name}`,
  scope: 'monitoring:read',
  actionMode: 'read',
  approvalPolicy: 'scope_only',
});

const surfaceContract = {
  core: {
    id: 'pulse_intelligence_core',
    label: 'Pulse Intelligence Core',
    description:
      'Canonical context, governed actions, safety gates, approval state, action audit, and verification.',
  },
  proactiveEngine: {
    id: 'pulse_patrol',
    label: 'Pulse Patrol',
    description:
      'Patrol is the first-party operations surface: it checks infrastructure, investigates issues, follows the chosen Patrol mode before acting, verifies outcomes, and records what happened.',
  },
  operatorSurfaces: [
    {
      id: 'pulse_assistant',
      label: 'Pulse Assistant',
      description:
        'The contextual explanation, approval, and handoff surface for Patrol findings, governed actions, verification, and operator questions.',
      native: true,
      externalAdapter: false,
      affordances: {
        tools: true,
        interactiveQuestions: true,
      },
    },
    {
      id: 'pulse_mcp',
      label: 'Pulse MCP',
      description: 'The external-agent adapter.',
      native: false,
      externalAdapter: true,
      affordances: {
        tools: true,
        resources: true,
        prompts: true,
        capabilityMetadata: true,
      },
    },
  ],
};

const mcpAdapter = {
  serverName: 'pulse',
  command: 'pulse-mcp',
  baseUrlFlag: '--base-url',
  defaultBaseUrl: 'http://localhost:7655',
  tokenEnv: 'PULSE_API_TOKEN',
  configFamilies: [
    {
      id: 'opencode',
      label: 'OpenCode',
      shape: 'opencode_mcp',
      description: "Uses OpenCode's top-level mcp object.",
      fileHints: ['opencode.json', 'opencode.jsonc'],
      clientLabels: ['OpenCode'],
    },
    {
      id: 'claude-style',
      label: 'Claude-style clients',
      shape: 'mcp_servers',
      description: 'Uses the common mcpServers object.',
      fileHints: ['claude_desktop_config.json', '.mcp.json'],
      clientLabels: ['Claude Desktop', 'Claude Code'],
    },
    {
      id: 'custom-mcp',
      label: 'custom MCP clients',
      shape: 'custom',
      clientLabels: ['custom MCP clients'],
    },
  ],
};

describe('agent capabilities API client', () => {
  it('fetches the public manifest through the shared API client', async () => {
    const manifest = {
      version: 'v1',
      surfaceContract,
      surfaceToolContracts: [],
      mcpAdapter,
      requiredScopes: ['monitoring:read'],
      categories: [{ id: 'context', label: 'Context (read-only)' }],
      workflowPrompts: [],
      capabilities: [capability('get_resource_context', 'context')],
    } satisfies AgentCapabilitiesManifest;
    vi.mocked(apiFetchJSON).mockResolvedValueOnce(manifest);

    await expect(fetchAgentCapabilitiesManifest()).resolves.toBe(manifest);

    expect(apiFetchJSON).toHaveBeenCalledWith(AGENT_CAPABILITIES_PATH, {
      skipAuth: true,
      headers: { Accept: 'application/json' },
    });
  });

  it('fetches the authenticated operations-loop status through the shared API client', async () => {
    const status = {
      nextAction: 'open_assistant',
      progressLabel: 'Open Assistant to explain the Patrol issue and safest next step.',
      steps: [
        { id: 'patrol', label: 'Patrol', status: 'complete', count: 1 },
        { id: 'assistant', label: 'Assistant', status: 'current' },
        { id: 'governance', label: 'Governance', status: 'pending', count: 0 },
        { id: 'verification', label: 'Verification', status: 'pending', count: 0 },
      ],
      patrolEvidenceCount: 1,
      patrolIssueEvidenceCount: 1,
      activeFindingCount: 1,
      pendingApprovalCount: 0,
      governedActionCount: 0,
      approvedDecisionCount: 0,
      rejectedDecisionCount: 0,
      verifiedOutcomeCount: 0,
      operationsLoopStarterCount: 0,
      assistantOperationsLoopStarterCount: 0,
      patrolOperationsLoopStarterCount: 0,
      patrolControlOperationsLoopStarterCount: 0,
      patrolControlCompletedOperationsLoopCount: 0,
      patrolControlResolvedOperationsLoopCount: 0,
      patrolControlValueState: 'not_started',
      patrolAutonomyOperationsLoopStarterCount: 0,
      patrolAutonomyCompletedOperationsLoopCount: 0,
      patrolAutonomyResolvedOperationsLoopCount: 0,
      patrolAutonomyValueState: 'not_started',
      proActivationOperationsLoopStarterCount: 0,
      proActivationCompletedOperationsLoopCount: 0,
      proActivationResolvedOperationsLoopCount: 0,
      proActivationValueProofState: 'not_started',
      mcpOperationsLoopStarterCount: 0,
      externalAgentReady: true,
      windowStart: '2026-06-01T00:00:00Z',
      generatedAt: '2026-06-20T00:00:00Z',
    } satisfies Awaited<ReturnType<typeof fetchAgentOperationsLoopStatus>>;
    vi.mocked(apiFetchJSON).mockResolvedValueOnce(status);

    await expect(fetchAgentOperationsLoopStatus()).resolves.toBe(status);

    expect(apiFetchJSON).toHaveBeenCalledWith(AGENT_OPERATIONS_LOOP_STATUS_PATH, {
      headers: { Accept: 'application/json' },
    });
  });

  it('exports the generated surface tool contract type for Assistant and MCP consumers', () => {
    const contract: AgentSurfaceToolContract = {
      surfaceId: 'pulse_assistant',
      surfaceLabel: 'Pulse Assistant',
      toolSource: 'assistant_registry',
      toolNames: ['pulse_query', 'pulse_question'],
      registryToolNames: ['pulse_query'],
      nativeToolNames: ['pulse_question'],
      affordances: {
        tools: true,
        interactiveQuestions: true,
      },
    };

    expect(contract.surfaceId).toBe('pulse_assistant');
    expect(contract.registryToolNames).toEqual(['pulse_query']);
    expect(contract.nativeToolNames).toEqual(['pulse_question']);
  });

  it('normalizes runtime surface tool contracts without local tool catalog drift', () => {
    const normalized = normalizeAgentSurfaceToolContract({
      surfaceId: ' pulse_assistant ',
      surfaceLabel: ' Pulse Assistant ',
      toolSource: ' assistant_registry ',
      toolNames: ['pulse_query', 'pulse_question', 'pulse_query', ' '],
      registryToolNames: ['pulse_query', 'pulse_query'],
      nativeToolNames: ['pulse_question'],
      capabilityNames: [' '],
    });

    expect(normalized).toEqual({
      surfaceId: 'pulse_assistant',
      surfaceLabel: 'Pulse Assistant',
      toolSource: 'assistant_registry',
      toolNames: ['pulse_query', 'pulse_question'],
      registryToolNames: ['pulse_query'],
      nativeToolNames: ['pulse_question'],
      capabilityNames: undefined,
      affordances: {
        tools: true,
        interactiveQuestions: true,
      },
    });
  });

  it('presents Assistant capability posture from the surface contract counts', () => {
    const presentation = getAgentSurfaceToolPosturePresentation({
      surfaceId: 'pulse_assistant',
      surfaceLabel: 'Pulse Assistant',
      toolSource: 'assistant_registry',
      toolNames: ['pulse_query', 'pulse_question'],
      registryToolNames: ['pulse_query'],
      nativeToolNames: ['pulse_question'],
    });

    expect(presentation).toMatchObject({
      label: '2 capabilities',
      tone: 'ready',
      toolCount: 2,
    });
    expect(presentation?.title).toContain('Pulse Assistant capability availability');
    expect(presentation?.title).toContain('Source: Assistant registry');
    expect(presentation?.detail).toContain('1 registry capability');
    expect(presentation?.detail).toContain('1 native capability');
    expect(presentation?.detail).toContain('Interactive questions');
  });

  it('uses manifest-owned external-adapter surface tool contracts before local fallback', () => {
    const surfaceContractWithCLI = {
      ...surfaceContract,
      operatorSurfaces: [
        ...surfaceContract.operatorSurfaces,
        {
          id: 'pulse_cli_agent',
          label: 'Pulse CLI Agent',
          description: 'Another external-agent adapter.',
          native: false,
          externalAdapter: true,
          affordances: {
            tools: true,
            capabilityMetadata: true,
          },
        },
      ],
    };
    const manifest = {
      version: 'v1',
      surfaceContract: surfaceContractWithCLI,
      surfaceToolContracts: [
        {
          surfaceId: 'pulse_mcp',
          surfaceLabel: 'Pulse MCP',
          toolSource: 'capability_manifest',
          toolNames: ['get_fleet_context'],
          capabilityNames: ['get_fleet_context'],
          affordances: {
            tools: true,
            resources: true,
            prompts: true,
            capabilityMetadata: true,
          },
        },
        {
          surfaceId: 'pulse_cli_agent',
          surfaceLabel: 'Pulse CLI Agent',
          toolSource: 'capability_manifest',
          toolNames: ['plan_action'],
          capabilityNames: ['plan_action'],
        },
      ],
      mcpAdapter,
      requiredScopes: [],
      categories: [],
      workflowPrompts: [],
      capabilities: [
        capability('get_fleet_context', 'context'),
        capability('plan_action', 'actions'),
      ],
    } satisfies AgentCapabilitiesManifest;

    const mcp = getAgentManifestSurfaceToolContract(manifest, AGENT_SURFACE_ID_PULSE_MCP);
    expect(mcp?.toolNames).toEqual(['get_fleet_context']);
    expect(mcp?.capabilityNames).toEqual(['get_fleet_context']);

    const contracts = getAgentManifestSurfaceToolContracts(manifest);
    expect(contracts.map((contract) => contract.surfaceId)).toEqual([
      'pulse_mcp',
      'pulse_cli_agent',
    ]);
    expect(contracts[1]?.toolNames).toEqual(['plan_action']);
  });

  it('does not infer external surface tools when stale payloads omit surface tool contracts', () => {
    const manifest = {
      version: 'v1',
      surfaceContract,
      surfaceToolContracts: [],
      mcpAdapter,
      requiredScopes: [],
      categories: [],
      workflowPrompts: [],
      capabilities: [
        capability('get_fleet_context', 'context'),
        {
          ...capability('subscribe_events', 'events'),
          description: 'Subscribe to Pulse Intelligence events.',
        },
        capability('plan_action', 'actions'),
      ],
    } satisfies AgentCapabilitiesManifest;

    const contract = getAgentManifestSurfaceToolContract(manifest, AGENT_SURFACE_ID_PULSE_MCP);

    expect(contract).toBeUndefined();
    expect(getAgentSurfaceToolPosturePresentation(contract)).toBeNull();
  });

  it('gates published external surface tool posture by the manifest surface affordance', () => {
    const disabledToolSurfaceContract = {
      ...surfaceContract,
      operatorSurfaces: surfaceContract.operatorSurfaces.map((surface) =>
        surface.id === 'pulse_mcp'
          ? {
              ...surface,
              affordances: {
                resources: true,
                prompts: true,
                capabilityMetadata: true,
              },
            }
          : surface,
      ),
    };
    const manifest = {
      version: 'v1',
      surfaceContract: disabledToolSurfaceContract,
      surfaceToolContracts: [
        {
          surfaceId: 'pulse_mcp',
          surfaceLabel: 'Pulse MCP',
          toolSource: 'capability_manifest',
          toolNames: ['get_fleet_context'],
          capabilityNames: ['get_fleet_context'],
          affordances: {
            tools: true,
            resources: true,
            prompts: true,
            capabilityMetadata: true,
          },
        },
      ],
      mcpAdapter,
      requiredScopes: [],
      categories: [],
      workflowPrompts: [],
      capabilities: [capability('get_fleet_context', 'context')],
    } satisfies AgentCapabilitiesManifest;

    const contract = getAgentManifestSurfaceToolContract(manifest, AGENT_SURFACE_ID_PULSE_MCP);

    expect(contract).toMatchObject({
      surfaceId: 'pulse_mcp',
      surfaceLabel: 'Pulse MCP',
      toolSource: 'capability_manifest',
      toolNames: [],
      capabilityNames: undefined,
      affordances: {
        resources: true,
        prompts: true,
        capabilityMetadata: true,
      },
    });

    const presentation = getAgentSurfaceToolPosturePresentation(contract);
    expect(presentation).toMatchObject({
      label: '0 capabilities',
      tone: 'empty',
      toolCount: 0,
    });
    expect(presentation?.detail).toContain('Resources');
    expect(presentation?.detail).not.toContain('published capability');
  });

  it('does not project native Assistant runtime tools from the static manifest', () => {
    const manifest = {
      version: 'v1',
      surfaceContract,
      surfaceToolContracts: [],
      mcpAdapter,
      requiredScopes: [],
      categories: [],
      workflowPrompts: [],
      capabilities: [capability('get_fleet_context', 'context')],
    } satisfies AgentCapabilitiesManifest;

    expect(getAgentManifestSurfaceToolContract(manifest, 'pulse_assistant')).toBeUndefined();
    expect(getAgentManifestSurfaceToolContract(manifest, 'missing_surface')).toBeUndefined();
  });

  it('uses manifest-owned category order and presentation copy', () => {
    const manifest = {
      version: 'v1',
      surfaceContract,
      surfaceToolContracts: [],
      mcpAdapter,
      requiredScopes: ['monitoring:read', 'ai:execute'],
      categories: [
        {
          id: 'context',
          label: 'Context (read-only)',
          description: 'Discovery and read-only situated reads. Agents start here.',
        },
        {
          id: 'action',
          label: 'Actions (governed plan/approval/execute)',
          description: 'Plan, approve, and execute capability invocations against a resource.',
        },
      ],
      capabilities: [
        capability('plan_action', 'action'),
        {
          ...capability('get_patrol_control_status', 'context'),
          description:
            'Return Patrol mode completed/resolved outcome status and compatibility aliases.',
        },
      ],
      workflowPrompts: [],
    } satisfies AgentCapabilitiesManifest;

    const sections = groupAgentCapabilitiesByManifestCategories(manifest);

    expect(sections.map((section) => section.id)).toEqual(['context', 'action']);
    expect(sections.map((section) => section.label)).toEqual([
      'Context (read-only)',
      'Actions (governed plan/approval/execute)',
    ]);
    expect(sections[0]?.description).toBe(
      'Discovery and read-only situated reads. Agents start here.',
    );
    expect(sections[0]?.entries.map((entry) => entry.name)).toEqual(['get_patrol_control_status']);
    expect(sections[0]?.entries[0]?.description).toBe(
      'Return Patrol mode completed/resolved outcome status and compatibility aliases.',
    );
    expect(sections[1]?.entries.map((entry) => entry.name)).toEqual(['plan_action']);
  });

  it('keeps older or extended manifests visible without a local category table', () => {
    const manifest = {
      version: 'v1',
      surfaceContract,
      surfaceToolContracts: [],
      mcpAdapter,
      requiredScopes: [],
      categories: [],
      workflowPrompts: [],
      capabilities: [
        capability('future_tool', 'future-category'),
        capability('uncategorized_tool', ' '),
      ],
    } satisfies AgentCapabilitiesManifest;

    const sections = groupAgentCapabilitiesByManifestCategories(manifest);

    expect(sections.map((section) => [section.id, section.label])).toEqual([
      ['future-category', 'future-category'],
      ['uncategorized', 'Uncategorized'],
    ]);
    expect(sections.flatMap((section) => section.entries.map((entry) => entry.name))).toEqual([
      'future_tool',
      'uncategorized_tool',
    ]);
  });

  it('projects the manifest-owned Pulse Intelligence surface contract for settings UI', () => {
    const manifest = {
      version: 'v1',
      surfaceContract,
      surfaceToolContracts: [],
      mcpAdapter,
      requiredScopes: [],
      categories: [],
      workflowPrompts: [],
      capabilities: [],
    } satisfies AgentCapabilitiesManifest;

    const entries = getAgentSurfaceContractEntries(manifest);

    expect(entries.map((entry) => entry.label)).toEqual([
      'Pulse Intelligence Core',
      'Pulse Patrol',
      'Pulse Assistant',
      'Pulse MCP',
    ]);
    expect(entries.map((entry) => entry.description)).toEqual([
      'Canonical context, governed actions, safety gates, approval state, action audit, and verification.',
      'Patrol is the first-party operations surface: it checks infrastructure, investigates issues, follows the chosen Patrol mode before acting, verifies outcomes, and records what happened.',
      'The contextual explanation, approval, and handoff surface for Patrol findings, governed actions, verification, and operator questions.',
      'The external-agent adapter.',
    ]);
    expect(entries[2]?.badges).toEqual(['Native surface', 'Actions', 'Interactive questions']);
    expect(entries[3]?.badges).toEqual([
      'External adapter',
      'Actions',
      'Resources',
      'Prompts',
      'Capability metadata',
    ]);
  });

  it('projects manifest-owned workflow prompts for Assistant and MCP consumers', () => {
    const manifest = {
      version: 'v1',
      surfaceContract,
      surfaceToolContracts: [],
      mcpAdapter,
      requiredScopes: [],
      categories: [],
      workflowPrompts: [
        {
          name: ' pulse_triage_fleet ',
          label: ' Triage fleet ',
          presentationKind: ' fleet ',
          description: ' Triage the fleet. ',
        },
        {
          name: 'pulse_investigate_resource',
          description: ' Investigate a resource. ',
          arguments: [
            {
              name: ' resourceId ',
              description: ' Canonical resource id. ',
              required: true,
            },
            { name: ' ', description: 'ignored' },
          ],
        },
        {
          name: ' ',
          description: 'ignored',
        },
      ],
      capabilities: [],
    } satisfies AgentCapabilitiesManifest;

    expect(getAgentWorkflowPrompts(manifest)).toEqual([
      {
        name: 'pulse_triage_fleet',
        label: 'Triage fleet',
        presentationKind: 'fleet',
        description: 'Triage the fleet.',
        arguments: [],
      },
      {
        name: 'pulse_investigate_resource',
        description: 'Investigate a resource.',
        arguments: [
          {
            name: 'resourceId',
            description: 'Canonical resource id.',
            required: true,
          },
        ],
      },
    ]);
  });

  it('derives Pulse MCP Patrol-control readiness from prompts and surface tools', () => {
    const patrolControlToolNames = [
      'get_patrol_control_status',
      'get_fleet_context',
      'get_resource_context',
      'list_findings',
      'plan_action',
      'decide_action',
      'execute_action',
      'resolve_finding',
    ];
    const manifest = {
      version: 'v1',
      surfaceContract,
      surfaceToolContracts: [
        {
          surfaceId: 'pulse_mcp',
          surfaceLabel: 'Pulse MCP',
          toolSource: 'capability_manifest',
          toolNames: patrolControlToolNames,
          capabilityNames: patrolControlToolNames,
          affordances: {
            tools: true,
            resources: true,
            prompts: true,
            capabilityMetadata: true,
          },
        },
      ],
      mcpAdapter,
      requiredScopes: [],
      categories: [],
      workflowPrompts: [{ name: 'pulse_operations_loop', label: 'Ask Patrol to handle an issue' }],
      capabilities: [],
    } satisfies AgentCapabilitiesManifest;

    expect(getAgentMCPOperationsLoopReadiness(manifest)).toEqual({
      available: true,
      hasAdapter: true,
      hasPrompt: true,
      hasSurfaceTools: true,
      missingCapabilities: [],
    });

    const baseMCPContract = manifest.surfaceToolContracts[0]!;
    const incomplete = {
      ...manifest,
      surfaceToolContracts: [
        {
          ...baseMCPContract,
          toolNames: patrolControlToolNames.filter((name) => name !== 'execute_action'),
          capabilityNames: patrolControlToolNames.filter((name) => name !== 'execute_action'),
        },
      ],
    } satisfies AgentCapabilitiesManifest;

    expect(getAgentMCPOperationsLoopReadiness(incomplete)).toMatchObject({
      available: false,
      hasAdapter: true,
      hasPrompt: true,
      hasSurfaceTools: false,
      missingCapabilities: ['execute_action'],
    });
  });

  it('projects unique stable error codes with their declaring capabilities', () => {
    const manifest = {
      version: 'v1',
      surfaceContract,
      surfaceToolContracts: [],
      mcpAdapter,
      requiredScopes: [],
      categories: [],
      workflowPrompts: [],
      capabilities: [
        {
          ...capability('get_resource_context', 'context'),
          errorCodes: ['resource_not_found', ''],
        },
        {
          ...capability('plan_action', 'actions'),
          errorCodes: [' resource_not_found ', 'action_execution_unavailable'],
        },
        {
          ...capability('execute_action', 'actions'),
          errorCodes: ['action_execution_unavailable'],
        },
      ],
    } satisfies AgentCapabilitiesManifest;

    expect(getAgentCapabilityErrorCodeSummaries(manifest)).toEqual([
      {
        code: 'resource_not_found',
        capabilityNames: ['get_resource_context', 'plan_action'],
      },
      {
        code: 'action_execution_unavailable',
        capabilityNames: ['plan_action', 'execute_action'],
      },
    ]);
  });

  it('omits empty surface-contract entries from older manifests', () => {
    const manifest = {
      version: 'v1',
      surfaceContract: {
        core: { id: '', label: '', description: '' },
        proactiveEngine: { id: '', label: '  ', description: '' },
        operatorSurfaces: [
          {
            id: 'pulse_mcp',
            label: 'Pulse MCP',
            description: '  ',
            native: false,
            externalAdapter: true,
          },
        ],
      },
      surfaceToolContracts: [],
      mcpAdapter,
      requiredScopes: [],
      categories: [],
      workflowPrompts: [],
      capabilities: [],
    } satisfies AgentCapabilitiesManifest;

    expect(getAgentSurfaceContractEntries(manifest)).toEqual([
      {
        id: 'pulse_mcp',
        label: 'Pulse MCP',
        description: undefined,
        badges: ['External adapter', 'Actions', 'Resources', 'Prompts', 'Capability metadata'],
      },
    ]);
  });

  it('projects manifest-owned MCP adapter setup for settings UI snippets', () => {
    const customAdapter = {
      ...mcpAdapter,
      serverName: 'pulse-prod',
      command: 'pulse-mcp-prod',
      baseUrlFlag: '--pulse-url',
      defaultBaseUrl: 'https://pulse.example.com',
      tokenEnv: 'PULSE_PROD_TOKEN',
      configFamilies: [
        {
          id: 'custom-opencode',
          label: 'Custom OpenCode',
          shape: 'opencode_mcp',
          clientLabels: ['OpenCode'],
        },
        {
          id: 'custom-mcp-servers',
          label: 'Custom MCP servers',
          shape: 'mcp_servers',
          clientLabels: ['Claude Desktop', 'Claude Code'],
        },
      ],
    };
    const manifest = {
      version: 'v1',
      surfaceContract,
      surfaceToolContracts: [],
      mcpAdapter: customAdapter,
      requiredScopes: [],
      categories: [],
      workflowPrompts: [],
      capabilities: [],
    } satisfies AgentCapabilitiesManifest;

    expect(normalizeAgentMCPAdapter(manifest.mcpAdapter)).toMatchObject({
      serverName: 'pulse-prod',
      command: 'pulse-mcp-prod',
      baseUrlFlag: '--pulse-url',
      tokenEnv: 'PULSE_PROD_TOKEN',
    });
    expect(getAgentMCPClientExamples(manifest)).toEqual([
      'OpenCode',
      'Claude Desktop',
      'Claude Code',
    ]);
    expect(getAgentMCPConfigFamilyByShape(manifest.mcpAdapter, 'mcp_servers')?.label).toBe(
      'Custom MCP servers',
    );

    expect(formatAgentOpenCodeMCPConfig(manifest.mcpAdapter, 'https://pulse.example.com')).toBe(
      JSON.stringify(
        {
          $schema: 'https://opencode.ai/config.json',
          mcp: {
            'pulse-prod': {
              type: 'local',
              command: ['pulse-mcp-prod', '--pulse-url', 'https://pulse.example.com'],
              enabled: true,
              environment: {
                PULSE_PROD_TOKEN: '<your-api-token>',
              },
            },
          },
        },
        null,
        2,
      ),
    );
    expect(formatAgentMCPServersConfig(manifest.mcpAdapter, 'https://pulse.example.com')).toBe(
      JSON.stringify(
        {
          mcpServers: {
            'pulse-prod': {
              command: 'pulse-mcp-prod',
              args: ['--pulse-url', 'https://pulse.example.com'],
              env: {
                PULSE_PROD_TOKEN: '<your-api-token>',
              },
            },
          },
        },
        null,
        2,
      ),
    );
  });

  it('uses generated backend manifest types for the frontend contract', () => {
    expect(generatedAgentCapabilitiesSource).toContain(
      '// Source: internal/agentcapabilities manifest structs.',
    );
    expect(generatedAgentCapabilitiesSource).toContain('export interface Manifest');
    expect(generatedAgentCapabilitiesSource).toContain('surfaceContract: SurfaceContract');
    expect(generatedAgentCapabilitiesSource).toContain('mcpAdapter: MCPAdapterContract');
    expect(generatedAgentCapabilitiesSource).toContain('workflowPrompts: PulseWorkflowPrompt[]');
    expect(generatedAgentCapabilitiesSource).toContain('export interface MCPAdapterContract');
    expect(generatedAgentCapabilitiesSource).toContain('export interface PulseWorkflowPrompt');
    expect(generatedAgentCapabilitiesSource).toContain('label?: string');
    expect(generatedAgentCapabilitiesSource).toContain('arguments?: PulseWorkflowPromptArgument[]');
    expect(generatedAgentCapabilitiesSource).toContain(
      'export interface SurfaceAffordanceContract',
    );
    expect(generatedAgentCapabilitiesSource).toContain('affordances?: SurfaceAffordanceContract');
    expect(generatedAgentCapabilitiesSource).toContain('configFamilies: MCPAdapterConfigFamily[]');
    expect(generatedAgentCapabilitiesSource).toContain(
      'operatorSurfaces: OperatorSurfaceContract[]',
    );
    expect(generatedAgentCapabilitiesSource).toContain('actionMode: AgentCapabilityActionMode');
    expect(generatedAgentCapabilitiesSource).toContain(
      'approvalPolicy: AgentCapabilityApprovalPolicy',
    );
    expect(generatedAgentCapabilitiesSource).toContain('inputSchema?: Record<string, unknown>');
    expect(generatedAgentCapabilitiesSource).toContain('export interface SurfaceToolContract');
    expect(generatedAgentCapabilitiesSource).toContain('surfaceId: string');
    expect(generatedAgentCapabilitiesSource).toContain('toolSource: string');
    expect(generatedAgentCapabilitiesSource).toContain('toolNames: string[]');
    expect(generatedAgentCapabilitiesSource).toContain('registryToolNames?: string[]');
    expect(generatedAgentCapabilitiesSource).toContain('capabilityNames?: string[]');
    expect(generatedAgentCapabilitiesSource).toContain('nativeToolNames?: string[]');
  });
});
