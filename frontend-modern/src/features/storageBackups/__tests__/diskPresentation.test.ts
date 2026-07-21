import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  buildPhysicalDiskPresentationDataMap,
  buildPhysicalDiskGroupFilterOptions,
  buildPhysicalDiskRoleFilterOptions,
  comparePhysicalDiskPresentation,
  extractPhysicalDiskPresentationData,
  filterAndSortPhysicalDisks,
  PHYSICAL_DISK_EMPTY_CARD_CLASS,
  PHYSICAL_DISK_ALL_GROUPS_FILTER_LABEL,
  PHYSICAL_DISK_ALL_ROLES_FILTER_LABEL,
  PHYSICAL_DISK_HEADER_DEVICE_CLASS,
  PHYSICAL_DISK_HEADER_DISK_CLASS,
  PHYSICAL_DISK_HEADER_LIFE_CLASS,
  PHYSICAL_DISK_NAME_TEXT_CLASS,
  PHYSICAL_DISK_SOURCE_BADGE_CLASS,
  PHYSICAL_DISK_TABLE_CLASS,
  PHYSICAL_DISK_TABLE_ROW_HOVER_CLASS,
  getPhysicalDiskEmptyStatePresentation,
  getPhysicalDiskCollectionMessages,
  getPhysicalDiskFieldStatusMessage,
  getPhysicalDiskHealthStatus,
  getPhysicalDiskHealthSummary,
  getPhysicalDiskHostLabel,
  getPhysicalDiskLifeLabel,
  getPhysicalDiskLifeTextClass,
  getPhysicalDiskNormalizedHealth,
  getPhysicalDiskParentLabel,
  getPhysicalDiskPlatformLabel,
  getPhysicalDiskRoleLabel,
  getPhysicalDiskRoleFilterValue,
  getPhysicalDiskSourceKey,
  getPhysicalDiskSourceBadgePresentation,
  hasUnraidPhysicalDiskFaultSignal,
  hasPhysicalDiskSmartWarning,
  isUnraidPhysicalDisk,
  matchesPhysicalDiskSearch,
  normalizePhysicalDiskFacetFilter,
  type PhysicalDiskPresentationData,
} from '@/features/storageBackups/diskPresentation';

function makeDiskData(
  overrides: Partial<PhysicalDiskPresentationData> = {},
): PhysicalDiskPresentationData {
  return {
    node: '',
    instance: '',
    devPath: '',
    model: '',
    serial: '',
    wwn: '',
    size: 0,
    health: 'UNKNOWN',
    riskReasons: [],
    wearout: -1,
    type: '',
    temperature: 0,
    rpm: 0,
    used: '',
    ...overrides,
  };
}

