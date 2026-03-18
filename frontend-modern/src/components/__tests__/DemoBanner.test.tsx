import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

/* ------------------------------------------------------------------ */
/*  Mocks                                                              */
/* ------------------------------------------------------------------ */

const apiFetchMock = vi.hoisted(() => vi.fn());

vi.mock('@/utils/apiClient', () => ({
  apiFetch: apiFetchMock,
}));

vi.mock('@/utils/logger', () => ({
  logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

/** Build a minimal Response-like object with the given demo header value. */
function fakeResponse(demoHeader: string | null) {
  const headers = new Headers();
  if (demoHeader !== null) {
    headers.set('X-Demo-Mode', demoHeader);
  }
  return { ok: true, headers };
}

/** Create a deferred promise whose resolution the test controls. */
function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

/* ------------------------------------------------------------------ */
/*  Tests                                                              */
/* ------------------------------------------------------------------ */

describe('DemoBanner', () => {
  beforeEach(() => {
    apiFetchMock.mockReset();
    sessionStorage.clear();
  });

  afterEach(cleanup);

  async function renderBanner() {
    const { DemoBanner } = await import('../DemoBanner');
    render(() => <DemoBanner />);
  }

  /* ---------- Visibility ---------- */

  it('shows the banner when X-Demo-Mode header is "true"', async () => {
    apiFetchMock.mockResolvedValue(fakeResponse('true'));

    await renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Demo instance with mock data (read-only)')).toBeInTheDocument();
    });
  });

  it('stays hidden when X-Demo-Mode header is absent', async () => {
    const d = deferred();
    apiFetchMock.mockReturnValue(d.promise);

    await renderBanner();

    // Banner must be hidden while request is in flight.
    expect(screen.queryByText('Demo instance with mock data (read-only)')).not.toBeInTheDocument();

    // Resolve with no demo header and verify banner stays hidden.
    d.resolve(fakeResponse(null));
    await waitFor(() => {
      expect(apiFetchMock).toHaveBeenCalledTimes(1);
    });
    expect(screen.queryByText('Demo instance with mock data (read-only)')).not.toBeInTheDocument();
  });

  it('stays hidden when X-Demo-Mode header is "false"', async () => {
    const d = deferred();
    apiFetchMock.mockReturnValue(d.promise);

    await renderBanner();

    expect(screen.queryByText('Demo instance with mock data (read-only)')).not.toBeInTheDocument();

    d.resolve(fakeResponse('false'));
    await waitFor(() => {
      expect(apiFetchMock).toHaveBeenCalledTimes(1);
    });
    expect(screen.queryByText('Demo instance with mock data (read-only)')).not.toBeInTheDocument();
  });

  it('stays hidden when the health check request fails', async () => {
    const d = deferred();
    apiFetchMock.mockReturnValue(d.promise);

    await renderBanner();

    expect(screen.queryByText('Demo instance with mock data (read-only)')).not.toBeInTheDocument();

    d.reject(new Error('network error'));
    await waitFor(() => {
      expect(apiFetchMock).toHaveBeenCalledTimes(1);
    });
    expect(screen.queryByText('Demo instance with mock data (read-only)')).not.toBeInTheDocument();
  });

  /* ---------- Dismiss ---------- */

  it('hides the banner when the dismiss button is clicked', async () => {
    apiFetchMock.mockResolvedValue(fakeResponse('true'));

    await renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Demo instance with mock data (read-only)')).toBeInTheDocument();
    });

    const dismissBtn = screen.getByTitle('Dismiss');
    fireEvent.click(dismissBtn);

    await waitFor(() => {
      expect(
        screen.queryByText('Demo instance with mock data (read-only)'),
      ).not.toBeInTheDocument();
    });
  });

  it('persists dismissal to sessionStorage', async () => {
    apiFetchMock.mockResolvedValue(fakeResponse('true'));

    await renderBanner();

    await waitFor(() => {
      expect(screen.getByText('Demo instance with mock data (read-only)')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTitle('Dismiss'));

    expect(sessionStorage.getItem('demoBannerDismissed')).toBe('true');
  });

  it('stays hidden when sessionStorage already has dismissal flag', async () => {
    sessionStorage.setItem('demoBannerDismissed', 'true');

    const d = deferred();
    apiFetchMock.mockReturnValue(d.promise);

    await renderBanner();

    expect(screen.queryByText('Demo instance with mock data (read-only)')).not.toBeInTheDocument();

    // Even when the API confirms demo mode, prior dismissal keeps the banner hidden.
    d.resolve(fakeResponse('true'));
    await waitFor(() => {
      expect(apiFetchMock).toHaveBeenCalledTimes(1);
    });
    expect(screen.queryByText('Demo instance with mock data (read-only)')).not.toBeInTheDocument();
  });

  /* ---------- API call ---------- */

  it('calls /api/health exactly once on mount', async () => {
    apiFetchMock.mockResolvedValue(fakeResponse(null));

    await renderBanner();

    await waitFor(() => {
      expect(apiFetchMock).toHaveBeenCalledTimes(1);
    });
    expect(apiFetchMock).toHaveBeenCalledWith('/api/health');
  });
});
