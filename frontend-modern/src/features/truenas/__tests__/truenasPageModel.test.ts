import { describe, expect, it } from 'vitest';
import type { RecoveryPoint } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import {
  TRUENAS_TAB_SPECS,
  buildTrueNASPageModel,
  buildTrueNASServiceRows,
  buildTrueNASStorageChildCounts,
  buildTrueNASStorageTopologyRows,
  buildTrueNASSystemChildCounts,
  filterTrueNASApps,
  filterTrueNASIncidents,
  filterTrueNASProtectionPoints,
  filterTrueNASServices,
  filterTrueNASStorageTopologyRows,
  filterTrueNASShares,
  filterTrueNASVMs,
  mapTrueNASAppStatus,
  mapTrueNASIncidentSeverity,
  mapTrueNASProtectionKind,
  mapTrueNASProtectionStatus,
  mapTrueNASServiceStatus,
  mapTrueNASShareStatus,
  mapTrueNASStorageStatus,
  mapTrueNASVMStatus,
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

describe('truenasPageModel', () => {
  it('declares the native TrueNAS section set around API facets', () => {
    expect(TRUENAS_TAB_SPECS.map((tab) => tab.id)).toEqual([
      'overview',
      'storage',
      'services',
      'apps',
      'vms',
      'shares',
      'protection',
    ]);
  });

  it('buckets systems, workloads, and native storage inventory by TrueNAS API facet', () => {
    const model = buildTrueNASPageModel([
      makeResource({
        id: 'truenas-system',
        type: 'agent',
        truenas: { services: [{ id: '1', service: 'smb', enabled: true, state: 'RUNNING' }] },
      }),
      makeResource({ id: 'truenas-vm', type: 'vm' }),
      makeResource({ id: 'truenas-app', type: 'app-container' }),
      makeResource({ id: 'truenas-share', type: 'network-share' }),
      makeResource({
        id: 'truenas-pool',
        type: 'storage',
        storage: { topology: 'pool', platform: 'truenas' },
      }),
      makeResource({
        id: 'truenas-dataset',
        type: 'storage',
        storage: { topology: 'dataset', platform: 'truenas' },
      }),
      makeResource({ id: 'truenas-disk', type: 'physical_disk' }),
      makeResource({ id: 'docker-host', type: 'agent', platformType: 'docker' }),
      makeResource({ id: 'pve-node', type: 'agent', platformType: 'proxmox-pve' }),
    ]);

    expect(model.systems.map((r) => r.id)).toEqual(['truenas-system']);
    expect(model.shares.map((r) => r.id)).toEqual(['truenas-share']);
    expect(model.vms.map((r) => r.id)).toEqual(['truenas-vm']);
    expect(model.apps.map((r) => r.id)).toEqual(['truenas-app']);
    expect(model.services.map((row) => row.service.service)).toEqual(['smb']);
    expect(model.pools.map((r) => r.id)).toEqual(['truenas-pool']);
    expect(model.datasets.map((r) => r.id)).toEqual(['truenas-dataset']);
    expect(model.disks.map((r) => r.id)).toEqual(['truenas-disk']);
    expect(model.resources.map((r) => r.id).sort()).toEqual(
      [
        'truenas-app',
        'truenas-dataset',
        'truenas-disk',
        'truenas-pool',
        'truenas-share',
        'truenas-system',
        'truenas-vm',
      ].sort(),
    );
  });

  it('counts overview inventory from each TrueNAS system hierarchy', () => {
    const primary = makeResource({
      id: 'system-primary',
      type: 'agent',
      name: 'nas-primary',
      truenas: {
        services: [
          { id: '1', service: 'smb', enabled: true, state: 'RUNNING' },
          { id: '2', service: 'ssh', enabled: false, state: 'STOPPED' },
        ],
      },
    });
    const backup = makeResource({
      id: 'system-backup',
      type: 'agent',
      name: 'nas-backup',
      truenas: { services: [{ id: '1', service: 'nfs', enabled: true, state: 'RUNNING' }] },
    });
    const primaryPool = makeResource({
      id: 'primary-pool-tank',
      type: 'storage',
      name: 'tank',
      parentId: primary.id,
      storage: { topology: 'pool', platform: 'truenas' },
    });
    const backupPool = makeResource({
      id: 'backup-pool-tank',
      type: 'storage',
      name: 'tank',
      parentId: backup.id,
      storage: { topology: 'pool', platform: 'truenas' },
    });
    const primaryDataset = makeResource({
      id: 'primary-dataset-media',
      type: 'storage',
      name: 'tank/media',
      parentId: primaryPool.id,
      storage: { topology: 'dataset', platform: 'truenas' },
    });
    const backupDataset = makeResource({
      id: 'backup-dataset-media',
      type: 'storage',
      name: 'tank/media',
      parentId: backupPool.id,
      storage: { topology: 'dataset', platform: 'truenas' },
    });

    const counts = buildTrueNASSystemChildCounts(
      [
        primary,
        backup,
        primaryPool,
        backupPool,
        primaryDataset,
        backupDataset,
        makeResource({ id: 'primary-share', type: 'network-share', parentId: primaryDataset.id }),
        makeResource({ id: 'backup-share', type: 'network-share', parentId: backupDataset.id }),
        makeResource({ id: 'primary-disk', type: 'physical_disk', parentId: primaryPool.id }),
        makeResource({ id: 'backup-disk', type: 'physical_disk', parentId: backupPool.id }),
        makeResource({ id: 'primary-vm', type: 'vm', parentId: primary.id }),
        makeResource({ id: 'backup-vm', type: 'vm', parentId: backup.id }),
        makeResource({ id: 'primary-app', type: 'app-container', parentId: primary.id }),
        makeResource({ id: 'backup-app', type: 'app-container', parentId: backup.id }),
      ],
      [primary, backup],
    );

    expect(counts.get(primary.id)).toEqual({
      pools: 1,
      datasets: 1,
      shares: 1,
      vms: 1,
      apps: 1,
      disks: 1,
      services: 2,
    });
    expect(counts.get(backup.id)).toEqual({
      pools: 1,
      datasets: 1,
      shares: 1,
      vms: 1,
      apps: 1,
      disks: 1,
      services: 1,
    });
  });

  it('keeps the single-system inventory fallback for older unparented TrueNAS resources', () => {
    const system = makeResource({
      id: 'system-primary',
      type: 'agent',
      truenas: { services: [{ id: '1', service: 'smb', enabled: true, state: 'RUNNING' }] },
    });
    const counts = buildTrueNASSystemChildCounts(
      [
        system,
        makeResource({ id: 'pool-tank', type: 'storage', storage: { topology: 'pool' } }),
        makeResource({ id: 'dataset-media', type: 'storage', storage: { topology: 'dataset' } }),
        makeResource({ id: 'share-media', type: 'network-share' }),
        makeResource({ id: 'disk-sda', type: 'physical_disk' }),
        makeResource({ id: 'vm-windows', type: 'vm' }),
        makeResource({ id: 'app-nextcloud', type: 'app-container' }),
      ],
      [system],
    );

    expect(counts.get(system.id)).toEqual({
      pools: 1,
      datasets: 1,
      shares: 1,
      vms: 1,
      apps: 1,
      disks: 1,
      services: 1,
    });
  });

  it('counts unparented TrueNAS inventory in mixed single-system snapshots', () => {
    const system = makeResource({
      id: 'system-primary',
      type: 'agent',
      truenas: { services: [{ id: '1', service: 'smb', enabled: true, state: 'RUNNING' }] },
    });
    const pool = makeResource({
      id: 'pool-tank',
      type: 'storage',
      parentId: system.id,
      storage: { topology: 'pool', platform: 'truenas' },
    });
    const dataset = makeResource({
      id: 'dataset-media',
      type: 'storage',
      parentId: pool.id,
      storage: { topology: 'dataset', platform: 'truenas' },
    });

    const counts = buildTrueNASSystemChildCounts(
      [
        system,
        pool,
        dataset,
        makeResource({ id: 'share-media', type: 'network-share' }),
        makeResource({ id: 'disk-sda', type: 'physical_disk' }),
        makeResource({ id: 'vm-windows', type: 'vm' }),
        makeResource({ id: 'app-nextcloud', type: 'app-container' }),
      ],
      [system],
    );

    expect(counts.get(system.id)).toEqual({
      pools: 1,
      datasets: 1,
      shares: 1,
      vms: 1,
      apps: 1,
      disks: 1,
      services: 1,
    });
  });

  it('builds and filters native TrueNAS service rows from system metadata', () => {
    const system = makeResource({
      id: 'system-primary',
      type: 'agent',
      name: 'nas-primary',
      truenas: {
        hostname: 'nas-primary',
        services: [
          { id: '1', service: 'smb', enabled: true, state: 'RUNNING', pids: [2418, 2420] },
          { id: '2', service: 'ssh', enabled: false, state: 'STOPPED' },
          { id: '3', service: 'smartd', enabled: true, state: 'STOPPED' },
        ],
      },
    });

    const rows = buildTrueNASServiceRows([system]);

    expect(rows.map((row) => row.service.service)).toEqual(['smartd', 'ssh', 'smb']);
    expect(mapTrueNASServiceStatus(rows[0])).toBe('stopped');
    expect(mapTrueNASServiceStatus(rows[1])).toBe('disabled');
    expect(mapTrueNASServiceStatus(rows[2])).toBe('running');
    expect(filterTrueNASServices(rows, 'nas-primary', 'all')).toHaveLength(3);
    expect(filterTrueNASServices(rows, 'smb', 'running').map((row) => row.service.service)).toEqual(
      ['smb'],
    );
    expect(filterTrueNASServices(rows, '', 'disabled').map((row) => row.service.service)).toEqual([
      'ssh',
    ]);
  });

  it('builds a native TrueNAS storage topology with pool child counts', () => {
    const system = makeResource({ id: 'system-primary', type: 'agent' });
    const pool = makeResource({
      id: 'pool-tank',
      type: 'storage',
      name: 'tank',
      parentId: system.id,
      storage: { topology: 'pool', platform: 'truenas', zfsPoolState: 'ONLINE' },
    });
    const dataset = makeResource({
      id: 'dataset-media',
      type: 'storage',
      name: 'tank/media',
      parentId: pool.id,
      storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
    });
    const share = makeResource({
      id: 'share-media',
      type: 'network-share',
      parentId: dataset.id,
      truenas: {
        share: {
          id: 'smb-1',
          name: 'Media',
          protocol: 'SMB',
          path: '/mnt/tank/media',
          dataset: 'tank/media',
        },
      },
    });
    const disk = makeResource({
      id: 'disk-sda',
      type: 'physical_disk',
      name: 'sda',
      parentId: pool.id,
      physicalDisk: {
        devPath: '/dev/sda',
        serial: 'serial-123',
        sizeBytes: 2_000_000_000_000,
        health: 'PASSED',
      },
    });

    const counts = buildTrueNASStorageChildCounts([system, pool, dataset, share, disk]);
    const rows = buildTrueNASStorageTopologyRows([system, pool, dataset, share, disk]);

    expect(counts.get(pool.id)).toEqual({ datasets: 1, shares: 1, disks: 1 });
    expect(counts.get(dataset.id)).toEqual({ datasets: 0, shares: 1, disks: 0 });
    expect(rows.map((row) => row.id)).toEqual([
      'pool:pool-tank',
      'dataset:dataset-media',
      'disk:disk-sda',
    ]);
    expect(rows.map((row) => row.depth)).toEqual([0, 1, 1]);
    expect(rows[1]?.parentRowId).toBe('pool:pool-tank');
    expect(rows[2]?.parentRowId).toBe('pool:pool-tank');
  });

  it('infers TrueNAS storage topology ownership from native pool and dataset labels', () => {
    const archivePool = makeResource({
      id: 'pool-archive',
      type: 'storage',
      name: 'archive',
      storage: { topology: 'pool', platform: 'truenas', zfsPoolState: 'ONLINE' },
    });
    const tankPool = makeResource({
      id: 'pool-tank',
      type: 'storage',
      name: 'tank',
      storage: { topology: 'pool', platform: 'truenas', zfsPoolState: 'ONLINE' },
    });
    const dataset = makeResource({
      id: 'dataset-media',
      type: 'storage',
      name: 'tank/media',
      storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
    });
    const share = makeResource({
      id: 'share-media',
      type: 'network-share',
      truenas: {
        share: {
          id: 'smb-1',
          name: 'Media',
          protocol: 'SMB',
          path: '/mnt/tank/media',
          dataset: 'tank/media',
        },
      },
    });
    const disk = makeResource({
      id: 'disk-sda',
      type: 'physical_disk',
      name: 'sda',
      physicalDisk: {
        storageGroup: 'tank',
        devPath: '/dev/sda',
        serial: 'serial-123',
        health: 'PASSED',
      },
    });

    const resources = [archivePool, tankPool, dataset, share, disk];
    const counts = buildTrueNASStorageChildCounts(resources);
    const rows = buildTrueNASStorageTopologyRows(resources);

    expect(counts.get(archivePool.id)).toEqual({ datasets: 0, shares: 0, disks: 0 });
    expect(counts.get(tankPool.id)).toEqual({ datasets: 1, shares: 1, disks: 1 });
    expect(counts.get(dataset.id)).toEqual({ datasets: 0, shares: 1, disks: 0 });
    expect(rows.map((row) => row.id)).toEqual([
      'pool:pool-archive',
      'pool:pool-tank',
      'dataset:dataset-media',
      'disk:disk-sda',
    ]);
    expect(rows[2]?.parentRowId).toBe('pool:pool-tank');
    expect(rows[3]?.parentRowId).toBe('pool:pool-tank');
  });

  it('nests TrueNAS datasets under the closest dataset path ancestor', () => {
    const pool = makeResource({
      id: 'pool-tank',
      type: 'storage',
      name: 'tank',
      storage: { topology: 'pool', platform: 'truenas', zfsPoolState: 'ONLINE' },
    });
    const media = makeResource({
      id: 'dataset-media',
      type: 'storage',
      name: 'tank/media',
      storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media' },
    });
    const photos = makeResource({
      id: 'dataset-photos',
      type: 'storage',
      name: 'tank/media/photos',
      storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media/photos' },
    });
    const raw = makeResource({
      id: 'dataset-raw',
      type: 'storage',
      name: 'tank/media/photos/raw',
      storage: { topology: 'dataset', platform: 'truenas', path: '/mnt/tank/media/photos/raw' },
    });
    const share = makeResource({
      id: 'share-raw',
      type: 'network-share',
      truenas: {
        share: {
          id: 'smb-raw',
          name: 'Raw Photos',
          protocol: 'SMB',
          path: '/mnt/tank/media/photos/raw',
          dataset: 'tank/media/photos/raw',
        },
      },
    });

    const resources = [pool, raw, media, photos, share];
    const counts = buildTrueNASStorageChildCounts(resources);
    const rows = buildTrueNASStorageTopologyRows(resources);

    expect(counts.get(pool.id)).toEqual({ datasets: 3, shares: 1, disks: 0 });
    expect(counts.get(media.id)).toEqual({ datasets: 2, shares: 1, disks: 0 });
    expect(counts.get(photos.id)).toEqual({ datasets: 1, shares: 1, disks: 0 });
    expect(counts.get(raw.id)).toEqual({ datasets: 0, shares: 1, disks: 0 });
    expect(rows.map((row) => [row.id, row.depth, row.parentRowId])).toEqual([
      ['pool:pool-tank', 0, undefined],
      ['dataset:dataset-media', 1, 'pool:pool-tank'],
      ['dataset:dataset-photos', 2, 'dataset:dataset-media'],
      ['dataset:dataset-raw', 3, 'dataset:dataset-photos'],
    ]);
    expect(filterTrueNASStorageTopologyRows(rows, 'raw', 'all').map((row) => row.id)).toEqual([
      'pool:pool-tank',
      'dataset:dataset-media',
      'dataset:dataset-photos',
      'dataset:dataset-raw',
    ]);
  });

  it('keeps pool ancestors visible when filtering matching storage children', () => {
    const pool = makeResource({
      id: 'pool-tank',
      type: 'storage',
      name: 'tank',
      storage: { topology: 'pool', platform: 'truenas', zfsPoolState: 'ONLINE' },
    });
    const degradedPool = makeResource({
      id: 'pool-archive',
      type: 'storage',
      name: 'archive',
      status: 'online',
      storage: { topology: 'pool', platform: 'truenas', zfsPoolState: 'DEGRADED' },
    });
    const disk = makeResource({
      id: 'disk-sdb',
      type: 'physical_disk',
      name: 'sdb',
      parentId: pool.id,
      physicalDisk: {
        devPath: '/dev/sdb',
        serial: 'serial-degraded',
        health: 'DEGRADED',
      },
    });

    expect(mapTrueNASStorageStatus(degradedPool)).toBe('attention');
    expect(mapTrueNASStorageStatus(disk)).toBe('attention');

    const rows = buildTrueNASStorageTopologyRows([pool, degradedPool, disk]);
    expect(
      filterTrueNASStorageTopologyRows(rows, 'serial-degraded', 'attention').map((row) => row.id),
    ).toEqual(['pool:pool-tank', 'disk:disk-sdb']);
  });

  it('filters apps using native TrueNAS app.query metadata', () => {
    const nextcloud = makeResource({
      id: 'app-nextcloud',
      type: 'app-container',
      parentName: 'truenas-main',
      truenas: {
        hostname: 'truenas-main',
        app: {
          id: 'nextcloud',
          name: 'Nextcloud',
          state: 'RUNNING',
          humanVersion: '29.0.7',
          images: ['docker.io/library/nextcloud:29.0.7'],
          usedPorts: [
            {
              containerPort: 443,
              protocol: 'tcp',
              hostPorts: [{ hostPort: 30443, hostIp: '0.0.0.0' }],
            },
          ],
          volumes: [{ source: '/mnt/tank/apps/nextcloud', destination: '/var/www/html' }],
        },
      },
    });
    const adguard = makeResource({
      id: 'app-adguard',
      type: 'app-container',
      status: 'offline',
      truenas: {
        app: {
          id: 'adguard',
          name: 'AdGuard Home',
          state: 'STOPPED',
          images: ['docker.io/adguard/adguardhome:v0.107'],
        },
      },
    });

    expect(mapTrueNASAppStatus(nextcloud)).toBe('running');
    expect(mapTrueNASAppStatus(adguard)).toBe('stopped');
    expect(filterTrueNASApps([nextcloud, adguard], '30443', 'all').map((r) => r.id)).toEqual([
      'app-nextcloud',
    ]);
    expect(
      filterTrueNASApps([nextcloud, adguard], 'adguardhome', 'stopped').map((r) => r.id),
    ).toEqual(['app-adguard']);
  });

  it('filters VMs using native TrueNAS vm.query metadata', () => {
    const windows = makeResource({
      id: 'vm-windows',
      type: 'vm',
      truenas: {
        hostname: 'truenas-main',
        vm: {
          id: '42',
          name: 'windows-lab',
          description: 'Build validation workstation',
          state: 'RUNNING',
          bootloader: 'UEFI',
          cpuMode: 'HOST-PASSTHROUGH',
          uuid: 'vm-uuid-1',
        },
      },
    });
    const ubuntu = makeResource({
      id: 'vm-ubuntu',
      type: 'vm',
      status: 'offline',
      truenas: {
        vm: {
          id: '43',
          name: 'ubuntu-build',
          state: 'STOPPED',
          machineType: 'q35',
        },
      },
    });

    expect(mapTrueNASVMStatus(windows)).toBe('running');
    expect(mapTrueNASVMStatus(ubuntu)).toBe('stopped');
    expect(filterTrueNASVMs([windows, ubuntu], 'passthrough', 'running').map((r) => r.id)).toEqual([
      'vm-windows',
    ]);
    expect(filterTrueNASVMs([windows, ubuntu], 'q35', 'stopped').map((r) => r.id)).toEqual([
      'vm-ubuntu',
    ]);
  });

  it('filters shares using native TrueNAS SMB and NFS metadata', () => {
    const media = makeResource({
      id: 'share-media',
      type: 'network-share',
      truenas: {
        hostname: 'truenas-main',
        share: {
          id: 'smb-1',
          name: 'Media',
          protocol: 'SMB',
          path: '/mnt/tank/media',
          dataset: 'tank/media',
          enabled: true,
          browsable: true,
          auditEnabled: true,
          aliases: ['media'],
        },
      },
    });
    const archive = makeResource({
      id: 'share-archive',
      type: 'network-share',
      status: 'offline',
      truenas: {
        share: {
          id: 'nfs-2',
          name: 'Archive',
          protocol: 'NFS',
          path: '/mnt/tank/archive',
          dataset: 'tank/archive',
          enabled: false,
          readOnly: true,
          networks: ['10.10.20.0/24'],
          security: ['SYS'],
        },
      },
    });

    expect(mapTrueNASShareStatus(media)).toBe('active');
    expect(mapTrueNASShareStatus(archive)).toBe('disabled');
    expect(filterTrueNASShares([media, archive], 'audit', 'active').map((r) => r.id)).toEqual([
      'share-media',
    ]);
    expect(
      filterTrueNASShares([media, archive], '10.10.20.0/24', 'disabled').map((r) => r.id),
    ).toEqual(['share-archive']);
  });

  it('derives native TrueNAS alert rows from resource incidents', () => {
    const system = makeResource({
      id: 'truenas-system',
      type: 'agent',
      incidentCount: 2,
      incidentSeverity: 'critical',
      incidentLabel: 'Storage Health Issue',
      incidentAction: 'Investigate storage health immediately',
      incidents: [
        {
          provider: 'truenas',
          nativeId: 'alert-system',
          code: 'truenas_volume_status',
          severity: 'CRITICAL',
          source: 'VolumeStatus',
          summary: 'Pool tank is FAULTED',
          startedAt: '2026-05-20T12:00:00Z',
        },
      ],
    });
    const disk = makeResource({
      id: 'truenas-disk-sda',
      type: 'physical_disk',
      parentName: 'tank',
      incidentCount: 1,
      incidentSeverity: 'warning',
      incidentLabel: 'Disk Health Issue',
      incidents: [
        {
          provider: 'truenas',
          nativeId: 'alert-smart',
          code: 'truenas_smart',
          severity: 'WARNING',
          source: 'SMART',
          summary: 'Device /dev/sda has unreadable sectors',
        },
      ],
    });
    const poolRollup = makeResource({
      id: 'truenas-pool-archive',
      type: 'storage',
      incidentCount: 1,
      incidentSeverity: 'warning',
      incidentCode: 'truenas_scrub',
      incidentLabel: 'Scrub Issue',
      incidentSummary: 'Last scrub found checksum errors',
    });
    const dockerIncident = makeResource({
      id: 'docker-host',
      type: 'agent',
      platformType: 'docker',
      incidentCount: 1,
      incidentSummary: 'Docker incident should not appear',
    });

    const model = buildTrueNASPageModel([system, disk, poolRollup, dockerIncident]);

    expect(model.incidents.map((incident) => incident.resourceId)).toEqual([
      'truenas-system',
      'truenas-disk-sda',
      'truenas-pool-archive',
    ]);
    expect(model.incidents[0]).toMatchObject({
      severityBucket: 'critical',
      code: 'truenas_volume_status',
      summary: 'Pool tank is FAULTED',
      action: 'Investigate storage health immediately',
    });
    expect(model.incidents[2]).toMatchObject({
      id: 'truenas-pool-archive:incident:rollup',
      code: 'truenas_scrub',
      summary: 'Last scrub found checksum errors',
    });
    expect(mapTrueNASIncidentSeverity('WARNING')).toBe('warning');
    expect(
      filterTrueNASIncidents(model.incidents, 'smart', 'warning').map(
        (incident) => incident.resourceId,
      ),
    ).toEqual(['truenas-disk-sda']);
  });

  it('classifies and filters TrueNAS protection points from canonical recovery payloads', () => {
    const snapshot = makeRecoveryPoint({
      id: 'snapshot-tank-apps',
      kind: 'snapshot',
      mode: 'snapshot',
      display: {
        itemLabel: 'tank/apps',
        detailsSummary: 'auto-20260331-0600',
        nodeHostLabel: 'truenas-main',
      },
      itemRef: { type: 'truenas-dataset', name: 'tank/apps', id: 'tank/apps' },
      details: {
        dataset: 'tank/apps',
        fullName: 'tank/apps@auto-20260331-0600',
        hostname: 'truenas-main',
        snapshot: 'auto-20260331-0600',
      },
    });
    const replication = makeRecoveryPoint({
      id: 'replicate-tank-apps',
      kind: 'backup',
      mode: 'remote',
      outcome: 'running',
      display: {
        itemLabel: 'tank/apps',
        repositoryLabel: 'vault/compliance/tank_apps',
        detailsSummary: 'replicate-tank-apps (tank/apps@auto-20260331-0600)',
      },
      repositoryRef: {
        type: 'truenas-dataset',
        name: 'vault/compliance/tank_apps',
        id: 'vault/compliance/tank_apps',
      },
      details: {
        direction: 'PUSH',
        lastSnapshot: 'tank/apps@auto-20260331-0600',
        lastState: 'RUNNING',
        sourceDatasets: ['tank/apps'],
        targetDataset: 'vault/compliance/tank_apps',
        taskId: 'rep-task-tank-apps',
        taskName: 'replicate-tank-apps',
      },
    });
    const legacyReplication = makeRecoveryPoint({
      id: 'legacy-task',
      kind: 'other',
      mode: 'local',
      outcome: 'warning',
      details: {
        targetDataset: 'offsite/archive_backups',
        taskName: 'replicate-archive-backups',
      },
    });

    expect(mapTrueNASProtectionKind(snapshot)).toBe('snapshot');
    expect(mapTrueNASProtectionKind(replication)).toBe('replication');
    expect(mapTrueNASProtectionKind(legacyReplication)).toBe('replication');
    expect(mapTrueNASProtectionStatus(replication)).toBe('running');
    expect(mapTrueNASProtectionStatus(legacyReplication)).toBe('warning');

    expect(
      filterTrueNASProtectionPoints(
        [snapshot, replication, legacyReplication],
        'vault',
        'running',
      ).map((point) => point.id),
    ).toEqual(['replicate-tank-apps']);
    expect(
      filterTrueNASProtectionPoints(
        [snapshot, replication, legacyReplication],
        'auto-20260331',
        'all',
      ).map((point) => point.id),
    ).toEqual(['snapshot-tank-apps', 'replicate-tank-apps']);
    expect(
      filterTrueNASProtectionPoints(
        [snapshot, replication, legacyReplication],
        'replication',
        'all',
      ).map((point) => point.id),
    ).toEqual(['replicate-tank-apps', 'legacy-task']);
  });

  it('orders TrueNAS protection points by latest recovery timestamp', () => {
    const older = makeRecoveryPoint({
      id: 'older-snapshot',
      kind: 'snapshot',
      mode: 'snapshot',
      completedAt: '2026-05-19T00:00:00Z',
    });
    const newer = makeRecoveryPoint({
      id: 'newer-replication',
      kind: 'backup',
      mode: 'remote',
      completedAt: '2026-05-21T00:00:00Z',
    });

    expect(sortTrueNASProtectionPoints([older, newer]).map((point) => point.id)).toEqual([
      'newer-replication',
      'older-snapshot',
    ]);
  });
});
