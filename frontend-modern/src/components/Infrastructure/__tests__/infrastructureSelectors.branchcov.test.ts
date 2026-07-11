import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  buildInfrastructureSummaryGroupScope,
  computeIOScale,
  filterResources,
  getOutlierEmphasis,
  infrastructureHasVisibleSummaryGroupScope,
  matchesSearch,
  sortResources,
} from '@/components/Infrastructure/infrastructureSelectors';

/**
 * Branch-coverage suite for the currently-uncovered branches of the target
 * functions in infrastructureSelectors.ts. The module-private helpers
 * (getSortValue, compareValues, defaultComparison, computeMedian,
 * computePercentile) are NOT exported, so they are exercised exclusively
 * through the exported entry points (sortResources, computeIOScale) and
 * asserted on observable return values — never re-implemented or imported
 * directly. The sibling infrastructureSelectors.test.ts already covers the
 * happy paths; this file targets only the remaining guard/edge arms.
 */

const makeResource = (i: number, overrides: Partial<Resource> = {}): Resource => ({
  id: `resource-${i}`,
  type: 'agent',
  name: `host-${i}`,
  displayName: `Host ${i}`,
  platformId: `platform-${i}`,
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000 + i,
  platformData: { sources: ['proxmox'] },
  ...overrides,
});

const BASELINE = { fontWeight: 'normal', color: 'text-muted', showOutlierHint: false };
const MEDIUM = { fontWeight: '500', color: 'text-base-content', showOutlierHint: true };
const HIGH = { fontWeight: '600', color: 'text-base-content', showOutlierHint: true };

describe('getSortValue (observed via sortResources)', () => {
  it('sorts by uptime and substitutes 0 when uptime is absent', () => {
    const resources = [
      makeResource(1, { displayName: 'Bravo', uptime: 100 }),
      makeResource(2, { displayName: 'Alpha', uptime: 50 }),
      makeResource(3, { displayName: 'Charlie' }), // uptime ?? 0
    ];
    expect(sortResources(resources, 'uptime', 'asc').map((r) => r.id)).toEqual([
      'resource-3',
      'resource-2',
      'resource-1',
    ]);
  });

  it('sorts by memory percent and yields null when memory is absent', () => {
    const resources = [
      makeResource(1, { displayName: 'Bravo', memory: { current: 80, total: 100, used: 80 } }),
      makeResource(2, { displayName: 'Alpha', memory: { current: 20, total: 100, used: 20 } }),
      makeResource(3, { displayName: 'Charlie' }), // null → bottom
    ];
    expect(sortResources(resources, 'memory', 'asc').map((r) => r.id)).toEqual([
      'resource-2',
      'resource-1',
      'resource-3',
    ]);
  });

  it('sorts by disk percent and yields null when disk is absent', () => {
    const resources = [
      makeResource(1, { displayName: 'Bravo', disk: { current: 90, total: 100, used: 90 } }),
      makeResource(2, { displayName: 'Alpha', disk: { current: 10, total: 100, used: 10 } }),
      makeResource(3, { displayName: 'Charlie' }), // null → bottom
    ];
    expect(sortResources(resources, 'disk', 'asc').map((r) => r.id)).toEqual([
      'resource-2',
      'resource-1',
      'resource-3',
    ]);
  });

  it('sorts by network byte total and yields null when network is absent', () => {
    const resources = [
      makeResource(1, { displayName: 'Bravo', network: { rxBytes: 100, txBytes: 50 } }),
      makeResource(2, { displayName: 'Alpha', network: { rxBytes: 10, txBytes: 5 } }),
      makeResource(3, { displayName: 'Charlie' }), // null → bottom
    ];
    expect(sortResources(resources, 'network', 'asc').map((r) => r.id)).toEqual([
      'resource-2',
      'resource-1',
      'resource-3',
    ]);
  });

  it('sorts by disk io rate total and yields null when diskIO is absent', () => {
    const resources = [
      makeResource(1, { displayName: 'Bravo', diskIO: { readRate: 200, writeRate: 100 } }),
      makeResource(2, { displayName: 'Alpha', diskIO: { readRate: 20, writeRate: 10 } }),
      makeResource(3, { displayName: 'Charlie' }), // null → bottom
    ];
    expect(sortResources(resources, 'diskio', 'asc').map((r) => r.id)).toEqual([
      'resource-2',
      'resource-1',
      'resource-3',
    ]);
  });

  it('sorts by temperature and yields null when temperature is absent', () => {
    const resources = [
      makeResource(1, { displayName: 'Bravo', temperature: 75 }),
      makeResource(2, { displayName: 'Alpha', temperature: 35 }),
      makeResource(3, { displayName: 'Charlie' }), // null → bottom
    ];
    expect(sortResources(resources, 'temp', 'asc').map((r) => r.id)).toEqual([
      'resource-2',
      'resource-1',
      'resource-3',
    ]);
  });

  it('returns null for an unknown sort key so ordering falls back to default', () => {
    const resources = [
      makeResource(1, { displayName: 'Bravo', status: 'offline' }),
      makeResource(2, { displayName: 'Alpha', status: 'online' }),
    ];
    // unknown key → getSortValue null for both → compareValues(null,null)=0
    // → defaultComparison (online first)
    expect(sortResources(resources, 'totally-unknown-key', 'asc').map((r) => r.id)).toEqual([
      'resource-2',
      'resource-1',
    ]);
  });
});

