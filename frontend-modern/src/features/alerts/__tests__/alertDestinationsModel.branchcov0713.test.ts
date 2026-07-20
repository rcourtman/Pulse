import { describe, expect, it } from 'vitest';

import type { AppriseConfig } from '@/api/notifications';
import type { UIAppriseConfig, UIEmailConfig } from '../types';

import {
  buildAppriseConfigPayload,
  buildEmailConfigPayload,
  normalizeAppriseConfig,
} from '../alertDestinationsModel';

// ---------------------------------------------------------------------------
// Fixtures — minimal valid UI configs.  Tests override only the fields
// relevant to the branch under examination.  Mirrors the helpers used by the
// sibling `branchcov0712` / `branchcov0712c` suites so conventions stay
// identical.  Defaults match `createDefaultAppriseConfig` /
// `createDefaultEmailConfig` in ../helpers.ts.
// ---------------------------------------------------------------------------

function makeUIAppriseConfig(overrides: Partial<UIAppriseConfig> = {}): UIAppriseConfig {
  return {
    enabled: false,
    mode: 'cli',
    targetsText: '',
    cliPath: 'apprise',
    timeoutSeconds: 15,
    serverUrl: '',
    configKey: '',
    apiKey: '',
    apiKeyHeader: 'X-API-KEY',
    skipTlsVerify: false,
    ...overrides,
  };
}

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

// Credential-shaped strings are built by interpolating a variable so the
// repo's gitleaks hook does not flag a hardcoded literal.
const fakeKey = 'k'.repeat(32);
const fakeToken = 't'.repeat(40);

