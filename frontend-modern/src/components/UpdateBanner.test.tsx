import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { UpdatePlan } from '@/api/updates';

// Mutable mock state shared across the hoisted vi.mock factories below.
const { mock } = vi.hoisted(() => ({
  mock: {
    runtimeBuild: 'community' as string,
    deploymentType: 'systemd' as string,
    dockerUpdate: undefined as
      | {
          version: string;
          image: string;
          imageDigest: string;
          loginCommand?: string;
          composePullCommand: string;
          composeUpCommand: string;
        }
      | undefined,
    releaseNotes: '',
    plan: {
      canAutoUpdate: true,
      requiresRoot: false,
      rollbackSupport: false,
      instructions: [] as string[],
    } as UpdatePlan,
  },
}));

// runtime.build is the binary-edition signal (business-hooks presence), which
// is what UpdateBanner keys the Pro paths off of.
vi.mock('@/stores/license', () => ({
  runtimeCapabilities: () => ({ runtime: { build: mock.runtimeBuild } }),
}));

vi.mock('@/stores/updates', () => ({
  updateStore: {
    updateInfo: () => ({
      available: true,
      currentVersion: '6.0.0',
      latestVersion: '6.1.0',
      releaseNotes: mock.releaseNotes,
      releaseDate: '',
      downloadUrl:
        'https://github.com/rcourtman/Pulse/releases/download/v6.1.0/pulse-v6.1.0-linux-amd64.tar.gz',
      isPrerelease: false,
      isMajorUpgrade: false,
      dockerUpdate: mock.dockerUpdate,
    }),
    versionInfo: () => ({ version: '6.0.0', deploymentType: mock.deploymentType }),
    isUpdateVisible: () => true,
    dismissUpdate: vi.fn(),
    applyUpdate: vi.fn(() => Promise.resolve(true)),
  },
}));

vi.mock('@/api/updates', () => ({
  UpdatesAPI: {
    getUpdatePlan: vi.fn(() => Promise.resolve(mock.plan)),
  },
}));

vi.mock('./UpdateConfirmationModal', () => ({
  UpdateConfirmationModal: () => null,
}));

import { UpdateBanner } from './UpdateBanner';

const PORTAL_URL = 'https://pulserelay.pro/download.html';

afterEach(() => {
  cleanup();
  mock.runtimeBuild = 'community';
  mock.deploymentType = 'systemd';
  mock.dockerUpdate = undefined;
  mock.releaseNotes = '';
  mock.plan = {
    canAutoUpdate: true,
    requiresRoot: false,
    rollbackSupport: false,
    instructions: [],
    readiness: undefined,
  };
});

