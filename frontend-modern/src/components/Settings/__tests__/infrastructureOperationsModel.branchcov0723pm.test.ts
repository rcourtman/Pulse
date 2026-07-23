import { describe, expect, it } from 'vitest';
import type { AgentCapability } from '@/utils/agentCapabilityPresentation';
import type { ConnectedInfrastructureItem, ConnectedInfrastructureSurface } from '@/types/api';
import type { UnifiedAgentRow, UnifiedAgentSurface } from '../infrastructureOperationsModel';
import {
  createSurfaceScopedRow,
  getCapabilityManagementPath,
  getCapabilitySurfaceLabel,
  getPlatformConnectionsViewForCapability,
  getStopMonitoringScopeLabel,
  getStopMonitoringSurfaces,
  hasMachineInstallActions,
  isPlatformConnectionsCapability,
  rowFromConnectedInfrastructureItem,
} from '../infrastructureOperationsModel';

type SurfaceKey = Parameters<typeof createSurfaceScopedRow>[1];

const makeRow = (overrides: Partial<UnifiedAgentRow>): UnifiedAgentRow => ({
  rowKey: 'row-1',
  id: 'agent-1',
  name: 'node-a',
  capabilities: [],
  status: 'active',
  upgradePlatform: 'linux',
  scope: { label: 'Default', category: 'default' },
  installFlags: [],
  searchText: '',
  surfaces: [],
  ...overrides,
});

const surface = (
  label: string,
  kind: AgentCapability,
  overrides: Partial<UnifiedAgentSurface> = {},
): UnifiedAgentSurface => ({
  key: kind,
  kind,
  label,
  detail: '',
  ...overrides,
});

const connectedSurface = (
  kind: ConnectedInfrastructureSurface['kind'],
  overrides: Partial<ConnectedInfrastructureSurface> = {},
): ConnectedInfrastructureSurface => ({
  id: `surface-${kind}`,
  kind,
  label: `${kind} label`,
  ...overrides,
});

const makeItem = (
  overrides: Partial<ConnectedInfrastructureItem>,
): ConnectedInfrastructureItem => ({
  id: 'agent-1',
  name: 'node-a',
  status: 'active',
  surfaces: [],
  ...overrides,
});

