import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  UNIFIED_RESOURCE_TABLE_DEFAULT_LAYOUT_WIDTH,
  buildHostRowIndexById,
  buildHostTableItems,
  getHostRevealTargetIndex,
  getHostSpacerHeights,
  getNextUnifiedResourceTableSortState,
  normalizeUnifiedResourceTableLayoutWidth,
  shouldShowClusterGroupTypeLabel,
  sortServiceResources,
} from '@/components/Infrastructure/unifiedResourceTableStateModel';

/**
 * Branch-coverage additions for unifiedResourceTableStateModel.ts targeting the
 * arms left cold by the existing happy-path spec
 * (unifiedResourceTableStateModel.test.ts).
 *
 * Each test drives a specific measured-cold branch (default fallback, map miss,
 * nullish coalescing RHS, non-windowed early return, ternary consequent) and
 * asserts on the concrete returned value — never a tautology.
 *
 * The makeResource fixture mirrors the existing spec's builder so import style
 * and field shape stay consistent.
 */

const makeResource = (id: string, overrides: Partial<Resource> = {}): Resource =>
  ({
    id,
    type: 'agent',
    name: `name-${id}`,
    displayName: `Display ${id}`,
    platformId: `platform-${id}`,
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    platformData: { sources: ['proxmox'] },
    ...overrides,
  }) as Resource;

describe('normalizeUnifiedResourceTableLayoutWidth — default-fallback arm', () => {
  // Drives branch 6 arm 0 (lines 69-71): the trailing
  // `return UNIFIED_RESOURCE_TABLE_DEFAULT_LAYOUT_WIDTH;` reached only when the
  // width argument is invalid AND the fallback is also invalid. The existing
  // spec only exercises a valid width or an invalid width with a valid
  // fallback, so the final default return is never reached.
  it('falls back to the default constant when the width is absent and the fallback is NaN', () => {
    expect(normalizeUnifiedResourceTableLayoutWidth(null, Number.NaN)).toBe(
      UNIFIED_RESOURCE_TABLE_DEFAULT_LAYOUT_WIDTH,
    );
  });

  it('falls back to the default constant when the width is absent and the fallback is 0', () => {
    expect(normalizeUnifiedResourceTableLayoutWidth(null, 0)).toBe(
      UNIFIED_RESOURCE_TABLE_DEFAULT_LAYOUT_WIDTH,
    );
  });
});

describe('getHostRevealTargetIndex — map-miss returns null', () => {
  // Drives branch 31 arm 0 (line 150: `rowIndexById.get(targetId) ?? null`):
  // the LHS of `??` is undefined because targetId is absent from the map. The
  // existing spec always resolves a present id, so the nullish-coalescing RHS
  // is never taken.
  const items = buildHostTableItems(
    [{ cluster: 'cluster-a', resources: [makeResource('a'), makeResource('b')] }],
    'grouped',
  );
  const rowIndexById = buildHostRowIndexById(items);

  it('returns null when expandedResourceId is not present in the row index', () => {
    // rowIndexById only holds 'a' (1) and 'b' (2); 'missing' is absent.
    expect(getHostRevealTargetIndex(rowIndexById, 'missing', null)).toBeNull();
  });

  it('returns null when revealedResourceId is the chosen target but absent from the row index', () => {
    // expandedResourceId is null so revealedResourceId 'ghost' wins the chain,
    // then misses the map lookup.
    expect(getHostRevealTargetIndex(rowIndexById, null, null, 'ghost')).toBeNull();
  });
});

describe('shouldShowClusterGroupTypeLabel — nullish clusterLabel arm', () => {
  // Drives branch 35 arm 0 (line 166: `(clusterLabel ?? '')`): the RHS of `??`
  // is only evaluated when clusterLabel is null/undefined. The existing spec
  // passes only concrete strings, so the coalescing RHS stays cold.
  it('returns false for a null clusterLabel without throwing', () => {
    expect(shouldShowClusterGroupTypeLabel(null)).toBe(false);
  });

  it('returns false for an undefined clusterLabel without throwing', () => {
    expect(shouldShowClusterGroupTypeLabel(undefined)).toBe(false);
  });
});

