import { describe, expect, it } from 'vitest';
import {
  buildTrueNASDetailSections,
  buildTrueNASDetailsSummary,
  type ResourceDetailDrawerTrueNASRow,
} from '@/components/Infrastructure/resourceDetailDrawerTrueNASModel';
import type {
  Resource,
  ResourcePhysicalDiskMeta,
  ResourceStorageMeta,
  ResourceTrueNASAppMeta,
  ResourceTrueNASAppPort,
  ResourceTrueNASMeta,
  ResourceTrueNASShareMeta,
  ResourceTrueNASVMMeta,
} from '@/types/resource';

// Every target helper (except the exported buildTrueNASDetailsSummary) is
// module-private, so each case drives it through the two public entry points
// (buildTrueNASDetailSections / buildTrueNASDetailsSummary) and asserts on the
// rendered summary string or section/row shape.  Fixture builders mirror the
// sibling test's `baseResource` convention.

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

// --- section / row accessors ---------------------------------------------

const allRows = (resource: Resource): ResourceDetailDrawerTrueNASRow[] =>
  buildTrueNASDetailSections(resource).flatMap((section) => section.rows);

const findRow = (resource: Resource, label: string): ResourceDetailDrawerTrueNASRow | undefined =>
  allRows(resource).find((row) => row.label === label);

// --- fixture builders -----------------------------------------------------

const systemResource = (truenas: Partial<ResourceTrueNASMeta>): Resource =>
  baseResource({ type: 'agent', truenas });

const storageRes = (
  storage: Partial<ResourceStorageMeta>,
  overrides: Partial<Resource> = {},
): Resource => baseResource({ storage, ...overrides });

const diskRes = (disk: Partial<ResourcePhysicalDiskMeta>): Resource =>
  baseResource({ physicalDisk: disk });

const shareRes = (share: Partial<ResourceTrueNASShareMeta>): Resource =>
  baseResource({ type: 'network-share', truenas: { share } });

const vmRes = (vm: Partial<ResourceTrueNASVMMeta>): Resource =>
  baseResource({ type: 'vm', truenas: { vm } });

const appRes = (app: Partial<ResourceTrueNASAppMeta>): Resource =>
  baseResource({ type: 'app-container', truenas: { app } });

// =========================================================================
// formatPercent
// =========================================================================

describe('formatPercent (via Storage "Percent" and disk SMART rows)', () => {
  it('formats >= 10 with one decimal', () => {
    expect(findRow(storageRes({}, { disk: { current: 42.5 } }), 'Percent')?.value).toBe('42.5%');
  });

  it('formats < 10 with two decimals', () => {
    expect(findRow(storageRes({}, { disk: { current: 5.5 } }), 'Percent')?.value).toBe('5.50%');
  });

  it('treats the >= 10 boundary as one decimal', () => {
    expect(findRow(storageRes({}, { disk: { current: 10 } }), 'Percent')?.value).toBe('10.0%');
  });

  it('treats 9.99 as two decimals', () => {
    expect(findRow(storageRes({}, { disk: { current: 9.99 } }), 'Percent')?.value).toBe('9.99%');
  });

  it('omits the row for undefined', () => {
    expect(findRow(storageRes({}), 'Percent')).toBeUndefined();
  });

  it('omits the row for NaN', () => {
    expect(findRow(storageRes({}, { disk: { current: Number.NaN } }), 'Percent')).toBeUndefined();
  });

  it('omits the row for a non-number value', () => {
    expect(
      findRow(storageRes({}, { disk: { current: 'lots' as unknown as number } }), 'Percent'),
    ).toBeUndefined();
  });

  it('works through the disk "Available spare" SMART row', () => {
    expect(findRow(diskRes({ smart: { availableSpare: 100 } }), 'Available spare')?.value).toBe(
      '100.0%',
    );
  });

  it('works through the disk "Percentage used" SMART row with a small value', () => {
    expect(findRow(diskRes({ smart: { percentageUsed: 0.5 } }), 'Percentage used')?.value).toBe(
      '0.50%',
    );
  });
});

// =========================================================================
// formatDurationSeconds
// =========================================================================

