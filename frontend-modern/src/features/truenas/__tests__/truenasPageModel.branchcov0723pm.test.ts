import { describe, expect, it } from 'vitest';
import type { RecoveryPoint } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import {
  buildTrueNASIncidentRows,
  buildTrueNASServiceRows,
  buildTrueNASStorageTopologyRows,
  filterTrueNASApps,
  filterTrueNASIncidents,
  filterTrueNASProtectionPoints,
  filterTrueNASShares,
  filterTrueNASStorageTopologyRows,
  filterTrueNASVMs,
  sortTrueNASProtectionPoints,
} from '../truenasPageModel';

// Branch-coverage completion: these specs target the residual fallback arms
// left uncovered by the happy-path suites. Every assertion proves a specific
// branch of the named function returns a concrete observable value.

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

describe('truenasPageModel branchcov0723pm', () => {
  describe('compareStorageResources tiebreakers (via buildTrueNASStorageTopologyRows sort)', () => {
    // compareStorageResources: rank -> displayName -> parentName -> id.
    // The existing suite only sorted pools with distinct status ranks, so the
    // name/parent/id tiebreak arms were never reached.

    it('breaks a rank tie by display-name localeCompare (nameDelta !== 0 return)', () => {
      const tank = makeResource({
        id: 'pool-tank',
        type: 'pool',
        name: 'tank',
        displayName: 'tank',
        status: 'online',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const archive = makeResource({
        id: 'pool-archive',
        type: 'pool',
        name: 'archive',
        displayName: 'archive',
        status: 'online',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const rows = buildTrueNASStorageTopologyRows([tank, archive]);
      // Both healthy (rank 3); 'archive' < 'tank' so archive sorts first.
      expect(rows.map((row) => row.id)).toEqual(['pool:pool-archive', 'pool:pool-tank']);
    });

    it('breaks a name tie by parentName localeCompare (parentDelta !== 0 return)', () => {
      const onNasB = makeResource({
        id: 'pool-on-b',
        type: 'pool',
        name: 'tank',
        displayName: 'tank',
        parentName: 'nas-b',
        status: 'online',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const onNasA = makeResource({
        id: 'pool-on-a',
        type: 'pool',
        name: 'tank',
        displayName: 'tank',
        parentName: 'nas-a',
        status: 'online',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const rows = buildTrueNASStorageTopologyRows([onNasB, onNasA]);
      // Same rank, same name; 'nas-a' < 'nas-b' so pool-on-a sorts first.
      expect(rows.map((row) => row.id)).toEqual(['pool:pool-on-a', 'pool:pool-on-b']);
    });

    it('falls back to "" via ?? when parentName is blank, then breaks the tie by id', () => {
      const e = makeResource({
        id: 'pool-e',
        type: 'pool',
        name: 'tank',
        displayName: 'tank',
        status: 'online',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const f = makeResource({
        id: 'pool-f',
        type: 'pool',
        name: 'tank',
        displayName: 'tank',
        status: 'online',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const rows = buildTrueNASStorageTopologyRows([f, e]);
      // parentName absent -> asTrimmedString -> ?? '' -> parentDelta 0 -> id
      // tiebreak: 'pool-e' < 'pool-f'.
      expect(rows.map((row) => row.id)).toEqual(['pool:pool-e', 'pool:pool-f']);
    });
  });

  describe('compareServiceRows tiebreakers + serviceStatusRank attention (via buildTrueNASServiceRows)', () => {
    it('hits the attention case of serviceStatusRank and orders it before running', () => {
      // webdav FAILED -> attention (rank 0); smb RUNNING -> running (rank 3).
      const system = makeResource({
        id: 'sys',
        type: 'agent',
        displayName: 'sys',
        truenas: {
          services: [
            { id: '1', service: 'smb', enabled: true, state: 'RUNNING' },
            { id: '2', service: 'webdav', enabled: true, state: 'FAILED' },
          ],
        },
      });
      const rows = buildTrueNASServiceRows([system]);
      // attention (0) sorts before running (3).
      expect(rows.map((row) => row.service.service)).toEqual(['webdav', 'smb']);
    });

    it('breaks a rank tie by systemName localeCompare (systemDelta !== 0 return)', () => {
      const nasB = makeResource({
        id: 'sys-b',
        type: 'agent',
        name: 'nas-b',
        displayName: 'nas-b',
        truenas: { services: [{ id: '1', service: 'smb', enabled: true, state: 'RUNNING' }] },
      });
      const nasA = makeResource({
        id: 'sys-a',
        type: 'agent',
        name: 'nas-a',
        displayName: 'nas-a',
        truenas: { services: [{ id: '1', service: 'nfs', enabled: true, state: 'RUNNING' }] },
      });
      const rows = buildTrueNASServiceRows([nasB, nasA]);
      // Both running (rank 3); 'nas-a' < 'nas-b' so nfs sorts first.
      expect(rows.map((row) => row.service.service)).toEqual(['nfs', 'smb']);
    });

    it('breaks a systemName tie by serviceDisplayName localeCompare', () => {
      const system = makeResource({
        id: 'sys',
        type: 'agent',
        displayName: 'sys',
        truenas: {
          services: [
            { id: '1', service: 'smb', enabled: true, state: 'RUNNING' },
            { id: '2', service: 'nfs', enabled: true, state: 'RUNNING' },
          ],
        },
      });
      const rows = buildTrueNASServiceRows([system]);
      // Same rank, same system; 'nfs' < 'smb'.
      expect(rows.map((row) => row.service.service)).toEqual(['nfs', 'smb']);
    });
  });

  describe('owningDatasetId parent-walk short-circuit (via buildTrueNASStorageTopologyRows)', () => {
    it('short-circuits the parent && isTrueNASPoolResource check when parentId is not a storage row', () => {
      // The dataset's parentId points at an agent, which is absent from the
      // storage-only resourceById map, so resourceById.get(...) is undefined
      // and both `parent && ...` guards short-circuit. The dataset lands at
      // the root because no pool/dataset ancestor is resolvable.
      const system = makeResource({ id: 'sys', type: 'agent' });
      const dataset = makeResource({
        id: 'ds-rootless',
        type: 'dataset',
        name: 'tank/media',
        parentId: system.id,
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
      });
      const rows = buildTrueNASStorageTopologyRows([system, dataset]);
      expect(rows.map((r) => [r.id, r.depth, r.parentRowId])).toEqual([
        ['dataset:ds-rootless', 0, undefined],
      ]);
    });

    it('continues the parent walk through a disk when the dataset parentId is a disk', () => {
      // A disk is in the storage resourceById map but is neither pool nor
      // dataset, so owningDatasetId must not short-circuit on it: it reads
      // parent?.parentId (the defined arm) and walks on to the pool. The
      // dataset ultimately nests under the pool that owns the disk.
      const pool = makeResource({
        id: 'pool-tank',
        type: 'pool',
        name: 'tank',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const disk = makeResource({
        id: 'disk-sda',
        type: 'physical_disk',
        name: 'sda',
        parentId: pool.id,
        physicalDisk: { devPath: '/dev/sda', serial: 'x' },
      });
      const dataset = makeResource({
        id: 'ds-x',
        type: 'dataset',
        name: 'tank/x',
        parentId: disk.id,
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/x' },
      });
      const rows = buildTrueNASStorageTopologyRows([pool, disk, dataset]);
      expect(rows.map((r) => [r.id, r.depth, r.parentRowId])).toEqual([
        ['pool:pool-tank', 0, undefined],
        ['dataset:ds-x', 1, 'pool:pool-tank'],
        ['disk:disk-sda', 1, 'pool:pool-tank'],
      ]);
    });
  });

  describe('sortTrueNASProtectionPoints label-fallback chain arms', () => {
    // Each operand of the `||` chain must be the resolving label for both
    // sides at least once. Equal timestamps force the label tiebreak.

    it('resolves both labels via display.subjectLabel (itemLabel absent)', () => {
      const z = makeRecoveryPoint({
        id: 'z-id',
        kind: 'snapshot',
        mode: 'snapshot',
        display: { subjectLabel: 'zzz' },
      });
      const a = makeRecoveryPoint({
        id: 'a-id',
        kind: 'snapshot',
        mode: 'snapshot',
        display: { subjectLabel: 'aaa' },
      });
      expect(sortTrueNASProtectionPoints([z, a]).map((p) => p.id)).toEqual(['a-id', 'z-id']);
    });

    it('resolves both labels via itemRef.name (display absent)', () => {
      const z = makeRecoveryPoint({
        id: 'z-id',
        kind: 'snapshot',
        mode: 'snapshot',
        itemRef: { type: 'truenas-dataset', name: 'zzz' },
      });
      const a = makeRecoveryPoint({
        id: 'a-id',
        kind: 'snapshot',
        mode: 'snapshot',
        itemRef: { type: 'truenas-dataset', name: 'aaa' },
      });
      expect(sortTrueNASProtectionPoints([z, a]).map((p) => p.id)).toEqual(['a-id', 'z-id']);
    });

    it('resolves both labels via subjectRef.name (display and itemRef absent)', () => {
      const z = makeRecoveryPoint({
        id: 'z-id',
        kind: 'snapshot',
        mode: 'snapshot',
        subjectRef: { type: 'truenas-dataset', name: 'zzz' },
      });
      const a = makeRecoveryPoint({
        id: 'a-id',
        kind: 'snapshot',
        mode: 'snapshot',
        subjectRef: { type: 'truenas-dataset', name: 'aaa' },
      });
      expect(sortTrueNASProtectionPoints([z, a]).map((p) => p.id)).toEqual(['a-id', 'z-id']);
    });
  });

  describe('trueNASProtectionSearchTokens subjectRef optional-chaining arms (via filterTrueNASProtectionPoints)', () => {
    it('matches by subjectRef namespace, uid, id, and name when subjectRef is fully populated', () => {
      const point = makeRecoveryPoint({
        id: 'p',
        kind: 'snapshot',
        mode: 'snapshot',
        subjectRef: {
          type: 'truenas-dataset',
          namespace: 'subj-ns-token',
          name: 'subj-name-token',
          uid: 'subj-uid-token',
          id: 'subj-id-token',
        },
      });
      expect(
        filterTrueNASProtectionPoints([point], 'subj-ns-token', 'all').map((p) => p.id),
      ).toEqual(['p']);
      expect(
        filterTrueNASProtectionPoints([point], 'subj-uid-token', 'all').map((p) => p.id),
      ).toEqual(['p']);
      expect(
        filterTrueNASProtectionPoints([point], 'subj-id-token', 'all').map((p) => p.id),
      ).toEqual(['p']);
      expect(
        filterTrueNASProtectionPoints([point], 'subj-name-token', 'all').map((p) => p.id),
      ).toEqual(['p']);
    });
  });

  describe('matchesTrueNASStorageSearch metricsTarget optional-chaining (via filterTrueNASStorageTopologyRows)', () => {
    it('matches a storage row by metricsTarget.resourceId', () => {
      const pool = makeResource({
        id: 'tank',
        type: 'pool',
        storage: { topology: 'pool', platform: 'truenas' },
        metricsTarget: { resourceType: 'storage', resourceId: 'metrics-target-token' },
      });
      const rows = buildTrueNASStorageTopologyRows([pool]);
      expect(
        filterTrueNASStorageTopologyRows(rows, 'metrics-target-token', 'all').map((r) => r.id),
      ).toEqual(['pool:tank']);
    });
  });

  describe('buildIncidentRow severity fallback to "info"', () => {
    it('defaults severity to info when incident and resource severity are both blank', () => {
      // Non-blank summary keeps the incident past hasIncidentSignal; both
      // severity sources blank forces the final || 'info' arm.
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        incidents: [{ code: 'truenas_x', severity: '   ', summary: 'something happened' }],
      });
      const row = buildTrueNASIncidentRows([resource])[0];
      expect(row?.severity).toBe('info');
      expect(row?.severityBucket).toBe('info');
    });
  });

  describe('buildTrueNASIncidentRows sort priority tiebreak', () => {
    it('breaks a severity-rank tie by priority descending (priorityDelta !== 0 return)', () => {
      // Both critical (rank 3). Higher priority sorts first even though its
      // resource name would sort later alphabetically, proving the priority
      // comparator decides the order rather than the name fallback.
      const highPriorityLateName = makeResource({
        id: 'r-z',
        type: 'pool',
        incidentPriority: 100,
        incidents: [{ code: 'cz', severity: 'critical', summary: 'sz' }],
      });
      const lowPriorityEarlyName = makeResource({
        id: 'r-a',
        type: 'pool',
        incidentPriority: 50,
        incidents: [{ code: 'ca', severity: 'critical', summary: 'sa' }],
      });
      const rows = buildTrueNASIncidentRows([highPriorityLateName, lowPriorityEarlyName]);
      expect(rows.map((row) => row.resourceId)).toEqual(['r-z', 'r-a']);
    });
  });

  describe('filter empty-needle early-return arms', () => {
    it('filterTrueNASApps keeps every app when the needle is blank', () => {
      const a = makeResource({ id: 'a', type: 'app-container' });
      const b = makeResource({ id: 'b', type: 'app-container' });
      expect(filterTrueNASApps([a, b], '', 'all').map((app) => app.id)).toEqual(['a', 'b']);
    });

    it('filterTrueNASVMs keeps every vm when the needle is blank', () => {
      const a = makeResource({ id: 'a', type: 'vm' });
      const b = makeResource({ id: 'b', type: 'vm' });
      expect(filterTrueNASVMs([a, b], '', 'all').map((vm) => vm.id)).toEqual(['a', 'b']);
    });

    it('filterTrueNASShares keeps every share when the needle is blank', () => {
      const a = makeResource({ id: 'a', type: 'network-share' });
      const b = makeResource({ id: 'b', type: 'network-share' });
      expect(filterTrueNASShares([a, b], '', 'all').map((share) => share.id)).toEqual(['a', 'b']);
    });

    it('filterTrueNASIncidents keeps every row when the needle is blank', () => {
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        incidents: [{ code: 'truenas_x', severity: 'info', summary: 's' }],
      });
      const rows = buildTrueNASIncidentRows([resource]);
      expect(filterTrueNASIncidents(rows, '', 'all')).toHaveLength(rows.length);
    });
  });

  describe('incidentSearchHaystack truenas.hostname optional-chaining (via filterTrueNASIncidents)', () => {
    it('matches an incident row by the resource truenas.hostname', () => {
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        truenas: { hostname: 'incident-host-token' },
        incidents: [{ code: 'truenas_x', severity: 'info', summary: 's' }],
      });
      const rows = buildTrueNASIncidentRows([resource]);
      expect(
        filterTrueNASIncidents(rows, 'incident-host-token', 'all').map((row) => row.resourceId),
      ).toEqual(['r']);
    });
  });
});