describe('normalizeAppriseConfig — branch coverage (0713)', () => {
  // -----------------------------------------------------------------------
  // config?.enabled ?? false
  //   - nullish coalescing: undefined / null  → false (right arm)
  //   - defined boolean                     → as-is (left arm)
  // -----------------------------------------------------------------------
  describe('enabled ?? false', () => {
    it('falls back to false when enabled is undefined', () => {
      const result = normalizeAppriseConfig({ enabled: undefined });
      expect(result.enabled).toBe(false);
    });

    it('falls back to false when the whole config is null', () => {
      const result = normalizeAppriseConfig(null);
      expect(result.enabled).toBe(false);
    });

    it('falls back to false when the whole config is undefined', () => {
      const result = normalizeAppriseConfig(undefined);
      expect(result.enabled).toBe(false);
    });

    it('preserves an explicit false', () => {
      const result = normalizeAppriseConfig({ enabled: false });
      expect(result.enabled).toBe(false);
    });

    it('preserves an explicit true', () => {
      const result = normalizeAppriseConfig({ enabled: true });
      expect(result.enabled).toBe(true);
    });
  });

  // -----------------------------------------------------------------------
  // config?.mode === 'http' ? 'http' : 'cli'
  //   - strict-equality true arm:  mode === 'http'
  //   - strict-equality false arm: 'cli', undefined, junk string, etc.
  // -----------------------------------------------------------------------
  describe('mode ternary', () => {
    it("returns 'http' when mode is exactly 'http'", () => {
      const result = normalizeAppriseConfig({ mode: 'http' });
      expect(result.mode).toBe('http');
    });

    it("returns 'cli' when mode is exactly 'cli'", () => {
      const result = normalizeAppriseConfig({ mode: 'cli' });
      expect(result.mode).toBe('cli');
    });

    it("returns 'cli' when mode is undefined", () => {
      const result = normalizeAppriseConfig({ mode: undefined });
      expect(result.mode).toBe('cli');
    });

    it("returns 'cli' for an unrecognized mode string (false arm of strict equality)", () => {
      // 'mailto' is a real Apprise schema but not 'http'; the normalizer
      // collapses anything non-'http' to 'cli'.  Cast because AppriseConfig
      // constrains mode to 'cli' | 'http'.
      const malformed = { mode: 'mailto' } as unknown as Parameters<
        typeof normalizeAppriseConfig
      >[0];
      const result = normalizeAppriseConfig(malformed);
      expect(result.mode).toBe('cli');
    });
  });

  // -----------------------------------------------------------------------
  // config?.cliPath || 'apprise'   (and the same || pattern for the four
  // other scalar string fields).  The || operator treats '' as falsy, so
  // an empty string and undefined both hit the right arm.
  // -----------------------------------------------------------------------
  describe("scalar string '||' fallbacks", () => {
    it("falls back cliPath to 'apprise' when empty or undefined", () => {
      expect(normalizeAppriseConfig({ cliPath: '' }).cliPath).toBe('apprise');
      expect(normalizeAppriseConfig({ cliPath: undefined }).cliPath).toBe('apprise');
    });

    it('forwards a custom cliPath verbatim', () => {
      expect(normalizeAppriseConfig({ cliPath: '/usr/local/bin/apprise' }).cliPath).toBe(
        '/usr/local/bin/apprise',
      );
    });

    it("falls back serverUrl to '' when empty or undefined", () => {
      expect(normalizeAppriseConfig({ serverUrl: '' }).serverUrl).toBe('');
      expect(normalizeAppriseConfig({ serverUrl: undefined }).serverUrl).toBe('');
    });

    it('forwards a custom serverUrl verbatim', () => {
      expect(normalizeAppriseConfig({ serverUrl: 'https://apprise.test' }).serverUrl).toBe(
        'https://apprise.test',
      );
    });

    it("falls back configKey to '' when empty or undefined", () => {
      expect(normalizeAppriseConfig({ configKey: '' }).configKey).toBe('');
      expect(normalizeAppriseConfig({ configKey: undefined }).configKey).toBe('');
    });

    it('forwards a custom configKey verbatim', () => {
      expect(normalizeAppriseConfig({ configKey: 'primary' }).configKey).toBe('primary');
    });

    it("falls back apiKey to '' when empty or undefined", () => {
      expect(normalizeAppriseConfig({ apiKey: '' }).apiKey).toBe('');
      expect(normalizeAppriseConfig({ apiKey: undefined }).apiKey).toBe('');
    });

    it('forwards a custom apiKey verbatim', () => {
      expect(normalizeAppriseConfig({ apiKey: fakeKey }).apiKey).toBe(fakeKey);
    });

    it("falls back apiKeyHeader to 'X-API-KEY' when empty or undefined", () => {
      expect(normalizeAppriseConfig({ apiKeyHeader: '' }).apiKeyHeader).toBe('X-API-KEY');
      expect(normalizeAppriseConfig({ apiKeyHeader: undefined }).apiKeyHeader).toBe('X-API-KEY');
    });

    it('forwards a custom apiKeyHeader verbatim', () => {
      expect(normalizeAppriseConfig({ apiKeyHeader: 'Authorization' }).apiKeyHeader).toBe(
        'Authorization',
      );
    });
  });

  // -----------------------------------------------------------------------
  // typeof config?.timeoutSeconds === 'number' && config.timeoutSeconds > 0
  //   - guard true  arm: positive number → forwarded
  //   - guard false arm: non-number, 0, negative, NaN, Infinity(? — >0 is
  //     true for Infinity, typeof number true, so Infinity IS forwarded),
  //     undefined, empty string.
  //   Note: Infinity is a number > 0 and passes the guard — pin this
  //   surprising behaviour down so it cannot change silently.
  // -----------------------------------------------------------------------
  describe('timeoutSeconds guard', () => {
    it('forwards a positive integer', () => {
      expect(normalizeAppriseConfig({ timeoutSeconds: 30 }).timeoutSeconds).toBe(30);
    });

    it('forwards a positive fractional value', () => {
      expect(normalizeAppriseConfig({ timeoutSeconds: 0.5 }).timeoutSeconds).toBe(0.5);
    });

    it('falls back to 15 for zero (boundary: not > 0)', () => {
      expect(normalizeAppriseConfig({ timeoutSeconds: 0 }).timeoutSeconds).toBe(15);
    });

    it('falls back to 15 for a negative number', () => {
      expect(normalizeAppriseConfig({ timeoutSeconds: -5 }).timeoutSeconds).toBe(15);
    });

    it('falls back to 15 for NaN (typeof number but not > 0)', () => {
      const malformed = { timeoutSeconds: Number.NaN } as Partial<AppriseConfig>;
      expect(normalizeAppriseConfig(malformed).timeoutSeconds).toBe(15);
    });

    it('falls back to 15 for a non-number value', () => {
      const malformed = { timeoutSeconds: '30' } as unknown as Parameters<
        typeof normalizeAppriseConfig
      >[0];
      expect(normalizeAppriseConfig(malformed).timeoutSeconds).toBe(15);
    });

    it('falls back to 15 when undefined', () => {
      expect(normalizeAppriseConfig({ timeoutSeconds: undefined }).timeoutSeconds).toBe(15);
    });

    it('surprisingly forwards Infinity because typeof Infinity === "number" and Infinity > 0', () => {
      // Documents current behaviour: the guard checks typeof and > 0 but
      // not Number.isFinite, so Infinity leaks through. Pinned, not fixed.
      const malformed = { timeoutSeconds: Number.POSITIVE_INFINITY } as Partial<AppriseConfig>;
      expect(normalizeAppriseConfig(malformed).timeoutSeconds).toBe(Number.POSITIVE_INFINITY);
    });
  });

  // -----------------------------------------------------------------------
  // Boolean(config?.skipTlsVerify)
  //   - undefined / false → false
  //   - true              → true
  //   - truthy non-bool   → true   (defensive: API may return 0/1 or a
  //                                 string before strict typing lands)
  //   - falsy non-bool    → false
  // -----------------------------------------------------------------------
  describe('skipTlsVerify Boolean() coercion', () => {
    it('coerces undefined to false', () => {
      expect(normalizeAppriseConfig({ skipTlsVerify: undefined }).skipTlsVerify).toBe(false);
    });

    it('preserves an explicit false', () => {
      expect(normalizeAppriseConfig({ skipTlsVerify: false }).skipTlsVerify).toBe(false);
    });

    it('preserves an explicit true', () => {
      expect(normalizeAppriseConfig({ skipTlsVerify: true }).skipTlsVerify).toBe(true);
    });

    it('coerces a truthy non-boolean (1) to true', () => {
      const malformed = { skipTlsVerify: 1 } as unknown as Parameters<
        typeof normalizeAppriseConfig
      >[0];
      expect(normalizeAppriseConfig(malformed).skipTlsVerify).toBe(true);
    });

    it('coerces a falsy non-boolean (0) to false', () => {
      const malformed = { skipTlsVerify: 0 } as unknown as Parameters<
        typeof normalizeAppriseConfig
      >[0];
      expect(normalizeAppriseConfig(malformed).skipTlsVerify).toBe(false);
    });
  });

  // -----------------------------------------------------------------------
  // formatAppriseTargets(config?.targets)
  //   helper: targets && targets.length > 0 ? targets.join('\n') : ''
  //   - undefined → ''
  //   - null      → ''
  //   - []        → ''
  //   - ['a']     → 'a'
  //   - ['a','b'] → 'a\nb'
  // -----------------------------------------------------------------------
  describe('targets → targetsText via formatAppriseTargets', () => {
    it("returns '' when targets is undefined", () => {
      expect(normalizeAppriseConfig({ targets: undefined }).targetsText).toBe('');
    });

    it("returns '' when targets is null", () => {
      const withNull = { targets: null } as unknown as Partial<AppriseConfig>;
      expect(normalizeAppriseConfig(withNull).targetsText).toBe('');
    });

    it("returns '' when targets is an empty array", () => {
      expect(normalizeAppriseConfig({ targets: [] }).targetsText).toBe('');
    });

    it('joins a single target without a trailing newline', () => {
      expect(normalizeAppriseConfig({ targets: ['mailto://user@example.com'] }).targetsText).toBe(
        'mailto://user@example.com',
      );
    });

    it("joins multiple targets with '\\n'", () => {
      expect(
        normalizeAppriseConfig({
          targets: ['mailto://a@example.com', 'tgram://bot/123'],
        }).targetsText,
      ).toBe('mailto://a@example.com\ntgram://bot/123');
    });
  });

  // -----------------------------------------------------------------------
  // Whole-object contract: feeding null / undefined must produce exactly the
  // createDefaultAppriseConfig() shape — every fallback arm firing at once.
  // -----------------------------------------------------------------------
  describe('whole-object defaults', () => {
    const expectedDefault: UIAppriseConfig = {
      enabled: false,
      mode: 'cli',
      targetsText: '',
      cliPath: 'apprise',
      timeoutSeconds: 15,
      serverUrl: '',
      configKey: '',
      apiKey: '',
      apiKeyHeader: 'X-API-KEY',
      skipTlsVerify: false,
    };

    it('returns the exact default shape for null input', () => {
      expect(normalizeAppriseConfig(null)).toStrictEqual(expectedDefault);
    });

    it('returns the exact default shape for undefined input', () => {
      expect(normalizeAppriseConfig(undefined)).toStrictEqual(expectedDefault);
    });

    it('returns the exact default shape for an empty object', () => {
      expect(normalizeAppriseConfig({})).toStrictEqual(expectedDefault);
    });
  });
});