describe('createSurfaceScopedRow', () => {
  // A row that carries every surface kind plus every action id / linked node
  // so each scope block can be distinguished by what it clears vs preserves.
  const fullRow = makeRow({
    rowKey: 'host-1',
    capabilities: ['agent', 'docker', 'kubernetes', 'proxmox', 'pbs', 'pmg', 'truenas'],
    agentActionId: 'agent-act',
    dockerActionId: 'docker-act',
    kubernetesActionId: 'k8s-act',
    linkedNodeId: 'node-99',
    surfaces: [
      surface('Host telemetry', 'agent'),
      surface('Docker runtime data', 'docker'),
      surface('Kubernetes cluster data', 'kubernetes'),
      surface('Proxmox data', 'proxmox'),
      surface('PBS data', 'pbs'),
      surface('PMG data', 'pmg'),
      surface('TrueNAS data', 'truenas'),
    ],
  });

  it('scopes to docker: clears agent + k8s action ids and the linked node, keeps docker', () => {
    const scoped = createSurfaceScopedRow(fullRow, 'docker');
    expect(scoped.rowKey).toBe('host-1-docker-surface');
    expect(scoped.capabilities).toEqual(['docker']);
    expect(scoped.agentActionId).toBeUndefined();
    expect(scoped.kubernetesActionId).toBeUndefined();
    expect(scoped.linkedNodeId).toBeUndefined();
    expect(scoped.dockerActionId).toBe('docker-act');
    expect(scoped.surfaces.map((s) => s.kind)).toEqual(['docker']);
  });

  it('scopes to kubernetes: clears agent + docker action ids and the linked node, keeps k8s', () => {
    const scoped = createSurfaceScopedRow(fullRow, 'kubernetes');
    expect(scoped.rowKey).toBe('host-1-kubernetes-surface');
    expect(scoped.capabilities).toEqual(['kubernetes']);
    expect(scoped.agentActionId).toBeUndefined();
    expect(scoped.dockerActionId).toBeUndefined();
    expect(scoped.linkedNodeId).toBeUndefined();
    expect(scoped.kubernetesActionId).toBe('k8s-act');
    expect(scoped.surfaces.map((s) => s.kind)).toEqual(['kubernetes']);
  });

  it('scopes to pbs: clears docker + k8s action ids but keeps the agent action id and node', () => {
    const scoped = createSurfaceScopedRow(fullRow, 'pbs');
    expect(scoped.rowKey).toBe('host-1-pbs-surface');
    expect(scoped.capabilities).toEqual(['pbs']);
    expect(scoped.dockerActionId).toBeUndefined();
    expect(scoped.kubernetesActionId).toBeUndefined();
    expect(scoped.agentActionId).toBe('agent-act');
    expect(scoped.linkedNodeId).toBe('node-99');
    expect(scoped.surfaces.map((s) => s.kind)).toEqual(['pbs']);
  });

  it('scopes to pmg: clears docker + k8s action ids but keeps the agent action id and node', () => {
    const scoped = createSurfaceScopedRow(fullRow, 'pmg');
    expect(scoped.rowKey).toBe('host-1-pmg-surface');
    expect(scoped.capabilities).toEqual(['pmg']);
    expect(scoped.dockerActionId).toBeUndefined();
    expect(scoped.kubernetesActionId).toBeUndefined();
    expect(scoped.agentActionId).toBe('agent-act');
    expect(scoped.linkedNodeId).toBe('node-99');
    expect(scoped.surfaces.map((s) => s.kind)).toEqual(['pmg']);
  });

  it('scopes to proxmox via the explicit surfaceKey', () => {
    const scoped = createSurfaceScopedRow(fullRow, 'proxmox');
    expect(scoped.rowKey).toBe('host-1-proxmox-surface');
    expect(scoped.capabilities).toEqual(['proxmox']);
    expect(scoped.dockerActionId).toBeUndefined();
    expect(scoped.kubernetesActionId).toBeUndefined();
    expect(scoped.surfaces.map((s) => s.kind)).toEqual(['proxmox']);
  });

  it('falls back to the proxmox block for an unrecognised surfaceKey', () => {
    // 'availability' is not a member of the surfaceKey union, so it misses
    // every `if` and lands in the default (proxmox) return.
    const scoped = createSurfaceScopedRow(fullRow, 'availability' as unknown as SurfaceKey);
    expect(scoped.rowKey).toBe('host-1-proxmox-surface');
    expect(scoped.capabilities).toEqual(['proxmox']);
    expect(scoped.surfaces.map((s) => s.kind)).toEqual(['proxmox']);
  });

  it('scopes to agent with a host-only row: capabilities collapse to just ["agent"]', () => {
    const scoped = createSurfaceScopedRow(
      makeRow({
        capabilities: ['agent'],
        surfaces: [surface('Host telemetry', 'agent')],
      }),
      'agent',
    );
    expect(scoped.rowKey).toBe('row-1-agent-surface');
    expect(scoped.capabilities).toEqual(['agent']);
    expect(scoped.surfaces.map((s) => s.kind)).toEqual(['agent']);
  });

  it('scopes to agent with docker present: appends docker to the host-managed set', () => {
    const scoped = createSurfaceScopedRow(
      makeRow({
        capabilities: ['agent', 'docker'],
        surfaces: [
          surface('Host telemetry', 'agent'),
          surface('Docker runtime data', 'docker'),
          surface('PBS data', 'pbs'),
        ],
      }),
      'agent',
    );
    expect(scoped.capabilities).toEqual(['agent', 'docker']);
    // The agent block surfaces filter keeps agent/docker/kubernetes only, so
    // the pbs surface is dropped even though it exists on the source row.
    expect(scoped.surfaces.map((s) => s.kind)).toEqual(['agent', 'docker']);
  });

  it('scopes to agent with kubernetes present (no docker): appends kubernetes only', () => {
    const scoped = createSurfaceScopedRow(
      makeRow({
        capabilities: ['agent', 'kubernetes'],
        surfaces: [
          surface('Host telemetry', 'agent'),
          surface('Kubernetes cluster data', 'kubernetes'),
        ],
      }),
      'agent',
    );
    expect(scoped.capabilities).toEqual(['agent', 'kubernetes']);
    expect(scoped.surfaces.map((s) => s.kind)).toEqual(['agent', 'kubernetes']);
  });

  it('scopes to agent with docker + kubernetes: keeps all three host-managed kinds', () => {
    const scoped = createSurfaceScopedRow(fullRow, 'agent');
    expect(scoped.capabilities).toEqual(['agent', 'docker', 'kubernetes']);
    expect(scoped.dockerActionId).toBeUndefined();
    expect(scoped.kubernetesActionId).toBeUndefined();
    // Unlike the docker/kubernetes blocks, the agent block does NOT clear the
    // host action id or the linked node.
    expect(scoped.agentActionId).toBe('agent-act');
    expect(scoped.linkedNodeId).toBe('node-99');
    expect(scoped.surfaces.map((s) => s.kind)).toEqual(['agent', 'docker', 'kubernetes']);
  });
});

