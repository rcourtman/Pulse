import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';

import type { LicenseEntitlements } from '@/api/license';
import { ProLicensePanel } from '../ProLicensePanel';

let mockEntitlements: LicenseEntitlements | null = null;

const loadLicenseStatusMock = vi.fn();
const startProTrialMock = vi.fn();
const activateLicenseMock = vi.fn();
const clearLicenseMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const trackUpgradeMetricEventMock = vi.fn();

vi.mock('@/stores/license', () => ({
  getUpgradeActionUrlOrFallback: () => '/pricing',
  isMultiTenantEnabled: () => true,
  licenseStatus: () => mockEntitlements,
  loadLicenseStatus: (...args: unknown[]) => loadLicenseStatusMock(...args),
  startProTrial: (...args: unknown[]) => startProTrialMock(...args),
}));

vi.mock('@/api/license', () => ({
  LicenseAPI: {
    activateLicense: (...args: unknown[]) => activateLicenseMock(...args),
    clearLicense: (...args: unknown[]) => clearLicenseMock(...args),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
  },
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackUpgradeMetricEvent: (...args: unknown[]) => trackUpgradeMetricEventMock(...args),
}));

describe('ProLicensePanel', () => {
  beforeEach(() => {
    mockEntitlements = {
      capabilities: [],
      limits: [],
      subscription_state: 'expired',
      upgrade_reasons: [],
      tier: 'free',
      trial_eligible: true,
    };

    loadLicenseStatusMock.mockReset();
    startProTrialMock.mockReset();
    activateLicenseMock.mockReset();
    clearLicenseMock.mockReset();
    notificationSuccessMock.mockReset();
    notificationErrorMock.mockReset();
    trackUpgradeMetricEventMock.mockReset();

    loadLicenseStatusMock.mockResolvedValue(undefined);
    startProTrialMock.mockResolvedValue(undefined);
    activateLicenseMock.mockResolvedValue({ success: true });
    clearLicenseMock.mockResolvedValue({ success: true });
  });

  afterEach(() => {
    cleanup();
  });

  it('shows start trial action only when trial_eligible is true', async () => {
    render(() => <ProLicensePanel />);

    await waitFor(() => {
      expect(loadLicenseStatusMock).toHaveBeenCalled();
    });

    expect(screen.getByRole('button', { name: /start 14-day pro trial/i })).toBeInTheDocument();
  });

  it('hides start trial action and shows trial-ended banner when trial was already used', async () => {
    mockEntitlements = {
      capabilities: [],
      limits: [],
      subscription_state: 'expired',
      upgrade_reasons: [],
      tier: 'free',
      trial_eligible: false,
      trial_eligibility_reason: 'already_used',
    };

    render(() => <ProLicensePanel />);

    expect(
      screen.queryByRole('button', { name: /start 14-day pro trial/i }),
    ).not.toBeInTheDocument();
    expect(screen.getByText('Your Pro trial has ended')).toBeInTheDocument();
  });

  it('renders trial countdown from entitlements payload', async () => {
    mockEntitlements = {
      capabilities: ['ai_patrol', 'ai_autofix'],
      limits: [],
      subscription_state: 'trial',
      upgrade_reasons: [],
      tier: 'pro',
      trial_expires_at: 1_893_456_000,
      trial_days_remaining: 7,
      trial_eligible: false,
    };

    render(() => <ProLicensePanel />);

    await waitFor(() => {
      expect(loadLicenseStatusMock).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(screen.getByText('Days Remaining')).toBeInTheDocument();
    });

    expect(screen.getByText('Trial')).toBeInTheDocument();
    expect(screen.getByText('7')).toBeInTheDocument();
    expect(screen.getByText('Patrol Auto-Fix')).toBeInTheDocument();
  });

  it('starts trial and records conversion metric when user clicks start', async () => {
    render(() => <ProLicensePanel />);

    fireEvent.click(screen.getByRole('button', { name: /start 14-day pro trial/i }));

    await waitFor(() => {
      expect(startProTrialMock).toHaveBeenCalledTimes(1);
    });
    expect(trackUpgradeMetricEventMock).toHaveBeenCalledWith({
      type: 'trial_started',
      surface: 'license_panel',
    });
    expect(notificationSuccessMock).toHaveBeenCalledWith('Pro trial started');
  });
});
