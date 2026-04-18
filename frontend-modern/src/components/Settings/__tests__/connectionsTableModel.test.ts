import { describe, expect, it } from 'vitest';
import type { NodeConfigWithStatus } from '@/types/nodes';
import type { TrueNASConnection } from '@/api/truenas';
import type { VMwareConnection } from '@/api/vmware';
import type { UnifiedAgentRow } from '../infrastructureOperationsModel';
import { buildConnectionRows } from '../connectionsTableModel';

const activeRow = (overrides: Partial<UnifiedAgentRow> = {}): UnifiedAgentRow =>
  ({
    rowKey: 'agent:tower',
    id: 'tower',
    name: 'tower',
    hostname: 'tower.local',
    capabilities: ['agent'],
    status: 'active',
    healthStatus: 'online',
    lastSeen: Date.now(),
    upgradePlatform: 'linux',
    scope: { label: 'Default', category: 'default' },
    installFlags: [],
    searchText: 'tower tower.local',
    surfaces: [
      {
        key: 'agent',
        kind: 'agent',
        label: 'Host telemetry',
        detail: 'Host telemetry',
        action: 'stop-monitoring',
        controlId: 'tower',
      },
    ],
    ...overrides,
  }) as UnifiedAgentRow;

const ignoredRow = (overrides: Partial<UnifiedAgentRow> = {}): UnifiedAgentRow =>
  ({
    ...activeRow(),
    rowKey: 'removed:tower',
    status: 'removed',
    removedAt: Date.now(),
    surfaces: [
      {
        key: 'agent',
        kind: 'agent',
        label: 'Host telemetry',
        detail: 'Host telemetry',
        action: 'allow-reconnect',
        controlId: 'tower',
      },
    ],
    ...overrides,
  }) as UnifiedAgentRow;

const pveNode = (overrides: Partial<NodeConfigWithStatus> = {}): NodeConfigWithStatus =>
  ({
    id: 'pve-1',
    name: 'pve-1',
    host: '10.0.0.1',
    user: 'root',
    type: 'pve',
    verifySSL: true,
    monitorVMs: true,
    monitorContainers: true,
    monitorStorage: true,
    monitorBackups: true,
    monitorPhysicalDisks: false,
    status: 'connected',
    ...overrides,
  }) as NodeConfigWithStatus;

const truenas = (overrides: Partial<TrueNASConnection> = {}): TrueNASConnection =>
  ({
    id: 'tn-1',
    name: 'Tower NAS',
    host: '10.0.0.20',
    useHttps: true,
    insecureSkipVerify: false,
    enabled: true,
    ...overrides,
  }) as TrueNASConnection;

const vmware = (overrides: Partial<VMwareConnection> = {}): VMwareConnection =>
  ({
    id: 'vm-1',
    name: 'vCenter',
    host: '10.0.0.30',
    insecureSkipVerify: false,
    enabled: true,
    ...overrides,
  }) as VMwareConnection;

describe('connectionsTableModel', () => {
  it('merges active reporting, ignored rows, and configured connections into one alpha-sorted ledger', () => {
    const rows = buildConnectionRows({
      activeRows: [activeRow({ name: 'tower' })],
      monitoringStoppedRows: [ignoredRow({ name: 'archive' })],
      pveNodes: [pveNode({ id: 'a', name: 'zeus' })],
      pbsNodes: [pveNode({ id: 'b', type: 'pbs', name: 'backup' })],
      pmgNodes: [],
      truenasConnections: [truenas({ id: 'c', name: 'mira' })],
      vmwareConnections: [vmware({ id: 'd', name: 'apex' })],
    });

    expect(rows.map((row) => row.name)).toEqual(['apex', 'archive', 'backup', 'mira', 'tower', 'zeus']);
    expect(rows.find((row) => row.name === 'archive')).toMatchObject({
      subtitle: 'Ignored by Pulse',
      manageLabel: 'Review ignored',
    });
    expect(rows.find((row) => row.name === 'tower')).toMatchObject({
      subtitle: 'Live reporting item',
      manageLabel: 'View details',
    });
    expect(rows.find((row) => row.name === 'zeus')).toMatchObject({
      subtitle: 'Configured platform connection',
      collectionLabel: 'API',
    });
  });

  it('drops configured rows that are already represented by the reporting projection', () => {
    const rows = buildConnectionRows({
      activeRows: [
        activeRow({
          name: 'tower',
          capabilities: ['truenas'],
          surfaces: [
            {
              key: 'truenas',
              kind: 'truenas',
              label: 'TrueNAS data',
              detail: 'TrueNAS data',
              idValue: '10.0.0.20',
            },
          ],
        }),
      ],
      monitoringStoppedRows: [],
      pveNodes: [],
      pbsNodes: [],
      pmgNodes: [],
      truenasConnections: [truenas()],
      vmwareConnections: [],
    });

    expect(rows).toHaveLength(1);
    expect(rows[0].name).toBe('tower');
  });

  it('can collapse to reporting-only rows for read-only sessions', () => {
    const rows = buildConnectionRows({
      activeRows: [activeRow()],
      monitoringStoppedRows: [ignoredRow()],
      pveNodes: [pveNode()],
      pbsNodes: [],
      pmgNodes: [],
      truenasConnections: [truenas()],
      vmwareConnections: [vmware()],
      includeConfigurationRows: false,
    });

    expect(rows).toHaveLength(2);
    expect(rows.every((row) => row.subtitle !== 'Configured platform connection')).toBe(true);
  });

  it('keeps guest-linked agents out of the top-level infrastructure ledger', () => {
    const rows = buildConnectionRows({
      activeRows: [
        activeRow({ name: 'tower' }),
        activeRow({
          rowKey: 'agent:guest-101',
          id: 'guest-101',
          name: 'debian-go',
          linkedVmId: '101',
        }),
      ],
      monitoringStoppedRows: [
        ignoredRow({
          rowKey: 'removed:guest-102',
          id: 'guest-102',
          name: 'archive-guest',
          linkedContainerId: '102',
        }),
      ],
      pveNodes: [],
      pbsNodes: [],
      pmgNodes: [],
      truenasConnections: [],
      vmwareConnections: [],
    });

    expect(rows.map((row) => row.name)).toEqual(['tower']);
  });

  it('keeps saved VMware connections visible even though the reporting projection does not own them yet', () => {
    const rows = buildConnectionRows({
      activeRows: [],
      monitoringStoppedRows: [],
      pveNodes: [],
      pbsNodes: [],
      pmgNodes: [],
      truenasConnections: [],
      vmwareConnections: [vmware({ name: 'lab-vcenter' })],
    });

    expect(rows).toEqual([
      expect.objectContaining({
        name: 'lab-vcenter',
        coverageLabels: ['VMware data'],
        manage: { kind: 'vmware-connection', connectionId: 'vm-1' },
      }),
    ]);
  });
});