describe('formatDurationSeconds (via System "Uptime" row)', () => {
  it('formats multi-day durations', () => {
    expect(findRow(systemResource({ uptimeSeconds: 172_800 }), 'Uptime')?.value).toBe('2d');
  });

  it('formats exactly one day', () => {
    expect(findRow(systemResource({ uptimeSeconds: 86_400 }), 'Uptime')?.value).toBe('1d');
  });

  it('formats hours when less than a day', () => {
    expect(findRow(systemResource({ uptimeSeconds: 7_200 }), 'Uptime')?.value).toBe('2h');
  });

  it('formats minutes when less than an hour', () => {
    expect(findRow(systemResource({ uptimeSeconds: 120 }), 'Uptime')?.value).toBe('2m');
  });

  it('returns "<1m" for sub-minute durations', () => {
    expect(findRow(systemResource({ uptimeSeconds: 30 }), 'Uptime')?.value).toBe('<1m');
  });

  it('omits the row for undefined uptime', () => {
    expect(findRow(systemResource({}), 'Uptime')).toBeUndefined();
  });

  it('omits the row for zero uptime', () => {
    expect(findRow(systemResource({ uptimeSeconds: 0 }), 'Uptime')).toBeUndefined();
  });

  it('omits the row for negative uptime', () => {
    expect(findRow(systemResource({ uptimeSeconds: -100 }), 'Uptime')).toBeUndefined();
  });

  it('omits the row for NaN uptime', () => {
    expect(findRow(systemResource({ uptimeSeconds: Number.NaN }), 'Uptime')).toBeUndefined();
  });

  it('falls back to resource.uptime when uptimeSeconds is absent', () => {
    const resource = baseResource({ type: 'agent', uptime: 3_600, truenas: {} });
    expect(findRow(resource, 'Uptime')?.value).toBe('1h');
  });
});

// =========================================================================
// summarizeList
// =========================================================================

describe('summarizeList (via App list rows)', () => {
  it('joins values within the default visible count', () => {
    expect(findRow(appRes({ usedHostIps: ['10.0.0.1', '10.0.0.2'] }), 'Host IPs')?.value).toBe(
      '10.0.0.1, 10.0.0.2',
    );
  });

  it('appends a +N suffix when values exceed the visible count', () => {
    expect(findRow(appRes({ usedHostIps: ['a', 'b', 'c', 'd', 'e'] }), 'Host IPs')?.value).toBe(
      'a, b, c +2',
    );
  });

  it('omits the row when the list is empty', () => {
    expect(findRow(appRes({ usedHostIps: [] }), 'Host IPs')).toBeUndefined();
  });

  it('trims and filters whitespace-only entries', () => {
    expect(findRow(appRes({ usedHostIps: ['  x  ', '', '  ', 'y'] }), 'Host IPs')?.value).toBe(
      'x, y',
    );
  });

  it('respects a custom visible count (Images uses 2)', () => {
    expect(findRow(appRes({ images: ['img1', 'img2', 'img3', 'img4'] }), 'Images')?.value).toBe(
      'img1, img2 +2',
    );
  });

  it('shows no suffix when count equals the visible count', () => {
    expect(findRow(appRes({ images: ['img1', 'img2'] }), 'Images')?.value).toBe('img1, img2');
  });

  it('populates the title attribute with the full list', () => {
    expect(findRow(appRes({ usedHostIps: ['a', 'b', 'c', 'd'] }), 'Host IPs')?.title).toBe(
      'a, b, c, d',
    );
  });
});

// =========================================================================
// portLabel
// =========================================================================

describe('portLabel (via App "Ports" row)', () => {
  type PortCase = { name: string; port: ResourceTrueNASAppPort; expected: string | undefined };

  const cases: PortCase[] = [
    {
      name: 'host -> container/protocol',
      port: { containerPort: 443, protocol: 'tcp', hostPorts: [{ hostPort: 30443 }] },
      expected: '30443 -> 443/tcp',
    },
    {
      name: 'host with IP -> container/protocol',
      port: {
        containerPort: 443,
        protocol: 'tcp',
        hostPorts: [{ hostIp: '0.0.0.0', hostPort: 30443 }],
      },
      expected: '0.0.0.0:30443 -> 443/tcp',
    },
    {
      name: 'containerPort only with protocol',
      port: { containerPort: 443, protocol: 'tcp' },
      expected: '443/tcp',
    },
    {
      name: 'containerPort only without protocol',
      port: { containerPort: 443 },
      expected: '443',
    },
    {
      name: 'hostPorts only, no containerPort',
      port: { hostPorts: [{ hostPort: 30443 }] },
      expected: '30443',
    },
    {
      name: 'protocol only, no containerPort or hostPorts',
      port: { protocol: 'tcp' },
      expected: 'tcp',
    },
    {
      name: 'multiple hostPorts',
      port: {
        containerPort: 443,
        protocol: 'tcp',
        hostPorts: [
          { hostIp: '0.0.0.0', hostPort: 30443 },
          { hostIp: '127.0.0.1', hostPort: 30444 },
        ],
      },
      expected: '0.0.0.0:30443, 127.0.0.1:30444 -> 443/tcp',
    },
  ];

  for (const { name, port, expected } of cases) {
    it(name, () => {
      expect(findRow(appRes({ usedPorts: [port] }), 'Ports')?.value).toBe(expected);
    });
  }

  it('filters out invalid (non-positive) host port numbers', () => {
    const port: ResourceTrueNASAppPort = { hostPorts: [{ hostPort: 0 }] };
    expect(findRow(appRes({ usedPorts: [port] }), 'Ports')).toBeUndefined();
  });

  it('uses hostIp-less format when hostIp is whitespace-only', () => {
    const port: ResourceTrueNASAppPort = {
      containerPort: 80,
      hostPorts: [{ hostIp: '   ', hostPort: 30080 }],
    };
    expect(findRow(appRes({ usedPorts: [port] }), 'Ports')?.value).toBe('30080 -> 80');
  });

  it('filters a port whose only hostPort entry has a negative port', () => {
    const port: ResourceTrueNASAppPort = { hostPorts: [{ hostPort: -1 }] };
    expect(findRow(appRes({ usedPorts: [port] }), 'Ports')).toBeUndefined();
  });
});

