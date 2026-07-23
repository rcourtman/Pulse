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
      '/docs/PRIVACY.md',
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

  it('shows curated highlights after the running release changes', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.0.5');
    versionInfoMock.mockReturnValue({
      version: '6.1.0-rc.1',
      isDevelopment: false,
      isSourceBuild: false,
    });
    getReleaseNotesMock.mockResolvedValue({
      version: 'v6.1.0-rc.1',
      releaseNotes: '## Highlights\n- Reviewed Actions inbox\n\n## Changes\n- Internal work',
      releaseDate: '2026-07-13T12:00:00Z',
      isPrerelease: true,
    });

    await renderCard();

    await waitFor(() => {
      expect(screen.getByTestId('whats-new-modal')).toBeInTheDocument();
    });
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText("What's new in v6.1.0-rc.1")).toBeInTheDocument();
    expect(screen.getByText('Reviewed Actions inbox')).toBeInTheDocument();
    expect(screen.queryByText('Internal work')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Full release notes →' })).toHaveAttribute(
      'href',
      'https://github.com/rcourtman/Pulse/releases/tag/v6.1.0-rc.1',
    );
  });

  it('persists dismissal for the current release', async () => {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, '6.0.5');
    versionInfoMock.mockReturnValue({
      version: '6.1.0-rc.1',
      isDevelopment: false,
      isSourceBuild: false,
    });
    getReleaseNotesMock.mockResolvedValue({
      version: '6.1.0-rc.1',
      releaseNotes: '## Highlights\n- Reviewed Actions inbox',
      releaseDate: '2026-07-13T12:00:00Z',
      isPrerelease: true,
    });

    await renderCard();
    await waitFor(() => expect(screen.getByText('Got it')).toBeInTheDocument());

    fireEvent.click(screen.getByText('Got it'));

    await waitFor(() => {
      expect(screen.queryByTestId('whats-new-modal')).not.toBeInTheDocument();
    });
    expect(localStorage.getItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN)).toBe('6.1.0-rc.1');
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
