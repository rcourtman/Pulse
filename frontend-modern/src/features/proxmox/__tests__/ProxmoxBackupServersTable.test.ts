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
  it('retains failed datastore-read evidence without changing server or row identity', () => {
    const server = makePbsResource();
    server.pbs!.datastores![0] = {
      name: 'tank',
      total: 0,
      used: 0,
      available: 0,
      usagePercent: 0,
      status: 'unavailable',
      error: 'synthetic status failure',
    };
    const rows = buildBackupServerRows([server]);
    const tank = rows.find((row) => row.datastore?.name === 'tank')!;
    expect(tank.key).toBe('pbs-1:tank');
    expect(tank.online).toBe(true);
    expect(tank.datastore).toEqual(server.pbs!.datastores![0]);
    expect(rows.find((row) => row.datastore?.name === 'offsite')?.datastore?.usagePercent).toBe(95);
  });

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
      nodeName: 'proxback-vm',
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
    agent: { agentId: 'agent-proxback', hostname: 'proxback-vm' },
    metricsTarget: { resourceType: 'agent', resourceId: 'agent-proxback' },
    ...overrides,
  }) as Resource;

describe('buildBackupServerRows PBS machine-identity boundary', () => {
  it.each(['name', 'displayName', 'platformId', 'instanceId', 'canonicalIdentity', 'identity'])(
    'does not import an unrelated guest History or host disks from a PBS %s collision',
    (field) => {
      const pbs = makeCorrelatablePbs();
      pbs.name = 'backup-connection';
      pbs.displayName = 'Backup connection';
      pbs.platformId = 'pbs-service';
      pbs.pbs = {
        ...pbs.pbs!,
        instanceId: 'pbs-service',
        hostname: '10.2.0.13',
        nodeName: undefined,
      };
      if (field === 'instanceId') pbs.pbs.instanceId = 'other-host';
      else if (field === 'canonicalIdentity') {
        pbs.canonicalIdentity = { hostname: 'other-host', platformId: 'other-host' };
      } else if (field === 'identity')
        pbs.identity = { hostname: 'other-host', ips: ['10.9.0.13'] };
      else pbs[field as 'name' | 'displayName' | 'platformId'] = 'other-host';
      const unrelated = makeCorrelatedAgent({
        id: 'vm-other',
        type: 'vm',
        name: 'other-host',
        agent: {
          agentId: 'agent-other',
          hostname: 'other-host',
          disks: [{ mountpoint: '/wrong-host' }],
        },
        disk: { current: 99 },
        metricsTarget: { resourceType: 'vm', resourceId: 'vm-other' },
      });

      const row = buildBackupServerRows([pbs, unrelated])[0];
      expect(row.resource.metricsTarget).toEqual(pbs.metricsTarget);
      expect(row.resource.agent).toBeUndefined();
      expect(row.resource.disk).toBeUndefined();
      expect(row.resource.sourceType).toBe('api');
    },
  );

  it('does not correlate different endpoint IPs through their first dotted token', () => {
    const pbs = makeCorrelatablePbs();
    pbs.name = 'backup-connection';
    pbs.pbs = { ...pbs.pbs!, hostname: '10.2.0.13', nodeName: undefined };
    const unrelated = makeCorrelatedAgent({
      name: 'other-host',
      agent: { agentId: 'agent-other', hostname: 'other-host' },
      identity: { ips: ['10.9.0.13'] },
    });
    expect(buildBackupServerRows([pbs, unrelated])[0].resource.metricsTarget).toEqual(
      pbs.metricsTarget,
    );
  });

  it('does not use a guessed endpoint-to-Agent match without backend corroboration', () => {
    const pbs = makeCorrelatablePbs();
    pbs.pbs = { ...pbs.pbs!, hostname: '10.2.0.13', nodeName: undefined };
    const unrelated = makeCorrelatedAgent({ identity: { ips: ['10.2.0.13'] } });
    expect(buildBackupServerRows([pbs, unrelated])[0].resource.metricsTarget).toEqual(
      pbs.metricsTarget,
    );
  });

  it('does not let a present label-colliding guest undo backend link withdrawal', () => {
    const retention = createPbsCorrelationRetention();
    const pbs = makeCorrelatablePbs();
    pbs.pbs = { ...pbs.pbs!, hostname: '10.2.0.13', linkedAgentId: 'agent-proxback' };
    buildBackupServerRows([pbs, makeCorrelatedAgent()], [], retention);
    const withdrawn = { ...pbs, pbs: { ...pbs.pbs!, linkedAgentId: undefined } };
    const unrelated = makeCorrelatedAgent({
      id: 'vm-other',
      type: 'vm',
      agent: { agentId: 'agent-other', hostname: pbs.name },
      metricsTarget: { resourceType: 'vm', resourceId: 'vm-other' },
      disk: { current: 99 },
    });
    for (let refresh = 0; refresh < 2; refresh++) {
      const row = buildBackupServerRows([withdrawn, unrelated], [], retention);
      expect(row[0].resource.metricsTarget).toEqual(pbs.metricsTarget);
      expect(row[0].resource.agent).toBeUndefined();
      expect(row[0].resource.disk).toBeUndefined();
      expect(retention.size).toBe(0);
    }
  });

  it('does not match a reported PBS node to a guest display name instead of its Agent hostname', () => {
    const pbs = makeCorrelatablePbs();
    pbs.pbs = { ...pbs.pbs!, nodeName: 'real-pbs.example' };
    const unrelated = makeCorrelatedAgent({
      name: 'real-pbs.example',
      agent: { agentId: 'agent-other', hostname: 'other-host.example' },
    });
    expect(buildBackupServerRows([pbs, unrelated])[0].resource.metricsTarget).toEqual(
      pbs.metricsTarget,
    );
  });

  it('does not treat different qualified machine names as the same host', () => {
    const pbs = makeCorrelatablePbs();
    pbs.pbs = { ...pbs.pbs!, nodeName: 'backup.production.example' };
    const unrelated = makeCorrelatedAgent({
      agent: { agentId: 'agent-other', hostname: 'backup.test.example' },
    });
    expect(buildBackupServerRows([pbs, unrelated])[0].resource.metricsTarget).toEqual(
      pbs.metricsTarget,
    );
  });

  it.each(['direct', 'platform'])(
    'preserves a fresh reported node-to-%s-Agent hostname match, ignoring display labels',
    (facet) => {
      const pbs = makeCorrelatablePbs();
      pbs.pbs = { ...pbs.pbs!, nodeName: ' PROXBACK-VM.EXAMPLE. ' };
      const agent = makeCorrelatedAgent({
        name: 'Custom host label',
        displayName: 'Custom display label',
        agent:
          facet === 'direct'
            ? { agentId: 'agent-proxback', hostname: 'proxback-vm.example' }
            : undefined,
        platformData:
          facet === 'platform'
            ? {
                agent: { agentId: 'agent-proxback', hostname: 'proxback-vm.example' },
              }
            : undefined,
      });
      expect(buildBackupServerRows([pbs, agent])[0].resource.metricsTarget).toEqual(
        agent.metricsTarget,
      );
    },
  );

  it('deduplicates a genuinely matching node/Agent surfaced as both guest and host', () => {
    const pbs = makeCorrelatablePbs();
    const host = makeCorrelatedAgent();
    const guest = {
      ...host,
      id: 'vm-proxback',
      type: 'vm',
      metricsTarget: {
        resourceType: 'vm',
        resourceId: 'vm-proxback',
      },
    } as Resource;
    expect(buildBackupServerRows([pbs, host, guest])[0].resource.metricsTarget).toEqual(
      guest.metricsTarget,
    );
  });

  it.each([-6, 6])('declines a node-name match with a %s-minute observation gap', (minutes) => {
    const pbs = makeCorrelatablePbs();
    const agent = makeCorrelatedAgent({ lastSeen: pbs.lastSeen + minutes * 60_000 });
    expect(buildBackupServerRows([pbs, agent])[0].resource.metricsTarget).toEqual(
      pbs.metricsTarget,
    );
  });

  it.each(['stale', 'old-report', 'invalid-report'])(
    'does not let a recent PVE row hide %s Agent evidence',
    (state) => {
      const pbs = makeCorrelatablePbs();
      const agent = makeCorrelatedAgent({
        type: 'vm',
        agent: {
          agentId: 'agent-proxback',
          hostname: 'proxback-vm',
          stale: state === 'stale',
          lastReportAt:
            state === 'old-report'
              ? new Date(pbs.lastSeen - 6 * 60_000).toISOString()
              : state === 'invalid-report'
                ? 'invalid'
                : undefined,
        },
      });
      expect(buildBackupServerRows([pbs, agent])[0].resource.metricsTarget).toEqual(
        pbs.metricsTarget,
      );
    },
  );

  it('declines an observed hostname without a concrete Agent identity', () => {
    const pbs = makeCorrelatablePbs();
    const agent = makeCorrelatedAgent({ agent: { hostname: 'proxback-vm' } });
    expect(buildBackupServerRows([pbs, agent])[0].resource.metricsTarget).toEqual(
      pbs.metricsTarget,
    );
  });

  it('revokes a retained node-name match when the reported machine identity disappears', () => {
    const retention = createPbsCorrelationRetention();
    const pbs = makeCorrelatablePbs();
    buildBackupServerRows([pbs, makeCorrelatedAgent()], [], retention);
    expect(retention.size).toBe(1);
    const uncorroborated = { ...pbs, pbs: { ...pbs.pbs!, nodeName: undefined } };
    expect(
      buildBackupServerRows([uncorroborated], [], retention)[0].resource.metricsTarget,
    ).toEqual(pbs.metricsTarget);
    expect(retention.size).toBe(0);
  });
});