// =========================================================================
// appVolumeLabels
// =========================================================================

describe('appVolumeLabels (via App "Volumes" row)', () => {
  it('formats source -> destination', () => {
    expect(
      findRow(appRes({ volumes: [{ source: '/mnt/tank/app', destination: '/data' }] }), 'Volumes')
        ?.value,
    ).toBe('/mnt/tank/app -> /data');
  });

  it('returns destination-only when source is absent', () => {
    expect(findRow(appRes({ volumes: [{ destination: '/data' }] }), 'Volumes')?.value).toBe(
      '/data',
    );
  });

  it('returns source-only when destination is absent', () => {
    expect(findRow(appRes({ volumes: [{ source: '/mnt/tank/app' }] }), 'Volumes')?.value).toBe(
      '/mnt/tank/app',
    );
  });

  it('filters volumes where neither source nor destination is present', () => {
    expect(
      findRow(appRes({ volumes: [{ source: '  ', destination: '' }] }), 'Volumes'),
    ).toBeUndefined();
  });
});

// =========================================================================
// appNetworkLabels
// =========================================================================

describe('appNetworkLabels (via App "Networks" row)', () => {
  it('uses the network name', () => {
    expect(findRow(appRes({ networks: [{ name: 'bridge' }] }), 'Networks')?.value).toBe('bridge');
  });

  it('falls back to id when name is absent', () => {
    expect(findRow(appRes({ networks: [{ id: 'net1' }] }), 'Networks')?.value).toBe('net1');
  });

  it('prefers name over id', () => {
    expect(findRow(appRes({ networks: [{ name: 'bridge', id: 'net1' }] }), 'Networks')?.value).toBe(
      'bridge',
    );
  });

  it('filters networks where neither name nor id is present', () => {
    expect(findRow(appRes({ networks: [{}] }), 'Networks')).toBeUndefined();
  });
});

// =========================================================================
// formatVMCpu
// =========================================================================

describe('formatVMCpu (via VM "vCPU" row)', () => {
  it('uses vcpus when present', () => {
    expect(findRow(vmRes({ vcpus: 4 }), 'vCPU')?.value).toBe('4 vCPU');
  });

  it('uses single vCPU correctly', () => {
    expect(findRow(vmRes({ vcpus: 1 }), 'vCPU')?.value).toBe('1 vCPU');
  });

  it('falls back to cores x threads when vcpus is absent', () => {
    expect(findRow(vmRes({ cores: 2, threads: 4 }), 'vCPU')?.value).toBe('2 cores x 4 threads');
  });

  it('falls back to cores-only when only cores is present', () => {
    expect(findRow(vmRes({ cores: 2 }), 'vCPU')?.value).toBe('2 cores');
  });

  it('falls back to threads-only when only threads is present', () => {
    expect(findRow(vmRes({ threads: 4 }), 'vCPU')?.value).toBe('4 threads');
  });

  it('omits the row when none of vcpus/cores/threads is present', () => {
    expect(findRow(vmRes({ state: 'running' }), 'vCPU')).toBeUndefined();
  });
});

// =========================================================================
// formatVMTopology
// =========================================================================

describe('formatVMTopology (via VM "Topology" row)', () => {
  it('formats cores x threads', () => {
    expect(findRow(vmRes({ cores: 2, threads: 4 }), 'Topology')?.value).toBe('2 cores x 4 threads');
  });

  it('formats cores-only', () => {
    expect(findRow(vmRes({ cores: 2 }), 'Topology')?.value).toBe('2 cores');
  });

  it('formats single core', () => {
    expect(findRow(vmRes({ cores: 1 }), 'Topology')?.value).toBe('1 core');
  });

  it('formats threads-only', () => {
    expect(findRow(vmRes({ threads: 4 }), 'Topology')?.value).toBe('4 threads');
  });

  it('omits the row when neither cores nor threads is present', () => {
    expect(findRow(vmRes({ vcpus: 4 }), 'Topology')).toBeUndefined();
  });
});

// =========================================================================
// storageStateTone
// =========================================================================