describe('getStopMonitoringSurfaces', () => {
  it('returns only the stop-monitoring surfaces when no agent surface is host-managed (false arm)', () => {
    // The docker surface has a stop-monitoring action but there is no agent
    // surface at all, so hostManagedStopApplies is false and the function
    // returns just the filtered stop-monitoring list instead of expanding.
    const row = makeRow({
      surfaces: [
        surface('Docker runtime data', 'docker', { action: 'stop-monitoring' }),
        surface('PBS data', 'pbs'),
      ],
    });
    expect(getStopMonitoringSurfaces(row).map((s) => s.kind)).toEqual(['docker']);
  });

  it('returns an empty list when no surface carries a stop-monitoring action', () => {
    const row = makeRow({
      surfaces: [
        surface('Host telemetry', 'agent', { action: 'allow-reconnect' }),
        surface('Docker runtime data', 'docker', { action: 'allow-reconnect' }),
      ],
    });
    expect(getStopMonitoringSurfaces(row)).toEqual([]);
  });
});

describe('getStopMonitoringScopeLabel', () => {
  it('falls back to the static copy when there are no stop-monitoring surfaces', () => {
    const row = makeRow({
      surfaces: [surface('Host telemetry', 'agent', { action: 'allow-reconnect' })],
    });
    expect(getStopMonitoringScopeLabel(row)).toBe('Reporting for this item');
  });
});

describe('hasMachineInstallActions', () => {
  it('returns false when both ids are undefined', () => {
    expect(hasMachineInstallActions(makeRow({}))).toBe(false);
  });

  it('returns false when both ids are whitespace-only (trim() zeroes them out)', () => {
    expect(hasMachineInstallActions(makeRow({ agentActionId: '   ', agentId: '\t' }))).toBe(false);
  });

  it('returns true when agentActionId is whitespace but agentId is present', () => {
    expect(hasMachineInstallActions(makeRow({ agentActionId: '  ', agentId: 'agent-1' }))).toBe(
      true,
    );
  });

  it('returns true when only agentActionId is present', () => {
    expect(hasMachineInstallActions(makeRow({ agentActionId: 'uninstall-1' }))).toBe(true);
  });
});

describe('getCapabilitySurfaceLabel', () => {
  it.each<[AgentCapability, string]>([
    ['agent', 'Host telemetry'],
    ['docker', 'Docker runtime data'],
    ['kubernetes', 'Kubernetes cluster data'],
    ['proxmox', 'Proxmox data'],
    ['pbs', 'PBS data'],
    ['pmg', 'PMG data'],
    ['truenas', 'TrueNAS data'],
  ])('returns the dedicated surface label for %s', (capability, label) => {
    expect(getCapabilitySurfaceLabel(capability)).toBe(label);
  });

  it('falls through to getAgentCapabilityLabel for an unrecognised capability, yielding undefined', () => {
    // The default arm delegates to getAgentCapabilityLabel, which itself has no
    // default case and returns undefined for a value outside its switch.
    expect(getCapabilitySurfaceLabel('foobar' as unknown as AgentCapability)).toBeUndefined();
  });
});

describe('getPlatformConnectionsViewForCapability', () => {
  it.each<[AgentCapability, 'proxmox' | 'truenas']>([
    ['proxmox', 'proxmox'],
    ['pbs', 'proxmox'],
    ['pmg', 'proxmox'],
    ['truenas', 'truenas'],
  ])('routes %s to the %s connections view', (capability, view) => {
    expect(getPlatformConnectionsViewForCapability(capability)).toBe(view);
  });

  it.each<AgentCapability>(['agent', 'docker', 'kubernetes'])(
    'returns null for the host-managed capability %s (default arm)',
    (capability) => {
      expect(getPlatformConnectionsViewForCapability(capability)).toBeNull();
    },
  );
});

describe('getCapabilityManagementPath', () => {
  it.each<AgentCapability>(['agent', 'docker', 'kubernetes'])(
    'returns null for non-platform host capabilities (%s)',
    (capability) => {
      expect(getCapabilityManagementPath(capability)).toBeNull();
    },
  );
});

