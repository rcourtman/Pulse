import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { UpdateInstallGuide } from '../UpdateInstallGuide';

describe('UpdateInstallGuide', () => {
  afterEach(() => {
    cleanup();
  });

  it.each([false, true])(
    'keeps source Docker builds out of release installation guidance (cached update: %s)',
    (available) => {
      render(() => (
        <UpdateInstallGuide
          versionInfo={{
            version: '6.4.1+test.1913.bd37ae18',
            build: 'development',
            runtime: 'go',
            isDocker: true,
            isSourceBuild: true,
            isDevelopment: true,
          }}
          updateInfo={{
            available,
            currentVersion: '6.4.1+test.1913.bd37ae18',
            latestVersion: '6.4.4',
            releaseNotes: '',
            downloadUrl: '',
            isPrerelease: false,
            isMajorUpgrade: false,
          }}
          updatePlan={null}
          isInstalling={false}
          dockerImageTag="latest"
          systemdDownloadCommand=""
          isProRuntime={false}
          onInstallUpdate={vi.fn()}
        />
      ));
      expect(
        screen.getByText('Replace the container image manually to change versions.'),
      ).toBeInTheDocument();
      expect(screen.queryByText('Docker Installation')).not.toBeInTheDocument();
      expect(document.querySelector('code')).toBeNull();
      expect(screen.queryByRole('button', { name: /install/i })).not.toBeInTheDocument();
    },
  );

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

  function renderPaidDockerGuide(loginCommand?: string) {
    render(() => (
      <UpdateInstallGuide
        versionInfo={dockerVersionInfo}
        updateInfo={{
          ...availableDockerUpdateInfo,
          dockerUpdate: { ...dockerUpdate, loginCommand },
        }}
        updatePlan={dockerUpdatePlan}
        isInstalling={false}
        dockerImageTag="v6.0.5"
        systemdDownloadCommand=""
        isProRuntime={true}
        onInstallUpdate={vi.fn()}
      />
    ));
  }

  it.each([
    ['legacy', dockerUpdate.loginCommand, 'lic_test'],
    ['current', "docker login 'registry.pulserelay.pro' --username 'pulse-pro'", 'pulse-pro'],
  ])(
    'keeps both digest pins and shows only hidden-prompt sign-in for %s guidance',
    (_, input, username) => {
      renderPaidDockerGuide(input);

      expect(screen.getByText(dockerUpdate.composePullCommand)).toBeInTheDocument();
      expect(screen.getByText(dockerUpdate.composeUpCommand)).toBeInTheDocument();
      expect(screen.queryByText(/docker pull rcourtman\/pulse/)).not.toBeInTheDocument();
      const login = screen.getByText(
        `docker login registry.pulserelay.pro --username '${username}'`,
      );
      expect(login.closest('p')).toHaveTextContent(/only at Docker’s hidden Password prompt/);
      expect(screen.queryByText(/<activation-key>|--password-stdin/)).not.toBeInTheDocument();
      expect(screen.getAllByRole('button', { name: 'Copy to clipboard' })).toHaveLength(2);
    },
  );

  it('links to Private Release Access rather than showing unknown broker login text', () => {
    const unrecognised = 'Unexpected broker sign-in guidance';
    renderPaidDockerGuide(unrecognised);
    expect(screen.queryByText(unrecognised)).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Private Release Access' })).toHaveAttribute(
      'href',
      'https://pulserelay.pro/download.html',
    );
  });

  it('does not invent a login action when guidance was not supplied', () => {
    renderPaidDockerGuide();
    expect(screen.queryByText(/If the pull is denied/)).not.toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Copy to clipboard' })).toHaveLength(2);
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
