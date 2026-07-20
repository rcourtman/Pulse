import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  buildPhysicalDiskGroupFilterOptions,
  buildPhysicalDiskPresentationDataMap,
  buildPhysicalDiskRoleFilterOptions,
  extractPhysicalDiskPresentationData,
  filterAndSortPhysicalDisks,
  getPhysicalDiskHealthSummary,
  getPhysicalDiskLifeTextClass,
  getPhysicalDiskPlatformLabel,
  getPhysicalDiskRoleLabel,
  matchesPhysicalDiskHealthFilter,
  PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS,
  type DiskHealthStatusPresentation,
  type PhysicalDiskPresentationData,
} from '@/features/storageBackups/diskPresentation';

// Shared factory mirroring the convention in diskPresentation.test.ts and
// diskPresentation.branchcov.test.ts so optional fields default safely.
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

// Helper to build a Resource carrying a physicalDisk payload, cast to satisfy
// the strict Resource type without depending on its full surface.
const diskResource = (id: string, physicalDisk: Record<string, unknown>): Resource =>
  ({
    id,
    name: id,
    platformType: 'agent',
    physicalDisk,
  }) as unknown as Resource;

// ---------------------------------------------------------------------------
// titleize  (module-private — exercised via getPhysicalDiskRoleLabel)
// ---------------------------------------------------------------------------

describe('titleize via getPhysicalDiskRoleLabel (branchcov2)', () => {
  it('title-cases a single plain word with no separators', () => {
    expect(getPhysicalDiskRoleLabel(makeDiskData({ storageRole: 'parity' }))).toBe('Parity');
  });

  it('preserves the remainder of the word verbatim (rest is NOT lowercased)', () => {
    // 'DATA' -> charAt(0).toUpperCase() + slice(1) -> 'D' + 'ATA' -> 'DATA'
    expect(getPhysicalDiskRoleLabel(makeDiskData({ storageRole: 'DATA' }))).toBe('DATA');
  });

  it('does not split camelCase and keeps inner capitalization verbatim', () => {
    expect(getPhysicalDiskRoleLabel(makeDiskData({ storageRole: 'cachePool' }))).toBe('CachePool');
  });

  it('strips leading/trailing separators via filter(Boolean)', () => {
    expect(getPhysicalDiskRoleLabel(makeDiskData({ storageRole: '-parity-' }))).toBe('Parity');
  });
});

// ---------------------------------------------------------------------------
// getPhysicalDiskPlatformLabel
// ---------------------------------------------------------------------------

describe('getPhysicalDiskPlatformLabel (branchcov2)', () => {
  it('returns the fallback label when it is a non-empty string', () => {
    expect(getPhysicalDiskPlatformLabel({} as Resource, 'TrueNAS')).toBe('TrueNAS');
  });

  it('returns "Unknown" when the fallback label is empty', () => {
    expect(getPhysicalDiskPlatformLabel({} as Resource, '')).toBe('Unknown');
  });
});

// ---------------------------------------------------------------------------
// extractPhysicalDiskPresentationData
// ---------------------------------------------------------------------------

