import { describe, expect, it } from 'vitest';

import { pbsInstanceFromResource, pmgInstanceFromResource } from '../resourceStateAdapters';
import type { Resource } from '@/types/resource';

// These mappers (mapPBS*/mapPMG*) are module-private; they are only reachable
// through the exported pbsInstanceFromResource / pmgInstanceFromResource entry
// points, which read the raw facet arrays off platformData and run each element
// through the mapper before `.filter(Boolean)`-ing nulls away.

const createPBSResource = (
  pbsFacet: Record<string, unknown>,
  overrides: Partial<Resource> = {},
): Resource =>
  ({
    id: 'pbs-1',
    type: 'pbs',
    name: 'pbs-name',
    displayName: 'PBS',
    platformId: 'pbs-1',
    platformType: 'proxmox-pbs',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    cpu: { current: 0 },
    memory: { current: 0, total: 0, used: 0 },
    disk: { current: 0, total: 0, used: 0 },
    platformData: { pbs: pbsFacet },
    ...overrides,
  }) as Resource;

const createPMGResource = (
  pmgFacet: Record<string, unknown>,
  overrides: Partial<Resource> = {},
): Resource =>
  ({
    id: 'pmg-1',
    type: 'pmg',
    name: 'pmg-name',
    displayName: 'PMG',
    platformId: 'pmg-1',
    platformType: 'proxmox-pmg',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    cpu: { current: 0 },
    memory: { current: 0, total: 0, used: 0 },
    disk: { current: 0, total: 0, used: 0 },
    platformData: { pmg: pmgFacet },
    ...overrides,
  }) as Resource;

describe('mapPBSNamespace (via pbsInstanceFromResource.datastores[].namespaces)', () => {
  it('maps a well-formed namespace', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({
        datastores: [{ namespaces: [{ path: '/ns/a', parent: '/ns', depth: 2 }] }],
      }),
    );

    expect(instance?.datastores[0].namespaces).toEqual([
      { path: '/ns/a', parent: '/ns', depth: 2 },
    ]);
  });

  it('defaults missing fields, trims strings, coerces NaN depth, and drops non-record entries', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({
        datastores: [
          {
            namespaces: [
              {},
              null,
              7,
              'nope',
              { path: '  /kept  ', parent: '   ', depth: Number.NaN },
            ],
          },
        ],
      }),
    );

    expect(instance?.datastores[0].namespaces).toEqual([
      { path: '', parent: '', depth: 0 },
      { path: '/kept', parent: '', depth: 0 },
    ]);
  });
});

describe('mapPBSDatastore (via pbsInstanceFromResource.datastores)', () => {
  it('maps a well-formed datastore with nested namespaces and dedup factor', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({
        datastores: [
          {
            name: 'store1',
            total: 1000,
            used: 250,
            free: 750,
            usage: 25,
            status: 'ok',
            error: '',
            namespaces: [{ path: '/a', parent: '/', depth: 1 }],
            deduplicationFactor: 3.5,
          },
        ],
      }),
    );

    expect(instance?.datastores).toEqual([
      {
        name: 'store1',
        total: 1000,
        used: 250,
        free: 750,
        usage: 25,
        status: 'ok',
        error: '',
        namespaces: [{ path: '/a', parent: '/', depth: 1 }],
        deduplicationFactor: 3.5,
      },
    ]);
  });

  it('falls back to available then computed free, computes usage from ratio, and drops non-records', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({
        datastores: [
          null,
          { name: 'avail', total: 1000, used: 200, available: 500 },
          { name: 'computed', total: 1000, used: 300 },
        ],
      }),
    );

    expect(instance?.datastores).toHaveLength(2);
    expect(instance?.datastores[0]).toMatchObject({ name: 'avail', free: 500, usage: 20 });
    expect(instance?.datastores[1]).toMatchObject({ name: 'computed', free: 700, usage: 30 });
  });

  it('prefers usage over usagePercent and returns 0 usage when total is 0', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({
        datastores: [
          { name: 'a', usage: 55, usagePercent: 99 },
          { name: 'b', usagePercent: 42 },
          { name: 'c', total: 0, used: 0 },
        ],
      }),
    );

    expect(instance?.datastores[0].usage).toBe(55);
    expect(instance?.datastores[1].usage).toBe(42);
    expect(instance?.datastores[2].usage).toBe(0);
  });

  it('clamps free to zero when used exceeds total', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({ datastores: [{ name: 'over', total: 10, used: 20 }] }),
    );

    expect(instance?.datastores[0].free).toBe(0);
    expect(instance?.datastores[0].usage).toBe(200);
  });

  it('defaults every field for an empty record and leaves dedup factor undefined', () => {
    const instance = pbsInstanceFromResource(createPBSResource({ datastores: [{}] }));

    expect(instance?.datastores[0]).toEqual({
      name: '',
      total: 0,
      used: 0,
      free: 0,
      usage: 0,
      status: '',
      error: '',
      namespaces: [],
    });
    expect(instance?.datastores[0].deduplicationFactor).toBeUndefined();
  });

  it('ignores non-numeric string totals/used (typed coercion rejects strings)', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({ datastores: [{ name: 's', total: '1000', used: '250' }] }),
    );

    expect(instance?.datastores[0].total).toBe(0);
    expect(instance?.datastores[0].used).toBe(0);
    expect(instance?.datastores[0].usage).toBe(0);
  });
});

