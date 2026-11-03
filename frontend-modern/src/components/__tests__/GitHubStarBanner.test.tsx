import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import gitHubStarBannerSource from '../GitHubStarBanner.tsx?raw';

type BannerResource = { id: string; name: string };

const wsState = vi.hoisted(() => ({ resources: [] as BannerResource[] }));
const mockInitialDataReceived = vi.hoisted(() => vi.fn<() => boolean>(() => true));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    state: wsState,
    initialDataReceived: mockInitialDataReceived,
  }),
}));

vi.mock('@/utils/logger', () => ({
  logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

const DISMISSED_KEY = 'pulse-github-star-dismissed';
const PROMPT_SHOWN_KEY = 'pulse-github-star-prompt-shown';
const ACTIVE_DAYS_KEY = 'pulse-github-star-active-days';
const LAST_ACTIVE_DATE_KEY = 'pulse-github-star-last-active-date';
const LOW_PRIORITY_NOTICE_OWNER_KEY = 'pulse-low-priority-notice-owner';

async function renderBanner() {
  const mod = await import('../GitHubStarBanner');
  render(() => <mod.GitHubStarBanner />);
}

function setResourceCount(count: number) {
  wsState.resources = Array.from({ length: count }, (_, index) => ({
    id: `res-${index}`,
    name: `Resource ${index}`,
  }));
}

function seedEligibleEngagement() {
  localStorage.setItem(ACTIVE_DAYS_KEY, '1');
  localStorage.setItem(LAST_ACTIVE_DATE_KEY, '2026-03-13');
}

describe('GitHubStarBanner', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-03-14T12:00:00Z'));
    wsState.resources = [];
    mockInitialDataReceived.mockReturnValue(true);
    localStorage.clear();
    sessionStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
    cleanup();
  });

  it('stays quiet without connected infrastructure', async () => {
    await renderBanner();

    expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
    expect(localStorage.getItem(ACTIVE_DAYS_KEY)).toBe('0');
  });

  it('waits for websocket initial data before recording engagement', async () => {
    mockInitialDataReceived.mockReturnValue(false);
    setResourceCount(3);

    await renderBanner();

    expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
    expect(localStorage.getItem(ACTIVE_DAYS_KEY)).toBe('0');
  });

  it('counts one distinct active day without showing the prompt', async () => {
    setResourceCount(3);

    await renderBanner();

    await waitFor(() => expect(localStorage.getItem(ACTIVE_DAYS_KEY)).toBe('1'));
    expect(localStorage.getItem(LAST_ACTIVE_DATE_KEY)).toBe('2026-03-14');
    expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
  });

  it('does not count repeated sessions on the same day', async () => {
    localStorage.setItem(ACTIVE_DAYS_KEY, '1');
    localStorage.setItem(LAST_ACTIVE_DATE_KEY, '2026-03-14');
    setResourceCount(3);

    await renderBanner();

    expect(localStorage.getItem(ACTIVE_DAYS_KEY)).toBe('1');
    expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
  });

  it('shows one compact prompt on the first return day with connected infrastructure', async () => {
    seedEligibleEngagement();
    setResourceCount(3);

    await renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Enjoying Pulse?')).toBeInTheDocument();
    });
    expect(
      screen.getByText(
        'If Pulse is useful to you, a GitHub star helps others discover it and shows your support for the project.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('Star on GitHub')).toBeInTheDocument();
    expect(screen.queryByText('Maybe later')).not.toBeInTheDocument();
    expect(screen.getByText('Enjoying Pulse?').closest('[aria-live]')).toHaveAttribute(
      'aria-live',
      'polite',
    );
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(localStorage.getItem(ACTIVE_DAYS_KEY)).toBe('2');
    expect(localStorage.getItem(PROMPT_SHOWN_KEY)).toBe('true');
  });

  it('never reopens after its single lifetime appearance', async () => {
    localStorage.setItem(PROMPT_SHOWN_KEY, 'true');
    localStorage.setItem(ACTIVE_DAYS_KEY, '2');
    localStorage.setItem(LAST_ACTIVE_DATE_KEY, '2026-03-13');
    setResourceCount(3);

    await renderBanner();

    expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
  });

  it('stays quiet when another low-priority notice owns the session', async () => {
    sessionStorage.setItem(LOW_PRIORITY_NOTICE_OWNER_KEY, 'release-update');
    seedEligibleEngagement();
    setResourceCount(3);

    await renderBanner();

    await waitFor(() => expect(localStorage.getItem(ACTIVE_DAYS_KEY)).toBe('2'));
    expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
    expect(localStorage.getItem(PROMPT_SHOWN_KEY)).not.toBe('true');
  });

  it('permanently dismisses the prompt from the close action', async () => {
    seedEligibleEngagement();
    setResourceCount(3);

    await renderBanner();
    await waitFor(() => expect(screen.getByText('Enjoying Pulse?')).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText("Close and don't show again"));

    await waitFor(() => {
      expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
    });
    expect(localStorage.getItem(DISMISSED_KEY)).toBe('true');

    cleanup();
    sessionStorage.clear();
    vi.setSystemTime(new Date('2026-03-15T12:00:00Z'));
    await renderBanner();

    expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
  });

  it('yields a visible prompt to a blocking dialog for the rest of the session', async () => {
    seedEligibleEngagement();
    setResourceCount(3);
    const { GitHubStarBanner } = await import('../GitHubStarBanner');
    const { Dialog } = await import('../shared/Dialog');
    let openDialog = () => {};

    render(() => {
      const [isOpen, setIsOpen] = createSignal(false);
      openDialog = () => setIsOpen(true);
      return (
        <>
          <GitHubStarBanner />
          <Dialog isOpen={isOpen()} onClose={() => setIsOpen(false)} ariaLabel="Test dialog">
            <button>Dialog action</button>
          </Dialog>
        </>
      );
    });

    await waitFor(() => expect(screen.getByText('Enjoying Pulse?')).toBeInTheDocument());
    openDialog();
    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();

    fireEvent.keyDown(document, { key: 'Escape' });
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
  });

  it('opens GitHub and permanently dismisses on the star action', async () => {
    seedEligibleEngagement();
    setResourceCount(3);
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);

    await renderBanner();
    await waitFor(() => expect(screen.getByText('Star on GitHub')).toBeInTheDocument());

    fireEvent.click(screen.getByText('Star on GitHub'));

    expect(openSpy).toHaveBeenCalledWith(
      'https://github.com/rcourtman/Pulse',
      '_blank',
      'noopener,noreferrer',
    );
    expect(localStorage.getItem(DISMISSED_KEY)).toBe('true');
  });
});

describe('GitHubStarBanner mobile navigation clearance', () => {
  it('reads the published bottom navigation height', () => {
    expect(gitHubStarBannerSource).toContain('bottom-(--pulse-mobile-nav-height)');
  });
});
