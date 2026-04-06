import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';

import type { LicenseEntitlements } from '@/api/license';
import { ProLicensePanel } from '../ProLicensePanel';
import proLicensePanelSource from '../ProLicensePanel.tsx?raw';
import proLicensePanelStateSource from '../useProLicensePanelState.ts?raw';
import proLicensePlanSectionSource from '../ProLicensePlanSection.tsx?raw';
import selfHostedCommercialActivationSectionSource from '../SelfHostedCommercialActivationSection.tsx?raw';
import {
  getPublicPricingUrl,
  SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID,
  SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID,
} from '@/utils/pricingHandoff';

let mockEntitlements: LicenseEntitlements | null = null;

const loadLicenseStatusMock = vi.fn();
const startProTrialMock = vi.fn();
const activateLicenseMock = vi.fn();
const clearLicenseMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const useLocationMock = vi.fn(() => ({
  search: '',
  pathname: '/settings/system/billing',
  hash: '',
}));
const navigateMock = vi.fn();
const getUpgradeActionDestinationMock = vi.hoisted(() => vi.fn());
const getUpgradeActionUrlOrFallbackMock = vi.hoisted(() => vi.fn());

vi.mock('@solidjs/router', () => ({
  useLocation: () => useLocationMock(),
  useNavigate: () => navigateMock,
}));

