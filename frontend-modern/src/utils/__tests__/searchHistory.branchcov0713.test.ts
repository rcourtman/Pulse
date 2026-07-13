/**
 * Branch-coverage tests for searchHistory.ts — supplemental pass.
 *
 * Scope: ONLY the uncovered branches of the exported functions
 * getSearchHistory, addSearchHistory, removeSearchHistory,
 * clearSearchHistory, and createSearchHistoryManager. The sibling
 * searchHistory.test.ts already drives the happy paths (add/remove/clear on
 * a working jsdom localStorage, malformed/non-array JSON, non-string array
 * filtering, case-insensitive dedup, and the default + custom maxEntries cap).
 *
 * This file drives the arms it leaves open:
 *
 * - isBrowser() false (SSR / no-window) early-return arm inside readHistory
 *   AND writeHistory, reached through every exported function and through the
 *   manager methods. Existing tests always run under jsdom where window and
 *   localStorage are both defined, so the `!isBrowser()` guard is never hit.
 * - writeHistory catch arm (localStorage.setItem throwing) — the existing
 *   mock never throws on setItem.
 * - readHistory catch arm entered via a throwing getItem (the sibling only
 *   enters catch via JSON.parse on malformed JSON).
 * - readHistory `!raw` guard fed an empty string from getItem (the sibling
 *   only feeds null).
 * - addSearchHistory Math.max(1, maxEntries) clamp behaviour for maxEntries
 *   <= 0 (the constant 1 wins over the supplied bound).
 * - removeSearchHistory strict (===) case-sensitive semantics, which differ
 *   from addSearchHistory's case-insensitive toLowerCase dedup.
 * - createSearchHistoryManager with an empty options object and with the
 *   manager operating in a non-browser context.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  addSearchHistory,
  clearSearchHistory,
  createSearchHistoryManager,
  getSearchHistory,
  removeSearchHistory,
} from '../searchHistory';

const KEY = 'branchcov_history';

describe('searchHistory branch coverage (supplemental)', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  describe('getSearchHistory', () => {
    it('returns [] when localStorage holds an empty string (!raw falsy edge)', () => {
      // The InMemoryStorage from test setup returns '' (not null) for a key
      // explicitly set to ''. '' is falsy, so the `!raw` guard fires before
      // JSON.parse is ever attempted.
      window.localStorage.setItem(KEY, '');
      expect(getSearchHistory(KEY)).toEqual([]);
    });

    it('returns [] when localStorage.getItem throws (readHistory catch arm via getItem)', () => {
      // The sibling test only enters the catch through JSON.parse on bad JSON;
      // this drives the same catch via a throwing getItem.
      const spy = vi.spyOn(window.localStorage, 'getItem').mockImplementation(() => {
        throw new Error('SecurityError');
      });
      expect(getSearchHistory(KEY)).toEqual([]);
      expect(spy).toHaveBeenCalledWith(KEY);
    });

    it('returns [] in a non-browser context (isBrowser false -> readHistory early return)', () => {
      // Pre-seed while window exists so the assertion proves the data was NOT
      // read (rather than merely absent).
      window.localStorage.setItem(KEY, JSON.stringify(['seeded']));
      vi.stubGlobal('window', undefined);
      expect(getSearchHistory(KEY)).toEqual([]);
    });
  });

  describe('addSearchHistory', () => {
    it('clamps maxEntries = 0 to 1 via Math.max(1, maxEntries)', () => {
      window.localStorage.setItem(KEY, JSON.stringify(['old1', 'old2']));
      // Math.max(1, 0) === 1, so only the single newest entry survives.
      const result = addSearchHistory(KEY, 'newest', 0);
      expect(result).toEqual(['newest']);
      expect(getSearchHistory(KEY)).toEqual(['newest']);
    });

    it('clamps a negative maxEntries to 1 as well (boundary value)', () => {
      window.localStorage.setItem(KEY, JSON.stringify(['a', 'b', 'c']));
      expect(addSearchHistory(KEY, 'd', -5)).toEqual(['d']);
    });

    it('returns the computed history without persisting when setItem throws (writeHistory catch)', () => {
      window.localStorage.setItem(KEY, JSON.stringify(['existing']));
      const setSpy = vi.spyOn(window.localStorage, 'setItem').mockImplementation(() => {
        throw new Error('QuotaExceeded');
      });
      // readHistory still works (getItem is not mocked); only persistence fails.
      const result = addSearchHistory(KEY, 'fresh');
      expect(result).toEqual(['fresh', 'existing']);
      expect(setSpy).toHaveBeenCalled();
      // The throw was swallowed, so the underlying store is untouched.
      expect(window.localStorage.getItem(KEY)).toBe(JSON.stringify(['existing']));
    });

    it('returns [term] without reading or persisting in a non-browser context', () => {
      // Seeded data exists, but readHistory returns [] under SSR; writeHistory
      // is a no-op. The returned array contains only the new term, proving
      // neither the seeded history nor persistence participated.
      window.localStorage.setItem(KEY, JSON.stringify(['seeded']));
      vi.stubGlobal('window', undefined);
      expect(addSearchHistory(KEY, 'term')).toEqual(['term']);
    });

    it('returns [] for a whitespace-only term in a non-browser context', () => {
      // `!trimmed` early-return feeds straight into readHistory which is itself
      // an early [] under SSR.
      vi.stubGlobal('window', undefined);
      expect(addSearchHistory(KEY, '   ')).toEqual([]);
    });
  });

  describe('removeSearchHistory', () => {
    it('is case-sensitive: a differently-cased term is NOT removed (=== keeps the entry)', () => {
      // addSearchHistory dedups case-insensitively (toLowerCase comparison),
      // but removeSearchHistory uses strict ===, so 'foo' does not match 'Foo'.
      window.localStorage.setItem(KEY, JSON.stringify(['Foo', 'bar']));
      const result = removeSearchHistory(KEY, 'foo');
      expect(result).toEqual(['Foo', 'bar']);
      expect(getSearchHistory(KEY)).toEqual(['Foo', 'bar']);
    });

    it('returns the filtered history without persisting when setItem throws', () => {
      window.localStorage.setItem(KEY, JSON.stringify(['x', 'y']));
      vi.spyOn(window.localStorage, 'setItem').mockImplementation(() => {
        throw new Error('SecurityError');
      });
      expect(removeSearchHistory(KEY, 'x')).toEqual(['y']);
    });

    it('returns [] in a non-browser context (readHistory early return, writeHistory no-op)', () => {
      // Seeded data exists; under SSR readHistory returns [] so nothing is
      // filtered out and nothing is written.
      window.localStorage.setItem(KEY, JSON.stringify(['keep']));
      vi.stubGlobal('window', undefined);
      expect(removeSearchHistory(KEY, 'keep')).toEqual([]);
    });
  });

  describe('clearSearchHistory', () => {
    it('returns [] even when setItem throws (writeHistory catch arm)', () => {
      const setSpy = vi.spyOn(window.localStorage, 'setItem').mockImplementation(() => {
        throw new Error('QuotaExceeded');
      });
      expect(clearSearchHistory(KEY)).toEqual([]);
      expect(setSpy).toHaveBeenCalled();
    });

    it('returns [] in a non-browser context without touching storage', () => {
      vi.stubGlobal('window', undefined);
      expect(clearSearchHistory(KEY)).toEqual([]);
    });
  });

  describe('createSearchHistoryManager', () => {
    it('falls back to the default cap (10) for an empty options object', () => {
      // options?.maxEntries evaluates to undefined (no short-circuit since the
      // object is present), then ?? DEFAULT_MAX_HISTORY yields 10.
      const manager = createSearchHistoryManager(KEY, {});
      for (let i = 0; i < 15; i++) {
        manager.add(`term${i}`);
      }
      expect(manager.read().length).toBe(10);
      expect(manager.read()[0]).toBe('term14');
    });

    it('honours an explicit maxEntries through manager.add (clamp at the bound)', () => {
      const manager = createSearchHistoryManager(KEY, { maxEntries: 2 });
      manager.add('first');
      manager.add('second');
      manager.add('third');
      expect(manager.read()).toEqual(['third', 'second']);
    });

    it('degrades gracefully in a non-browser context across all four methods', () => {
      // Pre-seed while window exists to prove the SSR path ignores storage.
      window.localStorage.setItem(KEY, JSON.stringify(['seeded']));
      vi.stubGlobal('window', undefined);
      const manager = createSearchHistoryManager(KEY);
      // read -> readHistory early-return -> []
      expect(manager.read()).toEqual([]);
      // add -> readHistory [], writeHistory no-op -> [term]
      expect(manager.add('x')).toEqual(['x']);
      // remove -> readHistory [], writeHistory no-op -> []
      expect(manager.remove('x')).toEqual([]);
      // clear -> writeHistory no-op -> []
      expect(manager.clear()).toEqual([]);
    });
  });
});
