import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  comparePhysicalDiskPresentation,
  extractPhysicalDiskPresentationData,
  getPhysicalDiskEmptyStatePresentation,
  getPhysicalDiskHealthStatus,
  getPhysicalDiskHostLabel,
  getPhysicalDiskNormalizedHealth,
  getPhysicalDiskRoleLabel,
  hasPhysicalDiskSmartWarning,
  matchesPhysicalDiskFilterState,
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

// ---------------------------------------------------------------------------
// titleize  (module-private — exercised via getPhysicalDiskRoleLabel)
// ---------------------------------------------------------------------------

describe('titleize via getPhysicalDiskRoleLabel', () => {
  it('splits on hyphens and underscores within a storage role', () => {
    expect(getPhysicalDiskRoleLabel(makeDiskData({ storageRole: 'hot-spare_pool' }))).toBe(
      'Hot Spare Pool',
    );
  });

  it('collapses consecutive separators and filters empty parts', () => {
    expect(getPhysicalDiskRoleLabel(makeDiskData({ storageRole: 'data__  pool' }))).toBe(
      'Data Pool',
    );
  });

  it('returns empty string when both storageRole and type are absent', () => {
    expect(getPhysicalDiskRoleLabel(makeDiskData({}))).toBe('');
  });

  it('title-cases an unrecognized disk type in the fallback path', () => {
    expect(getPhysicalDiskRoleLabel(makeDiskData({ type: 'nvme-of' }))).toBe('Nvme Of disk');
  });
});

// ---------------------------------------------------------------------------
// getPhysicalDiskHealthFilterEmptyTitle  (module-private — exercised via
// getPhysicalDiskEmptyStatePresentation with diskCount > 0)
// ---------------------------------------------------------------------------

describe('getPhysicalDiskHealthFilterEmptyTitle via getPhysicalDiskEmptyStatePresentation', () => {
  it.each([
    ['healthy', 'No healthy disks found'],
    ['warning', 'No warning disks found'],
    ['critical', 'No critical disks found'],
    ['offline', 'No offline disks found'],
    ['unknown', 'No disks with unknown health'],
  ] as const)(
    'returns dedicated title "%s" for healthFilter "%s" when disks exist but none match',
    (filter, expectedTitle) => {
      const result = getPhysicalDiskEmptyStatePresentation({
        selectedNodeName: null,
        searchTerm: '',
        diskCount: 3,
        hasPVENodes: true,
        healthFilter: filter,
      });
      expect(result.title).toBe(expectedTitle);
    },
  );

  it('falls back to "No disks match these filters" when healthFilter is "all" and diskCount > 0', () => {
    const result = getPhysicalDiskEmptyStatePresentation({
      selectedNodeName: null,
      searchTerm: '',
      diskCount: 2,
      hasPVENodes: true,
    });
    expect(result.title).toBe('No disks match these filters');
  });
});

// ---------------------------------------------------------------------------
// extractPhysicalDiskPresentationData
// ---------------------------------------------------------------------------

