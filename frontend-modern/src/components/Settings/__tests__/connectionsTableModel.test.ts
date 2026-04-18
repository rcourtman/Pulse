import { describe, expect, it } from 'vitest';
import type { UnifiedAgentRow } from '../infrastructureOperationsModel';
import { buildInfrastructureSystemRows } from '../connectionsTableModel';

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

describe('connectionsTableModel', () => {
  it('builds one alpha-sorted row per monitored top-level system', () => {
    const rows = buildInfrastructureSystemRows({
      activeRows: [activeRow({ name: 'tower' })],
      monitoringStoppedRows: [ignoredRow({ name: 'archive' })],
    });

    expect(rows.map((row) => row.name)).toEqual(['archive', 'tower']);
    expect(rows.find((row) => row.name === 'archive')).toMatchObject({
      subtitle: 'Ignored by Pulse',
      manageLabel: 'Review ignored',
    });
    expect(rows.find((row) => row.name === 'tower')).toMatchObject({
      subtitle: undefined,
      manageLabel: 'View details',
    });
  });

  it('keeps platform-collected systems as system rows instead of separate connection rows', () => {
    const rows = buildInfrastructureSystemRows({
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
    });

    expect(rows).toHaveLength(1);
    expect(rows[0].name).toBe('tower');
    expect(rows[0].collectionLabel).toBe('API');
    expect(rows[0].coverageLabels).toEqual(['TrueNAS data']);
  });

  it('keeps guest-linked agents out of the top-level infrastructure ledger', () => {
    const rows = buildInfrastructureSystemRows({
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
    });

    expect(rows.map((row) => row.name)).toEqual(['tower']);
  });

  it('returns no rows when nothing is being monitored yet', () => {
    expect(
      buildInfrastructureSystemRows({
        activeRows: [],
        monitoringStoppedRows: [],
      }),
    ).toEqual([]);
  });
});
