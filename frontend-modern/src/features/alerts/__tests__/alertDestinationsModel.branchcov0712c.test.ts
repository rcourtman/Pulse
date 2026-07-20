import { describe, expect, it } from 'vitest';

import type { UIEmailConfig } from '../types';

import { buildEmailConfigPayload } from '../alertDestinationsModel';

// ---------------------------------------------------------------------------
// Fixture — minimal valid UIEmailConfig. Tests override only the fields
// relevant to the branch under examination. Mirrors the helper used by the
// sibling `branchcov0712` suite so conventions stay identical.
// ---------------------------------------------------------------------------

function makeUIEmailConfig(overrides: Partial<UIEmailConfig> = {}): UIEmailConfig {
  return {
    enabled: true,
    provider: 'smtp',
    server: 'smtp.internal',
    port: 587,
    username: 'ops@example.com',
    password: 'secret',
    from: 'pulse@example.com',
    to: ['alerts@example.com'],
    tls: true,
    startTLS: true,
    replyTo: '',
    maxRetries: 3,
    retryDelay: 60,
    rateLimit: 0,
    ...overrides,
  };
}

describe('buildEmailConfigPayload — branch coverage (batch 3 / 0712c)', () => {
  // -----------------------------------------------------------------------
  // to.map((entry) => entry.trim())
  // The trim arm is exercised for each distinct class of whitespace
  // `String.prototype.trim` removes. The sibling suite already covers
  // ' ', '\t', '\n'; here we drive the remaining whitespace code points
  // so the trim callback is hit with inputs that previously had no
  // dedicated test. Several of these also collapse to '' and thus feed
  // the filter false arm with exotic-whitespace-only entries.
  // -----------------------------------------------------------------------
  describe('trim arm — exotic whitespace', () => {
    it('strips carriage-return and CRLF sequences from both ends', () => {
      const result = buildEmailConfigPayload(makeUIEmailConfig({ to: ['\r\ruser@test.com\r\n'] }));
      expect(result.to).toStrictEqual(['user@test.com']);
    });

    it('strips form-feed and vertical-tab characters', () => {
      // \f (form feed, U+000C) and \v (vertical tab, U+000B) are both
      // in the trim set; an entry made solely of them must collapse and
      // be dropped by the filter.
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({ to: ['\f\v', '\fkeep-f@test.com\v'] }),
      );
      expect(result.to).toStrictEqual(['keep-f@test.com']);
    });

    it('strips Unicode whitespace: NBSP, em space, ideographic space', () => {
      // U+00A0 (no-break space), U+2003 (em space), U+3000 (ideographic
      // space) are all removed by trim(). A whitespace-only entry drops;
      // a padded entry is cleaned.
      const nbsp = '\u00A0';
      const em = '\u2003';
      const ideo = '\u3000';
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({
          to: [`${nbsp}${em}${ideo}`, `${nbsp}unicode@test.com${ideo}`],
        }),
      );
      expect(result.to).toStrictEqual(['unicode@test.com']);
    });

    it('drops entries composed of Unicode line/paragraph separators', () => {
      // U+2028 (line separator) and U+2029 (paragraph separator) are
      // whitespace from trim()'s perspective; a sole entry of them must
      // be filtered out, leaving an empty to list.
      const result = buildEmailConfigPayload(makeUIEmailConfig({ to: ['\u2028\u2029'] }));
      expect(result.to).toStrictEqual([]);
    });
  });

  // -----------------------------------------------------------------------
  // to.filter((entry) => entry.length > 0)
  // The predicate is `entry.length > 0`, NOT Boolean coercion. The
  // sibling suite never feeds a recipient whose truthiness differs from
  // its non-emptiness, so these cases pin down the exact semantics of
  // the filter arm.
  // -----------------------------------------------------------------------
  describe('filter predicate — length-vs-truthiness', () => {
    it('keeps the string "0" because length > 0 (not truthiness)', () => {
      // "0" is falsy under Boolean() but has length 1; the contract is
      // to keep it.
      const result = buildEmailConfigPayload(makeUIEmailConfig({ to: ['0'] }));
      expect(result.to).toStrictEqual(['0']);
    });

    it('keeps the literal string "false" because length > 0', () => {
      const result = buildEmailConfigPayload(makeUIEmailConfig({ to: ['false'] }));
      expect(result.to).toStrictEqual(['false']);
    });

    it('drops a single whitespace-only entry, invoking the filter predicate exactly once on the false arm', () => {
      // Existing suites drop whitespace with multi-element arrays or an
      // empty array. A single-element drop exercises the false arm with
      // the predicate called exactly once.
      const result = buildEmailConfigPayload(makeUIEmailConfig({ to: ['   '] }));
      expect(result.to).toStrictEqual([]);
    });

    it('keeps every entry when none collapse to empty (filter true arm only)', () => {
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({
          to: ['one@test.com', 'two@test.com', 'three@test.com'],
        }),
      );
      expect(result.to).toStrictEqual(['one@test.com', 'two@test.com', 'three@test.com']);
    });
  });

  // -----------------------------------------------------------------------
  // trim() only strips leading/trailing whitespace. Internal whitespace —
  // including characters trim recognises — must survive. This separates
  // "trim both ends" from a hypothetical "trim everywhere" implementation.
  // -----------------------------------------------------------------------
  describe('internal whitespace preservation', () => {
    it('preserves an internal newline while trimming ends', () => {
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({ to: ['  multi\nline@test.com  '] }),
      );
      expect(result.to).toStrictEqual(['multi\nline@test.com']);
    });

    it('preserves an internal tab while trimming ends', () => {
      const result = buildEmailConfigPayload(makeUIEmailConfig({ to: ['\ta\tb@test.com\t'] }));
      expect(result.to).toStrictEqual(['a\tb@test.com']);
    });
  });

  // -----------------------------------------------------------------------
  // Output contract — assert the EXACT key set so any future field added
  // to UIEmailConfig that fails to be forwarded (or any UI-only field
  // that leaks through) is caught. This is the structural complement to
  // the value-level toStrictEqual checks in the sibling suite.
  // -----------------------------------------------------------------------
  describe('payload contract — exact key set', () => {
    it('emits exactly the 11 EmailConfig keys in declaration order and omits replyTo/maxRetries/retryDelay/rateLimit', () => {
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({
          replyTo: 'reply@example.com',
          maxRetries: 9,
          retryDelay: 300,
          rateLimit: 50,
        }),
      );
      expect(Object.keys(result)).toStrictEqual([
        'enabled',
        'provider',
        'server',
        'port',
        'username',
        'password',
        'from',
        'to',
        'tls',
        'startTLS',
      ]);
      expect(result).not.toHaveProperty('replyTo');
      expect(result).not.toHaveProperty('maxRetries');
      expect(result).not.toHaveProperty('retryDelay');
      // rateLimit exists on EmailConfig (optional) yet is never
      // forwarded — see GLM_REPORT.md (suspected source bug).
      expect(result).not.toHaveProperty('rateLimit');
    });
  });

  // -----------------------------------------------------------------------
  // Scalar passthrough — provider is a free-form string; verify both an
  // empty and a custom value flow through untouched (distinct from the
  // hardcoded 'smtp'/'ses' values used elsewhere).
  // -----------------------------------------------------------------------
  describe('provider passthrough', () => {
    it('forwards an empty provider string unchanged', () => {
      const result = buildEmailConfigPayload(makeUIEmailConfig({ provider: '' }));
      expect(result.provider).toBe('');
    });

    it('forwards a custom provider string unchanged', () => {
      const result = buildEmailConfigPayload(makeUIEmailConfig({ provider: 'mailgun' }));
      expect(result.provider).toBe('mailgun');
    });
  });

  // -----------------------------------------------------------------------
  // Defensive gap — the function performs no type guard on `to` entries
  // before calling .trim(). A non-string entry (impossible via the typed
  // surface but reachable through `as unknown as`) propagates a TypeError,
  // documenting that there is no graceful handling branch to hit.
  // -----------------------------------------------------------------------
  describe('defensive behaviour on malformed input', () => {
    it('throws a TypeError when a to entry is not a string (no guard branch)', () => {
      const malformed = makeUIEmailConfig({
        to: ['ok@test.com', 12345] as unknown as string[],
      });
      expect(() => buildEmailConfigPayload(malformed)).toThrow(TypeError);
    });
  });
});
