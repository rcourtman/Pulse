import { describe, expect, it } from 'vitest';

import {
  type AuditVerificationStatus,
  getAuditEventStatusPresentation,
  getAuditEventTypeBadgeClass,
  getAuditLogEmptyState,
  getAuditLogFetchErrorMessage,
  getAuditLogFeatureGateCopy,
  getAuditLogLoadingState,
  getAuditVerificationBadgePresentation,
} from '@/utils/auditLogPresentation';

// Supplemental branch-coverage for `auditLogPresentation.ts`.
//
// `getAuditEventStatusPresentation` is already exhaustively covered by the
// sibling `auditLogPresentation.branchcov0712.test.ts` (both ternary arms,
// truthiness coercion, exact icon references, and enumerable-key shape), so it
// is intentionally not re-tested here. This file targets the remaining
// uncovered branches of the other six exported functions.

describe('getAuditEventTypeBadgeClass (branch coverage)', () => {
  it('matches known event types after trimming surrounding whitespace', () => {
    // Exercises the `(event ?? '').trim()` normalisation: padded input must
    // still resolve to the canonical case arm.
    expect(getAuditEventTypeBadgeClass('  login  ')).toBe(
      'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
    );
    expect(getAuditEventTypeBadgeClass('\tconfig_change\n')).toBe(
      'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
    );
    expect(getAuditEventTypeBadgeClass(' startup ')).toBe(
      'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
    );
  });

  it('routes the explicit oidc_token_refresh fallthrough case to the default badge', () => {
    // `case 'oidc_token_refresh':` has no body and falls through to default.
    expect(getAuditEventTypeBadgeClass('oidc_token_refresh')).toBe(
      'bg-surface-alt text-base-content',
    );
  });

  it('routes an unrecognised event type through the default arm', () => {
    expect(getAuditEventTypeBadgeClass('something_new')).toBe('bg-surface-alt text-base-content');
  });

  it('coerces undefined to an empty trimmed string and lands on the default arm', () => {
    expect(getAuditEventTypeBadgeClass(undefined)).toBe('bg-surface-alt text-base-content');
  });

  it('honours the nullish-coalesce fallback for an explicit null argument', () => {
    // `(null ?? '').trim()` -> '' -> default case.
    expect(getAuditEventTypeBadgeClass(null)).toBe('bg-surface-alt text-base-content');
  });

  it('treats whitespace-only input as empty after trimming', () => {
    expect(getAuditEventTypeBadgeClass('    ')).toBe('bg-surface-alt text-base-content');
  });

  it('keeps the three coloured badges distinct from the default surface badge', () => {
    const login = getAuditEventTypeBadgeClass('login');
    const config = getAuditEventTypeBadgeClass('config_change');
    const startup = getAuditEventTypeBadgeClass('startup');
    const fallback = getAuditEventTypeBadgeClass('logout');
    expect(new Set([login, config, startup, fallback]).size).toBe(4);
  });
});

describe('getAuditVerificationBadgePresentation (branch coverage)', () => {
  it('returns the Not-checked badge for an explicit null state', () => {
    // The `!state` guard arm with null rather than undefined.
    expect(getAuditVerificationBadgePresentation(null)).toStrictEqual({
      label: 'Not checked',
      className: 'bg-surface-alt text-base-content',
    });
  });

  it('returns the Error badge for the error status case', () => {
    expect(getAuditVerificationBadgePresentation({ status: 'error' })).toStrictEqual({
      label: 'Error',
      className: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
    });
  });

  it('returns the Unavailable badge for the unavailable status (default arm)', () => {
    // 'unavailable' is a valid AuditVerificationStatus but has no explicit case,
    // so it routes through the switch default.
    expect(getAuditVerificationBadgePresentation({ status: 'unavailable' })).toStrictEqual({
      label: 'Unavailable',
      className: 'bg-surface-alt text-base-content',
    });
  });

  it('routes a malformed status string through the default arm to Unavailable', () => {
    const bogus = { status: 'definitely-not-a-status' as unknown as AuditVerificationStatus };
    expect(getAuditVerificationBadgePresentation(bogus)).toStrictEqual({
      label: 'Unavailable',
      className: 'bg-surface-alt text-base-content',
    });
  });

  it('distinguishes the three outcome colours while sharing one neutral class', () => {
    const notChecked = getAuditVerificationBadgePresentation(undefined);
    const unavailable = getAuditVerificationBadgePresentation({ status: 'unavailable' });
    const verified = getAuditVerificationBadgePresentation({ status: 'verified' });
    const failed = getAuditVerificationBadgePresentation({ status: 'failed' });
    const errored = getAuditVerificationBadgePresentation({ status: 'error' });

    // The verified/failed/error outcomes each carry a distinct colour class.
    expect(new Set([verified.className, failed.className, errored.className]).size).toBe(3);
    // The "Not checked" (no state) and "Unavailable" (default arm) fallbacks
    // intentionally reuse the same neutral surface-alt class but keep
    // different labels.
    expect(notChecked.className).toBe(unavailable.className);
    expect(notChecked.className).toBe('bg-surface-alt text-base-content');
    expect(notChecked.label).toBe('Not checked');
    expect(unavailable.label).toBe('Unavailable');
    // Every coloured class differs from the neutral fallback.
    for (const coloured of [verified.className, failed.className, errored.className]) {
      expect(coloured).not.toBe(notChecked.className);
    }
  });
});

