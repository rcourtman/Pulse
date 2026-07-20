/**
 * Branch-coverage tests for systemLogsPresentation.ts — second pass.
 *
 * Focused exclusively on the branches of the three target functions that the
 * sibling `systemLogsPresentation.test.ts` does not yet reach:
 *   - getSystemLogLineClass    (every uncovered pattern, precedence, substring
 *                              semantics, and the no-guard throw paths)
 *   - getSystemLogStreamPresentation (ternary truthiness coercion via casts)
 *   - getSystemLogBufferSummary     (boundary / NaN / Infinity / coercion edges)
 *
 * The sibling test already covers ERROR pattern 0, WARN pattern 2, DEBUG
 * pattern 1, the default slate arm, the two canonical stream states, and one
 * buffer summary — those are intentionally NOT re-asserted here.
 *
 * `includesAny` is module-private (not exported); it has no other caller, so
 * it is exercised transitively through `getSystemLogLineClass` rather than
 * imported directly.
 */
import { describe, expect, it } from 'vitest';

import {
  getSystemLogBufferSummary,
  getSystemLogLineClass,
  getSystemLogStreamPresentation,
} from '@/utils/systemLogsPresentation';

describe('getSystemLogLineClass — branch coverage (batch 2)', () => {
  describe('ERROR_PATTERNS — every pattern reaches the red arm', () => {
    // Sibling already covers ERROR pattern 0 ('"level":"error"'); these cover
    // the remaining two patterns: 'ERR' and '[ERROR]'.
    it('matches the bare ERR token (error pattern 1)', () => {
      expect(getSystemLogLineClass('disk ERR timeout')).toBe('text-red-400');
    });

    it('matches the bracketed [ERROR] token (error pattern 2)', () => {
      expect(getSystemLogLineClass('[ERROR] connection refused')).toBe('text-red-400');
    });
  });

  describe('WARNING_PATTERNS — every pattern reaches the amber arm', () => {
    // Sibling already covers WARN pattern 2 ('[WARN]'); these cover patterns
    // 0 ('"level":"warn"') and 1 ('WRN').
    it('matches the JSON warn level (warning pattern 0)', () => {
      expect(getSystemLogLineClass('{"level":"warn","msg":"degraded"}')).toBe('text-amber-400');
    });

    it('matches the bare WRN token (warning pattern 1)', () => {
      expect(getSystemLogLineClass('WRN cache miss rate high')).toBe('text-amber-400');
    });
  });

  describe('DEBUG_PATTERNS — every pattern reaches the blue arm', () => {
    // Sibling already covers DEBUG pattern 1 ('DBG'); these cover patterns
    // 0 ('"level":"debug"') and 2 ('[DEBUG]').
    it('matches the JSON debug level (debug pattern 0)', () => {
      expect(getSystemLogLineClass('{"level":"debug","msg":"refresh"}')).toBe('text-blue-400');
    });

    it('matches the bracketed [DEBUG] token (debug pattern 2)', () => {
      expect(getSystemLogLineClass('[DEBUG] polling interval=30s')).toBe('text-blue-400');
    });
  });

  describe('if/else precedence — first matching severity wins', () => {
    it('prefers error over warn when a line contains both', () => {
      // ERROR_PATTERNS is checked first, so a combined line resolves to red.
      expect(getSystemLogLineClass('[ERROR] rolled back [WARN] stage')).toBe('text-red-400');
    });

    it('prefers error over debug when a line contains both', () => {
      expect(getSystemLogLineClass('[ERROR] and [DEBUG] trace attached')).toBe('text-red-400');
    });

    it('prefers warn over debug when a line contains both', () => {
      // ERROR_PATTERNS miss, WARNING_PATTERNS hit before DEBUG_PATTERNS.
      expect(getSystemLogLineClass('[WARN] retrying [DEBUG] attempt=2')).toBe('text-amber-400');
    });

    it('resolves to red when all three severities appear in one line', () => {
      expect(getSystemLogLineClass('[DEBUG] [WARN] [ERROR] boom ERR WRN DBG')).toBe('text-red-400');
    });
  });

  describe('substring matching via includesAny — no word-boundary anchoring', () => {
    it('matches ERR as a substring inside a larger word (TERROR)', () => {
      // value.includes('ERR') has no anchoring, so 'TERROR' hits the error arm.
      expect(getSystemLogLineClass('TERROR protocol mismatch')).toBe('text-red-400');
    });

    it('does NOT treat the word WARNING as a warn token (no WRN substring)', () => {
      // 'WARNING' contains neither 'WRN', '[WARN]', nor '"level":"warn"', so it
      // falls through every guard to the default slate arm.
      expect(getSystemLogLineClass('WARNING sustained cpu pressure')).toBe('text-slate-300');
    });

    it('matches ERR embedded mid-word after whitespace (TERROR prefixed)', () => {
      // Confirms the substring hit is position-independent.
      expect(getSystemLogLineClass('node TERROR fail')).toBe('text-red-400');
    });
  });

  describe('default arm — unmatched payloads fall through to slate', () => {
    it('returns the slate class for an empty string', () => {
      // No pattern matches '' -> default return.
      expect(getSystemLogLineClass('')).toBe('text-slate-300');
    });

    it('returns the slate class for a plain info line with no tokens', () => {
      // 'INFO' is not in any pattern list (patterns use ERR/WRN/DBG), so this
      // is a meaningful non-match that documents the gap.
      expect(getSystemLogLineClass('INFO request handled in 12ms')).toBe('text-slate-300');
    });
  });

  describe('missing input guard — non-string inputs throw (no null guard)', () => {
    // getSystemLogLineClass immediately calls log.includes(...) via includesAny
    // with no type guard, so null/undefined/number throw a TypeError at
    // runtime. These document the absence of a guard rather than a branch.
    it('throws a TypeError when log is null', () => {
      const log = null as unknown as Parameters<typeof getSystemLogLineClass>[0];
      expect(() => getSystemLogLineClass(log)).toThrow(TypeError);
    });

    it('throws a TypeError when log is undefined', () => {
      const log = undefined as unknown as Parameters<typeof getSystemLogLineClass>[0];
      expect(() => getSystemLogLineClass(log)).toThrow(TypeError);
    });

    it('throws a TypeError when log is a number (no .includes method)', () => {
      const log = 0 as unknown as Parameters<typeof getSystemLogLineClass>[0];
      expect(() => getSystemLogLineClass(log)).toThrow(TypeError);
    });
  });
});