describe('storageStateTone (via Storage "State" row tone)', () => {
  it('returns success for zfsPoolState "online"', () => {
    expect(findRow(storageRes({ zfsPoolState: 'ONLINE' }), 'State')?.tone).toBe('success');
  });

  it('returns success for zfsPoolState "healthy"', () => {
    expect(findRow(storageRes({ zfsPoolState: 'HEALTHY' }), 'State')?.tone).toBe('success');
  });

  it('returns success for arrayState "mounted" when zfsPoolState is absent', () => {
    expect(findRow(storageRes({ arrayState: 'mounted' }), 'State')?.tone).toBe('success');
  });

  it('returns warning for zfsPoolState "degraded"', () => {
    expect(findRow(storageRes({ zfsPoolState: 'DEGRADED' }), 'State')?.tone).toBe('warning');
  });

  it('returns warning for arrayState "warning" when zfsPoolState is absent', () => {
    expect(findRow(storageRes({ arrayState: 'warning' }), 'State')?.tone).toBe('warning');
  });

  it('returns warning for zfsPoolState "offline"', () => {
    expect(findRow(storageRes({ zfsPoolState: 'OFFLINE' }), 'State')?.tone).toBe('warning');
  });

  it('returns default for an unrecognized state', () => {
    expect(findRow(storageRes({ zfsPoolState: 'SCRUBBING' }), 'State')?.tone).toBe('default');
  });

  it('prefers zfsPoolState over arrayState', () => {
    const resource = storageRes({ zfsPoolState: 'ONLINE', arrayState: 'degraded' });
    expect(findRow(resource, 'State')?.tone).toBe('success');
  });

  it('falls back to resource.status when both pool/array states are absent', () => {
    expect(findRow(storageRes({}, { status: 'degraded' }), 'State')?.tone).toBe('warning');
  });
});

// =========================================================================
// storageProtectionLabel
// =========================================================================

describe('storageProtectionLabel (via Storage "Protection" row)', () => {
  it('uppercases "zfs"', () => {
    expect(findRow(storageRes({ protection: 'zfs' }), 'Protection')?.value).toBe('ZFS');
  });

  it('uppercases "raidz1"', () => {
    expect(findRow(storageRes({ protection: 'raidz1' }), 'Protection')?.value).toBe('RAIDZ1');
  });

  it('uppercases "raidz2"', () => {
    expect(findRow(storageRes({ protection: 'raidz2' }), 'Protection')?.value).toBe('RAIDZ2');
  });

  it('uppercases bare "raidz"', () => {
    expect(findRow(storageRes({ protection: 'raidz' }), 'Protection')?.value).toBe('RAIDZ');
  });

  it('title-cases other protection values', () => {
    expect(findRow(storageRes({ protection: 'mirror' }), 'Protection')?.value).toBe('Mirror');
  });

  it('title-cases delimited values', () => {
    expect(findRow(storageRes({ protection: 'stripe_mirror' }), 'Protection')?.value).toBe(
      'Stripe Mirror',
    );
  });

  it('omits the row when protection is absent', () => {
    expect(findRow(storageRes({}), 'Protection')).toBeUndefined();
  });
});

// =========================================================================
// storageUsageLabel
// =========================================================================

describe('storageUsageLabel (via Storage "Usage" row)', () => {
  it('formats used / total when both are present', () => {
    expect(
      findRow(storageRes({}, { disk: { current: 0, used: 1024, total: 2048 } }), 'Usage')?.value,
    ).toBe('1.00 KB / 2.00 KB');
  });

  it('prefers used/total over percent', () => {
    expect(
      findRow(storageRes({}, { disk: { used: 1024, total: 2048, current: 50 } }), 'Usage')?.value,
    ).toBe('1.00 KB / 2.00 KB');
  });

  it('falls back to percent when used/total are absent', () => {
    expect(findRow(storageRes({}, { disk: { current: 42.5 } }), 'Usage')?.value).toBe('42.5%');
  });

  it('omits the row when nothing is present', () => {
    expect(findRow(storageRes({}), 'Usage')).toBeUndefined();
  });
});

// =========================================================================
// diskTypeLabel
// =========================================================================

describe('diskTypeLabel (via Disk "Type" row)', () => {
  type DiskTypeCase = { input: string; expected: string };

  const cases: DiskTypeCase[] = [
    { input: 'nvme', expected: 'NVMe' },
    { input: 'NVME', expected: 'NVMe' },
    { input: 'sata', expected: 'SATA' },
    { input: 'sas', expected: 'SAS' },
    { input: 'ssd', expected: 'SSD' },
    { input: 'hdd', expected: 'HDD' },
    { input: 'usb_flash', expected: 'Usb Flash' },
  ];

  for (const { input, expected } of cases) {
    it(`maps "${input}" to "${expected}"`, () => {
      expect(findRow(diskRes({ diskType: input }), 'Type')?.value).toBe(expected);
    });
  }

  it('omits the row when diskType is absent', () => {
    expect(findRow(diskRes({}), 'Type')).toBeUndefined();
  });
});

// =========================================================================
// serviceNameLabel
// =========================================================================