describe('getAuditLogLoadingState (branch coverage)', () => {
  it('returns the canonical loading copy as a readonly literal', () => {
    expect(getAuditLogLoadingState()).toStrictEqual({ text: 'Loading audit events…' });
  });

  it('always returns the same singleton shape across calls (deterministic)', () => {
    expect(getAuditLogLoadingState()).toEqual(getAuditLogLoadingState());
  });

  it('exposes only the text key', () => {
    expect(Object.keys(getAuditLogLoadingState())).toStrictEqual(['text']);
  });
});

describe('getAuditLogFetchErrorMessage (branch coverage)', () => {
  it('maps a bare HTTP 500 status (no message) to the internal-error copy', () => {
    // Exercises the `errorLike?.status === 500` operand of the || independently
    // of the message-regex operand.
    expect(getAuditLogFetchErrorMessage({ status: 500 })).toBe(
      'Audit events could not be loaded because the server returned an internal error. Check the server logs, then refresh the audit log.',
    );
  });

  it('maps a matching message to the internal-error copy even without a 500 status', () => {
    // Regex operand of the || is true while status operand is absent.
    expect(
      getAuditLogFetchErrorMessage({
        message: 'failed to fetch audit events: internal server error',
      }),
    ).toBe(
      'Audit events could not be loaded because the server returned an internal error. Check the server logs, then refresh the audit log.',
    );
  });

  it('tolerates arbitrary whitespace between the prefix and "internal server error" via \\s*', () => {
    // The regex uses `\s*`; multiple spaces must still match.
    expect(
      getAuditLogFetchErrorMessage({
        message: 'Failed to fetch audit events:   internal server error',
      }),
    ).toBe(
      'Audit events could not be loaded because the server returned an internal error. Check the server logs, then refresh the audit log.',
    );
  });

  it('uses strict equality for status, so a numeric-looking string does not trigger the 500 arm', () => {
    // `'500' === 500` is false, and the message does not match the regex, so it
    // falls through to the Unknown-error fallback.
    expect(getAuditLogFetchErrorMessage({ status: '500' })).toBe('Unknown error');
  });

  it('returns the underlying message for a generic Error that matches no known code/regex', () => {
    // `error instanceof Error` -> error.message arm.
    expect(getAuditLogFetchErrorMessage(new Error('network unreachable'))).toBe(
      'network unreachable',
    );
  });

  it('returns "Unknown error" for a non-Error primitive value', () => {
    expect(getAuditLogFetchErrorMessage('boom')).toBe('Unknown error');
  });

  it('returns "Unknown error" for null', () => {
    // null is not an Error and exposes no matching code/status/message.
    expect(getAuditLogFetchErrorMessage(null)).toBe('Unknown error');
  });

  it('returns "Unknown error" for undefined', () => {
    expect(getAuditLogFetchErrorMessage(undefined)).toBe('Unknown error');
  });

  it('returns "Unknown error" for a plain object with no recognised fields', () => {
    expect(getAuditLogFetchErrorMessage({})).toBe('Unknown error');
  });

  it('returns "Unknown error" for an object whose code/status are unrecognised', () => {
    expect(
      getAuditLogFetchErrorMessage({ code: 'something_else', status: 404, message: 'mystery' }),
    ).toBe('Unknown error');
  });

  it('prefers the code-based arms over the status-based arm when both are set', () => {
    // A busy code with a 500 status must return the busy copy, proving the
    // code checks short-circuit before the status/regex block.
    expect(getAuditLogFetchErrorMessage({ code: 'audit_store_busy', status: 500 })).toBe(
      'Audit log storage is busy. Wait a moment, then refresh the audit log.',
    );
  });
});