describe('getSystemLogStreamPresentation — ternary truthiness coercion (batch 2)', () => {
  // The sibling test covers the canonical boolean true/false. These exercise
  // the ternary condition's truthiness evaluation when a non-boolean is
  // smuggled in through a cast — proving the condition is `paused ? ...` and
  // not `paused === true ? ...`.

  it('treats numeric 0 as falsy and selects the live presentation', () => {
    const paused = 0 as unknown as Parameters<typeof getSystemLogStreamPresentation>[0];
    const result = getSystemLogStreamPresentation(paused);
    expect(result.label).toBe('Streaming');
    expect(result.toggleTitle).toBe('Pause Stream');
  });

  it('treats empty string as falsy and selects the live presentation', () => {
    const paused = '' as unknown as Parameters<typeof getSystemLogStreamPresentation>[0];
    const result = getSystemLogStreamPresentation(paused);
    expect(result.label).toBe('Streaming');
    expect(result.toggleTitle).toBe('Pause Stream');
  });

  it('treats numeric 1 as truthy and selects the paused presentation', () => {
    const paused = 1 as unknown as Parameters<typeof getSystemLogStreamPresentation>[0];
    const result = getSystemLogStreamPresentation(paused);
    expect(result.label).toBe('Paused');
    expect(result.toggleTitle).toBe('Resume Stream');
  });

  it('treats a non-empty string as truthy and selects the paused presentation', () => {
    const paused = 'paused' as unknown as Parameters<typeof getSystemLogStreamPresentation>[0];
    const result = getSystemLogStreamPresentation(paused);
    expect(result.label).toBe('Paused');
    expect(result.toggleTitle).toBe('Resume Stream');
  });

  it('returns the distinct indicator class for each coerced arm', () => {
    // Locks the observable side-effect of the branch: live pulses emerald,
    // paused is a steady amber. Uses two different coerced truthy/falsy
    // values than the cases above to avoid pure duplication.
    const truthy = 'truthy' as unknown as Parameters<typeof getSystemLogStreamPresentation>[0];
    const falsy = NaN as unknown as Parameters<typeof getSystemLogStreamPresentation>[0];
    expect(getSystemLogStreamPresentation(truthy).indicatorClass).toBe('bg-amber-400');
    expect(getSystemLogStreamPresentation(falsy).indicatorClass).toBe(
      'bg-emerald-400 animate-pulse',
    );
  });
});

