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

export interface TrialStartErrorLike {
  status?: number;
  code?: string;
  message?: string;
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

function normalizeTrialStartError(error?: unknown): TrialStartErrorLike | null {
  if (!error) return null;
  if (typeof error === 'string') return { message: error };
  if (typeof error !== 'object') return null;

  const value = error as TrialStartErrorLike;
  return {
    status: value.status,
    code: value.code,
    message: value.message,
  };
}

export function getTrialStartErrorMessage(
  error?: unknown,
  options: TrialStartErrorOptions = {},
): string {
  const normalized = normalizeTrialStartError(error);
  if (normalized?.code === 'trial_already_used') {
    return getTrialAlreadyUsedMessage();
  }
  if (normalized?.status === 429) {
    return getTrialTryAgainLaterMessage();
  }
  if (normalized?.message?.trim()) {
    return normalized.message.trim();
  }
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