describe('serviceNameLabel (via System "Names" row)', () => {
  type SvcCase = { name: string; service?: string; id?: string; expected: string };

  const cases: SvcCase[] = [
    { name: 'ftp', service: 'ftp', id: '1', expected: 'FTP' },
    { name: 'nfs', service: 'nfs', id: '1', expected: 'NFS' },
    { name: 's3', service: 's3', id: '1', expected: 'S3' },
    { name: 'smb', service: 'smb', id: '1', expected: 'SMB' },
    { name: 'snmp', service: 'snmp', id: '1', expected: 'SNMP' },
    { name: 'ssh', service: 'ssh', id: '1', expected: 'SSH' },
    { name: 'ups', service: 'ups', id: '1', expected: 'UPS' },
    { name: 'smartd', service: 'smartd', id: '1', expected: 'SMART' },
    { name: 'arbitrary service', service: 'docker', id: '1', expected: 'Docker' },
    { name: 'id fallback', id: 'myservice', expected: 'Myservice' },
    { name: 'delimited id', id: 'web_server', expected: 'Web Server' },
  ];

  for (const { name, service, id, expected } of cases) {
    it(name, () => {
      const resource = systemResource({ services: [{ id, service }] });
      expect(findRow(resource, 'Names')?.value).toBe(expected);
    });
  }

  it('excludes a service from Names when neither service nor id is set', () => {
    const resource = systemResource({
      services: [{ id: '1', service: 'smb' }, {}],
    });
    expect(findRow(resource, 'Names')?.value).toBe('SMB');
  });
});

// =========================================================================
// shareStateLabel
// =========================================================================

describe('shareStateLabel (via Share "State" row value)', () => {
  it('returns "Disabled" when enabled === false', () => {
    expect(findRow(shareRes({ enabled: false }), 'State')?.value).toBe('Disabled');
  });

  it('returns "Locked" when locked is true and enabled is not false', () => {
    expect(findRow(shareRes({ locked: true }), 'State')?.value).toBe('Locked');
  });

  it('returns "Enabled" when enabled is true', () => {
    expect(findRow(shareRes({ enabled: true }), 'State')?.value).toBe('Enabled');
  });

  it('returns "Enabled" by default when neither flag is set', () => {
    expect(findRow(shareRes({}), 'State')?.value).toBe('Enabled');
  });
});

// =========================================================================
// shareStateTone
// =========================================================================

describe('shareStateTone (via Share "State" row tone)', () => {
  it('returns warning when enabled === false', () => {
    expect(findRow(shareRes({ enabled: false }), 'State')?.tone).toBe('warning');
  });

  it('returns warning when locked is true', () => {
    expect(findRow(shareRes({ locked: true }), 'State')?.tone).toBe('warning');
  });

  it('returns success when enabled and not locked', () => {
    expect(findRow(shareRes({ enabled: true }), 'State')?.tone).toBe('success');
  });
});

// =========================================================================
// shareModeLabel
// =========================================================================

describe('shareModeLabel (via Share "Mode" row)', () => {
  it('returns "Read-only" when readOnly === true', () => {
    expect(findRow(shareRes({ readOnly: true }), 'Mode')?.value).toBe('Read-only');
  });

  it('returns "Read/write" when readOnly === false', () => {
    expect(findRow(shareRes({ readOnly: false }), 'Mode')?.value).toBe('Read/write');
  });

  it('omits the row when readOnly is undefined', () => {
    expect(findRow(shareRes({}), 'Mode')).toBeUndefined();
  });
});

// =========================================================================
// shareUserGroupLabel
// =========================================================================

describe('shareUserGroupLabel (via Share "Map root" row)', () => {
  it('joins user and group with a colon', () => {
    expect(
      findRow(shareRes({ mapRootUser: 'root', mapRootGroup: 'wheel' }), 'Map root')?.value,
    ).toBe('root:wheel');
  });

  it('returns user-only when group is absent', () => {
    expect(findRow(shareRes({ mapRootUser: 'root' }), 'Map root')?.value).toBe('root');
  });

  it('returns group-only when user is absent', () => {
    expect(findRow(shareRes({ mapRootGroup: 'wheel' }), 'Map root')?.value).toBe('wheel');
  });

  it('omits the row when neither user nor group is set', () => {
    expect(findRow(shareRes({}), 'Map root')).toBeUndefined();
  });

  it('works for the "Map all" row independently', () => {
    expect(
      findRow(shareRes({ mapAllUser: 'nobody', mapAllGroup: 'nogroup' }), 'Map all')?.value,
    ).toBe('nobody:nogroup');
  });
});

// =========================================================================
// buildTrueNASDetailsSummary
// =========================================================================

