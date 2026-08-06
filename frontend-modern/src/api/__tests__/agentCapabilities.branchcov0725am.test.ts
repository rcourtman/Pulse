import { describe, expect, it } from 'vitest';
import {
  AGENT_SURFACE_ID_PULSE_MCP,
  getAgentCapabilityErrorCodeSummaries,
  getAgentMCPClientExamples,
  getAgentMCPConfigFamilyByShape,
  getAgentMCPOperationsLoopReadiness,
  getAgentManifestSurfaceToolContract,
  getAgentManifestSurfaceToolContracts,
  getAgentSurfaceContractEntries,
  getAgentWorkflowPrompts,
  groupAgentCapabilitiesByManifestCategories,
  type AgentCapabilitiesManifest,
  type AgentCapability,
  type AgentCapabilityCategory,
  type AgentMCPAdapterContract,
  type AgentOperatorSurfaceContract,
  type AgentSurfaceContractComponent,
} from '../agentCapabilities';

const capability = (overrides: Partial<AgentCapability> = {}): AgentCapability => ({
  name: 'cap',
  category: 'context',
  description: 'desc',
  method: 'GET',
  path: '/api/cap',
  scope: 'monitoring:read',
  actionMode: 'read',
  approvalPolicy: 'scope_only',
  ...overrides,
});

const component = (
  overrides: Partial<AgentSurfaceContractComponent> = {},
): AgentSurfaceContractComponent => ({
  id: 'component_id',
  label: 'Component',
  description: 'component description',
  ...overrides,
});

const operatorSurface = (
  overrides: Partial<AgentOperatorSurfaceContract> = {},
): AgentOperatorSurfaceContract => ({
  id: 'pulse_assistant',
  label: 'Pulse Assistant',
  description: 'assistant surface',
  native: true,
  externalAdapter: false,
  ...overrides,
});

const adapter = (overrides: Partial<AgentMCPAdapterContract> = {}): AgentMCPAdapterContract => ({
  serverName: 'pulse',
  command: 'pulse-mcp',
  baseUrlFlag: '--base-url',
  defaultBaseUrl: 'http://localhost:7655',
  tokenEnv: 'PULSE_API_TOKEN',
  configFamilies: [],
  ...overrides,
});

const manifest = (
  overrides: Partial<AgentCapabilitiesManifest> = {},
): AgentCapabilitiesManifest => ({
  version: 'v1',
  surfaceContract: {
    core: component({ id: 'core', label: 'Core' }),
    proactiveEngine: component({ id: 'patrol', label: 'Patrol' }),
    operatorSurfaces: [],
  },
  surfaceToolContracts: [],
  mcpAdapter: adapter(),
  requiredScopes: [],
  categories: [],
  workflowPrompts: [],
  capabilities: [],
  ...overrides,
});

