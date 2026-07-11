import { describe, expect, it } from 'vitest';
import {
  getAgentMCPAdapterConfigFamilies,
  getAgentSurfaceContractEntries,
  getAgentSurfaceToolPosturePresentation,
  groupAgentCapabilitiesByManifestCategories,
  normalizeAgentSurfaceToolContract,
  type AgentCapabilitiesManifest,
  type AgentCapability,
  type AgentMCPAdapterContract,
  type AgentOperatorSurfaceContract,
  type AgentSurfaceAffordanceContract,
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

const component = (overrides: Partial<AgentSurfaceContractComponent> = {}): AgentSurfaceContractComponent => ({
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

const manifest = (overrides: Partial<AgentCapabilitiesManifest> = {}): AgentCapabilitiesManifest => ({
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

const affordance = (key: keyof AgentSurfaceAffordanceContract): AgentSurfaceAffordanceContract => {
  const obj: AgentSurfaceAffordanceContract = {};
  obj[key] = true;
  return obj;
};

const DEFAULT_CONFIG_FAMILY_IDS = ['opencode', 'claude-style', 'custom-mcp'];

describe('agentCapabilities branch coverage', () => {
  describe('getAgentSurfaceToolPosturePresentation', () => {
    it('renders singular breakdown strings and capability_manifest source for one tool of each kind', () => {
      const presentation = getAgentSurfaceToolPosturePresentation({
        surfaceId: 'pulse_mcp',
        surfaceLabel: ' Pulse MCP ',
        toolSource: ' capability_manifest ',
        toolNames: ['get_context'],
        registryToolNames: ['get_context'],
        capabilityNames: ['get_context'],
        nativeToolNames: ['get_context'],
      });

      expect(presentation).toMatchObject({
        surfaceLabel: 'Pulse MCP',
        label: '1 capability',
        tone: 'ready',
        toolCount: 1,
      });
      expect(presentation?.detail).toBe(
        '1 registry capability, 1 published capability, 1 native capability, Actions, Resources, Prompts, Capability metadata',
      );
      expect(presentation?.title).toBe(
        'Pulse MCP capability availability. Source: Capability manifest. 1 registry capability, 1 published capability, 1 native capability, Actions, Resources, Prompts, Capability metadata',
      );
    });

    it('renders plural breakdown strings and falls back to surfaceId when surfaceLabel is absent', () => {
      const presentation = getAgentSurfaceToolPosturePresentation({
        surfaceId: 'pulse_assistant',
        toolSource: 'assistant_registry',
        toolNames: ['q1', 'q2'],
        registryToolNames: ['q1', 'q2'],
        capabilityNames: ['q1', 'q2'],
        nativeToolNames: ['q1', 'q2'],
      });

      expect(presentation).toMatchObject({
        surfaceLabel: 'pulse_assistant',
        label: '2 capabilities',
        tone: 'ready',
        toolCount: 2,
      });
      expect(presentation?.detail).toBe(
        '2 registry capabilities, 2 published capabilities, 2 native capabilities, Actions, Interactive questions',
      );
      expect(presentation?.title).toContain('Source: Assistant registry');
    });

    it('uses the default toolSource word splitter and omits detail when nothing is available', () => {
      const presentation = getAgentSurfaceToolPosturePresentation({
        surfaceId: 'standalone_cli',
        toolSource: 'my-cool_source',
        toolNames: [],
      });

      expect(presentation).toMatchObject({
        surfaceLabel: 'standalone_cli',
        label: '0 capabilities',
        tone: 'empty',
        toolCount: 0,
      });
      expect(presentation?.detail).toBeUndefined();
      expect(presentation?.title).toBe(
        'standalone_cli capability availability. Source: My Cool Source',
      );
    });

    it('returns null when the contract normalizes away', () => {
      expect(getAgentSurfaceToolPosturePresentation(undefined)).toBeNull();
      expect(
        getAgentSurfaceToolPosturePresentation({ surfaceId: '   ', toolSource: 'x', toolNames: [] }),
      ).toBeNull();
    });
  });

  describe('normalizedConfigFamilies (via getAgentMCPAdapterConfigFamilies)', () => {
    it('clones the default families when no adapter or families are supplied', () => {
      const families = getAgentMCPAdapterConfigFamilies(undefined);

      expect(families.map((family) => family.id)).toEqual(DEFAULT_CONFIG_FAMILY_IDS);
      // cloned arrays are fresh copies of the defaults (fileHints present on opencode)
      expect(families[0]?.fileHints).toEqual([
        'opencode.json',
        'opencode.jsonc',
        '~/.config/opencode/opencode.json',
      ]);
      // the custom-mcp default carries no fileHints
      expect(families[2]?.fileHints).toBeUndefined();
    });

    it('falls back to defaults for an empty families array', () => {
      expect(
        getAgentMCPAdapterConfigFamilies(adapter({ configFamilies: [] })).map((f) => f.id),
      ).toEqual(DEFAULT_CONFIG_FAMILY_IDS);
    });

    it('falls back to defaults when every supplied family is unidentifiable', () => {
      const families = getAgentMCPAdapterConfigFamilies({
        ...adapter(),
        configFamilies: [{ id: '   ', label: '   ', shape: '   ' }],
      });

      expect(families.map((f) => f.id)).toEqual(DEFAULT_CONFIG_FAMILY_IDS);
    });

    it('derives id from label and filters fileHints/clientLabels, dropping empty descriptions', () => {
      const families = getAgentMCPAdapterConfigFamilies({
        ...adapter(),
        configFamilies: [
          {
            id: '   ',
            label: ' My Client ',
            shape: 'my_shape',
            description: '   ',
            fileHints: [' hint/a ', '', '   '],
            clientLabels: [' My Client ', ''],
          },
        ],
      });

      expect(families).toEqual([
        {
          id: 'My Client',
          label: 'My Client',
          shape: 'my_shape',
          description: undefined,
          fileHints: ['hint/a'],
          clientLabels: ['My Client'],
        },
      ]);
    });

    it('derives id and label from shape when only shape is present and keeps undefined fileHints', () => {
      const families = getAgentMCPAdapterConfigFamilies({
        ...adapter(),
        configFamilies: [
          {
            id: '',
            label: '',
            shape: 'only_shape',
            description: 'kept description',
            clientLabels: ['x'],
          },
        ],
      });

      expect(families).toEqual([
        {
          id: 'only_shape',
          label: 'only_shape',
          shape: 'only_shape',
          description: 'kept description',
          fileHints: undefined,
          clientLabels: ['x'],
        },
      ]);
    });

    it('normalizes multiple mixed families alongside the defaults only when all are dropped', () => {
      const families = getAgentMCPAdapterConfigFamilies({
        ...adapter(),
        configFamilies: [
          { id: 'good', label: 'Good', shape: 'good_shape' },
          { id: '', label: '', shape: '' },
        ],
      });

      expect(families.map((f) => f.id)).toEqual(['good']);
      expect(families[0]?.description).toBeUndefined();
    });
  });

  describe('groupAgentCapabilitiesByManifestCategories', () => {
    it('returns an empty array for an undefined manifest', () => {
      expect(groupAgentCapabilitiesByManifestCategories(undefined)).toEqual([]);
    });

    it('sanitizes legacy capability descriptions including the empty case', () => {
      const sections = groupAgentCapabilitiesByManifestCategories(
        manifest({
          categories: [{ id: 'context', label: 'Context' }],
          capabilities: [
            capability({ name: 'a', description: 'Uses legacy Pro activation aliases here.' }),
            capability({ name: 'b', description: 'Pro activation completed/resolved proof.' }),
            capability({
              name: 'c',
              description: 'Patrol autonomy completed/resolved outcome status.',
            }),
            capability({
              name: 'd',
              description: 'Patrol autonomy completed/resolved outcome evidence.',
            }),
            capability({ name: 'e', description: '   ' }),
          ],
        }),
      );

      expect(sections[0]?.entries.map((entry) => entry.description)).toEqual([
        'Uses compatibility aliases here.',
        'Patrol mode completed/resolved outcome status.',
        'Patrol mode completed/resolved outcome status.',
        'Patrol mode outcome evidence.',
        '',
      ]);
    });

    it('uses fallback labels, drops empty descriptions, and skips categories with no entries', () => {
      const sections = groupAgentCapabilitiesByManifestCategories(
        manifest({
          categories: [
            { id: 'context', label: '   ' },
            { id: 'orphaned', label: 'Orphaned', description: '   ' },
          ],
          capabilities: [capability({ name: 'cap', category: 'context' })],
        }),
      );

      expect(sections).toHaveLength(1);
      expect(sections[0]?.id).toBe('context');
      expect(sections[0]?.label).toBe('context');
      expect(sections[0]?.description).toBeUndefined();
      expect(sections[0]?.entries.map((entry) => entry.name)).toEqual(['cap']);
    });
  });

  describe('surfaceAffordancesDeclared (via normalizeAgentSurfaceToolContract)', () => {
    const declaredKeys: Array<keyof AgentSurfaceAffordanceContract> = [
      'tools',
      'resources',
      'prompts',
      'capabilityMetadata',
      'interactiveQuestions',
    ];

    for (const key of declaredKeys) {
      it(`keeps affordances and gates tools when only ${key} is declared`, () => {
        const normalized = normalizeAgentSurfaceToolContract({
          surfaceId: 'standalone',
          toolSource: 'x',
          toolNames: ['t'],
          affordances: affordance(key),
        });

        expect(normalized?.affordances).toEqual(affordance(key));
        expect(normalized?.toolNames).toEqual(key === 'tools' ? ['t'] : []);
      });
    }

    it('falls back to default affordances for a known surface when none are declared', () => {
      const normalized = normalizeAgentSurfaceToolContract({
        surfaceId: 'pulse_mcp',
        toolSource: 'x',
        toolNames: ['t'],
      });

      expect(normalized?.affordances).toEqual({
        tools: true,
        resources: true,
        prompts: true,
        capabilityMetadata: true,
      });
    });

    it('yields undefined affordances for an unknown surface when none are declared', () => {
      const normalized = normalizeAgentSurfaceToolContract({
        surfaceId: 'unknown_surface',
        toolSource: 'x',
        toolNames: ['t'],
      });

      expect(normalized?.affordances).toBeUndefined();
      expect(normalized?.toolNames).toEqual([]);
    });

    it('treats an all-false affordances object as undeclared', () => {
      const normalized = normalizeAgentSurfaceToolContract({
        surfaceId: 'pulse_assistant',
        toolSource: 'x',
        toolNames: ['t'],
        affordances: { tools: false, resources: false },
      });

      expect(normalized?.affordances).toEqual({ tools: true, interactiveQuestions: true });
    });
  });

  describe('normalizeAgentSurfaceToolContract', () => {
    it('returns undefined for an undefined contract', () => {
      expect(normalizeAgentSurfaceToolContract(undefined)).toBeUndefined();
    });

    it('returns undefined when surfaceId is empty or whitespace', () => {
      expect(
        normalizeAgentSurfaceToolContract({ surfaceId: '', toolSource: 'x', toolNames: [] }),
      ).toBeUndefined();
      expect(
        normalizeAgentSurfaceToolContract({ surfaceId: '   ', toolSource: 'x', toolNames: [] }),
      ).toBeUndefined();
    });

    it('defaults an empty toolSource to unknown and an empty surfaceLabel to undefined', () => {
      const normalized = normalizeAgentSurfaceToolContract({
        surfaceId: 's',
        surfaceLabel: '   ',
        toolSource: '   ',
        toolNames: [],
      });

      expect(normalized?.toolSource).toBe('unknown');
      expect(normalized?.surfaceLabel).toBeUndefined();
    });

    it('clears tool lists and returns undefined optionals when tools are disabled', () => {
      const normalized = normalizeAgentSurfaceToolContract({
        surfaceId: 'unknown_x',
        toolSource: 'x',
        toolNames: ['a', 'b'],
        registryToolNames: ['a'],
        capabilityNames: ['c'],
        nativeToolNames: ['n'],
      });

      expect(normalized?.toolNames).toEqual([]);
      expect(normalized?.registryToolNames).toBeUndefined();
      expect(normalized?.capabilityNames).toBeUndefined();
      expect(normalized?.nativeToolNames).toBeUndefined();
    });

    it('dedupes and trims all tool lists when tools are enabled', () => {
      const normalized = normalizeAgentSurfaceToolContract({
        surfaceId: 'pulse_mcp',
        toolSource: 'x',
        toolNames: [' a ', 'a', ' b ', ''],
        registryToolNames: ['a', 'a', '   '],
        capabilityNames: ['c1', 'c1', 'c2'],
        nativeToolNames: ['n1'],
      });

      expect(normalized?.toolNames).toEqual(['a', 'b']);
      expect(normalized?.registryToolNames).toEqual(['a']);
      expect(normalized?.capabilityNames).toEqual(['c1', 'c2']);
      expect(normalized?.nativeToolNames).toEqual(['n1']);
    });

    it('returns undefined optionals for explicitly empty lists when tools are enabled', () => {
      const normalized = normalizeAgentSurfaceToolContract({
        surfaceId: 'pulse_mcp',
        toolSource: 'x',
        toolNames: [],
        registryToolNames: [],
        capabilityNames: [],
        nativeToolNames: [],
      });

      expect(normalized?.registryToolNames).toBeUndefined();
      expect(normalized?.capabilityNames).toBeUndefined();
      expect(normalized?.nativeToolNames).toBeUndefined();
    });
  });

  describe('trimSurfaceContractComponent (via getAgentSurfaceContractEntries)', () => {
    it('returns an empty array for an undefined manifest', () => {
      expect(getAgentSurfaceContractEntries(undefined)).toEqual([]);
    });

    it('returns an empty array when surfaceContract is missing', () => {
      const malformed = { version: 'v1' } as unknown as AgentCapabilitiesManifest;
      expect(getAgentSurfaceContractEntries(malformed)).toEqual([]);
    });

    it('omits empty-label components/surfaces, falls id back to label, and drops empty descriptions', () => {
      const entries = getAgentSurfaceContractEntries(
        manifest({
          surfaceContract: {
            core: { id: '', label: 'Core', description: '   ' },
            proactiveEngine: { id: 'p', label: '   ', description: 'kept' },
            operatorSurfaces: [
              operatorSurface({ id: '', label: 'Native Surface', description: '  ', native: true }),
              operatorSurface({ id: 'ext', label: '   ', description: 'x', externalAdapter: true }),
            ],
          },
        }),
      );

      expect(entries).toEqual([
        {
          id: 'Core',
          label: 'Core',
          description: undefined,
          badges: [],
        },
        {
          id: 'Native Surface',
          label: 'Native Surface',
          description: undefined,
          badges: ['Native surface'],
        },
      ]);
    });

    it('projects default affordance badges for a known native surface', () => {
      const entries = getAgentSurfaceContractEntries(
        manifest({
          surfaceContract: {
            core: component({ id: 'c', label: 'C' }),
            proactiveEngine: component({ id: 'p', label: 'P' }),
            operatorSurfaces: [operatorSurface({ id: 'pulse_assistant', label: 'Assistant' })],
          },
        }),
      );

      expect(entries[2]?.badges).toEqual(['Native surface', 'Actions', 'Interactive questions']);
    });

    it('projects only declared affordance badges for an external adapter surface', () => {
      const entries = getAgentSurfaceContractEntries(
        manifest({
          surfaceContract: {
            core: component({ id: 'c', label: 'C' }),
            proactiveEngine: component({ id: 'p', label: 'P' }),
            operatorSurfaces: [
              operatorSurface({
                id: 'ext',
                label: 'Ext',
                native: false,
                externalAdapter: true,
                affordances: { resources: true },
              }),
            ],
          },
        }),
      );

      expect(entries[2]?.badges).toEqual(['External adapter', 'Resources']);
    });
  });
});
