import { describe, expect, it } from 'vitest';

import {
  filterSummarySeriesByGroupScope,
  isSummarySeriesInGroupScope,
  normalizeSummarySeriesGroupScope,
  resolveSummaryGroupMemberInteractionState,
  resolveSummaryScopeState,
  type SummarySeriesGroupScope,
} from '@/components/shared/summaryCardInteraction';

// Branch-coverage tests for the UNCOVERED guard arms of
// summaryCardInteraction.ts. The existing spec (summaryCardInteraction.test.ts)
// exercises the precedence/happy paths; this file targets the null/empty/
// malformed-input arms that never fire there. A scoped v8 coverage run against
// the existing spec alone leaves exactly 8 of 56 branch arms cold (0 functions):
//   b6  normalizeSummarySeriesGroupScope: `scope.seriesIds ?? []` nullish-fallback
//   b8  normalizeSummarySeriesGroupScope: valid id but `seriesIds.length === 0` -> null
//   b10 normalizeSummarySeriesGroupScope: `typeof scope.label !== 'string'` else arm
//   b11 normalizeSummarySeriesGroupScope: `label || null` right operand (falsy label)
//   b14 isSummarySeriesInGroupScope: valid scope but empty seriesId -> false
//   b19 resolveSummaryGroupMemberInteractionState: empty seriesId -> 'default'
//   b37 resolveSummaryScopeState: hoveredGroupScope truthy -> group/preview return
//   b43 filterSummarySeriesByGroupScope: null/invalid scope -> `[...series]`
// Every asserted value below is hand-computed against the source in
// src/components/shared/summaryCardInteraction.ts — no snapshots, no
// constant-equals-itself tautologies. Deliberately-malformed inputs (missing
// required `seriesIds`, non-string `label`) are cast via `as unknown as`.

const validScope: SummarySeriesGroupScope = {
  id: 'cluster-a',
  label: 'Cluster A',
  seriesIds: ['alpha', 'beta'],
};

