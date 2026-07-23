import { describe, expect, it } from 'vitest';
import type { ZFSPool } from '@/types/api';
import {
  getStorageBarLabel,
  getStorageBarUsagePercent,
  getStorageBarZfsSummary,
} from '@/features/storageBackups/storageBarPresentation';

// ---------------------------------------------------------------------------
// Fixture builder — mirrors the convention in storageBarPresentation.test.ts
// (which casts a partial with `as any`) but stays fully typed so the pre-push
// tsc stays happy. Only the fields getStorageBarZfsSummary reads need to be
// overridden; everything else defaults to a clean ONLINE pool with no errors.
// ---------------------------------------------------------------------------

const makeZfsPool = (overrides: Partial<ZFSPool> = {}): ZFSPool =>
  ({
    name: 'tank',
    state: 'ONLINE',
    status: 'Healthy',
    scan: 'none',
    readErrors: 0,
    writeErrors: 0,
    checksumErrors: 0,
    devices: [],
    ...overrides,
  }) as ZFSPool;

// ===========================================================================
// Branch map for storageBarPresentation.ts
//
//   getStorageBarUsagePercent (L36-39)
//     L37 `if (total <= 0) return 0`
//         true  arm : total<=0  -> 0           [UNCOVERED — covered here]
//         false arm : total>0                 (covered by happy path)
//
//   getStorageBarZfsSummary (L61-77)
//     L62 `if (!zfsPool) return null`
//         true  arm : zfsPool missing/null    [UNCOVERED — covered here]
//         false arm                          (covered by happy path)
//     L64 `zfsPool.scan || ''`
//         falsy arm : scan undefined/''       [UNCOVERED — covered here]
//         truthy arm                         (covered by happy path)
//     L65 `readErrors>0 || writeErrors>0 || checksumErrors>0`
//         readErrors>0 short-circuit only     [UNCOVERED — covered here]
//         writeErrors>0 arm                   [UNCOVERED — covered here]
//         checksumErrors>0 arm                [UNCOVERED — covered here]
//         all-false arm                       [UNCOVERED — covered here]
//     L69 `scan.toLowerCase().includes('scrub')`
//         true  arm                           [UNCOVERED — covered here]
//         false arm                          (covered by happy path: 'resilver…')
//     L70 `scan.toLowerCase().includes('resilver')`
//         true  arm                          (covered by happy path)
//         false arm                           [UNCOVERED — covered here]
//     L72 `scan && scan !== 'none' ? scan : ''`
//         false arm : scan==='none'           [UNCOVERED — covered here]
//         true  arm                          (covered by happy path)
//     L73 `hasErrors ? 'Errors:…' : ''`
//         false arm                           [UNCOVERED — covered here]
//         true  arm                          (covered by happy path)
// ===========================================================================