describe('extractPhysicalDiskPresentationData (branchcov2)', () => {
  it('preserves an explicit wearout of 0 via nullish coalescing (not -1)', () => {
    const data = extractPhysicalDiskPresentationData(
      diskResource('d', { wearout: 0, temperature: 0 }),
    );
    // `pd.wearout ?? -1` keeps 0; an `||` would have collapsed it to -1.
    expect(data.wearout).toBe(0);
    expect(data.temperature).toBe(0);
  });

  it('falls back model to resource.name when physicalDisk is present but model is absent', () => {
    const resource = {
      id: 'd',
      name: 'named-disk',
      platformType: 'agent',
      physicalDisk: { health: 'PASSED' },
    } as unknown as Resource;
    expect(extractPhysicalDiskPresentationData(resource).model).toBe('named-disk');
  });

  it('returns an empty model when both pd.model and resource.name are absent', () => {
    const resource = {
      id: 'd',
      platformType: 'agent',
      physicalDisk: { health: 'PASSED' },
    } as unknown as Resource;
    expect(extractPhysicalDiskPresentationData(resource).model).toBe('');
  });

  it('maps an empty smart object to a smartAttributes shell with all-undefined fields', () => {
    const data = extractPhysicalDiskPresentationData(diskResource('d', { smart: {} }));
    expect(data.smartAttributes).toStrictEqual({
      powerOnHours: undefined,
      powerCycles: undefined,
      reallocatedSectors: undefined,
      pendingSectors: undefined,
      offlineUncorrectable: undefined,
      udmaCrcErrors: undefined,
      percentageUsed: undefined,
      availableSpare: undefined,
      mediaErrors: undefined,
      unsafeShutdowns: undefined,
    });
  });

  it('reads diskType into the type field and falls back sizeBytes/used to defaults when absent', () => {
    const data = extractPhysicalDiskPresentationData(
      diskResource('d', { diskType: 'nvme', health: 'GOOD' }),
    );
    expect(data.type).toBe('nvme');
    expect(data.size).toBe(0);
    expect(data.used).toBe('');
    expect(data.health).toBe('GOOD');
  });
});

// ---------------------------------------------------------------------------
// buildPhysicalDiskPresentationDataMap
// ---------------------------------------------------------------------------

describe('buildPhysicalDiskPresentationDataMap (branchcov2)', () => {
  it('returns an empty Map for a null disks argument (defensive `disks || []` arm)', () => {
    expect(buildPhysicalDiskPresentationDataMap(null as unknown as Resource[]).size).toBe(0);
  });

  it('returns an empty Map for an undefined disks argument', () => {
    expect(buildPhysicalDiskPresentationDataMap(undefined as unknown as Resource[]).size).toBe(0);
  });

  it('keys entries by resource.id and extracts presentation data', () => {
    const a = diskResource('a', { model: 'Drive A', health: 'PASSED' });
    const b = diskResource('b', { model: 'Drive B', health: 'FAILED' });
    const map = buildPhysicalDiskPresentationDataMap([a, b]);
    expect(map.size).toBe(2);
    expect(map.get('a')?.model).toBe('Drive A');
    expect(map.get('a')?.health).toBe('PASSED');
    expect(map.get('b')?.health).toBe('FAILED');
  });

  it('overwrites the entry when two resources share an id (last wins)', () => {
    const first = diskResource('dup', { model: 'First' });
    const second = diskResource('dup', { model: 'Second' });
    const map = buildPhysicalDiskPresentationDataMap([first, second]);
    expect(map.size).toBe(1);
    expect(map.get('dup')?.model).toBe('Second');
  });
});

// ---------------------------------------------------------------------------
// filterAndSortPhysicalDisks
// ---------------------------------------------------------------------------

