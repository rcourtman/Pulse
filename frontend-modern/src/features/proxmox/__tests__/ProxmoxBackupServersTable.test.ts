import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { buildBackupServerRows } from '../ProxmoxBackupServersTable';

const makePbsResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'pbs-1',
    type: 'pbs',
    name: 'pbs-main',
    displayName: 'pbs-main',
    platformId: 'pbs-main',
    platformType: 'proxmox-pbs',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    cpu: { current: 12.4 },
    memory: { current: 40, total: 8_000, used: 3_200, free: 4_800 },
    uptime: 1_036_800, // 12d
    pbs: {
      instanceId: 'pbs-main',
      version: '3.2.1',
      connectionHealth: 'healthy',
      datastores: [
        { name: 'tank', total: 1_000, used: 400, available: 600, usagePercent: 40 },
        { name: 'offsite', total: 2_000, used: 1_900, available: 100, usagePercent: 95 },
      ],
    },
    ...overrides,
  }) as Resource;

describe('buildBackupServerRows', () => {
  it('carries host CPU, memory, and uptime onto every datastore row of the server', () => {
    const rows = buildBackupServerRows([makePbsResource()]);

    expect(rows).toHaveLength(2);
    for (const row of rows) {
      expect(row.cpuPercent).toBeCloseTo(12.4);
      expect(row.memoryPercent).toBeCloseTo(40);
      expect(row.memoryUsed).toBe(3_200);
      expect(row.memoryTotal).toBe(8_000);
      expect(row.uptimeSeconds).toBe(1_036_800);
    }
    expect(rows.map((row) => row.datastore?.name)).toEqual(['tank', 'offsite']);
  });

  it('keeps the reachability row when a server reports no datastores or metrics', () => {
    const rows = buildBackupServerRows([
      makePbsResource({
        id: 'pbs-2',
        name: 'pbs-empty',
        status: 'offline',
        cpu: undefined,
        memory: undefined,
        uptime: undefined,
        pbs: { instanceId: 'pbs-empty', connectionHealth: 'error', datastores: [] },
      }),
    ]);

    expect(rows).toHaveLength(1);
    expect(rows[0].online).toBe(false);
    expect(rows[0].cpuPercent).toBeUndefined();
    expect(rows[0].memoryPercent).toBeUndefined();
    expect(rows[0].uptimeSeconds).toBeUndefined();
  });
});
