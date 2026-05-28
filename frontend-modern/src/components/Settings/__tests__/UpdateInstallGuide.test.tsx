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
                summary: 'Registered agents exist, but no loaded API token grants agent reporting scope.',
              },
            ],
          },
        }}
        isInstalling={false}
        dockerImageTag="v6.0.0"
        systemdDownloadCommand="curl -fsSL https://example.invalid | tar"
        onInstallUpdate={onInstallUpdate}
      />
    ));

    const installButton = screen.getByRole('button', { name: 'Install blocked' });
    expect(installButton).toBeDisabled();
    fireEvent.click(installButton);
    expect(onInstallUpdate).not.toHaveBeenCalled();
  });
});