describe('mapPBSBackupJob (via pbsInstanceFromResource.backupJobs)', () => {
  it('maps a well-formed backup job', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({
        backupJobs: [
          {
            id: 'job-1',
            store: 'store1',
            type: 'vm',
            vmid: '100',
            lastBackup: '2026-01-01T00:00:00Z',
            nextRun: '2026-01-02T00:00:00Z',
            status: 'ok',
            error: '',
          },
        ],
      }),
    );

    expect(instance?.backupJobs).toEqual([
      {
        id: 'job-1',
        store: 'store1',
        type: 'vm',
        vmid: '100',
        lastBackup: '2026-01-01T00:00:00Z',
        nextRun: '2026-01-02T00:00:00Z',
        status: 'ok',
        error: '',
      },
    ]);
  });

  it('defaults every string field (including id) and drops non-record entries', () => {
    const instance = pbsInstanceFromResource(createPBSResource({ backupJobs: [{}, null, 'x'] }));

    expect(instance?.backupJobs).toHaveLength(1);
    expect(instance?.backupJobs[0]).toEqual({
      id: '',
      store: '',
      type: '',
      vmid: '',
      lastBackup: '',
      nextRun: '',
      status: '',
      error: '',
    });
  });
});

describe('mapPBSSyncJob (via pbsInstanceFromResource.syncJobs)', () => {
  it('maps a well-formed sync job', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({
        syncJobs: [
          {
            id: 'sync-1',
            store: 'store1',
            remote: 'remote-a',
            status: 'running',
            lastSync: '2026-01-01T00:00:00Z',
            nextRun: '2026-01-02T00:00:00Z',
            error: '',
          },
        ],
      }),
    );

    expect(instance?.syncJobs).toEqual([
      {
        id: 'sync-1',
        store: 'store1',
        remote: 'remote-a',
        status: 'running',
        lastSync: '2026-01-01T00:00:00Z',
        nextRun: '2026-01-02T00:00:00Z',
        error: '',
      },
    ]);
  });

  it('defaults missing string fields and drops non-record entries', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({ syncJobs: [{ id: 'kept' }, null, 42] }),
    );

    expect(instance?.syncJobs).toHaveLength(1);
    expect(instance?.syncJobs[0]).toEqual({
      id: 'kept',
      store: '',
      remote: '',
      status: '',
      lastSync: '',
      nextRun: '',
      error: '',
    });
  });
});

describe('mapPBSVerifyJob (via pbsInstanceFromResource.verifyJobs)', () => {
  it('maps a well-formed verify job', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({
        verifyJobs: [
          {
            id: 'verify-1',
            store: 'store1',
            status: 'ok',
            lastVerify: '2026-01-01T00:00:00Z',
            nextRun: '2026-01-02T00:00:00Z',
            error: '',
          },
        ],
      }),
    );

    expect(instance?.verifyJobs).toEqual([
      {
        id: 'verify-1',
        store: 'store1',
        status: 'ok',
        lastVerify: '2026-01-01T00:00:00Z',
        nextRun: '2026-01-02T00:00:00Z',
        error: '',
      },
    ]);
  });

  it('defaults missing string fields and drops non-record entries', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({ verifyJobs: [{ id: 'kept' }, null] }),
    );

    expect(instance?.verifyJobs).toHaveLength(1);
    expect(instance?.verifyJobs[0]).toEqual({
      id: 'kept',
      store: '',
      status: '',
      lastVerify: '',
      nextRun: '',
      error: '',
    });
  });
});

