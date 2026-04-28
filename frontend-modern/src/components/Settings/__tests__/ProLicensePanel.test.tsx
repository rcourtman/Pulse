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
  SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
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

const trackPricingViewedMock = vi.fn();
const trackCheckoutClickedMock = vi.fn();
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
const presentationPolicyHidesUpgradePromptsMock = vi.fn(() => true);
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
  presentationPolicyHidesUpgradePrompts: () => presentationPolicyHidesUpgradePromptsMock(),
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

vi.mock('@/utils/upgradeMetrics', () => ({
  trackPricingViewed: (...args: unknown[]) => trackPricingViewedMock(...args),
  trackCheckoutClicked: (...args: unknown[]) => trackCheckoutClickedMock(...args),
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
    trackPricingViewedMock.mockReset();
    trackCheckoutClickedMock.mockReset();
    licenseEntitlementsLoadErrorMock.mockReset();
    startProTrialMock.mockReset();
    activateLicenseMock.mockReset();
    clearLicenseMock.mockReset();
    notificationSuccessMock.mockReset();
    notificationErrorMock.mockReset();
    presentationPolicyHidesCommercialSurfacesMock.mockReset();
    presentationPolicyHidesUpgradePromptsMock.mockReset();
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
    presentationPolicyHidesUpgradePromptsMock.mockReturnValue(true);
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
    expect(screen.queryByText('Plans & Activation')).not.toBeInTheDocument();
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

    expect(screen.getByText('Plans & Activation')).toBeInTheDocument();
    expect(screen.getByText('Current plan: Community')).toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /start 14-day pro trial/i }),
    ).not.toBeInTheDocument();
  });

  it('keeps the self-hosted billing page non-promotional by default', async () => {
    renderPanel();

    await waitFor(() => {
      expect(loadLicenseEntitlementsMock).toHaveBeenCalled();
    });

    expect(screen.getByText('Current plan: Community')).toBeInTheDocument();
    expect(screen.queryByText(/^Expired$/)).not.toBeInTheDocument();
    expect(screen.queryByText('Compare self-hosted plans')).not.toBeInTheDocument();
    expect(screen.queryByText('Optional extras')).not.toBeInTheDocument();
    expect(screen.queryByText('What Relay adds')).not.toBeInTheDocument();
    expect(screen.queryByText('What Pulse Pro adds')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Compare plans' })).not.toBeInTheDocument();
    expect(trackPricingViewedMock).not.toHaveBeenCalled();
  });

  it('tracks compare-plan checkout intent from the explicit self-hosted billing handoff', async () => {
    useLocationMock.mockReturnValue({
      search: `?intent=${SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT}`,
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    await waitFor(() => {
      expect(loadLicenseEntitlementsMock).toHaveBeenCalled();
    });

    await fireEvent.click(screen.getAllByRole('link', { name: 'Compare plans' })[0]);

    expect(trackCheckoutClickedMock).toHaveBeenCalledWith(
      'settings_self_hosted_billing_compare_prompt',
      SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
    );
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
    expect(screen.getByText('Safe Remediation Workflows')).toBeInTheDocument();
  });

  it('shows active recurring v5 plan terms as uncapped even if stale limit metadata is present', async () => {
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
      monitored_system_continuity: {
        plan_limit: 12,
        grandfathered_floor: 23,
        effective_limit: 23,
        capture_pending: false,
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
      expect(screen.getByText('Plan Terms')).toBeInTheDocument();
    });

    expect(screen.getAllByText('Active').length).toBeGreaterThan(0);
    expect(screen.getByText('V5 Pro Monthly (Grandfathered)')).toBeInTheDocument();
    expect(screen.getByText('Core Monitoring')).toBeInTheDocument();
    expect(
      within(screen.getByText('Core Monitoring').parentElement as HTMLElement).getByText(
        'Unlimited',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Included Monitored Systems')).not.toBeInTheDocument();
    expect(screen.queryByText('Grandfathered monitored-system floor')).not.toBeInTheDocument();
    expect(screen.queryByText('Plan Monitored System Limit')).not.toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Plan' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
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
        'See which self-hosted tier this instance unlocked, what capabilities are active, and how plan status or continuity affects this install.',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Included Monitored Systems')).not.toBeInTheDocument();
    expect(screen.getByText('Guest Capacity')).toBeInTheDocument();
    expect(screen.getByText('Core Monitoring')).toBeInTheDocument();
    expect(screen.queryByText('Monitored Systems')).not.toBeInTheDocument();
    expect(screen.queryByText('Capacity Status')).not.toBeInTheDocument();
    expect(screen.queryByText('Monitored-system policy')).not.toBeInTheDocument();
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
      expect(screen.getByText('Grandfathered price')).toBeInTheDocument();
      expect(
        screen.getAllByText(
          /keeps its existing recurring price and uncapped monitored-system and guest capacity/i,
        ).length,
      ).toBeGreaterThan(0);
      expect(
        screen.queryByText(/keeps its existing recurring price and uncapped guest capacity/i),
      ).not.toBeInTheDocument();
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
      expect(screen.getByText('Guest Capacity')).toBeInTheDocument();
      expect(screen.queryByText('Monitored-system policy')).not.toBeInTheDocument();
      expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
      expect(screen.getAllByText('Unlimited').length).toBeGreaterThan(0);
      expect(screen.queryByText('Plan Monitored System Limit')).not.toBeInTheDocument();

      cleanup();
    }
  });

  it('uses shared current-plan metadata for uncapped retail self-hosted tiers', async () => {
    mockEntitlements = {
      capabilities: ['relay', 'mobile_app', 'push_notifications', 'ai_patrol', 'ai_autofix'],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      plan_version: 'pro_monthly',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
    };

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Metric History')).toBeInTheDocument();
    });

    expect(screen.getByText('Core Monitoring')).toBeInTheDocument();
    expect(screen.getByText('Metric History')).toBeInTheDocument();
    expect(screen.getByText('Included Extras')).toBeInTheDocument();
    expect(screen.getByText('Current plan: Pulse Pro')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Pulse Pro is active on this instance. Root-cause analysis, safe remediation workflows, and 90-day history are unlocked right now.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('Primary capabilities')).toBeInTheDocument();
    expect(screen.getByText('Alert Root-Cause Analysis')).toBeInTheDocument();
    expect(screen.getByText('Safe Remediation Workflows')).toBeInTheDocument();
    expect(screen.getByText('Included extras')).toBeInTheDocument();
    expect(screen.getByText('Advanced SSO (SAML/Multi-Provider)')).toBeInTheDocument();
    expect(screen.queryByText('Optional extras')).not.toBeInTheDocument();
    expect(screen.getByText('90 days')).toBeInTheDocument();
    expect(screen.getByText('Analysis, remediation, and admin controls')).toBeInTheDocument();
    expect(screen.queryByText('Guest Capacity')).not.toBeInTheDocument();
    expect(screen.queryByText('Included Monitored Systems')).not.toBeInTheDocument();
    expect(screen.queryByText('Monitored-system policy')).not.toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
  });

  it('shows Relay entitlement summaries from the paid capabilities unlocked on this instance', async () => {
    mockEntitlements = {
      capabilities: ['relay', 'mobile_app', 'push_notifications'],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'relay',
      plan_version: 'relay_monthly',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
    };

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Current plan: Relay')).toBeInTheDocument();
    });

    expect(
      screen.getByText(
        'Relay is active on this instance. Remote access, mobile, push, and longer history are unlocked right now.',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Optional extras')).not.toBeInTheDocument();
    expect(screen.queryByText('What Relay adds')).not.toBeInTheDocument();
    expect(screen.queryByText('What Pulse Pro adds')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'See all plans' })).not.toBeInTheDocument();
    expect(screen.getByText('Pulse Relay (Remote Access)')).toBeInTheDocument();
    expect(screen.getByText('Mobile App Access')).toBeInTheDocument();
    expect(screen.getByText('Push Notifications')).toBeInTheDocument();
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
      screen.getAllByText(/verifying the grandfathered monitored-system floor/i).length,
    ).toBeGreaterThan(0);
    expect(screen.getByText('Continuity pending')).toBeInTheDocument();
    expect(screen.getAllByText('Verifying…').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Unavailable').length).toBeGreaterThan(0);
    expect(screen.getByText('Plan Monitored System Limit')).toBeInTheDocument();
    expect(screen.getByText('Effective Monitored System Limit')).toBeInTheDocument();
    expect(screen.getByText('Continuity Capture')).toBeInTheDocument();
    expect(screen.getByText('Pending')).toBeInTheDocument();
    expect(screen.queryByText('0 / 10')).not.toBeInTheDocument();
  });

  it('explains why an over-policy migrated installation is still monitoring above the finite policy', async () => {
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
    expect(screen.getByText('Monitored-system policy')).toBeInTheDocument();
    expect(screen.getByText('Why is continuity still pending?')).toBeInTheDocument();
    expect(
      screen.queryByText('Monitoring continues above the current policy boundary'),
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
      screen.getAllByText(/keeps an effective monitored-system limit of 23/i).length,
    ).toBeGreaterThan(0);
    expect(screen.getByText('Grandfathered floor')).toBeInTheDocument();
    expect(screen.getAllByText('23 monitored systems').length).toBeGreaterThan(0);
    expect(screen.getByText('Monitored-system policy')).toBeInTheDocument();
    expect(
      screen.getByText('Existing monitoring continues. Additional monitored systems are paused.'),
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
    expect(screen.getByText('Alert Root-Cause Analysis')).toBeInTheDocument();
    expect(screen.getByText('Safe Remediation Workflows')).toBeInTheDocument();
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
    expect(screen.queryByText('Kubernetes AI Analysis (Compatibility)')).not.toBeInTheDocument();
    expect(screen.queryByText('Multi-User Mode')).not.toBeInTheDocument();
    expect(screen.queryByText('White-Label Branding')).not.toBeInTheDocument();
    expect(screen.queryByText('Unlimited Instances')).not.toBeInTheDocument();
  });

  it('shows migration guidance when the pasted key looks like a legacy v5 license', async () => {
    renderPanel();

    fireEvent.click(screen.getByText('Redeem existing key'));

    fireEvent.input(screen.getByLabelText(/license or activation key/i), {
      target: { value: 'header.payload.signature' },
    });

    expect(screen.getByText('Legacy v5 license detected')).toBeInTheDocument();
    expect(
      screen.getByText(/exchange this key into the v6 activation model automatically/i),
    ).toBeInTheDocument();
  });

  it('shows the hosted activation success banner on the Pro settings route', async () => {
    mockEntitlements = {
      capabilities: ['ai_patrol', 'ai_autofix', 'ai_alerts'],
      limits: [],
      subscription_state: 'trial',
      upgrade_reasons: [],
      tier: 'pro',
      trial_eligible: false,
    };
    useLocationMock.mockReturnValue({
      search: '?trial=activated',
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    expect(screen.getByText('Pulse Pro trial is now active')).toBeInTheDocument();
    expect(screen.getByText(/this instance now has Pulse Pro trial access/i)).toBeInTheDocument();
    expect(screen.getByText('Available during this trial')).toBeInTheDocument();
    expect(screen.getAllByText('Safe Remediation Workflows').length).toBeGreaterThan(0);
    expect(navigateMock).toHaveBeenCalledWith(SELF_HOSTED_PRO_BILLING_PLAN_HREF, {
      replace: true,
      scroll: false,
    });
  });

  it.each([
    {
      purchase: SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED,
      title: 'Pulse Pro is now active',
      actionLabel: null,
      actionHref: null,
    },
    {
      purchase: SELF_HOSTED_PRO_BILLING_PURCHASE_CANCELLED,
      title: 'Checkout cancelled',
      actionLabel: 'Compare plans',
      actionHref: getSelfHostedPurchaseStartUrl(SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT),
      redirectedHref: getSelfHostedBillingHref('plan', {
        intent: SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
      }),
    },
    {
      purchase: SELF_HOSTED_PRO_BILLING_PURCHASE_EXPIRED,
      title: 'Upgrade return expired',
      actionLabel: 'Compare plans',
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
      if (purchase === SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED) {
        mockEntitlements = {
          capabilities: ['relay', 'mobile_app', 'push_notifications', 'ai_autofix'],
          limits: [],
          subscription_state: 'active',
          upgrade_reasons: [],
          tier: 'pro',
          licensed_email: 'owner@example.com',
          trial_eligible: false,
        };
      }
      useLocationMock.mockReturnValue({
        search:
          purchase === SELF_HOSTED_PRO_BILLING_PURCHASE_CANCELLED
            ? `?purchase=${purchase}&intent=${SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT}`
            : `?purchase=${purchase}`,
        pathname: '/settings/system/billing/plan',
        hash: '',
      });

      renderPanel();

      expect(screen.getByText(title)).toBeInTheDocument();
      if (purchase === SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED) {
        expect(
          screen.getByText(/Checkout completed and this instance is now running Pulse Pro/i),
        ).toBeInTheDocument();
        expect(screen.getByText('Available now on this instance')).toBeInTheDocument();
        expect(screen.getAllByText('Safe Remediation Workflows').length).toBeGreaterThan(0);
      }
      if (actionLabel && actionHref) {
        const actionLinks = screen.getAllByRole('link', { name: actionLabel });
        expect(actionLinks.some((link) => link.getAttribute('href') === actionHref)).toBe(true);
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
    mockEntitlements = {
      capabilities: ['relay', 'mobile_app', 'push_notifications', 'ai_autofix'],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
    };
    useLocationMock.mockReturnValue({
      search: `?purchase=${SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED}&intent=${SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT}`,
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    expect(screen.getByText('Pulse Pro is now active')).toBeInTheDocument();
    expect(
      screen.getByText(/Checkout completed and this instance is now running Pulse Pro/i),
    ).toBeInTheDocument();
    expect(screen.queryByText('Compare self-hosted plans')).not.toBeInTheDocument();
    expect(screen.getAllByText('Safe Remediation Workflows').length).toBeGreaterThan(0);
    expect(screen.queryByText('Optional extras')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Review plan' })).not.toBeInTheDocument();
  });

  it('opens recovery by default when the billing route requests the recovery detail', async () => {
    useLocationMock.mockReturnValue({
      search: '?details=recovery',
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    const recoveryDisclosure = screen.getAllByText('Redeem existing key')[0]?.closest('details');
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

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: 'Usage' })).toHaveAttribute('aria-selected', 'true');
      expect(
        document.getElementById(SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID),
      ).not.toBeInTheDocument();
      expect(document.getElementById(SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID)).toBeInTheDocument();
    });
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
    expect(
      document.getElementById(SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID),
    ).not.toBeInTheDocument();
    expect(document.getElementById(SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID)).toBeInTheDocument();
    expect(navigateMock).toHaveBeenCalledWith(SELF_HOSTED_PRO_BILLING_PLAN_ROUTE, {
      replace: true,
      scroll: false,
    });
  });

  it('renders the self-hosted plan-selection prompt on the plan compare route', async () => {
    useLocationMock.mockReturnValue({
      search: `?intent=${SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT}`,
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    expect(screen.getAllByText('Compare self-hosted plans').length).toBeGreaterThan(0);
    expect(screen.getByText(/Community is active on this instance/i)).toBeInTheDocument();
    const compareLinks = screen.getAllByRole('link', { name: 'Compare plans' });
    expect(
      compareLinks.some(
        (link) =>
          link.getAttribute('href') ===
          getSelfHostedPurchaseStartUrl(SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT),
      ),
    ).toBe(true);
    expect(screen.queryByRole('button', { name: 'Hide counting rules' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'View counting rules' })).not.toBeInTheDocument();
  });

  it('keeps monitored-system counting guidance out of the plan surface', async () => {
    renderPanel();

    expect(screen.queryByRole('button', { name: 'View counting rules' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Hide counting rules' })).not.toBeInTheDocument();
    expect(screen.queryByText('Monitored-system policy')).not.toBeInTheDocument();
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
    expect(proLicensePanelStateSource).toContain('getSelfHostedCurrentPlanPresentation({');
    expect(proLicensePanelStateSource).toContain('getSelfHostedCurrentPlanStatusPresentation');
    expect(proLicensePanelStateSource).toContain('getSelfHostedPlanComparisonPresentation({');
    expect(proLicensePanelStateSource).toContain('getSelfHostedActivationSuccessPresentation({');
    expect(proLicensePanelStateSource).not.toContain('runStartProTrialAction({');
    expect(proLicensePanelStateSource).not.toContain('startProTrial()');
    expect(proLicensePanelStateSource).toContain("'A license or activation key is required'");
    expect(proLicensePlanSectionSource).toContain('getLicenseStatusLoadingState');
    expect(proLicensePlanSectionSource).toContain('getNoActiveProLicenseState');
    expect(proLicensePlanSectionSource).toContain('getTrialEndedProLicenseNotice');
    expect(proLicensePlanSectionSource).toContain('currentPlanSummary.title');
    expect(proLicensePlanSectionSource).toContain('currentPlanSummary.supplementalBadges');
    expect(proLicensePlanSectionSource).toContain('props.activationSuccessSummary');
    expect(proLicensePlanSectionSource).toContain('props.planComparisonSummary.cards.length > 0');
    expect(proLicensePlanSectionSource).toContain(
      'SELF_HOSTED_PRO_BILLING_PRESENTATION.planComparisonSectionTitle',
    );
    expect(proLicensePlanSectionSource).toContain('summary().highlightsLabel');
    expect(proLicensePlanSectionSource).toContain('currentPlanSummary.unlockedFeaturesLabel');
    expect(proLicensePlanSectionSource).toContain('currentPlanSummary.includedExtras.length > 0');
    expect(proLicensePlanSectionSource).not.toContain('getInactiveProUpsellNotice');
    expect(proLicensePlanSectionSource).not.toContain('MonitoredSystemDefinitionDisclosure');
    expect(proLicensePlanSectionSource).not.toContain('trialStartTitle');
    expect(proLicensePlanSectionSource).not.toContain('trialStartIdleActionLabel');
    expect(proLicensePlanSectionSource).toContain(
      'const trialEndedNotice = props.trialEnded ? getTrialEndedProLicenseNotice() : null;',
    );
    expect(proLicensePlanSectionSource).toContain('planSelectionPrompt');
    expect(proLicensePlanSectionSource).not.toContain(
      "resolveSelfHostedPurchaseStartDestination('self_hosted_plan')",
    );
    expect(proLicensePlanSectionSource).not.toContain('Your Pro trial has ended');
    expect(proLicensePlanSectionSource).not.toContain(
      'Turn alert noise into root-cause analysis, safe remediation workflows, and 90-day history.',
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
