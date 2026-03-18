export type UpgradeActionButtonTone = 'primary' | 'warning';

export interface UpgradeActionButtonOptions {
  tone?: UpgradeActionButtonTone;
  mobileFullWidth?: boolean;
}

export const UPGRADE_ACTION_LABEL = 'Upgrade to Pro';
export const UPGRADE_TRIAL_LABEL = 'Start free trial';
export const UPGRADE_TRIAL_LINK_CLASS =
  'text-sm text-indigo-500 hover:underline disabled:opacity-50';

export interface TrialStartErrorOptions {
  branded?: boolean;
}

export function getProTrialStartedMessage(): string {
  return 'Pro trial started';
}

export function getTrialAlreadyUsedMessage(): string {
  return 'Trial already used';
}

export function getTrialTryAgainLaterMessage(): string {
  return 'Try again later';
}

export function getTrialStartErrorMessage(
  message?: string,
  options: TrialStartErrorOptions = {},
): string {
  if (message) return message;
  return options.branded ? 'Failed to start Pro trial' : 'Failed to start trial';
}

export function getUpgradeActionButtonClass(
  options: UpgradeActionButtonOptions = {},
): string {
  const { tone = 'primary', mobileFullWidth = true } = options;
  const widthClass = mobileFullWidth ? 'w-full sm:w-auto' : 'w-auto';

  if (tone === 'warning') {
    return `${widthClass} inline-flex min-h-10 sm:min-h-9 items-center justify-center gap-2 rounded-md border border-amber-300 bg-amber-100 px-4 py-2 text-sm font-medium text-amber-800 transition-colors hover:bg-amber-200 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-100 dark:hover:bg-amber-800`;
  }

  return `${widthClass} inline-flex min-h-10 sm:min-h-9 items-center justify-center gap-2 rounded-md bg-blue-600 px-5 py-2.5 text-center text-sm font-semibold text-white transition-colors hover:bg-blue-700`;
}
