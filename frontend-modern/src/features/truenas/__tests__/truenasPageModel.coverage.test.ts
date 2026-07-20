import { describe, expect, it } from 'vitest';
import type { RecoveryPoint } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import {
  buildTrueNASPageModel,
  buildTrueNASProtectionPosture,
  buildTrueNASStorageChildCounts,
  buildTrueNASStorageTopologyRows,
  buildTrueNASSystemChildCounts,
  mapTrueNASProtectionKind,
  mapTrueNASProtectionStatus,
  sortTrueNASProtectionPoints,
} from '../truenasPageModel';

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'truenas',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

const makeRecoveryPoint = (
  point: Partial<RecoveryPoint> & Pick<RecoveryPoint, 'id' | 'kind' | 'mode'>,
): RecoveryPoint => ({
  outcome: 'success',
  platform: 'truenas',
  startedAt: '2026-05-20T00:00:00Z',
  completedAt: '2026-05-20T00:00:00Z',
  ...point,
});

describe('truenasPageModel coverage', () => {
  describe('dataset/share/disk type-dispatch fallbacks', () => {
    it('classifies type "pool" resources as pools without requiring storage.topology', () => {
      const model = buildTrueNASPageModel([
        makeResource({ id: 'sys', type: 'agent' }),
        makeResource({ id: 'tank', type: 'pool' }),
      ]);
      expect(model.pools.map((r) => r.id)).toEqual(['tank']);
      expect(model.resources.map((r) => r.id)).toContain('tank');
    });

    it('classifies type "dataset" resources as datasets without requiring storage.topology', () => {
      const model = buildTrueNASPageModel([
        makeResource({ id: 'sys', type: 'agent' }),
        makeResource({ id: 'media', type: 'dataset' }),
      ]);
      expect(model.datasets.map((r) => r.id)).toEqual(['media']);
    });

    it('counts pools via storage.topology when type is "storage" and topology is "pool"', () => {
      const pool = makeResource({
        id: 'tank',
        type: 'storage',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const counts = buildTrueNASSystemChildCounts(
        [makeResource({ id: 'sys', type: 'agent' }), pool],
        [makeResource({ id: 'sys', type: 'agent' })],
      );
      expect(counts.get('sys')?.pools).toBe(1);
    });

    it('returns an empty map when no systems are provided', () => {
      const counts = buildTrueNASSystemChildCounts(
        [makeResource({ id: 'orphan-pool', type: 'pool' })],
        [],
      );
      expect(counts.size).toBe(0);
    });

    it('counts zero services when system has no truenas metadata at all', () => {
      const system = makeResource({ id: 'sys', type: 'agent' });
      const counts = buildTrueNASSystemChildCounts([system], [system]);
      expect(counts.get(system.id)?.services).toBe(0);
    });

    it('counts zero services when system truenas exists but services array is absent', () => {
      const system = makeResource({ id: 'sys', type: 'agent', truenas: { hostname: 'nas' } });
      const counts = buildTrueNASSystemChildCounts([system], [system]);
      expect(counts.get(system.id)?.services).toBe(0);
    });

    it('does not count unparented resources when multiple systems exist', () => {
      const sysA = makeResource({ id: 'sys-a', type: 'agent' });
      const sysB = makeResource({ id: 'sys-b', type: 'agent' });
      const orphanPool = makeResource({ id: 'orphan-pool', type: 'pool' });
      const counts = buildTrueNASSystemChildCounts([sysA, sysB, orphanPool], [sysA, sysB]);
      expect(counts.get(sysA.id)?.pools).toBe(0);
      expect(counts.get(sysB.id)?.pools).toBe(0);
    });

    it('does not attribute a non-agent resource whose id collides with the single-system id', () => {
      const system = makeResource({ id: 'sys', type: 'agent' });
      const collidingPool = makeResource({ id: 'sys', type: 'pool' });
      const counts = buildTrueNASSystemChildCounts([system, collidingPool], [system]);
      expect(counts.get(system.id)?.pools).toBe(0);
    });

    it('does not increment storage child counts for non-storage child types', () => {
      const pool = makeResource({
        id: 'pool-tank',
        type: 'pool',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const vm = makeResource({ id: 'vm-1', type: 'vm', parentId: pool.id });
      const counts = buildTrueNASStorageChildCounts([pool, vm]);
      expect(counts.get(pool.id)).toEqual({ datasets: 0, shares: 0, disks: 0 });
    });

    it('does not infer pool relationship when resource has no inferable pool name', () => {
      const pool = makeResource({
        id: 'pool-tank',
        type: 'pool',
        name: 'tank',
        storage: { topology: 'pool', platform: 'truenas', zfsPoolState: 'ONLINE' },
      });
      const blankShare = makeResource({
        id: 'blank-share',
        type: 'network-share',
        name: '   ',
        displayName: '   ',
      });
      const counts = buildTrueNASStorageChildCounts([pool, blankShare]);
      expect(counts.get(pool.id)).toEqual({ datasets: 0, shares: 0, disks: 0 });
    });
  });

  describe('numeric guards', () => {
    it('treats absent timestamps as zero when sorting protection points', () => {
      const a = makeRecoveryPoint({
        id: 'a-point',
        kind: 'snapshot',
        mode: 'snapshot',
        completedAt: null,
        startedAt: null,
      });
      const z = makeRecoveryPoint({
        id: 'z-point',
        kind: 'snapshot',
        mode: 'snapshot',
        completedAt: null,
        startedAt: null,
      });
      const sorted = sortTrueNASProtectionPoints([z, a]);
      expect(sorted.map((p) => p.id)).toEqual(['a-point', 'z-point']);
    });

    it('falls back to startedAt when completedAt is absent', () => {
      const older = makeRecoveryPoint({
        id: 'older',
        kind: 'snapshot',
        mode: 'snapshot',
        completedAt: null,
        startedAt: '2026-01-01T00:00:00Z',
      });
      const newer = makeRecoveryPoint({
        id: 'newer',
        kind: 'snapshot',
        mode: 'snapshot',
        completedAt: null,
        startedAt: '2026-06-01T00:00:00Z',
      });
      expect(sortTrueNASProtectionPoints([older, newer]).map((p) => p.id)).toEqual([
        'newer',
        'older',
      ]);
    });

    it('treats an invalid date string as NaN and falls back to zero timestamp', () => {
      const invalid = makeRecoveryPoint({
        id: 'invalid',
        kind: 'snapshot',
        mode: 'snapshot',
        completedAt: 'not-a-real-date',
        startedAt: null,
      });
      const valid = makeRecoveryPoint({
        id: 'valid',
        kind: 'snapshot',
        mode: 'snapshot',
        completedAt: '2026-03-01T00:00:00Z',
        startedAt: '2026-03-01T00:00:00Z',
      });
      expect(sortTrueNASProtectionPoints([invalid, valid]).map((p) => p.id)).toEqual([
        'valid',
        'invalid',
      ]);
    });

    it('breaks ties by label when timestamps are equal', () => {
      const same = '2026-05-20T00:00:00Z';
      const z = makeRecoveryPoint({
        id: 'z-point',
        kind: 'snapshot',
        mode: 'snapshot',
        completedAt: same,
        startedAt: same,
      });
      const a = makeRecoveryPoint({
        id: 'a-point',
        kind: 'snapshot',
        mode: 'snapshot',
        completedAt: same,
        startedAt: same,
      });
      expect(sortTrueNASProtectionPoints([z, a]).map((p) => p.id)).toEqual(['a-point', 'z-point']);
    });
  });

  describe('replication status ranking', () => {
    it('maps "ok" outcome to the success bucket', () => {
      expect(
        mapTrueNASProtectionStatus(
          makeRecoveryPoint({ id: 'p', kind: 'snapshot', mode: 'snapshot', outcome: 'ok' }),
        ),
      ).toBe('success');
    });

    it('maps "warn" outcome to the warning bucket', () => {
      expect(
        mapTrueNASProtectionStatus(
          makeRecoveryPoint({ id: 'p', kind: 'snapshot', mode: 'snapshot', outcome: 'warn' }),
        ),
      ).toBe('warning');
    });

    it('maps "failed" outcome to the failed bucket', () => {
      expect(
        mapTrueNASProtectionStatus(
          makeRecoveryPoint({ id: 'p', kind: 'snapshot', mode: 'snapshot', outcome: 'failed' }),
        ),
      ).toBe('failed');
    });

    it('maps "failure" outcome to the failed bucket', () => {
      expect(
        mapTrueNASProtectionStatus(
          makeRecoveryPoint({ id: 'p', kind: 'snapshot', mode: 'snapshot', outcome: 'failure' }),
        ),
      ).toBe('failed');
    });

    it('maps "error" outcome to the failed bucket', () => {
      expect(
        mapTrueNASProtectionStatus(
          makeRecoveryPoint({ id: 'p', kind: 'snapshot', mode: 'snapshot', outcome: 'error' }),
        ),
      ).toBe('failed');
    });

    it('maps an unrecognized outcome string to the unknown bucket', () => {
      expect(
        mapTrueNASProtectionStatus(
          makeRecoveryPoint({ id: 'p', kind: 'snapshot', mode: 'snapshot', outcome: 'pending' }),
        ),
      ).toBe('unknown');
    });

    it('tallies failed and unknown outcomes in the protection posture', () => {
      const posture = buildTrueNASProtectionPosture([
        makeRecoveryPoint({ id: 'ok', kind: 'snapshot', mode: 'snapshot', outcome: 'success' }),
        makeRecoveryPoint({ id: 'fail', kind: 'snapshot', mode: 'snapshot', outcome: 'failed' }),
        makeRecoveryPoint({
          id: 'mystery',
          kind: 'snapshot',
          mode: 'snapshot',
          outcome: 'pending',
        }),
      ]);
      expect(posture).toEqual({
        healthy: 1,
        warning: 0,
        failed: 1,
        running: 0,
        unknown: 1,
        attention: 1,
      });
    });

    it('returns a fully zeroed posture for an empty recovery-point array', () => {
      expect(buildTrueNASProtectionPosture([])).toEqual({
        healthy: 0,
        warning: 0,
        failed: 0,
        running: 0,
        unknown: 0,
        attention: 0,
      });
    });

    it('classifies as replication when only details.taskId is present', () => {
      expect(
        mapTrueNASProtectionKind(
          makeRecoveryPoint({
            id: 'p',
            kind: 'other',
            mode: 'local',
            details: { taskId: 'rep-task-42' },
          }),
        ),
      ).toBe('replication');
    });

    it('classifies as replication when only details.targetDataset is present', () => {
      expect(
        mapTrueNASProtectionKind(
          makeRecoveryPoint({
            id: 'p',
            kind: 'other',
            mode: 'local',
            details: { targetDataset: 'offsite/backup' },
          }),
        ),
      ).toBe('replication');
    });

    it('classifies as other when details is null and no replication signals exist', () => {
      expect(
        mapTrueNASProtectionKind(
          makeRecoveryPoint({
            id: 'p',
            kind: 'other',
            mode: 'local',
            details: null,
          }),
        ),
      ).toBe('other');
    });

    it('classifies as other when details has no replication fields', () => {
      expect(
        mapTrueNASProtectionKind(
          makeRecoveryPoint({
            id: 'p',
            kind: 'other',
            mode: 'local',
            details: { snapshot: 'auto-20260101' },
          }),
        ),
      ).toBe('other');
    });
  });

  describe('dataset-child merge without platformData.truenas', () => {
    it('places orphaned datasets with no pool or dataset parent at root level', () => {
      const orphan = makeResource({
        id: 'ds-orphan',
        type: 'dataset',
        name: 'lonely/data',
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/lonely/data' },
      });
      const rows = buildTrueNASStorageTopologyRows([orphan]);
      expect(rows.map((r) => [r.id, r.depth, r.parentRowId])).toEqual([
        ['dataset:ds-orphan', 0, undefined],
      ]);
    });

    it('renders disks not under any pool at depth 0', () => {
      const disk = makeResource({
        id: 'loose-disk',
        type: 'physical_disk',
        name: 'sda',
        physicalDisk: { devPath: '/dev/sda', serial: 'x' },
      });
      const rows = buildTrueNASStorageTopologyRows([disk]);
      expect(rows.map((r) => [r.id, r.depth, r.parentRowId])).toEqual([
        ['disk:loose-disk', 0, undefined],
      ]);
    });

    it('infers dataset-share relationship from storage path when truenas metadata is absent', () => {
      const dataset = makeResource({
        id: 'ds-media',
        type: 'dataset',
        name: 'tank/media',
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
      });
      const share = makeResource({
        id: 'share-media',
        type: 'network-share',
        name: 'share-media',
        storage: { path: '/mnt/tank/media' },
      });
      const counts = buildTrueNASStorageChildCounts([dataset, share]);
      expect(counts.get(dataset.id)).toEqual({ datasets: 0, shares: 1, disks: 0 });
    });

    it('nests child datasets under explicit parentId even without truenas metadata', () => {
      const parent = makeResource({
        id: 'ds-parent',
        type: 'dataset',
        name: 'tank/media',
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
      });
      const child = makeResource({
        id: 'ds-child',
        type: 'dataset',
        name: 'tank/media/photos',
        parentId: parent.id,
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media/photos' },
      });
      const rows = buildTrueNASStorageTopologyRows([parent, child]);
      expect(rows.map((r) => [r.id, r.depth, r.parentRowId])).toEqual([
        ['dataset:ds-parent', 0, undefined],
        ['dataset:ds-child', 1, 'dataset:ds-parent'],
      ]);
    });

    it('nests datasets by storage-path inference when parentId is absent', () => {
      const parent = makeResource({
        id: 'ds-tank',
        type: 'dataset',
        name: 'tank',
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank' },
      });
      const child = makeResource({
        id: 'ds-tank-media',
        type: 'dataset',
        name: 'tank/media',
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
      });
      const rows = buildTrueNASStorageTopologyRows([parent, child]);
      expect(rows.map((r) => [r.id, r.depth, r.parentRowId])).toEqual([
        ['dataset:ds-tank', 0, undefined],
        ['dataset:ds-tank-media', 1, 'dataset:ds-tank'],
      ]);
    });

    it('attributes orphaned disks to the sole pool via single-pool fallback', () => {
      const pool = makeResource({
        id: 'only-pool',
        type: 'pool',
        name: 'tank',
        storage: { topology: 'pool', platform: 'truenas', zfsPoolState: 'ONLINE' },
      });
      const disk = makeResource({
        id: 'loose-disk',
        type: 'physical_disk',
        name: 'sda',
        physicalDisk: { devPath: '/dev/sda', serial: 'x' },
      });
      const rows = buildTrueNASStorageTopologyRows([pool, disk]);
      expect(rows.map((r) => [r.id, r.depth, r.parentRowId])).toEqual([
        ['pool:only-pool', 0, undefined],
        ['disk:loose-disk', 1, 'pool:only-pool'],
      ]);
    });

    it('does not nest datasets under candidates from a different pool', () => {
      const poolA = makeResource({
        id: 'pool-a',
        type: 'pool',
        name: 'alpha',
        storage: { topology: 'pool', platform: 'truenas', zfsPoolState: 'ONLINE' },
      });
      const poolB = makeResource({
        id: 'pool-b',
        type: 'pool',
        name: 'beta',
        storage: { topology: 'pool', platform: 'truenas', zfsPoolState: 'ONLINE' },
      });
      const datasetB = makeResource({
        id: 'ds-b',
        type: 'dataset',
        name: 'alpha/media',
        storage: {
          topology: 'dataset',
          platform: 'truenas',
          pool: 'beta',
          path: '/mnt/alpha/media',
        },
      });
      const datasetA = makeResource({
        id: 'ds-a',
        type: 'dataset',
        name: 'alpha/media/photos',
        storage: {
          topology: 'dataset',
          platform: 'truenas',
          path: '/mnt/alpha/media/photos',
        },
      });
      const rows = buildTrueNASStorageTopologyRows([poolA, poolB, datasetB, datasetA]);
      const aRow = rows.find((r) => r.id === 'dataset:ds-a');
      expect(aRow?.parentRowId).not.toBe('dataset:ds-b');
      const bRow = rows.find((r) => r.id === 'dataset:ds-b');
      expect(bRow?.parentRowId).not.toBe('dataset:ds-b');
    });

    it('returns empty storage child counts for a pool when no children match', () => {
      const pool = makeResource({
        id: 'pool-tank',
        type: 'pool',
        name: 'tank',
        storage: { topology: 'pool', platform: 'truenas', zfsPoolState: 'ONLINE' },
      });
      const unrelatedShare = makeResource({
        id: 'share-other',
        type: 'network-share',
        name: 'share-other',
        storage: { path: '/mnt/other/data' },
      });
      const counts = buildTrueNASStorageChildCounts([pool, unrelatedShare]);
      expect(counts.get(pool.id)).toEqual({ datasets: 0, shares: 0, disks: 0 });
    });
  });
});