vi.mock('@/stores/license', () => ({
  getUpgradeActionDestination: (...args: unknown[]) => getUpgradeActionDestinationMock(...args),
  getUpgradeActionUrlOrFallback: (...args: unknown[]) => getUpgradeActionUrlOrFallbackMock(...args),
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
    navigateMock.mockReset();
    getUpgradeActionDestinationMock.mockReset();
    getUpgradeActionUrlOrFallbackMock.mockReset();
    loadLicenseStatusMock.mockResolvedValue(undefined);
    startProTrialMock.mockResolvedValue(undefined);
    activateLicenseMock.mockResolvedValue({ success: true });
    clearLicenseMock.mockResolvedValue({ success: true });
    getUpgradeActionDestinationMock.mockImplementation((feature?: string) => ({
      href: getPublicPricingUrl(feature),
      external: true,
    }));
    getUpgradeActionUrlOrFallbackMock.mockImplementation((feature?: string) => getPublicPricingUrl(feature));
    useLocationMock.mockReturnValue({
      search: '',
      pathname: '/settings/system/billing',
      hash: '',
    });
  });

  afterEach(() => {
    cleanup();
  });

  it('shows start trial action only when trial_eligible is true', async () => {
    render(() => <ProLicensePanel />);

    await waitFor(() => {
      expect(loadLicenseStatusMock).toHaveBeenCalled();
    });

    expect(screen.getByText('Pulse Pro')).toBeInTheDocument();
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

    expect(screen.getAllByText('Trial').length).toBeGreaterThan(0);
    expect(screen.getByText('7')).toBeInTheDocument();
    expect(screen.getByText('Patrol Auto-Fix')).toBeInTheDocument();
  });

  it('shows migrated plan terms when plan_version is present', async () => {
    mockEntitlements = {
      capabilities: ['ai_patrol'],
      limits: [{ key: 'max_monitored_systems', limit: 12, current: 5, state: 'ok' }],
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

    expect(screen.getByText('5 / 12')).toBeInTheDocument();
    expect(screen.getByText('7')).toBeInTheDocument();
    expect(screen.getAllByText('Active').length).toBeGreaterThan(0);
    expect(screen.getByText('V5 Lifetime Grandfathered')).toBeInTheDocument();
    expect(screen.getByText('Included Monitored Systems')).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Plan' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'Usage' })).toHaveAttribute('aria-selected', 'false');

    fireEvent.click(screen.getByRole('tab', { name: 'Usage' }));

    expect(navigateMock).toHaveBeenCalledWith('/settings/system/billing#pulse-pro-usage', {
      replace: false,
      scroll: false,
    });
  });

  it('shows recurring grandfathered pricing continuity for migrated v5 Pro plans', async () => {
    const tests = [
      {
        name: 'monthly',
        planVersion: 'v5_pro_monthly_grandfathered',
        expectedLabel: 'V5 Pro Monthly (Grandfathered)',
      },
      {
        name: 'annual',
        planVersion: 'v5_pro_annual_grandfathered',
        expectedLabel: 'V5 Pro Annual (Grandfathered)',
      },
    ] as const;

    for (const tc of tests) {
      mockEntitlements = {
        capabilities: ['ai_patrol'],
        limits: [],
        subscription_state: 'active',
        upgrade_reasons: [],
        tier: 'pro',
        plan_version: tc.planVersion,
        licensed_email: 'owner@example.com',
        is_lifetime: false,
        trial_eligible: false,
      };

      render(() => <ProLicensePanel />);

      await waitFor(() => {
        expect(screen.getByText('Plan Terms')).toBeInTheDocument();
      });

      expect(screen.getByText(tc.expectedLabel)).toBeInTheDocument();
      expect(screen.getByText('Grandfathered v5 pricing')).toBeInTheDocument();
      expect(
        screen.getByText(/keeps its existing recurring price until you cancel/i),
      ).toBeInTheDocument();

      cleanup();
    }
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
        'multi_tenant',
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
    expect(screen.getByText('Multi-Tenant Mode')).toBeInTheDocument();
    expect(screen.queryByText('Multi-User Mode')).not.toBeInTheDocument();
    expect(screen.queryByText('White-Label Branding')).not.toBeInTheDocument();
    expect(screen.queryByText('Unlimited Instances')).not.toBeInTheDocument();
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
    useLocationMock.mockReturnValue({
      search: '?trial=activated',
      pathname: '/settings/system/billing',
      hash: '',
    });

    render(() => <ProLicensePanel />);

    expect(screen.getByText('Trial activated')).toBeInTheDocument();
    expect(
      screen.getByText(/Pulse activated the Pro trial for this instance/i),
    ).toBeInTheDocument();
    expect(navigateMock).toHaveBeenCalledWith('/settings/system/billing', {
      replace: true,
      scroll: false,
    });
  });

  it('focuses the usage billing section when the route hash requests it', async () => {
    useLocationMock.mockReturnValue({
      search: '',
      pathname: '/settings/system/billing',
      hash: `#${SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID}`,
    });

    render(() => <ProLicensePanel />);

    expect(screen.getByRole('tab', { name: 'Usage' })).toHaveAttribute('aria-selected', 'true');
    expect(document.getElementById(SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID)).not.toBeInTheDocument();
    expect(document.getElementById(SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID)).toBeInTheDocument();
    expect(screen.getByText('Monitored Systems')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'View counting rules' })).toBeInTheDocument();
  });

  it('navigates between plan and usage focus states through the billing subtabs', async () => {
    useLocationMock.mockReturnValue({
      search: '',
      pathname: '/settings/system/billing',
      hash: `#${SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID}`,
    });

    render(() => <ProLicensePanel />);

    fireEvent.click(screen.getByRole('tab', { name: 'Usage' }));

    expect(navigateMock).toHaveBeenCalledWith('/settings/system/billing#pulse-pro-usage', {
      replace: false,
      scroll: false,
    });
  });

  it('shows a migration-pending notice and hides the trial CTA', async () => {
    mockEntitlements = {
      capabilities: [],
      limits: [],
      subscription_state: 'expired',
      upgrade_reasons: [],
      tier: 'free',
      trial_eligible: false,
      trial_eligibility_reason: 'commercial_migration_pending',
      commercial_migration: {
        source: 'v5_license',
        state: 'pending',
        reason: 'exchange_unavailable',
        recommended_action: 'retry_activation',
      },
    };

    render(() => <ProLicensePanel />);

    expect(screen.getByText('v5 license migration pending')).toBeInTheDocument();
    expect(screen.getByText(/automatic v6 exchange did not complete yet/i)).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /start 14-day pro trial/i }),
    ).not.toBeInTheDocument();
  });

  it('shows reason-specific guidance when migration is rate limited', async () => {
    mockEntitlements = {
      capabilities: [],
      limits: [],
      subscription_state: 'expired',
      upgrade_reasons: [],
      tier: 'free',
      trial_eligible: false,
      trial_eligibility_reason: 'commercial_migration_pending',
      commercial_migration: {
        source: 'v5_license',
        state: 'pending',
        reason: 'exchange_rate_limited',
        recommended_action: 'retry_activation',
      },
    };

    render(() => <ProLicensePanel />);

    expect(screen.getByText('v5 license migration pending')).toBeInTheDocument();
    expect(screen.getByText(/rate-limited right now/i)).toBeInTheDocument();
    expect(screen.getByText(/retry activation from this instance/i)).toBeInTheDocument();
  });

  it('shows reason-specific guidance when the v5 key is unsupported', async () => {
    mockEntitlements = {
      capabilities: [],
      limits: [],
      subscription_state: 'expired',
      upgrade_reasons: [],
      tier: 'free',
      trial_eligible: false,
      trial_eligibility_reason: 'commercial_migration_failed',
      commercial_migration: {
        source: 'v5_license',
        state: 'failed',
        reason: 'exchange_unsupported',
        recommended_action: 'enter_supported_v5_key',
      },
    };

    render(() => <ProLicensePanel />);

    expect(screen.getByText('v5 license migration needs attention')).toBeInTheDocument();
    expect(
      screen.getByText(/not a supported v5 pro\/lifetime migration input/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/retry with the original v5 pro\/lifetime key from this instance/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /start 14-day pro trial/i }),
    ).not.toBeInTheDocument();
  });

  it('surfaces backend trial-start messages instead of collapsing every conflict into already-used', async () => {
    startProTrialMock.mockRejectedValue(
      Object.assign(
        new Error('Trial cannot be started while a paid v5 license migration is pending'),
        {
          status: 409,
          code: 'trial_not_available',
        },
      ),
    );

    render(() => <ProLicensePanel />);

    fireEvent.click(screen.getByRole('button', { name: /start 14-day pro trial/i }));

    await waitFor(() => {
      expect(notificationErrorMock).toHaveBeenCalledWith(
        'Trial cannot be started while a paid v5 license migration is pending',
      );
    });
  });

  it('maps explicit trial-already-used errors to the canonical message', async () => {
    startProTrialMock.mockRejectedValue(
      Object.assign(new Error('conflict'), {
        status: 409,
        code: 'trial_already_used',
      }),
    );

    render(() => <ProLicensePanel />);

    fireEvent.click(screen.getByRole('button', { name: /start 14-day pro trial/i }));

    await waitFor(() => {
      expect(notificationErrorMock).toHaveBeenCalledWith('Trial already used');
    });
  });

  it('maps rate-limited trial starts to the canonical retry message', async () => {
    startProTrialMock.mockRejectedValue(
      Object.assign(new Error('rate limited'), {
        status: 429,
      }),
    );

    render(() => <ProLicensePanel />);

    fireEvent.click(screen.getByRole('button', { name: /start 14-day pro trial/i }));

    await waitFor(() => {
      expect(notificationErrorMock).toHaveBeenCalledWith('Try again later');
    });
  });

  it('keeps Pro license split into shell, runtime, and plan owners', () => {
    expect(proLicensePanelSource).toContain('./useProLicensePanelState');
    expect(proLicensePanelSource).toContain('./ProLicensePlanSection');
    expect(proLicensePanelSource).toContain('SelfHostedCommercialActivationSection');
    expect(proLicensePanelSource).toContain('SELF_HOSTED_PRO_BILLING_PRESENTATION');
    expect(proLicensePanelSource).toContain("value={state.activeSection()}");
    expect(proLicensePanelSource).toContain('<Subtabs');
    expect(proLicensePanelSource).not.toContain('createSignal(');
    expect(proLicensePanelSource).not.toContain('useLocation()');
    expect(proLicensePanelStateSource).toContain('useLocation');
    expect(proLicensePanelStateSource).toContain('const activeSection = createMemo<SelfHostedBillingSection>(() => {');
    expect(proLicensePanelStateSource).toContain('const setActiveSection = (section: SelfHostedBillingSection) => {');
    expect(proLicensePanelStateSource).toContain('loadLicenseStatus(true)');
    expect(proLicensePanelStateSource).toContain('buildSelfHostedCommercialPlanModel');
    expect(proLicensePanelStateSource).toContain('runStartProTrialAction({');
    expect(proLicensePanelStateSource).not.toContain('startProTrial()');
    expect(proLicensePlanSectionSource).toContain('getLicenseStatusLoadingState');
    expect(proLicensePlanSectionSource).toContain('getNoActiveProLicenseState');
    expect(proLicensePlanSectionSource).toContain('getTrialEndedProLicenseNotice');
    expect(proLicensePlanSectionSource).toContain('getInactiveProUpsellNotice');
    expect(proLicensePlanSectionSource).not.toContain(
      'const trialEndedNotice = getTrialEndedProLicenseNotice();',
    );
    expect(proLicensePlanSectionSource).not.toContain(
      'const inactiveProUpsellNotice = getInactiveProUpsellNotice();',
    );
    expect(proLicensePlanSectionSource).toContain(
      'const trialEndedNotice = props.trialEnded ? getTrialEndedProLicenseNotice() : null;',
    );
    expect(proLicensePlanSectionSource).toContain(
      '!props.hasPaidFeatures && !props.trialEnded ? getInactiveProUpsellNotice() : null;',
    );
    expect(proLicensePlanSectionSource).not.toContain('Your Pro trial has ended');
    expect(proLicensePlanSectionSource).not.toContain('Unlock Pulse Patrol, alert analysis, auto-fix, and more.');
    expect(selfHostedCommercialActivationSectionSource).toContain(
      'SELF_HOSTED_ACTIVATION_PRESENTATION',
    );
    expect(selfHostedCommercialActivationSectionSource).toContain('TERMS_DOC_URL');
    expect(selfHostedCommercialActivationSectionSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/TERMS.md',
    );
    expect(selfHostedCommercialActivationSectionSource).not.toContain('Start 14-day Pro Trial');
    expect(selfHostedCommercialActivationSectionSource).not.toContain(
      'Legacy v5 license detected',
    );
    expect(proLicensePanelSource).toContain('id={SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID}');
    expect(proLicensePanelSource).toContain('id={SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID}');
  });
});
