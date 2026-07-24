import { describe, expect, it } from 'vitest';
import {
  getWebInterfaceSuggestedUrlFallback,
  shouldShowWebInterfaceSuggestedDiagnostic,
  shouldShowWebInterfaceSuggestedUrl,
} from '@/components/shared/webInterfaceUrlFieldModel';

// Branch-coverage suite for the three still-zero-hit pure helpers in
// webInterfaceUrlFieldModel.ts (the 0712 suite covers the validation and
// target-label helpers; these three had no direct callers in specs).
//
// Every assertion is a concrete equality check against the real return shape /
// primitive so it pins the exact arm taken. Where a falsy/whitespace input is
// passed the cast is through `unknown` to satisfy strict null checks while
// still exercising the real runtime coercion inside the source.

describe('webInterfaceUrlFieldModel.branchcov0724pm', () => {
  describe('shouldShowWebInterfaceSuggestedDiagnostic', () => {
    // Source:
    //   return (
    //     !options.discoveryLoading &&
    //     !normalizeWebInterfaceUrl(options.currentUrl) &&
    //     !normalizeWebInterfaceUrl(options.suggestedUrl) &&
    //     Boolean(options.suggestedUrlDiagnostic)
    //   );
    // The diagnostic banner is the "we have nothing to suggest but we can tell
    // the user why" state, so it only shows once discovery has settled, the
    // user has not typed anything, and we did not find a URL — i.e. all four
    // guards must be true.

    it('returns true when every guard passes (not loading, empty current, empty suggestion, diagnostic present)', () => {
      expect(
        shouldShowWebInterfaceSuggestedDiagnostic({
          discoveryLoading: false,
          currentUrl: '',
          suggestedUrl: '',
          suggestedUrlDiagnostic: 'No web port detected',
        }),
      ).toBe(true);
    });

    it('treats omitted discoveryLoading as not-loading (`!undefined` truthy arm) and still shows the diagnostic', () => {
      // discoveryLoading is optional; omitting it must not suppress the banner.
      expect(
        shouldShowWebInterfaceSuggestedDiagnostic({
          currentUrl: undefined,
          suggestedUrl: undefined,
          suggestedUrlDiagnostic: 'No web port detected',
        }),
      ).toBe(true);
    });

    it('short-circuits to false while discovery is still loading (first guard false)', () => {
      // Even with a diagnostic and no URL, we stay quiet until loading finishes.
      expect(
        shouldShowWebInterfaceSuggestedDiagnostic({
          discoveryLoading: true,
          currentUrl: '',
          suggestedUrl: '',
          suggestedUrlDiagnostic: 'No web port detected',
        }),
      ).toBe(false);
    });

    it('returns false once the user has typed a currentUrl (second guard false)', () => {
      expect(
        shouldShowWebInterfaceSuggestedDiagnostic({
          discoveryLoading: false,
          currentUrl: 'https://198.51.100.100:8006',
          suggestedUrl: '',
          suggestedUrlDiagnostic: 'No web port detected',
        }),
      ).toBe(false);
    });

    it('treats a whitespace-only currentUrl as empty and still proceeds past the second guard', () => {
      // normalizeWebInterfaceUrl trims, so '   ' === '' for this check — the
      // banner can still show even though the raw field contains spaces.
      expect(
        shouldShowWebInterfaceSuggestedDiagnostic({
          discoveryLoading: false,
          currentUrl: '   ',
          suggestedUrl: '',
          suggestedUrlDiagnostic: 'No web port detected',
        }),
      ).toBe(true);
    });

    it('returns false when a suggestedUrl exists (third guard false) — the suggestion card wins over the diagnostic', () => {
      expect(
        shouldShowWebInterfaceSuggestedDiagnostic({
          discoveryLoading: false,
          currentUrl: '',
          suggestedUrl: 'https://198.51.100.100:8006',
          suggestedUrlDiagnostic: 'No web port detected',
        }),
      ).toBe(false);
    });

    it('treats a whitespace-only suggestedUrl as empty and still proceeds past the third guard', () => {
      expect(
        shouldShowWebInterfaceSuggestedDiagnostic({
          discoveryLoading: false,
          currentUrl: '',
          suggestedUrl: '\t',
          suggestedUrlDiagnostic: 'No web port detected',
        }),
      ).toBe(true);
    });

    it('returns false when the diagnostic is the empty string (fourth guard false via Boolean(""))', () => {
      expect(
        shouldShowWebInterfaceSuggestedDiagnostic({
          discoveryLoading: false,
          currentUrl: '',
          suggestedUrl: '',
          suggestedUrlDiagnostic: '',
        }),
      ).toBe(false);
    });

    it('treats a whitespace-only diagnostic as present and returns true — diagnostic is NOT trimmed (Boolean(" ") is truthy)', () => {
      // Observable divergence from currentUrl/suggestedUrl: those are run
      // through normalizeWebInterfaceUrl (which trims) before the truthiness
      // check, but the diagnostic is fed straight to Boolean(). A whitespace
      // diagnostic therefore still shows. Pinned here so a future refactor
      // that "normalizes" the diagnostic is forced to reconsider the UX.
      expect(
        shouldShowWebInterfaceSuggestedDiagnostic({
          discoveryLoading: false,
          currentUrl: '',
          suggestedUrl: '',
          suggestedUrlDiagnostic: '   ',
        }),
      ).toBe(true);
    });
  });

  describe('shouldShowWebInterfaceSuggestedUrl', () => {
    // Source:
    //   const suggested = normalizeWebInterfaceUrl(options.suggestedUrl);
    //   if (classifyWebInterfaceUrl(suggested).status !== 'valid') return false;
    //   return suggested !== normalizeWebInterfaceUrl(options.currentUrl);

    it('returns false when there is no suggestion (classify "missing" — early-return arm)', () => {
      expect(
        shouldShowWebInterfaceSuggestedUrl({
          currentUrl: '',
          suggestedUrl: '',
        }),
      ).toBe(false);
    });

    it('returns false when the suggestion does not parse as a URL (classify "invalid" — catch arm feeds the early return)', () => {
      expect(
        shouldShowWebInterfaceSuggestedUrl({
          currentUrl: '',
          suggestedUrl: 'not-a-url',
        }),
      ).toBe(false);
    });

    it('returns false when the suggestion uses a non-http(s) scheme (classify "invalid" via protocol guard)', () => {
      expect(
        shouldShowWebInterfaceSuggestedUrl({
          currentUrl: '',
          suggestedUrl: 'ftp://example.com',
        }),
      ).toBe(false);
    });

    it('returns true when the suggestion is valid and the user has entered nothing', () => {
      expect(
        shouldShowWebInterfaceSuggestedUrl({
          currentUrl: '',
          suggestedUrl: 'https://198.51.100.100:8006',
        }),
      ).toBe(true);
    });

    it('returns true when the suggestion is valid and differs from the user-entered URL', () => {
      expect(
        shouldShowWebInterfaceSuggestedUrl({
          currentUrl: 'https://10.0.0.5:8006',
          suggestedUrl: 'https://198.51.100.100:8006',
        }),
      ).toBe(true);
    });

    it('returns false when the user has already typed exactly the suggested URL (the !== tie arm)', () => {
      // The suggestion card would be redundant noise here, so it is hidden.
      expect(
        shouldShowWebInterfaceSuggestedUrl({
          currentUrl: 'https://198.51.100.100:8006',
          suggestedUrl: 'https://198.51.100.100:8006',
        }),
      ).toBe(false);
    });

    it('returns false when the current URL matches the suggestion after trimming (normalization boundary on the tie)', () => {
      // Both sides are normalized before the comparison, so leading/trailing
      // whitespace cannot fool the tie detector.
      expect(
        shouldShowWebInterfaceSuggestedUrl({
          currentUrl: '  https://198.51.100.100:8006  ',
          suggestedUrl: 'https://198.51.100.100:8006',
        }),
      ).toBe(false);
    });

    it('normalizes the suggestion before classifying, so a padded valid suggestion still counts as valid', () => {
      // '  https://...  ' → 'https://...' which is valid; then differs from
      // the empty current URL, so the card shows.
      expect(
        shouldShowWebInterfaceSuggestedUrl({
          currentUrl: '',
          suggestedUrl: '  https://198.51.100.100:8006  ',
        }),
      ).toBe(true);
    });
  });

  describe('getWebInterfaceSuggestedUrlFallback', () => {
    // Source is a thin delegate to getDiscoverySuggestedURLFallback, which
    // returns `{ title: 'No suggested URL available', description: diagnostic || '' }`.
    // We assert the concrete returned object (literal title + echoed/fallback
    // description) — never that the delegate equals its delegate.

    it('returns the fixed title with the diagnostic echoed into description when a diagnostic is provided', () => {
      expect(getWebInterfaceSuggestedUrlFallback('No web port detected')).toStrictEqual({
        title: 'No suggested URL available',
        description: 'No web port detected',
      });
    });

    it('falls back to an empty description when the diagnostic is undefined (`diagnostic || ""` falsy arm)', () => {
      expect(getWebInterfaceSuggestedUrlFallback()).toStrictEqual({
        title: 'No suggested URL available',
        description: '',
      });
    });

    it('echoes a diagnostic that contains only whitespace rather than treating it as missing', () => {
      // `diagnostic || ''` does not trim, so a whitespace diagnostic survives
      // into description. Pinned to surface the asymmetry with the truthiness
      // handling used by the sibling helpers.
      expect(getWebInterfaceSuggestedUrlFallback('   ').description).toBe('   ');
    });
  });
});
