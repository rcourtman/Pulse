import type { LicenseCommercialPosture } from '@/api/license';

export const ACTIVE_USE_TRIAL_NUDGE_SNOOZE_KEY = 'pulse_active_use_nudge_snoozed';
export const ACTIVE_USE_TRIAL_NUDGE_FIRST_SEEN_KEY = 'pulse_first_seen_ts';
export const ACTIVE_USE_TRIAL_NUDGE_MIN_AGE_MS = 7 * 24 * 60 * 60 * 1000;
export const ACTIVE_USE_TRIAL_NUDGE_REFRESH_MS = 60 * 60 * 1000;

export const ACTIVE_USE_TRIAL_NUDGE_TITLE =
  'Experience the full power of Pulse — start your free trial';
export const ACTIVE_USE_TRIAL_NUDGE_START_LABEL = 'Start 14-day trial';
export const ACTIVE_USE_TRIAL_NUDGE_STARTING_LABEL = 'Starting...';
export const ACTIVE_USE_TRIAL_NUDGE_SNOOZE_LABEL = 'Snooze 7d';

export function isActiveUseTrialNudgeEligible(
  entitlements: LicenseCommercialPosture | null | undefined,
): boolean {
  if (!entitlements) return false;

  return (
    entitlements.tier === 'free' &&
    entitlements.subscription_state !== 'trial' &&
    entitlements.subscription_state !== 'active' &&
    entitlements.trial_eligible !== false
  );
}

export function isActiveUseTrialNudgeOldEnough(now: number, firstSeen: number): boolean {
  return now - firstSeen >= ACTIVE_USE_TRIAL_NUDGE_MIN_AGE_MS;
}
