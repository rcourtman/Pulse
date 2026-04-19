import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';

import type { LicenseEntitlements } from '@/api/license';
import { ProLicensePanel } from '../ProLicensePanel';
import proLicensePanelSource from '../ProLicensePanel.tsx?raw';
import proLicensePanelStateSource from '../useProLicensePanelState.ts?raw';
import proLicensePlanSectionSource from '../ProLicensePlanSection.tsx?raw';
import selfHostedCommercialRecoverySectionSource from '../SelfHostedCommercialRecoverySection.tsx?raw';
import {
  getSelfHostedBillingHref,
  getPublicPricingUrl,
  getSelfHostedPurchaseStartUrl,
  SELF_HOSTED_PRO_BILLING_PLAN_HREF,
  SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT,
  SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED,
  SELF_HOSTED_PRO_BILLING_PURCHASE_CANCELLED,
  SELF_HOSTED_PRO_BILLING_PURCHASE_EXPIRED,
  SELF_HOSTED_PRO_BILLING_PURCHASE_FAILED,
  SELF_HOSTED_PRO_BILLING_PURCHASE_UNAVAILABLE,
  SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
  SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID,
  SELF_HOSTED_PRO_BILLING_USAGE_HREF,
  SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID,
  SELF_HOSTED_PRO_BILLING_PLAN_ROUTE,
} from '@/utils/pricingHandoff';

let mockEntitlements: LicenseEntitlements | null = null;

const loadRuntimeLicenseStatusMock = vi.fn();
const loadCommercialPostureMock = vi.fn();
const loadLicenseEntitlementsMock = vi.fn();
const licenseEntitlementsLoadErrorMock = vi.fn(() => null);
const startProTrialMock = vi.fn();
const activateLicenseMock = vi.fn();
const clearLicenseMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const presentationPolicyHidesCommercialSurfacesMock = vi.fn(() => false);
const sessionPresentationPolicyResolvedMock = vi.fn(() => true);
const useLocationMock = vi.fn(() => ({
  search: '',
  pathname: '/settings/system/billing/plan',
  hash: '',
}));
const navigateMock = vi.fn();
const getUpgradeActionDestinationMock = vi.hoisted(() => vi.fn());
const getUpgradeActionUrlOrFallbackMock = vi.hoisted(() => vi.fn());

vi.mock('@solidjs/router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@solidjs/router')>();
  return {
    ...actual,
    useLocation: () => useLocationMock(),
    useNavigate: () => navigateMock,
  };
});

vi.mock('@/stores/license', () => ({
  isMultiTenantEnabled: () => true,
  loadRuntimeCapabilities: (...args: unknown[]) => loadRuntimeLicenseStatusMock(...args),
}));

vi.mock('@/stores/licenseCommercial', () => ({
  getUpgradeActionDestination: (...args: unknown[]) => getUpgradeActionDestinationMock(...args),
  getUpgradeActionUrlOrFallback: (...args: unknown[]) => getUpgradeActionUrlOrFallbackMock(...args),
  loadCommercialPosture: (...args: unknown[]) => loadCommercialPostureMock(...args),
  startProTrial: (...args: unknown[]) => startProTrialMock(...args),
}));

