import type { ProRuntimeSwitch } from '@/api/updates';

export interface ProRuntimeSwitchStep {
  id: string;
  title: string;
  command?: string;
  note?: string;
}

export const PULSE_PRO_DOWNLOADS_URL = 'https://pulserelay.pro/download.html';

export const PRO_RUNTIME_SWITCH_REQUEST_COPY = {
  accepted: 'Switch requested. Follow update progress or check Update history for the result.',
  unknown:
    'Could not confirm whether the switch started. Check update progress and Update history before trying again.',
} as const;

// What the Updates page shows a community install about the Pulse Pro runtime.
// Only installs the license server would serve Pro downloads to see anything;
// every other reason stays hidden, and the License panel keeps its own notice.
export type ProRuntimeSwitchPresentation =
  | { kind: 'hidden' }
  | { kind: 'in_place'; title: string; body: string; version: string }
  | { kind: 'docker'; title: string; body: string; steps: ProRuntimeSwitchStep[]; note: string }
  | { kind: 'older_release'; title: string; body: string }
  | { kind: 'prerelease_only'; title: string; body: string }
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

  // The only Pulse Pro build on offer is a preview, and this install follows
  // the stable channel, which never installs previews.
  if (result.reason === 'prerelease_only' && version) {
    return {
      kind: 'prerelease_only',
      title: 'The stable Pulse Pro build is not out yet',
      body: `Your Pro license needs the Pulse Pro build, but the only one available right now (${version}) is a preview release, and this install only takes stable releases. The switch appears here once a stable Pulse Pro release is published.`,
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
