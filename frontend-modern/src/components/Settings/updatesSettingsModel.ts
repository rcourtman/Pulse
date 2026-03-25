import type { UpdateInfo, UpdatePlan, VersionInfo } from '@/api/updates';

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
      title: 'Pre-release',
      description: 'Early preview builds for staging, internal validation, and opt-in testers',
      tone: 'accent',
      disabled: versionInfo?.isSourceBuild,
    },
  ];
}

export function buildIdleDockerUpdateCommand(): string {
  return 'docker pull rcourtman/pulse:latest && docker restart pulse';
}

export function buildIdleDockerComposeCommand(): string {
  return 'docker-compose pull && docker-compose up -d';
}

export function buildUpdateInstallGuide(
  versionInfo: Pick<
    VersionInfo,
    'deploymentType' | 'isDocker'
  > | null | undefined,
  updateInfo: Pick<UpdateInfo, 'latestVersion'> | null | undefined,
  updatePlan: Pick<UpdatePlan, 'canAutoUpdate'> | null | undefined,
  dockerImageTag: string,
  systemdDownloadCommand: string,
): UpdateInstallGuide | null {
  if (!versionInfo) {
    return null;
  }

  const introText = updatePlan?.canAutoUpdate
    ? 'Click "Install Update" above for automatic installation, or update manually:'
    : 'Follow these steps to update manually:';

  if (versionInfo.deploymentType === 'proxmoxve') {
    return {
      headerTitle: 'Update Available',
      headerSummary: `Version ${updateInfo?.latestVersion} is ready to install`,
      introText,
      steps: [
        { id: 'open-console', title: 'Open your Pulse LXC console' },
        { id: 'run-update', title: 'Run the update command:', command: 'update' },
        {
          id: 'run-update-note',
          title: '',
          note: 'The script will automatically download and install the latest version.',
        },
      ],
    };
  }

  if (versionInfo.deploymentType === 'docker' || (!versionInfo.deploymentType && versionInfo.isDocker)) {
    return {
      headerTitle: 'Update Available',
      headerSummary: `Version ${updateInfo?.latestVersion} is ready to install`,
      introText,
      steps: [
        {
          id: 'docker-pull',
          title: 'Pull the latest image',
          command: `docker pull rcourtman/pulse:${dockerImageTag}`,
        },
        {
          id: 'docker-restart',
          title: 'Restart the container',
          command: 'docker restart pulse',
        },
        {
          id: 'docker-compose-note',
          title: '',
          note: 'Or use Docker Compose: docker-compose pull && docker-compose up -d',
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
            'block rounded-md border border-border bg-base p-3 font-mono text-sm text-base-content whitespace-pre-wrap break-all',
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
        { id: 'development-pull', title: 'Pull the latest changes', command: 'git pull origin main' },
        { id: 'development-build', title: 'Rebuild and restart', command: 'make build && make run' },
      ],
    };
  }

  return null;
}
