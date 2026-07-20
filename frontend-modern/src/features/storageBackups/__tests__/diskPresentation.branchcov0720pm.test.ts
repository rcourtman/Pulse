import { describe, expect, it } from 'vitest';
import type { PhysicalDiskFieldStatus, Resource } from '@/types/resource';
import {
  PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS,
  buildPhysicalDiskPresentationDataMap,
  buildPhysicalDiskGroupFilterOptions,
  buildPhysicalDiskRoleFilterOptions,
  comparePhysicalDiskPresentation,
  extractPhysicalDiskPresentationData,
  filterAndSortPhysicalDisks,
  getPhysicalDiskCollectionMessages,
  getPhysicalDiskEmptyStatePresentation,
  getPhysicalDiskFieldStatusMessage,
  getPhysicalDiskHealthStatus,
  getPhysicalDiskHealthSummary,
  getPhysicalDiskHostLabel,
  getPhysicalDiskLifeTextClass,
  getPhysicalDiskNormalizedHealth,
  getPhysicalDiskPlatformLabel,
  getPhysicalDiskRoleLabel,
  getPhysicalDiskSourceBadgePresentation,
  hasPhysicalDiskSmartWarning,
  hasUnraidPhysicalDiskFaultSignal,
  isUnraidPhysicalDisk,
  matchesPhysicalDiskHealthFilter,
  matchesPhysicalDiskSearch,
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

describe('diskPresentation.branchcov0720pm', () => {
  describe('getPhysicalDiskHealthFilterEmptyTitle switch arms', () => {
    it('returns the matching empty-state title for the healthy filter', () => {
      expect(
        getPhysicalDiskEmptyStatePresentation({
          selectedNodeName: null,
          searchTerm: '',
          diskCount: 2,
          hasPVENodes: true,
          healthFilter: 'healthy',
        }).title,
      ).toBe('No healthy disks found');
    });

    it('returns the matching empty-state title for the warning filter', () => {
      expect(
        getPhysicalDiskEmptyStatePresentation({
          selectedNodeName: null,
          searchTerm: '',
          diskCount: 2,
          hasPVENodes: true,
          healthFilter: 'warning',
        }).title,
      ).toBe('No warning disks found');
    });

    it('returns the matching empty-state title for the critical filter', () => {
      expect(
        getPhysicalDiskEmptyStatePresentation({
          selectedNodeName: null,
          searchTerm: '',
          diskCount: 2,
          hasPVENodes: true,
          healthFilter: 'critical',
        }).title,
      ).toBe('No critical disks found');
    });

    it('returns the matching empty-state title for the offline filter', () => {
      expect(
        getPhysicalDiskEmptyStatePresentation({
          selectedNodeName: null,
          searchTerm: '',
          diskCount: 2,
          hasPVENodes: true,
          healthFilter: 'offline',
        }).title,
      ).toBe('No offline disks found');
    });

    it('returns the matching empty-state title for the unknown filter', () => {
      expect(
        getPhysicalDiskEmptyStatePresentation({
          selectedNodeName: null,
          searchTerm: '',
          diskCount: 2,
          hasPVENodes: true,
          healthFilter: 'unknown',
        }).title,
      ).toBe('No disks with unknown health');
    });

    it('falls back to the default-empty title when healthFilter is "all" but disks are present', () => {
      expect(
        getPhysicalDiskEmptyStatePresentation({
          selectedNodeName: null,
          searchTerm: 'zzz',
          diskCount: 2,
          hasPVENodes: true,
          healthFilter: 'all',
        }).title,
      ).toBe('No disks match these filters');
    });
  });

  describe('getPhysicalDiskEmptyStatePresentation ternary arms', () => {
    it('returns null nodeMessage and searchMessage when nothing is selected', () => {
      const presentation = getPhysicalDiskEmptyStatePresentation({
        selectedNodeName: null,
        searchTerm: '',
        diskCount: 0,
        hasPVENodes: false,
      });
      expect(presentation.nodeMessage).toBeNull();
      expect(presentation.searchMessage).toBeNull();
      expect(presentation.showRequirements).toBe(false);
    });

    it('returns a searchMessage when a searchTerm is supplied', () => {
      const presentation = getPhysicalDiskEmptyStatePresentation({
        selectedNodeName: null,
        searchTerm: 'wd20',
        diskCount: 0,
        hasPVENodes: false,
      });
      expect(presentation.searchMessage).toBe('matching "wd20"');
    });
  });

  describe('getPhysicalDiskFieldStatusMessage default arm', () => {
    it('returns empty string for an unrecognized state via the default switch arm', () => {
      const malformed = {
        state: 'totally-bogus',
        source: 'test',
      } as unknown as PhysicalDiskFieldStatus;
      expect(getPhysicalDiskFieldStatusMessage('Field', malformed)).toBe('');
    });

    it('returns empty string when status is null or available', () => {
      expect(getPhysicalDiskFieldStatusMessage('Field', null)).toBe('');
      expect(
        getPhysicalDiskFieldStatusMessage('Field', { state: 'available', source: 'test' }),
      ).toBe('');
    });
  });

  describe('getPhysicalDiskCollectionMessages empty paths', () => {
    it('returns an empty array when the disk has no collection evidence', () => {
      expect(getPhysicalDiskCollectionMessages(makeDiskData())).toEqual([]);
    });
  });

  describe('getPhysicalDiskPlatformLabel fallback', () => {
    it('returns "Unknown" when no fallback label is supplied', () => {
      expect(getPhysicalDiskPlatformLabel({} as Resource, '')).toBe('Unknown');
    });
  });

  describe('getPhysicalDiskSourceBadgePresentation fallback arms', () => {
    it('falls back to the platform-derived label and base tone when the presentation is missing', () => {
      const badge = getPhysicalDiskSourceBadgePresentation({
        platformType: '',
      } as unknown as Resource);
      expect(badge.label).toBe('Unknown');
      expect(badge.className).toContain('text-base-content');
    });
  });

  describe('getPhysicalDiskHostLabel fallback arms', () => {
    it('falls back to resource.parentName when disk.node is empty', () => {
      expect(
        getPhysicalDiskHostLabel(makeDiskData(), { parentName: 'pve-node-1' } as Resource),
      ).toBe('pve-node-1');
    });

    it('returns an empty string when neither node nor parentName is set', () => {
      expect(getPhysicalDiskHostLabel(makeDiskData(), {} as Resource)).toBe('');
    });
  });

  describe('extractPhysicalDiskPresentationData fallback arms', () => {
    it('reads physicalDisk from platformData when the top-level field is absent', () => {
      const resource = {
        name: 'fallback-disk',
        platformData: { physicalDisk: { devPath: '/dev/sdz', health: 'PASSED', wearout: 88 } },
      } as unknown as Resource;
      const data = extractPhysicalDiskPresentationData(resource);
      expect(data.devPath).toBe('/dev/sdz');
      expect(data.health).toBe('PASSED');
      expect(data.wearout).toBe(88);
    });

    it('returns defaults when neither physicalDisk nor platformData.physicalDisk is present', () => {
      const data = extractPhysicalDiskPresentationData({ name: 'empty' } as unknown as Resource);
      expect(data.devPath).toBe('');
      expect(data.model).toBe('empty');
      expect(data.health).toBe('UNKNOWN');
      expect(data.wearout).toBe(-1);
      expect(data.temperature).toBe(0);
      expect(data.rpm).toBe(0);
      expect(data.riskLevel).toBeUndefined();
      expect(data.smartAttributes).toBeUndefined();
    });

    it('coerces zero values through the ?? and || branches', () => {
      const data = extractPhysicalDiskPresentationData({
        name: 'zeroed',
        physicalDisk: {
          devPath: '',
          model: '',
          serial: '',
          wwn: '',
          diskType: '',
          sizeBytes: 0,
          health: '',
          wearout: 0,
          temperature: 0,
          rpm: 0,
          used: '',
        },
      } as unknown as Resource);
      expect(data.model).toBe('zeroed');
      expect(data.health).toBe('UNKNOWN');
      expect(data.size).toBe(0);
    });

    it('returns an empty model when neither pd.model nor resource.name is set', () => {
      const data = extractPhysicalDiskPresentationData({
        name: '',
        physicalDisk: { devPath: '/dev/sda' },
      } as unknown as Resource);
      expect(data.model).toBe('');
    });

    it('reads sources from platformData.sourceStatus object keys', () => {
      const resource = {
        name: 'src',
        platformData: { sourceStatus: { proxmox: { ok: true }, agent: { ok: false } } },
      } as unknown as Resource;
      expect(getPhysicalDiskNormalizedHealth(resource, makeDiskData({ health: 'UNKNOWN' }))).toBe(
        'unknown',
      );
    });

    it('treats a non-object sourceStatus as no extra sources', () => {
      const resource = {
        name: 'srcstr',
        platformData: { sourceStatus: 'not-an-object' as unknown as object },
      } as unknown as Resource;
      expect(getPhysicalDiskNormalizedHealth(resource, makeDiskData({ health: 'UNKNOWN' }))).toBe(
        'unknown',
      );
    });
  });

  describe('Unraid detection branch arms', () => {
    it('detects Unraid disks via storageState + storageRole without the unraid-array group', () => {
      const disk = makeDiskData({
        storageRole: 'parity',
        storageState: 'online',
        storageGroup: 'other-group',
      });
      expect(isUnraidPhysicalDisk(disk)).toBe(true);
    });

    it('rejects disks whose role is not a known Unraid role even with a storage state', () => {
      const disk = makeDiskData({
        storageRole: 'spare',
        storageState: 'online',
        storageGroup: 'other-group',
      });
      expect(isUnraidPhysicalDisk(disk)).toBe(false);
    });

    it('rejects disks with no storageState and a non-unraid group', () => {
      const disk = makeDiskData({
        storageRole: 'data',
        storageState: '',
        storageGroup: 'other-group',
      });
      expect(isUnraidPhysicalDisk(disk)).toBe(false);
    });

    it('returns false from the fault-signal helper for non-Unraid disks', () => {
      expect(
        hasUnraidPhysicalDiskFaultSignal(makeDiskData({ health: 'FAILED', storageRole: 'spare' })),
      ).toBe(false);
    });

    it('flags Unraid disks via bad health states even without errorCount', () => {
      const disk = makeDiskData({
        health: 'FAULTED',
        storageRole: 'data',
        storageGroup: 'unraid-array',
        storageState: 'disabled',
      });
      expect(hasUnraidPhysicalDiskFaultSignal(disk)).toBe(true);
    });
  });

  describe('hasPhysicalDiskSmartWarning attribute arms', () => {
    it('detects warnings from reallocated sectors', () => {
      expect(
        hasPhysicalDiskSmartWarning({
          ...makeDiskData(),
          smartAttributes: { reallocatedSectors: 1 },
        }),
      ).toBe(true);
    });

    it('detects warnings from media errors', () => {
      expect(
        hasPhysicalDiskSmartWarning({
          ...makeDiskData(),
          smartAttributes: { mediaErrors: 4 },
        }),
      ).toBe(true);
    });

    it('returns false when attrs are present but all counters are zero', () => {
      expect(
        hasPhysicalDiskSmartWarning({
          ...makeDiskData(),
          smartAttributes: { reallocatedSectors: 0, pendingSectors: 0, mediaErrors: 0 },
        }),
      ).toBe(false);
    });
  });

  describe('getPhysicalDiskHealthStatus remaining arms', () => {
    it('returns Healthy for PASSED health on a non-Unraid disk', () => {
      const disk = makeDiskData({ health: 'PASSED', type: 'ssd', wearout: 90 });
      expect(getPhysicalDiskHealthStatus(disk)).toMatchObject({
        label: 'Healthy',
        summary: 'No active disk-health issues.',
      });
    });

    it('returns Healthy for GOOD health', () => {
      const disk = makeDiskData({ health: 'GOOD', type: 'hdd', wearout: 90 });
      expect(getPhysicalDiskHealthStatus(disk).label).toBe('Healthy');
    });

    it('returns Unknown for an Unraid disk that is not in the online state', () => {
      const disk = makeDiskData({
        health: 'UNKNOWN',
        storageRole: 'data',
        storageGroup: 'unraid-array',
        storageState: 'standby',
      });
      expect(getPhysicalDiskHealthStatus(disk).label).toBe('Unknown');
    });

    it('uses "SMART counters indicate elevated risk." when low life is not the cause', () => {
      const disk = makeDiskData({
        health: 'PASSED',
        type: 'hdd',
        wearout: 60,
        smartAttributes: { pendingSectors: 5 },
      });
      expect(getPhysicalDiskHealthStatus(disk).summary).toBe(
        'SMART counters indicate elevated risk.',
      );
    });

    it('uses "SSD life is running low." when wearout is below 10', () => {
      const disk = makeDiskData({
        health: 'PASSED',
        type: 'ssd',
        wearout: 5,
      });
      expect(getPhysicalDiskHealthStatus(disk).summary).toBe('SSD life is running low.');
    });
  });

  describe('getPhysicalDiskNormalizedHealth coverage', () => {
    it('classifies an offline resource as offline', () => {
      const disk = makeDiskData({ health: 'PASSED', type: 'ssd', wearout: 90 });
      const resource = { status: 'offline' } as Resource;
      expect(getPhysicalDiskNormalizedHealth(resource, disk)).toBe('offline');
    });

    it('classifies a failed disk as critical', () => {
      const disk = makeDiskData({ health: 'FAILED', riskLevel: 'critical' });
      expect(getPhysicalDiskNormalizedHealth({} as Resource, disk)).toBe('critical');
    });

    it('classifies a SMART-warning disk as warning', () => {
      const disk = makeDiskData({
        health: 'PASSED',
        type: 'hdd',
        wearout: 60,
        smartAttributes: { pendingSectors: 2 },
      });
      expect(getPhysicalDiskNormalizedHealth({} as Resource, disk)).toBe('warning');
    });

    it('classifies a healthy disk as healthy', () => {
      const disk = makeDiskData({ health: 'PASSED', type: 'ssd', wearout: 90 });
      expect(getPhysicalDiskNormalizedHealth({} as Resource, disk)).toBe('healthy');
    });

    it('classifies an UNKNOWN-health non-Unraid disk as unknown', () => {
      const disk = makeDiskData({ health: 'UNKNOWN' });
      expect(getPhysicalDiskNormalizedHealth({} as Resource, disk)).toBe('unknown');
    });

    it('treats an empty/undefined health string as Unknown label', () => {
      const empty = makeDiskData({ health: '' as unknown as undefined });
      expect(getPhysicalDiskHealthStatus(empty).label).toBe('Unknown');
      const undefinedHealth = makeDiskData({
        health: undefined as unknown as string,
      });
      expect(getPhysicalDiskHealthStatus(undefinedHealth).label).toBe('Unknown');
    });
  });

  describe('matchesPhysicalDiskHealthFilter', () => {
    it('returns true for the all filter regardless of health', () => {
      expect(matchesPhysicalDiskHealthFilter('unknown', 'all')).toBe(true);
    });

    it('returns true for the attention filter against offline', () => {
      expect(matchesPhysicalDiskHealthFilter('offline', 'attention')).toBe(true);
    });

    it('returns false for the attention filter against healthy', () => {
      expect(matchesPhysicalDiskHealthFilter('healthy', 'attention')).toBe(false);
    });

    it('returns true for an exact match against the warning filter', () => {
      expect(matchesPhysicalDiskHealthFilter('warning', 'warning')).toBe(true);
    });

    it('returns false for a non-match against the critical filter', () => {
      expect(matchesPhysicalDiskHealthFilter('warning', 'critical')).toBe(false);
    });
  });

  describe('getPhysicalDiskHealthSummary empty arm', () => {
    it('returns an empty string when the summary is whitespace', () => {
      expect(
        getPhysicalDiskHealthSummary({
          label: 'Healthy',
          summary: '   ',
          tone: 'text-base-content',
        }),
      ).toBe('');
    });
  });

  describe('getPhysicalDiskRoleLabel ssd/hdd arms', () => {
    it('returns SSD for type ssd', () => {
      expect(getPhysicalDiskRoleLabel(makeDiskData({ type: 'ssd' }))).toBe('SSD');
    });

    it('returns HDD for type hdd', () => {
      expect(getPhysicalDiskRoleLabel(makeDiskData({ type: 'hdd' }))).toBe('HDD');
    });

    it('returns a Titleized disk label for other types', () => {
      expect(getPhysicalDiskRoleLabel(makeDiskData({ type: 'usb-c' }))).toBe('Usb C disk');
    });

    it('returns empty string when no role or type is set', () => {
      expect(getPhysicalDiskRoleLabel(makeDiskData())).toBe('');
    });
  });

  describe('getPhysicalDiskLifeTextClass muted placeholder arm', () => {
    it('returns the muted placeholder class when wearout is 0', () => {
      expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 0 }))).toBe(
        PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS,
      );
    });

    it('returns the muted placeholder class when wearout is not a number', () => {
      expect(
        getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 'nan' as unknown as number })),
      ).toBe(PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS);
    });
  });

  describe('matchesPhysicalDiskSearch empty freeTerms arm', () => {
    it('returns true when only node terms match and there are no free terms', () => {
      const disk = makeDiskData({ node: 'tower' });
      const resource = { parentName: 'tower' } as Resource;
      expect(matchesPhysicalDiskSearch(resource, disk, 'node:tower')).toBe(true);
    });

    it('returns true when the search term is empty', () => {
      const disk = makeDiskData({ node: 'tower' });
      const resource = { parentName: 'tower' } as Resource;
      expect(matchesPhysicalDiskSearch(resource, disk, '')).toBe(true);
    });
  });

  describe('comparePhysicalDiskPresentation tie-breaker arms', () => {
    it('falls back to node localeCompare when priorities match', () => {
      const aDisk = makeDiskData({ node: 'alpha', devPath: '/dev/sda' });
      const bDisk = makeDiskData({ node: 'beta', devPath: '/dev/sda' });
      expect(
        comparePhysicalDiskPresentation({} as Resource, aDisk, {} as Resource, bDisk),
      ).toBeLessThan(0);
    });

    it('falls back to devPath localeCompare when priority and node match', () => {
      const aDisk = makeDiskData({ node: 'tower', devPath: '/dev/sda' });
      const bDisk = makeDiskData({ node: 'tower', devPath: '/dev/sdb' });
      expect(
        comparePhysicalDiskPresentation({} as Resource, aDisk, {} as Resource, bDisk),
      ).toBeLessThan(0);
    });

    it('falls back to resource.name when devPath is empty', () => {
      const aDisk = makeDiskData({ node: 'tower', devPath: '' });
      const bDisk = makeDiskData({ node: 'tower', devPath: '' });
      expect(
        comparePhysicalDiskPresentation(
          { name: 'aaa' } as Resource,
          aDisk,
          { name: 'bbb' } as Resource,
          bDisk,
        ),
      ).toBeLessThan(0);
    });
  });

  describe('buildPhysicalDiskPresentationDataMap empty input arm', () => {
    it('returns an empty Map when disks is null', () => {
      expect(buildPhysicalDiskPresentationDataMap(null as unknown as Resource[]).size).toBe(0);
    });
  });

  describe('filterAndSortPhysicalDisks null/selectedNode arms', () => {
    it('returns an empty array when disks is null', () => {
      expect(
        filterAndSortPhysicalDisks(null as unknown as Resource[], {
          selectedNode: null,
          searchTerm: '',
          getDiskData: () => makeDiskData(),
          matchesNode: () => true,
        }),
      ).toEqual([]);
    });

    it('filters by selected node via the provided matchesNode callback', () => {
      const inNode = {
        id: 'a',
        name: 'a',
        physicalDisk: { devPath: '/dev/sda', health: 'PASSED' },
      } as unknown as Resource;
      const outNode = {
        id: 'b',
        name: 'b',
        physicalDisk: { devPath: '/dev/sdb', health: 'PASSED' },
      } as unknown as Resource;
      const result = filterAndSortPhysicalDisks([inNode, outNode], {
        selectedNode: { id: 'node-1', name: 'tower' } as Resource,
        searchTerm: '',
        getDiskData: (disk) => extractPhysicalDiskPresentationData(disk),
        matchesNode: (disk, node) => disk.id === 'a' && node.id === 'node-1',
      });
      expect(result.map((d) => d.id)).toEqual(['a']);
    });

    it('exposes the proxmox instance from the selected node to the matchesNode callback', () => {
      const inNode = {
        id: 'a',
        name: 'a',
        physicalDisk: { devPath: '/dev/sda', health: 'PASSED' },
      } as unknown as Resource;
      const result = filterAndSortPhysicalDisks([inNode], {
        selectedNode: {
          id: 'node-1',
          name: 'tower',
          platformData: { proxmox: { instance: 'pve1' } },
        } as unknown as Resource,
        searchTerm: '',
        getDiskData: (disk) => extractPhysicalDiskPresentationData(disk),
        matchesNode: (_disk, node) => node.instance === 'pve1',
      });
      expect(result.map((d) => d.id)).toEqual(['a']);
    });
  });

  describe('buildPhysicalDisk*FilterOptions null input arm', () => {
    it('returns only the "all" option when disks is null', () => {
      expect(buildPhysicalDiskRoleFilterOptions(null as unknown as Resource[])).toHaveLength(1);
      expect(buildPhysicalDiskGroupFilterOptions(null as unknown as Resource[])).toHaveLength(1);
    });
  });

  describe('filter state "all" arms', () => {
    it('skips source/health/role/group checks when filters are at their defaults', () => {
      const resource = {
        id: 'd1',
        name: 'd1',
        physicalDisk: { devPath: '/dev/sda', health: 'PASSED', diskType: 'ssd' },
      } as unknown as Resource;
      const resource2 = {
        id: 'd2',
        name: 'd2',
        physicalDisk: { devPath: '/dev/sdb', health: 'PASSED', diskType: 'hdd' },
      } as unknown as Resource;
      const result = filterAndSortPhysicalDisks([resource, resource2], {
        selectedNode: null,
        sourceFilter: 'all',
        healthFilter: 'all',
        roleFilter: 'all',
        groupFilter: 'all',
        searchTerm: '',
        getDiskData: (d) => extractPhysicalDiskPresentationData(d),
        matchesNode: () => true,
      });
      expect(result.map((d) => d.id)).toEqual(['d1', 'd2']);
    });

    it('rejects disks whose computed role filter value differs from the selected role', () => {
      const resource = {
        id: 'd1',
        name: 'd1',
        physicalDisk: { devPath: '/dev/sda', health: 'PASSED', diskType: 'ssd' },
      } as unknown as Resource;
      const result = filterAndSortPhysicalDisks([resource], {
        selectedNode: null,
        roleFilter: 'parity',
        searchTerm: '',
        getDiskData: (d) => extractPhysicalDiskPresentationData(d),
        matchesNode: () => true,
      });
      expect(result).toEqual([]);
    });

    it('rejects disks whose computed group filter value differs from the selected group', () => {
      const resource = {
        id: 'd1',
        name: 'd1',
        physicalDisk: {
          devPath: '/dev/sda',
          health: 'PASSED',
          diskType: 'ssd',
          storageGroup: 'tank',
        },
      } as unknown as Resource;
      const result = filterAndSortPhysicalDisks([resource], {
        selectedNode: null,
        groupFilter: 'other-pool',
        searchTerm: '',
        getDiskData: (d) => extractPhysicalDiskPresentationData(d),
        matchesNode: () => true,
      });
      expect(result).toEqual([]);
    });
  });
});