describe('compareValues (observed via sortResources)', () => {
  it('sorts a null metric after a finite metric in asc order (aEmpty branch)', () => {
    const resources = [
      makeResource(1, { displayName: 'Alpha' }), // no cpu → null
      makeResource(2, { displayName: 'Bravo', cpu: { current: 42 } }),
    ];
    expect(sortResources(resources, 'cpu', 'asc').map((r) => r.id)).toEqual([
      'resource-2',
      'resource-1',
    ]);
  });

  it('places a finite metric before a null metric in asc order (bEmpty branch)', () => {
    const resources = [
      makeResource(1, { displayName: 'Alpha', cpu: { current: 42 } }),
      makeResource(2, { displayName: 'Bravo' }), // no cpu → null
    ];
    expect(sortResources(resources, 'cpu', 'asc').map((r) => r.id)).toEqual([
      'resource-1',
      'resource-2',
    ]);
  });

  it('treats two null metric values as equal and defers to the default tie-break', () => {
    const resources = [
      makeResource(1, { displayName: 'Bravo', status: 'online' }), // no cpu
      makeResource(2, { displayName: 'Alpha', status: 'online' }), // no cpu
    ];
    // compareValues(null,null) === 0 → defaultComparison → Alpha before Bravo
    expect(sortResources(resources, 'cpu', 'asc').map((r) => r.id)).toEqual([
      'resource-2',
      'resource-1',
    ]);
  });
});

describe('defaultComparison (observed via sortResources default key)', () => {
  it('orders two offline resources by display name via localeCompare', () => {
    const resources = [
      makeResource(1, { displayName: 'Zeta', status: 'offline' }),
      makeResource(2, { displayName: 'Alpha', status: 'offline' }),
    ];
    expect(sortResources(resources, 'default', 'asc').map((r) => r.id)).toEqual([
      'resource-2',
      'resource-1',
    ]);
  });

  it('treats a "stopped" status as offline via the second isResourceOnline guard', () => {
    const resources = [
      makeResource(1, { displayName: 'Stopped-One', status: 'stopped' }),
      makeResource(2, { displayName: 'Online-One', status: 'online' }),
    ];
    // 'stopped' → isResourceOnline false → online resource sorts first
    expect(sortResources(resources, 'default', 'asc').map((r) => r.id)).toEqual([
      'resource-2',
      'resource-1',
    ]);
  });
});

describe('matchesSearch (branch coverage)', () => {
  it('returns true for an empty search term', () => {
    expect(matchesSearch(makeResource(1), '')).toBe(true);
  });

  it('matches against identity.hostname', () => {
    const resource = makeResource(1, {
      displayName: 'Showcase',
      identity: { hostname: 'db-prod-01.example.com' },
    });
    expect(matchesSearch(resource, 'db-prod-01')).toBe(true);
    expect(matchesSearch(resource, 'EXAMPLE.COM')).toBe(true);
  });

  it('does not crash and still matches by name/id when identity and tags are absent', () => {
    const resource = makeResource(1, { identity: undefined });
    expect(matchesSearch(resource, 'resource-1')).toBe(true);
    expect(matchesSearch(resource, 'host')).toBe(true);
    expect(matchesSearch(resource, 'no-such-term')).toBe(false);
  });
});