describe('buildAppriseConfigPayload — branch coverage (0713)', () => {
  // -----------------------------------------------------------------------
  // config.mode passthrough — both union arms forwarded unchanged.
  // -----------------------------------------------------------------------
  describe('mode passthrough', () => {
    it("forwards mode 'http' unchanged", () => {
      expect(buildAppriseConfigPayload(makeUIAppriseConfig({ mode: 'http' })).mode).toBe('http');
    });

    it("forwards mode 'cli' unchanged", () => {
      expect(buildAppriseConfigPayload(makeUIAppriseConfig({ mode: 'cli' })).mode).toBe('cli');
    });
  });

  // -----------------------------------------------------------------------
  // config.enabled / config.skipTlsVerify passthrough — boolean forwarded
  // as-is (no coercion happens on the way out; that already happened on the
  // way in via normalizeAppriseConfig).
  // -----------------------------------------------------------------------
  describe('boolean passthrough', () => {
    it('forwards enabled true', () => {
      expect(buildAppriseConfigPayload(makeUIAppriseConfig({ enabled: true })).enabled).toBe(true);
    });

    it('forwards enabled false', () => {
      expect(buildAppriseConfigPayload(makeUIAppriseConfig({ enabled: false })).enabled).toBe(
        false,
      );
    });

    it('forwards skipTlsVerify true', () => {
      expect(
        buildAppriseConfigPayload(makeUIAppriseConfig({ skipTlsVerify: true })).skipTlsVerify,
      ).toBe(true);
    });

    it('forwards skipTlsVerify false', () => {
      expect(
        buildAppriseConfigPayload(makeUIAppriseConfig({ skipTlsVerify: false })).skipTlsVerify,
      ).toBe(false);
    });
  });

  // -----------------------------------------------------------------------
  // parseAppriseTargets(config.targetsText)
  //   value.split(/\r?\n|,/).map(trim).filter(len>0 && firstOccurrence)
  //   - empty / whitespace-only → []
  //   - single line             → [line]
  //   - \n separated            → split
  //   - \r\n separated          → split (the optional \r arm of the regex)
  //   - comma separated         → split
  //   - mixed \n + ,            → split on both
  //   - empty entries from adjacent separators → filtered
  //   - leading/trailing whitespace per entry  → trimmed
  //   - duplicate entries after trim            → deduped (first wins)
  // -----------------------------------------------------------------------
  describe('targetsText → targets via parseAppriseTargets', () => {
    it('returns [] for an empty targetsText', () => {
      expect(
        buildAppriseConfigPayload(makeUIAppriseConfig({ targetsText: '' })).targets,
      ).toStrictEqual([]);
    });

    it('returns [] for a whitespace-only targetsText (filtered after trim)', () => {
      expect(
        buildAppriseConfigPayload(makeUIAppriseConfig({ targetsText: '  \n  ,' })).targets,
      ).toStrictEqual([]);
    });

    it('returns a single-element array for a one-line targetsText', () => {
      expect(
        buildAppriseConfigPayload(makeUIAppriseConfig({ targetsText: 'mailto://a@example.com' }))
          .targets,
      ).toStrictEqual(['mailto://a@example.com']);
    });

    it('splits on bare-LF newlines', () => {
      expect(
        buildAppriseConfigPayload(
          makeUIAppriseConfig({ targetsText: 'a@example.com\nb@example.com' }),
        ).targets,
      ).toStrictEqual(['a@example.com', 'b@example.com']);
    });

    it('splits on CRLF newlines (exercises the optional \\r arm of /\\r?\\n/)', () => {
      expect(
        buildAppriseConfigPayload(
          makeUIAppriseConfig({ targetsText: 'a@example.com\r\nb@example.com' }),
        ).targets,
      ).toStrictEqual(['a@example.com', 'b@example.com']);
    });

    it('splits on commas', () => {
      expect(
        buildAppriseConfigPayload(
          makeUIAppriseConfig({ targetsText: 'a@example.com,b@example.com' }),
        ).targets,
      ).toStrictEqual(['a@example.com', 'b@example.com']);
    });

    it('splits on a mixture of newlines and commas', () => {
      expect(
        buildAppriseConfigPayload(
          makeUIAppriseConfig({
            targetsText: 'a@example.com\nb@example.com,c@example.com',
          }),
        ).targets,
      ).toStrictEqual(['a@example.com', 'b@example.com', 'c@example.com']);
    });

    it('drops entries produced by adjacent/leading/trailing separators', () => {
      // Leading newline, trailing comma, double comma, blank line — all
      // yield empty trimmed entries that the filter removes.
      expect(
        buildAppriseConfigPayload(
          makeUIAppriseConfig({
            targetsText: '\na@example.com,,b@example.com\n,c@example.com,',
          }),
        ).targets,
      ).toStrictEqual(['a@example.com', 'b@example.com', 'c@example.com']);
    });

    it('trims whitespace around each entry before filtering', () => {
      expect(
        buildAppriseConfigPayload(
          makeUIAppriseConfig({
            targetsText: '  a@example.com  , \nb@example.com\t',
          }),
        ).targets,
      ).toStrictEqual(['a@example.com', 'b@example.com']);
    });

    it('dedupes identical entries after trimming, keeping the first occurrence', () => {
      // The two literal duplicates and the padded duplicate all trim to the
      // same string; only the first is kept.
      expect(
        buildAppriseConfigPayload(
          makeUIAppriseConfig({
            targetsText: 'a@example.com, a@example.com ,  a@example.com',
          }),
        ).targets,
      ).toStrictEqual(['a@example.com']);
    });

    it('treats differently-cased entries as distinct (no lowercasing)', () => {
      expect(
        buildAppriseConfigPayload(makeUIAppriseConfig({ targetsText: 'A@x.com,a@x.com' })).targets,
      ).toStrictEqual(['A@x.com', 'a@x.com']);
    });

    it('preserves internal whitespace within an entry (trim is ends-only)', () => {
      expect(
        buildAppriseConfigPayload(makeUIAppriseConfig({ targetsText: 'a b@example.com' })).targets,
      ).toStrictEqual(['a b@example.com']);
    });
  });

  // -----------------------------------------------------------------------
  // Scalar string/number passthrough — every remaining field is forwarded
  // verbatim.  Drive each with a non-default, non-empty value (and the
  // boundary timeoutSeconds: 0) so the assertion cannot pass against the
  // fixture default by accident.
  // -----------------------------------------------------------------------
  describe('scalar passthrough', () => {
    it('forwards cliPath, serverUrl, configKey, apiKeyHeader verbatim', () => {
      const result = buildAppriseConfigPayload(
        makeUIAppriseConfig({
          cliPath: '/usr/local/bin/apprise',
          serverUrl: 'https://apprise.test',
          configKey: 'primary',
          apiKey: fakeKey,
          apiKeyHeader: 'Authorization',
        }),
      );
      expect(result.cliPath).toBe('/usr/local/bin/apprise');
      expect(result.serverUrl).toBe('https://apprise.test');
      expect(result.configKey).toBe('primary');
      expect(result.apiKey).toBe(fakeKey);
      expect(result.apiKeyHeader).toBe('Authorization');
    });

    it('forwards a positive timeoutSeconds verbatim', () => {
      expect(
        buildAppriseConfigPayload(makeUIAppriseConfig({ timeoutSeconds: 45 })).timeoutSeconds,
      ).toBe(45);
    });

    it('forwards timeoutSeconds = 0 verbatim (no clamping on the way out)', () => {
      // buildAppriseConfigPayload performs no guard on timeoutSeconds —
      // the inbound normalizeAppriseConfig guard is the only place 0 is
      // rewritten. Pin this so a future clamp here would be noticed.
      expect(
        buildAppriseConfigPayload(makeUIAppriseConfig({ timeoutSeconds: 0 })).timeoutSeconds,
      ).toBe(0);
    });
  });

  // -----------------------------------------------------------------------
  // Output contract — exact key set in declaration order. Catches any
  // future field added to UIAppriseConfig that fails to be forwarded, and
  // catches UI-only fields (targetsText) leaking into the API payload.
  // -----------------------------------------------------------------------
  describe('payload contract — exact key set', () => {
    it('emits exactly the 10 AppriseConfig keys and omits targetsText', () => {
      const result = buildAppriseConfigPayload(
        makeUIAppriseConfig({
          enabled: true,
          mode: 'http',
          targetsText: 'a@example.com\nb@example.com',
          cliPath: '/usr/bin/apprise',
          timeoutSeconds: 20,
          serverUrl: 'https://apprise.test',
          configKey: 'primary',
          apiKey: fakeKey,
          apiKeyHeader: 'X-API-KEY',
          skipTlsVerify: false,
        }),
      );
      expect(Object.keys(result)).toStrictEqual([
        'enabled',
        'mode',
        'targets',
        'cliPath',
        'timeoutSeconds',
        'serverUrl',
        'configKey',
        'apiKey',
        'apiKeyHeader',
        'skipTlsVerify',
      ]);
      expect(result).not.toHaveProperty('targetsText');
    });

    it('matches the exact AppriseConfig payload for a fully-populated input', () => {
      const result = buildAppriseConfigPayload(
        makeUIAppriseConfig({
          enabled: true,
          mode: 'http',
          targetsText: '  a@example.com , b@example.com ',
          cliPath: '/usr/bin/apprise',
          timeoutSeconds: 20,
          serverUrl: 'https://apprise.test',
          configKey: 'primary',
          apiKey: fakeKey,
          apiKeyHeader: 'X-API-KEY',
          skipTlsVerify: true,
        }),
      );
      expect(result).toStrictEqual({
        enabled: true,
        mode: 'http',
        targets: ['a@example.com', 'b@example.com'],
        cliPath: '/usr/bin/apprise',
        timeoutSeconds: 20,
        serverUrl: 'https://apprise.test',
        configKey: 'primary',
        apiKey: fakeKey,
        apiKeyHeader: 'X-API-KEY',
        skipTlsVerify: true,
      });
    });
  });
});

