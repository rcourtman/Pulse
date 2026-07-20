import { describe, expect, it } from 'vitest';
import { orderSourcePlatformKeys } from '@/utils/sourcePlatformOptions';

// `orderSourcePlatformKeys` is the only exported function with uncovered branches
// after the sibling `sourcePlatformOptions.test.ts` suite (which exercises only the
// happy path of canonical keys present in DEFAULT_SOURCE_PLATFORM_ORDER). This file
// targets the gaps: empty/degenerate inputs, the .filter(Boolean) falsy outcome,
// Set-based deduplication, non-Array iterables, the explicit-preferredOrder arm,
// the indexA === -1 / indexB === -1 sort arms, and the localeCompare fallback that
// fires when NEITHER compared key appears in preferredOrder.

describe('orderSourcePlatformKeys — branch coverage (batch 0713)', () => {
  describe('empty / falsy inputs', () => {
    it('returns [] for an empty array', () => {
      expect(orderSourcePlatformKeys([])).toEqual([]);
    });

    it('returns [] for an empty Set (Iterable protocol, not Array)', () => {
      expect(orderSourcePlatformKeys(new Set<string>())).toEqual([]);
    });

    it('returns [] when every entry normalizes to empty string (.filter(Boolean) all-falsy arm)', () => {
      // normalizeSourcePlatformQueryValue trims+lowercases; whitespace-only inputs
      // collapse to '' and are stripped by .filter(Boolean).
      expect(orderSourcePlatformKeys(['', '   ', '\t', '\n'])).toEqual([]);
    });

    it('strips entries that normalize to "" while keeping valid ones (.filter(Boolean) truthy + falsy arms)', () => {
      expect(orderSourcePlatformKeys(['agent', '', '   ', 'docker'])).toEqual(['agent', 'docker']);
    });

    it('tolerates null/undefined entries that violate the declared Iterable<string>', () => {
      // The implementation funnels every entry through normalizeSourcePlatformQueryValue,
      // which accepts `string | null | undefined` and maps nullish to "". Those empty
      // results are then dropped by .filter(Boolean), so the canonical keys survive.
      const malformed = ['agent', null, undefined, 'docker'] as unknown as Parameters<
        typeof orderSourcePlatformKeys
      >[0];
      expect(orderSourcePlatformKeys(malformed)).toEqual(['agent', 'docker']);
    });
  });

  describe('Iterable protocol support', () => {
    it('deduplicates alias and case variants through the inner new Set(...) construction', () => {
      // 'PBS' and 'pbs' both alias to 'proxmox-pbs'; 'AGENT' and 'agent' both
      // canonicalize to 'agent'. The surrounding new Set collapses each pair to a
      // single canonical entry, so the output has one of each.
      expect(orderSourcePlatformKeys(['PBS', 'pbs', 'AGENT', 'agent'])).toEqual([
        'agent',
        'proxmox-pbs',
      ]);
    });

    it('accepts a generator as input (Iterable, not Array)', () => {
      const generate = function* (): Generator<string> {
        yield 'docker';
        yield 'agent';
      };
      expect(orderSourcePlatformKeys(generate())).toEqual(['agent', 'docker']);
    });
  });

  describe('sort comparator: localeCompare fallback for two unknown keys', () => {
    it('orders two unknown keys alphabetically by presentation label when neither is in preferredOrder', () => {
      // Both 'zzz-custom' and 'aaa-custom' miss DEFAULT_SOURCE_PLATFORM_ORDER, so the
      // outer `if (indexA !== -1 || indexB !== -1)` is false and the comparator
      // falls through to getSourcePlatformLabel(a).localeCompare(getSourcePlatformLabel(b)).
      // Labels: 'Aaa Custom' < 'Zzz Custom' -> aaa-custom sorts first.
      expect(orderSourcePlatformKeys(['zzz-custom', 'aaa-custom'])).toEqual([
        'aaa-custom',
        'zzz-custom',
      ]);
    });

    it('keeps the "all" sentinel (truthy after normalize) and routes it through the label fallback', () => {
      // normalizeSourcePlatformQueryValue('all') returns 'all', which is truthy and
      // survives .filter(Boolean). 'all' is NOT in DEFAULT_SOURCE_PLATFORM_ORDER, so
      // it falls through to localeCompare alongside other unknowns.
      expect(orderSourcePlatformKeys(['all', 'agent'])).toEqual(['agent', 'all']);
    });
  });

  describe('sort comparator: known-vs-unknown arms', () => {
    it('places a known key before an unknown key (indexB === -1 -> return -1 arm)', () => {
      // Input order [known, unknown]: insertion sort calls compare('agent', 'custom-x').
      // indexA = 0 (agent in DEFAULT order), indexB = -1 (custom-x absent) -> return -1
      // -> agent stays before custom-x.
      expect(orderSourcePlatformKeys(['agent', 'custom-x'])).toEqual(['agent', 'custom-x']);
    });

    it('places an unknown key after a known key (indexA === -1 -> return 1 arm)', () => {
      // Input order [unknown, known]: insertion sort calls compare('custom-x', 'agent').
      // indexA = -1 (custom-x absent), indexB = 0 (agent in DEFAULT order) -> return 1
      // -> custom-x is pushed after agent.
      expect(orderSourcePlatformKeys(['custom-x', 'agent'])).toEqual(['agent', 'custom-x']);
    });

    it('exercises all four comparator outcomes across a single mixed input', () => {
      // agent (index 0) vs docker (index 6) -> return indexA - indexB arm.
      // agent/docker vs aaa-custom/zzz-custom -> indexA === -1 and indexB === -1 arms.
      // aaa-custom vs zzz-custom -> localeCompare fallback.
      expect(orderSourcePlatformKeys(['zzz-custom', 'agent', 'docker', 'aaa-custom'])).toEqual([
        'agent',
        'docker',
        'aaa-custom',
        'zzz-custom',
      ]);
    });
  });

  describe('explicit preferredOrder parameter (default-parameter branch)', () => {
    it('honors a caller-supplied preferredOrder instead of the default', () => {
      // With preferredOrder = ['docker', 'agent'], docker precedes agent even though
      // DEFAULT_SOURCE_PLATFORM_ORDER ranks agent first. This exercises the
      // "argument provided" arm of the default parameter.
      expect(orderSourcePlatformKeys(['agent', 'docker'], ['docker', 'agent'])).toEqual([
        'docker',
        'agent',
      ]);
    });

    it('still falls back to localeCompare for keys absent from the custom preferredOrder', () => {
      // preferredOrder = ['agent'] excludes both custom keys, forcing the comparator
      // into its localeCompare fallback branch.
      expect(orderSourcePlatformKeys(['zzz-custom', 'aaa-custom'], ['agent'])).toEqual([
        'aaa-custom',
        'zzz-custom',
      ]);
    });

    it('ranks preferred keys ahead of non-preferred keys, then orders unknowns by label', () => {
      // 'agent' is the only entry in the supplied preferredOrder, so it sorts first;
      // 'aaa-custom' and 'bbb-custom' fall through to label-based ordering.
      expect(orderSourcePlatformKeys(['bbb-custom', 'agent', 'aaa-custom'], ['agent'])).toEqual([
        'agent',
        'aaa-custom',
        'bbb-custom',
      ]);
    });
  });
});
