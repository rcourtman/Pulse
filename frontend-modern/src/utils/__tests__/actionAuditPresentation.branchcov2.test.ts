import { describe, expect, it } from 'vitest';

import {
  formatActionApprovalPolicyLabel,
  formatActionCapabilityLabel,
  getActionAuditRecordStatePresentation,
  getActionAuditRefusalPresentation,
  getActionAuditResultPresentation,
  getActionAuditStatePresentation,
  getActionAuditVerification,
  getActionAuditVerificationOutcomePresentation,
  shouldRenderActionAuditVerification,
} from '@/utils/actionAuditPresentation';

// `getActionAuditResultMessage` is module-private (not exported); its branches
// are exercised transitively through the failure path of
// `getActionAuditResultPresentation`, which is the only caller.

describe('actionAuditPresentation branch coverage (supplemental)', () => {
  describe('getActionAuditStatePresentation', () => {
    it('resolves every canonical lifecycle state that the sibling test omits', () => {
      expect(getActionAuditStatePresentation('planned')).toStrictEqual({
        label: 'Planned',
        className: 'bg-surface-alt text-muted border-border',
      });
      expect(getActionAuditStatePresentation('approved')).toStrictEqual({
        label: 'Approved',
        className:
          'bg-blue-100 text-blue-800 border-blue-200 dark:bg-blue-900 dark:text-blue-200 dark:border-blue-700',
      });
      expect(getActionAuditStatePresentation('rejected')).toStrictEqual({
        label: 'Rejected',
        className:
          'bg-red-100 text-red-800 border-red-200 dark:bg-red-900 dark:text-red-200 dark:border-red-700',
      });
      expect(getActionAuditStatePresentation('executing')).toStrictEqual({
        label: 'Executing',
        className:
          'bg-sky-100 text-sky-800 border-sky-200 dark:bg-sky-900 dark:text-sky-200 dark:border-sky-700',
      });
    });

    it('falls back to the Unknown badge for undefined and unrecognised states (?? arm)', () => {
      const unknown = {
        label: 'Unknown',
        className: 'bg-surface-alt text-muted border-border',
      };
      expect(getActionAuditStatePresentation(undefined)).toStrictEqual(unknown);
      expect(getActionAuditStatePresentation('frobnicated')).toStrictEqual(unknown);
    });
  });

  describe('formatActionCapabilityLabel', () => {
    it('returns the bare "Action" fallback for nullish and whitespace-only input', () => {
      // `(capabilityName || '')` short-circuit on undefined, then trim -> empty -> early return.
      expect(formatActionCapabilityLabel(undefined)).toBe('Action');
      expect(formatActionCapabilityLabel('   ')).toBe('Action');
    });

    it('title-cases a single lowercase word without touching separators', () => {
      expect(formatActionCapabilityLabel('reboot')).toBe('Reboot');
    });

    it('splits on hyphen runs the same way as underscores and dots', () => {
      // Exercises the `/[._-]+/g` replace arm for the hyphen class.
      expect(formatActionCapabilityLabel('restart-service')).toBe('Restart Service');
    });

    it('collapses consecutive separators and trims surrounding whitespace', () => {
      // `a__b` -> replace -> `a  b` -> split(/\s+/) -> ['a','b'].
      expect(formatActionCapabilityLabel('  a__b  ')).toBe('A B');
    });

    it('only re-cases the first character of each word, leaving the tail verbatim', () => {
      // Documents the behaviour: slice(0,1).toUpperCase() + slice(1) preserves the
      // rest verbatim, so 'updateContainer' becomes 'UpdateContainer' (camelCase
      // tail is kept, not lowercased).
      expect(formatActionCapabilityLabel('docker.updateContainer')).toBe(
        'Docker UpdateContainer',
      );
    });
  });

  describe('formatActionApprovalPolicyLabel', () => {
    it('maps the "none" policy arm the sibling test skips', () => {
      expect(formatActionApprovalPolicyLabel('none')).toBe('No approval');
    });

    it('trims before matching, so a padded canonical value still hits its case', () => {
      expect(formatActionApprovalPolicyLabel('  none  ')).toBe('No approval');
    });

    it('routes nullish/empty policy through the default arm onto "Policy"', () => {
      // default -> formatActionCapabilityLabel(policy || 'Policy').
      expect(formatActionApprovalPolicyLabel(undefined)).toBe('Policy');
      expect(formatActionApprovalPolicyLabel('')).toBe('Policy');
    });

    it('title-cases an unknown policy name via the default arm', () => {
      expect(formatActionApprovalPolicyLabel('break_glass')).toBe('Break Glass');
      expect(formatActionApprovalPolicyLabel('custom-policy')).toBe('Custom Policy');
    });
  });

  describe('getActionAuditResultMessage (via getActionAuditResultPresentation failure path)', () => {
    it('uses result.output when errorMessage is absent (|| arm)', () => {
      // Private helper branch: `result?.errorMessage || result?.output`.
      const presentation = getActionAuditResultPresentation({
        result: { success: false, output: 'stderr dump from executor' },
      });
      expect(presentation).toStrictEqual({
        kind: 'failure',
        label: 'Execution failed',
        detail: 'stderr dump from executor',
        className:
          'border-red-200 bg-red-50 text-red-800 dark:border-red-800 dark:bg-red-950/40 dark:text-red-300',
      });
    });

    it('returns an empty message when neither errorMessage nor output is set', () => {
      // Final `|| ''` fallback inside the helper, then `|| undefined` on detail.
      const presentation = getActionAuditResultPresentation({
        result: { success: false },
      });
      expect(presentation).toStrictEqual({
        kind: 'failure',
        label: 'Execution failed',
        detail: undefined,
        className:
          'border-red-200 bg-red-50 text-red-800 dark:border-red-800 dark:bg-red-950/40 dark:text-red-300',
      });
    });
  });

  describe('getActionAuditRefusalPresentation', () => {
    it('returns undefined when there is no result at all (!audit.result arm)', () => {
      expect(getActionAuditRefusalPresentation({})).toBeUndefined();
    });

    it('returns undefined when the result was successful (audit.result.success arm)', () => {
      expect(
        getActionAuditRefusalPresentation({ result: { success: true, output: 'ok' } }),
      ).toBeUndefined();
    });

    it('matches a refusal prefix case-insensitively via the toLowerCase() normaliser', () => {
      // normalizedMessage = message.toLowerCase(); 'PLAN_DRIFT:' lowercases to the prefix.
      const presentation = getActionAuditRefusalPresentation({
        result: { success: false, errorMessage: 'PLAN_DRIFT: policy version changed' },
      });
      expect(presentation).toStrictEqual({
        prefix: 'plan_drift:',
        label: 'Plan changed',
        detail:
          'Pulse refused the action before dispatch because the approved plan no longer matched the current resource or policy state.',
        recordedDetail: 'policy version changed',
        className:
          'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300',
      });
    });

    it('drops recordedDetail when the message has no detail after the prefix', () => {
      // `recordedDetail || undefined` falsy arm.
      const presentation = getActionAuditRefusalPresentation({
        result: { success: false, errorMessage: 'plan_drift:' },
      });
      expect(presentation?.recordedDetail).toBeUndefined();
      expect(presentation?.label).toBe('Plan changed');
    });

    it('returns undefined when the failure message carries no recognised prefix', () => {
      // ACTION_REFUSAL_PREFIXES.find() returns undefined -> second early return.
      expect(
        getActionAuditRefusalPresentation({
          result: { success: false, errorMessage: 'totally unknown failure mode' },
        }),
      ).toBeUndefined();
    });
  });

  describe('getActionAuditRecordStatePresentation', () => {
    it('keeps the generic Failed badge when a failed result is not a refusal', () => {
      // state === 'failed' && refusal -> falsey refusal -> falls through to state lookup.
      expect(
        getActionAuditRecordStatePresentation({
          state: 'failed',
          result: { success: false, errorMessage: 'executor exited with code 1' },
        }),
      ).toStrictEqual({
        label: 'Failed',
        className:
          'bg-red-100 text-red-800 border-red-200 dark:bg-red-900 dark:text-red-200 dark:border-red-700',
      });
    });

    it('delegates to the state lookup for non-failed states', () => {
      // state !== 'failed' -> skips the refusal check entirely.
      expect(
        getActionAuditRecordStatePresentation({ state: 'executing' }),
      ).toStrictEqual({
        label: 'Executing',
        className:
          'bg-sky-100 text-sky-800 border-sky-200 dark:bg-sky-900 dark:text-sky-200 dark:border-sky-700',
      });
    });
  });

  describe('getActionAuditResultPresentation', () => {
    it('returns undefined when the record has no result (!result arm)', () => {
      expect(getActionAuditResultPresentation({})).toBeUndefined();
    });

    it('renders the success branch with a trimmed output detail', () => {
      const presentation = getActionAuditResultPresentation({
        result: { success: true, output: '  reloaded  ' },
      });
      expect(presentation).toStrictEqual({
        kind: 'success',
        label: 'Result',
        detail: 'reloaded',
        className: 'border-border bg-surface text-base-content',
      });
    });

    it('omits detail on the success branch when output is missing or blank', () => {
      // `result.output?.trim() || undefined` -> undefined for absent and whitespace-only.
      expect(
        getActionAuditResultPresentation({ result: { success: true } })?.detail,
      ).toBeUndefined();
      expect(
        getActionAuditResultPresentation({ result: { success: true, output: '   ' } })
          ?.detail,
      ).toBeUndefined();
    });
  });

  describe('getActionAuditVerificationOutcomePresentation', () => {
    it('returns undefined when there is no verification outcome (!status arm)', () => {
      // outcome undefined -> status '' -> early return.
      expect(getActionAuditVerificationOutcomePresentation({})).toBeUndefined();
      expect(
        getActionAuditVerificationOutcomePresentation({
          verificationOutcome: { status: '' },
        }),
      ).toBeUndefined();
      // whitespace-only status trims to '' and also returns undefined.
      expect(
        getActionAuditVerificationOutcomePresentation({
          verificationOutcome: { status: '   ' },
        }),
      ).toBeUndefined();
    });

    it('normalises uppercase status to the canonical map key via toLowerCase()', () => {
      expect(
        getActionAuditVerificationOutcomePresentation({
          verificationOutcome: { status: 'VERIFIED' },
        }),
      ).toStrictEqual({
        label: 'Verification confirmed',
        detail: 'Pulse confirmed the intended state after execution.',
        evidenceSummary: undefined,
        className:
          'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300',
      });
    });

    it('collapses a whitespace-only evidenceSummary to undefined', () => {
      // `evidenceSummary?.trim()` -> '' -> `|| undefined`.
      const presentation = getActionAuditVerificationOutcomePresentation({
        verificationOutcome: { status: 'failed', evidenceSummary: '   ' },
      });
      expect(presentation?.evidenceSummary).toBeUndefined();
      expect(presentation?.label).toBe('Verification failed');
    });
  });

  describe('getActionAuditVerification', () => {
    it('returns undefined when neither top-level nor result verification exists', () => {
      // `audit.verification ?? audit.result?.verification` -> undefined ?? undefined.
      expect(getActionAuditVerification({})).toBeUndefined();
      expect(getActionAuditVerification({ result: { success: true } })).toBeUndefined();
    });

    it('prefers the top-level verification over the result-embedded one (?? left operand)', () => {
      const top = { ran: true, success: true, command: 'top-level' };
      const result = { ran: false, success: false, command: 'embedded' };
      expect(
        getActionAuditVerification({
          verification: top,
          result: { success: true, verification: result },
        }),
      ).toStrictEqual(top);
    });
  });

  describe('shouldRenderActionAuditVerification', () => {
    it('returns false when no verification object exists at all', () => {
      // `undefined?.ran === true` -> false.
      expect(shouldRenderActionAuditVerification({})).toBe(false);
    });

    it('returns true from the result-embedded fallback verification', () => {
      // Exercises the ?? right operand of getActionAuditVerification plus the true arm.
      expect(
        shouldRenderActionAuditVerification({
          result: { success: true, verification: { ran: true, success: true } },
        }),
      ).toBe(true);
    });

    it('uses strict equality against the boolean true (truthy non-boolean does not count)', () => {
      // `ran` is typed boolean; we deliberately coerce a non-boolean to prove the
      // `=== true` strictness rather than a truthiness check.
      expect(
        shouldRenderActionAuditVerification({
          result: {
            success: true,
            verification: {
              ran: 1 as unknown as boolean,
              success: true,
            },
          },
        }),
      ).toBe(false);
    });
  });
});
