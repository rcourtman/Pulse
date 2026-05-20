import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  TRUENAS_TAB_SPECS,
  buildTrueNASPageModel,
  buildTrueNASSystemChildCounts,
  filterTrueNASApps,
  filterTrueNASIncidents,
  filterTrueNASShares,
  filterTrueNASVMs,
  mapTrueNASAppStatus,
  mapTrueNASIncidentSeverity,
  mapTrueNASShareStatus,
  mapTrueNASVMStatus,
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

describe('truenasPageModel', () => {
  it('declares the TrueNAS section set as Overview + Storage + Protection', () => {
    expect(TRUENAS_TAB_SPECS.map((tab) => tab.id)).toEqual(['overview', 'storage', 'protection']);
  });

  it('buckets systems and apps while keeping storage inventory in scope for shared surfaces', () => {
    const model = buildTrueNASPageModel([
      makeResource({ id: 'truenas-system', type: 'agent' }),
      makeResource({ id: 'truenas-vm', type: 'vm' }),
      makeResource({ id: 'truenas-app', type: 'app-container' }),
      makeResource({ id: 'truenas-share', type: 'network-share' }),
      makeResource({ id: 'truenas-pool', type: 'pool' }),
      makeResource({ id: 'truenas-disk', type: 'physical_disk' }),
      makeResource({ id: 'docker-host', type: 'agent', platformType: 'docker' }),
      makeResource({ id: 'pve-node', type: 'agent', platformType: 'proxmox-pve' }),
    ]);

    expect(model.systems.map((r) => r.id)).toEqual(['truenas-system']);
    expect(model.shares.map((r) => r.id)).toEqual(['truenas-share']);
    expect(model.vms.map((r) => r.id)).toEqual(['truenas-vm']);
    expect(model.apps.map((r) => r.id)).toEqual(['truenas-app']);
    expect(model.resources.map((r) => r.id).sort()).toEqual(
      [
        'truenas-app',
        'truenas-disk',
        'truenas-pool',
        'truenas-share',
        'truenas-system',
        'truenas-vm',
      ].sort(),
    );
  });

  it('counts overview inventory from each TrueNAS system hierarchy', () => {
    const primary = makeResource({ id: 'system-primary', type: 'agent', name: 'nas-primary' });
    const backup = makeResource({ id: 'system-backup', type: 'agent', name: 'nas-backup' });
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
        makeResource({ id: 'primary-app', type: 'app-container', parentId: primary.id }),
        makeResource({ id: 'backup-app', type: 'app-container', parentId: backup.id }),
      ],
      [primary, backup],
    );

    expect(counts.get(primary.id)).toEqual({
      pools: 1,
      datasets: 1,
      shares: 1,
      apps: 1,
      disks: 1,
    });
    expect(counts.get(backup.id)).toEqual({
      pools: 1,
      datasets: 1,
      shares: 1,
      apps: 1,
      disks: 1,
    });
  });

  it('keeps the single-system inventory fallback for older unparented TrueNAS resources', () => {
    const system = makeResource({ id: 'system-primary', type: 'agent' });
    const counts = buildTrueNASSystemChildCounts(
      [
        system,
        makeResource({ id: 'pool-tank', type: 'storage', storage: { topology: 'pool' } }),
        makeResource({ id: 'dataset-media', type: 'storage', storage: { topology: 'dataset' } }),
        makeResource({ id: 'share-media', type: 'network-share' }),
        makeResource({ id: 'disk-sda', type: 'physical_disk' }),
        makeResource({ id: 'app-nextcloud', type: 'app-container' }),
      ],
      [system],
    );

    expect(counts.get(system.id)).toEqual({
      pools: 1,
      datasets: 1,
      shares: 1,
      apps: 1,
      disks: 1,
    });
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
});