describe('filterResources (branch coverage)', () => {
  it('drops resources that carry no platform sources when a sources filter is active', () => {
    const resources = [
      makeResource(1, { platformData: { sources: ['proxmox'] } }),
      makeResource(2, { platformData: {} }), // resourceSources.length === 0 → excluded
      makeResource(3, { platformData: { sources: ['docker'] } }),
    ];
    expect(
      filterResources(resources, new Set(['proxmox-pve', 'docker']), new Set(), []).map(
        (r) => r.id,
      ),
    ).toEqual(['resource-1', 'resource-3']);
  });

  it('excludes resources whose sources do not intersect the active source filter', () => {
    const resources = [
      makeResource(1, { platformData: { sources: ['proxmox'] } }),
      makeResource(2, { platformData: { sources: ['docker'] } }),
    ];
    expect(
      filterResources(resources, new Set(['proxmox-pve']), new Set(), []).map((r) => r.id),
    ).toEqual(['resource-1']);
  });

  it('normalizes a missing status to "unknown" and filters against it', () => {
    const resources = [
      makeResource(1, { status: 'online' }),
      makeResource(2, { status: '' as unknown as Resource['status'] }), // || 'unknown'
      makeResource(3, { status: 'degraded' }),
    ];
    expect(
      filterResources(resources, new Set(), new Set(['unknown']), []).map((r) => r.id),
    ).toEqual(['resource-2']);
  });
});

describe('buildInfrastructureSummaryGroupScope (branch coverage)', () => {
  it('returns null when the group has no resources', () => {
    expect(buildInfrastructureSummaryGroupScope({ cluster: 'alpha', resources: [] })).toBeNull();
  });

  it('returns null when every resource id is blank or whitespace', () => {
    const resources = [makeResource(1, { id: '   ' }), makeResource(2, { id: '' })];
    expect(buildInfrastructureSummaryGroupScope({ cluster: 'alpha', resources })).toBeNull();
  });

  it('deduplicates series ids and emits a plural cluster label', () => {
    const resources = [
      makeResource(1, { id: 'node-a' }),
      makeResource(2, { id: 'node-a' }),
      makeResource(3, { id: 'node-b' }),
    ];
    expect(buildInfrastructureSummaryGroupScope({ cluster: 'beta', resources })).toEqual({
      id: 'cluster:beta',
      label: 'beta (3 resources)',
      seriesIds: ['node-a', 'node-b'],
    });
  });

  it('emits a standalone label and plural resourceCount for an empty cluster', () => {
    const resources = [makeResource(1, { id: 'solo-1' }), makeResource(2, { id: 'solo-2' })];
    expect(buildInfrastructureSummaryGroupScope({ cluster: '', resources })).toEqual({
      id: 'cluster:__standalone__',
      label: 'Standalone (2 resources)',
      seriesIds: ['solo-1', 'solo-2'],
    });
  });
});

describe('infrastructureHasVisibleSummaryGroupScope (branch coverage)', () => {
  const resources = [makeResource(1, { id: 'node-a' })];

  it('returns false when scope is null', () => {
    expect(infrastructureHasVisibleSummaryGroupScope(resources, null)).toBe(false);
  });

  it('returns false when scope is undefined', () => {
    expect(infrastructureHasVisibleSummaryGroupScope(resources, undefined)).toBe(false);
  });

  it('returns false when resources is empty but scope is present', () => {
    expect(
      infrastructureHasVisibleSummaryGroupScope([], {
        id: 'cluster:alpha',
        label: 'alpha (1 resource)',
        seriesIds: ['node-a'],
      }),
    ).toBe(false);
  });

  it('trims series ids before comparing against visible resource ids', () => {
    expect(
      infrastructureHasVisibleSummaryGroupScope(resources, {
        id: 'cluster:alpha',
        label: 'alpha (1 resource)',
        seriesIds: ['  node-a  '],
      }),
    ).toBe(true);
  });
});

