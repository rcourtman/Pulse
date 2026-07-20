import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { UpdateInstallGuide } from '../UpdateInstallGuide';

describe('UpdateInstallGuide', () => {
  afterEach(() => {
    cleanup();
  });

  it('shows upgrade readiness checks from the update plan', () => {
    render(() => (
      <UpdateInstallGuide
        versionInfo={{
          version: 'v6.0.0-rc.6',
          build: 'release',
          runtime: 'go',
          isDocker: false,
          isSourceBuild: false,
          isDevelopment: false,
          deploymentType: 'systemd',
        }}
        updateInfo={{
          available: true,
          currentVersion: 'v6.0.0-rc.6',
          latestVersion: 'v6.0.0',
          releaseNotes: '',
          releaseDate: '2026-05-28T00:00:00Z',
          downloadUrl: 'https://example.invalid/pulse.tar.gz',
          isPrerelease: false,
          isMajorUpgrade: false,
        }}
        updatePlan={{
          canAutoUpdate: true,
          instructions: ['Install Pulse'],
          prerequisites: [],
          requiresRoot: true,
          rollbackSupport: true,
          readiness: {
            status: 'ready',
            summary: 'Upgrade checks passed for this Pulse instance.',
            checks: [
              {
                id: 'agent-continuity',
                status: 'pass',
                title: 'Agent continuity',
                summary: 'Registered agents have recent heartbeats.',
                details: ['1 v5 or legacy agent can continue reporting.'],
              },
            ],
          },
        }}
        isInstalling={false}
        dockerImageTag="v6.0.0"
        systemdDownloadCommand="curl -fsSL https://example.invalid | tar"
        isProRuntime={false}
        onInstallUpdate={vi.fn()}
      />
    ));

    expect(screen.getByText('Upgrade checks')).toBeInTheDocument();
    expect(screen.getByText('Upgrade checks passed for this Pulse instance.')).toBeInTheDocument();
    expect(screen.getByText('Agent continuity')).toBeInTheDocument();
    expect(screen.getByText('1 v5 or legacy agent can continue reporting.')).toBeInTheDocument();
  });

  it('blocks the automatic install action when readiness is blocked', () => {
    const onInstallUpdate = vi.fn();
    render(() => (
      <UpdateInstallGuide
        versionInfo={{
          version: 'v6.0.0-rc.6',
          build: 'release',
          runtime: 'go',
          isDocker: false,
          isSourceBuild: false,
          isDevelopment: false,
          deploymentType: 'systemd',
        }}
        updateInfo={{
          available: true,
          currentVersion: 'v6.0.0-rc.6',
          latestVersion: 'v6.0.0',
          releaseNotes: '',
          releaseDate: '2026-05-28T00:00:00Z',
          downloadUrl: 'https://example.invalid/pulse.tar.gz',
          isPrerelease: false,
          isMajorUpgrade: false,
        }}
        updatePlan={{
          canAutoUpdate: true,
          instructions: ['Install Pulse'],
          prerequisites: [],
          requiresRoot: true,
          rollbackSupport: true,
          readiness: {
            status: 'blocked',
            summary: 'Resolve 1 blocked upgrade check before installing this update.',
            checks: [
              {
                id: 'agent-token-scopes',
                status: 'blocked',
                title: 'Agent token scopes',
                summary:
                  'Registered agents exist, but no loaded API token grants agent reporting scope.',
              },
            ],
          },
        }}
        isInstalling={false}
        dockerImageTag="v6.0.0"
        systemdDownloadCommand="curl -fsSL https://example.invalid | tar"
        isProRuntime={false}
        onInstallUpdate={onInstallUpdate}
      />
    ));

    const installButton = screen.getByRole('button', { name: 'Install blocked' });
    expect(installButton).toBeDisabled();
    fireEvent.click(installButton);
    expect(onInstallUpdate).not.toHaveBeenCalled();
  });

  const dockerVersionInfo = {
    version: 'v6.0.0',
    build: 'release',
    runtime: 'go',
    isDocker: true,
    isSourceBuild: false,
    isDevelopment: false,
    deploymentType: 'docker',
  };

  const dockerUpdatePlan = {
    canAutoUpdate: false,
    instructions: [],
    prerequisites: [],
    requiresRoot: false,
    rollbackSupport: true,
  };

  const digest = 'sha256:' + 'ab'.repeat(32);
  const pinnedRef = `registry.pulserelay.pro/pulse/pulse-pro@${digest}`;
  const dockerUpdate = {
    version: 'v6.0.5',
    image: 'registry.pulserelay.pro/pulse/pulse-pro',
    imageDigest: digest,
    loginCommand:
      "printf '%s' '<activation-key>' | docker login registry.pulserelay.pro -u 'lic_test' --password-stdin",
    composePullCommand: `PULSE_IMAGE='${pinnedRef}' docker compose pull`,
    composeUpCommand: `PULSE_IMAGE='${pinnedRef}' docker compose up -d`,
  };

  const availableDockerUpdateInfo = {
    available: true,
    currentVersion: 'v6.0.0',
    latestVersion: 'v6.0.5',
    releaseNotes: '',
    releaseDate: '2026-07-01T00:00:00Z',
    downloadUrl: 'https://license.example.invalid/v1/downloads/pulse-pro?version=v6.0.5',
    isPrerelease: false,
    isMajorUpgrade: false,
  };

  it('shows the broker digest-pinned commands for a Pro docker install, never the community image', () => {
    render(() => (
      <UpdateInstallGuide
        versionInfo={dockerVersionInfo}
        updateInfo={{ ...availableDockerUpdateInfo, dockerUpdate }}
        updatePlan={dockerUpdatePlan}
        isInstalling={false}
        dockerImageTag="v6.0.5"
        systemdDownloadCommand=""
        isProRuntime={true}
        onInstallUpdate={vi.fn()}
      />
    ));

    expect(screen.getByText(`PULSE_IMAGE='${pinnedRef}' docker compose pull`)).toBeInTheDocument();
    expect(screen.getByText(`PULSE_IMAGE='${pinnedRef}' docker compose up -d`)).toBeInTheDocument();
    expect(screen.queryByText(/docker pull rcourtman\/pulse/)).not.toBeInTheDocument();
  });

  it('withholds community commands on a Pro docker install even without broker commands', () => {
    render(() => (
      <UpdateInstallGuide
        versionInfo={dockerVersionInfo}
        updateInfo={availableDockerUpdateInfo}
        updatePlan={dockerUpdatePlan}
        isInstalling={false}
        dockerImageTag="v6.0.5"
        systemdDownloadCommand=""
        isProRuntime={true}
        onInstallUpdate={vi.fn()}
      />
    ));

    expect(screen.getByText(/Private Release Access/)).toBeInTheDocument();
    expect(
      screen.queryByText(/docker compose pull && docker compose up -d/),
    ).not.toBeInTheDocument();
  });

  it('replaces the idle docker community commands with the Pro notice', () => {
    render(() => (
      <UpdateInstallGuide
        versionInfo={dockerVersionInfo}
        updateInfo={{ ...availableDockerUpdateInfo, available: false, latestVersion: 'v6.0.0' }}
        updatePlan={null}
        isInstalling={false}
        dockerImageTag="v6.0.0"
        systemdDownloadCommand=""
        isProRuntime={true}
        onInstallUpdate={vi.fn()}
      />
    ));

    expect(screen.getByText(/digest-pinned commands from your license server/)).toBeInTheDocument();
    expect(screen.queryByText(/docker pull rcourtman\/pulse/)).not.toBeInTheDocument();
    expect(
      screen.queryByText('docker compose pull && docker compose up -d'),
    ).not.toBeInTheDocument();
  });

  it('keeps the community docker guidance for community installs', () => {
    render(() => (
      <UpdateInstallGuide
        versionInfo={dockerVersionInfo}
        updateInfo={availableDockerUpdateInfo}
        updatePlan={dockerUpdatePlan}
        isInstalling={false}
        dockerImageTag="v6.0.5"
        systemdDownloadCommand=""
        isProRuntime={false}
        onInstallUpdate={vi.fn()}
      />
    ));

    expect(screen.getByText('docker compose pull && docker compose up -d')).toBeInTheDocument();
    expect(screen.getByText(/docker pull rcourtman\/pulse:v6\.0\.5/)).toBeInTheDocument();
  });
});
