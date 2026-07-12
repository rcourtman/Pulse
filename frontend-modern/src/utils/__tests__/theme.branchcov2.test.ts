/**
 * Branch-coverage tests for theme.ts — second pass.
 *
 * Targets branches of the named helpers that the sibling theme.test.ts does
 * not yet reach:
 * - normalizeThemePreference: the `value ?? null` coalescing for `undefined`,
 *   the isThemePreference truthy arm for 'dark'/'system', and the falsy
 *   fallback for empty / malformed non-string input.
 * - hasStoredThemePreference / getStoredThemePreference: the safeGet catch arm
 *   (localStorage.getItem throws), the non-canonical-stored fallback, and the
 *   SSR (`typeof window === 'undefined'`) early null-return arm.
 * - persistThemePreference: the safeSet catch arm (setItem throws, swallowed),
 *   the 'light' canonical write, and the SSR no-op arm.
 * - computeIsDark: the 'system' arm reading matchMedia for both match outcomes,
 *   the exact media-query string passed through, and the SSR early-return.
 * - applyThemeClass: classList.toggle force semantics (idempotent add/remove)
 *   and the SSR no-op arm.
 *
 * SSR arms are reached by stubbing `window` to undefined for a single assertion
 * and restoring it inside a `finally`; the module guards on
 * `typeof window === 'undefined'` before touching window/document, so the stub
 * is safe as long as it is removed before any other code runs.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { STORAGE_KEYS } from '@/utils/localStorage';
import {
  applyThemeClass,
  computeIsDark,
  getStoredThemePreference,
  hasStoredThemePreference,
  normalizeThemePreference,
  persistThemePreference,
} from '@/utils/theme';

const THEME_KEY = STORAGE_KEYS.THEME_PREFERENCE;
const DARK_MEDIA_QUERY = '(prefers-color-scheme: dark)';

describe('normalizeThemePreference (branch coverage)', () => {
  it("returns each canonical preference unchanged via the isThemePreference truthy arm", () => {
    // 'dark' and 'system' arms are not exercised by the sibling test.
    expect(normalizeThemePreference('dark')).toBe('dark');
    expect(normalizeThemePreference('system')).toBe('system');
    expect(normalizeThemePreference('light')).toBe('light');
  });

  it('coalesces undefined to null via `??` and falls back to system (falsy arm)', () => {
    // `value ?? null` evaluates the right-hand `null` for undefined input.
    expect(normalizeThemePreference(undefined)).toBe('system');
    expect(normalizeThemePreference(null)).toBe('system');
  });

  it('normalizes an empty string to system (falsy isThemePreference arm)', () => {
    expect(normalizeThemePreference('')).toBe('system');
  });

  it('falls back to system for a malformed non-string value cast through the param type', () => {
    const malformed = 42 as unknown as Parameters<typeof normalizeThemePreference>[0];
    expect(normalizeThemePreference(malformed)).toBe('system');
  });
});

describe('hasStoredThemePreference (branch coverage)', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it('returns true for every canonical value stored under the canonical key', () => {
    window.localStorage.setItem(THEME_KEY, 'light');
    expect(hasStoredThemePreference()).toBe(true);

    window.localStorage.setItem(THEME_KEY, 'system');
    expect(hasStoredThemePreference()).toBe(true);
  });

  it('returns false when the stored value is not a recognized preference', () => {
    window.localStorage.setItem(THEME_KEY, 'banana');
    expect(hasStoredThemePreference()).toBe(false);
  });

  it('returns false (swallowing the throw) when localStorage.getItem throws', () => {
    // safeGet catch arm.
    const spy = vi.spyOn(window.localStorage, 'getItem').mockImplementation(() => {
      throw new Error('SecurityError');
    });
    expect(hasStoredThemePreference()).toBe(false);
    spy.mockRestore();
  });

  it('returns false in an SSR context where window is undefined (safeGet early null arm)', () => {
    vi.stubGlobal('window', undefined);
    try {
      expect(hasStoredThemePreference()).toBe(false);
    } finally {
      vi.unstubAllGlobals();
    }
  });
});

describe('getStoredThemePreference (branch coverage)', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it('returns dark and system verbatim when stored (truthy isThemePreference arm)', () => {
    window.localStorage.setItem(THEME_KEY, 'dark');
    expect(getStoredThemePreference()).toBe('dark');

    window.localStorage.setItem(THEME_KEY, 'system');
    expect(getStoredThemePreference()).toBe('system');
  });

  it('falls back to system when nothing is stored (explicitPreference === null)', () => {
    expect(getStoredThemePreference()).toBe('system');
  });

  it('falls back to system for a non-canonical stored value (falsy arm)', () => {
    window.localStorage.setItem(THEME_KEY, 'garbage');
    expect(getStoredThemePreference()).toBe('system');
  });

  it('falls back to system when localStorage.getItem throws (safeGet catch arm)', () => {
    const spy = vi.spyOn(window.localStorage, 'getItem').mockImplementation(() => {
      throw new Error('SecurityError');
    });
    expect(getStoredThemePreference()).toBe('system');
    spy.mockRestore();
  });

  it('returns system in an SSR context where window is undefined (safeGet early null arm)', () => {
    vi.stubGlobal('window', undefined);
    try {
      expect(getStoredThemePreference()).toBe('system');
    } finally {
      vi.unstubAllGlobals();
    }
  });
});

describe('persistThemePreference (branch coverage)', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it('writes the light preference verbatim under the canonical key', () => {
    persistThemePreference('light');
    expect(window.localStorage.getItem(THEME_KEY)).toBe('light');
  });

  it('overwrites a previous value with the latest preference', () => {
    persistThemePreference('dark');
    persistThemePreference('light');
    expect(window.localStorage.getItem(THEME_KEY)).toBe('light');
  });

  it('swallows the error and does not throw when localStorage.setItem throws (safeSet catch arm)', () => {
    const spy = vi.spyOn(window.localStorage, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceededError');
    });
    expect(() => persistThemePreference('dark')).not.toThrow();
    // The throw short-circuited the write, so nothing was persisted.
    expect(window.localStorage.getItem(THEME_KEY)).toBeNull();
    spy.mockRestore();
  });

  it('is a no-op in an SSR context where window is undefined (safeSet early-return arm)', () => {
    vi.stubGlobal('window', undefined);
    try {
      expect(() => persistThemePreference('dark')).not.toThrow();
    } finally {
      vi.unstubAllGlobals();
    }
    // No write occurred because the SSR guard returned before touching storage.
    expect(window.localStorage.getItem(THEME_KEY)).toBeNull();
  });
});

describe('computeIsDark (branch coverage)', () => {
  const originalMatchMediaDescriptor = Object.getOwnPropertyDescriptor(window, 'matchMedia');

  afterEach(() => {
    if (originalMatchMediaDescriptor) {
      Object.defineProperty(window, 'matchMedia', originalMatchMediaDescriptor);
    }
  });

  it('queries the exact prefers-color-scheme query and returns false when the OS reports light', () => {
    const matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      configurable: true,
      value: matchMedia,
    });

    expect(computeIsDark('system')).toBe(false);
    expect(matchMedia).toHaveBeenCalledTimes(1);
    expect(matchMedia).toHaveBeenCalledWith(DARK_MEDIA_QUERY);
  });

  it('returns true for system when the OS reports a dark scheme (matchMedia matches arm)', () => {
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      configurable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: true,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });

    expect(computeIsDark('system')).toBe(true);
  });

  it('short-circuits to true/false for dark/light without consulting matchMedia', () => {
    const matchMedia = vi.fn();
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      configurable: true,
      value: matchMedia,
    });

    expect(computeIsDark('dark')).toBe(true);
    expect(computeIsDark('light')).toBe(false);
    expect(matchMedia).not.toHaveBeenCalled();
  });

  it('returns false for system in an SSR context (typeof window === "undefined" arm)', () => {
    vi.stubGlobal('window', undefined);
    try {
      expect(computeIsDark('system')).toBe(false);
    } finally {
      vi.unstubAllGlobals();
    }
  });
});

describe('applyThemeClass (branch coverage)', () => {
  beforeEach(() => {
    document.documentElement.classList.remove('dark');
  });

  afterEach(() => {
    document.documentElement.classList.remove('dark');
  });

  it('force-adds and force-removes the dark class (classList.toggle second-arg semantics)', () => {
    expect(document.documentElement.classList.contains('dark')).toBe(false);

    applyThemeClass(true);
    expect(document.documentElement.classList.contains('dark')).toBe(true);

    // Idempotent: force=true keeps the class even when already present.
    applyThemeClass(true);
    expect(document.documentElement.classList.contains('dark')).toBe(true);

    applyThemeClass(false);
    expect(document.documentElement.classList.contains('dark')).toBe(false);

    // Idempotent: force=false keeps it absent.
    applyThemeClass(false);
    expect(document.documentElement.classList.contains('dark')).toBe(false);
  });

  it('is a no-op in an SSR context where window is undefined (early-return arm)', () => {
    applyThemeClass(true); // establish a known starting state in jsdom.
    expect(document.documentElement.classList.contains('dark')).toBe(true);

    vi.stubGlobal('window', undefined);
    try {
      expect(() => applyThemeClass(false)).not.toThrow();
    } finally {
      vi.unstubAllGlobals();
    }
    // SSR guard returned before touching document, so the class is unchanged.
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });
});
