import type { DockerUpdateCommands, UpdateInfo, UpdatePlan, VersionInfo } from '@/api/updates';

export type UpdateChannelOptionValue = 'stable' | 'rc';

export interface UpdateChannelCardOption {
  value: UpdateChannelOptionValue;
  title: string;
  description: string;
  tone: 'success' | 'accent';
  disabled?: boolean;
}

export interface UpdateInstallStep {
  id: string;
  title: string;
  command?: string;
  note?: string;
  commandCodeClass?: string;
}

export interface UpdateInstallGuide {
  headerTitle: string;
  headerSummary: string;
  introText: string;
  steps: UpdateInstallStep[];
  registryLogin?: { promptCommand: string | null };
}

export function getUpdateChannelCardOptions(
  versionInfo?: Pick<VersionInfo, 'isSourceBuild'> | null,
): UpdateChannelCardOption[] {
  return [
    {
      value: 'stable',
      title: 'Stable',
      description: 'Production-ready releases for paid and self-hosted environments',
      tone: 'success',
      disabled: versionInfo?.isSourceBuild,
    },
    {
      value: 'rc',
      title: 'Preview',
      description: 'Beta user-testing builds and release candidates for opt-in validation',
      tone: 'accent',
      disabled: versionInfo?.isSourceBuild,
    },
  ];
}

export function buildIdleDockerComposeCommand(): string {
  return 'docker compose pull && docker compose up -d';
}

export function buildIdleDockerUpdateCommand(): string {
  return 'docker pull rcourtman/pulse:latest && docker stop pulse && docker rm pulse';
}

// Copy shown in the idle Docker box for Pro-runtime containers instead of the
// community pull commands: a Pro compose file pins the exact image digest, so
// a plain `docker compose pull` can never update it, and pulling the public
// rcourtman/pulse image would silently downgrade the install to the community
// build. The digest-pinned commands come from the license server on the
// update-check response when a new release is available.
export const IDLE_DOCKER_PRO_NOTICE =
  'This container runs the Pulse Pro image, so it updates with digest-pinned commands from your license server, not the public rcourtman/pulse image. When a new release is available, the exact commands appear here.';

const proDockerCommandClass =
  'block rounded-md border border-border bg-page p-2 pr-14 font-mono text-xs text-green-700 dark:text-green-400 whitespace-pre-wrap break-all sm:p-3 sm:pr-14 sm:text-sm';

// Reconstruct only the non-secret arguments from the two known broker forms.
// Never redisplay an unrecognised line or carry a password into the command.
const legacyLogin =
  /^printf '%s' '<activation-key>' \| docker login ([a-z0-9.:-]+) -u '([a-z0-9_.@-]+)' --password-stdin$/i;
