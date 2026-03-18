import type { VersionInfo } from '@/api/updates';

export interface UpdateBuildBadge {
  label: string;
  className: string;
}

export function getUpdateBuildBadges(
  versionInfo?: Pick<VersionInfo, 'isDevelopment' | 'isDocker' | 'isSourceBuild'> | null,
): UpdateBuildBadge[] {
  if (!versionInfo) return [];

  const badges: UpdateBuildBadge[] = [];

  if (versionInfo.isDevelopment) {
    badges.push({
      label: 'Development',
      className:
        'inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
    });
  }

  if (versionInfo.isDocker) {
    badges.push({
      label: 'Docker',
      className:
        'inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
    });
  }

  if (versionInfo.isSourceBuild) {
    badges.push({
      label: 'Source',
      className:
        'inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full bg-surface-alt text-base-content',
    });
  }

  return badges;
}

export function getUpdateAvailabilityHeading(available: boolean): string {
  return available ? 'Available' : 'Status';
}

export function getUpdatePrimaryStatusLabel(available: boolean): string {
  return available ? 'Update Ready' : 'Up to date';
}

export function getUpdateCheckModeLabel(enabled: boolean): string {
  return enabled ? 'Auto-check enabled' : 'Manual checks only';
}
