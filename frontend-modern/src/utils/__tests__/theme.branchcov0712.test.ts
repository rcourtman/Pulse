/**
 * Branch-coverage tests for theme.ts — second pass.
 *
 * Scope: ONLY `getStoredThemePreference`. The sibling theme.test.ts exercises
 * the happy path for this function; this file drives every remaining branch:
 *
 * - The `isThemePreference(explicitPreference)` truthy arm, for each of the
 *   three canonical values it accepts ('light', 'dark', 'system').
 * - The falsy arm that falls back to 'system', reached through every path that
 *   `safeGet` + `isThemePreference` can produce a non-matching value:
 *     - nothing stored (null),
 *     - a non-canonical stored string,
 *     - an empty string,
 *     - `localStorage.getItem` throwing (safeGet catch arm),
 *     - an SSR context where `typeof window === 'undefined'` (safeGet early
 *       null-return arm).
 *
 * SSR arms are reached by stubbing `window` to undefined for a single
 * assertion and restoring it inside a `finally`; the module guards on
 * `typeof window === 'undefined'` before touching window, so the stub is safe
 * as long as it is removed before any other code runs.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { STORAGE_KEYS } from '@/utils/localStorage';
import { getStoredThemePreference } from '@/utils/theme';

const THEME_KEY = STORAGE_KEYS.THEME_PREFERENCE;

describe('getStoredThemePreference (branch coverage)', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  describe('isThemePreference truthy arm — returns each canonical value verbatim', () => {
    it("returns 'light' when 'light' is stored", () => {
      window.localStorage.setItem(THEME_KEY, 'light');
      expect(getStoredThemePreference()).toBe('light');
    });

    it("returns 'dark' when 'dark' is stored", () => {
      window.localStorage.setItem(THEME_KEY, 'dark');
      expect(getStoredThemePreference()).toBe('dark');
    });

    it("returns 'system' when 'system' is stored (does NOT collapse to the fallback)", () => {
      // Distinguishes the truthy 'system' arm from the falsy 'system' fallback:
      // both yield 'system', but only the truthy arm reads the stored value.
      window.localStorage.setItem(THEME_KEY, 'system');
      expect(getStoredThemePreference()).toBe('system');
      expect(window.localStorage.getItem(THEME_KEY)).toBe('system');
    });
  });

  describe('falsy arm — falls back to system for every non-matching stored value', () => {
    it("falls back to 'system' when nothing is stored (explicitPreference === null)", () => {
      expect(window.localStorage.getItem(THEME_KEY)).toBeNull();
      expect(getStoredThemePreference()).toBe('system');
    });

    it("falls back to 'system' for a non-canonical stored string", () => {
      window.localStorage.setItem(THEME_KEY, 'banana');
      expect(getStoredThemePreference()).toBe('system');
    });

    it("falls back to 'system' for an empty string (isThemePreference rejects '')", () => {
      window.localStorage.setItem(THEME_KEY, '');
      expect(getStoredThemePreference()).toBe('system');
    });

    it("falls back to 'system' when localStorage.getItem throws (safeGet catch arm)", () => {
      const spy = vi.spyOn(window.localStorage, 'getItem').mockImplementation(() => {
        throw new Error('SecurityError');
      });
      expect(getStoredThemePreference()).toBe('system');
      expect(spy).toHaveBeenCalledWith(THEME_KEY);
    });

    it("returns 'system' in an SSR context where window is undefined (safeGet early null arm)", () => {
      vi.stubGlobal('window', undefined);
      expect(getStoredThemePreference()).toBe('system');
    });
  });
});