describe('summaryCardInteraction.branchcov0724pm', () => {
  // -------------------------------------------------------------------------
  // normalizeSummarySeriesGroupScope — the 4 normalization cold arms live here.
  // The existing spec always passes a fully-populated scope with at least one
  // valid series id and a string label, so every defensive branch is cold.
  // -------------------------------------------------------------------------
  describe('normalizeSummarySeriesGroupScope', () => {
    it('returns null when seriesIds is nullish, exercising the `?? []` fallback (b6) and the empty -> null path (b8)', () => {
      // `scope.seriesIds` is undefined -> the `?? []` right operand is taken
      // (b6). The resulting list is empty, so `seriesIds.length === 0` is true
      // (b8) and the function returns null. seriesIds is a required field on the
      // type, so the malformed object is cast.
      const malformed = { id: 'cluster-a' } as unknown as SummarySeriesGroupScope;
      expect(normalizeSummarySeriesGroupScope(malformed)).toBeNull();
    });

    it('returns null when seriesIds is present but empty, proving b8 fires independently of the `?? []` fallback', () => {
      // seriesIds is a real (empty) array -> `?? []` NOT taken (b6 stays cold
      // here), but `seriesIds.length === 0` is still true -> b8 -> null.
      expect(normalizeSummarySeriesGroupScope({ id: 'cluster-a', seriesIds: [] })).toBeNull();
    });

    it('coerces a non-string label to "" then null, covering the ternary else arm (b10) and the `|| null` arm (b11)', () => {
      // scope.label omitted -> `typeof undefined === 'string'` is false (b10
      // else arm) -> label = '' -> `'' || null` takes the right operand (b11)
      // -> returned label is null. Asserts the full observable shape, not just
      // the label slot.
      expect(normalizeSummarySeriesGroupScope({ id: 'cluster-a', seriesIds: ['alpha'] })).toEqual({
        id: 'cluster-a',
        label: null,
        seriesIds: ['alpha'],
      });
    });

    it('yields label: null for a whitespace-only string label that trims to empty', () => {
      // label IS a string (ternary then-arm, already covered) but trims to ''
      // -> `'' || null` right operand (b11) -> label: null.
      expect(
        normalizeSummarySeriesGroupScope({
          id: 'cluster-a',
          label: '   ',
          seriesIds: ['alpha'],
        }),
      ).toEqual({
        id: 'cluster-a',
        label: null,
        seriesIds: ['alpha'],
      });
    });

    it('trims and preserves a real label string (then + left-operand path, regression guard for the b10/b11 neighbours)', () => {
      // label is a non-empty string -> then-arm trims it -> `label || null`
      // left operand -> returned verbatim. Locks the non-cold neighbour so the
      // b10/b11 coverage cannot be silently regarded by a refactor. Asserting
      // the full shape avoids a null-deref (the helper returns ... | null).
      expect(
        normalizeSummarySeriesGroupScope({
          id: 'cluster-a',
          label: '  Cluster A  ',
          seriesIds: ['alpha'],
        }),
      ).toEqual({
        id: 'cluster-a',
        label: 'Cluster A',
        seriesIds: ['alpha'],
      });
    });
  });

  // -------------------------------------------------------------------------
  // isSummarySeriesInGroupScope — the `!normalizedSeriesId` short-circuit.
  // The existing spec only ever calls this with a non-empty seriesId.
  // -------------------------------------------------------------------------
  describe('isSummarySeriesInGroupScope', () => {
    it('returns false when the scope is valid but the seriesId is empty/blank (b14)', () => {
      // normalizedScope is valid (truthy) so `!normalizedScope` is false; the
      // `||` evaluates `!normalizedSeriesId` which is true for '' / '   ' /
      // null -> returns false without reaching .includes.
      expect(isSummarySeriesInGroupScope(validScope, '')).toBe(false);
      expect(isSummarySeriesInGroupScope(validScope, '   ')).toBe(false);
      expect(isSummarySeriesInGroupScope(validScope, null)).toBe(false);
      expect(isSummarySeriesInGroupScope(validScope, undefined)).toBe(false);
    });

    it('still returns true for a real match so the b14 guard is proven not to over-reject', () => {
      expect(isSummarySeriesInGroupScope(validScope, 'alpha')).toBe(true);
      // 'alpha' with surrounding whitespace still matches after trim.
      expect(isSummarySeriesInGroupScope(validScope, '  alpha  ')).toBe(true);
    });
  });

  // -------------------------------------------------------------------------
  // resolveSummaryGroupMemberInteractionState — the empty-seriesId early
  // return. The existing spec always passes a concrete seriesId.
  // -------------------------------------------------------------------------
  describe('resolveSummaryGroupMemberInteractionState', () => {
    it("returns 'default' when seriesId is missing/blank, before consulting any group scope (b19)", () => {
      // The very first guard `if (!normalizedSeriesId) return 'default';` fires
      // for '' / '   ' / undefined / null.
      expect(resolveSummaryGroupMemberInteractionState({})).toBe('default');
      expect(resolveSummaryGroupMemberInteractionState({ seriesId: '' })).toBe('default');
      expect(resolveSummaryGroupMemberInteractionState({ seriesId: '   ' })).toBe('default');
      expect(resolveSummaryGroupMemberInteractionState({ seriesId: null })).toBe('default');
    });

    it("returns 'default' for an empty seriesId even when a hoveredGroupScope is supplied, proving the guard precedes the scope checks", () => {
      // A valid hoveredGroupScope would otherwise yield 'preview' for an in-scope
      // id; the empty-seriesId guard must short-circuit first.
      expect(
        resolveSummaryGroupMemberInteractionState({
          seriesId: '',
          hoveredGroupScope: validScope,
          focusedGroupScope: validScope,
        }),
      ).toBe('default');
    });
  });

  // -------------------------------------------------------------------------
  // resolveSummaryScopeState — the hoveredGroupScope truthy arm. Every existing
  // call either resolves an active entity series first, or reaches this point
  // only with hoveredGroupScope === null/undefined (focused path).
  // -------------------------------------------------------------------------
  describe('resolveSummaryScopeState', () => {
    it('returns a group/preview scope state when only hoveredGroupScope is supplied (b37)', () => {
      // No series ids and no explicit groupScope option -> groupScope resolves
      // to hoveredGroupScope via resolveSummaryGroupScope; all three series
      // guards are skipped (empty ids) -> hoveredGroupScope is truthy at
      // line 150 -> kind 'group', source 'preview'.
      expect(resolveSummaryScopeState({ hoveredGroupScope: validScope })).toEqual({
        groupScope: validScope,
        kind: 'group',
        seriesId: null,
        source: 'preview',
      });
    });

    it('falls through to the hoveredGroupScope group return even when a hovered series id is filtered out by that same scope (b37 via the scope-mismatch skip)', () => {
      // groupScope resolves to hoveredGroupScope (alpha,beta). hoveredSeriesId
      // 'gamma' is truthy but NOT in the resolved scope -> the entity condition
      // is false -> skipped -> reaches the hoveredGroupScope check -> b37.
      expect(
        resolveSummaryScopeState({
          hoveredSeriesId: 'gamma',
          hoveredGroupScope: validScope,
        }),
      ).toEqual({
        groupScope: validScope,
        kind: 'group',
        seriesId: null,
        source: 'preview',
      });
    });

    it('prefers a valid chartHoveredSeriesId over the hoveredGroupScope group return, confirming b37 is precedence-gated (regression guard)', () => {
      // chartHoveredSeriesId 'alpha' is in the resolved scope -> entity/preview
      // return wins; the hoveredGroupScope group arm (b37) is NOT taken here.
      expect(
        resolveSummaryScopeState({
          chartHoveredSeriesId: 'alpha',
          hoveredGroupScope: validScope,
        }),
      ).toEqual({
        groupScope: validScope,
        kind: 'entity',
        seriesId: 'alpha',
        source: 'preview',
      });
    });
  });

  // -------------------------------------------------------------------------
  // filterSummarySeriesByGroupScope — the no-scope -> shallow-copy arm. The
  // existing spec only calls this with a valid scope (the .filter path).
  // -------------------------------------------------------------------------
  describe('filterSummarySeriesByGroupScope', () => {
    it('returns a copy of every series when the scope is null/undefined (b43)', () => {
      const series = [{ id: 'alpha' }, { id: 'beta' }, { id: 'gamma' }];
      // null scope -> normalizedScope null -> `return [...series]` (b43).
      const fromNull = filterSummarySeriesByGroupScope(series, null);
      expect(fromNull.map((s) => s.id)).toEqual(['alpha', 'beta', 'gamma']);
      // Must be a NEW array reference (shallow copy), not the input identity.
      expect(fromNull).not.toBe(series);
      expect(fromNull).toEqual(series);

      const fromUndefined = filterSummarySeriesByGroupScope(series, undefined);
      expect(fromUndefined.map((s) => s.id)).toEqual(['alpha', 'beta', 'gamma']);
      expect(fromUndefined).not.toBe(series);
    });

    it('returns a copy of every series when the scope normalizes to null because its id is blank (b43 via invalid id)', () => {
      const series = [{ id: 'alpha' }];
      // id '' -> normalizeSummarySeriesGroupScope returns null -> b43.
      const result = filterSummarySeriesByGroupScope(series, {
        id: '',
        seriesIds: ['alpha'],
      });
      expect(result.map((s) => s.id)).toEqual(['alpha']);
      expect(result).not.toBe(series);
    });

    it('returns a copy of every series when the scope normalizes to null because its seriesIds are empty (b43 via empty list)', () => {
      const series = [{ id: 'alpha' }, { id: 'beta' }];
      // valid id but empty seriesIds -> normalize returns null -> b43.
      const result = filterSummarySeriesByGroupScope(series, {
        id: 'cluster-a',
        seriesIds: [],
      });
      expect(result.map((s) => s.id)).toEqual(['alpha', 'beta']);
      expect(result).not.toBe(series);
    });

    it('still filters down to in-scope series for a valid scope so the b43 copy path is proven not to over-include', () => {
      const series = [{ id: 'alpha' }, { id: 'beta' }, { id: 'gamma' }];
      const result = filterSummarySeriesByGroupScope(series, validScope);
      expect(result.map((s) => s.id)).toEqual(['alpha', 'beta']);
    });
  });
});
