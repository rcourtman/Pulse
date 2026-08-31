import type { VersionInfo } from '@/api/updates';
import { formatRelativeTime } from '@/utils/format';

export interface UpdateBuildBadge {
  label: string;
  className: string;
}

export const UPDATES_PANEL_COPY = {
  title: 'Pulse server updates',
  description:
    'Manage the Pulse server runtime. Pulse Agent updates are diagnosed under Infrastructure.',
  currentVersionLabel: 'Server version',
  checkNowLabel: 'Check Now',
  checkingLabel: 'Checking...',
  updatePreferencesTitle: 'Update Preferences',
  autoUpdateTitle: 'Automatic Stable Updates',
  autoUpdateDescription:
    'Supported host installs can automatically apply stable releases. Preview testing always stays manual.',
  previewChannelTitle: 'Preview builds stay on a manual channel.',
  previewChannelDescription:
    'Use beta builds for user testing and release candidates only when they may become stable without product changes. Keep preview installs on staging or internal validation environments.',
  previewChannelAutoUpdateNotice:
    'Automatic stable updates are unavailable while the Preview channel is selected.',
  // The host systemd timer owns the schedule (daily overnight window with a
  // randomized delay, see install.sh); it is not configurable from the UI.
  autoUpdateScheduleNote:
    'The host checks for stable releases once a day overnight and applies them automatically.',
} as const;

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

// The displayed verdict can come from a day-old cached check, so "Up to date"
// must carry the age of the check it is based on or it reads as a live
// comparison (#1601).
export function getUpdateCheckedLabel(lastCheckedMs: number | null | undefined): string {
  if (!lastCheckedMs || lastCheckedMs <= 0) return 'Not checked yet';
  const relative = formatRelativeTime(lastCheckedMs);
  return relative ? `Checked ${relative}` : 'Not checked yet';
}

export function getUpdateCheckActionLabel(checking: boolean): string {
  return checking ? UPDATES_PANEL_COPY.checkingLabel : UPDATES_PANEL_COPY.checkNowLabel;
}
