export type UpgradeActionButtonTone = 'primary' | 'warning';

export interface UpgradeActionButtonOptions {
  tone?: UpgradeActionButtonTone;
  mobileFullWidth?: boolean;
}

export const UPGRADE_ACTION_LABEL = 'View plans';

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
