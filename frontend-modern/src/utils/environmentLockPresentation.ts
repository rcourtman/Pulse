export const ENVIRONMENT_LOCK_BADGE_CLASS =
  'shrink-0 inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300';

export const ENVIRONMENT_LOCK_BADGE_LABEL = 'ENV';

export function getEnvironmentLockTitle(envVar: string): string {
  return `Locked by environment variable ${envVar}`;
}

export const ENVIRONMENT_LOCK_BUTTON_TITLE = 'Locked by environment variable';