// ---------------------------------------------------------------------------
// buildEmailConfigPayload — focused additions only.
// The sibling suites `branchcov0712` and `branchcov0712c` already exhaust
// the `to.map(trim)` / `to.filter(length>0)` arms, scalar passthrough for
// port/password/from/server/enabled/tls/startTLS/provider, the exact key
// set, and the malformed non-string entry.  This block adds the few
// passthrough arms they leave open: `username` (empty + custom) and a
// realistic long credential-shaped password (the existing suites only use
// 'secret' or '').  No assertions are duplicated.
// ---------------------------------------------------------------------------
describe('buildEmailConfigPayload — branch coverage additions (0713)', () => {
  describe('username passthrough', () => {
    it('forwards an empty username unchanged', () => {
      const result = buildEmailConfigPayload(makeUIEmailConfig({ username: '' }));
      expect(result.username).toBe('');
    });

    it('forwards a custom username unchanged', () => {
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({ username: 'svc-account@internal' }),
      );
      expect(result.username).toBe('svc-account@internal');
    });
  });

  describe('password passthrough with a realistic credential', () => {
    it('forwards a long token-shaped password verbatim and preserves it in the exact payload', () => {
      const password = `Bearer ${fakeToken}`;
      const result = buildEmailConfigPayload(makeUIEmailConfig({ password }));
      expect(result.password).toBe(password);
      // Confirm the long credential survives alongside a normal recipient
      // list — exercises the full payload build, not just one getter.
      expect(result).toStrictEqual({
        enabled: true,
        provider: 'smtp',
        server: 'smtp.internal',
        port: 587,
        username: 'ops@example.com',
        password,
        from: 'pulse@example.com',
        to: ['alerts@example.com'],
        tls: true,
        startTLS: true,
      });
    });
  });
});