const promptLogin = /^docker login ('?)([a-z0-9.:-]+)\1 (?:-u|--username) '([a-z0-9_.@-]+)'$/i;
const registryHost =
  /^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*(?::[0-9]{1,5})?$/i;

export function getBrokerRegistryLoginPrompt({
  image,
  loginCommand,
}: DockerUpdateCommands): string | null {
  if (typeof loginCommand !== 'string' || typeof image !== 'string') return null;
  const legacy = legacyLogin.exec(loginCommand);
  const current = legacy ? null : promptLogin.exec(loginCommand);
  const match = legacy ?? current;
  if (!match || match[0] !== loginCommand) return null;
  const [registry, username] = match.slice(legacy ? 1 : 2);
  if (
    registry.length > 253 ||
    !registryHost.test(registry) ||
    !image.startsWith(`${registry}/`) ||
    username.length > 128 ||
    username.startsWith('-') ||
    username.toLowerCase().includes('ppk_live_')
  ) {
    return null;
  }
  return `docker login ${registry} --username '${username}'`;
}

function brokerPasswordPrompt(dockerUpdate: DockerUpdateCommands): string | null {
  return getBrokerRegistryLoginPrompt(dockerUpdate);
}

function buildProDockerUpdateSteps(dockerUpdate: DockerUpdateCommands): UpdateInstallStep[] {
  return [
    {
      id: 'docker-pro-pull',
      title: 'Pull the new Pulse Pro image (pinned to this release’s digest)',
      command: dockerUpdate.composePullCommand,
      commandCodeClass: proDockerCommandClass,
    },
    {
      id: 'docker-pro-up',
      title: 'Recreate the container on the new image',
      command: dockerUpdate.composeUpCommand,
      commandCodeClass: proDockerCommandClass,
    },
  ];
}

export function buildUpdateInstallGuide(
  versionInfo: Pick<VersionInfo, 'deploymentType' | 'isDocker'> | null | undefined,
  updateInfo: Pick<UpdateInfo, 'latestVersion' | 'dockerUpdate'> | null | undefined,
  updatePlan: Pick<UpdatePlan, 'canAutoUpdate'> | null | undefined,
  dockerImageTag: string,
  systemdDownloadCommand: string,
  isProRuntime = false,
): UpdateInstallGuide | null {
  if (!versionInfo) {
    return null;
  }

  const introText = updatePlan?.canAutoUpdate
    ? 'Click "Install Update" above for automatic installation, or update manually:'
    : 'Follow these steps to update manually:';

  if (versionInfo.deploymentType === 'proxmoxve') {
    const canAutoUpdate = updatePlan?.canAutoUpdate === true;
    return {
      headerTitle: 'Update Available',
      headerSummary: `Version ${updateInfo?.latestVersion} is ready to install`,
      introText: canAutoUpdate
        ? 'Use "Install Update" above. It stages and verifies the release before stopping Pulse and preserves rollback.'
        : 'Use the supported signed Pulse installer for this manual update.',
      steps: canAutoUpdate
        ? [
            {
              id: 'proxmox-safe-update',
              title: 'Install from this page',
              note: 'Do not run a legacy community-scripts `update` helper. Under memory or disk pressure it can stop Pulse before its download succeeds.',
            },
          ]
        : [
            {
              id: 'proxmox-signed-installer',
              title: 'Follow the signed installer instructions',
              note: 'Use docs/INSTALL.md from the matching Pulse release and follow its pinned SSH-signature verification flow. Do not run a legacy community-scripts `update` helper.',
            },
          ],
    };
  }

  if (
    versionInfo.deploymentType === 'docker' ||
    (!versionInfo.deploymentType && versionInfo.isDocker)
  ) {
    // Pro runtime: only ever show the broker's digest-pinned commands. The
    // community pull below would replace the container with the community
    // build and silently strip Pro features.
    if (updateInfo?.dockerUpdate) {
      return {
        headerTitle: 'Update Available',
        headerSummary: `Version ${updateInfo?.latestVersion} is ready to install`,
        introText: 'Run these two commands on the Docker host to update:',
        steps: buildProDockerUpdateSteps(updateInfo.dockerUpdate),
        ...(updateInfo.dockerUpdate.loginCommand
          ? { registryLogin: { promptCommand: brokerPasswordPrompt(updateInfo.dockerUpdate) } }
          : {}),
      };
    }
    if (isProRuntime) {
      return {
        headerTitle: 'Update Available',
        headerSummary: `Version ${updateInfo?.latestVersion} is ready to install`,
        introText: 'Follow these steps to update:',
        steps: [
          {
            id: 'docker-pro-unavailable',
            title: '',
            note: 'This container runs the Pulse Pro image, and the license server did not provide Docker update commands for this release. Update from Private Release Access (https://pulserelay.pro/download.html), which shows the digest-pinned pull commands for your license. Do not pull the public rcourtman/pulse image: it would replace this container with the community build.',
          },
        ],
      };
    }
    return {
      headerTitle: 'Update Available',
      headerSummary: `Version ${updateInfo?.latestVersion} is ready to install`,
      introText,
      steps: [
        {
          id: 'docker-compose-update',
          title: 'Pull the new image and recreate the container',
          command: 'docker compose pull && docker compose up -d',
        },
        {
          id: 'docker-manual-note',
          title: '',
          note: `Not using Compose? Run docker pull rcourtman/pulse:${dockerImageTag}, then docker stop pulse && docker rm pulse and re-run your original docker run command. A plain docker restart keeps the old image running.`,
        },
      ],
    };
  }

  if (versionInfo.deploymentType === 'systemd' || versionInfo.deploymentType === 'manual') {
    return {
      headerTitle: 'Update Available',
      headerSummary: `Version ${updateInfo?.latestVersion} is ready to install`,
      introText,
      steps: [
        { id: 'systemd-stop', title: 'Stop the service', command: 'sudo systemctl stop pulse' },
        {
          id: 'systemd-download',
          title: 'Download and extract the new version',
          command: systemdDownloadCommand,
          commandCodeClass:
            'block rounded-md border border-border bg-page p-3 font-mono text-sm text-base-content whitespace-pre-wrap break-all',
        },
        { id: 'systemd-start', title: 'Start the service', command: 'sudo systemctl start pulse' },
      ],
    };
  }

  if (versionInfo.deploymentType === 'development') {
    return {
      headerTitle: 'Update Available',
      headerSummary: `Version ${updateInfo?.latestVersion} is ready to install`,
      introText,
      steps: [
        {
          id: 'development-pull',
          title: 'Pull the latest changes',
          command: 'git pull origin main',
        },
        {
          id: 'development-build',
          title: 'Rebuild and restart',
          command: 'make build && make run',
        },
      ],
    };
  }

  return null;
}