describe('buildTrueNASDetailsSummary branch coverage', () => {
  it('returns null for a non-truenas-scoped resource', () => {
    expect(buildTrueNASDetailsSummary(baseResource({ platformType: 'docker' }))).toBeNull();
  });

  it('returns null for a bare VM with no data', () => {
    expect(buildTrueNASDetailsSummary(vmRes({}))).toBeNull();
  });

  it('returns null for a bare physical disk with no data', () => {
    expect(buildTrueNASDetailsSummary(diskRes({}))).toBeNull();
  });

  it('returns null for a bare truenas system with no data', () => {
    expect(buildTrueNASDetailsSummary(systemResource({}))).toBeNull();
  });

  it('summarizes a VM without a device count', () => {
    const resource = vmRes({ state: 'running', vcpus: 2, memoryBytes: 4 * 1024 ** 3 });
    expect(buildTrueNASDetailsSummary(resource)).toBe('Running, 2 vCPU, 4.00 GB');
  });

  it('summarizes an app with no available updates', () => {
    const resource = appRes({ state: 'running', containerCount: 1 });
    expect(buildTrueNASDetailsSummary(resource)).toBe('Running, 1 container, 0 ports');
  });

  it('summarizes an app with both update types available', () => {
    const resource = appRes({ upgradeAvailable: true, imageUpdatesAvailable: true });
    expect(buildTrueNASDetailsSummary(resource)).toBe('0 containers, 0 ports, 2 updates');
  });

  it('summarizes a disabled share as just "Disabled"', () => {
    expect(buildTrueNASDetailsSummary(shareRes({ enabled: false }))).toBe('Disabled');
  });

  it('summarizes a locked share as just "Locked"', () => {
    expect(buildTrueNASDetailsSummary(shareRes({ locked: true }))).toBe('Locked');
  });

  it('includes uptime in system summary via uptimeSeconds', () => {
    expect(
      buildTrueNASDetailsSummary(systemResource({ version: 'v1', uptimeSeconds: 86_400 })),
    ).toBe('v1, 1d');
  });

  it('includes service count in system summary', () => {
    const resource = systemResource({
      version: 'v1',
      services: [{ id: '1', service: 'smb' }],
    });
    expect(buildTrueNASDetailsSummary(resource)).toBe('v1, 1 service');
  });
});

// =========================================================================
// buildTrueNASAppSections
// =========================================================================

describe('buildTrueNASAppSections additional branches', () => {
  it('sets State tone to success when running', () => {
    expect(findRow(appRes({ state: 'running' }), 'State')?.tone).toBe('success');
  });

  it('sets State tone to warning when not running', () => {
    expect(findRow(appRes({ state: 'stopped' }), 'State')?.tone).toBe('warning');
  });

  it('shows "Available" with warning tone when upgrade is available', () => {
    const row = findRow(appRes({ upgradeAvailable: true }), 'App updates');
    expect(row?.value).toBe('Available');
    expect(row?.tone).toBe('warning');
  });

  it('shows "Current" with success tone when no upgrade', () => {
    const row = findRow(appRes({ upgradeAvailable: false }), 'App updates');
    expect(row?.value).toBe('Current');
    expect(row?.tone).toBe('success');
  });

  it('omits App updates when upgradeAvailable is undefined', () => {
    expect(findRow(appRes({ state: 'running' }), 'App updates')).toBeUndefined();
  });

  it('shows "Available" with warning tone when image updates are available', () => {
    const row = findRow(appRes({ imageUpdatesAvailable: true }), 'Image updates');
    expect(row?.value).toBe('Available');
    expect(row?.tone).toBe('warning');
  });

  it('shows "Current" with success tone when no image updates', () => {
    const row = findRow(appRes({ imageUpdatesAvailable: false }), 'Image updates');
    expect(row?.value).toBe('Current');
    expect(row?.tone).toBe('success');
  });

  it('omits Image updates when imageUpdatesAvailable is undefined', () => {
    expect(findRow(appRes({ state: 'running' }), 'Image updates')).toBeUndefined();
  });

  it('uses containerCount directly for Containers row', () => {
    expect(findRow(appRes({ containerCount: 3 }), 'Containers')?.value).toBe('3');
  });

  it('falls back to containers array length for Containers row', () => {
    expect(findRow(appRes({ containers: [{ id: 'c1' }, { id: 'c2' }] }), 'Containers')?.value).toBe(
      '2',
    );
  });

  it('prefers humanVersion over version', () => {
    expect(findRow(appRes({ humanVersion: '29.0.0', version: '29.0' }), 'Version')?.value).toBe(
      '29.0.0',
    );
  });

  it('falls back to version when humanVersion is absent', () => {
    expect(findRow(appRes({ version: '29.0' }), 'Version')?.value).toBe('29.0');
  });

  it('shows "Yes" for custom app', () => {
    expect(findRow(appRes({ customApp: true }), 'Custom app')?.value).toBe('Yes');
  });

  it('shows "No" for non-custom app', () => {
    expect(findRow(appRes({ customApp: false }), 'Custom app')?.value).toBe('No');
  });
});

// =========================================================================
// buildTrueNASVMSections
// =========================================================================

