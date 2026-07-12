import { describe, expect, it } from 'vitest';

import type { UIEmailConfig } from '../types';

import { buildEmailConfigPayload } from '../alertDestinationsModel';

// ---------------------------------------------------------------------------
// Fixture — minimal valid UIEmailConfig.  Tests override only the fields
// relevant to the branch under examination.
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

describe('buildEmailConfigPayload — branch coverage (batch 2)', () => {
  // -----------------------------------------------------------------------
  // to: .map(trim) / .filter(length > 0)
  //   - filter true  arm: entry.length > 0  → element kept
  //   - filter false arm: entry.length === 0 → element dropped
  //   - map  no-op  arm: already-trimmed string
  //   - map  strip  arm: leading/trailing/whitespace removed
  //   - zero-iteration path: empty input array
  // -----------------------------------------------------------------------
  describe('to array transformation', () => {
    it('returns an empty to array when the input to list is empty', () => {
      const result = buildEmailConfigPayload(makeUIEmailConfig({ to: [] }));
      expect(result.to).toStrictEqual([]);
    });

    it('drops every recipient when all entries are whitespace-only', () => {
      // Each entry is non-empty before trim but becomes '' after trim,
      // exercising the `entry.length > 0 === false` filter arm for
      // strings that only become empty *after* trimming.
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({ to: ['   ', '\t', '\n'] }),
      );
      expect(result.to).toStrictEqual([]);
    });

    it('strips leading-only, trailing-only, and tab-padded whitespace independently', () => {
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({
          to: [
            '  leading@test.com',
            'trailing@test.com  ',
            '\ttab@test.com\t',
          ],
        }),
      );
      expect(result.to).toStrictEqual([
        'leading@test.com',
        'trailing@test.com',
        'tab@test.com',
      ]);
    });

    it('preserves internal whitespace within recipient entries', () => {
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({ to: ['  alice bob@test.com  '] }),
      );
      expect(result.to).toStrictEqual(['alice bob@test.com']);
    });

    it('keeps a single already-trimmed entry without modification', () => {
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({ to: ['clean@test.com'] }),
      );
      expect(result.to).toStrictEqual(['clean@test.com']);
    });

    it('interleaves kept and dropped entries preserving original order', () => {
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({
          to: ['', 'keep-1@test.com', '   ', '  keep-2@test.com  ', ''],
        }),
      );
      expect(result.to).toStrictEqual(['keep-1@test.com', 'keep-2@test.com']);
    });
  });

  // -----------------------------------------------------------------------
  // Exact payload shape — toStrictEqual verifies that UI-only fields
  // (replyTo, maxRetries, retryDelay, rateLimit) are absent from the
  // output and that falsy scalar values pass through unchanged.
  // -----------------------------------------------------------------------
  describe('payload shape', () => {
    it('produces the exact EmailConfig shape with all falsy values preserved', () => {
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({
          enabled: false,
          provider: 'ses',
          server: 'email.example.com',
          port: 0,
          username: 'user@example.com',
          password: '',
          from: 'alerts@example.com',
          to: ['ops@example.com'],
          tls: false,
          startTLS: false,
          replyTo: 'reply@example.com',
          maxRetries: 5,
          retryDelay: 120,
          rateLimit: 100,
        }),
      );
      expect(result).toStrictEqual({
        enabled: false,
        provider: 'ses',
        server: 'email.example.com',
        port: 0,
        username: 'user@example.com',
        password: '',
        from: 'alerts@example.com',
        to: ['ops@example.com'],
        tls: false,
        startTLS: false,
      });
    });

    it('does not include rateLimit in the payload even when set in the UI config', () => {
      // SUSPECTED SOURCE BUG: UIEmailConfig.rateLimit is required but
      // buildEmailConfigPayload never forwards it to the backend payload.
      // See GLM_REPORT.md.
      const result = buildEmailConfigPayload(makeUIEmailConfig({ rateLimit: 100 }));
      expect(result).not.toHaveProperty('rateLimit');
    });

    it('does not include replyTo, maxRetries, or retryDelay in the payload', () => {
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({
          replyTo: 'reply@example.com',
          maxRetries: 10,
          retryDelay: 300,
        }),
      );
      expect(result).not.toHaveProperty('replyTo');
      expect(result).not.toHaveProperty('maxRetries');
      expect(result).not.toHaveProperty('retryDelay');
    });
  });

  // -----------------------------------------------------------------------
  // Scalar passthrough — verify individual fields are forwarded verbatim,
  // including edge values (zero port, empty password, empty from/server).
  // -----------------------------------------------------------------------
  describe('scalar passthrough', () => {
    it('forwards a zero port unchanged', () => {
      const result = buildEmailConfigPayload(makeUIEmailConfig({ port: 0 }));
      expect(result.port).toBe(0);
    });

    it('forwards an empty password string unchanged', () => {
      const result = buildEmailConfigPayload(makeUIEmailConfig({ password: '' }));
      expect(result.password).toBe('');
    });

    it('forwards an empty from address unchanged', () => {
      const result = buildEmailConfigPayload(makeUIEmailConfig({ from: '' }));
      expect(result.from).toBe('');
    });

    it('forwards an empty server string unchanged', () => {
      const result = buildEmailConfigPayload(makeUIEmailConfig({ server: '' }));
      expect(result.server).toBe('');
    });

    it('forwards enabled, tls, and startTLS false values unchanged', () => {
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({ enabled: false, tls: false, startTLS: false }),
      );
      expect(result.enabled).toBe(false);
      expect(result.tls).toBe(false);
      expect(result.startTLS).toBe(false);
    });

    it('forwards enabled, tls, and startTLS true values unchanged', () => {
      const result = buildEmailConfigPayload(
        makeUIEmailConfig({ enabled: true, tls: true, startTLS: true }),
      );
      expect(result.enabled).toBe(true);
      expect(result.tls).toBe(true);
      expect(result.startTLS).toBe(true);
    });
  });
});