describe('storageBarPresentation branch coverage', () => {
  // =========================================================================
  // getStorageBarUsagePercent — L37 `if (total <= 0) return 0` true arm
  // =========================================================================
  describe('getStorageBarUsagePercent — total<=0 guard (L37 true arm)', () => {
    it('returns exactly 0 when total is 0 (no NaN, no Infinity)', () => {
      expect(getStorageBarUsagePercent(40, 0)).toBe(0);
    });

    it('returns exactly 0 when total is negative (guard is <=, not <)', () => {
      // Locks the boundary: -100 is strictly < 0, proving the guard matches
      // the `<= 0` predicate rather than `=== 0`.
      expect(getStorageBarUsagePercent(40, -100)).toBe(0);
    });

    it('still computes a real ratio for total === 1 (proves guard is total<=0, not total<1)', () => {
      // 0.5 of 1 byte = 50% — sanity check that small positive totals are NOT
      // collapsed to 0 by some other implicit clamp.
      expect(getStorageBarUsagePercent(0.5, 1)).toBe(50);
    });
  });

  describe('getStorageBarUsagePercent — boundary ratios', () => {
    it('returns exactly 0 when used is 0 (zero percent usage)', () => {
      expect(getStorageBarUsagePercent(0, 100)).toBe(0);
    });

    it('returns a value over 100 when used exceeds total (over-capacity)', () => {
      expect(getStorageBarUsagePercent(150, 100)).toBe(150);
    });

    it('returns a negative ratio when used is negative (no clamp on `used`)', () => {
      // Documents current behaviour: only `total` is guarded; `used` flows
      // straight into the division, so a malformed negative `used` surfaces
      // as a negative percentage rather than 0.
      expect(getStorageBarUsagePercent(-10, 100)).toBe(-10);
    });
  });

  // =========================================================================
  // getStorageBarLabel — composes the percent guard + formatters; pin the
  // observable string for the boundary cases so the format is locked.
  // =========================================================================
  describe('getStorageBarLabel — observable label for guard + over-100 cases', () => {
    it('produces "0% (used/0 B)" when total is 0 (percent short-circuited to 0)', () => {
      // percent(40,0)=0 -> formatPercent(0)='0%'; formatBytes(40)='40.0 B'; formatBytes(0)='0 B'.
      expect(getStorageBarLabel(40, 0)).toBe('0% (40.0 B/0 B)');
    });

    it('produces a >100% label when used exceeds total (formatPercent has no clamp either)', () => {
      // percent(150,100)=150 -> '150%'; formatBytes(150)='150 B'; formatBytes(100)='100 B'.
      expect(getStorageBarLabel(150, 100)).toBe('150% (150 B/100 B)');
    });
  });

  // =========================================================================
  // getStorageBarZfsSummary — L62 `if (!zfsPool) return null` true arm
  // =========================================================================
  describe('getStorageBarZfsSummary — missing pool (L62 true arm)', () => {
    it('returns null when called with no argument', () => {
      expect(getStorageBarZfsSummary()).toBeNull();
    });

    it('returns null when called with explicit undefined', () => {
      expect(getStorageBarZfsSummary(undefined)).toBeNull();
    });

    it('returns null when called with null cast through unknown (defensive null guard)', () => {
      // ZFSPool is typed as a required interface, so null must be injected
      // via `as unknown as ZFSPool` to bypass strict null checks at the call
      // site without weakening the source signature.
      expect(getStorageBarZfsSummary(null as unknown as ZFSPool)).toBeNull();
    });
  });

  // =========================================================================
  // getStorageBarZfsSummary — L64 `zfsPool.scan || ''` falsy arm
  // =========================================================================
  describe('getStorageBarZfsSummary — scan falsy fallback (L64 falsy arm)', () => {
    it('substitutes empty string when scan is undefined (observable via output.scan / substring flags)', () => {
      // L64 falsy -> scan becomes '' -> L72 ternary false -> output.scan = ''.
      // Also proves the downstream substring checks run against '', so both
      // isScrubbing and isResilvering come back false.
      const summary = getStorageBarZfsSummary(
        makeZfsPool({ scan: undefined } as unknown as ZFSPool),
      );
      expect(summary).not.toBeNull();
      expect(summary!.scan).toBe('');
      expect(summary!.isScrubbing).toBe(false);
      expect(summary!.isResilvering).toBe(false);
    });

    it('substitutes empty string when scan is the empty string', () => {
      const summary = getStorageBarZfsSummary(makeZfsPool({ scan: '' }));
      expect(summary!.scan).toBe('');
    });
  });

  // =========================================================================
  // getStorageBarZfsSummary — L65 OR chain (3 arms + all-false arm)
  // =========================================================================
  describe('getStorageBarZfsSummary — error-count OR chain (L65)', () => {
    it('flags hasErrors=true via readErrors only (first OR arm, short-circuit)', () => {
      const summary = getStorageBarZfsSummary(
        makeZfsPool({ readErrors: 1, writeErrors: 0, checksumErrors: 0 }),
      );
      expect(summary!.hasErrors).toBe(true);
      expect(summary!.errorSummary).toBe('Errors: R:1 W:0 C:0');
    });

    it('flags hasErrors=true via writeErrors only (second OR arm)', () => {
      const summary = getStorageBarZfsSummary(
        makeZfsPool({ readErrors: 0, writeErrors: 7, checksumErrors: 0 }),
      );
      expect(summary!.hasErrors).toBe(true);
      expect(summary!.errorSummary).toBe('Errors: R:0 W:7 C:0');
    });

    it('flags hasErrors=true via checksumErrors only (third OR arm)', () => {
      const summary = getStorageBarZfsSummary(
        makeZfsPool({ readErrors: 0, writeErrors: 0, checksumErrors: 4 }),
      );
      expect(summary!.hasErrors).toBe(true);
      expect(summary!.errorSummary).toBe('Errors: R:0 W:0 C:4');
    });

    it('hasErrors=false when all error counts are 0 (OR chain all-false arm + L73 false arm)', () => {
      const summary = getStorageBarZfsSummary(
        makeZfsPool({ readErrors: 0, writeErrors: 0, checksumErrors: 0 }),
      );
      expect(summary!.hasErrors).toBe(false);
      // L73 false arm: errorSummary collapses to ''.
      expect(summary!.errorSummary).toBe('');
    });

    it('treats negative error counts as no-error (each `> 0` is false)', () => {
      // Documents current behaviour: the guards are strictly `> 0`, so a
      // malformed negative count does NOT flip hasErrors. Exercises the false
      // arm of every comparison in the OR chain simultaneously.
      const summary = getStorageBarZfsSummary(
        makeZfsPool({ readErrors: -1, writeErrors: -2, checksumErrors: -3 }),
      );
      expect(summary!.hasErrors).toBe(false);
      expect(summary!.errorSummary).toBe('');
    });
  });

  // =========================================================================
  // getStorageBarZfsSummary — L69/L70 scan substring arms
  // =========================================================================
  describe('getStorageBarZfsSummary — scan substring arms (L69/L70)', () => {
    it('detects scrub when scan contains "scrub" (L69 true arm) and not resilver (L70 false arm)', () => {
      const summary = getStorageBarZfsSummary(makeZfsPool({ scan: 'scrub in progress' }));
      expect(summary!.isScrubbing).toBe(true);
      expect(summary!.isResilvering).toBe(false);
    });

    it('detects resilver when scan contains "resilver" (L70 true arm) and not scrub (L69 false arm)', () => {
      const summary = getStorageBarZfsSummary(makeZfsPool({ scan: 'resilver completed' }));
      expect(summary!.isResilvering).toBe(true);
      expect(summary!.isScrubbing).toBe(false);
    });

    it('detects both substrings when scan mentions scrub and resilver', () => {
      const summary = getStorageBarZfsSummary(makeZfsPool({ scan: 'scrub+resilver running' }));
      expect(summary!.isScrubbing).toBe(true);
      expect(summary!.isResilvering).toBe(true);
    });

    it('matches neither substring for an unrelated scan (both false arms)', () => {
      const summary = getStorageBarZfsSummary(makeZfsPool({ scan: 'idle' }));
      expect(summary!.isScrubbing).toBe(false);
      expect(summary!.isResilvering).toBe(false);
    });

    it('matches scrub case-insensitively (proves .toLowerCase() runs on the scan value)', () => {
      // 'SCRUB PENDING' -> lowercased 'scrub pending' -> includes('scrub') true.
      const summary = getStorageBarZfsSummary(makeZfsPool({ scan: 'SCRUB PENDING' }));
      expect(summary!.isScrubbing).toBe(true);
    });
  });

  // =========================================================================
  // getStorageBarZfsSummary — L72 `scan && scan !== 'none' ? scan : ''`
  // =========================================================================
  describe('getStorageBarZfsSummary — scan output ternary (L72)', () => {
    it('collapses scan === "none" to empty string (L72 false arm via equality)', () => {
      // 'none' is truthy, so the first half of the && is true, but the
      // second half (`scan !== 'none'`) is false -> whole && is false -> ''.
      const summary = getStorageBarZfsSummary(makeZfsPool({ scan: 'none' }));
      expect(summary!.scan).toBe('');
    });

    it('passes through a non-"none" scan value verbatim (L72 true arm)', () => {
      const summary = getStorageBarZfsSummary(makeZfsPool({ scan: 'scrub repaired 0 errors' }));
      expect(summary!.scan).toBe('scrub repaired 0 errors');
    });
  });

  // =========================================================================
  // Full-shape regression for the clean-pool case — exercises every "false"
  // arm of getStorageBarZfsSummary in one assertion so the canonical object
  // shape is locked alongside the per-arm tests above.
  // =========================================================================
  describe('getStorageBarZfsSummary — clean-pool canonical shape', () => {
    it('returns the canonical clean-pool summary object', () => {
      // Arms exercised simultaneously:
      //   L62 false (pool present)
      //   L64 truthy (scan='none' is truthy)
      //   L65 all-false (all counts 0)
      //   L69 false, L70 false ('none' contains neither substring)
      //   L72 false (scan==='none')
      //   L73 false (hasErrors false)
      expect(
        getStorageBarZfsSummary(
          makeZfsPool({
            state: 'ONLINE',
            scan: 'none',
            readErrors: 0,
            writeErrors: 0,
            checksumErrors: 0,
          }),
        ),
      ).toEqual({
        hasErrors: false,
        isScrubbing: false,
        isResilvering: false,
        state: 'ONLINE',
        scan: '',
        errorSummary: '',
      });
    });
  });
});