describe('agentCapabilities branch coverage 0725am', () => {
  describe('getAgentMCPConfigFamilyByShape (line 330)', () => {
    it('returns undefined for a whitespace-only shape so the lookup is skipped entirely', () => {
      const adapterWithFamilies = adapter({
        configFamilies: [
          { id: 'opencode', label: 'OpenCode', shape: 'opencode_mcp' },
          { id: 'claude-style', label: 'Claude', shape: 'mcp_servers' },
        ],
      });

      expect(getAgentMCPConfigFamilyByShape(adapterWithFamilies, '   ')).toBeUndefined();
      // contrast: a real shape still resolves, proving the early return is shape-gated
      expect(getAgentMCPConfigFamilyByShape(adapterWithFamilies, 'mcp_servers')?.id).toBe(
        'claude-style',
      );
    });
  });

  describe('getAgentMCPClientExamples (line 344)', () => {
    it('falls back to the family label when a family has no usable clientLabels', () => {
      const manifestWithoutClientLabels = manifest({
        mcpAdapter: adapter({
          configFamilies: [
            { id: 'solo', label: 'Solo Client', shape: 'solo_shape' },
            { id: 'all-blank', label: 'All Blank', shape: 'blank_shape', clientLabels: ['  ', ''] },
          ],
        }),
      });

      // both families fall through to the else-branch (line 344): one has no clientLabels at all,
      // the other's normalize away to an empty list
      expect(getAgentMCPClientExamples(manifestWithoutClientLabels)).toEqual([
        'Solo Client',
        'All Blank',
      ]);
    });
  });

  describe('groupAgentCapabilitiesByManifestCategories (line 442)', () => {
    it('treats a missing categories array as an empty iteration and still groups capabilities', () => {
      const manifestWithoutCategories = manifest({
        capabilities: [capability({ name: 'orphaned_cap', category: 'orphaned' })],
        categories: undefined as unknown as AgentCapabilityCategory[],
      });

      const sections = groupAgentCapabilitiesByManifestCategories(manifestWithoutCategories);

      // no manifest categories were iterated, so the capability surfaces via the unknown-category
      // fallback path with its raw category id as the section label
      expect(sections.map((section) => [section.id, section.label])).toEqual([
        ['orphaned', 'orphaned'],
      ]);
      expect(sections[0]?.entries.map((entry) => entry.name)).toEqual(['orphaned_cap']);
    });
  });

  describe('normalizeSurfaceAffordances via getAgentSurfaceContractEntries (line 508)', () => {
    it('returns an empty affordance object for an unknown but non-empty surface id', () => {
      const entries = getAgentSurfaceContractEntries(
        manifest({
          surfaceContract: {
            core: component({ id: 'c', label: 'C' }),
            proactiveEngine: component({ id: 'p', label: 'P' }),
            operatorSurfaces: [
              operatorSurface({
                id: 'ext_unknown',
                label: 'Ext Unknown',
                native: false,
                externalAdapter: true,
              }),
              operatorSurface({
                id: 'pulse_mcp',
                label: 'Known MCP',
                native: false,
                externalAdapter: true,
              }),
            ],
          },
        }),
      );

      // the unknown id takes the `?? {}` arm: no affordance badges are projected
      expect(entries[2]?.badges).toEqual(['External adapter']);
      // contrast: a known id (pulse_mcp) projects the full default affordance badge set
      expect(entries[3]?.badges).toEqual([
        'External adapter',
        'Actions',
        'Resources',
        'Prompts',
        'Capability metadata',
      ]);
    });
  });

  describe('getAgentWorkflowPrompts (line 551)', () => {
    it('returns an empty array when the manifest itself is missing', () => {
      expect(getAgentWorkflowPrompts(undefined)).toEqual([]);
    });

    it('treats a missing workflowPrompts array as an empty iteration', () => {
      const manifestWithoutPrompts = {
        ...manifest(),
        workflowPrompts: undefined as unknown as AgentCapabilitiesManifest['workflowPrompts'],
      };

      expect(getAgentWorkflowPrompts(manifestWithoutPrompts)).toEqual([]);
    });
  });

  describe('getAgentMCPOperationsLoopReadiness (line 596)', () => {
    it('treats a missing surface tool contract as an empty tool-name set and reports every capability missing', () => {
      const readiness = getAgentMCPOperationsLoopReadiness(
        manifest({
          workflowPrompts: [{ name: 'pulse_operations_loop', label: 'Operations loop' }],
        }),
      );

      // contract is undefined -> `contract?.toolNames ?? []` takes the ?? arm; every patrol-control
      // capability name ends up in missingCapabilities and hasSurfaceTools is false.
      expect(readiness.hasSurfaceTools).toBe(false);
      expect(readiness.available).toBe(false);
      expect(readiness.missingCapabilities).toEqual([
        'get_patrol_control_status',
        'get_fleet_context',
        'get_resource_context',
        'list_findings',
        'plan_action',
        'decide_action',
        'execute_action',
        'resolve_finding',
      ]);
    });
  });

  describe('getAgentManifestSurfaceToolContract', () => {
    it('returns undefined when the manifest is missing (line 642)', () => {
      expect(
        getAgentManifestSurfaceToolContract(undefined, AGENT_SURFACE_ID_PULSE_MCP),
      ).toBeUndefined();
    });

    it('returns undefined when the surface id is whitespace (line 645)', () => {
      const manifestWithAdapterSurface = manifest({
        surfaceContract: {
          core: component({ id: 'c', label: 'C' }),
          proactiveEngine: component({ id: 'p', label: 'P' }),
          operatorSurfaces: [
            operatorSurface({
              id: AGENT_SURFACE_ID_PULSE_MCP,
              label: 'Pulse MCP',
              native: false,
              externalAdapter: true,
            }),
          ],
        },
        surfaceToolContracts: [
          {
            surfaceId: AGENT_SURFACE_ID_PULSE_MCP,
            surfaceLabel: 'Pulse MCP',
            toolSource: 'capability_manifest',
            toolNames: ['get_fleet_context'],
          },
        ],
      });

      expect(
        getAgentManifestSurfaceToolContract(manifestWithAdapterSurface, '   '),
      ).toBeUndefined();
    });

    it('falls back to the normalized surface id when the surface label is blank (line 655)', () => {
      const manifestWithBlankLabel = manifest({
        surfaceContract: {
          core: component({ id: 'c', label: 'C' }),
          proactiveEngine: component({ id: 'p', label: 'P' }),
          operatorSurfaces: [
            operatorSurface({
              id: AGENT_SURFACE_ID_PULSE_MCP,
              label: '   ',
              native: false,
              externalAdapter: true,
              affordances: { tools: true },
            }),
          ],
        },
        surfaceToolContracts: [
          {
            surfaceId: AGENT_SURFACE_ID_PULSE_MCP,
            toolSource: 'capability_manifest',
            toolNames: ['get_fleet_context'],
            capabilityNames: ['get_fleet_context'],
          },
        ],
      });

      const contract = getAgentManifestSurfaceToolContract(
        manifestWithBlankLabel,
        AGENT_SURFACE_ID_PULSE_MCP,
      );

      // surface.label?.trim() is empty -> falls back to normalizedSurfaceId ('pulse_mcp')
      expect(contract?.surfaceLabel).toBe(AGENT_SURFACE_ID_PULSE_MCP);
      expect(contract?.toolNames).toEqual(['get_fleet_context']);
    });
  });

  describe('getAgentManifestSurfaceToolContracts (line 664)', () => {
    it('treats a missing manifest as an empty surface iteration', () => {
      expect(getAgentManifestSurfaceToolContracts(undefined)).toEqual([]);
    });
  });

  describe('getAgentCapabilityErrorCodeSummaries', () => {
    it('returns an empty array when the manifest is missing (line 714)', () => {
      expect(getAgentCapabilityErrorCodeSummaries(undefined)).toEqual([]);
    });

    it('skips capabilities whose name trims to empty (line 719)', () => {
      const summaries = getAgentCapabilityErrorCodeSummaries(
        manifest({
          capabilities: [
            capability({ name: '   ', errorCodes: ['should_never_appear'] }),
            capability({ name: 'real_cap', errorCodes: ['real_code'] }),
          ],
        }),
      );

      // the whitespace-name capability is skipped at line 719, so its code never lands in a summary
      expect(summaries).toEqual([{ code: 'real_code', capabilityNames: ['real_cap'] }]);
      expect(summaries.some((summary) => summary.code === 'should_never_appear')).toBe(false);
    });

    it('treats a capability without an errorCodes list as an empty iteration (line 721)', () => {
      const { errorCodes: _omitted, ...capabilityWithoutErrorCodes } = capability({
        name: 'no_codes',
      });
      void _omitted;

      const summaries = getAgentCapabilityErrorCodeSummaries(
        manifest({
          capabilities: [
            capabilityWithoutErrorCodes as AgentCapability,
            capability({ name: 'with_codes', errorCodes: ['declared_code'] }),
          ],
        }),
      );

      // the capability without errorCodes contributes nothing (`capability.errorCodes ?? []` arm)
      // while the sibling capability still declares its code
      expect(summaries).toEqual([{ code: 'declared_code', capabilityNames: ['with_codes'] }]);
    });
  });
});