describe('filterAndSortPhysicalDisks (branchcov2)', () => {
  it('returns an empty array for a null disks argument', () => {
    expect(
      filterAndSortPhysicalDisks(null as unknown as Resource[], {
        selectedNode: null,
        searchTerm: '',
        getDiskData: (d) => extractPhysicalDiskPresentationData(d),
        matchesNode: () => true,
      }),
    ).toEqual([]);
  });

  it('filters by selectedNode via matchesNode, forwarding id/name/instance (incl. proxmox.instance)', () => {
    const diskA = {
      id: 'a',
      name: 'a',
      parentName: 'n1',
      platformType: 'agent',
      physicalDisk: { devPath: '/dev/sda', health: 'PASSED' },
    } as unknown as Resource;
    const diskB = {
      id: 'b',
      name: 'b',
      parentName: 'n2',
      platformType: 'agent',
      physicalDisk: { devPath: '/dev/sdb', health: 'PASSED' },
    } as unknown as Resource;
    const selectedNode = {
      id: 'node-1',
      name: 'n1',
      platformData: { proxmox: { instance: 'pve1' } },
    } as unknown as Resource;

    const captured: { id: string; name: string; instance?: string }[] = [];
    const result = filterAndSortPhysicalDisks([diskA, diskB], {
      selectedNode,
      searchTerm: '',
      getDiskData: (d) => extractPhysicalDiskPresentationData(d),
      matchesNode: (disk, node) => {
        captured.push({ id: node.id, name: node.name, instance: node.instance });
        return disk.parentName === node.name;
      },
    });

    // Both calls forward the derived node identity (instance read via the
    // `(platformData as any)?.proxmox?.instance` optional chain).
    expect(captured).toEqual([
      { id: 'node-1', name: 'n1', instance: 'pve1' },
      { id: 'node-1', name: 'n1', instance: 'pve1' },
    ]);
    expect(result.map((d) => d.id)).toEqual(['a']);
  });

  it('sorts by priority (critical risk before healthy) when no node is selected', () => {
    const healthy = diskResource('healthy', { devPath: '/dev/sda', health: 'PASSED' });
    const critical = diskResource('critical', {
      devPath: '/dev/sdb',
      health: 'PASSED',
      risk: { level: 'critical' },
    });
    const result = filterAndSortPhysicalDisks([healthy, critical], {
      selectedNode: null,
      searchTerm: '',
      getDiskData: (d) => extractPhysicalDiskPresentationData(d),
      matchesNode: () => true,
    });
    expect(result.map((d) => d.id)).toEqual(['critical', 'healthy']);
  });
});

// ---------------------------------------------------------------------------
// matchesPhysicalDiskHealthFilter
// ---------------------------------------------------------------------------

describe('matchesPhysicalDiskHealthFilter (branchcov2)', () => {
  it('returns true for filter "all" regardless of health', () => {
    expect(matchesPhysicalDiskHealthFilter('healthy', 'all')).toBe(true);
    expect(matchesPhysicalDiskHealthFilter('critical', 'all')).toBe(true);
  });

  it.each([['warning'], ['critical'], ['offline']] as const)(
    'returns true for "attention" when health is %s',
    (health) => {
      expect(matchesPhysicalDiskHealthFilter(health, 'attention')).toBe(true);
    },
  );

  it.each(['healthy', 'unknown'])('returns false for "attention" when health is %s', (health) => {
    expect(matchesPhysicalDiskHealthFilter(health as 'healthy', 'attention')).toBe(false);
  });

  it('returns true on an exact health/filter match', () => {
    expect(matchesPhysicalDiskHealthFilter('warning', 'warning')).toBe(true);
    expect(matchesPhysicalDiskHealthFilter('critical', 'critical')).toBe(true);
  });

  it('returns false on an exact health/filter mismatch', () => {
    expect(matchesPhysicalDiskHealthFilter('healthy', 'warning')).toBe(false);
    expect(matchesPhysicalDiskHealthFilter('offline', 'critical')).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// getPhysicalDiskHealthSummary
// ---------------------------------------------------------------------------

describe('getPhysicalDiskHealthSummary (branchcov2)', () => {
  const statusWith = (summary?: string): DiskHealthStatusPresentation =>
    ({ label: 'X', summary, tone: 't' }) as unknown as DiskHealthStatusPresentation;

  it('returns empty string when summary is undefined (optional-chain arm)', () => {
    expect(getPhysicalDiskHealthSummary(statusWith(undefined))).toBe('');
  });

  it('returns empty string when summary is whitespace-only', () => {
    expect(getPhysicalDiskHealthSummary(statusWith('   '))).toBe('');
  });

  it('returns empty string when summary equals the canonical "no issues" message', () => {
    expect(getPhysicalDiskHealthSummary(statusWith('No active disk-health issues.'))).toBe('');
  });

  it('returns the trimmed summary for a real, non-canonical message', () => {
    expect(getPhysicalDiskHealthSummary(statusWith('  Pending sectors detected.  '))).toBe(
      'Pending sectors detected.',
    );
  });
});

// ---------------------------------------------------------------------------
// getPhysicalDiskLifeTextClass
// ---------------------------------------------------------------------------

describe('getPhysicalDiskLifeTextClass (branchcov2)', () => {
  it('returns the muted placeholder class for the unreported boundary wearout -1', () => {
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: -1 }))).toBe(
      PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS,
    );
  });

  it('returns the muted placeholder class for wearout 0 (<= 0 branch)', () => {
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 0 }))).toBe(
      PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS,
    );
  });

  it('returns the muted placeholder class when wearout is not a number', () => {
    expect(
      getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 'nope' as unknown as number })),
    ).toBe(PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS);
  });

  it('returns the red class for wearout just below 20', () => {
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 19 }))).toBe(
      'text-red-600 dark:text-red-400',
    );
  });

  it('returns the amber class at the wearout=20 boundary (< 20 is false, < 50 true)', () => {
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 20 }))).toBe(
      'text-amber-600 dark:text-amber-400',
    );
  });

  it('returns the amber class for wearout just below 50', () => {
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 49 }))).toBe(
      'text-amber-600 dark:text-amber-400',
    );
  });

  it('returns the green class at the wearout=50 boundary (< 50 is false)', () => {
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 50 }))).toBe(
      'text-green-600 dark:text-green-400',
    );
  });
});

