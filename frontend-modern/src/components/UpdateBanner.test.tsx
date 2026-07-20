import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

// Mutable mock state shared across the hoisted vi.mock factories below.
const { mock } = vi.hoisted(() => ({
  mock: {
    runtimeBuild: 'community' as string,
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
    plan: {
      canAutoUpdate: true,
      requiresRoot: false,
      rollbackSupport: false,
      instructions: [] as string[],
    },
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
      releaseNotes: '',
      releaseDate: '',
      downloadUrl:
        'https://github.com/rcourtman/Pulse/releases/download/v6.1.0/pulse-v6.1.0-linux-amd64.tar.gz',
      isPrerelease: false,
      isMajorUpgrade: false,
      dockerUpdate: mock.dockerUpdate,
    }),
    versionInfo: () => ({ version: '6.0.0', deploymentType: 'systemd' }),
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
  mock.dockerUpdate = undefined;
  mock.plan = {
    canAutoUpdate: true,
    requiresRoot: false,
    rollbackSupport: false,
    instructions: [],
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