describe('UpdateBanner Pro edition update paths', () => {
  it('keeps in-app apply for the Pro binary on auto-updatable deployments', async () => {
    mock.runtimeBuild = 'pro';

    render(() => <UpdateBanner />);

    // The Pro binary updates from the license server download broker, so the
    // in-app apply affordance must render once the plan resolves.
    expect(await screen.findByRole('button', { name: 'Apply Update' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Private Release Access/ })).not.toBeInTheDocument();
  });

  it('explains the private release source when the Pro banner is expanded', async () => {
    mock.runtimeBuild = 'pro';

    render(() => <UpdateBanner />);
    await screen.findByRole('button', { name: 'Apply Update' });
    fireEvent.click(screen.getByTitle('Show more'));

    expect(screen.getByText(/private Pulse Pro build from the license server/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Apply Update Automatically' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Private Release Access/ })).not.toBeInTheDocument();
  });

  it('routes non-auto-updatable Pro deployments to the portal', async () => {
    mock.runtimeBuild = 'pro';
    mock.plan = {
      canAutoUpdate: false,
      requiresRoot: false,
      rollbackSupport: false,
      instructions: [],
    };

    render(() => <UpdateBanner />);

    const portalLink = await screen.findByRole('link', { name: /Private Release Access/ });
    expect(portalLink).toHaveAttribute('href', PORTAL_URL);
    expect(screen.queryByRole('button', { name: /Apply Update/ })).not.toBeInTheDocument();
  });

  it('shows the portal steps (archive + .sshsig) for manual Pro deployments when expanded', async () => {
    mock.runtimeBuild = 'pro';
    mock.plan = {
      canAutoUpdate: false,
      requiresRoot: false,
      rollbackSupport: false,
      instructions: [],
    };

    render(() => <UpdateBanner />);
    await screen.findByRole('link', { name: /Private Release Access/ });
    fireEvent.click(screen.getByTitle('Show more'));

    expect(screen.getByText('Pulse Pro update')).toBeInTheDocument();
    expect(screen.getByText(/install\.sh --archive/)).toBeInTheDocument();
    expect(screen.getByText(/\.sshsig/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Apply Update/ })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Open Private Release Access/ })).toHaveAttribute(
      'href',
      PORTAL_URL,
    );
  });

  it('shows digest-pinned docker commands for Pro docker deployments when expanded', async () => {
    const digest = 'sha256:' + 'ab'.repeat(32);
    const pinnedRef = `registry.pulserelay.pro/pulse/pulse-pro@${digest}`;
    mock.runtimeBuild = 'pro';
    mock.plan = {
      canAutoUpdate: false,
      requiresRoot: false,
      rollbackSupport: false,
      instructions: [],
    };
    mock.dockerUpdate = {
      version: 'v6.1.0',
      image: 'registry.pulserelay.pro/pulse/pulse-pro',
      imageDigest: digest,
      composePullCommand: `PULSE_IMAGE='${pinnedRef}' docker compose pull`,
      composeUpCommand: `PULSE_IMAGE='${pinnedRef}' docker compose up -d`,
    };

    render(() => <UpdateBanner />);
    await screen.findByRole('link', { name: /Private Release Access/ });
    fireEvent.click(screen.getByTitle('Show more'));

    expect(screen.getByText('Pulse Pro update')).toBeInTheDocument();
    expect(screen.getByText(`PULSE_IMAGE='${pinnedRef}' docker compose pull`)).toBeInTheDocument();
    expect(screen.getByText(`PULSE_IMAGE='${pinnedRef}' docker compose up -d`)).toBeInTheDocument();
    // The binary archive steps and community image must not appear for a
    // Docker Pro deployment with broker commands available.
    expect(screen.queryByText(/install\.sh --archive/)).not.toBeInTheDocument();
    expect(screen.queryByText(/docker pull rcourtman\/pulse/)).not.toBeInTheDocument();
  });

  it('keeps the in-app apply button for the community binary', async () => {
    mock.runtimeBuild = 'community';

    render(() => <UpdateBanner />);

    // Apply appears once the (async) update plan resolves with canAutoUpdate.
    expect(await screen.findByRole('button', { name: 'Apply Update' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Private Release Access/ })).not.toBeInTheDocument();
  });
});