vi.mock('@/stores/licenseEntitlements', () => ({
  licenseEntitlements: () => mockEntitlements,
  licenseEntitlementsLoadError: () => licenseEntitlementsLoadErrorMock(),
  loadLicenseEntitlements: (...args: unknown[]) => loadLicenseEntitlementsMock(...args),
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesCommercialSurfaces: () => presentationPolicyHidesCommercialSurfacesMock(),
  sessionPresentationPolicyResolved: () => sessionPresentationPolicyResolvedMock(),
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
  const renderPanel = () =>
    render(() => (
      <Router>
        <Route path="/" component={() => <ProLicensePanel />} />
      </Router>
    ));

  beforeEach(() => {
    mockEntitlements = {
      capabilities: [],
      limits: [],
      subscription_state: 'expired',
      upgrade_reasons: [],
      tier: 'free',
      trial_eligible: true,
    };

    loadRuntimeLicenseStatusMock.mockReset();
    loadCommercialPostureMock.mockReset();
    loadLicenseEntitlementsMock.mockReset();
    licenseEntitlementsLoadErrorMock.mockReset();
    startProTrialMock.mockReset();
    activateLicenseMock.mockReset();
    clearLicenseMock.mockReset();
    notificationSuccessMock.mockReset();
    notificationErrorMock.mockReset();
    presentationPolicyHidesCommercialSurfacesMock.mockReset();
    sessionPresentationPolicyResolvedMock.mockReset();
    useLocationMock.mockReset();
    navigateMock.mockReset();
    getUpgradeActionDestinationMock.mockReset();
    getUpgradeActionUrlOrFallbackMock.mockReset();
    loadRuntimeLicenseStatusMock.mockResolvedValue(undefined);
    loadCommercialPostureMock.mockResolvedValue(undefined);
    loadLicenseEntitlementsMock.mockResolvedValue(undefined);
    licenseEntitlementsLoadErrorMock.mockReturnValue(null);
    startProTrialMock.mockResolvedValue(undefined);
    activateLicenseMock.mockResolvedValue({ success: true });
    clearLicenseMock.mockResolvedValue({ success: true });
    presentationPolicyHidesCommercialSurfacesMock.mockReturnValue(false);
    sessionPresentationPolicyResolvedMock.mockReturnValue(true);
    getUpgradeActionDestinationMock.mockImplementation((feature?: string) => ({
      href: getPublicPricingUrl(feature),
      external: true,
    }));
    getUpgradeActionUrlOrFallbackMock.mockImplementation((feature?: string) =>
      getPublicPricingUrl(feature),
    );
    useLocationMock.mockReturnValue({
      search: '',
      pathname: '/settings/system/billing/plan',
      hash: '',
    });
  });

  afterEach(() => {
    cleanup();
  });

  it('fails closed until the session presentation policy is resolved', () => {
    sessionPresentationPolicyResolvedMock.mockReturnValue(false);

    renderPanel();

    expect(loadLicenseEntitlementsMock).not.toHaveBeenCalled();
    expect(screen.getByText('Loading settings access')).toBeInTheDocument();
    expect(
      screen.getByText(/before showing license, billing, or usage details/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /start 14-day pro trial/i }),
    ).not.toBeInTheDocument();
  });

  it('hides commercial details in demo mode without loading license state', () => {
    presentationPolicyHidesCommercialSurfacesMock.mockReturnValue(true);

    renderPanel();

    expect(loadLicenseEntitlementsMock).not.toHaveBeenCalled();
    expect(loadRuntimeLicenseStatusMock).not.toHaveBeenCalled();
    expect(loadCommercialPostureMock).not.toHaveBeenCalled();
    expect(screen.getByText('License and billing details are hidden')).toBeInTheDocument();
    expect(screen.getByText(/instead of creating a demo license/i)).toBeInTheDocument();
    expect(screen.queryByText('Pulse Pro')).not.toBeInTheDocument();
    expect(screen.queryByText('Monitored Systems')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /start 14-day pro trial/i }),
    ).not.toBeInTheDocument();
  });

  it('does not show a trial-start CTA on the Pro license settings page', async () => {
    renderPanel();

    await waitFor(() => {
      expect(loadLicenseEntitlementsMock).toHaveBeenCalled();
    });

    expect(screen.getByText('Pulse Pro')).toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /start 14-day pro trial/i }),
    ).not.toBeInTheDocument();
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

    renderPanel();

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

    renderPanel();

    await waitFor(() => {
      expect(loadLicenseEntitlementsMock).toHaveBeenCalled();
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
      plan_version: 'v5_pro_monthly_grandfathered',
      licensed_email: 'owner@example.com',
      is_lifetime: false,
      trial_eligible: false,
    };

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Plan Terms')).toBeInTheDocument();
    });

    expect(screen.getAllByText('5 monitored systems').length).toBeGreaterThan(0);
    expect(screen.getAllByText('7 remaining').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Active').length).toBeGreaterThan(0);
    expect(screen.getByText('V5 Pro Monthly (Grandfathered)')).toBeInTheDocument();
    expect(screen.getByText('Included Monitored Systems')).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Plan' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'Usage' })).toHaveAttribute('aria-selected', 'false');

    fireEvent.click(screen.getByRole('tab', { name: 'Usage' }));

    expect(navigateMock).toHaveBeenCalledWith(SELF_HOSTED_PRO_BILLING_USAGE_HREF, {
      replace: false,
      scroll: false,
    });
  });

  it('shows lifetime grandfathered plans as uncapped', async () => {
    mockEntitlements = {
      capabilities: ['ai_patrol'],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'lifetime',
      plan_version: 'v5_lifetime_grandfathered',
      licensed_email: 'owner@example.com',
      is_lifetime: true,
      trial_eligible: false,
    };

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Plan Terms')).toBeInTheDocument();
    });

    expect(screen.getByText('V5 Lifetime Grandfathered')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Review your active plan, expiry, and the paid capabilities that come with it.',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Included Monitored Systems')).not.toBeInTheDocument();
    expect(screen.getByText('Max Guests')).toBeInTheDocument();
    expect(screen.getByText('Core Monitoring')).toBeInTheDocument();
    expect(screen.queryByText('Monitored Systems')).not.toBeInTheDocument();
    expect(screen.queryByText('Capacity Status')).not.toBeInTheDocument();
    expect(screen.queryByText('Monitoring capacity')).not.toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
    expect(screen.getAllByText('Unlimited').length).toBeGreaterThan(0);
    expect(screen.queryByText('5 / 12')).not.toBeInTheDocument();
  });

  it('shows recurring grandfathered v5 Pro plans as uncapped while they remain active', async () => {
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

      renderPanel();

      await waitFor(() => {
        expect(screen.getByText('Plan Terms')).toBeInTheDocument();
      });

      expect(screen.getByText(tc.expectedLabel)).toBeInTheDocument();
      expect(screen.getByText('Grandfathered v5 pricing')).toBeInTheDocument();
      expect(
        screen.getByText(
          /keeps its existing recurring price and uncapped monitored-system and guest capacity until you cancel/i,
        ),
      ).toBeInTheDocument();
      expect(
        within(screen.getByText('Core Monitoring').parentElement as HTMLElement).getByText(
          'Unlimited',
        ),
      ).toBeInTheDocument();
      expect(screen.queryByText('Capacity Status')).not.toBeInTheDocument();
      expect(screen.queryByText('Included Monitored Systems')).not.toBeInTheDocument();
      expect(screen.getByText('Max Guests')).toBeInTheDocument();
      expect(screen.queryByText('Monitoring capacity')).not.toBeInTheDocument();
      expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
      expect(screen.getAllByText('Unlimited').length).toBeGreaterThan(0);
      expect(screen.queryByText('Plan Monitored System Limit')).not.toBeInTheDocument();

      cleanup();
    }
  });

  it('shows continuity verification while a bounded fallback migration is still pending', async () => {
    mockEntitlements = {
      capabilities: ['relay'],
      limits: [
        {
          key: 'max_monitored_systems',
          limit: 10,
          current: 0,
          current_available: false,
          current_unavailable_reason: 'supplemental_inventory_unsettled',
          state: 'ok',
        },
      ],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      plan_version: 'legacy_migration_fallback',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
      monitored_system_continuity: {
        plan_limit: 10,
        effective_limit: 10,
        capture_pending: true,
      },
      monitored_system_capacity: {
        mode: 'usage_unavailable',
        urgency: 'ok',
        current: 0,
        limit: 10,
        current_available: false,
        current_unavailable_reason: 'supplemental_inventory_unsettled',
        available_slots: 0,
        overage: 0,
        reason: 'legacy_migration_capture_pending',
        blocks_new_systems: false,
        existing_monitoring_continues: false,
      },
    };

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Migration continuity verification pending')).toBeInTheDocument();
    });

    expect(
      screen.getByText(/verifying the grandfathered monitored-system floor/i),
    ).toBeInTheDocument();
    expect(screen.getAllByText('Verifying…').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Unavailable').length).toBeGreaterThan(0);
    expect(screen.getByText('Plan Monitored System Limit')).toBeInTheDocument();
    expect(screen.getByText('Effective Monitored System Limit')).toBeInTheDocument();
    expect(screen.getByText('Continuity Capture')).toBeInTheDocument();
    expect(screen.getByText('Pending')).toBeInTheDocument();
    expect(screen.queryByText('0 / 10')).not.toBeInTheDocument();
  });

  it('explains why an over-plan migrated installation is still monitoring above the current plan limit', async () => {
    mockEntitlements = {
      capabilities: ['relay'],
      limits: [
        {
          key: 'max_monitored_systems',
          limit: 10,
          current: 23,
          current_available: true,
          state: 'enforced',
        },
      ],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      plan_version: 'legacy_migration_fallback',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
      monitored_system_continuity: {
        plan_limit: 10,
        effective_limit: 10,
        capture_pending: true,
      },
      monitored_system_capacity: {
        mode: 'over_limit_frozen',
        urgency: 'enforced',
        current: 23,
        limit: 10,
        current_available: true,
        available_slots: 0,
        overage: 13,
        reason: 'legacy_migration_capture_pending',
        blocks_new_systems: true,
        existing_monitoring_continues: true,
      },
    };

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Migration continuity verification pending')).toBeInTheDocument();
    });

    expect(screen.getAllByText(/already monitoring 23/i).length).toBeGreaterThan(0);
    expect(screen.getByText('Monitoring capacity')).toBeInTheDocument();
    expect(screen.getByText('Why is continuity still pending?')).toBeInTheDocument();
    expect(
      screen.queryByText('Monitoring continues above the current plan limit'),
    ).not.toBeInTheDocument();
  });

  it('shows monitored-system continuity once a bounded fallback migration floor is captured', async () => {
    mockEntitlements = {
      capabilities: ['relay'],
      limits: [
        {
          key: 'max_monitored_systems',
          limit: 23,
          current: 23,
          current_available: true,
          state: 'enforced',
        },
      ],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      plan_version: 'legacy_migration_fallback',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
      monitored_system_continuity: {
        plan_limit: 10,
        grandfathered_floor: 23,
        effective_limit: 23,
        capture_pending: false,
        captured_at: 1_768_000_000,
      },
      monitored_system_capacity: {
        mode: 'at_limit_blocking_new',
        urgency: 'enforced',
        current: 23,
        limit: 23,
        current_available: true,
        available_slots: 0,
        overage: 0,
        reason: 'limit_reached',
        blocks_new_systems: true,
        existing_monitoring_continues: true,
      },
    };

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Grandfathered monitored-system floor')).toBeInTheDocument();
    });

    expect(
      screen.getByText(/keeps an effective monitored-system limit of 23/i),
    ).toBeInTheDocument();
    expect(screen.getAllByText('23 monitored systems').length).toBeGreaterThan(0);
    expect(screen.getByText('Monitoring capacity')).toBeInTheDocument();
    expect(
      screen.getByText('Existing monitoring continues. New monitored systems are blocked.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Review monitored systems' })).toBeInTheDocument();
    expect(screen.getByText('Plan Monitored System Limit')).toBeInTheDocument();
    expect(screen.getByText('Effective Monitored System Limit')).toBeInTheDocument();
    expect(screen.getByText('Grandfathered Floor')).toBeInTheDocument();
    expect(screen.queryByText('Included Monitored Systems')).not.toBeInTheDocument();
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

    renderPanel();

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

  it('shows migration guidance when the pasted key looks like a legacy v5 license', async () => {
    renderPanel();

    fireEvent.click(screen.getByText('Redeem existing key'));

    fireEvent.input(screen.getByLabelText(/pulse pro key/i), {
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
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    expect(screen.getByText('Trial activated')).toBeInTheDocument();
    expect(
      screen.getByText(/Pulse activated the Pro trial for this instance/i),
    ).toBeInTheDocument();
    expect(navigateMock).toHaveBeenCalledWith(SELF_HOSTED_PRO_BILLING_PLAN_HREF, {
      replace: true,
      scroll: false,
    });
  });

  it.each([
    {
      purchase: SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED,
      title: 'Pulse Pro activated',
      actionLabel: null,
      actionHref: null,
    },
    {
      purchase: SELF_HOSTED_PRO_BILLING_PURCHASE_CANCELLED,
      title: 'Checkout cancelled',
      actionLabel: 'Compare plans',
      actionHref: getSelfHostedPurchaseStartUrl(),
    },
    {
      purchase: SELF_HOSTED_PRO_BILLING_PURCHASE_EXPIRED,
      title: 'Upgrade return expired',
      actionLabel: 'Restart upgrade',
      actionHref: getSelfHostedPurchaseStartUrl(),
    },
    {
      purchase: SELF_HOSTED_PRO_BILLING_PURCHASE_FAILED,
      title: 'Activation needs attention',
      actionLabel: 'Open recovery',
      actionHref: getSelfHostedBillingHref('plan', {
        detail: SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
      }),
      redirectedHref: getSelfHostedBillingHref('plan', {
        detail: SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
      }),
    },
    {
      purchase: SELF_HOSTED_PRO_BILLING_PURCHASE_UNAVAILABLE,
      title: 'Pulse Account unavailable',
      actionLabel: 'Try again',
      actionHref: getSelfHostedPurchaseStartUrl(),
    },
  ])(
    'shows the purchase arrival notice for $purchase',
    async ({
      purchase,
      title,
      actionLabel,
      actionHref,
      redirectedHref = SELF_HOSTED_PRO_BILLING_PLAN_HREF,
    }) => {
      useLocationMock.mockReturnValue({
        search: `?purchase=${purchase}`,
        pathname: '/settings/system/billing/plan',
        hash: '',
      });

      renderPanel();

      expect(screen.getByText(title)).toBeInTheDocument();
      if (actionLabel && actionHref) {
        expect(screen.getByRole('link', { name: actionLabel })).toHaveAttribute('href', actionHref);
      } else {
        expect(screen.queryByRole('link', { name: 'Review usage' })).not.toBeInTheDocument();
      }
      expect(navigateMock).toHaveBeenCalledWith(redirectedHref, {
        replace: true,
        scroll: false,
      });
    },
  );

  it('returns self-hosted plan purchases to the plan surface instead of the legacy usage tab', async () => {
    useLocationMock.mockReturnValue({
      search: `?purchase=${SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED}&intent=${SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT}`,
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    expect(screen.getByText('Pulse Pro activated')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Review plan' })).toHaveAttribute(
      'href',
      SELF_HOSTED_PRO_BILLING_PLAN_HREF,
    );
    expect(screen.queryByText('Compare self-hosted plans')).not.toBeInTheDocument();
  });

  it('opens recovery by default when the billing route requests the recovery detail', async () => {
    useLocationMock.mockReturnValue({
      search: '?details=recovery',
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    const recoveryDisclosure = screen.getByText('Redeem existing key').closest('details');
    expect(recoveryDisclosure).toHaveAttribute('open');
  });

  it('focuses the usage billing section when a bounded legacy billing route requests counting rules', async () => {
    mockEntitlements = {
      capabilities: ['relay'],
      limits: [{ key: 'max_monitored_systems', limit: 10, current: 5, state: 'ok' }],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      plan_version: 'legacy_migration_fallback',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
    };
    useLocationMock.mockReturnValue({
      search: '?details=counting-rules',
      pathname: '/settings/system/billing/usage',
      hash: '',
    });

    renderPanel();

    expect(screen.getByRole('tab', { name: 'Usage' })).toHaveAttribute('aria-selected', 'true');
    expect(
      document.getElementById(SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID),
    ).not.toBeInTheDocument();
    expect(document.getElementById(SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID)).toBeInTheDocument();
    expect(screen.getByText('Monitored Systems')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Hide counting rules' })).toBeInTheDocument();
  });

  it('redirects uncapped self-hosted billing usage routes back to the plan surface', async () => {
    useLocationMock.mockReturnValue({
      search: '?details=counting-rules',
      pathname: '/settings/system/billing/usage',
      hash: '',
    });

    renderPanel();

    await waitFor(() => {
      expect(loadLicenseEntitlementsMock).toHaveBeenCalled();
    });

    expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
    expect(document.getElementById(SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID)).not.toBeInTheDocument();
    expect(document.getElementById(SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID)).toBeInTheDocument();
    expect(navigateMock).toHaveBeenCalledWith(SELF_HOSTED_PRO_BILLING_PLAN_ROUTE, {
      replace: true,
      scroll: false,
    });
  });

  it('does not render a monitored-system upgrade arrival callout on the plan upgrade route', async () => {
    useLocationMock.mockReturnValue({
      search: `?intent=${SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT}`,
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    expect(screen.queryByText('Compare self-hosted plans')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Compare plans' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Hide counting rules' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'View counting rules' })).not.toBeInTheDocument();
  });

  it('keeps monitored-system counting guidance out of the plan surface', async () => {
    renderPanel();

    expect(screen.queryByRole('button', { name: 'View counting rules' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Hide counting rules' })).not.toBeInTheDocument();
    expect(screen.queryByText('Monitoring capacity')).not.toBeInTheDocument();
  });

  it('navigates between plan and usage focus states through the billing subtabs when legacy capacity handling is active', async () => {
    mockEntitlements = {
      capabilities: ['relay'],
      limits: [{ key: 'max_monitored_systems', limit: 10, current: 5, state: 'ok' }],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      plan_version: 'legacy_migration_fallback',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
    };
    useLocationMock.mockReturnValue({
      search: '',
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    fireEvent.click(screen.getByRole('tab', { name: 'Usage' }));

    expect(navigateMock).toHaveBeenCalledWith(SELF_HOSTED_PRO_BILLING_USAGE_HREF, {
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

    renderPanel();

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

    renderPanel();

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

    renderPanel();

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

  it('keeps Pro license split into shell, runtime, and plan owners', () => {
    expect(proLicensePanelSource).toContain('./useProLicensePanelState');
    expect(proLicensePanelSource).toContain('sessionPresentationPolicyResolved');
    expect(proLicensePanelSource).toContain('presentationPolicyHidesCommercialSurfaces');
    expect(proLicensePanelSource).toContain('ProLicensePanelContent');
    expect(proLicensePanelSource).toContain('./ProLicensePlanSection');
    expect(proLicensePanelSource).toContain('SelfHostedCommercialRecoverySection');
    expect(proLicensePanelSource).toContain('SELF_HOSTED_PRO_BILLING_PRESENTATION');
    expect(proLicensePanelSource).toContain('value={state.activeSection()}');
    expect(proLicensePanelSource).toContain('<Subtabs');
    expect(proLicensePanelSource).not.toContain('createSignal(');
    expect(proLicensePanelSource).not.toContain('useLocation()');
    expect(proLicensePanelStateSource).toContain('useLocation');
    expect(proLicensePanelStateSource).toContain('resolveSelfHostedBillingSection');
    expect(proLicensePanelStateSource).toContain('getSelfHostedBillingPlanIntent');
    expect(proLicensePanelStateSource).toContain('getSelfHostedBillingUsageDetail');
    expect(proLicensePanelStateSource).toContain('const setActiveSection = (section: string) => {');
    expect(proLicensePanelStateSource).toContain('loadLicenseEntitlements(true)');
    expect(proLicensePanelStateSource).toContain('loadCommercialPosture(true)');
    expect(proLicensePanelStateSource).toContain('loadRuntimeCapabilities(true)');
    expect(proLicensePanelStateSource).toContain('buildSelfHostedCommercialPlanModel');
    expect(proLicensePanelStateSource).toContain('runStartProTrialAction({');
    expect(proLicensePanelStateSource).not.toContain('startProTrial()');
    expect(proLicensePanelStateSource).toContain("'A Pulse Pro key is required'");
    expect(proLicensePlanSectionSource).toContain('getLicenseStatusLoadingState');
    expect(proLicensePlanSectionSource).toContain('getNoActiveProLicenseState');
    expect(proLicensePlanSectionSource).toContain('getTrialEndedProLicenseNotice');
    expect(proLicensePlanSectionSource).not.toContain('getInactiveProUpsellNotice');
    expect(proLicensePlanSectionSource).not.toContain('MonitoredSystemDefinitionDisclosure');
    expect(proLicensePlanSectionSource).not.toContain('trialStartTitle');
    expect(proLicensePlanSectionSource).not.toContain('trialStartIdleActionLabel');
    expect(proLicensePlanSectionSource).toContain(
      'const trialEndedNotice = props.trialEnded ? getTrialEndedProLicenseNotice() : null;',
    );
    expect(proLicensePlanSectionSource).not.toContain('monitoredSystemUpgradeArrivalTitle');
    expect(proLicensePlanSectionSource).not.toContain(
      "resolveSelfHostedPurchaseStartDestination('self_hosted_plan')",
    );
    expect(proLicensePlanSectionSource).not.toContain('Your Pro trial has ended');
    expect(proLicensePlanSectionSource).not.toContain(
      'Unlock Pulse Patrol, alert analysis, auto-fix, and more.',
    );
    expect(selfHostedCommercialRecoverySectionSource).toContain(
      'SELF_HOSTED_RECOVERY_PRESENTATION',
    );
    expect(selfHostedCommercialRecoverySectionSource).toContain('TERMS_DOC_URL');
    expect(selfHostedCommercialRecoverySectionSource).toContain('disclosureLabel');
    expect(selfHostedCommercialRecoverySectionSource).toContain('recoverySectionTitle');
    expect(selfHostedCommercialRecoverySectionSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/TERMS.md',
    );
    expect(selfHostedCommercialRecoverySectionSource).not.toContain('Start 14-day Pro Trial');
    expect(selfHostedCommercialRecoverySectionSource).not.toContain('Legacy v5 license detected');
    expect(proLicensePanelSource).toContain('id={SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID}');
    expect(proLicensePanelSource).toContain('id={SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID}');
  });
});
