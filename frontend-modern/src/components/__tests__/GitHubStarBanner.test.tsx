import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

/* ------------------------------------------------------------------ */
/*  Mocks                                                              */
/* ------------------------------------------------------------------ */

type BannerResource = { id: string; name: string };

const mockResources = vi.hoisted(() => vi.fn<() => BannerResource[]>(() => []));

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    resources: mockResources,
  }),
}));

vi.mock('@/utils/logger', () => ({
  logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const DISMISSED_KEY = 'pulse-github-star-dismissed';
const FIRST_SEEN_KEY = 'pulse-github-star-first-seen';
const SNOOZED_KEY = 'pulse-github-star-snoozed-until';

async function renderBanner() {
  const mod = await import('../GitHubStarBanner');
  render(() => <mod.GitHubStarBanner />);
}

/** Set mockResources to return N fake resources. */
function setResourceCount(n: number) {
  const resources = Array.from({ length: n }, (_, i) => ({
    id: `res-${i}`,
    name: `Resource ${i}`,
  }));
  mockResources.mockReturnValue(resources);
}

/* ------------------------------------------------------------------ */
/*  Tests                                                              */
/* ------------------------------------------------------------------ */

describe('GitHubStarBanner', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockResources.mockReturnValue([]);
    localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
    cleanup();
  });

  /* ---------- Visibility: not shown scenarios ---------- */

  it('does not render when there are no resources', async () => {
    mockResources.mockReturnValue([]);

    await renderBanner();

    expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
  });

  it('does not render on the first day infrastructure is seen (records first-seen date)', async () => {
    vi.setSystemTime(new Date('2026-03-01T12:00:00Z'));
    setResourceCount(3);

    await renderBanner();

    // Should not show modal
    expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();

    // Should have recorded today as first-seen date
    expect(localStorage.getItem(FIRST_SEEN_KEY)).toBe('2026-03-01');
  });

  it('does not render when first-seen date is today (same day)', async () => {
    vi.setSystemTime(new Date('2026-03-01T12:00:00Z'));
    localStorage.setItem(FIRST_SEEN_KEY, '2026-03-01');
    setResourceCount(3);

    await renderBanner();

    expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
  });

  it('does not render when permanently dismissed', async () => {
    vi.setSystemTime(new Date('2026-03-05T12:00:00Z'));
    localStorage.setItem(FIRST_SEEN_KEY, '2026-03-01');
    localStorage.setItem(DISMISSED_KEY, 'true');
    setResourceCount(3);

    await renderBanner();

    expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
  });

  it('does not render when within snooze period', async () => {
    vi.setSystemTime(new Date('2026-03-05T12:00:00Z'));
    localStorage.setItem(FIRST_SEEN_KEY, '2026-03-01');
    localStorage.setItem(SNOOZED_KEY, '2026-03-10'); // snoozed until the 10th
    setResourceCount(3);

    await renderBanner();

    expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
  });

  /* ---------- Visibility: shown scenarios ---------- */

  it('renders when returning user has infrastructure (different day than first seen)', async () => {
    vi.setSystemTime(new Date('2026-03-05T12:00:00Z'));
    localStorage.setItem(FIRST_SEEN_KEY, '2026-03-01');
    setResourceCount(3);

    await renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Enjoying Pulse?')).toBeInTheDocument();
    });
  });

  it('renders when snooze period has expired', async () => {
    vi.setSystemTime(new Date('2026-03-15T12:00:00Z'));
    localStorage.setItem(FIRST_SEEN_KEY, '2026-03-01');
    localStorage.setItem(SNOOZED_KEY, '2026-03-10'); // expired 5 days ago
    setResourceCount(3);

    await renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Enjoying Pulse?')).toBeInTheDocument();
    });
  });

  /* ---------- Content ---------- */

  it('displays the expected content when shown', async () => {
    vi.setSystemTime(new Date('2026-03-05T12:00:00Z'));
    localStorage.setItem(FIRST_SEEN_KEY, '2026-03-01');
    setResourceCount(3);

    await renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Enjoying Pulse?')).toBeInTheDocument();
    });
    expect(screen.getByText(/independent developer/)).toBeInTheDocument();
    expect(screen.getByText('Star on GitHub')).toBeInTheDocument();
    expect(screen.getByText('Maybe later')).toBeInTheDocument();
    expect(screen.getByLabelText("Close and don't show again")).toBeInTheDocument();
  });

  /* ---------- Dismiss (X button) ---------- */

  it('hides the modal and persists dismissal when the X button is clicked', async () => {
    vi.setSystemTime(new Date('2026-03-05T12:00:00Z'));
    localStorage.setItem(FIRST_SEEN_KEY, '2026-03-01');
    setResourceCount(3);

    await renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Enjoying Pulse?')).toBeInTheDocument();
    });

    const closeBtn = screen.getByLabelText("Close and don't show again");
    fireEvent.click(closeBtn);

    await waitFor(() => {
      expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
    });

    expect(localStorage.getItem(DISMISSED_KEY)).toBe('true');
  });

  /* ---------- Star on GitHub ---------- */

  it('opens GitHub repo, dismisses permanently on star click', async () => {
    vi.setSystemTime(new Date('2026-03-05T12:00:00Z'));
    localStorage.setItem(FIRST_SEEN_KEY, '2026-03-01');
    setResourceCount(3);

    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);

    await renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Star on GitHub')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Star on GitHub'));

    expect(openSpy).toHaveBeenCalledWith(
      'https://github.com/rcourtman/Pulse',
      '_blank',
      'noopener,noreferrer',
    );

    await waitFor(() => {
      expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
    });

    // Permanent dismissal
    expect(localStorage.getItem(DISMISSED_KEY)).toBe('true');
  });

  /* ---------- Maybe later (snooze) ---------- */

  it('hides the modal and sets a 7-day snooze when "Maybe later" is clicked', async () => {
    vi.setSystemTime(new Date('2026-03-05T12:00:00Z'));
    localStorage.setItem(FIRST_SEEN_KEY, '2026-03-01');
    setResourceCount(3);

    await renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Maybe later')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Maybe later'));

    await waitFor(() => {
      expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
    });

    // Should snooze for 7 days from today
    expect(localStorage.getItem(SNOOZED_KEY)).toBe('2026-03-12');

    // Should NOT permanently dismiss
    expect(localStorage.getItem(DISMISSED_KEY)).not.toBe('true');
  });

  it('uses the shared dialog close path to snooze on Escape', async () => {
    vi.setSystemTime(new Date('2026-03-05T12:00:00Z'));
    localStorage.setItem(FIRST_SEEN_KEY, '2026-03-01');
    setResourceCount(3);

    await renderBanner();

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    fireEvent.keyDown(document, { key: 'Escape' });

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    expect(localStorage.getItem(SNOOZED_KEY)).toBe('2026-03-12');
    expect(localStorage.getItem(DISMISSED_KEY)).not.toBe('true');
  });

  /* ---------- Edge cases ---------- */

  it('renders on the exact snooze expiry date (snooze date == today)', async () => {
    vi.setSystemTime(new Date('2026-03-10T12:00:00Z'));
    localStorage.setItem(FIRST_SEEN_KEY, '2026-03-01');
    localStorage.setItem(SNOOZED_KEY, '2026-03-10'); // today === snooze date, so today < snooze is false
    setResourceCount(3);

    await renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Enjoying Pulse?')).toBeInTheDocument();
    });
  });

  it('does not write first-seen date when there are no resources', async () => {
    vi.setSystemTime(new Date('2026-03-01T12:00:00Z'));
    mockResources.mockReturnValue([]);

    await renderBanner();

    // The localStorage signal initializes with '' and syncs it, but the
    // component never sets it to a real date when there are no resources.
    const stored = localStorage.getItem(FIRST_SEEN_KEY);
    expect(stored === null || stored === '').toBe(true);
  });

  it('stays dismissed across re-renders after clicking X', async () => {
    vi.setSystemTime(new Date('2026-03-05T12:00:00Z'));
    localStorage.setItem(FIRST_SEEN_KEY, '2026-03-01');
    setResourceCount(3);

    await renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Enjoying Pulse?')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByLabelText("Close and don't show again"));

    await waitFor(() => {
      expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
    });

    // Verify localStorage has dismissed flag persisted
    expect(localStorage.getItem(DISMISSED_KEY)).toBe('true');

    // Clean up and re-render — should stay hidden due to persisted dismissal
    cleanup();
    await renderBanner();

    await waitFor(() => {
      expect(screen.queryByText('Enjoying Pulse?')).not.toBeInTheDocument();
    });
  });
});
