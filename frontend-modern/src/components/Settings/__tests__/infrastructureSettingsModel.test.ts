import { describe, expect, it } from 'vitest';
import { filterRepresentedDiscoveredServers } from '../infrastructureSettingsModel';
import type { InfrastructureSystemRow } from '../connectionsTableModel';

describe('filterRepresentedDiscoveredServers', () => {
  it('removes a discovered platform candidate when the same platform member is already represented by host aliases', () => {
    const rows: InfrastructureSystemRow[] = [
      {
        id: 'pve:homelab',
        ownerType: 'pve',
        name: 'homelab',
        subtitle: 'Cluster · 1 node',
        source: 'api',
        host: undefined,
        coverageLabels: ['VMs', 'Containers', 'Storage', 'Backups'],
        statusLabel: 'Active',
        statusClassName: 'bg-green-100 text-green-800',
        agentUpdateCount: 0,
        lastActivityText: '4s ago',
        enabled: true,
        canEdit: true,
        canPause: true,
        canRemove: true,
        isAgent: false,
        isCluster: true,
        attachedConnections: [],
        members: [
          {
            id: 'node-pi',
            name: 'pi',
            subtitle: 'API contact',
            source: 'agent',
            host: 'https://pi:8006',
            hostAliases: ['pi', '192.168.0.2'],
            coverageLabels: ['Host telemetry'],
            statusLabel: 'Active',
            statusClassName: 'bg-green-100 text-green-800',
            lastActivityText: '4s ago',
            primary: true,
          },
        ],
        connection: {
          id: 'pve:homelab',
          type: 'pve',
          name: 'homelab',
          address: 'https://pi:8006',
          state: 'active',
          stateReason: '',
          enabled: true,
          surfaces: ['vms', 'containers', 'storage', 'backups'],
          scope: { vms: true, containers: true, storage: true, backups: true },
          lastSeen: '2026-04-23T12:00:00Z',
          lastError: null,
          source: 'manual',
          capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
        },
      },
    ];

    const filtered = filterRepresentedDiscoveredServers(
      [
        {
          ip: '192.168.0.2',
          port: 8006,
          type: 'pve',
          version: '8.2.2',
        },
      ],
      [],
      rows,
    );

    expect(filtered).toEqual([]);
  });

  it('keeps a discovered platform candidate when the matching host is only represented by an agent row', () => {
    const rows: InfrastructureSystemRow[] = [
      {
        id: 'agent:pi',
        ownerType: 'agent',
        name: 'pi',
        subtitle: 'via Pulse Agent',
        source: 'agent',
        host: undefined,
        coverageLabels: ['Host telemetry'],
        statusLabel: 'Active',
        statusClassName: 'bg-green-100 text-green-800',
        agentUpdateCount: 0,
        lastActivityText: '4s ago',
        enabled: true,
        canEdit: false,
        canPause: false,
        canRemove: true,
        isAgent: true,
        isCluster: false,
        attachedConnections: [],
        members: [],
        connection: {
          id: 'agent:pi',
          type: 'agent',
          name: 'pi',
          address: 'pi',
          state: 'active',
          stateReason: '',
          enabled: true,
          surfaces: ['host'],
          scope: { host: true },
          lastSeen: '2026-04-23T12:00:00Z',
          lastError: null,
          source: 'agent',
          capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
        },
      },
    ];

    const filtered = filterRepresentedDiscoveredServers(
      [
        {
          ip: '192.168.0.2',
          port: 8006,
          type: 'pve',
          version: '8.2.2',
          hostname: 'pi',
        },
      ],
      [],
      rows,
    );

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.ip).toBe('192.168.0.2');
  });

  it('removes a discovered platform candidate when the platform row carries an attached agent alias for the same host', () => {
    const rows: InfrastructureSystemRow[] = [
      {
        id: 'pve:pi',
        ownerType: 'pve',
        name: 'pi',
        subtitle: 'via platform API and Pulse Agent',
        source: 'both',
        host: 'https://pi:8006',
        coverageLabels: ['VMs', 'Containers', 'Storage', 'Backups', 'Host telemetry'],
        statusLabel: 'Active',
        statusClassName: 'bg-green-100 text-green-800',
        agentUpdateCount: 0,
        lastActivityText: '4s ago',
        enabled: true,
        canEdit: true,
        canPause: true,
        canRemove: true,
        isAgent: false,
        isCluster: false,
        attachedConnections: [
          {
            id: 'agent:pi',
            type: 'agent',
            name: 'pi',
            address: 'pi',
            hostAliases: ['pi', '192.168.0.2'],
            state: 'active',
            stateReason: '',
            enabled: true,
            surfaces: ['host'],
            scope: { host: true },
            lastSeen: '2026-04-23T12:00:00Z',
            lastError: null,
            source: 'agent',
            capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
          },
        ],
        members: [],
        connection: {
          id: 'pve:pi',
          type: 'pve',
          name: 'pi',
          address: 'https://pi:8006',
          state: 'active',
          stateReason: '',
          enabled: true,
          surfaces: ['vms', 'containers', 'storage', 'backups'],
          scope: { vms: true, containers: true, storage: true, backups: true },
          lastSeen: '2026-04-23T12:00:00Z',
          lastError: null,
          source: 'manual',
          capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
        },
      },
    ];

    const filtered = filterRepresentedDiscoveredServers(
      [
        {
          ip: '192.168.0.2',
          port: 8006,
          type: 'pve',
          version: '8.2.2',
        },
      ],
      [],
      rows,
    );

    expect(filtered).toEqual([]);
  });
});
