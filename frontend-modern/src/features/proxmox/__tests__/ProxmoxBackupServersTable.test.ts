import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { buildBackupServerRows, createPbsCorrelationRetention } from '../ProxmoxBackupServersTable';

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
    expect(rows.map((row) => row.datastore?.name)).toEqual(['offsite', 'tank']);
  });

  it('keeps server and datastore rows stable when provider order changes', () => {
    const firstServer = makePbsResource({
      id: 'pbs-zulu',
      name: 'zulu',
      pbs: {
        instanceId: 'zulu',
        datastores: [
          { name: 'tank', total: 1_000, used: 400, available: 600 },
          { name: 'archive', total: 2_000, used: 500, available: 1_500 },
        ],
      },
    });
    const secondServer = makePbsResource({
      id: 'pbs-alpha',
      name: 'alpha',
      pbs: {
        instanceId: 'alpha',
        datastores: [
          { name: 'fast', total: 1_000, used: 200, available: 800 },
          { name: 'bulk', total: 4_000, used: 1_000, available: 3_000 },
        ],
      },
    });

    const orderedKeys = buildBackupServerRows([firstServer, secondServer]).map((row) => row.key);
    const reorderedKeys = buildBackupServerRows([
      {
        ...secondServer,
        pbs: { ...secondServer.pbs!, datastores: [...secondServer.pbs!.datastores!].reverse() },
      },
      {
        ...firstServer,
        pbs: { ...firstServer.pbs!, datastores: [...firstServer.pbs!.datastores!].reverse() },
      },
    ]).map((row) => row.key);

    expect(orderedKeys).toEqual([
      'pbs-alpha:bulk',
      'pbs-alpha:fast',
      'pbs-zulu:archive',
      'pbs-zulu:tank',
    ]);
    expect(reorderedKeys).toEqual(orderedKeys);
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

const makeCorrelatablePbs = (lastSeen = 1_700_000_000_000): Resource =>
  makePbsResource({
    id: 'pbs-1',
    name: 'proxback',
    displayName: 'proxback',
    platformId: 'proxback',
    sources: ['pbs'],
    lastSeen,
    pbs: {
      instanceId: 'proxback',
      hostname: 'proxback-vm',
      connectionHealth: 'healthy',
      datastores: [{ name: 'tank', total: 1_000, used: 400, available: 600 }],
    },
    // The PBS service target names the service key, not the host series.
    metricsTarget: { resourceType: 'agent', resourceId: 'proxback' },
  });

const makeCorrelatedAgent = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'agent-proxback',
    type: 'agent',
    name: 'proxback',
    displayName: 'proxback',
    platformId: 'agent-proxback',
    platformType: 'proxmox-pbs',
    sourceType: 'hybrid',
    sources: ['agent', 'pbs'],
    status: 'online',
    lastSeen: 1_700_000_000_000,
    agent: { agentId: 'agent-proxback', hostname: 'proxback' },
    metricsTarget: { resourceType: 'agent', resourceId: 'agent-proxback' },
    ...overrides,
  }) as Resource;

describe('buildBackupServerRows PBS host correlation retention', () => {
  it('uses the PBS-reported node name when its connection label and endpoint are not the host identity', () => {
    const pbs = makePbsResource({
      id: 'pbs-1',
      name: 'backup-connection',
      displayName: 'backup-connection',
      platformId: 'backup-connection',
      sources: ['pbs'],
      pbs: {
        instanceId: 'backup-connection',
        hostname: '10.0.0.5',
        nodeName: 'proxback-vm',
        connectionHealth: 'healthy',
        datastores: [{ name: 'tank', total: 1_000, used: 400, available: 600 }],
      },
      metricsTarget: { resourceType: 'agent', resourceId: 'backup-connection' },
    });
    const agent = makeCorrelatedAgent({
      name: 'proxback-vm',
      displayName: 'proxback-vm',
      agent: { agentId: 'agent-proxback', hostname: 'proxback-vm' },
      metricsTarget: { resourceType: 'agent', resourceId: 'agent-proxback' },
    });

    const rows = buildBackupServerRows([pbs, agent]);

    expect(rows).toHaveLength(1);
    expect(rows[0].resource.id).toBe('pbs-1');
    expect(rows[0].resource.pbs?.instanceId).toBe('backup-connection');
    expect(rows[0].resource.metricsTarget).toEqual({
      resourceType: 'agent',
      resourceId: 'agent-proxback',
    });
  });

  it('retains the resolved host target while a refresh snapshot omits the host row', () => {
    const retention = createPbsCorrelationRetention();
    const pbs = makeCorrelatablePbs();
    const agent = makeCorrelatedAgent();

    const first = buildBackupServerRows([pbs, agent], [], retention);
    expect(first[0].resource.metricsTarget?.resourceId).toBe('agent-proxback');

    // The host row is briefly absent; the service target must not replace it.
    const omitted = buildBackupServerRows([pbs], [], retention);
    expect(omitted[0].resource.metricsTarget?.resourceId).toBe('agent-proxback');

    // A later snapshot that carries the host again confirms the same target.
    const restored = buildBackupServerRows([pbs, agent], [], retention);
    expect(restored[0].resource.metricsTarget?.resourceId).toBe('agent-proxback');
  });

  it('does not reuse a remembered host when the current snapshot is ambiguous', () => {
    const retention = createPbsCorrelationRetention();
    const pbs = makeCorrelatablePbs();
    const agent = makeCorrelatedAgent();
    buildBackupServerRows([pbs, agent], [], retention);

    const otherAgent = makeCorrelatedAgent({
      id: 'agent-other',
      platformId: 'agent-other',
      agent: { agentId: 'agent-other', hostname: 'proxback' },
      metricsTarget: { resourceType: 'agent', resourceId: 'agent-other' },
    });
    const ambiguous = buildBackupServerRows([pbs, agent, otherAgent], [], retention);

    expect(ambiguous[0].resource.metricsTarget?.resourceId).toBe('proxback');
  });

  it('drops a remembered host once it is stale relative to the server', () => {
    const retention = createPbsCorrelationRetention();
    const agent = makeCorrelatedAgent({ lastSeen: 1_700_000_000_000 });
    const pbs = makeCorrelatablePbs(1_700_000_000_000);
    buildBackupServerRows([pbs, agent], [], retention);

    const stale = makeCorrelatablePbs(1_700_000_000_000 + 6 * 60 * 1000);
    const rows = buildBackupServerRows([stale], [], retention);
    expect(rows[0].resource.metricsTarget?.resourceId).toBe('proxback');
    expect(retention.size).toBe(0);
  });

  it('prunes remembered hosts for servers that are no longer present', () => {
    const retention = createPbsCorrelationRetention();
    const pbs = makeCorrelatablePbs();
    buildBackupServerRows([pbs, makeCorrelatedAgent()], [], retention);
    expect(retention.has('pbs-1')).toBe(true);

    buildBackupServerRows([makePbsResource({ id: 'pbs-2', name: 'other' })], [], retention);
    expect(retention.has('pbs-1')).toBe(false);
  });
});