describe('extractPhysicalDiskPresentationData branch coverage', () => {
  it('falls back to platformData.physicalDisk when resource.physicalDisk is absent', () => {
    const resource = {
      name: 'disk-1',
      platformType: 'agent',
      platformData: {
        physicalDisk: {
          devPath: '/dev/sda',
          model: 'Foo Drive',
          health: 'PASSED',
          diskType: 'ssd',
        },
      },
    } as unknown as Resource;

    const data = extractPhysicalDiskPresentationData(resource);
    expect(data.devPath).toBe('/dev/sda');
    expect(data.model).toBe('Foo Drive');
    expect(data.health).toBe('PASSED');
    expect(data.type).toBe('ssd');
  });

  it('returns a shell with defaults when neither physicalDisk nor platformData.physicalDisk exist', () => {
    const resource = { name: 'bare', platformType: 'agent' } as unknown as Resource;
    const data = extractPhysicalDiskPresentationData(resource);

    expect(data.devPath).toBe('');
    expect(data.model).toBe('bare');
    expect(data.serial).toBe('');
    expect(data.wwn).toBe('');
    expect(data.type).toBe('');
    expect(data.size).toBe(0);
    expect(data.health).toBe('UNKNOWN');
    expect(data.wearout).toBe(-1);
    expect(data.temperature).toBe(0);
    expect(data.rpm).toBe(0);
    expect(data.used).toBe('');
    expect(data.riskReasons).toEqual([]);
    expect(data.smartAttributes).toBeUndefined();
    expect(data.riskLevel).toBeUndefined();
  });

  it('filters non-string, null, and empty-string summaries from risk.reasons', () => {
    const resource = {
      name: 'disk',
      platformType: 'agent',
      physicalDisk: {
        risk: {
          level: 'warning',
          reasons: [
            { summary: 'Valid reason' },
            { summary: '' },
            { summary: null },
            { summary: 42 },
            { notSummary: 'x' },
            null,
          ],
        },
      },
    } as unknown as Resource;

    const data = extractPhysicalDiskPresentationData(resource);
    expect(data.riskLevel).toBe('warning');
    expect(data.riskReasons).toEqual(['Valid reason']);
  });

  it('returns empty riskReasons when risk.reasons is not an array', () => {
    const resource = {
      name: 'disk',
      platformType: 'agent',
      physicalDisk: {
        risk: { level: 'critical', reasons: 'not-an-array' },
      },
    } as unknown as Resource;

    const data = extractPhysicalDiskPresentationData(resource);
    expect(data.riskLevel).toBe('critical');
    expect(data.riskReasons).toEqual([]);
  });

  it('returns empty riskReasons when risk is absent', () => {
    const resource = {
      name: 'disk',
      platformType: 'agent',
      physicalDisk: { health: 'PASSED' },
    } as unknown as Resource;

    const data = extractPhysicalDiskPresentationData(resource);
    expect(data.riskReasons).toEqual([]);
    expect(data.riskLevel).toBeUndefined();
  });

  it('extracts all SMART attributes when pd.smart is present', () => {
    const resource = {
      name: 'disk',
      platformType: 'agent',
      physicalDisk: {
        smart: {
          powerOnHours: 1000,
          powerCycles: 50,
          reallocatedSectors: 1,
          pendingSectors: 2,
          offlineUncorrectable: 3,
          udmaCrcErrors: 4,
          percentageUsed: 5,
          availableSpare: 6,
          mediaErrors: 7,
          unsafeShutdowns: 8,
        },
      },
    } as unknown as Resource;

    const data = extractPhysicalDiskPresentationData(resource);
    expect(data.smartAttributes).toEqual({
      powerOnHours: 1000,
      powerCycles: 50,
      reallocatedSectors: 1,
      pendingSectors: 2,
      offlineUncorrectable: 3,
      udmaCrcErrors: 4,
      percentageUsed: 5,
      availableSpare: 6,
      mediaErrors: 7,
      unsafeShutdowns: 8,
    });
  });

  it('passes through optional storage fields and counters from physicalDisk', () => {
    const resource = {
      name: 'disk',
      platformType: 'agent',
      physicalDisk: {
        storageRole: 'data',
        storageGroup: 'array-1',
        storageState: 'online',
        spunDown: false,
        readCount: 100,
        writeCount: 200,
        errorCount: 5,
        sizeBytes: 1000000,
        wearout: 80,
        temperature: 35,
        rpm: 7200,
        used: 'ZFS',
      },
    } as unknown as Resource;

    const data = extractPhysicalDiskPresentationData(resource);
    expect(data.storageRole).toBe('data');
    expect(data.storageGroup).toBe('array-1');
    expect(data.storageState).toBe('online');
    expect(data.spunDown).toBe(false);
    expect(data.readCount).toBe(100);
    expect(data.writeCount).toBe(200);
    expect(data.errorCount).toBe(5);
    expect(data.size).toBe(1000000);
    expect(data.wearout).toBe(80);
    expect(data.temperature).toBe(35);
    expect(data.rpm).toBe(7200);
    expect(data.used).toBe('ZFS');
  });
});