describe('buildTrueNASVMSections additional branches', () => {
  it('omits Domain state when state and domainState match (sameState)', () => {
    const resource = vmRes({ state: 'running', domainState: 'RUNNING', vcpus: 1 });
    expect(findRow(resource, 'Domain state')).toBeUndefined();
  });

  it('shows Domain state when state and domainState differ', () => {
    const resource = vmRes({ state: 'running', domainState: 'paused', vcpus: 1 });
    expect(findRow(resource, 'Domain state')?.value).toBe('Paused');
  });

  it('sets State tone to warning for a non-running state', () => {
    expect(findRow(vmRes({ state: 'stopped', vcpus: 1 }), 'State')?.tone).toBe('warning');
  });

  it('uses domainState for State when state is absent', () => {
    const resource = vmRes({ domainState: 'running', vcpus: 1 });
    expect(findRow(resource, 'State')?.value).toBe('Running');
    expect(findRow(resource, 'State')?.tone).toBe('success');
  });

  it('joins machineType and archType in the Machine row', () => {
    expect(
      findRow(vmRes({ vcpus: 1, machineType: 'q35', archType: 'x86_64' }), 'Machine')?.value,
    ).toBe('q35 / x86_64');
  });

  it('shows only machineType when archType is absent', () => {
    expect(findRow(vmRes({ vcpus: 1, machineType: 'q35' }), 'Machine')?.value).toBe('q35');
  });

  it('omits the Machine row when both are absent', () => {
    expect(findRow(vmRes({ vcpus: 1 }), 'Machine')).toBeUndefined();
  });
});

// =========================================================================
// buildTrueNASDiskSections
// =========================================================================

describe('buildTrueNASDiskSections additional branches', () => {
  it('sets Temperature tone to warning at 55 degrees', () => {
    expect(findRow(diskRes({ temperature: 55 }), 'Temperature')?.tone).toBe('warning');
  });

  it('sets Temperature tone to warning above 55 degrees', () => {
    expect(findRow(diskRes({ temperature: 60 }), 'Temperature')?.tone).toBe('warning');
  });

  it('sets Temperature tone to default below 55 degrees', () => {
    expect(findRow(diskRes({ temperature: 40 }), 'Temperature')?.tone).toBe('default');
  });

  it('formats Wearout percentage for a valid value', () => {
    expect(findRow(diskRes({ wearout: 50 }), 'Wearout')?.value).toBe('50.0%');
  });

  it('omits Wearout for a negative value', () => {
    expect(findRow(diskRes({ wearout: -1 }), 'Wearout')).toBeUndefined();
  });

  it('omits Wearout when undefined', () => {
    expect(findRow(diskRes({ health: 'passed' }), 'Wearout')).toBeUndefined();
  });

  it('shows SMART reallocated sectors with warning tone when count > 0', () => {
    const row = findRow(diskRes({ smart: { reallocatedSectors: 4 } }), 'Reallocated');
    expect(row?.value).toBe('4');
    expect(row?.tone).toBe('warning');
  });

  it('shows SMART reallocated sectors with default tone when count is 0', () => {
    const row = findRow(diskRes({ smart: { reallocatedSectors: 0 } }), 'Reallocated');
    expect(row?.value).toBe('0');
    expect(row?.tone).toBe('default');
  });

  it('shows Risk reasons with summarizeList +N suffix', () => {
    const resource = diskRes({
      risk: {
        level: 'warning',
        reasons: [
          { code: 'r1', severity: 'warning', summary: 'reason one' },
          { code: 'r2', severity: 'warning', summary: 'reason two' },
          { code: 'r3', severity: 'warning', summary: 'reason three' },
        ],
      },
    });
    expect(findRow(resource, 'Reasons')?.value).toBe('reason one, reason two +1');
    expect(findRow(resource, 'Risk')?.value).toBe('Warning');
    expect(findRow(resource, 'Risk')?.tone).toBe('warning');
  });

  it('uses resource.name as Device fallback when devPath is absent', () => {
    expect(findRow(diskRes({}), 'Device')?.value).toBe('truenas-resource');
  });

  it('uses resource.parentName as Group fallback when storageGroup is absent', () => {
    const resource = baseResource({
      parentName: 'tank',
      physicalDisk: { health: 'passed' },
    });
    expect(findRow(resource, 'Group')?.value).toBe('tank');
  });
});

// =========================================================================
// buildTrueNASStorageSections
// =========================================================================

