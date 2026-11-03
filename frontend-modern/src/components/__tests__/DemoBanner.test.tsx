import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

/* ------------------------------------------------------------------ */
/*  Mocks                                                              */
/* ------------------------------------------------------------------ */

const presentationPolicyIsDemoModeMock = vi.hoisted(() => vi.fn());

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsDemoMode: () => presentationPolicyIsDemoModeMock(),
}));

vi.mock('@/utils/logger', () => ({
  logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

/* ------------------------------------------------------------------ */
/*  Tests                                                              */
/* ------------------------------------------------------------------ */

describe('DemoBanner', () => {
  beforeEach(() => {
    presentationPolicyIsDemoModeMock.mockReset();
    presentationPolicyIsDemoModeMock.mockReturnValue(false);
    sessionStorage.clear();
  });

  afterEach(cleanup);

  async function renderBanner() {
    const { DemoBanner } = await import('../DemoBanner');
    render(() => <DemoBanner />);
  }

  /* ---------- Visibility ---------- */

  it('shows the banner when demo mode is enabled', async () => {
    presentationPolicyIsDemoModeMock.mockReturnValue(true);

    await renderBanner();

    expect(screen.getByText('Demo instance with mock data (read-only)')).toBeInTheDocument();
  });

  it('links the demo back to the install steps in a new tab', async () => {
    presentationPolicyIsDemoModeMock.mockReturnValue(true);

    await renderBanner();

    const link = screen.getByRole('link', { name: 'Run Pulse on your own hardware' });
    expect(link).toHaveAttribute('href', 'https://pulserelay.pro/#setup');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('stays hidden when demo mode is disabled', async () => {
    await renderBanner();

    expect(screen.queryByText('Demo instance with mock data (read-only)')).not.toBeInTheDocument();
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });

  /* ---------- Dismiss ---------- */

  it('hides the banner when the dismiss button is clicked', async () => {
    presentationPolicyIsDemoModeMock.mockReturnValue(true);

    await renderBanner();

    expect(screen.getByText('Demo instance with mock data (read-only)')).toBeInTheDocument();

    const dismissBtn = screen.getByTitle('Dismiss');
    fireEvent.click(dismissBtn);

    await waitFor(() => {
      expect(
        screen.queryByText('Demo instance with mock data (read-only)'),
      ).not.toBeInTheDocument();
    });
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });

  it('persists dismissal to sessionStorage', async () => {
    presentationPolicyIsDemoModeMock.mockReturnValue(true);

    await renderBanner();

    expect(screen.getByText('Demo instance with mock data (read-only)')).toBeInTheDocument();

    fireEvent.click(screen.getByTitle('Dismiss'));

    expect(sessionStorage.getItem('demoBannerDismissed')).toBe('true');
  });

  it('stays hidden when sessionStorage already has dismissal flag', async () => {
    sessionStorage.setItem('demoBannerDismissed', 'true');
    presentationPolicyIsDemoModeMock.mockReturnValue(true);

    await renderBanner();

    expect(screen.queryByText('Demo instance with mock data (read-only)')).not.toBeInTheDocument();
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });

  it('keeps the install handoff dismissed across remounts but restores it in a new session', async () => {
    presentationPolicyIsDemoModeMock.mockReturnValue(true);
    await renderBanner();
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss demo banner' }));
    cleanup();

    await renderBanner();
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
    cleanup();

    sessionStorage.clear();
    await renderBanner();
    expect(
      screen.getByRole('link', { name: 'Run Pulse on your own hardware' }),
    ).toBeInTheDocument();
    expect(screen.getByText('Demo instance with mock data (read-only)')).toBeInTheDocument();
  });
});