describe('UpdateBanner Proxmox LXC update safety', () => {
  it('routes an auto-updatable LXC to in-product apply instead of the legacy update helper', async () => {
    mock.deploymentType = 'proxmoxve';

    render(() => <UpdateBanner />);

    expect(await screen.findByRole('button', { name: 'Apply Update' })).toBeInTheDocument();
    expect(
      screen.getByText(/use Apply Update here\. Do not use a legacy community 'update' helper/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/type 'update' in console/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByTitle('Show more'));
    expect(screen.getByText(/Safe upgrade:/)).toBeInTheDocument();
  });
});

describe('UpdateBanner release-note preview', () => {
  it("shows customer-facing What's improved bullets in the expanded banner", async () => {
    mock.releaseNotes = `# Pulse v6.4.0-rc.2 Release Notes

Infrastructure views stay fast and predictable as your environment grows.

## What's improved

- **Faster infrastructure views** — Tables stay responsive as estates grow.
- **Lighter realtime updates** — Pages do less work when resources change.

## Fixes

- Saved API keys are no longer returned to the browser.
`;

    render(() => <UpdateBanner />);
    await screen.findByRole('button', { name: 'Apply Update' });
    fireEvent.click(screen.getByTitle('Show more'));

    expect(screen.getByText("What's new in v6.1.0")).toBeInTheDocument();
    expect(screen.getByText(/Faster infrastructure views/)).toBeInTheDocument();
    expect(screen.getByText(/Lighter realtime updates/)).toBeInTheDocument();
    expect(screen.queryByText(/Saved API keys/)).not.toBeInTheDocument();
  });
});

describe('UpdateBanner upgrade readiness checks', () => {
  it('blocks in-app apply and surfaces blocked checks when readiness is blocked', async () => {
    mock.plan = {
      canAutoUpdate: true,
      requiresRoot: false,
      rollbackSupport: true,
      instructions: [],
      readiness: {
        status: 'blocked',
        summary: 'Resolve 1 blocked upgrade check before installing this update.',
        checks: [
          {
            id: 'server-update-path',
            status: 'pass',
            title: 'Server update path',
            summary: 'Automatic install and rollback support are available for this deployment.',
          },
          {
            id: 'agent-token-scopes',
            status: 'blocked',
            title: 'Agent token scopes',
            summary: 'All loaded agent reporting tokens are expired.',
            details: [
              'Create or refresh an agent install token before relying on agent reconnect.',
            ],
          },
        ],
      },
    };

    render(() => <UpdateBanner />);

    expect(await screen.findByText('Update blocked')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Apply Update' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Review checks' }));

    const panel = screen.getByTestId('update-readiness-panel');
    expect(panel).toHaveAttribute('role', 'alert');
    expect(
      within(panel).getByText('Resolve 1 blocked upgrade check before installing this update.'),
    ).toBeInTheDocument();
    expect(within(panel).getByText('Agent token scopes')).toBeInTheDocument();
    expect(
      within(panel).getByText('All loaded agent reporting tokens are expired.'),
    ).toBeInTheDocument();
    expect(
      within(panel).getByText(
        'Create or refresh an agent install token before relying on agent reconnect.',
      ),
    ).toBeInTheDocument();

    // Blocked checks appear before passing checks so operators see the blocker first.
    const items = within(panel).getAllByRole('listitem');
    expect(items[0]).toHaveTextContent('Agent token scopes');
    expect(items[0]).toHaveTextContent('Blocked');

    const installBlockedButton = screen.getByRole('button', { name: 'Install blocked' });
    expect(installBlockedButton).toBeDisabled();
    expect(
      screen.queryByRole('button', { name: 'Apply Update Automatically' }),
    ).not.toBeInTheDocument();
  });

  it('surfaces upgrade warnings in collapsed and expanded banner for manual Docker updates', async () => {
    mock.deploymentType = 'docker';
    mock.plan = {
      canAutoUpdate: false,
      requiresRoot: false,
      rollbackSupport: false,
      instructions: ['docker pull rcourtman/pulse:v6.1.0'],
      readiness: {
        status: 'attention',
        summary: 'Update can proceed, but review 1 upgrade warning before installing.',
        checks: [
          {
            id: 'container-data-persistence',
            status: 'warning',
            title: 'Container data persistence',
            summary: 'Pulse data directory is not mounted on a persistent volume.',
            details: ['Mount a persistent volume at /data before recreating the container.'],
          },
        ],
      },
    };

    render(() => <UpdateBanner />);

    const warningButton = await screen.findByRole('button', { name: '1 upgrade warning' });
    expect(screen.getByText('Manual steps required')).toBeInTheDocument();

    fireEvent.click(warningButton);

    const panel = screen.getByTestId('update-readiness-panel');
    expect(panel).toHaveAttribute('role', 'region');
    expect(within(panel).getByText('Container data persistence')).toBeInTheDocument();
    expect(
      within(panel).getByText(
        'Mount a persistent volume at /data before recreating the container.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('docker pull rcourtman/pulse:v6.1.0')).toBeInTheDocument();
  });

  it('keeps the collapsed banner clean when all readiness checks pass', async () => {
    mock.plan = {
      canAutoUpdate: true,
      requiresRoot: false,
      rollbackSupport: true,
      instructions: [],
      readiness: {
        status: 'ready',
        summary: 'Upgrade checks passed for this Pulse instance.',
        checks: [
          {
            id: 'server-update-path',
            status: 'pass',
            title: 'Server update path',
            summary: 'Automatic install and rollback support are available for this deployment.',
          },
        ],
      },
    };

    render(() => <UpdateBanner />);

    expect(await screen.findByRole('button', { name: 'Apply Update' })).toBeInTheDocument();
    expect(screen.queryByText('Update blocked')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /upgrade warning/i })).not.toBeInTheDocument();

    fireEvent.click(screen.getByTitle('Show more'));
    expect(screen.getByText('Upgrade checks passed for this Pulse instance.')).toBeInTheDocument();
  });
});