describe('buildBackupServerRows PBS host correlation retention', () => {
  it('keeps three PBS guests on their own History targets across PVE and VirtualBox hosts', () => {
    const pbsServers = ['one', 'two', 'three'].map((suffix) => {
      const pbs = makeCorrelatablePbs();
      pbs.id = `pbs-${suffix}`;
      pbs.name = `backup-connection-${suffix}`;
      pbs.pbs = {
        ...pbs.pbs!,
        instanceId: `pbs-${suffix}`,
        hostname: `10.2.0.${suffix === 'one' ? 13 : suffix === 'two' ? 23 : 33}`,
        nodeName: undefined,
        linkedAgentId: `agent-${suffix}`,
      };
      pbs.metricsTarget = { resourceType: 'agent', resourceId: `pbs-${suffix}` };
      return pbs;
    });
    const agents = ['one', 'two', 'three'].map((suffix) =>
      makeCorrelatedAgent({
        id: `agent-${suffix}`,
        name: `guest-${suffix}`,
        agent: { agentId: `agent-${suffix}`, hostname: `guest-${suffix}` },
        metricsTarget: { resourceType: 'agent', resourceId: `agent-${suffix}` },
      }),
    );
    const pveGuests = agents.slice(0, 2).map((agent): Resource => ({
      ...agent,
      id: `vm-${agent.agent!.agentId}`,
      type: 'vm',
      sources: ['proxmox', 'agent'],
      metricsTarget: { resourceType: 'vm', resourceId: `vm-${agent.agent!.agentId}` },
    }));
    const otherAgents = Array.from({ length: 6 }, (_, index) =>
      makeCorrelatedAgent({
        id: `agent-unrelated-${index}`,
        name: `backup-connection-${index % 3 === 0 ? 'one' : index % 3 === 1 ? 'two' : 'three'}`,
        agent: { agentId: `agent-unrelated-${index}` },
      }),
    );

    const rows = buildBackupServerRows([...pbsServers, ...agents, ...pveGuests, ...otherAgents]);
    expect(
      rows.map((row) => [row.resource.id, row.resource.agent?.agentId, row.resource.metricsTarget]),
    ).toEqual([
      ['pbs-one', 'agent-one', { resourceType: 'vm', resourceId: 'vm-agent-one' }],
      ['pbs-three', 'agent-three', { resourceType: 'agent', resourceId: 'agent-three' }],
      ['pbs-two', 'agent-two', { resourceType: 'vm', resourceId: 'vm-agent-two' }],
    ]);
  });

  it('keeps the agent UUID through PBS-only to side-by-side PVE/PBS refreshes without nodeName', () => {
    const retention = createPbsCorrelationRetention();
    const pbs = makeCorrelatablePbs();
    pbs.name = 'backup-connection';
    pbs.displayName = 'Backup connection';
    pbs.platformId = 'pbs-service';
    pbs.pbs = {
      ...pbs.pbs!,
      instanceId: 'pbs-service',
      hostname: '10.0.0.5',
      nodeName: undefined, // API tokens cannot read the PBS /nodes endpoint.
      linkedAgentId: 'agent-uuid', // Corroborated by the backend's host interface match.
    };
    pbs.metricsTarget = { resourceType: 'agent', resourceId: 'pbs-service' };
    const standalone = makeCorrelatedAgent({
      id: 'agent-uuid',
      name: 'backup-host.local',
      displayName: 'backup-host.local',
      platformId: 'agent-uuid',
      agent: { agentId: 'agent-uuid', hostname: 'backup-host.local' },
      metricsTarget: { resourceType: 'agent', resourceId: 'agent-uuid' },
    });
    const pveMerged = {
      ...standalone,
      id: 'pve-host',
      name: 'PVE display label',
      displayName: 'PVE display label',
      platformId: 'pve/backup-host',
      platformType: 'proxmox-pve',
      sources: ['proxmox', 'agent', 'pbs'],
      proxmox: { nodeName: 'backup-host' },
    } as Resource;

    expect(
      buildBackupServerRows([pbs, standalone], [], retention)[0].resource.metricsTarget,
    ).toEqual({
      resourceType: 'agent',
      resourceId: 'agent-uuid',
    });
    expect(
      buildBackupServerRows([pbs, pveMerged], [], retention)[0].resource.metricsTarget,
    ).toEqual({
      resourceType: 'agent',
      resourceId: 'agent-uuid',
    });
    expect(
      buildBackupServerRows([pbs, pveMerged], [], retention)[0].resource.metricsTarget,
    ).toEqual({
      resourceType: 'agent',
      resourceId: 'agent-uuid',
    });
  });

  it('does not replace an explicit PBS host link with a same-name different agent', () => {
    const pbs = makeCorrelatablePbs();
    pbs.pbs = { ...pbs.pbs!, linkedAgentId: 'agent-uuid' };
    const impostor = makeCorrelatedAgent({
      id: 'agent-other',
      agent: { agentId: 'agent-other', hostname: 'proxback-vm' },
      metricsTarget: { resourceType: 'agent', resourceId: 'agent-other' },
    });
    expect(buildBackupServerRows([pbs, impostor])[0].resource.metricsTarget?.resourceId).toBe(
      'proxback',
    );
  });

  it('does not mistake a PVE-only node for a second host agent by name', () => {
    const pbs = makeCorrelatablePbs();
    const agent = makeCorrelatedAgent();
    const pveOnly = {
      ...agent,
      id: 'pve-only',
      sources: ['proxmox'],
      agent: undefined,
      platformData: { sources: ['proxmox'], proxmox: { nodeName: 'proxback' } },
      metricsTarget: { resourceType: 'node', resourceId: 'pve-only' },
    } as Resource;

    expect(buildBackupServerRows([pbs, agent, pveOnly])[0].resource.metricsTarget?.resourceId).toBe(
      'agent-proxback',
    );
  });

  it('invalidates retained history when the corroborated agent identity changes', () => {
    const retention = createPbsCorrelationRetention();
    const pbs = makeCorrelatablePbs();
    pbs.pbs = { ...pbs.pbs!, linkedAgentId: 'agent-proxback' };
    buildBackupServerRows([pbs, makeCorrelatedAgent()], [], retention);

    const replacement = {
      ...pbs,
      pbs: { ...pbs.pbs!, linkedAgentId: 'agent-replacement' },
    } as Resource;
    expect(
      buildBackupServerRows([replacement], [], retention)[0].resource.metricsTarget?.resourceId,
    ).toBe('proxback');
    expect(retention.size).toBe(0);
  });

  it('drops retained host History when a backend link is withdrawn', () => {
    const retention = createPbsCorrelationRetention();
    const pbs = makeCorrelatablePbs();
    pbs.pbs = { ...pbs.pbs!, linkedAgentId: 'agent-proxback' };
    expect(
      buildBackupServerRows([pbs, makeCorrelatedAgent()], [], retention)[0].resource.metricsTarget
        ?.resourceId,
    ).toBe('agent-proxback');

    const withdrawn = { ...pbs, pbs: { ...pbs.pbs!, linkedAgentId: undefined } } as Resource;
    expect(
      buildBackupServerRows([withdrawn], [], retention)[0].resource.metricsTarget?.resourceId,
    ).toBe('proxback');
    expect(retention.size).toBe(0);
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
      agent: { agentId: 'agent-other', hostname: 'proxback-vm' },
      metricsTarget: { resourceType: 'agent', resourceId: 'agent-other' },
    });
    const ambiguous = buildBackupServerRows([pbs, agent, otherAgent], [], retention);

    expect(ambiguous[0].resource.metricsTarget?.resourceId).toBe('proxback');
    expect(retention.size).toBe(0);
    expect(buildBackupServerRows([pbs], [], retention)[0].resource.metricsTarget?.resourceId).toBe(
      'proxback',
    );
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
