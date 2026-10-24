import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { STORAGE_KEYS } from '@/utils/localStorage';

const versionInfoMock = vi.hoisted(() => vi.fn());
const getReleaseNotesMock = vi.hoisted(() => vi.fn());

vi.mock('@/stores/updates', () => ({
  updateStore: {
    versionInfo: () => versionInfoMock(),
  },
}));

vi.mock('@/api/updates', () => ({
  UpdatesAPI: {
    getReleaseNotes: () => getReleaseNotesMock(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

describe('WhatsNewCard', () => {
  beforeEach(() => {
    versionInfoMock.mockReset();
    getReleaseNotesMock.mockReset();
    localStorage.clear();
    sessionStorage.clear();
  });

  afterEach(cleanup);

  async function renderCard() {
    const { WhatsNewCard } = await import('../WhatsNewCard');
    render(() => <WhatsNewCard />);
  }

  it('records the first published version silently as the baseline', async () => {
    versionInfoMock.mockReturnValue({
      version: '6.1.0-rc.1',
      isDevelopment: false,
      isSourceBuild: false,
    });

    await renderCard();

    expect(screen.queryByTestId('whats-new-modal')).not.toBeInTheDocument();
    expect(getReleaseNotesMock).not.toHaveBeenCalled();
    expect(localStorage.getItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN)).toBe('6.1.0-rc.1');
  });

  it('never shows a telemetry payload notice to an existing installation', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.1.0-rc.1');
    versionInfoMock.mockReturnValue({
      version: '6.1.0-rc.1',
      isDevelopment: false,
      isSourceBuild: false,
    });

    await renderCard();

    expect(screen.queryByTestId('telemetry-payload-update-notice')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Disable telemetry' })).not.toBeInTheDocument();
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('announces the running release without opening a blocking dialog', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.0.5');
    versionInfoMock.mockReturnValue({
      version: '6.1.0-rc.1',
      isDevelopment: false,
      isSourceBuild: false,
    });
    getReleaseNotesMock.mockResolvedValue({
      version: 'v6.1.0-rc.1',
      releaseNotes: [
        '## Highlights',
        '- General agent improvements',
        '',
        '## Added',
        '- Actions now has a dedicated inbox for approvals.',
        '',
        '## Fixed',
        '- Acknowledged alerts now stay dismissed after refresh.',
        '',
        '## Release Qualification',
        '- Internal work',
      ].join('\n'),
      releaseDate: '2026-07-13T12:00:00Z',
      isPrerelease: true,
    });

    await renderCard();

    await waitFor(() => {
      expect(screen.getByTestId('whats-new-notice')).toBeInTheDocument();
    });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByText('Pulse updated to v6.1.0-rc.1')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: "See what's new in Pulse 6.1.0-rc.1" }),
    ).toBeInTheDocument();
    expect(localStorage.getItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN)).toBe('6.1.0-rc.1');

    fireEvent.click(screen.getByRole('button', { name: "See what's new in Pulse 6.1.0-rc.1" }));

    expect(screen.queryByTestId('whats-new-notice')).not.toBeInTheDocument();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText("What's new in Pulse v6.1.0-rc.1")).toBeInTheDocument();
    expect(
      screen.getByText(`Released ${new Date('2026-07-13T12:00:00Z').toLocaleDateString()}`),
    ).toBeInTheDocument();
    expect(screen.getByText('Added')).toBeInTheDocument();
    expect(
      screen.getByText('Actions now has a dedicated inbox for approvals.'),
    ).toBeInTheDocument();
    expect(screen.getByText('Fixed')).toBeInTheDocument();
    expect(
      screen.getByText('Acknowledged alerts now stay dismissed after refresh.'),
    ).toBeInTheDocument();
    expect(screen.getByText('Acknowledged alerts now stay dismissed after refresh.')).toBeVisible();
    // The dialog is a changelog: the Highlights summary is not repeated above it.
    expect(screen.queryByText('General agent improvements')).not.toBeInTheDocument();
    expect(screen.queryByText('All changes')).not.toBeInTheDocument();
    expect(document.querySelector('[data-testid="whats-new-modal"] details')).toBeNull();
    expect(screen.queryByText('Internal work')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Full release notes →' })).toHaveAttribute(
      'href',
      'https://github.com/rcourtman/Pulse/releases/tag/v6.1.0-rc.1',
    );
  });

  it('dismisses the compact notice without reopening it on reload', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.0.5');
    versionInfoMock.mockReturnValue({
      version: '6.1.0-rc.1',
      isDevelopment: false,
      isSourceBuild: false,
    });
    getReleaseNotesMock.mockResolvedValue({
      version: '6.1.0-rc.1',
      releaseNotes: '## Added\n- Actions now has a dedicated inbox for approvals.',
      releaseDate: '2026-07-13T12:00:00Z',
      isPrerelease: true,
    });

    await renderCard();
    await waitFor(() => expect(screen.getByTestId('whats-new-notice')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss update notice' }));

    await waitFor(() => {
      expect(screen.queryByTestId('whats-new-notice')).not.toBeInTheDocument();
    });
    expect(localStorage.getItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN)).toBe('6.1.0-rc.1');

    cleanup();
    await renderCard();

    expect(screen.queryByTestId('whats-new-notice')).not.toBeInTheDocument();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('yields the notice slot to a higher-priority owner already holding the session', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.0.5');
    sessionStorage.setItem('pulse-low-priority-notice-owner', 'github-star');
    versionInfoMock.mockReturnValue({
      version: '6.1.0-rc.1',
      isDevelopment: false,
      isSourceBuild: false,
    });
    getReleaseNotesMock.mockResolvedValue({
      version: '6.1.0-rc.1',
      releaseNotes: '## Improved\n- Faster resource updates.',
      releaseDate: '2026-07-13T12:00:00Z',
      isPrerelease: true,
    });

    await renderCard();

    await waitFor(() => {
      expect(localStorage.getItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN)).toBe('6.1.0-rc.1');
    });
    expect(screen.queryByTestId('whats-new-notice')).not.toBeInTheDocument();
  });

  it('falls back to the Highlights list for a release without change groups', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.0.5');
    versionInfoMock.mockReturnValue({
      version: '6.1.0-rc.1',
      isDevelopment: false,
      isSourceBuild: false,
    });
    getReleaseNotesMock.mockResolvedValue({
      version: '6.1.0-rc.1',
      releaseNotes: '## Highlights\n- General improvements',
      releaseDate: '2026-07-13T12:00:00Z',
      isPrerelease: true,
    });

    await renderCard();

    await waitFor(() => {
      expect(localStorage.getItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN)).toBe('6.1.0-rc.1');
    });
    expect(screen.queryByTestId('whats-new-modal')).not.toBeInTheDocument();
    fireEvent.click(
      await screen.findByRole('button', { name: "See what's new in Pulse 6.1.0-rc.1" }),
    );
    expect(screen.getByText('General improvements')).toBeVisible();
  });

  it('orders warnings, screenshots, changes and upgrade notes without disclosures', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.5.0');
    versionInfoMock.mockReturnValue({
      version: '6.6.0',
      isDevelopment: false,
      isSourceBuild: false,
    });
    const assetBase = 'https://github.com/rcourtman/Pulse/releases/download/v6.6.0/';
    getReleaseNotesMock.mockResolvedValue({
      version: '6.6.0',
      releaseNotes: [
        '## Highlights',
        '- Find backup failures faster.',
        '## Proxmox, PBS and backups',
        '- Open Backups to inspect recent failures.',
        '## Known issues',
        'Older agents need an update.',
        '## Breaking changes',
        'Migrate the old configuration before enabling collection.',
        '## Before you upgrade',
        'Verify the published checksums.',
        '## See the difference',
        '### Backup coverage at a glance',
        'Compare backup coverage across nodes.',
        '| Before | Now |',
        '| --- | --- |',
        `| ![Before](${assetBase}release-note-backups-before.png) | ![Now](${assetBase}release-note-backups-now.png) |`,
        '## Install',
        'Internal installation instructions.',
      ].join('\n'),
    });
    await renderCard();
    fireEvent.click(await screen.findByRole('button', { name: "See what's new in Pulse 6.6.0" }));
    expect(screen.getByText('Older agents need an update.')).toBeVisible();
    expect(
      screen.getByText('Migrate the old configuration before enabling collection.'),
    ).toBeVisible();
    const warnings = screen.getByRole('region', { name: 'Important release information' });
    const comparison = screen.getByRole('region', { name: 'Backup coverage at a glance' });
    const changes = screen.getByRole('region', { name: 'Changes' });
    const upgradeNotes = screen.getByRole('region', { name: 'Upgrade notes' });
    expect(warnings.nextElementSibling).toBe(comparison);
    expect(comparison.nextElementSibling).toBe(changes);
    expect(changes.nextElementSibling).toBe(upgradeNotes);
    expect(screen.queryByRole('region', { name: 'Release highlights' })).toBeNull();
    expect(screen.queryByText('Find backup failures faster.')).not.toBeInTheDocument();
    expect(screen.getByText('Open Backups to inspect recent failures.')).toBeVisible();
    expect(screen.getByText('Verify the published checksums.')).toBeVisible();
    expect(document.querySelector('[data-testid="whats-new-modal"] details')).toBeNull();
    const after = screen.getByRole('img', { name: 'Backup coverage at a glance, now' });
    expect(after).toHaveAttribute(
      'src',
      '/api/updates/release-notes/assets/v6.6.0/release-note-backups-now.png',
    );
    expect(after).toHaveAttribute('referrerpolicy', 'no-referrer');
    expect(
      screen.getByRole('link', { name: /View Backup coverage at a glance, now, full size/ }),
    ).toHaveAttribute('rel', 'noopener noreferrer');
    fireEvent.error(after);
    expect(screen.getByText('Screenshot unavailable.')).toBeVisible();
    expect(
      screen.getByRole('img', { name: 'Backup coverage at a glance, before' }),
    ).toBeInTheDocument();
    expect(screen.queryByText('Internal installation instructions.')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Got it' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('renders text-only legacy releases without requiring highlights or images', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.0.5');
    versionInfoMock.mockReturnValue({
      version: '6.1.0',
      isDevelopment: false,
      isSourceBuild: false,
    });
    getReleaseNotesMock.mockResolvedValue({
      version: '6.1.0',
      releaseNotes:
        '## Fixed\n- Alerts stay dismissed. <script>window.pwned=true</script><img src="https://example.com/tracker.png">',
    });
    await renderCard();
    fireEvent.click(await screen.findByRole('button', { name: "See what's new in Pulse 6.1.0" }));
    expect(screen.getByText(/Alerts stay dismissed/)).toBeVisible();
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
    expect(document.querySelector('script')).not.toBeInTheDocument();
  });

  it('stays quiet for development builds', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.0.5');
    versionInfoMock.mockReturnValue({
      version: '6.1.0-rc.1-dirty',
      isDevelopment: true,
      isSourceBuild: false,
    });

    await renderCard();

    expect(screen.queryByTestId('whats-new-modal')).not.toBeInTheDocument();
    expect(getReleaseNotesMock).not.toHaveBeenCalled();
    expect(localStorage.getItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN)).toBe('6.0.5');
  });
});