// ---------------------------------------------------------------------------
// buildPhysicalDiskFacetOptions  (module-private — exercised via
// buildPhysicalDiskRoleFilterOptions / buildPhysicalDiskGroupFilterOptions)
// ---------------------------------------------------------------------------

describe('buildPhysicalDiskFacetOptions via role/group filter option builders (branchcov2)', () => {
  it('returns only the "all" option for a null disks argument', () => {
    const options = buildPhysicalDiskRoleFilterOptions(null as unknown as Resource[]);
    expect(options).toEqual([{ value: 'all', label: 'All roles' }]);
  });

  it('deduplicates, skips empty labels, falls back to type, and sorts by label', () => {
    const cacheA = diskResource('a', { storageRole: 'cache' });
    const cacheB = diskResource('b', { storageRole: 'cache' }); // duplicate of 'cache'
    const parity = diskResource('c', { storageRole: 'parity' });
    const ssd = diskResource('d', { diskType: 'ssd' }); // role via type fallback -> 'SSD'
    const empty = diskResource('e', {}); // no role, no type -> '' skipped

    const options = buildPhysicalDiskRoleFilterOptions([cacheA, cacheB, parity, ssd, empty]);
    expect(options).toEqual([
      { value: 'all', label: 'All roles' },
      { value: 'cache', label: 'Cache' },
      { value: 'parity', label: 'Parity' },
      { value: 'ssd', label: 'SSD' },
    ]);
  });

  it('exercises the group-facet path: storageGroup label is deduped and sorted', () => {
    const a = diskResource('a', { storageGroup: 'tank' });
    const b = diskResource('b', { storageGroup: 'tank' }); // duplicate
    const c = diskResource('c', { storageGroup: 'mirror' });
    const d = diskResource('d', {}); // no group, no used -> '' skipped

    const options = buildPhysicalDiskGroupFilterOptions([a, b, c, d]);
    expect(options).toEqual([
      { value: 'all', label: 'All groups' },
      { value: 'mirror', label: 'mirror' },
      { value: 'tank', label: 'tank' },
    ]);
  });

  it('uses the "used" fallback label for group facet when storageGroup is absent', () => {
    const zfs = diskResource('a', { used: 'ZFS' });
    const options = buildPhysicalDiskGroupFilterOptions([zfs]);
    expect(options).toEqual([
      { value: 'all', label: 'All groups' },
      { value: 'zfs', label: 'ZFS' },
    ]);
  });
});