describe('mapPBSPruneJob (via pbsInstanceFromResource.pruneJobs)', () => {
  it('maps a well-formed prune job', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({
        pruneJobs: [
          {
            id: 'prune-1',
            store: 'store1',
            status: 'ok',
            lastPrune: '2026-01-01T00:00:00Z',
            nextRun: '2026-01-02T00:00:00Z',
            error: '',
          },
        ],
      }),
    );

    expect(instance?.pruneJobs).toEqual([
      {
        id: 'prune-1',
        store: 'store1',
        status: 'ok',
        lastPrune: '2026-01-01T00:00:00Z',
        nextRun: '2026-01-02T00:00:00Z',
        error: '',
      },
    ]);
  });

  it('defaults missing string fields and drops non-record entries', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({ pruneJobs: [{ id: 'kept' }, null, 'nope'] }),
    );

    expect(instance?.pruneJobs).toHaveLength(1);
    expect(instance?.pruneJobs[0]).toEqual({
      id: 'kept',
      store: '',
      status: '',
      lastPrune: '',
      nextRun: '',
      error: '',
    });
  });
});

describe('mapPBSGarbageJob (via pbsInstanceFromResource.garbageJobs)', () => {
  it('maps a well-formed garbage job including removedBytes', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({
        garbageJobs: [
          {
            id: 'gc-1',
            store: 'store1',
            status: 'ok',
            lastGarbage: '2026-01-01T00:00:00Z',
            nextRun: '2026-01-02T00:00:00Z',
            removedBytes: 1048576,
            error: '',
          },
        ],
      }),
    );

    expect(instance?.garbageJobs).toEqual([
      {
        id: 'gc-1',
        store: 'store1',
        status: 'ok',
        lastGarbage: '2026-01-01T00:00:00Z',
        nextRun: '2026-01-02T00:00:00Z',
        removedBytes: 1048576,
        error: '',
      },
    ]);
  });

  it('defaults removedBytes to 0 and drops non-record entries', () => {
    const instance = pbsInstanceFromResource(
      createPBSResource({ garbageJobs: [{ id: 'kept' }, null] }),
    );

    expect(instance?.garbageJobs).toHaveLength(1);
    expect(instance?.garbageJobs[0]).toEqual({
      id: 'kept',
      store: '',
      status: '',
      lastGarbage: '',
      nextRun: '',
      removedBytes: 0,
      error: '',
    });
  });
});

describe('mapPMGNodeStatus (via pmgInstanceFromResource.nodes)', () => {
  it('maps a well-formed node with a full queue status', () => {
    const instance = pmgInstanceFromResource(
      createPMGResource({
        nodes: [
          {
            name: 'node-a',
            status: 'online',
            role: 'master',
            uptime: 3600,
            loadAvg: '0.10 0.20 0.30',
            queueStatus: {
              active: 1,
              deferred: 2,
              hold: 3,
              incoming: 4,
              total: 10,
              oldestAge: 60,
              updatedAt: '2026-01-01T00:00:00Z',
            },
          },
        ],
      }),
    );

    expect(instance?.nodes).toEqual([
      {
        name: 'node-a',
        status: 'online',
        role: 'master',
        uptime: 3600,
        loadAvg: '0.10 0.20 0.30',
        queueStatus: {
          active: 1,
          deferred: 2,
          hold: 3,
          incoming: 4,
          total: 10,
          oldestAge: 60,
          updatedAt: '2026-01-01T00:00:00Z',
        },
      },
    ]);
  });

  it('defaults every queue subfield when queueStatus is an empty object', () => {
    const instance = pmgInstanceFromResource(
      createPMGResource({ nodes: [{ name: 'n', status: 'up', queueStatus: {} }] }),
    );

    expect(instance?.nodes?.[0].queueStatus).toEqual({
      active: 0,
      deferred: 0,
      hold: 0,
      incoming: 0,
      total: 0,
      oldestAge: 0,
      updatedAt: '',
    });
  });

  it('omits queueStatus when absent, defaults name/status, and drops non-record entries', () => {
    const instance = pmgInstanceFromResource(createPMGResource({ nodes: [{}, null] }));

    expect(instance?.nodes).toHaveLength(1);
    expect(instance?.nodes?.[0].name).toBe('');
    expect(instance?.nodes?.[0].status).toBe('');
    expect(instance?.nodes?.[0].role).toBeUndefined();
    expect(instance?.nodes?.[0].uptime).toBeUndefined();
    expect(instance?.nodes?.[0].loadAvg).toBeUndefined();
    expect(instance?.nodes?.[0].queueStatus).toBeUndefined();
  });
});

describe('mapPMGSpamBucket (via pmgInstanceFromResource.spamDistribution)', () => {
  it('prefers score over bucket and defaults count when absent', () => {
    const instance = pmgInstanceFromResource(
      createPMGResource({
        spamDistribution: [
          { score: '5+', count: 10 },
          { bucket: 'high', count: 5 },
          { score: '5+', bucket: 'ignored' },
        ],
      }),
    );

    expect(instance?.spamDistribution).toEqual([
      { score: '5+', count: 10 },
      { score: 'high', count: 5 },
      { score: '5+', count: 0 },
    ]);
  });

  it('defaults to empty score and zero count, dropping non-record entries', () => {
    const instance = pmgInstanceFromResource(
      createPMGResource({ spamDistribution: [{}, null, 3] }),
    );

    expect(instance?.spamDistribution).toEqual([{ score: '', count: 0 }]);
  });
});

