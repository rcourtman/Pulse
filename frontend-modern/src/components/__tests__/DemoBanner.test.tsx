import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

/* ------------------------------------------------------------------ */
/*  Mocks                                                              */
/* ------------------------------------------------------------------ */

const demoModeEnabledMock = vi.hoisted(() => vi.fn());
const ensureDemoModeResolvedMock = vi.hoisted(() => vi.fn());

vi.mock('@/stores/demoMode', () => ({
  demoModeEnabled: () => demoModeEnabledMock(),
  ensureDemoModeResolved: (...args: unknown[]) => ensureDemoModeResolvedMock(...args),
}));

vi.mock('@/utils/logger', () => ({
  logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

/* ------------------------------------------------------------------ */
/*  Tests                                                              */
/* ------------------------------------------------------------------ */

describe('DemoBanner', () => {
  beforeEach(() => {
    demoModeEnabledMock.mockReset();
    ensureDemoModeResolvedMock.mockReset();
    demoModeEnabledMock.mockReturnValue(false);
    ensureDemoModeResolvedMock.mockResolvedValue(false);
    sessionStorage.clear();
  });

  afterEach(cleanup);

  async function renderBanner() {
    const { DemoBanner } = await import('../DemoBanner');
    render(() => <DemoBanner />);
  }

  /* ---------- Visibility ---------- */

  it('shows the banner when demo mode is enabled', async () => {
    demoModeEnabledMock.mockReturnValue(true);

    await renderBanner();

    expect(screen.getByText('Demo instance with mock data (read-only)')).toBeInTheDocument();
  });

  it('stays hidden when demo mode is disabled', async () => {
    await renderBanner();

    expect(screen.queryByText('Demo instance with mock data (read-only)')).not.toBeInTheDocument();
  });

  /* ---------- Dismiss ---------- */

  it('hides the banner when the dismiss button is clicked', async () => {
    demoModeEnabledMock.mockReturnValue(true);

    await renderBanner();

    expect(screen.getByText('Demo instance with mock data (read-only)')).toBeInTheDocument();

    const dismissBtn = screen.getByTitle('Dismiss');
    fireEvent.click(dismissBtn);

    await waitFor(() => {
      expect(
        screen.queryByText('Demo instance with mock data (read-only)'),
      ).not.toBeInTheDocument();
    });
  });

  it('persists dismissal to sessionStorage', async () => {
    demoModeEnabledMock.mockReturnValue(true);

    await renderBanner();

    expect(screen.getByText('Demo instance with mock data (read-only)')).toBeInTheDocument();

    fireEvent.click(screen.getByTitle('Dismiss'));

    expect(sessionStorage.getItem('demoBannerDismissed')).toBe('true');
  });

  it('stays hidden when sessionStorage already has dismissal flag', async () => {
    sessionStorage.setItem('demoBannerDismissed', 'true');
    demoModeEnabledMock.mockReturnValue(true);

    await renderBanner();

    expect(screen.queryByText('Demo instance with mock data (read-only)')).not.toBeInTheDocument();
  });

  /* ---------- Demo-mode resolve ---------- */

  it('resolves demo mode exactly once on mount', async () => {
    ensureDemoModeResolvedMock.mockResolvedValue(false);

    await renderBanner();

    await waitFor(() => {
      expect(ensureDemoModeResolvedMock).toHaveBeenCalledTimes(1);
    });
  });
});
