import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getPhysicalDiskSourceBadgePresentation,
  getPhysicalDiskSourceKey,
  hasUnraidPhysicalDiskFaultSignal,
  isUnraidPhysicalDisk,
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

// ---------------------------------------------------------------------------
// normalizePhysicalDiskState  (module-private — exercised via isUnraidPhysicalDisk)
// ---------------------------------------------------------------------------

describe('normalizePhysicalDiskState via isUnraidPhysicalDisk', () => {
  it('lowercases and trims storageGroup before comparing to "unraid-array"', () => {
    expect(
      isUnraidPhysicalDisk(makeDiskData({ storageGroup: '  UNRAID-Array  ' })),
    ).toBe(true);
  });

  it('lowercases and trims storageRole on the role+state path', () => {
    expect(
      isUnraidPhysicalDisk(
        makeDiskData({ storageRole: '  Cache  ', storageState: ' spinning ' }),
      ),
    ).toBe(true);
  });

  it('returns false on the role path when storageState is whitespace-only', () => {
    expect(
      isUnraidPhysicalDisk(
        makeDiskData({ storageRole: 'data', storageState: '   ' }),
      ),
    ).toBe(false);
  });

  it('returns false when neither storageGroup nor role+state match', () => {
    expect(
      isUnraidPhysicalDisk(
        makeDiskData({ storageRole: 'hot-spare', storageState: 'active' }),
      ),
    ).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// normalizePhysicalDiskHealth  (module-private — exercised via hasUnraidPhysicalDiskFaultSignal)
// ---------------------------------------------------------------------------

describe('normalizePhysicalDiskHealth via hasUnraidPhysicalDiskFaultSignal', () => {
  it('uppercases and trims health before checking the bad-health set', () => {
    const disk = makeDiskData({
      storageGroup: 'unraid-array',
      storageState: 'online',
      health: '  faulted  ',
      errorCount: 0,
    });
    expect(hasUnraidPhysicalDiskFaultSignal(disk)).toBe(true);
  });

  it('maps empty-string health to a value outside the bad-health set', () => {
    const disk = makeDiskData({
      storageGroup: 'unraid-array',
      storageState: 'online',
      health: '',
      errorCount: 0,
    });
    expect(hasUnraidPhysicalDiskFaultSignal(disk)).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// hasUnraidPhysicalDiskFaultSignal
// ---------------------------------------------------------------------------

describe('hasUnraidPhysicalDiskFaultSignal', () => {
  it('returns false for a non-Unraid disk regardless of errors or fault state', () => {
    expect(
      hasUnraidPhysicalDiskFaultSignal(
        makeDiskData({
          storageGroup: 'proxmox',
          storageState: 'missing',
          health: 'FAILED',
          errorCount: 99,
        }),
      ),
    ).toBe(false);
  });

  it.each(['disabled', 'invalid', 'missing', 'wrong', 'error', 'failed', 'faulted'])(
    'returns true for Unraid fault state "%s" via storageState (with clean health)',
    (faultState) => {
      expect(
        hasUnraidPhysicalDiskFaultSignal(
          makeDiskData({
            storageGroup: 'unraid-array',
            storageState: faultState,
            health: 'PASSED',
            errorCount: 0,
          }),
        ),
      ).toBe(true);
    },
  );

  it.each(['BAD', 'CRITICAL', 'ERROR', 'FAILED', 'FAIL', 'FAULTED', 'UNHEALTHY'])(
    'returns true for bad-health "%s" when storageState is clean and errorCount is zero',
    (health) => {
      expect(
        hasUnraidPhysicalDiskFaultSignal(
          makeDiskData({
            storageGroup: 'unraid-array',
            storageState: 'online',
            health,
            errorCount: 0,
          }),
        ),
      ).toBe(true);
    },
  );

  it('returns false for an Unraid disk with undefined storageState and known-good health', () => {
    expect(
      hasUnraidPhysicalDiskFaultSignal(
        makeDiskData({
          storageGroup: 'unraid-array',
          storageState: undefined,
          health: 'PASSED',
          errorCount: 0,
        }),
      ),
    ).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// readPhysicalDiskSourceCandidates  (module-private — exercised via getPhysicalDiskSourceKey)
// ---------------------------------------------------------------------------

describe('readPhysicalDiskSourceCandidates via getPhysicalDiskSourceKey', () => {
  it('reads platformData.sources', () => {
    const resource = {
      platformType: '',
      platformData: { sources: ['truenas'] },
    } as unknown as Resource;
    expect(getPhysicalDiskSourceKey(resource)).toBe('truenas');
  });

  it('reads direct resource.sources', () => {
    const resource = {
      platformType: '',
      sources: ['kubernetes'],
    } as unknown as Resource;
    expect(getPhysicalDiskSourceKey(resource)).toBe('kubernetes');
  });

  it('reads keys from platformData.sourceStatus object', () => {
    const resource = {
      platformType: '',
      platformData: { sourceStatus: { docker: { state: 'ok' } } },
    } as unknown as Resource;
    expect(getPhysicalDiskSourceKey(resource)).toBe('docker');
  });

  it('merges all three source arrays into one candidate list', () => {
    const resource = {
      platformType: '',
      sources: ['agent'],
      platformData: {
        sources: ['docker'],
        sourceStatus: { kubernetes: {} },
      },
    } as unknown as Resource;
    // resolvePlatformTypeFromSources applies priority (kubernetes > docker > agent),
    // so the result 'kubernetes' (sourced from sourceStatus) proves all three arrays
    // were read and merged — the individual tests above isolate each source path.
    expect(getPhysicalDiskSourceKey(resource)).toBe('kubernetes');
  });

  it('filters non-string, empty, and whitespace-only entries from source arrays', () => {
    const resource = {
      platformType: '',
      sources: [42, null, { platform: 'x' }, '', '   ', 'truenas'],
    } as unknown as Resource;
    expect(getPhysicalDiskSourceKey(resource)).toBe('truenas');
  });

  it('ignores a non-array sources value (falls back to platformType)', () => {
    const resource = {
      platformType: 'proxmox-pve',
      sources: 'truenas' as unknown as string[],
    } as unknown as Resource;
    expect(getPhysicalDiskSourceKey(resource)).toBe('proxmox-pve');
  });

  it('ignores a non-object sourceStatus value', () => {
    const resource = {
      platformType: '',
      platformData: { sourceStatus: 'truenas' },
    } as unknown as Resource;
    expect(getPhysicalDiskSourceKey(resource)).toBe('');
  });
});

// ---------------------------------------------------------------------------
// getPhysicalDiskSourceBadgePresentation
// ---------------------------------------------------------------------------

describe('getPhysicalDiskSourceBadgePresentation fallback branches', () => {
  it('falls back to "Unknown" label and text-base-content tone for an empty source key', () => {
    const result = getPhysicalDiskSourceBadgePresentation({} as Resource);
    expect(result.label).toBe('Unknown');
    expect(result.className.startsWith('text-base-content')).toBe(true);
    expect(result.className).toContain('text-[9px]');
  });

  it('title-cases an unrecognized non-empty source key for the label', () => {
    const resource = { platformType: 'zebra-fs' } as unknown as Resource;
    const result = getPhysicalDiskSourceBadgePresentation(resource);
    expect(result.label).toBe('Zebra Fs');
    expect(result.className.startsWith('text-base-content')).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// slugifyPhysicalDiskFacetValue  (module-private — exercised via normalizePhysicalDiskFacetFilter)
// ---------------------------------------------------------------------------

describe('slugifyPhysicalDiskFacetValue via normalizePhysicalDiskFacetFilter', () => {
  it('replaces special-character runs with single hyphens', () => {
    expect(normalizePhysicalDiskFacetFilter('Foo Bar & Baz!')).toBe('foo-bar-baz');
  });

  it('strips leading and trailing non-alphanumeric characters', () => {
    expect(normalizePhysicalDiskFacetFilter('!!!Cache!!!')).toBe('cache');
  });

  it('collapses multiple separators into one hyphen', () => {
    expect(normalizePhysicalDiskFacetFilter('a   b___c')).toBe('a-b-c');
  });

  it('returns "all" when only non-alphanumeric characters are provided', () => {
    expect(normalizePhysicalDiskFacetFilter('!!!???')).toBe('all');
  });
});

// ---------------------------------------------------------------------------
// normalizePhysicalDiskFacetFilter
// ---------------------------------------------------------------------------

describe('normalizePhysicalDiskFacetFilter', () => {
  it('returns "all" for null', () => {
    expect(normalizePhysicalDiskFacetFilter(null)).toBe('all');
  });

  it('returns "all" for undefined', () => {
    expect(normalizePhysicalDiskFacetFilter(undefined)).toBe('all');
  });

  it('returns "all" for empty string', () => {
    expect(normalizePhysicalDiskFacetFilter('')).toBe('all');
  });

  it('returns "all" when the slugified value equals the default filter "all"', () => {
    expect(normalizePhysicalDiskFacetFilter('ALL')).toBe('all');
  });

  it('preserves a normal slug value', () => {
    expect(normalizePhysicalDiskFacetFilter('Parity Disk 2')).toBe('parity-disk-2');
  });
});
