import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { TrialBanner } from '@/components/shared/TrialBanner';
import trialBannerSource from '@/components/shared/TrialBanner.tsx?raw';
import trialBannerModelSource from '@/components/shared/trialBannerModel.ts?raw';
import trialBannerStateSource from '@/components/shared/useTrialBannerState.ts?raw';
import { TRIAL_BANNER_SNOOZE_KEY } from '@/components/shared/trialBannerModel';
import { getPublicPricingUrl } from '@/utils/pricingHandoff';

const {
  demoModeEnabledMock,
  ensureDemoModeResolvedMock,
  getUpgradeActionDestinationMock,
  getUpgradeActionUrlOrFallbackMock,
  licenseStatusMock,
  loadLicenseStatusMock,
  isUpsellSnoozedMock,
  snoozeUpsellMock,
} =
  vi.hoisted(() => ({
    demoModeEnabledMock: vi.fn(),
    ensureDemoModeResolvedMock: vi.fn(),
    getUpgradeActionDestinationMock: vi.fn(),
    getUpgradeActionUrlOrFallbackMock: vi.fn(),
    licenseStatusMock: vi.fn(),
    loadLicenseStatusMock: vi.fn(),
    isUpsellSnoozedMock: vi.fn(),
    snoozeUpsellMock: vi.fn(),
  }));

vi.mock('@/stores/license', () => ({
  getUpgradeActionDestination: (...args: unknown[]) => getUpgradeActionDestinationMock(...args),
  getUpgradeActionUrlOrFallback: (...args: unknown[]) => getUpgradeActionUrlOrFallbackMock(...args),
  licenseStatus: (...args: unknown[]) => licenseStatusMock(...args),
  loadLicenseStatus: (...args: unknown[]) => loadLicenseStatusMock(...args),
}));

vi.mock('@/stores/demoMode', () => ({
  demoModeEnabled: () => demoModeEnabledMock(),
  ensureDemoModeResolved: (...args: unknown[]) => ensureDemoModeResolvedMock(...args),
}));

vi.mock('@/utils/snooze', () => ({
  isUpsellSnoozed: (...args: unknown[]) => isUpsellSnoozedMock(...args),
  snoozeUpsell: (...args: unknown[]) => snoozeUpsellMock(...args),
}));

describe('TrialBanner', () => {
  beforeEach(() => {
    cleanup();
    demoModeEnabledMock.mockReset();
    ensureDemoModeResolvedMock.mockReset();
    getUpgradeActionDestinationMock.mockReset();
    getUpgradeActionUrlOrFallbackMock.mockReset();
    licenseStatusMock.mockReset();
    loadLicenseStatusMock.mockReset();
    isUpsellSnoozedMock.mockReset();
    snoozeUpsellMock.mockReset();
    getUpgradeActionDestinationMock.mockReturnValue({
      href: getPublicPricingUrl('trial_banner'),
      external: true,
    });
    demoModeEnabledMock.mockReturnValue(false);
    ensureDemoModeResolvedMock.mockResolvedValue(false);
    getUpgradeActionUrlOrFallbackMock.mockReturnValue(getPublicPricingUrl('trial_banner'));
    loadLicenseStatusMock.mockResolvedValue(undefined);
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
    expect(trialBannerSource).not.toContain('loadLicenseStatus');
    expect(trialBannerSource).not.toContain('licenseStatus');
    expect(trialBannerSource).not.toContain('getUpgradeActionUrlOrFallback');

    expect(trialBannerStateSource).toContain('export function useTrialBannerState');
    expect(trialBannerStateSource).toContain('createSignal');
    expect(trialBannerStateSource).toContain('createMemo');
    expect(trialBannerStateSource).toContain('loadLicenseStatus');
    expect(trialBannerStateSource).toContain('ensureDemoModeResolved');
    expect(trialBannerStateSource).toContain('licenseStatus');
    expect(trialBannerStateSource).toContain('getUpgradeActionDestination');
    expect(trialBannerStateSource).toContain('snoozeUpsell');

    expect(trialBannerModelSource).toContain('TRIAL_BANNER_SNOOZE_KEY');
    expect(trialBannerModelSource).toContain('normalizeTrialBannerDaysRemaining');
    expect(trialBannerModelSource).toContain('getTrialBannerToneClass');
    expect(trialBannerModelSource).toContain('getTrialBannerStatusLabel');
    expect(trialBannerModelSource).toContain('TRIAL_BANNER_UPGRADE_LABEL');
  });

  it('loads license status on mount and renders trial details when active', async () => {
    licenseStatusMock.mockReturnValue({
      subscription_state: 'trial',
      trial_days_remaining: 4.8,
    });

    render(() => <TrialBanner />);

    await waitFor(() => {
      expect(loadLicenseStatusMock).toHaveBeenCalled();
      expect(ensureDemoModeResolvedMock).toHaveBeenCalled();
    });
    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(screen.getByText('Pro Trial:')).toBeInTheDocument();
    expect(screen.getByText('4 days remaining')).toBeInTheDocument();
    expect(screen.getByText('Upgrade').closest('a')).toHaveAttribute(
      'href',
      getPublicPricingUrl('trial_banner'),
    );
  });

  it('shows active fallback when trial days are unavailable', () => {
    licenseStatusMock.mockReturnValue({
      subscription_state: 'trial',
      trial_days_remaining: undefined,
    });

    render(() => <TrialBanner />);

    expect(screen.getByText('Active')).toBeInTheDocument();
  });

  it('stays hidden in demo mode even when the workspace is on trial', () => {
    demoModeEnabledMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({
      subscription_state: 'trial',
      trial_days_remaining: 2,
    });

    render(() => <TrialBanner />);

    expect(screen.queryByRole('status')).toBeNull();
  });

  it('snoozes and hides the action row', async () => {
    licenseStatusMock.mockReturnValue({
      subscription_state: 'trial',
      trial_days_remaining: 2,
    });

    render(() => <TrialBanner />);

    fireEvent.click(screen.getByRole('button', { name: 'Snooze 7d' }));

    expect(snoozeUpsellMock).toHaveBeenCalledWith(TRIAL_BANNER_SNOOZE_KEY);
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: 'Snooze 7d' })).toBeNull();
    });
  });
});
