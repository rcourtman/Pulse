import { describe, expect, it } from 'vitest';
import type { RecoveryPoint } from '@/types/recovery';
import type { Resource, ResourceTrueNASServiceMeta } from '@/types/resource';
import {
  buildTrueNASIncidentRows,
  buildTrueNASServiceRows,
  buildTrueNASStorageChildCounts,
  buildTrueNASStorageTopologyRows,
  buildTrueNASSystemChildCounts,
  filterTrueNASApps,
  filterTrueNASProtectionPoints,
  filterTrueNASShares,
  filterTrueNASStorageTopologyRows,
  mapTrueNASAppStatus,
  mapTrueNASServiceStatus,
  mapTrueNASShareStatus,
  mapTrueNASStorageStatus,
  mapTrueNASVMStatus,
  sortTrueNASProtectionPoints,
} from '../truenasPageModel';
import type { TrueNASServiceRow } from '../truenasPageModel';

// Several target functions are module-private (not exported) and some are
// nested closures. They are exercised here through their public callers; every
// assertion is written to prove a specific branch of the named function.

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

const makeServiceRow = (
  overrides: Partial<{ system: Resource; service: Partial<ResourceTrueNASServiceMeta> }>,
): TrueNASServiceRow => {
  const system = overrides.system ?? makeResource({ id: 'sys', type: 'agent' });
  const service: ResourceTrueNASServiceMeta = {
    id: 'svc-1',
    service: 'smb',
    enabled: true,
    state: 'RUNNING',
    ...overrides.service,
  };
  return {
    id: `${system.id}:service:${service.id ?? service.service ?? 'unknown'}`,
    system,
    systemId: system.id,
    systemName: system.displayName,
    service,
  };
};

// platformData.sourceStatus.truenas.status set to a non-connected value makes
// hasImpairedResourceSource(...) return true.
const impairedSource = (): Resource['platformData'] => ({
  sourceStatus: { truenas: { status: 'offline' } },
});