describe('isPlatformConnectionsCapability', () => {
  it.each<AgentCapability>(['docker', 'kubernetes', 'agent'])(
    'returns false for the host capability %s',
    (capability) => {
      expect(isPlatformConnectionsCapability(capability)).toBe(false);
    },
  );

  it.each<AgentCapability>(['proxmox', 'pbs', 'pmg', 'truenas'])(
    'returns true for the platform capability %s',
    (capability) => {
      expect(isPlatformConnectionsCapability(capability)).toBe(true);
    },
  );
});

describe('installFlagsForCapabilities (via rowFromConnectedInfrastructureItem)', () => {
  const scope = { label: 'Default', category: 'default' as const };

  it('enables Docker without silently disabling the host for a docker-only item', () => {
    const row = rowFromConnectedInfrastructureItem(
      makeItem({ surfaces: [connectedSurface('docker')] }),
      scope,
    );
    expect(row.installFlags).toEqual(['--enable-docker']);
  });

  it('emits only --enable-kubernetes for a kubernetes-only item', () => {
    const row = rowFromConnectedInfrastructureItem(
      makeItem({ surfaces: [connectedSurface('kubernetes')] }),
      scope,
    );
    expect(row.installFlags).toEqual(['--enable-kubernetes']);
  });

  it('emits the pve proxmox type for a proxmox-only item', () => {
    const row = rowFromConnectedInfrastructureItem(
      makeItem({ surfaces: [connectedSurface('proxmox')] }),
      scope,
    );
    expect(row.installFlags).toEqual(['--enable-proxmox', '--proxmox-type pve']);
  });

  it('prefers proxmox-pve over pbs when both capabilities are present (else-if precedence)', () => {
    // installFlagsForCapabilities uses `else if (capabilities.includes('pbs'))`,
    // so a proxmox+pbs item pins to pve and never emits the pbs type.
    const row = rowFromConnectedInfrastructureItem(
      makeItem({
        surfaces: [connectedSurface('proxmox'), connectedSurface('pbs')],
      }),
      scope,
    );
    expect(row.installFlags).toEqual(['--enable-proxmox', '--proxmox-type pve']);
  });

  it('emits no flags for an agent-only item', () => {
    const row = rowFromConnectedInfrastructureItem(
      makeItem({ surfaces: [connectedSurface('agent')] }),
      scope,
    );
    expect(row.installFlags).toEqual([]);
  });
});

