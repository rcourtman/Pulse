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
const useLocationMock = vi.fn(() => ({ search: '' }));

vi.mock('@solidjs/router', () => ({
  useLocation: () => useLocationMock(),
}));

vi.mock('@/stores/license', () => ({
  getUpgradeActionUrlOrFallback: () => '/pricing',
  isMultiTenantEnabled: () => true,
  licenseLoadError: () => false,
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
    useLocationMock.mockReset();
    loadLicenseStatusMock.mockResolvedValue(undefined);
    startProTrialMock.mockResolvedValue(undefined);
    activateLicenseMock.mockResolvedValue({ success: true });
    clearLicenseMock.mockResolvedValue({ success: true });
    useLocationMock.mockReturnValue({ search: '' });
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

  it('shows migrated plan terms when plan_version is present', async () => {
    mockEntitlements = {
      capabilities: ['ai_patrol'],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      plan_version: 'v5_lifetime_grandfathered',
      licensed_email: 'owner@example.com',
      is_lifetime: true,
      trial_eligible: false,
    };

    render(() => <ProLicensePanel />);

    await waitFor(() => {
      expect(screen.getByText('Plan Terms')).toBeInTheDocument();
    });

    expect(screen.getByText('V5 Lifetime Grandfathered')).toBeInTheDocument();
  });

  it('renders all capability strings as human-readable labels (no raw snake_case)', async () => {
    mockEntitlements = {
      capabilities: [
        'ai_patrol',
        'sso',
        'update_alerts',
        'rbac',
        'advanced_sso',
        'audit_logging',
        'advanced_reporting',
        'agent_profiles',
        'relay',
        'mobile_app',
        'push_notifications',
        'long_term_metrics',
        'ai_alerts',
        'ai_autofix',
        'kubernetes_ai',
        'multi_user',
        'white_label',
        'multi_tenant',
        'unlimited',
      ],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'enterprise',
    };

    render(() => <ProLicensePanel />);

    await waitFor(() => {
      expect(screen.getByText('Basic SSO (OIDC)')).toBeInTheDocument();
    });

    // Verify every capability renders with its expected label
    expect(screen.getByText('Pulse Patrol')).toBeInTheDocument();
    expect(screen.getByText('Pulse Alert Analysis')).toBeInTheDocument();
    expect(screen.getByText('Patrol Auto-Fix')).toBeInTheDocument();
    expect(screen.getByText('Kubernetes Insights')).toBeInTheDocument();
    expect(screen.getByText('Update Alerts')).toBeInTheDocument();
    expect(screen.getByText('Advanced SSO (SAML/Multi-Provider)')).toBeInTheDocument();
    expect(screen.getByText('Role-Based Access Control (RBAC)')).toBeInTheDocument();
    expect(screen.getByText('Audit Logging')).toBeInTheDocument();
    expect(screen.getByText('PDF/CSV Reporting')).toBeInTheDocument();
    expect(screen.getByText('Centralized Agent Profiles')).toBeInTheDocument();
    expect(screen.getByText('Pulse Relay (Remote Access)')).toBeInTheDocument();
    expect(screen.getByText('Mobile App Access')).toBeInTheDocument();
    expect(screen.getByText('Push Notifications')).toBeInTheDocument();
    expect(screen.getByText('Extended Metric History')).toBeInTheDocument();
    expect(screen.getByText('Multi-User Mode')).toBeInTheDocument();
    expect(screen.getByText('White-Label Branding')).toBeInTheDocument();
    expect(screen.getByText('Multi-Tenant Mode')).toBeInTheDocument();
    expect(screen.getByText('Unlimited Instances')).toBeInTheDocument();
  });

  it('starts trial and records conversion metric when user clicks start', async () => {
    render(() => <ProLicensePanel />);

    fireEvent.click(screen.getByRole('button', { name: /start 14-day pro trial/i }));

    await waitFor(() => {
      expect(startProTrialMock).toHaveBeenCalledTimes(1);
    });
    expect(notificationSuccessMock).toHaveBeenCalledWith('Pro trial started');
  });

  it('shows migration guidance when the pasted key looks like a legacy v5 license', async () => {
    render(() => <ProLicensePanel />);

    fireEvent.input(screen.getByLabelText(/license \/ activation key/i), {
      target: { value: 'header.payload.signature' },
    });

    expect(screen.getByText('Legacy v5 license detected')).toBeInTheDocument();
    expect(
      screen.getByText(/exchange this key into the v6 activation model automatically/i),
    ).toBeInTheDocument();
  });

  it('shows the hosted activation success banner on the Pro settings route', async () => {
    useLocationMock.mockReturnValue({ search: '?trial=activated' });

    render(() => <ProLicensePanel />);

    expect(screen.getByText('Trial activated')).toBeInTheDocument();
    expect(
      screen.getByText(/Pulse activated the Pro trial for this instance/i),
    ).toBeInTheDocument();
  });
});