describe('getAuditLogFeatureGateCopy (branch coverage)', () => {
  it('returns the default Audit Logging copy when called with no options', () => {
    // Exercises the default parameter value `{}` and the falsy arm.
    expect(getAuditLogFeatureGateCopy()).toStrictEqual({
      title: 'Audit Logging',
      body: 'Persistent, searchable audit logs with cryptographic signature verification.',
    });
  });

  it('returns the default copy for an empty options object', () => {
    expect(getAuditLogFeatureGateCopy({})).toStrictEqual({
      title: 'Audit Logging',
      body: 'Persistent, searchable audit logs with cryptographic signature verification.',
    });
  });

  it('falls back to the default copy when paidRuntimeRequired is explicitly false', () => {
    expect(getAuditLogFeatureGateCopy({ paidRuntimeRequired: false })).toStrictEqual({
      title: 'Audit Logging',
      body: 'Persistent, searchable audit logs with cryptographic signature verification.',
    });
  });

  it('returns the Pro-runtime copy when paidRuntimeRequired is truthy', () => {
    expect(getAuditLogFeatureGateCopy({ paidRuntimeRequired: true })).toStrictEqual({
      title: 'Pulse Pro runtime required',
      body: 'Your Pro license is active, but this install is running the community runtime. Install the private Pulse Pro runtime to use Audit Log and Audit Webhooks. Public GitHub releases and the public Docker image do not include those Pro runtime hooks.',
    });
  });

  it('produces a distinct title for the two arms', () => {
    const paid = getAuditLogFeatureGateCopy({ paidRuntimeRequired: true }).title;
    const community = getAuditLogFeatureGateCopy({ paidRuntimeRequired: false }).title;
    expect(paid).not.toBe(community);
  });
});

describe('getAuditLogEmptyState (branch coverage)', () => {
  it('returns the filtered description when at least one filter is active', () => {
    expect(getAuditLogEmptyState(1)).toStrictEqual({
      title: 'No audit events found',
      description: 'No events match your current filters. Try adjusting or clearing them.',
    });
  });

  it('returns the filtered description for a large filter count', () => {
    expect(getAuditLogEmptyState(42).description).toBe(
      'No events match your current filters. Try adjusting or clearing them.',
    );
  });

  it('returns the unfiltered description when no filters are active (boundary zero)', () => {
    expect(getAuditLogEmptyState(0)).toStrictEqual({
      title: 'No audit events found',
      description: 'Audit logging is active, but no events have been recorded yet.',
    });
  });

  it('treats a negative count as no-filters (the > 0 guard is false)', () => {
    // Documents the guard's strict inequality: negatives route to the unfiltered arm.
    expect(getAuditLogEmptyState(-3).description).toBe(
      'Audit logging is active, but no events have been recorded yet.',
    );
  });

  it('shares the same title across both arms but diverges on the description', () => {
    const filtered = getAuditLogEmptyState(2);
    const unfiltered = getAuditLogEmptyState(0);
    expect(filtered.title).toBe(unfiltered.title);
    expect(filtered.description).not.toBe(unfiltered.description);
  });
});

describe('getAuditEventStatusPresentation (already covered — guard)', () => {
  it('is exhaustively covered by the sibling branchcov0712 suite; both arms remain stable', () => {
    // Sanity guard so this target function is represented without duplicating
    // the sibling suite's detailed icon-reference / truthiness assertions.
    expect(getAuditEventStatusPresentation(true).className).toBe('w-4 h-4 text-emerald-400');
    expect(getAuditEventStatusPresentation(false).className).toBe('w-4 h-4 text-rose-400');
  });
});