describe('getOutlierEmphasis (branch coverage)', () => {
  const denseStats = { median: 10, mad: 2, max: 30, p97: 20, p99: 25, count: 10 };

  it('returns baseline for a non-finite value', () => {
    expect(getOutlierEmphasis(Number.NaN, denseStats)).toEqual(BASELINE);
  });

  it('returns baseline when value is <= 0', () => {
    expect(getOutlierEmphasis(0, denseStats)).toEqual(BASELINE);
    expect(getOutlierEmphasis(-5, denseStats)).toEqual(BASELINE);
  });

  it('returns baseline when stats.max <= 0', () => {
    expect(
      getOutlierEmphasis(5, { median: 0, mad: 0, max: 0, p97: 0, p99: 0, count: 5 }),
    ).toEqual(BASELINE);
  });

  describe('small sample path (count < 4)', () => {
    const smallStats = { median: 50, mad: 0, max: 100, p97: 99, p99: 99, count: 3 };

    it('returns medium emphasis at and above the 0.995 ratio boundary', () => {
      expect(getOutlierEmphasis(100, smallStats)).toEqual(MEDIUM); // ratio 1.0
      expect(getOutlierEmphasis(99.5, smallStats)).toEqual(MEDIUM); // ratio 0.995
    });

    it('returns baseline just below the 0.995 ratio boundary', () => {
      expect(getOutlierEmphasis(99.4, smallStats)).toEqual(BASELINE); // ratio 0.994
      expect(getOutlierEmphasis(50, smallStats)).toEqual(BASELINE);
    });
  });

  describe('flat distribution path (mad === 0, count >= 4)', () => {
    const flatStats = { median: 50, mad: 0, max: 100, p97: 80, p99: 90, count: 5 };

    it('returns high emphasis when value >= p99', () => {
      expect(getOutlierEmphasis(90, flatStats)).toEqual(HIGH);
      expect(getOutlierEmphasis(95, flatStats)).toEqual(HIGH);
    });

    it('returns medium emphasis when value >= p97 but < p99', () => {
      expect(getOutlierEmphasis(80, flatStats)).toEqual(MEDIUM);
      expect(getOutlierEmphasis(85, flatStats)).toEqual(MEDIUM);
    });

    it('returns baseline when value > 0 but < p97', () => {
      expect(getOutlierEmphasis(70, flatStats)).toEqual(BASELINE);
    });
  });

  it('returns baseline when modifiedZ is high but value sits below p97 (both && guards fail)', () => {
    // median 10, mad 0.1, value 15 → modifiedZ = 0.6745 * 50 = 33.7 (>= 6.5)
    // but 15 < p99(25) and 15 < p97(20) → neither outlier guard passes.
    const stats = { median: 10, mad: 0.1, max: 30, p97: 20, p99: 25, count: 10 };
    expect(getOutlierEmphasis(15, stats)).toEqual(BASELINE);
  });
});

describe('computeMedian / computePercentile (observed via computeIOScale)', () => {
  it('averages the two middle values for an even count of samples', () => {
    const resources = [
      makeResource(1, {
        network: { rxBytes: 10, txBytes: 0 }, // total 10
        diskIO: { readRate: 1, writeRate: 0 }, // total 1
      }),
      makeResource(2, {
        network: { rxBytes: 20, txBytes: 0 }, // total 20
        diskIO: { readRate: 3, writeRate: 0 }, // total 3
      }),
    ];
    expect(computeIOScale(resources)).toEqual({
      network: { median: 15, mad: 5, max: 20, p97: 20, p99: 20, count: 2 },
      diskIO: { median: 2, mad: 1, max: 3, p97: 3, p99: 3, count: 2 },
    });
  });

  it('selects the upper percentile element for a larger odd-length sample', () => {
    const values = [2, 4, 6, 8, 100];
    const resources = values.map((rxBytes, i) =>
      makeResource(i + 1, { network: { rxBytes, txBytes: 0 } }),
    );
    expect(computeIOScale(resources).network).toEqual({
      median: 6, // sorted[2]
      mad: 2, // deviations from 6: 4,2,0,2,94 → sorted 0,2,2,4,94 → median 2
      max: 100,
      p97: 100, // index = ceil(0.97 * 5) - 1 = 4
      p99: 100,
      count: 5,
    });
  });

  it('reports zeroed diskIO stats when no resource contributes diskIO samples', () => {
    const resources = [
      makeResource(1, { network: { rxBytes: 5, txBytes: 5 } }),
      makeResource(2, { network: { rxBytes: 15, txBytes: 5 } }),
    ];
    expect(computeIOScale(resources).diskIO).toEqual({
      median: 0,
      mad: 0,
      max: 0,
      p97: 0,
      p99: 0,
      count: 0,
    });
  });
});