describe('truenasPageModel coverage2', () => {
  describe('mapTrueNASVMStatus', () => {
    it('maps the "active" vm state to running', () => {
      const vm = makeResource({ id: 'vm', type: 'vm', truenas: { vm: { state: 'ACTIVE' } } });
      expect(mapTrueNASVMStatus(vm)).toBe('running');
    });

    it('maps shutoff/shutdown/poweroff vm states to stopped', () => {
      expect(
        mapTrueNASVMStatus(makeResource({ id: 'a', type: 'vm', truenas: { vm: { state: 'SHUTOFF' } } })),
      ).toBe('stopped');
      expect(
        mapTrueNASVMStatus(makeResource({ id: 'b', type: 'vm', truenas: { vm: { state: 'shutdown' } } })),
      ).toBe('stopped');
      expect(
        mapTrueNASVMStatus(makeResource({ id: 'c', type: 'vm', truenas: { vm: { state: 'POWEROFF' } } })),
      ).toBe('stopped');
    });

    it('maps paused/suspended/error/crashed vm states to attention', () => {
      expect(
        mapTrueNASVMStatus(makeResource({ id: 'a', type: 'vm', truenas: { vm: { state: 'PAUSED' } } })),
      ).toBe('attention');
      expect(
        mapTrueNASVMStatus(
          makeResource({ id: 'b', type: 'vm', truenas: { vm: { state: 'suspended' } } }),
        ),
      ).toBe('attention');
      expect(
        mapTrueNASVMStatus(makeResource({ id: 'c', type: 'vm', truenas: { vm: { state: 'error' } } })),
      ).toBe('attention');
      expect(
        mapTrueNASVMStatus(
          makeResource({ id: 'd', type: 'vm', truenas: { vm: { state: 'crashed' } } }),
        ),
      ).toBe('attention');
    });

    it('falls back to domainState when vm.state is absent', () => {
      const vm = makeResource({
        id: 'vm',
        type: 'vm',
        truenas: { vm: { domainState: 'RUNNING' } },
      });
      expect(mapTrueNASVMStatus(vm)).toBe('running');
    });

    it('falls back to resource.status running/online when no vm metadata is present', () => {
      expect(mapTrueNASVMStatus(makeResource({ id: 'vm', type: 'vm', status: 'online' }))).toBe(
        'running',
      );
      expect(mapTrueNASVMStatus(makeResource({ id: 'vm2', type: 'vm', status: 'running' }))).toBe(
        'running',
      );
    });

    it('falls back to resource.status stopped/offline when no vm metadata is present', () => {
      expect(mapTrueNASVMStatus(makeResource({ id: 'vm', type: 'vm', status: 'offline' }))).toBe(
        'stopped',
      );
      expect(mapTrueNASVMStatus(makeResource({ id: 'vm2', type: 'vm', status: 'stopped' }))).toBe(
        'stopped',
      );
    });

    it('returns attention when no vm metadata and status is unrecognized', () => {
      expect(mapTrueNASVMStatus(makeResource({ id: 'vm', type: 'vm', status: 'unknown' }))).toBe(
        'attention',
      );
    });
  });

  describe('mapTrueNASAppStatus', () => {
    it('maps crashed/deploying/stopping app states to attention', () => {
      expect(
        mapTrueNASAppStatus(
          makeResource({ id: 'a', type: 'app-container', truenas: { app: { state: 'CRASHED' } } }),
        ),
      ).toBe('attention');
      expect(
        mapTrueNASAppStatus(
          makeResource({ id: 'b', type: 'app-container', truenas: { app: { state: 'deploying' } } }),
        ),
      ).toBe('attention');
      expect(
        mapTrueNASAppStatus(
          makeResource({ id: 'c', type: 'app-container', truenas: { app: { state: 'STOPPING' } } }),
        ),
      ).toBe('attention');
    });

    it('falls back to resource.status running when no app state is present', () => {
      expect(
        mapTrueNASAppStatus(makeResource({ id: 'a', type: 'app-container', status: 'online' })),
      ).toBe('running');
    });

    it('falls back to resource.status stopped when no app state is present', () => {
      expect(
        mapTrueNASAppStatus(makeResource({ id: 'a', type: 'app-container', status: 'stopped' })),
      ).toBe('stopped');
    });

    it('maps degraded/paused resource status to attention when no app state is present', () => {
      expect(
        mapTrueNASAppStatus(makeResource({ id: 'a', type: 'app-container', status: 'paused' })),
      ).toBe('attention');
      expect(
        mapTrueNASAppStatus(makeResource({ id: 'b', type: 'app-container', status: 'degraded' })),
      ).toBe('attention');
    });

    it('returns attention for an unrecognized status with no app state', () => {
      expect(
        mapTrueNASAppStatus(makeResource({ id: 'a', type: 'app-container', status: 'unknown' })),
      ).toBe('attention');
    });
  });

  describe('mapTrueNASShareStatus', () => {
    it('marks a locked share as attention', () => {
      const share = makeResource({
        id: 'share',
        type: 'network-share',
        truenas: { share: { enabled: true, locked: true } },
      });
      expect(mapTrueNASShareStatus(share)).toBe('attention');
    });

    it('marks offline/stopped resource status as disabled even without share metadata', () => {
      expect(
        mapTrueNASShareStatus(
          makeResource({ id: 'a', type: 'network-share', status: 'offline' }),
        ),
      ).toBe('disabled');
      expect(
        mapTrueNASShareStatus(
          makeResource({ id: 'b', type: 'network-share', status: 'stopped' }),
        ),
      ).toBe('disabled');
    });

    it('marks degraded/paused resource status as attention', () => {
      expect(
        mapTrueNASShareStatus(
          makeResource({ id: 'a', type: 'network-share', status: 'degraded' }),
        ),
      ).toBe('attention');
      expect(
        mapTrueNASShareStatus(
          makeResource({ id: 'b', type: 'network-share', status: 'paused' }),
        ),
      ).toBe('attention');
    });

    it('marks online/running resource status as active when share.enabled is absent', () => {
      expect(
        mapTrueNASShareStatus(
          makeResource({ id: 'a', type: 'network-share', status: 'online' }),
        ),
      ).toBe('active');
      expect(
        mapTrueNASShareStatus(
          makeResource({ id: 'b', type: 'network-share', status: 'running' }),
        ),
      ).toBe('active');
    });

    it('returns attention for an unrecognized status with no share metadata', () => {
      expect(
        mapTrueNASShareStatus(
          makeResource({ id: 'a', type: 'network-share', status: 'unknown' }),
        ),
      ).toBe('attention');
    });
  });

  describe('mapTrueNASServiceStatus', () => {
    it('maps started/active service states to running', () => {
      expect(mapTrueNASServiceStatus(makeServiceRow({ service: { state: 'STARTED' } }))).toBe(
        'running',
      );
      expect(mapTrueNASServiceStatus(makeServiceRow({ service: { state: 'active' } }))).toBe(
        'running',
      );
    });

    it('maps failed/error/crashed/degraded/unknown service states to attention', () => {
      for (const state of ['FAILED', 'error', 'crashed', 'degraded', 'unknown']) {
        expect(mapTrueNASServiceStatus(makeServiceRow({ service: { state } }))).toBe('attention');
      }
    });

    it('maps stop/inactive service states to stopped when enabled is not false', () => {
      expect(mapTrueNASServiceStatus(makeServiceRow({ service: { state: 'STOP', enabled: true } }))).toBe(
        'stopped',
      );
      expect(
        mapTrueNASServiceStatus(makeServiceRow({ service: { state: 'inactive', enabled: undefined } })),
      ).toBe('stopped');
    });

    it('returns disabled when enabled is false and state is unrecognized', () => {
      expect(
        mapTrueNASServiceStatus(makeServiceRow({ service: { state: 'weird', enabled: false } })),
      ).toBe('disabled');
    });

    it('returns disabled for a stopped service with enabled false', () => {
      expect(
        mapTrueNASServiceStatus(makeServiceRow({ service: { state: 'stopped', enabled: false } })),
      ).toBe('disabled');
    });

    it('returns attention when no state and enabled is not false', () => {
      expect(
        mapTrueNASServiceStatus(makeServiceRow({ service: { state: undefined, enabled: true } })),
      ).toBe('attention');
    });
  });

  describe('mapTrueNASStorageStatus', () => {
    it('maps warning/degraded/faulted/critical/failed/failure/paused statuses to attention', () => {
      for (const status of [
        'warning',
        'degraded',
        'faulted',
        'critical',
        'failed',
        'failure',
        'paused',
      ]) {
        expect(
          mapTrueNASStorageStatus(makeResource({ id: 's', type: 'pool', status: status as Resource['status'] })),
        ).toBe('attention');
      }
    });

    it('maps offline/stopped statuses to offline', () => {
      expect(
        mapTrueNASStorageStatus(makeResource({ id: 'a', type: 'pool', status: 'offline' })),
      ).toBe('offline');
      expect(
        mapTrueNASStorageStatus(makeResource({ id: 'b', type: 'pool', status: 'stopped' })),
      ).toBe('offline');
    });

    it('maps online/running/healthy statuses to healthy', () => {
      expect(mapTrueNASStorageStatus(makeResource({ id: 'a', type: 'pool', status: 'online' }))).toBe(
        'healthy',
      );
      expect(
        mapTrueNASStorageStatus(makeResource({ id: 'b', type: 'pool', status: 'running' })),
      ).toBe('healthy');
      expect(
        mapTrueNASStorageStatus(
          makeResource({ id: 'c', type: 'pool', status: 'healthy' as Resource['status'] }),
        ),
      ).toBe('healthy');
    });

    it('returns unknown when no status or zfs/disk signals are present', () => {
      expect(mapTrueNASStorageStatus(makeResource({ id: 'a', type: 'pool', status: 'unknown' }))).toBe(
        'unknown',
      );
    });

    it('treats healthy zfsPoolState and passed disk health as not attention', () => {
      expect(
        mapTrueNASStorageStatus(
          makeResource({
            id: 'pool',
            type: 'pool',
            status: 'unknown',
            storage: { zfsPoolState: 'ONLINE' },
          }),
        ),
      ).toBe('unknown');
      expect(
        mapTrueNASStorageStatus(
          makeResource({
            id: 'disk',
            type: 'physical_disk',
            status: 'unknown',
            physicalDisk: { health: 'PASSED' },
          }),
        ),
      ).toBe('unknown');
    });

    it('marks attention for an unhealthy zfsPoolState that is not offline/online', () => {
      expect(
        mapTrueNASStorageStatus(
          makeResource({
            id: 'pool',
            type: 'pool',
            status: 'unknown',
            storage: { zfsPoolState: 'SUSPENDED' },
          }),
        ),
      ).toBe('attention');
    });
  });

  describe('storageStatusRank ordering (via buildTrueNASStorageTopologyRows sort)', () => {
    it('orders topology rows attention < offline < unknown < healthy', () => {
      const attentionPool = makeResource({
        id: 'pool-attention',
        type: 'pool',
        name: 'zzz-attention',
        status: 'degraded',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const offlinePool = makeResource({
        id: 'pool-offline',
        type: 'pool',
        name: 'yyy-offline',
        status: 'offline',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const unknownPool = makeResource({
        id: 'pool-unknown',
        type: 'pool',
        name: 'xxx-unknown',
        status: 'unknown',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const healthyPool = makeResource({
        id: 'pool-healthy',
        type: 'pool',
        name: 'aaa-healthy',
        status: 'online',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const rows = buildTrueNASStorageTopologyRows([healthyPool, unknownPool, offlinePool, attentionPool]);
      expect(rows.map((row) => row.id)).toEqual([
        'pool:pool-attention',
        'pool:pool-offline',
        'pool:pool-unknown',
        'pool:pool-healthy',
      ]);
    });
  });

  describe('sortTrueNASProtectionPoints label-fallback chain', () => {
    it('uses display.subjectLabel when display.itemLabel is absent', () => {
      const usesSubject = makeRecoveryPoint({
        id: 'zzz-id',
        kind: 'snapshot',
        mode: 'snapshot',
        display: { subjectLabel: 'aaa' },
      });
      const usesItem = makeRecoveryPoint({
        id: 'mmm-id',
        kind: 'snapshot',
        mode: 'snapshot',
        display: { itemLabel: 'zzz' },
      });
      // Equal timestamps -> label tiebreak. usesSubject's label resolves to
      // subjectLabel 'aaa' (not its id 'zzz-id'), so it sorts first.
      expect(sortTrueNASProtectionPoints([usesSubject, usesItem]).map((p) => p.id)).toEqual([
        'zzz-id',
        'mmm-id',
      ]);
    });

    it('uses itemRef.name when no display labels are present', () => {
      const usesItemRef = makeRecoveryPoint({
        id: 'zzz-id',
        kind: 'snapshot',
        mode: 'snapshot',
        itemRef: { type: 'truenas-dataset', name: 'aaa' },
      });
      const fallbackId = makeRecoveryPoint({
        id: 'mmm-id',
        kind: 'snapshot',
        mode: 'snapshot',
      });
      expect(sortTrueNASProtectionPoints([usesItemRef, fallbackId]).map((p) => p.id)).toEqual([
        'zzz-id',
        'mmm-id',
      ]);
    });

    it('uses subjectRef.name when display and itemRef are absent', () => {
      const usesSubjectRef = makeRecoveryPoint({
        id: 'zzz-id',
        kind: 'snapshot',
        mode: 'snapshot',
        subjectRef: { type: 'truenas-dataset', name: 'aaa' },
      });
      const fallbackId = makeRecoveryPoint({
        id: 'mmm-id',
        kind: 'snapshot',
        mode: 'snapshot',
      });
      expect(sortTrueNASProtectionPoints([usesSubjectRef, fallbackId]).map((p) => p.id)).toEqual([
        'zzz-id',
        'mmm-id',
      ]);
    });
  });

  describe('trueNASProtectionSearchTokens (via filterTrueNASProtectionPoints)', () => {
    it('matches a point by a token only present in details.sourceDatasets', () => {
      const point = makeRecoveryPoint({
        id: 'p',
        kind: 'snapshot',
        mode: 'snapshot',
        details: { sourceDatasets: ['only-in-source-datasets'] },
      });
      const other = makeRecoveryPoint({ id: 'q', kind: 'snapshot', mode: 'snapshot' });
      expect(
        filterTrueNASProtectionPoints([point, other], 'only-in-source-datasets', 'all').map(
          (p) => p.id,
        ),
      ).toEqual(['p']);
    });

    it('ignores non-array sourceDatasets while still tokenizing scalar details fields', () => {
      const point = makeRecoveryPoint({
        id: 'p',
        kind: 'snapshot',
        mode: 'snapshot',
        // sourceDatasets as a string must be ignored by the tokenizer, but the
        // scalar hostname field is still searchable.
        details: { sourceDatasets: 'not-an-array', hostname: 'unique-host-token' },
      });
      expect(
        filterTrueNASProtectionPoints([point], 'unique-host-token', 'all').map((p) => p.id),
      ).toEqual(['p']);
      expect(
        filterTrueNASProtectionPoints([point], 'not-an-array', 'all').map((p) => p.id),
      ).toEqual([]);
    });

    it('matches by repositoryRef name', () => {
      const point = makeRecoveryPoint({
        id: 'p',
        kind: 'backup',
        mode: 'remote',
        repositoryRef: { type: 'truenas-dataset', name: 'vault-repo-name' },
      });
      expect(
        filterTrueNASProtectionPoints([point], 'vault-repo-name', 'all').map((p) => p.id),
      ).toEqual(['p']);
    });
  });

  describe('buildIncidentRow fallbacks (via buildTrueNASIncidentRows)', () => {
    it('falls back to resource.incidentSeverity/code when incident fields are blank', () => {
      // Incident keeps a non-blank summary so it passes hasIncidentSignal;
      // blank severity/code then fall back to the resource-level values.
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        incidentSeverity: 'warning',
        incidentCode: 'truenas_custom',
        incidents: [{ code: '   ', severity: '   ', summary: 'incident summary', source: 'VolumeStatus' }],
      });
      const rows = buildTrueNASIncidentRows([resource]);
      expect(rows).toHaveLength(1);
      expect(rows[0]?.severity).toBe('warning');
      expect(rows[0]?.code).toBe('truenas_custom');
      expect(rows[0]?.summary).toBe('incident summary');
      expect(rows[0]?.source).toBe('VolumeStatus');
    });

    it('falls back to resource.incidentSummary when incident.summary is blank', () => {
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        incidentSummary: 'resource-level summary',
        incidents: [{ code: 'truenas_x', severity: 'info', summary: '   ' }],
      });
      expect(buildTrueNASIncidentRows([resource])[0]?.summary).toBe('resource-level summary');
    });

    it('uses provider as source when source is absent', () => {
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        incidents: [{ code: 'truenas_x', severity: 'info', summary: 's', provider: 'AdapterX' }],
      });
      expect(buildTrueNASIncidentRows([resource])[0]?.source).toBe('AdapterX');
    });

    it('defaults source to truenas when neither source nor provider is set', () => {
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        incidents: [{ code: 'truenas_x', severity: 'info', summary: 's' }],
      });
      expect(buildTrueNASIncidentRows([resource])[0]?.source).toBe('truenas');
    });

    it('defaults category to health and action to Investigate in TrueNAS', () => {
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        incidents: [{ code: 'truenas_x', severity: 'info', summary: 's' }],
      });
      const row = buildTrueNASIncidentRows([resource])[0];
      expect(row?.category).toBe('health');
      expect(row?.action).toBe('Investigate in TrueNAS');
    });

    it('uses resource.incidentCategory and resource.incidentAction when present', () => {
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        incidentCategory: 'capacity',
        incidentAction: 'Expand the pool',
        incidents: [{ code: 'truenas_x', severity: 'info', summary: 's' }],
      });
      const row = buildTrueNASIncidentRows([resource])[0];
      expect(row?.category).toBe('capacity');
      expect(row?.action).toBe('Expand the pool');
    });

    it('uses explicit incidentPriority when present', () => {
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        incidentPriority: 42,
        incidents: [{ code: 'truenas_x', severity: 'critical', summary: 's' }],
      });
      expect(buildTrueNASIncidentRows([resource])[0]?.priority).toBe(42);
    });

    it('derives priority from severity rank (x1000) when incidentPriority is absent', () => {
      const critical = makeResource({
        id: 'crit',
        type: 'pool',
        incidents: [{ code: 'c', severity: 'critical', summary: 's' }],
      });
      const info = makeResource({
        id: 'info',
        type: 'pool',
        incidents: [{ code: 'i', severity: 'info', summary: 's' }],
      });
      const criticalRow = buildTrueNASIncidentRows([critical])[0];
      const infoRow = buildTrueNASIncidentRows([info])[0];
      expect(criticalRow?.priority).toBe(3000);
      expect(infoRow?.priority).toBe(1000);
    });

    it('uses nativeId in the row id when present, else code', () => {
      const withNative = makeResource({
        id: 'a',
        type: 'pool',
        incidents: [
          { code: 'truenas_code', severity: 'info', summary: 's', nativeId: 'alert-7' },
        ],
      });
      const row = buildTrueNASIncidentRows([withNative])[0];
      expect(row?.id).toBe('a:incident:alert-7:0');
    });

    it('falls back to code in the row id when nativeId is absent', () => {
      const noNative = makeResource({
        id: 'a',
        type: 'pool',
        incidents: [{ code: 'truenas_code', severity: 'info', summary: 's' }],
      });
      const row = buildTrueNASIncidentRows([noNative])[0];
      expect(row?.id).toBe('a:incident:truenas_code:0');
    });

    it('titleizes the code into the label when incidentLabel is absent', () => {
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        // Non-blank code keeps the incident past hasIncidentSignal; blank
        // summary forces the summary fallback to resourceIncidentLabel, which
        // titleizes the (incident) code.
        incidents: [{ code: 'truenas_scrub_failed', severity: 'info', summary: '   ' }],
      });
      const row = buildTrueNASIncidentRows([resource])[0];
      expect(row?.label).toBe('Scrub Failed');
      expect(row?.summary).toBe('Scrub Failed');
    });

    it('returns the literal "TrueNAS Alert" label when code and label are all blank', () => {
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        // No incidentLabel; incident.code blank but summary present passes the
        // hasIncidentSignal filter, exercising the "TrueNAS Alert" fallback.
        incidents: [{ code: '   ', severity: 'info', summary: 'something happened' }],
      });
      expect(buildTrueNASIncidentRows([resource])[0]?.label).toBe('TrueNAS Alert');
    });
  });

  describe('buildRollupIncidentRow (via buildTrueNASIncidentRows)', () => {
    it('builds a rollup with pluralized summary when only incidentCount>1 is set', () => {
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        incidentCount: 3,
      });
      const rows = buildTrueNASIncidentRows([resource]);
      expect(rows).toHaveLength(1);
      expect(rows[0]?.id).toBe('r:incident:rollup');
      expect(rows[0]?.summary).toBe('3 active TrueNAS alerts');
    });

    it('singularizes the summary when incidentCount is exactly 1', () => {
      const resource = makeResource({ id: 'r', type: 'pool', incidentCount: 1 });
      expect(buildTrueNASIncidentRows([resource])[0]?.summary).toBe('1 active TrueNAS alert');
    });

    it('prefers incidentSummary and incidentLabel over the generated count text', () => {
      const withSummary = makeResource({
        id: 'a',
        type: 'pool',
        incidentCount: 5,
        incidentSummary: 'rolled up summary',
      });
      const withLabel = makeResource({
        id: 'b',
        type: 'pool',
        incidentCount: 5,
        incidentLabel: 'label text',
      });
      expect(buildTrueNASIncidentRows([withSummary])[0]?.summary).toBe('rolled up summary');
      expect(buildTrueNASIncidentRows([withLabel])[0]?.summary).toBe('label text');
    });

    it('defaults rollup severity to info and code to truenas_alert', () => {
      const resource = makeResource({ id: 'r', type: 'pool', incidentCount: 1 });
      const row = buildTrueNASIncidentRows([resource])[0];
      expect(row?.severity).toBe('info');
      expect(row?.code).toBe('truenas_alert');
    });

    it('uses incidentSeverity and incidentCode when set on the resource', () => {
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        incidentCount: 1,
        incidentSeverity: 'critical',
        incidentCode: 'truenas_pool',
      });
      const row = buildTrueNASIncidentRows([resource])[0];
      expect(row?.severity).toBe('critical');
      expect(row?.code).toBe('truenas_pool');
    });

    it('uses count-based summary when incidentCode (not count) is the rollup trigger', () => {
      // incidentCount is absent (0); the rollup is triggered by incidentCode.
      const resource = makeResource({
        id: 'r',
        type: 'pool',
        incidentCode: 'truenas_alert',
      });
      const row = buildTrueNASIncidentRows([resource])[0];
      // count || 1 -> 1, but count === 1 is false (count is 0) -> plural 's'.
      expect(row?.summary).toBe('1 active TrueNAS alerts');
    });
  });

  describe('appSearchHaystack + portSearchTokens (via filterTrueNASApps)', () => {
    const baseApp = (overrides: Partial<Resource>): Resource =>
      makeResource({ id: 'app', type: 'app-container', ...overrides });

    it('matches by container id, serviceName, image, and state', () => {
      const app = baseApp({
        truenas: {
          app: {
            containers: [
              {
                id: 'ctr-id-token',
                serviceName: 'svc-name-token',
                image: 'img-token',
                state: 'ctr-state-token',
              },
            ],
          },
        },
      });
      expect(filterTrueNASApps([app], 'ctr-id-token', 'all').map((a) => a.id)).toEqual(['app']);
      expect(filterTrueNASApps([app], 'svc-name-token', 'all')).toHaveLength(1);
      expect(filterTrueNASApps([app], 'img-token', 'all')).toHaveLength(1);
      expect(filterTrueNASApps([app], 'ctr-state-token', 'all')).toHaveLength(1);
    });

    it('matches by volume source/destination/mode/type', () => {
      const app = baseApp({
        truenas: {
          app: {
            volumes: [{ source: 'vol-src-token', destination: 'vol-dst-token', mode: 'rw-mode', type: 'bind-vol' }],
          },
        },
      });
      expect(filterTrueNASApps([app], 'vol-src-token', 'all')).toHaveLength(1);
      expect(filterTrueNASApps([app], 'vol-dst-token', 'all')).toHaveLength(1);
      expect(filterTrueNASApps([app], 'rw-mode', 'all')).toHaveLength(1);
      expect(filterTrueNASApps([app], 'bind-vol', 'all')).toHaveLength(1);
    });

    it('matches by network id/name, usedHostIps, and images', () => {
      const app = baseApp({
        truenas: {
          app: {
            networks: [{ id: 'net-id-token', name: 'net-name-token' }],
            usedHostIps: ['10.0.0.99'],
            images: ['registry.example.com/app:tag'],
          },
        },
      });
      expect(filterTrueNASApps([app], 'net-id-token', 'all')).toHaveLength(1);
      expect(filterTrueNASApps([app], 'net-name-token', 'all')).toHaveLength(1);
      expect(filterTrueNASApps([app], '10.0.0.99', 'all')).toHaveLength(1);
      expect(filterTrueNASApps([app], 'registry.example.com', 'all')).toHaveLength(1);
    });

    it('matches by app usedPorts containerPort and protocol', () => {
      const app = baseApp({
        truenas: {
          app: {
            usedPorts: [{ containerPort: 9090, protocol: 'udp' }],
          },
        },
      });
      expect(filterTrueNASApps([app], '9090', 'all')).toHaveLength(1);
      expect(filterTrueNASApps([app], 'udp', 'all')).toHaveLength(1);
    });

    it('matches by app hostPorts hostPort and hostIp', () => {
      const app = baseApp({
        truenas: {
          app: {
            usedPorts: [
              { hostPorts: [{ hostPort: 30456, hostIp: '192.168.1.5' }] },
            ],
          },
        },
      });
      expect(filterTrueNASApps([app], '30456', 'all')).toHaveLength(1);
      expect(filterTrueNASApps([app], '192.168.1.5', 'all')).toHaveLength(1);
    });

    it('matches by docker publicPort, privatePort, protocol, and ip', () => {
      const app = baseApp({
        docker: {
          ports: [{ publicPort: 8080, privatePort: 80, protocol: 'tcp', ip: '172.16.0.2' }],
        },
      });
      expect(filterTrueNASApps([app], '8080', 'all')).toHaveLength(1);
      expect(filterTrueNASApps([app], '80', 'all')).toHaveLength(1);
      expect(filterTrueNASApps([app], 'tcp', 'all')).toHaveLength(1);
      expect(filterTrueNASApps([app], '172.16.0.2', 'all')).toHaveLength(1);
    });
  });

  describe('shareSearchHaystack (via filterTrueNASShares)', () => {
    const baseShare = (overrides: Partial<Resource>): Resource =>
      makeResource({ id: 'share', type: 'network-share', ...overrides });

    it('matches an enabled share by the "enabled active" token', () => {
      const share = baseShare({ truenas: { share: { enabled: true } } });
      expect(filterTrueNASShares([share], 'enabled', 'all')).toHaveLength(1);
      expect(filterTrueNASShares([share], 'active', 'all')).toHaveLength(1);
    });

    it('matches a disabled share by the "disabled" token', () => {
      const share = baseShare({ truenas: { share: { enabled: false } } });
      expect(filterTrueNASShares([share], 'disabled', 'all')).toHaveLength(1);
      expect(filterTrueNASShares([share], 'active', 'all')).toHaveLength(0);
    });

    it('matches read-only and read-write tokens depending on readOnly', () => {
      const ro = baseShare({ truenas: { share: { readOnly: true } } });
      const rw = baseShare({ id: 'rw', truenas: { share: { readOnly: false } } });
      expect(filterTrueNASShares([ro], 'read-only', 'all')).toHaveLength(1);
      expect(filterTrueNASShares([ro], 'readonly', 'all')).toHaveLength(1);
      expect(filterTrueNASShares([rw], 'read-write', 'all')).toHaveLength(1);
    });

    it('matches browsable/locked/abe/audit/snapshots flag tokens', () => {
      const share = baseShare({
        truenas: {
          share: {
            enabled: true,
            browsable: true,
            locked: true,
            accessBasedEnumeration: true,
            auditEnabled: true,
            exposeSnapshots: true,
          },
        },
      });
      expect(filterTrueNASShares([share], 'browsable', 'all')).toHaveLength(1);
      expect(filterTrueNASShares([share], 'locked', 'all')).toHaveLength(1);
      expect(filterTrueNASShares([share], 'abe', 'all')).toHaveLength(1);
      expect(filterTrueNASShares([share], 'audit', 'all')).toHaveLength(1);
      expect(filterTrueNASShares([share], 'snapshots', 'all')).toHaveLength(1);
    });

    it('matches by aliases, hosts, networks, and security entries', () => {
      const share = baseShare({
        truenas: {
          share: {
            enabled: true,
            aliases: ['alias-token'],
            hosts: ['host-token'],
            networks: ['10.20.30.0/24'],
            security: ['SEC-TOKEN'],
          },
        },
      });
      expect(filterTrueNASShares([share], 'alias-token', 'all')).toHaveLength(1);
      expect(filterTrueNASShares([share], 'host-token', 'all')).toHaveLength(1);
      expect(filterTrueNASShares([share], '10.20.30.0', 'all')).toHaveLength(1);
      expect(filterTrueNASShares([share], 'sec-token', 'all')).toHaveLength(1);
    });
  });

  describe('matchesTrueNASStorageSearch + filterTrueNASStorageTopologyRow (via filterTrueNASStorageTopologyRows)', () => {
    it('returns all rows when search is blank', () => {
      const pool = makeResource({
        id: 'tank',
        type: 'pool',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const rows = buildTrueNASStorageTopologyRows([pool]);
      expect(filterTrueNASStorageTopologyRows(rows, '   ', 'all')).toHaveLength(rows.length);
    });

    it('matches storage risk level and protection summary', () => {
      const pool = makeResource({
        id: 'tank',
        type: 'pool',
        storage: {
          topology: 'pool',
          platform: 'truenas',
          risk: { level: 'elevated-risk-token' },
          riskSummary: 'risk-summary-token',
          protectionSummary: 'protection-summary-token',
        },
      });
      const rows = buildTrueNASStorageTopologyRows([pool]);
      expect(filterTrueNASStorageTopologyRows(rows, 'elevated-risk-token', 'all')).toHaveLength(1);
      expect(filterTrueNASStorageTopologyRows(rows, 'risk-summary-token', 'all')).toHaveLength(1);
      expect(filterTrueNASStorageTopologyRows(rows, 'protection-summary-token', 'all')).toHaveLength(1);
    });

    it('matches disk devPath/model/serial/wwn/diskType/storageState', () => {
      const disk = makeResource({
        id: 'sda',
        type: 'physical_disk',
        physicalDisk: {
          devPath: '/dev/sda-token',
          model: 'model-token',
          serial: 'serial-token',
          wwn: 'wwn-token',
          diskType: 'ssd-type-token',
          storageState: 'spun-down-token',
        },
      });
      const rows = buildTrueNASStorageTopologyRows([disk]);
      expect(filterTrueNASStorageTopologyRows(rows, 'sda-token', 'all')).toHaveLength(1);
      expect(filterTrueNASStorageTopologyRows(rows, 'model-token', 'all')).toHaveLength(1);
      expect(filterTrueNASStorageTopologyRows(rows, 'serial-token', 'all')).toHaveLength(1);
      expect(filterTrueNASStorageTopologyRows(rows, 'wwn-token', 'all')).toHaveLength(1);
      expect(filterTrueNASStorageTopologyRows(rows, 'ssd-type-token', 'all')).toHaveLength(1);
      expect(filterTrueNASStorageTopologyRows(rows, 'spun-down-token', 'all')).toHaveLength(1);
    });

    it('matches incident summary and code attached to a resource', () => {
      const pool = makeResource({
        id: 'tank',
        type: 'pool',
        storage: { topology: 'pool', platform: 'truenas' },
        incidents: [{ code: 'incident-code-token', severity: 'warning', summary: 'incident-summary-token' }],
      });
      const rows = buildTrueNASStorageTopologyRows([pool]);
      expect(filterTrueNASStorageTopologyRows(rows, 'incident-code-token', 'all')).toHaveLength(1);
      expect(filterTrueNASStorageTopologyRows(rows, 'incident-summary-token', 'all')).toHaveLength(1);
    });

    it('matches a row by its kind keyword (pool) even when the resource name does not contain it', () => {
      // type 'pool' with name 'tank' (no 'pool' substring anywhere searchable).
      const pool = makeResource({ id: 'tank', type: 'pool', name: 'tank' });
      const rows = buildTrueNASStorageTopologyRows([pool]);
      expect(filterTrueNASStorageTopologyRows(rows, 'pool', 'all').map((r) => r.id)).toEqual([
        'pool:tank',
      ]);
    });

    it('filters out healthy rows when status is attention, and keeps them when status is healthy', () => {
      const healthyPool = makeResource({
        id: 'hp',
        type: 'pool',
        status: 'online',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const rows = buildTrueNASStorageTopologyRows([healthyPool]);
      expect(filterTrueNASStorageTopologyRows(rows, '', 'attention')).toHaveLength(0);
      expect(filterTrueNASStorageTopologyRows(rows, '', 'healthy')).toHaveLength(1);
    });
  });

  describe('resourceDisplayName fallback chain (via buildTrueNASServiceRows systemName)', () => {
    it('uses name when displayName is blank', () => {
      const system = makeResource({
        id: 'sys',
        type: 'agent',
        name: 'by-name',
        displayName: '   ',
        truenas: { services: [{ id: '1', service: 'smb', enabled: true, state: 'RUNNING' }] },
      });
      expect(buildTrueNASServiceRows([system])[0]?.systemName).toBe('by-name');
    });

    it('uses truenas.hostname when name and displayName are blank', () => {
      const system = makeResource({
        id: 'sys',
        type: 'agent',
        name: '   ',
        displayName: '   ',
        truenas: {
          hostname: 'nas-host',
          services: [{ id: '1', service: 'smb', enabled: true, state: 'RUNNING' }],
        },
      });
      expect(buildTrueNASServiceRows([system])[0]?.systemName).toBe('nas-host');
    });

    it('falls back to resource id when no display fields are set', () => {
      const system = makeResource({
        id: 'sys-id',
        type: 'agent',
        name: '   ',
        displayName: '   ',
        truenas: { services: [{ id: '1', service: 'smb', enabled: true, state: 'RUNNING' }] },
      });
      expect(buildTrueNASServiceRows([system])[0]?.systemName).toBe('sys-id');
    });
  });

  describe('serviceDisplayName fallback chain (via buildTrueNASServiceRows row id)', () => {
    it('uses service when present', () => {
      const system = makeResource({
        id: 'sys',
        type: 'agent',
        truenas: { services: [{ service: 'nfs', enabled: true, state: 'RUNNING' }] },
      });
      expect(buildTrueNASServiceRows([system])[0]?.id).toBe('sys:service:nfs');
    });

    it('falls back to service id when service field is absent', () => {
      const system = makeResource({
        id: 'sys',
        type: 'agent',
        truenas: { services: [{ id: 'fallback-id', enabled: true, state: 'RUNNING' }] },
      });
      expect(buildTrueNASServiceRows([system])[0]?.id).toBe('sys:service:fallback-id');
    });

    it('falls back to "unknown" when neither service nor id is present', () => {
      const system = makeResource({
        id: 'sys',
        type: 'agent',
        truenas: { services: [{ enabled: true, state: 'RUNNING' }] },
      });
      expect(buildTrueNASServiceRows([system])[0]?.id).toBe('sys:service:unknown');
    });
  });

  describe('inferTrueNASPoolName inference sources (via buildTrueNASStorageChildCounts)', () => {
    const tankPool = (): Resource =>
      makeResource({
        id: 'pool-tank',
        type: 'pool',
        name: 'tank',
        storage: { topology: 'pool', platform: 'truenas' },
      });

    it('infers the pool from physicalDisk.storageGroup', () => {
      const disk = makeResource({
        id: 'disk-sda',
        type: 'physical_disk',
        physicalDisk: { storageGroup: 'tank', devPath: '/dev/sda' },
      });
      expect(buildTrueNASStorageChildCounts([tankPool(), disk]).get('pool-tank')).toEqual({
        datasets: 0,
        shares: 0,
        disks: 1,
      });
    });

    it('infers the pool from share.dataset first segment', () => {
      const share = makeResource({
        id: 'share',
        type: 'network-share',
        truenas: { share: { dataset: 'tank/media' } },
      });
      expect(buildTrueNASStorageChildCounts([tankPool(), share]).get('pool-tank')).toEqual({
        datasets: 0,
        shares: 1,
        disks: 0,
      });
    });

    it('infers the pool from share.path first segment when dataset is absent', () => {
      const share = makeResource({
        id: 'share',
        type: 'network-share',
        truenas: { share: { path: '/mnt/tank/media' } },
      });
      expect(buildTrueNASStorageChildCounts([tankPool(), share]).get('pool-tank')?.shares).toBe(1);
    });

    it('infers the pool from storage.pool when no disk/share signals exist', () => {
      const disk = makeResource({
        id: 'disk',
        type: 'physical_disk',
        storage: { pool: 'tank' },
      });
      expect(buildTrueNASStorageChildCounts([tankPool(), disk]).get('pool-tank')?.disks).toBe(1);
    });

    it('infers the pool from storage.path first segment when storage.pool is absent', () => {
      const disk = makeResource({
        id: 'disk',
        type: 'physical_disk',
        storage: { path: '/mnt/tank/data' },
      });
      expect(buildTrueNASStorageChildCounts([tankPool(), disk]).get('pool-tank')?.disks).toBe(1);
    });

    it('infers the pool from resource name first segment as a last resort', () => {
      const disk = makeResource({
        id: 'disk',
        type: 'physical_disk',
        name: 'tank',
      });
      expect(buildTrueNASStorageChildCounts([tankPool(), disk]).get('pool-tank')?.disks).toBe(1);
    });
  });

  describe('hasInferredStorageRelationship branches (via buildTrueNASStorageChildCounts)', () => {
    it('relates a share to a dataset by storage-path containment', () => {
      const dataset = makeResource({
        id: 'ds-media',
        type: 'dataset',
        name: 'tank/media',
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
      });
      const share = makeResource({
        id: 'share',
        type: 'network-share',
        storage: { path: '/mnt/tank/media' },
      });
      expect(buildTrueNASStorageChildCounts([dataset, share]).get('ds-media')).toEqual({
        datasets: 0,
        shares: 1,
        disks: 0,
      });
    });

    it('does not relate a share whose path does not descend from the dataset', () => {
      const dataset = makeResource({
        id: 'ds-media',
        type: 'dataset',
        name: 'tank/media',
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
      });
      const share = makeResource({
        id: 'share',
        type: 'network-share',
        storage: { path: '/mnt/other/data' },
      });
      expect(buildTrueNASStorageChildCounts([dataset, share]).get('ds-media')).toEqual({
        datasets: 0,
        shares: 0,
        disks: 0,
      });
    });

    it('returns false for a pool when the resource has no inferable pool name', () => {
      const pool = makeResource({
        id: 'pool-tank',
        type: 'pool',
        name: 'tank',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const blankShare = makeResource({
        id: 'blank-share',
        type: 'network-share',
        name: '   ',
        displayName: '   ',
      });
      expect(buildTrueNASStorageChildCounts([pool, blankShare]).get('pool-tank')).toEqual({
        datasets: 0,
        shares: 0,
        disks: 0,
      });
    });
  });

  describe('buildTrueNASSystemChildCounts parent-walk branches', () => {
    it('terminates the parent walk on a parentId cycle without attributing to any system', () => {
      const sysA = makeResource({ id: 'sys-a', type: 'agent' });
      const sysB = makeResource({ id: 'sys-b', type: 'agent' });
      // Two resources that point at each other; neither chains to a system and
      // there are multiple systems so no single-system fallback applies.
      const cyclicA = makeResource({ id: 'cyc-a', type: 'pool', parentId: 'cyc-b' });
      const cyclicB = makeResource({ id: 'cyc-b', type: 'pool', parentId: 'cyc-a' });
      const counts = buildTrueNASSystemChildCounts([sysA, sysB, cyclicA, cyclicB], [sysA, sysB]);
      expect(counts.get(sysA.id)?.pools).toBe(0);
      expect(counts.get(sysB.id)?.pools).toBe(0);
    });

    it('skips agent-typed entries in the resource loop', () => {
      const sys = makeResource({ id: 'sys', type: 'agent' });
      // A second agent row is present in resources but must not be counted.
      const otherAgent = makeResource({ id: 'agent-2', type: 'agent' });
      const counts = buildTrueNASSystemChildCounts([sys, otherAgent], [sys]);
      expect(counts.get(sys.id)?.pools).toBe(0);
    });
  });

  describe('buildTrueNASStorageTopologyRows ownership branches', () => {
    it('walks the parent chain through a dataset to find the owning pool for a disk', () => {
      // owningPoolId parent-walk: disk -> dataset -> pool.
      const pool = makeResource({
        id: 'pool-tank',
        type: 'pool',
        name: 'tank',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const dataset = makeResource({
        id: 'ds-media',
        type: 'dataset',
        name: 'tank/media',
        parentId: pool.id,
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
      });
      const disk = makeResource({
        id: 'disk-sda',
        type: 'physical_disk',
        name: 'sda',
        parentId: dataset.id,
        physicalDisk: { devPath: '/dev/sda', serial: 'x' },
      });
      const rows = buildTrueNASStorageTopologyRows([pool, dataset, disk]);
      const diskRow = rows.find((r) => r.id === 'disk:disk-sda');
      expect(diskRow?.parentRowId).toBe('pool:pool-tank');
    });

    it('returns empty owningDatasetId when the dataset parent is a pool (root dataset under pool)', () => {
      const pool = makeResource({
        id: 'pool-tank',
        type: 'pool',
        name: 'tank',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const dataset = makeResource({
        id: 'ds-root',
        type: 'dataset',
        name: 'tank/root',
        parentId: pool.id,
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/root' },
      });
      const rows = buildTrueNASStorageTopologyRows([pool, dataset]);
      const dsRow = rows.find((r) => r.id === 'dataset:ds-root');
      // owningDatasetId short-circuits to '' because the parent is a pool, so
      // the dataset nests directly under the pool, not under another dataset.
      expect(dsRow?.parentRowId).toBe('pool:pool-tank');
      expect(dsRow?.depth).toBe(1);
    });

    it('breaks a mutually-referencing dataset cycle without infinite recursion', () => {
      // appendDatasetTree visiting-set guard: A.parentId -> B and B.parentId -> A
      // makes each the other's owning dataset, forming a cycle in
      // childDatasetsByDataset. The visiting set must terminate the recursion.
      const a = makeResource({
        id: 'ds-a',
        type: 'dataset',
        name: 'a',
        parentId: 'ds-b',
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/a' },
      });
      const b = makeResource({
        id: 'ds-b',
        type: 'dataset',
        name: 'b',
        parentId: 'ds-a',
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/b' },
      });
      const rows = buildTrueNASStorageTopologyRows([a, b]);
      const ids = rows.map((r) => r.id);
      expect(ids).toContain('dataset:ds-a');
      expect(ids).toContain('dataset:ds-b');
      // Exactly two rows: the cycle is broken, no third (re-)emission of ds-a.
      expect(rows).toHaveLength(2);
    });

    it('does not re-emit at root a dataset already nested under a pool', () => {
      const pool = makeResource({
        id: 'pool-tank',
        type: 'pool',
        name: 'tank',
        storage: { topology: 'pool', platform: 'truenas' },
      });
      const dataset = makeResource({
        id: 'ds-media',
        type: 'dataset',
        name: 'tank/media',
        parentId: pool.id,
        storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
      });
      const rows = buildTrueNASStorageTopologyRows([pool, dataset]);
      // dataset appears exactly once (nested under pool), never duplicated at root.
      expect(rows.filter((r) => r.id === 'dataset:ds-media')).toHaveLength(1);
    });
  });

  describe('impaired-source short-circuit across status mappers', () => {
    it('returns attention for vm/app/share/service/storage when the truenas source is impaired', () => {
      const vm = makeResource({ id: 'vm', type: 'vm', platformData: impairedSource() });
      expect(mapTrueNASVMStatus(vm)).toBe('attention');

      const app = makeResource({ id: 'app', type: 'app-container', platformData: impairedSource() });
      expect(mapTrueNASAppStatus(app)).toBe('attention');

      const share = makeResource({
        id: 'share',
        type: 'network-share',
        platformData: impairedSource(),
      });
      expect(mapTrueNASShareStatus(share)).toBe('attention');

      const serviceRow = makeServiceRow({
        system: makeResource({ id: 'sys', type: 'agent', platformData: impairedSource() }),
      });
      expect(mapTrueNASServiceStatus(serviceRow)).toBe('attention');

      const pool = makeResource({ id: 'pool', type: 'pool', platformData: impairedSource() });
      expect(mapTrueNASStorageStatus(pool)).toBe('attention');
    });
  });
});
