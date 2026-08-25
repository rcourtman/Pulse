import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { STORAGE_KEYS } from '@/utils/localStorage';

const versionInfoMock = vi.hoisted(() => vi.fn());
const getReleaseNotesMock = vi.hoisted(() => vi.fn());
const navigateMock = vi.hoisted(() => vi.fn());

vi.mock('@solidjs/router', () => ({
  useNavigate: () => navigateMock,
}));

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
    navigateMock.mockReset();
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
    expect(screen.queryByTestId('telemetry-payload-update-notice')).not.toBeInTheDocument();
    expect(getReleaseNotesMock).not.toHaveBeenCalled();
    expect(localStorage.getItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN)).toBe('6.1.0-rc.1');
    expect(localStorage.getItem(STORAGE_KEYS.TELEMETRY_PAYLOAD_NOTICE_SEEN)).toBe('2');
  });

  it('shows the telemetry payload update once to an existing installation', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.1.0-rc.1');
    versionInfoMock.mockReturnValue({
      version: '6.1.0-rc.1',
      isDevelopment: false,
      isSourceBuild: false,
    });

    await renderCard();

    expect(screen.getByTestId('telemetry-payload-update-notice')).toBeInTheDocument();
    expect(screen.getByText('Telemetry payload updated.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Preview payload' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Disable telemetry' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Privacy details' })).toHaveAttribute(
      'href',
      '/docs/PRIVACY',
    );
  });

  it('opens the exact payload preview and permanently dismisses the notice', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.1.0-rc.1');
    versionInfoMock.mockReturnValue({
      version: '6.1.0-rc.1',
      isDevelopment: false,
      isSourceBuild: false,
    });

    await renderCard();
    fireEvent.click(screen.getByRole('button', { name: 'Preview payload' }));

    expect(navigateMock).toHaveBeenCalledWith(
      '/settings/system-general?telemetryAction=preview#usage-telemetry',
    );
    expect(localStorage.getItem(STORAGE_KEYS.TELEMETRY_PAYLOAD_NOTICE_SEEN)).toBe('2');
    expect(screen.queryByTestId('telemetry-payload-update-notice')).not.toBeInTheDocument();
  });

  it('opens the disable action and permanently dismisses the notice', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.1.0-rc.1');
    versionInfoMock.mockReturnValue({
      version: '6.1.0-rc.1',
      isDevelopment: false,
      isSourceBuild: false,
    });

    await renderCard();
    fireEvent.click(screen.getByRole('button', { name: 'Disable telemetry' }));

    expect(navigateMock).toHaveBeenCalledWith(
      '/settings/system-general?telemetryAction=disable#usage-telemetry',
    );
    expect(localStorage.getItem(STORAGE_KEYS.TELEMETRY_PAYLOAD_NOTICE_SEEN)).toBe('2');
  });

  it('does not show the telemetry notice after it has been acknowledged', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.1.0-rc.1');
    localStorage.setItem(STORAGE_KEYS.TELEMETRY_PAYLOAD_NOTICE_SEEN, '2');
    versionInfoMock.mockReturnValue({
      version: '6.1.0-rc.1',
      isDevelopment: false,
      isSourceBuild: false,
    });

    await renderCard();

    expect(screen.queryByTestId('telemetry-payload-update-notice')).not.toBeInTheDocument();
  });

  it('announces the running release without opening a blocking dialog', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.0.5');
    localStorage.setItem(STORAGE_KEYS.TELEMETRY_PAYLOAD_NOTICE_SEEN, '2');
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
    expect(screen.getByText('Pulse v6.1.0-rc.1 changelog')).toBeInTheDocument();
    expect(screen.getByText('Added')).toBeInTheDocument();
    expect(
      screen.getByText('Actions now has a dedicated inbox for approvals.'),
    ).toBeInTheDocument();
    expect(screen.getByText('Fixed')).toBeInTheDocument();
    expect(
      screen.getByText('Acknowledged alerts now stay dismissed after refresh.'),
    ).toBeInTheDocument();
    expect(screen.queryByText('General agent improvements')).not.toBeInTheDocument();
    expect(screen.queryByText('Internal work')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Full release notes →' })).toHaveAttribute(
      'href',
      'https://github.com/rcourtman/Pulse/releases/tag/v6.1.0-rc.1',
    );
  });

  it('dismisses the compact notice without reopening it on reload', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.0.5');
    localStorage.setItem(STORAGE_KEYS.TELEMETRY_PAYLOAD_NOTICE_SEEN, '2');
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

  it('keeps the release notice quiet when the telemetry disclosure owns the session', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.0.5');
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
      expect(screen.getByTestId('telemetry-payload-update-notice')).toBeInTheDocument();
      expect(localStorage.getItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN)).toBe('6.1.0-rc.1');
    });
    expect(screen.queryByTestId('whats-new-notice')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss telemetry payload update' }));

    expect(screen.queryByTestId('telemetry-payload-update-notice')).not.toBeInTheDocument();
    expect(screen.queryByTestId('whats-new-notice')).not.toBeInTheDocument();
  });

  it('stays quiet when a release has only a highlights summary', async () => {
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
    expect(screen.queryByTestId('telemetry-payload-update-notice')).not.toBeInTheDocument();
    expect(getReleaseNotesMock).not.toHaveBeenCalled();
    expect(localStorage.getItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN)).toBe('6.0.5');
    expect(localStorage.getItem(STORAGE_KEYS.TELEMETRY_PAYLOAD_NOTICE_SEEN)).toBeNull();
  });
});