// ---------------------------------------------------------------------------
// matchesPhysicalDiskFilterState
// ---------------------------------------------------------------------------

describe('matchesPhysicalDiskFilterState branch coverage', () => {
  const baseResource = {
    id: 'disk-1',
    name: 'disk-1',
    status: 'online',
    platformType: 'proxmox-pve',
    identity: { hostname: 'node1' },
  } as unknown as Resource;
  const baseDisk = makeDiskData({
    node: 'node1',
    health: 'PASSED',
    type: 'ssd',
    storageRole: 'cache',
    storageGroup: 'tank',
  });

  it('returns false when sourceFilter does not match the disk source', () => {
    expect(
      matchesPhysicalDiskFilterState(baseResource, baseDisk, { sourceFilter: 'truenas' }),
    ).toBe(false);
  });

  it('returns true when sourceFilter matches', () => {
    expect(
      matchesPhysicalDiskFilterState(baseResource, baseDisk, { sourceFilter: 'proxmox-pve' }),
    ).toBe(true);
  });

  it('returns false when healthFilter does not match the disk health', () => {
    expect(
      matchesPhysicalDiskFilterState(baseResource, baseDisk, { healthFilter: 'critical' }),
    ).toBe(false);
  });

  it('returns true when healthFilter "attention" matches a warning disk', () => {
    const warningDisk = makeDiskData({ node: 'node1', health: 'PASSED', riskLevel: 'warning' });
    expect(
      matchesPhysicalDiskFilterState(baseResource, warningDisk, { healthFilter: 'attention' }),
    ).toBe(true);
  });

  it('returns false when roleFilter does not match', () => {
    expect(
      matchesPhysicalDiskFilterState(baseResource, baseDisk, { roleFilter: 'parity' }),
    ).toBe(false);
  });

  it('returns true when roleFilter matches', () => {
    expect(
      matchesPhysicalDiskFilterState(baseResource, baseDisk, { roleFilter: 'cache' }),
    ).toBe(true);
  });

  it('returns false when groupFilter does not match', () => {
    expect(
      matchesPhysicalDiskFilterState(baseResource, baseDisk, { groupFilter: 'mirror' }),
    ).toBe(false);
  });

  it('returns true when groupFilter matches', () => {
    expect(
      matchesPhysicalDiskFilterState(baseResource, baseDisk, { groupFilter: 'tank' }),
    ).toBe(true);
  });

  it('returns false when search term does not match any haystack field', () => {
    expect(
      matchesPhysicalDiskFilterState(baseResource, baseDisk, { searchTerm: 'nonexistent-term' }),
    ).toBe(false);
  });

  it('returns true when all filters are default/empty', () => {
    expect(matchesPhysicalDiskFilterState(baseResource, baseDisk, {})).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// getPhysicalDiskNormalizedHealth
// ---------------------------------------------------------------------------

describe('getPhysicalDiskNormalizedHealth branch coverage', () => {
  it('returns "offline" when resource.status is "offline" (checked before Healthy)', () => {
    const resource = { status: 'offline' } as Resource;
    const disk = makeDiskData({ health: 'PASSED' });
    expect(getPhysicalDiskNormalizedHealth(resource, disk)).toBe('offline');
  });

  it('returns "critical" for Replace Now status', () => {
    const resource = { status: 'online' } as Resource;
    const disk = makeDiskData({ health: 'FAILED' });
    expect(getPhysicalDiskNormalizedHealth(resource, disk)).toBe('critical');
  });

  it('returns "healthy" for Healthy status (PASSED health)', () => {
    const resource = { status: 'online' } as Resource;
    const disk = makeDiskData({ health: 'PASSED' });
    expect(getPhysicalDiskNormalizedHealth(resource, disk)).toBe('healthy');
  });

  it('returns "healthy" for Healthy status (GOOD health)', () => {
    const resource = { status: 'online' } as Resource;
    const disk = makeDiskData({ health: 'GOOD' });
    expect(getPhysicalDiskNormalizedHealth(resource, disk)).toBe('healthy');
  });

  it('returns "unknown" for an unrecognized health label', () => {
    const resource = { status: 'online' } as Resource;
    const disk = makeDiskData({ health: 'SOMETHING' });
    expect(getPhysicalDiskNormalizedHealth(resource, disk)).toBe('unknown');
  });

  it('returns "healthy" for Online status before checking resource.status', () => {
    const resource = { status: 'offline' } as Resource;
    const disk = makeDiskData({
      health: 'UNKNOWN',
      storageRole: 'data',
      storageGroup: 'unraid-array',
      storageState: 'online',
    });
    expect(getPhysicalDiskNormalizedHealth(resource, disk)).toBe('healthy');
  });
});

// ---------------------------------------------------------------------------
// getPhysicalDiskHostLabel
// ---------------------------------------------------------------------------

describe('getPhysicalDiskHostLabel branch coverage', () => {
  it('returns disk.node when set', () => {
    expect(getPhysicalDiskHostLabel(makeDiskData({ node: 'tower' }), {} as Resource)).toBe('tower');
  });

  it('falls back to resource.parentName when disk.node is empty', () => {
    expect(
      getPhysicalDiskHostLabel(makeDiskData({ node: '' }), { parentName: 'pve-01' } as Resource),
    ).toBe('pve-01');
  });

  it('returns empty string when both disk.node and parentName are empty', () => {
    expect(
      getPhysicalDiskHostLabel(makeDiskData({ node: '' }), { parentName: '' } as Resource),
    ).toBe('');
  });

  it('trims whitespace from the resolved host label', () => {
    expect(
      getPhysicalDiskHostLabel(makeDiskData({ node: '  tower  ' }), {} as Resource),
    ).toBe('tower');
  });
});

// ---------------------------------------------------------------------------
// comparePhysicalDiskPresentation
// ---------------------------------------------------------------------------

describe('comparePhysicalDiskPresentation branch coverage', () => {
  const r = { name: 'd' } as Resource;

  it('sorts critical risk (300) ahead of no risk (0)', () => {
    const noneDisk = makeDiskData({ node: 'n', devPath: '/dev/sda' });
    const criticalDisk = makeDiskData({ node: 'n', devPath: '/dev/sda', riskLevel: 'critical' });
    expect(comparePhysicalDiskPresentation(r, criticalDisk, r, noneDisk)).toBeLessThan(0);
  });

  it('adds +50 priority for SMART warnings on top of warning risk', () => {
    const warningWithSmart = makeDiskData({
      node: 'n',
      devPath: '/dev/sda',
      riskLevel: 'warning',
      smartAttributes: { reallocatedSectors: 1 },
    });
    const warningNoSmart = makeDiskData({ node: 'n', devPath: '/dev/sdb', riskLevel: 'warning' });
    expect(comparePhysicalDiskPresentation(r, warningWithSmart, r, warningNoSmart)).toBeLessThan(0);
  });

  it('falls back to node localeCompare when priorities are equal', () => {
    const diskA = makeDiskData({ node: 'alpha', devPath: '/dev/sda' });
    const diskB = makeDiskData({ node: 'beta', devPath: '/dev/sda' });
    expect(comparePhysicalDiskPresentation(r, diskA, r, diskB)).toBeLessThan(0);
  });

  it('falls back to devPath localeCompare when priority and node are equal', () => {
    const diskA = makeDiskData({ node: 'n', devPath: '/dev/sda' });
    const diskB = makeDiskData({ node: 'n', devPath: '/dev/sdb' });
    expect(comparePhysicalDiskPresentation(r, diskA, r, diskB)).toBeLessThan(0);
  });

  it('falls back to resource.name when both devPaths are empty', () => {
    const rA = { name: 'aaa' } as Resource;
    const rB = { name: 'bbb' } as Resource;
    const diskA = makeDiskData({ node: 'n', devPath: '' });
    const diskB = makeDiskData({ node: 'n', devPath: '' });
    expect(comparePhysicalDiskPresentation(rA, diskA, rB, diskB)).toBeLessThan(0);
  });
});

// ---------------------------------------------------------------------------
// hasPhysicalDiskSmartWarning
// ---------------------------------------------------------------------------

describe('hasPhysicalDiskSmartWarning branch coverage', () => {
  it('returns false when smartAttributes is undefined', () => {
    expect(hasPhysicalDiskSmartWarning(makeDiskData({}))).toBe(false);
  });

  it('returns true when reallocatedSectors > 0', () => {
    expect(
      hasPhysicalDiskSmartWarning(makeDiskData({ smartAttributes: { reallocatedSectors: 5 } })),
    ).toBe(true);
  });

  it('returns true when mediaErrors > 0', () => {
    expect(
      hasPhysicalDiskSmartWarning(makeDiskData({ smartAttributes: { mediaErrors: 3 } })),
    ).toBe(true);
  });

  it('returns false when all warning counters are zero', () => {
    expect(
      hasPhysicalDiskSmartWarning(
        makeDiskData({
          smartAttributes: { reallocatedSectors: 0, pendingSectors: 0, mediaErrors: 0 },
        }),
      ),
    ).toBe(false);
  });

  it('returns false when warning counters are undefined but other attrs exist', () => {
    expect(
      hasPhysicalDiskSmartWarning(makeDiskData({ smartAttributes: { powerOnHours: 1000 } })),
    ).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// getPhysicalDiskHealthStatus
// ---------------------------------------------------------------------------

describe('getPhysicalDiskHealthStatus branch coverage', () => {
  it('returns Replace Now for critical riskLevel even when health is not FAILED', () => {
    const disk = makeDiskData({ health: 'PASSED', riskLevel: 'critical', riskReasons: ['Bad'] });
    expect(getPhysicalDiskHealthStatus(disk)).toEqual({
      label: 'Replace Now',
      summary: 'Bad',
      tone: 'text-red-700 dark:text-red-300',
    });
  });

  it('uses default summary for Replace Now when riskReasons is empty', () => {
    const disk = makeDiskData({ health: 'FAILED', riskReasons: [] });
    const status = getPhysicalDiskHealthStatus(disk);
    expect(status.label).toBe('Replace Now');
    expect(status.summary).toBe('Disk health has degraded to a critical state.');
  });

  it('returns Needs Attention for SMART warning without risk level (default summary)', () => {
    const disk = makeDiskData({
      health: 'PASSED',
      smartAttributes: { reallocatedSectors: 1 },
    });
    const status = getPhysicalDiskHealthStatus(disk);
    expect(status.label).toBe('Needs Attention');
    expect(status.summary).toBe('SMART counters indicate elevated risk.');
    expect(status.tone).toBe('text-amber-700 dark:text-amber-300');
  });

  it('returns Needs Attention with low-life SSD summary when wearout is between 0 and 10', () => {
    const disk = makeDiskData({ health: 'PASSED', wearout: 5 });
    const status = getPhysicalDiskHealthStatus(disk);
    expect(status.label).toBe('Needs Attention');
    expect(status.summary).toBe('SSD life is running low.');
  });

  it('uses riskReasons[0] for Needs Attention summary when available', () => {
    const disk = makeDiskData({
      health: 'PASSED',
      riskLevel: 'warning',
      riskReasons: ['Wear leveling exceeded.'],
    });
    expect(getPhysicalDiskHealthStatus(disk).summary).toBe('Wear leveling exceeded.');
  });

  it('returns Healthy for GOOD health', () => {
    const disk = makeDiskData({ health: 'GOOD' });
    expect(getPhysicalDiskHealthStatus(disk)).toEqual({
      label: 'Healthy',
      summary: 'No active disk-health issues.',
      tone: 'text-base-content',
    });
  });

  it('returns Healthy for PASSED health', () => {
    const disk = makeDiskData({ health: 'PASSED' });
    expect(getPhysicalDiskHealthStatus(disk).label).toBe('Healthy');
  });

  it('returns Unknown with "Health state is not reported." for unrecognized health', () => {
    const disk = makeDiskData({ health: 'WEIRD' });
    expect(getPhysicalDiskHealthStatus(disk)).toEqual({
      label: 'Unknown',
      summary: 'Health state is not reported.',
      tone: 'text-base-content',
    });
  });

  it('returns Unknown label for Unraid disk whose storageState is not "online"', () => {
    const disk = makeDiskData({
      health: 'UNKNOWN',
      storageRole: 'data',
      storageGroup: 'unraid-array',
      storageState: 'spinning',
    });
    const status = getPhysicalDiskHealthStatus(disk);
    expect(status.label).toBe('Unknown');
    expect(status.summary).toBe('No active disk-health issues.');
  });

  it('does not classify wearout of 0 as low life (unreported boundary)', () => {
    const disk = makeDiskData({ health: 'PASSED', wearout: 0 });
    expect(getPhysicalDiskHealthStatus(disk).label).toBe('Healthy');
  });

  it('does not classify wearout of exactly 10 as low life (upper boundary)', () => {
    const disk = makeDiskData({ health: 'PASSED', wearout: 10 });
    expect(getPhysicalDiskHealthStatus(disk).label).toBe('Healthy');
  });
});

// ---------------------------------------------------------------------------
// getPhysicalDiskEmptyStatePresentation
// ---------------------------------------------------------------------------

describe('getPhysicalDiskEmptyStatePresentation branch coverage', () => {
  it('includes searchMessage when searchTerm is provided', () => {
    const result = getPhysicalDiskEmptyStatePresentation({
      selectedNodeName: null,
      searchTerm: 'ssd',
      diskCount: 5,
      hasPVENodes: true,
    });
    expect(result.searchMessage).toBe('matching "ssd"');
  });

  it('returns null searchMessage when searchTerm is empty', () => {
    const result = getPhysicalDiskEmptyStatePresentation({
      selectedNodeName: null,
      searchTerm: '',
      diskCount: 0,
      hasPVENodes: false,
    });
    expect(result.searchMessage).toBeNull();
  });

  it('hides requirements when a scoped filter is active even if diskCount is 0', () => {
    const result = getPhysicalDiskEmptyStatePresentation({
      selectedNodeName: null,
      searchTerm: 'foo',
      diskCount: 0,
      hasPVENodes: true,
    });
    expect(result.showRequirements).toBe(false);
  });

  it('hides requirements when hasPVENodes is false', () => {
    const result = getPhysicalDiskEmptyStatePresentation({
      selectedNodeName: null,
      searchTerm: '',
      diskCount: 0,
      hasPVENodes: false,
    });
    expect(result.showRequirements).toBe(false);
  });

  it('builds filterMessages only for provided labels', () => {
    const result = getPhysicalDiskEmptyStatePresentation({
      selectedNodeName: null,
      searchTerm: '',
      diskCount: 3,
      hasPVENodes: true,
      sourceFilterLabel: 'PVE',
    });
    expect(result.filterMessages).toEqual(['from PVE']);
  });

  it('returns null nodeMessage when selectedNodeName is null', () => {
    const result = getPhysicalDiskEmptyStatePresentation({
      selectedNodeName: null,
      searchTerm: '',
      diskCount: 0,
      hasPVENodes: false,
    });
    expect(result.nodeMessage).toBeNull();
  });

  it('includes all requirement fields when showRequirements is true', () => {
    const result = getPhysicalDiskEmptyStatePresentation({
      selectedNodeName: null,
      searchTerm: '',
      diskCount: 0,
      hasPVENodes: true,
    });
    expect(result.showRequirements).toBe(true);
    expect(result.requirementsTitle).toBe('Physical disk monitoring requirements:');
    expect(result.requirementsNote).toBe(
      'Note: Both Pulse and Proxmox must have SMART monitoring enabled.',
    );
    expect(result.requirementsItems).toHaveLength(3);
  });
});
