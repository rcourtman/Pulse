/**
 * Branch-coverage tests for resourceTypePresentation.ts (0724pm batch).
 *
 * Measured baseline (existing resourceTypePresentation.test.ts only):
 *   functions: 4/4 covered
 *   branches : 6/10 arms covered  -> 4 cold
 *
 * Cold arms found by scoped v8 coverage (coverage-final.json):
 *   branch#2  L240:40-72  `canonicalResourceTypeForDisplay`
 *                          `canonicalizeFrontendResourceType(value) || value.trim().toLowerCase()`
 *                          -> the `value.trim().toLowerCase()` fallback, taken when the
 *                             input is a non-empty string that is NOT a known canonical alias.
 *   branch#4  L245:21-33  `getResourceTypePresentation`
 *                          `if (!resourceType) return null;`
 *                          -> the `return null` arm (falsy / empty / undefined resourceType).
 *   branch#7  L251:27-256  `getResourceTypePresentation`
 *                          `if (presentation) return presentation;` fall-through to the
 *                          default-badge object returned for an unknown canonical type.
 *   branch#9  L259:45-59  `getResourceTypeLabel`
 *                          `getResourceTypePresentation(resourceType)?.label || null`
 *                          -> the `|| null` arm, taken when no presentation resolves.
 *
 * This file exercises each cold arm with real observable values. It does NOT
 * modify the source module or any existing test file.
 *
 * Suspected SOURCE bug (reported, NOT fixed): canonicalResourceTypeForDisplay
 * (line 239) is typed `(value: string)` and falls back to `value.trim().toLowerCase()`.
 * Its callee canonicalizeFrontendResourceType accepts `unknown` and gracefully
 * returns undefined for non-strings, but this wrapper calls `.trim()` on the
 * raw value and throws TypeError if a non-string reaches it at runtime
 * (see the "malformed non-string input" test below).
 */
import { describe, expect, it } from 'vitest';

import {
  canonicalResourceTypeForDisplay,
  getResourceTypeLabel,
  getResourceTypePresentation,
} from '@/utils/resourceTypePresentation';

const DEFAULT_BADGE_CLASSES = 'bg-surface-alt text-base-content';

/* ------------------------------------------------------------------ *
 * branch#2 - canonicalResourceTypeForDisplay fallback arm
 *   canonicalizeFrontendResourceType(value) || value.trim().toLowerCase()
 *
 * The fallback fires for any non-empty string that is NOT one of the
 * canonical aliases defined in resourceTypeCompat.ts.
 * ------------------------------------------------------------------ */

describe('canonicalResourceTypeForDisplay - unknown string falls back to trimmed lowercase form', () => {
  it('returns the trimmed, lowercased value when canonicalization has no mapping', () => {
    expect(canonicalResourceTypeForDisplay('totally-unknown-xyz')).toBe('totally-unknown-xyz');
  });
});

/* ------------------------------------------------------------------ *
 * branch#2 + branch#7 - getResourceTypePresentation unknown-type default
 *
 *   externalPresentation undefined  (line 247-248, already hot)
 *   -> canonicalType from canonicalResourceTypeForDisplay fallback (branch#2)
 *   -> presentation undefined (line 250-251)
 *   -> fall through to the default badge object (branch#7)
 * ------------------------------------------------------------------ */

describe('getResourceTypePresentation - unknown type resolves to a default badge', () => {
  it('uses the canonicalized type as the label with the default badge classes', () => {
    const presentation = getResourceTypePresentation('totally-unknown-xyz');
    expect(presentation).toEqual({
      label: 'totally-unknown-xyz',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    });
  });

  it('returns a distinct object per call (default is built inline, not cached)', () => {
    const a = getResourceTypePresentation('unknown-a');
    const b = getResourceTypePresentation('unknown-b');
    expect(a).not.toBe(b);
    expect(a?.label).toBe('unknown-a');
    expect(b?.label).toBe('unknown-b');
  });
});

/* ------------------------------------------------------------------ *
 * branch#4 - getResourceTypePresentation falsy resourceType arm
 *   if (!resourceType) return null;
 * ------------------------------------------------------------------ */

describe('getResourceTypePresentation - falsy resourceType returns null', () => {
  it('returns null for an empty string', () => {
    expect(getResourceTypePresentation('')).toBeNull();
  });
});

/* ------------------------------------------------------------------ *
 * branch#9 - getResourceTypeLabel null-when-unresolved arm
 *   getResourceTypePresentation(resourceType)?.label || null
 *
 * Fires when getResourceTypePresentation itself returns null (falsy input),
 * leaving `?.label` undefined and the `|| null` arm to provide the value.
 * ------------------------------------------------------------------ */

describe('getResourceTypeLabel - returns null when no presentation resolves', () => {
  it('returns null for an empty string', () => {
    expect(getResourceTypeLabel('')).toBeNull();
  });
});

/* ------------------------------------------------------------------ *
 * Malformed input (deliberately mistyped) - documents suspected source bug.
 *
 * canonicalResourceTypeForDisplay is typed (value: string) but its callee
 * accepts unknown. A non-string passed at runtime reaches the fallback
 * `value.trim()` and throws TypeError instead of degrading gracefully.
 * Reported in GLM_REPORT; NOT fixed here.
 * ------------------------------------------------------------------ */

describe('canonicalResourceTypeForDisplay - malformed non-string input', () => {
  it('throws TypeError because the fallback calls .trim() on the raw value', () => {
    expect(() => canonicalResourceTypeForDisplay(123 as unknown as string)).toThrow(TypeError);
    expect(() => canonicalResourceTypeForDisplay(123 as unknown as string)).toThrow(
      'value.trim is not a function',
    );
  });
});