describe('buildTrueNASStorageSections additional branches', () => {
  it('uses resource.parentName as Pool fallback when storage.pool is absent', () => {
    const resource = baseResource({ parentName: 'tank', storage: {} });
    expect(findRow(resource, 'Pool')?.value).toBe('tank');
  });

  it('shows Risk level with warning tone', () => {
    const resource = storageRes({ risk: { level: 'warning' } });
    expect(findRow(resource, 'Risk')?.value).toBe('Warning');
    expect(findRow(resource, 'Risk')?.tone).toBe('warning');
  });

  it('shows Risk level with default tone for non-warning level', () => {
    const resource = storageRes({ risk: { level: 'ok' } });
    expect(findRow(resource, 'Risk')?.value).toBe('Ok');
    expect(findRow(resource, 'Risk')?.tone).toBe('default');
  });

  it('shows Protection reduced (booleanValue) with warning tone when true', () => {
    const row = findRow(storageRes({ protectionReduced: true }), 'Protection reduced');
    expect(row?.value).toBe('Enabled');
    expect(row?.tone).toBe('warning');
  });

  it('shows Protection reduced (booleanValue) with success tone when false', () => {
    const row = findRow(storageRes({ protectionReduced: false }), 'Protection reduced');
    expect(row?.value).toBe('Disabled');
    expect(row?.tone).toBe('success');
  });

  it('shows risk reasons with +N suffix', () => {
    const resource = storageRes({
      risk: {
        level: 'warning',
        reasons: [
          { code: 'r1', severity: 'warning', summary: 'reason one' },
          { code: 'r2', severity: 'warning', summary: 'reason two' },
          { code: 'r3', severity: 'warning', summary: 'reason three' },
        ],
      },
    });
    expect(findRow(resource, 'Reasons')?.value).toBe('reason one, reason two +1');
  });

  it('shows posture summary with warning tone when present', () => {
    const row = findRow(storageRes({ postureSummary: 'Needs attention' }), 'Posture');
    expect(row?.value).toBe('Needs attention');
    expect(row?.tone).toBe('warning');
  });

  it('omits Posture when absent', () => {
    expect(findRow(storageRes({}), 'Posture')).toBeUndefined();
  });
});

// =========================================================================
// buildTrueNASSystemSections
// =========================================================================

describe('buildTrueNASSystemSections additional branches', () => {
  it('shows Storage risk with warning tone when level is warning', () => {
    const row = findRow(systemResource({ storageRisk: { level: 'warning' } }), 'Storage risk');
    expect(row?.value).toBe('Warning');
    expect(row?.tone).toBe('warning');
  });

  it('shows Storage risk with default tone for a non-warning level', () => {
    const row = findRow(systemResource({ storageRisk: { level: 'ok' } }), 'Storage risk');
    expect(row?.value).toBe('Ok');
    expect(row?.tone).toBe('default');
  });

  it('shows Storage summary with warning tone', () => {
    const row = findRow(systemResource({ storageRiskSummary: 'Pool degraded' }), 'Storage summary');
    expect(row?.value).toBe('Pool degraded');
    expect(row?.tone).toBe('warning');
  });

  it('shows Protection reduced with warning tone when true', () => {
    const row = findRow(systemResource({ protectionReduced: true }), 'Protection reduced');
    expect(row?.value).toBe('Yes');
    expect(row?.tone).toBe('warning');
  });

  it('shows Protection reduced with success tone when false', () => {
    const row = findRow(systemResource({ protectionReduced: false }), 'Protection reduced');
    expect(row?.value).toBe('No');
    expect(row?.tone).toBe('success');
  });

  it('shows Rebuild active with warning tone when in progress', () => {
    const row = findRow(systemResource({ rebuildInProgress: true }), 'Rebuild active');
    expect(row?.value).toBe('Yes');
    expect(row?.tone).toBe('warning');
  });

  it('shows PIDs row with summarized list when pids are present', () => {
    const resource = systemResource({
      services: [{ id: '1', service: 'smb', pids: [100, 200] }],
    });
    expect(findRow(resource, 'PIDs')?.value).toBe('100, 200');
  });

  it('omits PIDs when all pids are non-positive', () => {
    const resource = systemResource({
      services: [{ id: '1', service: 'smb', pids: [0, -1] }],
    });
    expect(findRow(resource, 'PIDs')).toBeUndefined();
  });

  it('shows service count when services are present', () => {
    const resource = systemResource({
      services: [
        { id: '1', service: 'smb' },
        { id: '2', service: 'nfs' },
      ],
    });
    expect(findRow(resource, 'Services')?.value).toBe('2 services');
  });

  it('shows single service count correctly', () => {
    const resource = systemResource({ services: [{ id: '1', service: 'smb' }] });
    expect(findRow(resource, 'Services')?.value).toBe('1 service');
  });

  it('shows Hostname from truenas.hostname', () => {
    expect(findRow(systemResource({ hostname: 'nas01' }), 'Hostname')?.value).toBe('nas01');
  });

  it('falls back to resource.name when hostname is absent', () => {
    expect(findRow(systemResource({}), 'Hostname')?.value).toBe('truenas-resource');
  });
});

// =========================================================================
// formatDiskHours (additional coverage)
// =========================================================================

describe('formatDiskHours additional edge cases', () => {
  it('formats a value with thousands separator', () => {
    expect(findRow(diskRes({ smart: { powerOnHours: 12_345 } }), 'Power on')?.value).toBe(
      '12,345h',
    );
  });

  it('omits the row for a string value cast to number', () => {
    const resource = diskRes({ smart: { powerOnHours: 'lots' as unknown as number } });
    expect(findRow(resource, 'Power on')).toBeUndefined();
  });
});