describe('getSystemLogBufferSummary — boundary & coercion edges (batch 2)', () => {
  // No control-flow branches exist here; coverage is gained by exercising the
  // template-literal interpolation at boundary values and for malformed inputs
  // that document the absence of any clamping / NaN guard.

  it('formats an empty buffer (logCount 0)', () => {
    expect(getSystemLogBufferSummary(0, 500)).toBe('Buffer: 0 / 500 lines');
  });

  it('formats a full buffer (logCount === maxLogs)', () => {
    expect(getSystemLogBufferSummary(500, 500)).toBe('Buffer: 500 / 500 lines');
  });

  it('formats a zero-capacity buffer (maxLogs 0)', () => {
    expect(getSystemLogBufferSummary(0, 0)).toBe('Buffer: 0 / 0 lines');
  });

  it('does not clamp a negative count (no guard)', () => {
    // Negative logCount survives untouched into the template literal.
    expect(getSystemLogBufferSummary(-5, 500)).toBe('Buffer: -5 / 500 lines');
  });

  it('does not clamp a count exceeding the maximum (no guard)', () => {
    // Over-capacity is rendered literally rather than being capped at maxLogs.
    expect(getSystemLogBufferSummary(750, 500)).toBe('Buffer: 750 / 500 lines');
  });

  it('interpolates NaN verbatim (no NaN guard)', () => {
    expect(getSystemLogBufferSummary(Number.NaN, 500)).toBe('Buffer: NaN / 500 lines');
  });

  it('interpolates Infinity verbatim (no finite guard)', () => {
    expect(getSystemLogBufferSummary(Number.POSITIVE_INFINITY, 500)).toBe(
      'Buffer: Infinity / 500 lines',
    );
  });

  it('String()-coerces a smuggled string count via template interpolation', () => {
    const logCount = '42' as unknown as Parameters<typeof getSystemLogBufferSummary>[0];
    expect(getSystemLogBufferSummary(logCount, 500)).toBe('Buffer: 42 / 500 lines');
  });

  it('String()-coerces null to "null" with no guard', () => {
    const logCount = null as unknown as Parameters<typeof getSystemLogBufferSummary>[0];
    expect(getSystemLogBufferSummary(logCount, 500)).toBe('Buffer: null / 500 lines');
  });

  it('String()-coerces undefined to "undefined" with no guard', () => {
    const logCount = undefined as unknown as Parameters<typeof getSystemLogBufferSummary>[0];
    expect(getSystemLogBufferSummary(logCount, 500)).toBe('Buffer: undefined / 500 lines');
  });

  it('String()-coerces a smuggled string maximum via template interpolation', () => {
    const maxLogs = '1k' as unknown as Parameters<typeof getSystemLogBufferSummary>[1];
    expect(getSystemLogBufferSummary(10, maxLogs)).toBe('Buffer: 10 / 1k lines');
  });
});
