export const TRIAL_BANNER_SNOOZE_KEY = 'pulse_trial_banner_snoozed';
export const TRIAL_BANNER_UPGRADE_REASON = 'trial_banner';
export const TRIAL_BANNER_TITLE = 'Pro Trial:';
export const TRIAL_BANNER_ACTIVE_LABEL = 'Active';
export const TRIAL_BANNER_UPGRADE_LABEL = 'Upgrade';
export const TRIAL_BANNER_SNOOZE_LABEL = 'Snooze 7d';

export function normalizeTrialBannerDaysRemaining(raw: unknown): number | null {
  if (typeof raw !== 'number' || !Number.isFinite(raw)) return null;
  return Math.max(0, Math.floor(raw));
}

export function getTrialBannerToneClass(daysRemaining: number | null): string {
  if (daysRemaining !== null && daysRemaining <= 1) {
    return 'border-red-200 bg-red-50 text-red-900 dark:border-red-900 dark:bg-red-900 dark:text-red-100';
  }

  if (daysRemaining !== null && daysRemaining <= 3) {
    return 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-900 dark:bg-amber-900 dark:text-amber-100';
  }

  return 'border-blue-200 bg-blue-50 text-blue-900 dark:border-blue-900 dark:bg-blue-900 dark:text-blue-100';
}

export function getTrialBannerStatusLabel(daysRemaining: number | null): string {
  if (daysRemaining === null) return TRIAL_BANNER_ACTIVE_LABEL;
  return `${daysRemaining} days remaining`;
}
