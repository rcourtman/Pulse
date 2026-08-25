import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
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
const LOCAL_DISMISS_BUTTON_CLASS =
  'inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content';
const LOCAL_PRIMARY_BUTTON_CLASS =
  'inline-flex min-h-9 items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700';

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
  localStorage.setItem(ACTIVE_DAYS_KEY, '13');
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

    expect(screen.queryByText('Finding Pulse useful?')).not.toBeInTheDocument();
    expect(localStorage.getItem(ACTIVE_DAYS_KEY)).toBe('0');
  });

  it('waits for websocket initial data before recording engagement', async () => {
    mockInitialDataReceived.mockReturnValue(false);
    setResourceCount(3);

    await renderBanner();

    expect(screen.queryByText('Finding Pulse useful?')).not.toBeInTheDocument();
    expect(localStorage.getItem(ACTIVE_DAYS_KEY)).toBe('0');
  });

  it('counts one distinct active day without showing the prompt', async () => {
    setResourceCount(3);

    await renderBanner();

    await waitFor(() => expect(localStorage.getItem(ACTIVE_DAYS_KEY)).toBe('1'));
    expect(localStorage.getItem(LAST_ACTIVE_DATE_KEY)).toBe('2026-03-14');
    expect(screen.queryByText('Finding Pulse useful?')).not.toBeInTheDocument();
  });

  it('does not count repeated sessions on the same day', async () => {
    localStorage.setItem(ACTIVE_DAYS_KEY, '7');
    localStorage.setItem(LAST_ACTIVE_DATE_KEY, '2026-03-14');
    setResourceCount(3);

    await renderBanner();

    expect(localStorage.getItem(ACTIVE_DAYS_KEY)).toBe('7');
    expect(screen.queryByText('Finding Pulse useful?')).not.toBeInTheDocument();
  });

  it('shows one compact prompt after fourteen distinct active days', async () => {
    seedEligibleEngagement();
    setResourceCount(3);

    await renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Finding Pulse useful?')).toBeInTheDocument();
    });
    expect(
      screen.getByText('Starring the project on GitHub helps others discover it.'),
    ).toBeInTheDocument();
    expect(screen.getByText('Star on GitHub')).toBeInTheDocument();
    expect(screen.queryByText('Maybe later')).not.toBeInTheDocument();
    expect(localStorage.getItem(ACTIVE_DAYS_KEY)).toBe('14');
    expect(localStorage.getItem(PROMPT_SHOWN_KEY)).toBe('true');
  });

  it('never reopens after its single lifetime appearance', async () => {
    localStorage.setItem(PROMPT_SHOWN_KEY, 'true');
    localStorage.setItem(ACTIVE_DAYS_KEY, '14');
    localStorage.setItem(LAST_ACTIVE_DATE_KEY, '2026-03-13');
    setResourceCount(3);

    await renderBanner();

    expect(screen.queryByText('Finding Pulse useful?')).not.toBeInTheDocument();
  });

  it('stays quiet when another low-priority notice owns the session', async () => {
    sessionStorage.setItem(LOW_PRIORITY_NOTICE_OWNER_KEY, 'release-update');
    seedEligibleEngagement();
    setResourceCount(3);

    await renderBanner();

    await waitFor(() => expect(localStorage.getItem(ACTIVE_DAYS_KEY)).toBe('14'));
    expect(screen.queryByText('Finding Pulse useful?')).not.toBeInTheDocument();
    expect(localStorage.getItem(PROMPT_SHOWN_KEY)).not.toBe('true');
  });

  it('permanently dismisses the prompt from the close action', async () => {
    seedEligibleEngagement();
    setResourceCount(3);

    await renderBanner();
    await waitFor(() => expect(screen.getByText('Finding Pulse useful?')).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText("Close and don't show again"));

    await waitFor(() => {
      expect(screen.queryByText('Finding Pulse useful?')).not.toBeInTheDocument();
    });
    expect(localStorage.getItem(DISMISSED_KEY)).toBe('true');
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

  it('uses runtime state and remains a non-blocking app-shell prompt', () => {
    expect(gitHubStarBannerSource).toContain('useWebSocket');
    expect(gitHubStarBannerSource).toContain('initialDataReceived');
    expect(gitHubStarBannerSource).not.toContain('useResources');
    expect(gitHubStarBannerSource).not.toContain('<Dialog');
    expect(gitHubStarBannerSource).toContain('dialogStackHasBlockingDialog');
    expect(gitHubStarBannerSource).toContain('reserveLowPriorityNoticeSession');
    expect(gitHubStarBannerSource).toContain('aria-live="polite"');
    expect(gitHubStarBannerSource).toContain('z-30');
  });

  it('routes prompt actions through shared Button primitives', () => {
    expect(gitHubStarBannerSource).toContain('@/components/shared/Button');
    expect(gitHubStarBannerSource).toContain('<ActionIconButton');
    expect(gitHubStarBannerSource).toContain('<Button');
    expect(gitHubStarBannerSource).not.toContain(LOCAL_DISMISS_BUTTON_CLASS);
    expect(gitHubStarBannerSource).not.toContain(LOCAL_PRIMARY_BUTTON_CLASS);
  });
});

describe('GitHubStarBanner mobile navigation clearance', () => {
  it('reads the published bottom navigation height', () => {
    expect(gitHubStarBannerSource).toContain('bottom-[var(--pulse-mobile-nav-height)]');
  });

  it('does not keep its own copy of the bar height', () => {
    expect(gitHubStarBannerSource).not.toMatch(/5rem\s*\+\s*env\(safe-area-inset-bottom/);
  });

  it('keeps its own desktop placement once the bar is gone', () => {
    expect(gitHubStarBannerSource).toContain('md:bottom-4');
  });
});
