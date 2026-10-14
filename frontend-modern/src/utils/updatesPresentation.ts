import type { ProRuntimeSwitch, VersionInfo } from '@/api/updates';
import { formatRelativeTime } from '@/utils/format';

export interface UpdateBuildBadge {
  label: string;
  className: string;
}

export const UPDATES_PANEL_COPY = {
  title: 'Updates',
  description: 'Keep Pulse up to date. Agent updates are diagnosed under Infrastructure.',
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
        'inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full bg-amber-100 text-amber-700 dark:bg-amber-900/25 dark:text-amber-300',
    });
  }

  if (versionInfo.isDocker) {
    badges.push({
      label: 'Docker',
      className:
        'inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/25 dark:text-blue-300',
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

export function getUpdatePrimaryStatusLabel(available: boolean, sourceBuild = false): string {
  if (sourceBuild) return 'Source build';
  return available ? 'Update Ready' : 'Up to date';
}

export function getUpdateCheckModeLabel(enabled: boolean, sourceBuild = false): string {
  if (sourceBuild) return 'Release updates disabled';
  return enabled ? 'Auto-check enabled' : 'Manual checks only';
}

// The displayed verdict can come from a day-old cached check, so "Up to date"
// must carry the age of the check it is based on or it reads as a live
// comparison (#1601).
export function getUpdateCheckedLabel(
  lastCheckedMs: number | null | undefined,
  nowMs = Date.now(),
): string {
  if (!lastCheckedMs || lastCheckedMs <= 0) return 'Not checked yet';
  const relative = formatRelativeTime(lastCheckedMs, { now: nowMs });
  return relative ? `Checked ${relative}` : 'Not checked yet';
}

export function getUpdateCheckActionLabel(checking: boolean): string {
  return checking ? UPDATES_PANEL_COPY.checkingLabel : UPDATES_PANEL_COPY.checkNowLabel;
}

// Older servers serialize an unknown release date as Go's zero time.
export function formatUpdateReleaseDate(value?: string): string | undefined {
  if (!value || value.startsWith('0001-01-01')) return undefined;
  const date = new Date(value);
  return Number.isFinite(date.getTime()) ? date.toLocaleDateString() : undefined;
}

export interface ProRuntimeSwitchStep {
  id: string;
  title: string;
  command?: string;
  note?: string;
}

export const PULSE_PRO_DOWNLOADS_URL = 'https://pulserelay.pro/download.html';

// What the Updates page shows a community install about the Pulse Pro runtime.
// Only installs the license server would serve Pro downloads to see anything;
// every other reason stays hidden, and the License panel keeps its own notice.
export type ProRuntimeSwitchPresentation =
  | { kind: 'hidden' }
  | { kind: 'in_place'; title: string; body: string; version: string }
  | { kind: 'docker'; title: string; body: string; steps: ProRuntimeSwitchStep[]; note: string }
  | { kind: 'older_release'; title: string; body: string }
  | { kind: 'unavailable'; body: string };

const SWITCH_TITLE = 'Your Pro license needs the Pulse Pro build';
const LOCKED_FEATURES = 'Patrol modes, Audit Log, RBAC and Reporting stay locked';

const versionLabel = (version: string) => (version.startsWith('v') ? version : `v${version}`);

export function getProRuntimeSwitchPresentation(
  result: ProRuntimeSwitch | null | undefined,
): ProRuntimeSwitchPresentation {
  if (!result) {
    return { kind: 'hidden' };
  }
  const version = result.version ? versionLabel(result.version) : '';

  if (result.available && result.installInPlace && version) {
    return {
      kind: 'in_place',
      title: SWITCH_TITLE,
      body: `This install runs the community build, so ${LOCKED_FEATURES}. Switching installs Pulse Pro ${version} here: Pulse backs up the current build, restarts, and keeps your settings and history.`,
      version,
    };
  }

  if (result.available && result.docker) {
    const docker = result.docker;
    const steps: ProRuntimeSwitchStep[] = [];
    if (docker.loginCommand) {
      steps.push({
        id: 'pro-runtime-login',
        title: 'Sign in to the Pulse Pro registry',
        command: docker.loginCommand,
        note: 'Replace <activation-key> with your activation key (it starts ppk_live_).',
      });
    }
    steps.push(
      {
        id: 'pro-runtime-pull',
        title: 'Pull the Pulse Pro image',
        command: docker.composePullCommand,
      },
      {
        id: 'pro-runtime-up',
        title: 'Recreate the Pulse container',
        command: docker.composeUpCommand,
      },
    );
    return {
      kind: 'docker',
      title: SWITCH_TITLE,
      body: `This container runs the community image, so ${LOCKED_FEATURES}. Run these commands on the Docker host, in the folder with your docker-compose.yml, to switch to Pulse Pro ${versionLabel(docker.version)}. Your data volume is kept.`,
      steps,
      note: `The commands set PULSE_IMAGE, which the Pulse docker-compose.yml reads. If your compose file names rcourtman/pulse directly, change that line to image: \${PULSE_IMAGE} first. For docker run installs, use ${PULSE_PRO_DOWNLOADS_URL}.`,
    };
  }

  if (result.reason === 'older_release' && version) {
    const current = versionLabel(result.currentVersion);
    return {
      kind: 'older_release',
      title: 'The Pulse Pro build is not out for this version yet',
      body: `Your Pro license needs the Pulse Pro build, but the latest one (${version}) is older than this install (${current}). The switch appears here once Pulse Pro ${current} or later is published.`,
    };
  }

  if (result.reason === 'unavailable') {
    return {
      kind: 'unavailable',
      body: `Could not check for the Pulse Pro build. Try again later, or download it from ${PULSE_PRO_DOWNLOADS_URL}.`,
    };
  }

  return { kind: 'hidden' };
}