describe('diskPresentation', () => {
  it('distinguishes unsupported, unavailable, and unexpectedly missing disk evidence', () => {
    expect(
      getPhysicalDiskFieldStatusMessage('Disk I/O', {
        state: 'unsupported',
        source: 'controller',
        reason: 'per-member counters unavailable',
      }),
    ).toBe('Disk I/O is unsupported: per-member counters unavailable');
    expect(
      getPhysicalDiskFieldStatusMessage('Temperature', {
        state: 'unavailable',
        source: 'smartctl',
        reason: 'collection deadline exceeded',
      }),
    ).toBe('Temperature is temporarily unavailable: collection deadline exceeded');
    expect(
      getPhysicalDiskFieldStatusMessage('Serial number', {
        state: 'missing',
        source: 'smartctl',
        reason: 'serial absent from successful response',
      }),
    ).toBe('Serial number is unexpectedly missing: serial absent from successful response');
    expect(
      getPhysicalDiskCollectionMessages(
        makeDiskData({
          collection: {
            serial: { state: 'available', source: 'smartctl' },
            temperature: { state: 'unavailable', source: 'smartctl' },
            io: { state: 'unsupported', source: 'controller' },
            controller: { state: 'missing', source: 'linux-sysfs' },
            pool: { state: 'available', source: 'zpool-status' },
          },
        }),
      ),
    ).toEqual([
      'Temperature is temporarily unavailable.',
      'Disk I/O is unsupported.',
      'Controller association is unexpectedly missing.',
    ]);
  });

  it('returns critical presentation for failed disks', () => {
    expect(PHYSICAL_DISK_EMPTY_CARD_CLASS).toBe('text-center');
    expect(PHYSICAL_DISK_TABLE_CLASS).toBe('w-full table-fixed text-xs');
    expect(PHYSICAL_DISK_TABLE_ROW_HOVER_CLASS).toContain('hover:bg-surface-hover');
    expect(PHYSICAL_DISK_HEADER_DISK_CLASS).toContain('uppercase');
    expect(PHYSICAL_DISK_HEADER_DEVICE_CLASS).toContain('md:table-cell');
    expect(PHYSICAL_DISK_HEADER_LIFE_CLASS).toContain('md:table-cell');
    expect(PHYSICAL_DISK_NAME_TEXT_CLASS).toContain('font-semibold');
    expect(PHYSICAL_DISK_SOURCE_BADGE_CLASS).toContain('justify-center');

    expect(
      getPhysicalDiskHealthStatus({
        ...makeDiskData({
          health: 'FAILED',
          wearout: 20,
          type: 'ssd',
        }),
      }),
    ).toEqual({
      label: 'Replace Now',
      summary: 'Disk health has degraded to a critical state.',
      tone: 'text-red-700 dark:text-red-300',
    });
  });

  it('detects SMART warnings from counters', () => {
    expect(
      hasPhysicalDiskSmartWarning({
        ...makeDiskData({
          health: 'PASSED',
          wearout: 50,
          type: 'hdd',
        }),
        smartAttributes: { pendingSectors: 2 },
      }),
    ).toBe(true);
  });

  it('returns role, parent, and platform labels canonically', () => {
    expect(
      getPhysicalDiskRoleLabel({
        ...makeDiskData({
          health: 'PASSED',
          wearout: 50,
          type: 'ssd',
        }),
        storageRole: 'cache_pool',
      }),
    ).toBe('Cache Pool');
    expect(getPhysicalDiskRoleLabel(makeDiskData({ type: 'nvme' }))).toBe('NVMe disk');
    expect(getPhysicalDiskRoleLabel(makeDiskData({ type: 'sata' }))).toBe('SATA disk');
    expect(getPhysicalDiskRoleLabel(makeDiskData({ type: 'sas' }))).toBe('SAS disk');
    expect(
      getPhysicalDiskParentLabel({
        ...makeDiskData({
          health: 'PASSED',
          wearout: 50,
          type: 'ssd',
        }),
        storageGroup: 'tank',
      }),
    ).toBe('tank');
    expect(getPhysicalDiskParentLabel(makeDiskData({ storageGroup: '', used: 'ZFS' }))).toBe('ZFS');
    expect(getPhysicalDiskParentLabel(makeDiskData({ storageGroup: 'tank', used: 'ZFS' }))).toBe(
      'tank',
    );
    expect(getPhysicalDiskParentLabel(makeDiskData({ storageGroup: '', used: 'unknown' }))).toBe(
      '',
    );
    expect(getPhysicalDiskLifeLabel(makeDiskData({ wearout: 96 }))).toBe('96%');
    expect(getPhysicalDiskLifeLabel(makeDiskData({ wearout: 0 }))).toBe('');
    expect(getPhysicalDiskLifeLabel(makeDiskData({ wearout: -1 }))).toBe('');
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 96 }))).toContain('text-green');
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 35 }))).toContain('text-amber');
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 9 }))).toContain('text-red');
    expect(getPhysicalDiskPlatformLabel({} as Resource, 'PVE')).toBe('PVE');
    expect(
      getPhysicalDiskSourceBadgePresentation({
        platformType: 'proxmox-pve',
      } as Resource),
    ).toEqual({
      label: 'PVE',
      className:
        'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-400 inline-flex max-w-full min-w-0 justify-center overflow-hidden text-ellipsis px-1 sm:px-1.5 py-px text-[9px] font-medium',
    });
    expect(
      getPhysicalDiskSourceKey({
        platformType: 'agent',
        platformData: { sources: ['agent', 'proxmox'] },
      } as unknown as Resource),
    ).toBe('proxmox-pve');
    expect(
      getPhysicalDiskSourceBadgePresentation({
        platformType: 'agent',
        platformData: { sources: ['agent', 'proxmox'] },
      } as unknown as Resource).label,
    ).toBe('PVE');
  });

  it('returns host, health summary, and empty-state presentation canonically', () => {
    expect(
      getPhysicalDiskHostLabel(
        {
          ...makeDiskData({
            health: 'PASSED',
            wearout: 50,
            type: 'ssd',
          }),
          node: 'tower',
        },
        { parentName: 'fallback-host' } as Resource,
      ),
    ).toBe('tower');

    expect(
      getPhysicalDiskHealthSummary({
        label: 'Healthy',
        summary: 'No active disk-health issues.',
        tone: 'text-base-content',
      }),
    ).toBe('');

    expect(
      getPhysicalDiskHealthSummary({
        label: 'Needs Attention',
        summary: 'Pending sectors detected.',
        tone: 'text-amber-700 dark:text-amber-300',
      }),
    ).toBe('Pending sectors detected.');
    expect(getPhysicalDiskHealthStatus(makeDiskData({ health: 'UNKNOWN' })).label).toBe('Unknown');
    // PVE reports SCSI/SAS drives as OK; older servers pass it through raw (#1595)
    expect(getPhysicalDiskHealthStatus(makeDiskData({ health: 'OK' })).label).toBe('Healthy');

    expect(
      getPhysicalDiskEmptyStatePresentation({
        selectedNodeName: 'tower',
        searchTerm: '',
        diskCount: 0,
        hasPVENodes: true,
      }),
    ).toMatchObject({
      title: 'No physical disks found',
      nodeMessage: 'for node tower',
      filterMessages: [],
      showRequirements: true,
      fallbackMessage:
        'No Proxmox nodes configured. Add Proxmox VE in Settings → Infrastructure to monitor physical disks.',
      requirementsTitle: 'Physical disk monitoring requirements:',
      requirementsItems: expect.arrayContaining([
        'Enable "Monitor physical disk health (SMART)" in Settings → Infrastructure for the Proxmox node',
      ]),
    });

    expect(
      getPhysicalDiskEmptyStatePresentation({
        selectedNodeName: null,
        searchTerm: '',
        diskCount: 4,
        hasPVENodes: true,
        healthFilter: 'attention',
        sourceFilterLabel: 'PVE',
        roleFilterLabel: 'Cache Pool',
        groupFilterLabel: 'tank',
      }),
    ).toMatchObject({
      title: 'No disks need attention',
      filterMessages: ['from PVE', 'with role Cache Pool', 'in tank'],
      showRequirements: false,
    });
  });

  it('keeps Unraid disks with missing SMART data out of attention unless Unraid reports a fault', () => {
    const spunDownUnraidDisk = makeDiskData({
      health: 'UNKNOWN',
      storageRole: 'data',
      storageGroup: 'unraid-array',
      storageState: 'online',
      spunDown: true,
      temperature: 0,
    });
    const spunDownResource = {
      status: 'offline',
      platformType: 'agent',
      physicalDisk: {
        health: 'UNKNOWN',
        storageRole: 'data',
        storageGroup: 'unraid-array',
        storageState: 'online',
        spunDown: true,
      },
    } as unknown as Resource;

    expect(isUnraidPhysicalDisk(spunDownUnraidDisk)).toBe(true);
    expect(hasUnraidPhysicalDiskFaultSignal(spunDownUnraidDisk)).toBe(false);
    expect(getPhysicalDiskHealthStatus(spunDownUnraidDisk)).toMatchObject({
      label: 'Online',
      summary: 'No active disk-health issues.',
    });
    expect(getPhysicalDiskNormalizedHealth(spunDownResource, spunDownUnraidDisk)).toBe('healthy');

    const diskWithUnraidErrors = makeDiskData({
      health: 'UNKNOWN',
      riskLevel: 'warning',
      riskReasons: ['Unraid disk disk2 reports 3 error(s)'],
      storageRole: 'data',
      storageGroup: 'unraid-array',
      storageState: 'online',
      errorCount: 3,
    });
    expect(hasUnraidPhysicalDiskFaultSignal(diskWithUnraidErrors)).toBe(true);
    expect(getPhysicalDiskHealthStatus(diskWithUnraidErrors)).toMatchObject({
      label: 'Needs Attention',
      summary: 'Unraid disk disk2 reports 3 error(s)',
    });

    const missingUnraidDisk = makeDiskData({
      health: 'FAILED',
      storageRole: 'data',
      storageGroup: 'unraid-array',
      storageState: 'missing',
      riskLevel: 'critical',
      riskReasons: ['Unraid disk disk3 is MISSING'],
    });
    expect(hasUnraidPhysicalDiskFaultSignal(missingUnraidDisk)).toBe(true);
    expect(getPhysicalDiskHealthStatus(missingUnraidDisk)).toMatchObject({
      label: 'Replace Now',
      summary: 'Unraid disk disk3 is MISSING',
    });
  });

  it('extracts search and sort presentation canonically from resources', () => {
    const warningDisk = {
      id: 'disk-warning',
      name: 'disk-warning',
      physicalDisk: {
        devPath: '/dev/sdb',
        model: 'Cache SSD',
        serial: 'SERIAL-2',
        diskType: 'ssd',
        health: 'PASSED',
        storageRole: 'cache_pool',
        storageGroup: 'tank',
        risk: {
          level: 'warning',
          reasons: [{ summary: 'Pending sectors detected.' }],
        },
        smart: { pendingSectors: 2 },
      },
      identity: { hostname: 'tower' },
      canonicalIdentity: { hostname: 'tower' },
      platformType: 'proxmox-pve',
    } as unknown as Resource;

    const healthyDisk = {
      id: 'disk-healthy',
      name: 'disk-healthy',
      physicalDisk: {
        devPath: '/dev/sda',
        model: 'Archive HDD',
        serial: 'SERIAL-1',
        diskType: 'hdd',
        health: 'PASSED',
      },
      identity: { hostname: 'tower' },
      canonicalIdentity: { hostname: 'tower' },
      platformType: 'proxmox-pve',
    } as unknown as Resource;

    const warningData = extractPhysicalDiskPresentationData(warningDisk);
    const healthyData = extractPhysicalDiskPresentationData(healthyDisk);

    expect(warningData.model).toBe('Cache SSD');
    expect(warningData.riskReasons).toEqual(['Pending sectors detected.']);
    expect(matchesPhysicalDiskSearch(warningDisk, warningData, 'cache')).toBe(true);
    expect(matchesPhysicalDiskSearch(warningDisk, warningData, 'tank')).toBe(true);
    expect(matchesPhysicalDiskSearch(warningDisk, warningData, 'node:tower')).toBe(true);
    expect(matchesPhysicalDiskSearch(warningDisk, warningData, 'node:pve1')).toBe(false);
    expect(getPhysicalDiskSourceKey(warningDisk)).toBe('proxmox-pve');
    expect(getPhysicalDiskRoleFilterValue(warningData)).toBe('cache-pool');
    expect(normalizePhysicalDiskFacetFilter(' Cache Pool ')).toBe('cache-pool');
    expect(buildPhysicalDiskRoleFilterOptions([warningDisk, healthyDisk])).toContainEqual({
      value: 'cache-pool',
      label: 'Cache Pool',
    });
    expect(buildPhysicalDiskRoleFilterOptions([warningDisk, healthyDisk])[0]).toEqual({
      value: 'all',
      label: PHYSICAL_DISK_ALL_ROLES_FILTER_LABEL,
    });
    expect(buildPhysicalDiskGroupFilterOptions([warningDisk, healthyDisk])).toContainEqual({
      value: 'tank',
      label: 'tank',
    });
    expect(buildPhysicalDiskGroupFilterOptions([warningDisk, healthyDisk])[0]).toEqual({
      value: 'all',
      label: PHYSICAL_DISK_ALL_GROUPS_FILTER_LABEL,
    });
    expect(getPhysicalDiskNormalizedHealth(warningDisk, warningData)).toBe('warning');
    expect(
      comparePhysicalDiskPresentation(warningDisk, warningData, healthyDisk, healthyData),
    ).toBeLessThan(0);
    expect(buildPhysicalDiskPresentationDataMap([warningDisk, healthyDisk]).size).toBe(2);
    expect(
      filterAndSortPhysicalDisks([warningDisk, healthyDisk], {
        selectedNode: null,
        searchTerm: 'cache',
        sourceFilter: 'proxmox-pve',
        healthFilter: 'attention',
        roleFilter: 'cache-pool',
        groupFilter: 'tank',
        getDiskData: (disk) => (disk.id === warningDisk.id ? warningData : healthyData),
        matchesNode: () => true,
      }).map((disk) => disk.id),
    ).toEqual(['disk-warning']);
    expect(
      filterAndSortPhysicalDisks([warningDisk, healthyDisk], {
        selectedNode: null,
        searchTerm: '',
        sourceFilter: 'agent',
        healthFilter: 'all',
        getDiskData: (disk) => (disk.id === warningDisk.id ? warningData : healthyData),
        matchesNode: () => true,
      }).map((disk) => disk.id),
    ).toEqual([]);
  });
});
