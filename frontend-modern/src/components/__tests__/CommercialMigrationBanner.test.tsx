import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { CommercialMigrationStatus } from '@/api/license';
import commercialMigrationBannerSource from '@/components/CommercialMigrationBanner.tsx?raw';

/* ------------------------------------------------------------------ */
/*  Mocks                                                              */
/* ------------------------------------------------------------------ */

const commercialPostureMock = vi.hoisted(() => vi.fn());
const commercialPostureLoadedMock = vi.hoisted(() => vi.fn());
const loadCommercialPostureMock = vi.hoisted(() => vi.fn());
const sessionPresentationPolicyResolvedMock = vi.hoisted(() => vi.fn());
const navigateMock = vi.hoisted(() => vi.fn());

vi.mock('@/stores/licenseCommercial', () => ({
  commercialPosture: () => commercialPostureMock(),
  commercialPostureLoaded: () => commercialPostureLoadedMock(),
  loadCommercialPosture: (force?: boolean) => loadCommercialPostureMock(force),
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  sessionPresentationPolicyResolved: () => sessionPresentationPolicyResolvedMock(),
}));

vi.mock('@solidjs/router', () => ({
  useNavigate: () => navigateMock,
}));

vi.mock('@/utils/logger', () => ({
  logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function setMigration(migration?: CommercialMigrationStatus) {
  commercialPostureMock.mockReturnValue(
    migration === undefined
      ? { tier: 'free', commercial_migration: undefined }
      : { tier: 'free', commercial_migration: migration },
  );
}

async function renderBanner() {
  const { CommercialMigrationBanner } = await import('../CommercialMigrationBanner');
  render(() => <CommercialMigrationBanner />);
}

describe('CommercialMigrationBanner', () => {
  beforeEach(() => {
    commercialPostureMock.mockReset();
    commercialPostureMock.mockReturnValue(null);
    commercialPostureLoadedMock.mockReset();
    commercialPostureLoadedMock.mockReturnValue(true);
    loadCommercialPostureMock.mockReset();
    sessionPresentationPolicyResolvedMock.mockReset();
    sessionPresentationPolicyResolvedMock.mockReturnValue(true);
    navigateMock.mockReset();
    sessionStorage.clear();
  });

  afterEach(cleanup);

  it('keeps commercial migration notice chrome on shared primitives', () => {
    expect(commercialMigrationBannerSource).toContain('InlineNotice');
    expect(commercialMigrationBannerSource).toContain('layout="banner"');
    expect(commercialMigrationBannerSource).toContain('lucide-solid/icons/alert-triangle');
    expect(commercialMigrationBannerSource).toContain('actionOnClick');
    expect(commercialMigrationBannerSource).not.toContain('<svg');
    expect(commercialMigrationBannerSource).not.toContain('<button');
    expect(commercialMigrationBannerSource).not.toContain('toneClasses');
    expect(commercialMigrationBannerSource).not.toContain('buttonClasses');
    expect(commercialMigrationBannerSource).not.toContain(
      'bg-amber-50 dark:bg-amber-900 border-b border-amber-200',
    );
    expect(commercialMigrationBannerSource).not.toContain(
      'bg-red-50 dark:bg-red-900 border-b border-red-200',
    );
    expect(commercialMigrationBannerSource).not.toContain(
      'p-1 rounded transition-colors opacity-70 hover:opacity-100',
    );
  });

  /* ---------- Visibility ---------- */

  it('stays hidden when there is no migration state', async () => {
    setMigration(undefined);

    await renderBanner();

    expect(screen.queryByText(/v5 license migration/)).not.toBeInTheDocument();
  });

  it('shows a pending migration with the panel notice copy', async () => {
    setMigration({
      source: 'v5_license',
      state: 'pending',
      reason: 'exchange_unavailable',
      recommended_action: 'retry_activation',
    });

    await renderBanner();

    expect(screen.getByText(/v5 license migration pending/)).toBeInTheDocument();
    expect(screen.getByText(/automatic v6 exchange did not complete yet/)).toBeInTheDocument();
  });

  it('shows a failed migration including the unreadable persisted license reason', async () => {
    setMigration({
      source: 'v5_license',
      state: 'failed',
      reason: 'persisted_license_unreadable',
      recommended_action: 'enter_supported_v5_key',
    });

    await renderBanner();

    expect(screen.getByText(/v5 license migration needs attention/)).toBeInTheDocument();
    expect(screen.getByText(/could not be read on this system/)).toBeInTheDocument();
  });

  /* ---------- Action ---------- */

  it('navigates to the license settings panel from the action button', async () => {
    setMigration({
      source: 'v5_license',
      state: 'pending',
      reason: 'exchange_unavailable',
      recommended_action: 'retry_activation',
    });

    await renderBanner();

    fireEvent.click(screen.getByText('Open license settings'));

    expect(navigateMock).toHaveBeenCalledWith('/settings/pulse-intelligence/billing/plan');
  });

  /* ---------- Dismiss ---------- */

  it('dismisses for the session, but resurfaces when the state changes', async () => {
    setMigration({
      source: 'v5_license',
      state: 'pending',
      reason: 'exchange_unavailable',
      recommended_action: 'retry_activation',
    });

    await renderBanner();

    fireEvent.click(screen.getByTitle('Dismiss for this session'));

    await waitFor(() => {
      expect(screen.queryByText(/v5 license migration pending/)).not.toBeInTheDocument();
    });
    expect(sessionStorage.getItem('commercialMigrationBannerDismissed:pending')).toBe('true');

    // A pending → failed transition must resurface the banner.
    setMigration({
      source: 'v5_license',
      state: 'failed',
      reason: 'exchange_invalid',
      recommended_action: 'enter_supported_v5_key',
    });
    cleanup();
    await renderBanner();

    expect(screen.getByText(/v5 license migration needs attention/)).toBeInTheDocument();
  });

  it('stays hidden when the current state was already dismissed this session', async () => {
    sessionStorage.setItem('commercialMigrationBannerDismissed:pending', 'true');
    setMigration({
      source: 'v5_license',
      state: 'pending',
      reason: 'exchange_unavailable',
      recommended_action: 'retry_activation',
    });

    await renderBanner();

    expect(screen.queryByText(/v5 license migration pending/)).not.toBeInTheDocument();
  });

  /* ---------- Data dependency ---------- */

  it('loads commercial posture itself when the app shell has not', async () => {
    commercialPostureLoadedMock.mockReturnValue(false);
    setMigration(undefined);

    await renderBanner();

    expect(loadCommercialPostureMock).toHaveBeenCalledWith(undefined);
  });

  it('does not reload posture when the app shell already loaded it', async () => {
    setMigration(undefined);

    await renderBanner();

    expect(loadCommercialPostureMock).not.toHaveBeenCalled();
  });

  it('defers the posture load until the presentation policy resolves', async () => {
    commercialPostureLoadedMock.mockReturnValue(false);
    sessionPresentationPolicyResolvedMock.mockReturnValue(false);
    setMigration(undefined);

    await renderBanner();

    expect(loadCommercialPostureMock).not.toHaveBeenCalled();
  });

  /* ---------- Self-healing ---------- */

  it('re-checks posture while pending so the banner clears after background migration', async () => {
    vi.useFakeTimers();
    try {
      setMigration({
        source: 'v5_license',
        state: 'pending',
        reason: 'exchange_unavailable',
        recommended_action: 'retry_activation',
      });

      await renderBanner();

      vi.advanceTimersByTime(61_000);
      expect(loadCommercialPostureMock).toHaveBeenCalledWith(true);
    } finally {
      vi.useRealTimers();
    }
  });
});
