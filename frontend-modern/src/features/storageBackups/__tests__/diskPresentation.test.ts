import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  buildPhysicalDiskPresentationDataMap,
  comparePhysicalDiskPresentation,
  extractPhysicalDiskPresentationData,
  filterAndSortPhysicalDisks,
  getPhysicalDiskExpandIconClass,
  PHYSICAL_DISK_EMPTY_CARD_CLASS,
  PHYSICAL_DISK_EXPAND_BUTTON_CLASS,
  PHYSICAL_DISK_HEADER_DISK_CLASS,
  PHYSICAL_DISK_HEADER_SOURCE_CLASS,
  PHYSICAL_DISK_NAME_TEXT_CLASS,
  PHYSICAL_DISK_SOURCE_BADGE_CLASS,
  PHYSICAL_DISK_TABLE_CLASS,
  getPhysicalDiskEmptyStatePresentation,
  getPhysicalDiskHealthStatus,
  getPhysicalDiskHealthSummary,
  getPhysicalDiskHostLabel,
  getPhysicalDiskParentLabel,
  getPhysicalDiskPlatformLabel,
  getPhysicalDiskRoleLabel,
  getPhysicalDiskSourceBadgePresentation,
  hasPhysicalDiskSmartWarning,
  matchesPhysicalDiskSearch,
} from '@/features/storageBackups/diskPresentation';

describe('diskPresentation', () => {
  it('returns critical presentation for failed disks', () => {
    expect(PHYSICAL_DISK_EMPTY_CARD_CLASS).toBe('text-center');
    expect(PHYSICAL_DISK_TABLE_CLASS).toBe('w-full text-xs');
    expect(PHYSICAL_DISK_EXPAND_BUTTON_CLASS).toContain('hover:bg-surface-hover');
    expect(PHYSICAL_DISK_HEADER_DISK_CLASS).toContain('uppercase');
    expect(PHYSICAL_DISK_HEADER_SOURCE_CLASS).toContain('w-[72px]');
    expect(PHYSICAL_DISK_NAME_TEXT_CLASS).toContain('font-semibold');
    expect(PHYSICAL_DISK_SOURCE_BADGE_CLASS).toContain('justify-center');

    expect(
      getPhysicalDiskHealthStatus({
        health: 'FAILED',
        wearout: 20,
        riskReasons: [],
        type: 'ssd',
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
        health: 'PASSED',
        wearout: 50,
        riskReasons: [],
        type: 'hdd',
        smartAttributes: { pendingSectors: 2 },
      }),
    ).toBe(true);
  });

  it('returns role, parent, and platform labels canonically', () => {
    expect(
      getPhysicalDiskRoleLabel({
        health: 'PASSED',
        wearout: 50,
        riskReasons: [],
        type: 'ssd',
        storageRole: 'cache_pool',
      }),
    ).toBe('Cache Pool');
    expect(
      getPhysicalDiskParentLabel({
        health: 'PASSED',
        wearout: 50,
        riskReasons: [],
        type: 'ssd',
        storageGroup: 'tank',
      }),
    ).toBe('tank');
    expect(getPhysicalDiskPlatformLabel({} as Resource, 'PVE')).toBe('PVE');
    expect(
      getPhysicalDiskSourceBadgePresentation({
        platformType: 'proxmox-pve',
      } as Resource),
    ).toEqual({
      label: 'PVE',
      className:
        'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-400 inline-flex min-w-[3.25rem] justify-center px-1.5 py-px text-[9px] font-medium',
    });
    expect(getPhysicalDiskExpandIconClass(true)).toContain('rotate-90');
    expect(getPhysicalDiskExpandIconClass(false)).not.toContain('rotate-90');
  });

  it('returns host, health summary, and empty-state presentation canonically', () => {
    expect(
      getPhysicalDiskHostLabel(
        {
          health: 'PASSED',
          wearout: 50,
          riskReasons: [],
          type: 'ssd',
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
      showRequirements: true,
      requirementsTitle: 'Physical disk monitoring requirements:',
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
    expect(comparePhysicalDiskPresentation(warningDisk, warningData, healthyDisk, healthyData)).toBeLessThan(0);
    expect(buildPhysicalDiskPresentationDataMap([warningDisk, healthyDisk]).size).toBe(2);
    expect(
      filterAndSortPhysicalDisks([warningDisk, healthyDisk], {
        selectedNode: null,
        searchTerm: 'cache',
        getDiskData: (disk) => (disk.id === warningDisk.id ? warningData : healthyData),
        matchesNode: () => true,
      }).map((disk) => disk.id),
    ).toEqual(['disk-warning']);
  });
});
