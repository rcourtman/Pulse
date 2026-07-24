import { describe, expect, it } from 'vitest';
import {
  buildTrueNASDetailSections,
  buildTrueNASDetailsSummary,
} from '@/components/Infrastructure/resourceDetailDrawerTrueNASModel';
import type { Resource } from '@/types/resource';

const baseResource = (overrides: Partial<Resource>): Resource =>
  ({
    id: 'truenas-resource',
    type: 'vm',
    name: 'truenas-resource',
    displayName: 'TrueNAS resource',
    platformId: 'truenas-main',
    platformType: 'truenas',
    sourceType: 'api',
    status: 'online',
    ...overrides,
  }) as Resource;

describe('resourceDetailDrawerTrueNASModel', () => {
  it('summarizes native TrueNAS system metadata for the detail drawer', () => {
    const resource = baseResource({
      type: 'agent',
      displayName: 'truenas-main',
      uptime: 42 * 86_400,
      truenas: {
        hostname: 'truenas-main',
        version: 'TrueNAS-SCALE-24.10.2',
        storageRisk: { level: 'warning' },
        storageRiskSummary: 'ZFS pool archive is DEGRADED',
        storagePostureSummary: 'One pool needs attention',
        protectionReduced: true,
        protectionSummary: 'Snapshots are current but replication is degraded',
        services: [
          { id: '1', service: 'smb', enabled: true, state: 'RUNNING', pids: [2418, 2420] },
          { id: '2', service: 'nfs', enabled: true, state: 'RUNNING', pids: [2501] },
          { id: '3', service: 'ssh', enabled: false, state: 'STOPPED' },
          { id: '4', service: 'smartd', enabled: true, state: 'STOPPED' },
        ],
      },
    });

    expect(buildTrueNASDetailsSummary(resource)).toBe(
      'TrueNAS-SCALE-24.10.2, 42d, 4 services, ZFS pool archive is DEGRADED',
    );
    expect(buildTrueNASDetailSections(resource).map((section) => section.label)).toEqual([
      'System',
      'Storage Health',
      'Services',
    ]);
    expect(buildTrueNASDetailSections(resource)[2]?.rows).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ label: 'Running', value: '2' }),
        expect.objectContaining({ label: 'Disabled', value: '1' }),
        expect.objectContaining({ label: 'Names', value: 'SMB, NFS, SSH, SMART' }),
      ]),
    );
  });

  it('summarizes native vm.query metadata for the detail drawer', () => {
    const resource = baseResource({
      type: 'vm',
      truenas: {
        vm: {
          name: 'ubuntu-build',
          state: 'RUNNING',
          vcpus: 4,
          memoryBytes: 8 * 1024 ** 3,
          deviceCount: 3,
          diskCount: 1,
          nicCount: 1,
          bootloader: 'UEFI',
          secureBoot: true,
          trustedPlatformModule: true,
        },
      },
    });

    expect(buildTrueNASDetailsSummary(resource)).toBe('Running, 4 vCPU, 8.00 GB, 3 devices');
    expect(buildTrueNASDetailSections(resource).map((section) => section.label)).toEqual([
      'Compute',
      'Runtime',
      'Devices',
      'Flags',
    ]);
  });

  it('summarizes native app.query metadata for the detail drawer', () => {
    const resource = baseResource({
      type: 'app-container',
      truenas: {
        app: {
          name: 'nextcloud',
          state: 'RUNNING',
          humanVersion: '29.0.0',
          containerCount: 2,
          upgradeAvailable: true,
          imageUpdatesAvailable: false,
          usedHostIps: ['0.0.0.0'],
          usedPorts: [
            {
              containerPort: 443,
              protocol: 'tcp',
              hostPorts: [{ hostIp: '0.0.0.0', hostPort: 30443 }],
            },
            {
              containerPort: 80,
              protocol: 'tcp',
              hostPorts: [{ hostIp: '0.0.0.0', hostPort: 30080 }],
            },
          ],
          volumes: [{ source: '/mnt/tank/apps/nextcloud', destination: '/data' }],
          images: ['nextcloud:29'],
        },
      },
    });

    expect(buildTrueNASDetailsSummary(resource)).toBe('Running, 2 containers, 2 ports, 1 update');
    expect(buildTrueNASDetailSections(resource).map((section) => section.label)).toEqual([
      'App',
      'Networking',
      'Storage',
    ]);
  });

  it('summarizes native SMB and NFS share metadata for the detail drawer', () => {
    const resource = baseResource({
      type: 'network-share',
      truenas: {
        share: {
          name: 'Media',
          protocol: 'SMB',
          dataset: 'tank/media',
          path: '/mnt/tank/media',
          enabled: true,
          readOnly: false,
          browsable: true,
          accessBasedEnumeration: true,
          auditEnabled: true,
          aliases: ['media'],
        },
      },
    });

    expect(buildTrueNASDetailsSummary(resource)).toBe('SMB, Enabled, tank/media, Read/write');
    expect(buildTrueNASDetailSections(resource).map((section) => section.label)).toEqual([
      'Share',
      'Access',
      'Clients',
    ]);
  });

  it('summarizes native TrueNAS storage pool metadata for the detail drawer', () => {
    const resource = baseResource({
      type: 'storage',
      displayName: 'archive',
      status: 'degraded',
      platformScopes: ['truenas'],
      disk: {
        current: 35.2,
        used: 25.3 * 1024 ** 4,
        total: 72 * 1024 ** 4,
      },
      childCount: 4,
      storage: {
        type: 'zfs-pool',
        topology: 'pool',
        platform: 'truenas',
        protection: 'zfs',
        zfsPoolState: 'DEGRADED',
        risk: {
          level: 'warning',
          reasons: [
            {
              code: 'zfs_pool_state',
              severity: 'warning',
              summary: 'ZFS pool archive is DEGRADED',
            },
          ],
        },
        riskSummary: 'ZFS pool archive is DEGRADED',
        protectionReduced: true,
        protectionSummary: 'ZFS pool archive is DEGRADED',
      },
    });

    expect(buildTrueNASDetailsSummary(resource)).toBe(
      'Pool, DEGRADED, 25.3 TB / 72.0 TB, ZFS pool archive is DEGRADED',
    );
    expect(buildTrueNASDetailSections(resource).map((section) => section.label)).toEqual([
      'Storage',
      'Capacity',
      'Health',
    ]);
    expect(buildTrueNASDetailSections(resource)[0]?.rows).toEqual(
      expect.arrayContaining([expect.objectContaining({ label: 'Protection', value: 'ZFS' })]),
    );
  });

  it('renders actionable canonical and native ZFS evidence without inventing a failed disk', () => {
    const resource = baseResource({
      type: 'storage',
      displayName: 'tank',
      status: 'degraded',
      platformScopes: ['truenas'],
      storage: {
        type: 'zfs-pool',
        topology: 'mirror',
        platform: 'truenas',
        zfsPoolState: 'DEGRADED',
        poolHealth: {
          scope: 'pool',
          provider: 'truenas',
          nativeId: 'pool-guid',
          canonicalState: 'DEGRADED',
          nativeState: 'DEGRADED',
          severity: 'critical',
          summary: 'Pool tank is degraded with a missing mirror member',
          recommendation: 'Confirm the affected mirror member in TrueNAS before replacement.',
          source: 'pool.query',
          evidenceCodes: ['zfs_pool_state', 'zfs_device_missing'],
        },
        zfsPool: {
          name: 'tank',
          state: 'DEGRADED',
          status: 'Degraded',
          scan: 'RESILVER SCANNING',
          scanDetails: {
            function: 'RESILVER',
            state: 'SCANNING',
            percentage: 33.3,
          },
          readErrors: 1,
          writeErrors: 0,
          checksumErrors: 2,
          devices: [
            {
              name: 'missing',
              type: 'UNAVAIL_DISK',
              role: 'data',
              parent: 'mirror-0',
              path: '/dev/disk/by-partuuid/missing',
              state: 'UNAVAIL',
              readErrors: 1,
              writeErrors: 0,
              checksumErrors: 0,
              missing: true,
            },
          ],
        },
      },
    });

    const healthRows = buildTrueNASDetailSections(resource).find(
      (section) => section.label === 'Health',
    )?.rows;
    expect(healthRows).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ label: 'Canonical state', value: 'DEGRADED' }),
        expect.objectContaining({ label: 'Native state', value: 'DEGRADED' }),
        expect.objectContaining({
          label: 'Scan / resilver',
          value: 'Resilver Scanning (33.3%)',
        }),
        expect.objectContaining({
          label: 'ZFS errors',
          value: 'Read 1 · Write 0 · Checksum 2',
        }),
        expect.objectContaining({
          label: 'Affected vdevs',
          value: '/dev/disk/by-partuuid/missing (Data / Unavail Disk): Missing · R 1 W 0 C 0',
        }),
        expect.objectContaining({
          label: 'Recommended',
          value: 'Confirm the affected mirror member in TrueNAS before replacement.',
        }),
        expect.objectContaining({
          label: 'Evidence',
          value: 'zfs_pool_state, zfs_device_missing',
        }),
        expect.objectContaining({ label: 'Evidence source', value: 'pool.query' }),
      ]),
    );
  });

  it('summarizes native TrueNAS physical disk metadata for the detail drawer', () => {
    const resource = baseResource({
      type: 'physical_disk',
      displayName: 'sdc',
      platformScopes: ['truenas'],
      physicalDisk: {
        devPath: '/dev/sdc',
        model: 'WD Red Pro',
        serial: 'WD-WX12A3456',
        diskType: 'sata',
        sizeBytes: 24 * 1024 ** 4,
        health: 'DEGRADED',
        temperature: 39,
        rpm: 7200,
        smart: {
          powerOnHours: 10_240,
          reallocatedSectors: 4,
          pendingSectors: 1,
        },
      },
    });

    expect(buildTrueNASDetailsSummary(resource)).toBe('SATA, Degraded, 24.0 TB, 39°C');
    expect(buildTrueNASDetailSections(resource).map((section) => section.label)).toEqual([
      'Disk',
      'Health',
      'SMART',
    ]);
  });
});