describe('rowFromConnectedInfrastructureItem', () => {
  const scope = { label: 'Default', category: 'default' as const };

  describe('name fallback chain (name || displayName || hostname || id)', () => {
    it('falls back to displayName when name is empty', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({ name: '', displayName: 'Display Name' }),
        scope,
      );
      expect(row.name).toBe('Display Name');
    });

    it('falls back to hostname when name and displayName are empty', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({ name: '', displayName: '', hostname: 'host.internal' }),
        scope,
      );
      expect(row.name).toBe('host.internal');
    });

    it('falls back to the id when name, displayName and hostname are all empty', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({ id: 'fallback-id', name: '', displayName: '', hostname: '' }),
        scope,
      );
      expect(row.name).toBe('fallback-id');
    });
  });

  describe('rowKey derivation', () => {
    it('prefixes removed-docker for an ignored item whose first surface is docker', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({
          id: 'd-1',
          status: 'ignored',
          surfaces: [connectedSurface('docker')],
        }),
        scope,
      );
      expect(row.rowKey).toBe('removed-docker-d-1');
    });

    it('prefixes removed-k8s for an ignored item whose first surface is kubernetes', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({
          id: 'k-1',
          status: 'ignored',
          surfaces: [connectedSurface('kubernetes')],
        }),
        scope,
      );
      expect(row.rowKey).toBe('removed-k8s-k-1');
    });

    it('prefixes removed-host for an ignored item whose first surface is agent', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({
          id: 'h-1',
          status: 'ignored',
          surfaces: [connectedSurface('agent')],
        }),
        scope,
      );
      expect(row.rowKey).toBe('removed-host-h-1');
    });

    it('prefixes removed-host for an ignored item with no surfaces', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({ id: 'h-2', status: 'ignored', surfaces: [] }),
        scope,
      );
      expect(row.rowKey).toBe('removed-host-h-2');
    });

    it('uses the kubernetes controlId for a kubernetes-only active item', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({
          id: 'k-2',
          surfaces: [connectedSurface('kubernetes', { controlId: 'k8s-ctrl' })],
        }),
        scope,
      );
      expect(row.rowKey).toBe('k8s-k8s-ctrl');
    });

    it('falls back to the item id when a kubernetes-only active item has no controlId', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({
          id: 'k-3',
          surfaces: [connectedSurface('kubernetes', { controlId: undefined })],
        }),
        scope,
      );
      expect(row.rowKey).toBe('k8s-k-3');
    });
  });

  describe('status mapping', () => {
    it('maps an ignored item status to the removed unified status', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({ status: 'ignored', surfaces: [connectedSurface('agent')] }),
        scope,
      );
      expect(row.status).toBe('removed');
    });

    it('keeps an active item status as active', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({ status: 'active', surfaces: [connectedSurface('agent')] }),
        scope,
      );
      expect(row.status).toBe('active');
    });
  });

  describe('upgradePlatform fallback', () => {
    it('defaults to linux when upgradePlatform is absent', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({ surfaces: [connectedSurface('agent')] }),
        scope,
      );
      expect(row.upgradePlatform).toBe('linux');
    });

    it('preserves an explicit freebsd upgrade platform', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({ upgradePlatform: 'freebsd', surfaces: [connectedSurface('agent')] }),
        scope,
      );
      expect(row.upgradePlatform).toBe('freebsd');
    });
  });

  describe('agentActionId / agentId fallbacks', () => {
    it('prefers uninstallAgentId over the agent surface controlId for agentActionId', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({
          uninstallAgentId: 'uninstall-1',
          surfaces: [connectedSurface('agent', { controlId: 'agent-ctrl' })],
        }),
        scope,
      );
      expect(row.agentActionId).toBe('uninstall-1');
    });

    it('uses the agent surface controlId when uninstallAgentId is absent', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({
          surfaces: [connectedSurface('agent', { controlId: 'agent-ctrl' })],
        }),
        scope,
      );
      expect(row.agentActionId).toBe('agent-ctrl');
    });

    it('prefers scopeAgentId over uninstallAgentId for agentId', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({
          scopeAgentId: 'scope-1',
          uninstallAgentId: 'uninstall-1',
          surfaces: [connectedSurface('agent')],
        }),
        scope,
      );
      expect(row.agentId).toBe('scope-1');
    });

    it('falls back to uninstallAgentId for agentId when scopeAgentId is absent', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({
          uninstallAgentId: 'uninstall-1',
          surfaces: [connectedSurface('agent')],
        }),
        scope,
      );
      expect(row.agentId).toBe('uninstall-1');
    });
  });

  describe('kubernetesInfo', () => {
    it('builds a kubernetesInfo object when a kubernetes surface is present', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({ surfaces: [connectedSurface('kubernetes')] }),
        scope,
      );
      expect(row.kubernetesInfo).toEqual({
        server: undefined,
        context: undefined,
        tokenName: undefined,
      });
    });

    it('leaves kubernetesInfo undefined for a host-only item with no kubernetes', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({ surfaces: [connectedSurface('agent')] }),
        scope,
      );
      expect(row.kubernetesInfo).toBeUndefined();
    });
  });

  describe('surfaceBreakdown label/detail fallbacks', () => {
    it('substitutes the capability surface label when the surface label is empty', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({ surfaces: [connectedSurface('docker', { label: '' })] }),
        scope,
      );
      expect(row.surfaces[0].label).toBe('Docker runtime data');
    });

    it('defaults an absent detail to an empty string', () => {
      const row = rowFromConnectedInfrastructureItem(
        makeItem({
          surfaces: [connectedSurface('agent', { label: 'Host', detail: undefined })],
        }),
        scope,
      );
      expect(row.surfaces[0].detail).toBe('');
    });

    it('routes an unrecognised surface kind through the agent default of agentCapabilityFromSurfaceKind', () => {
      // agentCapabilityFromSurfaceKind has a default -> 'agent' arm that is
      // only reachable when surface.kind is not one of the known union
      // members. Cast a bogus kind to exercise it; the capability collapses to
      // 'agent'.
      const row = rowFromConnectedInfrastructureItem(
        makeItem({
          surfaces: [
            connectedSurface('agent', {
              id: 'bogus',
              kind: 'not-a-real-kind' as unknown as ConnectedInfrastructureSurface['kind'],
              label: 'Bogus',
            }),
          ],
        }),
        scope,
      );
      expect(row.capabilities).toEqual(['agent']);
      expect(row.surfaces[0].kind).toBe('agent');
    });
  });
});
