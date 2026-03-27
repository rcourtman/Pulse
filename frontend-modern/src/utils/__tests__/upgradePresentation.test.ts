import { describe, expect, it } from 'vitest';
import {
  getTrialStartErrorKind,
  getProTrialStartedMessage,
  getTrialAlreadyUsedMessage,
  getTrialStartErrorMessage,
  normalizeTrialStartError,
  getTrialTryAgainLaterMessage,
  getUpgradeActionButtonClass,
  UPGRADE_ACTION_LABEL,
  UPGRADE_TRIAL_LABEL,
  UPGRADE_TRIAL_LINK_CLASS,
} from '@/utils/upgradePresentation';

describe('upgradePresentation', () => {
  it('returns canonical upgrade labels', () => {
    expect(UPGRADE_ACTION_LABEL).toBe('Upgrade to Pro');
    expect(UPGRADE_TRIAL_LABEL).toBe('Start free trial');
    expect(UPGRADE_TRIAL_LINK_CLASS).toContain('text-indigo-500');
  });

  it('returns canonical trial start messages', () => {
    expect(getProTrialStartedMessage()).toBe('Pro trial started');
    expect(getTrialAlreadyUsedMessage()).toBe('Trial already used');
    expect(getTrialTryAgainLaterMessage()).toBe('Try again later');
    expect(getTrialTryAgainLaterMessage(30)).toBe('Try again in about a minute');
    expect(getTrialTryAgainLaterMessage(90)).toBe('Try again in about 2 minutes');
    expect(getTrialTryAgainLaterMessage(3600)).toBe('Try again in about 1 hour');
    expect(getTrialStartErrorMessage()).toBe('Failed to start trial');
    expect(getTrialStartErrorMessage(undefined, { branded: true })).toBe(
      'Failed to start Pro trial',
    );
    expect(getTrialStartErrorMessage('temporary failure')).toBe('temporary failure');
    expect(getTrialStartErrorMessage({ code: 'trial_already_used' })).toBe('Trial already used');
    expect(getTrialStartErrorMessage({ status: 429 })).toBe('Try again later');
    expect(getTrialStartErrorMessage({ status: 429, retryAfterSeconds: 120 })).toBe(
      'Try again in about 2 minutes',
    );
    expect(getTrialStartErrorKind({ code: 'trial_already_used' })).toBe('already_used');
    expect(getTrialStartErrorKind({ status: 429 })).toBe('retry_later');
    expect(getTrialStartErrorKind({ message: 'temporary failure' })).toBe('other');
    expect(normalizeTrialStartError({ status: 409, code: 'trial_not_available' })).toEqual({
      status: 409,
      code: 'trial_not_available',
      message: undefined,
      retryAfterSeconds: undefined,
    });
    expect(
      getTrialStartErrorMessage({
        code: 'trial_not_available',
        message: 'Trial cannot be started while migration is pending',
      }),
    ).toBe('Trial cannot be started while migration is pending');
  });

  it('returns the primary upgrade action button classes by default', () => {
    const classes = getUpgradeActionButtonClass();
    expect(classes).toContain('bg-blue-600');
    expect(classes).toContain('w-full sm:w-auto');
    expect(classes).toContain('text-white');
  });

  it('returns the warning upgrade action button classes when requested', () => {
    const classes = getUpgradeActionButtonClass({ tone: 'warning', mobileFullWidth: false });
    expect(classes).toContain('border-amber-300');
    expect(classes).toContain('bg-amber-100');
    expect(classes).toContain('w-auto');
  });
});