describe('mapPMGRelayDomain (via pmgInstanceFromResource.relayDomains)', () => {
  it('maps a relay domain with an optional comment', () => {
    const instance = pmgInstanceFromResource(
      createPMGResource({
        relayDomains: [{ domain: 'example.com', comment: 'primary' }, { domain: 'bare.example' }],
      }),
    );

    expect(instance?.relayDomains).toHaveLength(2);
    expect(instance?.relayDomains?.[0]).toEqual({ domain: 'example.com', comment: 'primary' });
    expect(instance?.relayDomains?.[1].domain).toBe('bare.example');
    expect(instance?.relayDomains?.[1].comment).toBeUndefined();
  });

  it('defaults empty domain and drops non-record entries', () => {
    const instance = pmgInstanceFromResource(createPMGResource({ relayDomains: [{}, null] }));

    expect(instance?.relayDomains).toHaveLength(1);
    expect(instance?.relayDomains?.[0].domain).toBe('');
    expect(instance?.relayDomains?.[0].comment).toBeUndefined();
  });
});

describe('mapPMGDomainStat (via pmgInstanceFromResource.domainStats)', () => {
  it('maps a well-formed domain stat including bytes', () => {
    const instance = pmgInstanceFromResource(
      createPMGResource({
        domainStats: [
          {
            domain: 'a.com',
            mailCount: 100,
            spamCount: 5,
            virusCount: 2,
            bytes: 4096,
          },
        ],
      }),
    );

    expect(instance?.domainStats).toEqual([
      { domain: 'a.com', mailCount: 100, spamCount: 5, virusCount: 2, bytes: 4096 },
    ]);
  });

  it('defaults counts to zero, leaves bytes undefined, and drops non-record entries', () => {
    const instance = pmgInstanceFromResource(
      createPMGResource({ domainStats: [{ domain: 'b.com' }, null, 'x'] }),
    );

    expect(instance?.domainStats).toHaveLength(1);
    expect(instance?.domainStats?.[0]).toEqual({
      domain: 'b.com',
      mailCount: 0,
      spamCount: 0,
      virusCount: 0,
    });
    expect(instance?.domainStats?.[0].bytes).toBeUndefined();
  });
});

describe('mapPMGMailCountPoint (via pmgInstanceFromResource.mailCount)', () => {
  it('maps a well-formed point and trims the timestamp string', () => {
    const instance = pmgInstanceFromResource(
      createPMGResource({
        mailCount: [
          {
            timestamp: '  2026-07-09T12:00:00Z  ',
            count: 9,
            countIn: 3,
            countOut: 6,
            spamIn: 1,
            spamOut: 2,
            virusIn: 0,
            virusOut: 0,
            rblRejects: 4,
            pregreet: 1,
            bouncesIn: 0,
            bouncesOut: 0,
            greylist: 7,
            index: 5,
            timeframe: 'day',
            windowStart: '2026-07-09T00:00:00Z',
            windowEnd: '2026-07-09T23:59:59Z',
          },
        ],
      }),
    );

    expect(instance?.mailCount).toEqual([
      {
        timestamp: '2026-07-09T12:00:00Z',
        count: 9,
        countIn: 3,
        countOut: 6,
        spamIn: 1,
        spamOut: 2,
        virusIn: 0,
        virusOut: 0,
        rblRejects: 4,
        pregreet: 1,
        bouncesIn: 0,
        bouncesOut: 0,
        greylist: 7,
        index: 5,
        timeframe: 'day',
        windowStart: '2026-07-09T00:00:00Z',
        windowEnd: '2026-07-09T23:59:59Z',
      },
    ]);
  });

  it('falls back to the epoch timestamp and zeroes every count for an empty record', () => {
    const instance = pmgInstanceFromResource(createPMGResource({ mailCount: [{}, null] }));

    expect(instance?.mailCount).toHaveLength(1);
    expect(instance?.mailCount?.[0]).toEqual({
      timestamp: '1970-01-01T00:00:00.000Z',
      count: 0,
      countIn: 0,
      countOut: 0,
      spamIn: 0,
      spamOut: 0,
      virusIn: 0,
      virusOut: 0,
      rblRejects: 0,
      pregreet: 0,
      bouncesIn: 0,
      bouncesOut: 0,
      greylist: 0,
      index: 0,
      timeframe: '',
    });
    expect(instance?.mailCount?.[0].windowStart).toBeUndefined();
    expect(instance?.mailCount?.[0].windowEnd).toBeUndefined();
  });
});
