import { describe, expect, it } from 'vitest';
import type { Connection } from '@/api/connections';
import type {
  InfrastructureSystemMemberRow,
  InfrastructureSystemRow,
} from '../connectionsTableModel';
import { collectInfrastructureAgentUpdateTargets } from '../infrastructureAgentUpdateCommandsModel';

const connection = (overrides: Partial<Connection> = {}): Connection => ({
  id: 'pve:homelab',
  type: 'pve',
  name: 'homelab',
  address: 'https://pve.lab:8006',
  state: 'active',
  stateReason: '',
  enabled: true,
  surfaces: ['vms'],
  scope: { vms: true },
  lastSeen: new Date().toISOString(),
  lastError: null,
  source: 'manual',
  capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
  ...overrides,
});

const emptyFleetRow = {
  fleetSignals: [],
  fleetHighlights: [],
} satisfies Pick<InfrastructureSystemRow, 'fleetSignals' | 'fleetHighlights'>;

const emptyFleetMember = {
  fleetSignals: [],
  fleetHighlights: [],
} satisfies Pick<InfrastructureSystemMemberRow, 'fleetSignals' | 'fleetHighlights'>;

const row = (overrides: Partial<InfrastructureSystemRow> = {}): InfrastructureSystemRow => {
  const primary = connection();
  return {
    id: primary.id,
    ownerType: 'pve',
    name: 'homelab',
    subtitle: 'Cluster · 2 nodes',
    source: 'both',
    host: primary.address,
    coverageLabels: ['VMs', 'Host telemetry'],
    statusLabel: 'Active',
    statusClassName: 'bg-green-100 text-green-800',
    agentUpdateCount: 0,
    lastActivityText: '1m ago',
    ...emptyFleetRow,
    enabled: true,
    canEdit: true,
    canPause: true,
    canRemove: true,
    isAgent: false,
    isCluster: false,
    attachedConnections: [],
    members: [],
    connection: primary,
    ...overrides,
  };
};

describe('infrastructure agent update commands model', () => {
  it('collects stale attached agents once with row-scoped update flags', () => {
    const staleAgent = connection({
      id: 'agent:agent-delly',
      type: 'agent',
      name: 'delly-agent',
      address: 'delly',
      surfaces: ['host'],
      scope: { host: true },
      source: 'agent',
      agentVersion: '5.1.34',
      expectedAgentVersion: '6.0.0-rc.6',
      agentUpdateAvailable: true,
      agentIdentity: { hostname: 'delly', platform: 'linux' },
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const currentAgent = connection({
      id: 'agent:agent-minipc',
      type: 'agent',
      name: 'minipc',
      address: 'minipc',
      surfaces: ['host'],
      scope: { host: true },
      source: 'agent',
      agentVersion: '6.0.0-rc.6',
      expectedAgentVersion: '6.0.0-rc.6',
      agentUpdateAvailable: false,
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });

    const targets = collectInfrastructureAgentUpdateTargets([
      row({
        attachedConnections: [staleAgent, currentAgent],
        members: [
          {
            id: 'node-delly',
            name: 'delly',
            subtitle: 'Primary node',
            source: 'both',
            host: 'https://delly:8006',
            coverageLabels: ['Host telemetry'],
            statusLabel: 'Active',
            statusClassName: 'bg-green-100 text-green-800',
            lastActivityText: '1m ago',
            ...emptyFleetMember,
            primary: true,
            agentConnection: staleAgent,
          },
        ],
      }),
    ]);

    expect(targets).toHaveLength(1);
    expect(targets[0]).toMatchObject({
      key: 'agent:agent-delly',
      displayName: 'delly',
      contextLabel: 'homelab',
      currentVersion: '5.1.34',
      expectedVersion: '6.0.0-rc.6',
      installFlags: ['--enable-proxmox', '--proxmox-type pve'],
    });
  });

  it('uses the explicit agent update target when the connection has no target version', () => {
    const staleAgent = connection({
      id: 'agent:agent-delly',
      type: 'agent',
      name: 'delly',
      address: 'delly',
      surfaces: ['host'],
      scope: { host: true },
      source: 'agent',
      agentVersion: 'v6.0.0-rc.5',
      agentUpdateAvailable: false,
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });

    const targets = collectInfrastructureAgentUpdateTargets(
      [
        row({
          attachedConnections: [staleAgent],
        }),
      ],
      'v6.0.0-rc.6',
    );

    expect(targets).toHaveLength(1);
    expect(targets[0]).toMatchObject({
      key: 'agent:agent-delly',
      currentVersion: 'v6.0.0-rc.5',
      expectedVersion: 'v6.0.0-rc.6',
    });
  });

  it('does not infer stale targets without an agent update target', () => {
    const staleAgent = connection({
      id: 'agent:agent-delly',
      type: 'agent',
      name: 'delly',
      address: 'delly',
      surfaces: ['host'],
      scope: { host: true },
      source: 'agent',
      agentVersion: 'v6.0.0-rc.5',
      agentUpdateAvailable: false,
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });

    const targets = collectInfrastructureAgentUpdateTargets([
      row({
        attachedConnections: [staleAgent],
      }),
    ]);

    expect(targets).toEqual([]);
  });

  it('filters to scoped agent IDs when a platform notice links to specific hosts', () => {
    const staleDelly = connection({
      id: 'agent:agent-delly',
      type: 'agent',
      name: 'delly',
      address: 'delly',
      surfaces: ['host'],
      scope: { host: true },
      source: 'agent',
      agentVersion: 'v6.0.0-rc.5',
      agentUpdateAvailable: false,
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const stalePi = connection({
      id: 'agent:agent-pi',
      type: 'agent',
      name: 'pi',
      address: 'pi',
      surfaces: ['host'],
      scope: { host: true },
      source: 'agent',
      agentVersion: 'v6.0.0-rc.5',
      agentUpdateAvailable: false,
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });

    const targets = collectInfrastructureAgentUpdateTargets(
      [
        row({
          attachedConnections: [staleDelly, stalePi],
        }),
      ],
      'v6.0.0-rc.6',
      ['agent-pi'],
    );

    expect(targets).toHaveLength(1);
    expect(targets[0]?.key).toBe('agent:agent-pi');
  });
});
