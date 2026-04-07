import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { TrialBanner } from '@/components/shared/TrialBanner';
import trialBannerSource from '@/components/shared/TrialBanner.tsx?raw';
import trialBannerModelSource from '@/components/shared/trialBannerModel.ts?raw';
import trialBannerStateSource from '@/components/shared/useTrialBannerState.ts?raw';
import { TRIAL_BANNER_SNOOZE_KEY } from '@/components/shared/trialBannerModel';
import { getPublicPricingUrl } from '@/utils/pricingHandoff';

const {
  presentationPolicyHidesCommercialSurfacesMock,
  getUpgradeActionDestinationMock,
  getUpgradeActionUrlOrFallbackMock,
  commercialPostureMock,
  isUpsellSnoozedMock,
  snoozeUpsellMock,
} =
  vi.hoisted(() => ({
    presentationPolicyHidesCommercialSurfacesMock: vi.fn(),
    getUpgradeActionDestinationMock: vi.fn(),
    getUpgradeActionUrlOrFallbackMock: vi.fn(),
    commercialPostureMock: vi.fn(),
    isUpsellSnoozedMock: vi.fn(),
    snoozeUpsellMock: vi.fn(),
  }));

vi.mock('@/stores/licenseCommercial', () => ({
  commercialTrialDaysRemaining: () => commercialPostureMock()?.trial_days_remaining ?? null,
  getUpgradeActionDestination: (...args: unknown[]) => getUpgradeActionDestinationMock(...args),
  getUpgradeActionUrlOrFallback: (...args: unknown[]) => getUpgradeActionUrlOrFallbackMock(...args),
  commercialPosture: (...args: unknown[]) => commercialPostureMock(...args),
  isCommercialTrialActive: () => commercialPostureMock()?.subscription_state === 'trial',
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesCommercialSurfaces: () =>
    presentationPolicyHidesCommercialSurfacesMock(),
}));

vi.mock('@/utils/snooze', () => ({
  isUpsellSnoozed: (...args: unknown[]) => isUpsellSnoozedMock(...args),
  snoozeUpsell: (...args: unknown[]) => snoozeUpsellMock(...args),
}));

describe('TrialBanner', () => {
  const renderBanner = () => render(() => (
    <Router>
      <Route path="/" component={() => <TrialBanner />} />
    </Router>
  ));

  beforeEach(() => {
    cleanup();
    presentationPolicyHidesCommercialSurfacesMock.mockReset();
    getUpgradeActionDestinationMock.mockReset();
    getUpgradeActionUrlOrFallbackMock.mockReset();
    commercialPostureMock.mockReset();
    isUpsellSnoozedMock.mockReset();
    snoozeUpsellMock.mockReset();
    getUpgradeActionDestinationMock.mockReturnValue({
      href: getPublicPricingUrl('trial_banner'),
      external: true,
    });
    presentationPolicyHidesCommercialSurfacesMock.mockReturnValue(false);
    getUpgradeActionUrlOrFallbackMock.mockReturnValue(getPublicPricingUrl('trial_banner'));
    isUpsellSnoozedMock.mockReturnValue(false);
  });

  afterEach(() => {
    cleanup();
  });

  it('keeps trial banner on shell, runtime, and model owners', () => {
    expect(trialBannerSource).toContain('useTrialBannerState');
    expect(trialBannerSource).toContain('TRIAL_BANNER_TITLE');
    expect(trialBannerSource).not.toContain('createSignal');
    expect(trialBannerSource).not.toContain('createMemo');
    expect(trialBannerSource).not.toContain('loadRuntimeCapabilities');
    expect(trialBannerSource).not.toContain('licenseStatus');
    expect(trialBannerSource).not.toContain('getUpgradeActionUrlOrFallback');

    expect(trialBannerStateSource).toContain('export function useTrialBannerState');
    expect(trialBannerStateSource).toContain('createSignal');
    expect(trialBannerStateSource).toContain('createMemo');
    expect(trialBannerStateSource).not.toContain('loadCommercialPosture');
    expect(trialBannerStateSource).toContain('presentationPolicyHidesCommercialSurfaces');
    expect(trialBannerStateSource).toContain('isCommercialTrialActive');
    expect(trialBannerStateSource).toContain('commercialTrialDaysRemaining');
    expect(trialBannerStateSource).toContain('getUpgradeActionDestination');
    expect(trialBannerStateSource).toContain('snoozeUpsell');

    expect(trialBannerModelSource).toContain('TRIAL_BANNER_SNOOZE_KEY');
    expect(trialBannerModelSource).toContain('normalizeTrialBannerDaysRemaining');
    expect(trialBannerModelSource).toContain('getTrialBannerToneClass');
    expect(trialBannerModelSource).toContain('getTrialBannerStatusLabel');
    expect(trialBannerModelSource).toContain('TRIAL_BANNER_UPGRADE_LABEL');
  });

  it('renders trial details from shared commercial posture when active', async () => {
    commercialPostureMock.mockReturnValue({
      subscription_state: 'trial',
      trial_days_remaining: 4.8,
    });

    renderBanner();
    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(screen.getByText('Pro Trial:')).toBeInTheDocument();
    expect(screen.getByText('4 days remaining')).toBeInTheDocument();
    expect(screen.getByText('Upgrade').closest('a')).toHaveAttribute(
      'href',
      getPublicPricingUrl('trial_banner'),
    );
  });

  it('shows active fallback when trial days are unavailable', () => {
    commercialPostureMock.mockReturnValue({
      subscription_state: 'trial',
      trial_days_remaining: undefined,
    });

    renderBanner();

    expect(screen.getByText('Active')).toBeInTheDocument();
  });

  it('stays hidden in demo mode even when the workspace is on trial', () => {
    presentationPolicyHidesCommercialSurfacesMock.mockReturnValue(true);
    commercialPostureMock.mockReturnValue({
      subscription_state: 'trial',
      trial_days_remaining: 2,
    });

    renderBanner();

    expect(screen.queryByRole('status')).toBeNull();
  });

  it('snoozes and hides the action row', async () => {
    commercialPostureMock.mockReturnValue({
      subscription_state: 'trial',
      trial_days_remaining: 2,
    });

    renderBanner();

    fireEvent.click(screen.getByRole('button', { name: 'Snooze 7d' }));

    expect(snoozeUpsellMock).toHaveBeenCalledWith(TRIAL_BANNER_SNOOZE_KEY);
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: 'Snooze 7d' })).toBeNull();
    });
  });
});
