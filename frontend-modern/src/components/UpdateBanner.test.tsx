import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

// Mutable mock state shared across the hoisted vi.mock factories below.
const { mock } = vi.hoisted(() => ({
  mock: {
    runtimeBuild: 'community' as string,
    plan: {
      canAutoUpdate: true,
      requiresRoot: false,
      rollbackSupport: false,
      instructions: [] as string[],
    },
  },
}));

// runtime.build is the binary-edition signal (business-hooks presence), which
// is what UpdateBanner keys the Pro path off of.
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
    }),
    versionInfo: () => ({ version: '6.0.0', deploymentType: 'systemd' }),
    isUpdateVisible: () => true,
    dismissUpdate: vi.fn(),
  },
}));

vi.mock('@/api/updates', () => ({
  UpdatesAPI: {
    getUpdatePlan: vi.fn(() => Promise.resolve(mock.plan)),
    applyUpdate: vi.fn(() => Promise.resolve()),
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
  mock.plan = { canAutoUpdate: true, requiresRoot: false, rollbackSupport: false, instructions: [] };
});

describe('UpdateBanner Pro edition guard', () => {
  it('suppresses in-app apply and routes the Pro binary to the portal', () => {
    mock.runtimeBuild = 'pro';

    render(() => <UpdateBanner />);

    // The in-app apply affordance must never render for the Pro binary, even
    // though the plan reports canAutoUpdate=true (systemd).
    expect(screen.queryByRole('button', { name: /Apply Update/ })).not.toBeInTheDocument();

    const portalLink = screen.getByRole('link', { name: /Private Release Access/ });
    expect(portalLink).toHaveAttribute('href', PORTAL_URL);
  });

  it('shows the portal steps (archive + .sshsig) when the Pro banner is expanded', () => {
    mock.runtimeBuild = 'pro';

    render(() => <UpdateBanner />);
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

  it('keeps the in-app apply button for the community binary', async () => {
    mock.runtimeBuild = 'community';

    render(() => <UpdateBanner />);

    // Apply appears once the (async) update plan resolves with canAutoUpdate.
    expect(await screen.findByRole('button', { name: 'Apply Update' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Private Release Access/ })).not.toBeInTheDocument();
  });
});