describe('getHostSpacerHeights — non-windowed early-return arm', () => {
  // Drives branch 38 arm 0 (lines 177-179): the `if (!isWindowed)` consequent
  // that returns `{ top: 0, bottom: 0 }`. The existing spec always calls with
  // isWindowed = true, so the zero-spacer early return is never produced.
  it('returns zero spacers regardless of window bounds when windowing is disabled', () => {
    const result = getHostSpacerHeights(100, 5, 20, false);
    expect(result).toEqual({ top: 0, bottom: 0 });
  });

  it('returns zero spacers even when a custom row height is supplied', () => {
    // Discriminator: a custom estimatedRowHeight (40) would otherwise scale the
    // top spacer to 5 * 40 = 200; the early return suppresses it entirely.
    const result = getHostSpacerHeights(100, 5, 20, false, 40);
    expect(result).toEqual({ top: 0, bottom: 0 });
    expect(result.top).not.toBe(200);
  });
});

describe('getNextUnifiedResourceTableSortState — asc default for name/source', () => {
  // Drives branch 43 arm 0 (line 204: the `'asc'` consequent of
  // `nextKey === 'name' || nextKey === 'source' ? 'asc' : 'desc'`). The existing
  // spec only enters the function with currentKey === nextKey (so it never
  // reaches line 204) or with nextKey 'cpu' (which takes the 'desc' alternate).
  it('initializes a name sort to ascending when switching from a different key', () => {
    expect(getNextUnifiedResourceTableSortState('default', 'asc', 'name')).toEqual({
      key: 'name',
      direction: 'asc',
    });
  });

  it('initializes a source sort to ascending when switching from a different key', () => {
    // Also drives the second operand of the `||` (`nextKey === 'source'`) true.
    expect(getNextUnifiedResourceTableSortState('cpu', 'desc', 'source')).toEqual({
      key: 'source',
      direction: 'asc',
    });
  });
});

describe('sortServiceResources — uncovered exported function', () => {
  // Drives fn 15 (sortServiceResources, line 217): the only exported function
  // the existing spec never invokes. It must (a) filter to the requested
  // service type and (b) delegate to sortResources with 'default'/'asc', which
  // ranks online resources first then by display name. Assertions are made on
  // the concrete id sequence so they cannot pass against an unfiltered or
  // unsorted list.
  const services: Resource[] = [
    makeResource('zebra', { type: 'pbs', displayName: 'Zeta', status: 'offline' }),
    makeResource('alpha', { type: 'pbs', displayName: 'Alpha', status: 'online' }),
    makeResource('mango', { type: 'pbs', displayName: 'Mango', status: 'online' }),
    makeResource('pmg-only', { type: 'pmg', displayName: 'Should Be Filtered', status: 'online' }),
    makeResource('agent-only', { type: 'agent', displayName: 'Also Filtered', status: 'online' }),
  ];

  it('returns only pbs resources, online-first then by display name', () => {
    const result = sortServiceResources(services, 'pbs');
    expect(result.map((r) => r.id)).toEqual(['alpha', 'mango', 'zebra']);
    expect(result.every((r) => r.type === 'pbs')).toBe(true);
  });

  it('returns only pmg resources when type is pmg', () => {
    const result = sortServiceResources(services, 'pmg');
    expect(result.map((r) => r.id)).toEqual(['pmg-only']);
    expect(result.every((r) => r.type === 'pmg')).toBe(true);
  });

  it('returns an empty array when the input list is empty', () => {
    expect(sortServiceResources([], 'pbs')).toEqual([]);
  });

  it('does not mutate the input array', () => {
    const snapshot = services.map((r) => r.id);
    sortServiceResources(services, 'pbs');
    expect(services.map((r) => r.id)).toEqual(snapshot);
  });
});
