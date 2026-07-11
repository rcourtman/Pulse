import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';

import type { LicenseEntitlements } from '@/api/license';
import type { AgentOperationsLoopStatus } from '@/api/agentCapabilities';
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
import { PATROL_CONTROL_STARTER_URL } from '@/utils/licensePresentation';
import { PATROL_CONTROL_PATH } from '@/routing/resourceLinks';

let mockEntitlements: LicenseEntitlements | null = null;

const PRO_RUNTIME_IDENTITY = {
  build: 'pro',
  label: 'Pulse Pro runtime',
  download_url: 'https://pulserelay.pro/download.html',
};

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
const fetchAgentOperationsLoopStatusMock = vi.hoisted(() => vi.fn());

const buildOperationsLoopStatus = (
  overrides: Partial<AgentOperationsLoopStatus> = {},
): AgentOperationsLoopStatus => ({
  nextAction: 'run_patrol',
  progressLabel: 'No Patrol work activity yet.',
  steps: [],
  patrolEvidenceCount: 0,
  patrolIssueEvidenceCount: 0,
  activeFindingCount: 0,
  pendingApprovalCount: 0,
  governedActionCount: 0,
  approvedDecisionCount: 0,
  rejectedDecisionCount: 0,
  verifiedOutcomeCount: 0,
  operationsLoopStarterCount: 0,
  assistantOperationsLoopStarterCount: 0,
  patrolOperationsLoopStarterCount: 0,
  patrolControlOperationsLoopStarterCount: 0,
  patrolControlCompletedOperationsLoopCount: 0,
  patrolControlResolvedOperationsLoopCount: 0,
  patrolControlValueState: 'not_started',
  patrolAutonomyOperationsLoopStarterCount: 0,
  patrolAutonomyCompletedOperationsLoopCount: 0,
  patrolAutonomyResolvedOperationsLoopCount: 0,
  patrolAutonomyValueState: 'not_started',
  proActivationOperationsLoopStarterCount: 0,
  proActivationCompletedOperationsLoopCount: 0,
  proActivationResolvedOperationsLoopCount: 0,
  proActivationValueProofState: 'not_started',
  mcpOperationsLoopStarterCount: 0,
  externalAgentReady: false,
  windowStart: '2026-06-21T00:00:00Z',
  generatedAt: '2026-06-21T00:00:00Z',
  ...overrides,
});

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

vi.mock('@/api/agentCapabilities', () => ({
  fetchAgentOperationsLoopStatus: (...args: unknown[]) =>
    fetchAgentOperationsLoopStatusMock(...args),
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
      trial_eligible: false,
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
    presentationPolicyHidesUpgradePromptsMock.mockReset();
    sessionPresentationPolicyResolvedMock.mockReset();
    useLocationMock.mockReset();
    navigateMock.mockReset();
    getUpgradeActionDestinationMock.mockReset();
    getUpgradeActionUrlOrFallbackMock.mockReset();
    fetchAgentOperationsLoopStatusMock.mockReset();
    loadRuntimeLicenseStatusMock.mockResolvedValue(undefined);
    loadCommercialPostureMock.mockResolvedValue(undefined);
    loadLicenseEntitlementsMock.mockResolvedValue(undefined);
    fetchAgentOperationsLoopStatusMock.mockResolvedValue(buildOperationsLoopStatus());
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
    expect(screen.queryByText('Plans & Billing')).not.toBeInTheDocument();
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

    expect(screen.getByText('Plans & Billing')).toBeInTheDocument();
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
    expect(screen.queryByText('Select a plan')).not.toBeInTheDocument();
    expect(screen.queryByText('Available plans')).not.toBeInTheDocument();
    expect(screen.queryByText('Relay plan')).not.toBeInTheDocument();
    expect(screen.queryByText('Pulse Pro plan')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'View plans' })).not.toBeInTheDocument();
  });

  it('opens compare-plan checkout from the explicit self-hosted billing handoff', async () => {
    useLocationMock.mockReturnValue({
      search: `?intent=${SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT}`,
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    await waitFor(() => {
      expect(loadLicenseEntitlementsMock).toHaveBeenCalled();
    });

    await fireEvent.click(screen.getAllByRole('link', { name: 'View plans' })[0]);
  });

  it('keeps explicit self-hosted plan comparison focused on optional paid extras', async () => {
    useLocationMock.mockReturnValue({
      search: `?intent=${SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT}`,
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    await waitFor(() => {
      expect(loadLicenseEntitlementsMock).toHaveBeenCalled();
    });

    expect(screen.getByText('Select a plan')).toBeInTheDocument();
    expect(screen.getByText('Available plans')).toBeInTheDocument();
    expect(screen.getByText('Relay plan')).toBeInTheDocument();
    expect(screen.getByText('Pulse Pro plan')).toBeInTheDocument();
    expect(screen.getByText('Remote web access via Relay')).toBeInTheDocument();
    expect(screen.getByText('Pulse Mobile pairing')).toBeInTheDocument();
    expect(screen.getByText('Push notifications')).toBeInTheDocument();
    expect(
      screen.getByText('Patrol modes: Ask first, Safe auto-fix, or Autopilot'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Patrol investigates issues and explains the root cause'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Patrol applies safe fixes and verifies the result'),
    ).toBeInTheDocument();
    expect(screen.getByText('90-day metric history')).toBeInTheDocument();
    expect(
      screen.getByText('Team controls: RBAC, audit logging, reporting, and agent profiles'),
    ).toBeInTheDocument();

    expect(screen.queryByText(/unlimited/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/trial/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/monitoring room/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/monitoring capacity/i)).not.toBeInTheDocument();
  });

  it('does not surface a trial-ended banner for retired self-hosted trial state', async () => {
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
    expect(screen.queryByText('Your Pro trial has ended')).not.toBeInTheDocument();
  });

  it.each([
    ['Community', 'community'],
    ['Relay', 'relay'],
  ])('does not load Patrol operator status for %s plans', async (planName, tier) => {
    mockEntitlements = {
      capabilities: [],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier,
      trial_eligible: false,
    };

    renderPanel();

    await waitFor(() => {
      expect(loadLicenseEntitlementsMock).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(screen.getByText(`Current plan: ${planName}`)).toBeInTheDocument();
    });

    expect(fetchAgentOperationsLoopStatusMock).not.toHaveBeenCalled();
    expect(screen.queryByText('Patrol work')).not.toBeInTheDocument();
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
    expect(screen.getByText('Patrol Handles Safe Fixes')).toBeInTheDocument();
  });

  it('keeps the Patrol mode entry point visible for existing active Pro plans', async () => {
    mockEntitlements = {
      capabilities: ['relay', 'mobile_app', 'push_notifications', 'ai_autofix'],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
      runtime: PRO_RUNTIME_IDENTITY,
    };

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Current plan: Pulse Pro')).toBeInTheDocument();
    });

    expect(screen.queryByText('Pulse Pro is now active')).not.toBeInTheDocument();
    const patrolControlLink = screen.getByRole('link', { name: 'Choose Patrol mode' });
    expect(patrolControlLink).toHaveAttribute('href', PATROL_CONTROL_STARTER_URL);
    expect(patrolControlLink).not.toHaveAttribute('target');
    patrolControlLink.addEventListener('click', (event) => event.preventDefault());
    fireEvent.click(patrolControlLink);
  });

  it('keeps external-agent proof behind Patrol mode presentation', async () => {
    mockEntitlements = {
      capabilities: [
        'relay',
        'mobile_app',
        'push_notifications',
        'ai_alerts',
        'ai_autofix',
        'rbac',
        'audit_logging',
        'advanced_reporting',
        'agent_profiles',
      ],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      licensed_email: 'owner@example.com',
      max_history_days: 90,
      trial_eligible: false,
      runtime: PRO_RUNTIME_IDENTITY,
    };
    fetchAgentOperationsLoopStatusMock.mockResolvedValue(
      buildOperationsLoopStatus({
        nextAction: 'open_mcp',
        progressLabel: 'Legacy status asked for MCP readiness after the outcome was verified.',
        patrolControlOperationsLoopStarterCount: 1,
        patrolControlResolvedOperationsLoopCount: 0,
        patrolControlValueState: 'verified_needs_mcp',
        externalAgentReady: false,
      }),
    );

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Current plan: Pulse Pro')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(fetchAgentOperationsLoopStatusMock).toHaveBeenCalled();
    });
    expect(screen.queryByText('Patrol work')).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol autonomy loop')).not.toBeInTheDocument();
    expect(screen.queryByText('Verified')).not.toBeInTheDocument();
    expect(
      screen.queryByText(
        'Patrol handled an infrastructure issue, verified the outcome, and recorded what happened.',
      ),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText('Legacy status asked for MCP readiness after the outcome was verified.'),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/Continue through Patrol investigation/i)).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Patrol history' })).not.toBeInTheDocument();
    const patrolControlLink = screen.getByRole('link', { name: 'Choose Patrol mode' });
    expect(patrolControlLink).toHaveAttribute('href', PATROL_CONTROL_STARTER_URL);
  });

  it('does not turn legacy Pro activation starter evidence into plan-page Patrol work', async () => {
    mockEntitlements = {
      capabilities: [
        'relay',
        'mobile_app',
        'push_notifications',
        'ai_alerts',
        'ai_autofix',
        'rbac',
        'audit_logging',
        'advanced_reporting',
        'agent_profiles',
      ],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      licensed_email: 'owner@example.com',
      max_history_days: 90,
      trial_eligible: false,
      runtime: PRO_RUNTIME_IDENTITY,
    };
    fetchAgentOperationsLoopStatusMock.mockResolvedValue(
      buildOperationsLoopStatus({
        nextAction: 'run_patrol',
        progressLabel: 'Legacy Pro activation entry recorded.',
        proActivationOperationsLoopStarterCount: 1,
        proActivationCompletedOperationsLoopCount: 0,
        proActivationResolvedOperationsLoopCount: 0,
        proActivationValueProofState: 'in_progress',
        externalAgentReady: false,
      }),
    );

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Current plan: Pulse Pro')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(fetchAgentOperationsLoopStatusMock).toHaveBeenCalled();
    });
    expect(screen.queryByText('Patrol work')).not.toBeInTheDocument();
    expect(screen.queryByText('Started')).not.toBeInTheDocument();
    expect(screen.queryByText('Legacy Pro activation entry recorded.')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Continue Patrol work' })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Choose Patrol mode' })).toHaveAttribute(
      'href',
      PATROL_CONTROL_STARTER_URL,
    );
  });

  it('keeps terminal Patrol decisions out of plan capability details', async () => {
    mockEntitlements = {
      capabilities: [
        'relay',
        'mobile_app',
        'push_notifications',
        'ai_alerts',
        'ai_autofix',
        'rbac',
        'audit_logging',
        'advanced_reporting',
        'agent_profiles',
      ],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      licensed_email: 'owner@example.com',
      max_history_days: 90,
      trial_eligible: false,
      runtime: PRO_RUNTIME_IDENTITY,
    };
    fetchAgentOperationsLoopStatusMock.mockResolvedValue(
      buildOperationsLoopStatus({
        nextAction: 'complete',
        progressLabel:
          'Patrol recorded a rejected change decision. Nothing was changed; approve a safer fix before marking the issue resolved.',
        patrolControlOperationsLoopStarterCount: 1,
        patrolControlCompletedOperationsLoopCount: 1,
        patrolControlResolvedOperationsLoopCount: 0,
        patrolControlValueState: 'governed_decision_recorded',
        externalAgentReady: false,
      }),
    );

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Current plan: Pulse Pro')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(fetchAgentOperationsLoopStatusMock).toHaveBeenCalled();
    });
    expect(screen.queryByText('Patrol work')).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol autonomy loop')).not.toBeInTheDocument();
    expect(screen.queryByText('Decision recorded')).not.toBeInTheDocument();
    expect(
      screen.getByText(
        'Pulse Pro is active on this instance. Review the current Patrol decision.',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/External-agent setup/i)).not.toBeInTheDocument();
    const patrolControlLink = screen.getByRole('link', { name: 'Review Patrol decision' });
    expect(patrolControlLink).toHaveAttribute('href', PATROL_CONTROL_PATH);
    patrolControlLink.addEventListener('click', (event) => event.preventDefault());
    fireEvent.click(patrolControlLink);
  });

  it('keeps verified Patrol history behind the Patrol mode action', async () => {
    mockEntitlements = {
      capabilities: [
        'relay',
        'mobile_app',
        'push_notifications',
        'ai_alerts',
        'ai_autofix',
        'rbac',
        'audit_logging',
        'advanced_reporting',
        'agent_profiles',
      ],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      licensed_email: 'owner@example.com',
      max_history_days: 90,
      trial_eligible: false,
      runtime: PRO_RUNTIME_IDENTITY,
    };
    fetchAgentOperationsLoopStatusMock.mockResolvedValue(
      buildOperationsLoopStatus({
        nextAction: 'complete',
        progressLabel:
          'Patrol verified the issue outcome and recorded the governed action history.',
        patrolControlOperationsLoopStarterCount: 1,
        patrolControlResolvedOperationsLoopCount: 1,
        patrolControlValueState: 'verified',
        externalAgentReady: true,
      }),
    );

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Current plan: Pulse Pro')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(fetchAgentOperationsLoopStatusMock).toHaveBeenCalled();
    });
    expect(screen.queryByText('Patrol work')).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol autonomy loop')).not.toBeInTheDocument();
    expect(screen.queryByText('Verified')).not.toBeInTheDocument();
    expect(
      screen.queryByText(
        'Patrol handled an infrastructure issue, verified the outcome, and recorded what happened.',
      ),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Patrol history' })).not.toBeInTheDocument();
    const patrolControlLink = screen.getByRole('link', { name: 'Choose Patrol mode' });
    expect(patrolControlLink).toHaveAttribute('href', PATROL_CONTROL_STARTER_URL);
    patrolControlLink.addEventListener('click', (event) => event.preventDefault());
    fireEvent.click(patrolControlLink);
  });

  it('shows active recurring v5 plan terms as unmetered even if stale limit metadata is present', async () => {
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
      runtime: PRO_RUNTIME_IDENTITY,
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
        'Included',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Included Monitored Systems')).not.toBeInTheDocument();
    expect(screen.queryByText('Legacy monitoring continuity')).not.toBeInTheDocument();
    expect(screen.queryByText('Plan Monitored System Limit')).not.toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Plan' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
  });

  it('shows lifetime grandfathered plans as unmetered', async () => {
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
      runtime: PRO_RUNTIME_IDENTITY,
    };

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Plan Terms')).toBeInTheDocument();
    });

    expect(screen.getByText('V5 Lifetime Grandfathered')).toBeInTheDocument();
    expect(
      screen.getByText('Current tier and enabled capabilities.'),
    ).toBeInTheDocument();
    expect(screen.queryByText('Included Monitored Systems')).not.toBeInTheDocument();
    expect(screen.queryByText('Guest Capacity')).not.toBeInTheDocument();
    expect(screen.getByText('Core Monitoring')).toBeInTheDocument();
    expect(screen.queryByText('Monitored Systems')).not.toBeInTheDocument();
    expect(screen.queryByText('Capacity Status')).not.toBeInTheDocument();
    expect(screen.queryByText('Monitored-system policy')).not.toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
    expect(screen.getByText('Permanent')).toBeInTheDocument();
    expect(screen.queryByText('Unlimited')).not.toBeInTheDocument();
    expect(screen.getAllByText('Included').length).toBeGreaterThan(0);
    expect(screen.queryByText('5 / 12')).not.toBeInTheDocument();
  });

  it('shows recurring grandfathered v5 Pro plans as unmetered while they remain active', async () => {
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
        runtime: PRO_RUNTIME_IDENTITY,
      };

      renderPanel();

      await waitFor(() => {
        expect(screen.getByText('Plan Terms')).toBeInTheDocument();
      });

      expect(screen.getByText(tc.expectedLabel)).toBeInTheDocument();
      expect(screen.getByText('Grandfathered v5 pricing')).toBeInTheDocument();
      expect(screen.getByText('Grandfathered price')).toBeInTheDocument();
      expect(
        screen.getAllByText(/keeps its existing recurring price until/i).length,
      ).toBeGreaterThan(0);
      expect(
        screen.queryByText(/keeps its existing recurring price and uncapped guest capacity/i),
      ).not.toBeInTheDocument();
      expect(
        screen.getAllByText(
          /self-hosted monitoring and child-resource volume are not metered in current v6 self-hosted packaging/i,
        ).length,
      ).toBeGreaterThan(0);
      expect(
        within(screen.getByText('Core Monitoring').parentElement as HTMLElement).getByText(
          'Included',
        ),
      ).toBeInTheDocument();
      expect(screen.queryByText('Capacity Status')).not.toBeInTheDocument();
      expect(screen.queryByText('Included Monitored Systems')).not.toBeInTheDocument();
      expect(screen.queryByText('Guest Capacity')).not.toBeInTheDocument();
      expect(screen.queryByText('Monitored-system policy')).not.toBeInTheDocument();
      expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
      expect(screen.getAllByText('Included').length).toBeGreaterThan(0);
      expect(screen.queryByText('Plan Monitored System Limit')).not.toBeInTheDocument();

      cleanup();
    }
  });

  it('uses shared current-plan metadata for unmetered retail self-hosted tiers', async () => {
    mockEntitlements = {
      capabilities: [
        'relay',
        'mobile_app',
        'push_notifications',
        'long_term_metrics',
        'ai_patrol',
        'ai_alerts',
        'ai_autofix',
        'advanced_sso',
        'rbac',
        'audit_logging',
        'advanced_reporting',
        'agent_profiles',
      ],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      plan_version: 'pro_monthly',
      licensed_email: 'owner@example.com',
      max_history_days: 90,
      trial_eligible: false,
      runtime: PRO_RUNTIME_IDENTITY,
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
        'Pulse Pro is active on this instance. It includes Patrol modes (Ask first, Safe auto-fix, Autopilot), 90-day metric history, RBAC, audit logging, reporting, and agent profiles.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('Primary capabilities')).toBeInTheDocument();
    expect(screen.getByText('Patrol Investigates Issues')).toBeInTheDocument();
    expect(screen.getByText('Patrol Handles Safe Fixes')).toBeInTheDocument();
    expect(screen.getByText('Included extras')).toBeInTheDocument();
    expect(screen.getByText('Capability details')).toBeInTheDocument();
    expect(screen.getByText('(5 items ready)')).toBeInTheDocument();
    const readinessDetails = screen
      .getByText('Capability details')
      .closest('details') as HTMLDetailsElement | null;
    expect(readinessDetails).not.toBeNull();
    expect(readinessDetails?.open).toBe(false);
    expect(
      screen.getByText(
        'Open this only when a Pro capability looks unavailable. Normal setup is choosing Patrol mode.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('Remote access, pairing, and push')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Relay, Pulse Mobile pairing, and push notifications are available on this instance.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('Patrol investigation and remediation')).toBeInTheDocument();
    expect(screen.getByText('Team controls')).toBeInTheDocument();
    expect(screen.getAllByText('Active').length).toBeGreaterThanOrEqual(4);
    expect(
      screen.queryByText(new RegExp(['optional', 'activation'].join(' '), 'i')),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(new RegExp(['recover', 'activation'].join(' '), 'i')),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/entitlement payload/i)).not.toBeInTheDocument();
    expect(screen.queryByText('Needs attention')).not.toBeInTheDocument();
    expect(screen.queryByText('Available plans')).not.toBeInTheDocument();
    expect(screen.getByText('90 days')).toBeInTheDocument();
    expect(screen.getByText('Patrol modes, history, and team controls')).toBeInTheDocument();
    expect(screen.queryByText('Guest Capacity')).not.toBeInTheDocument();
    expect(screen.queryByText('Included Monitored Systems')).not.toBeInTheDocument();
    expect(screen.queryByText('Monitored-system policy')).not.toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
  });

  it('shows Relay entitlement summaries from the paid capabilities available on this instance', async () => {
    mockEntitlements = {
      capabilities: ['relay', 'mobile_app', 'push_notifications'],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'relay',
      plan_version: 'relay_monthly',
      licensed_email: 'owner@example.com',
      max_history_days: 14,
      trial_eligible: false,
    };

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Current plan: Relay')).toBeInTheDocument();
    });

    expect(
      screen.getByText(
        'Relay is active on this instance. It includes remote web access, Pulse Mobile pairing, push notifications, and 14-day metric history.',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Available plans')).not.toBeInTheDocument();
    expect(screen.queryByText('Relay plan')).not.toBeInTheDocument();
    expect(screen.queryByText('Pulse Pro plan')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'View plans' })).not.toBeInTheDocument();
    expect(screen.getByText('Relay status')).toBeInTheDocument();
    expect(screen.getByText('Remote access, pairing, and push')).toBeInTheDocument();
    expect(screen.getAllByText('14-day metric history').length).toBeGreaterThan(0);
    expect(screen.getByText('Pulse Relay (Remote Access)')).toBeInTheDocument();
    expect(screen.getByText('Pulse Mobile Pairing')).toBeInTheDocument();
    expect(screen.getByText('Push Notifications')).toBeInTheDocument();
  });

  it('keeps pending fallback migration volume metadata audit-only on self-hosted Pro', async () => {
    mockEntitlements = {
      capabilities: ['relay'],
      limits: [
        {
          key: 'max_monitored_systems',
          limit: 10,
          current: 0,
          state: 'ok',
        },
      ],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      plan_version: 'legacy_migration_fallback',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
      runtime: PRO_RUNTIME_IDENTITY,
    };

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Current plan: Pulse Pro')).toBeInTheDocument();
    });

    expect(screen.getByText('Legacy Migration Fallback')).toBeInTheDocument();
    expect(screen.getByText('Core Monitoring')).toBeInTheDocument();
    expect(
      within(screen.getByText('Core Monitoring').parentElement as HTMLElement).getByText(
        'Included',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Legacy continuity verification pending')).not.toBeInTheDocument();
    expect(screen.queryByText('Continuity pending')).not.toBeInTheDocument();
    expect(screen.queryByText('Plan Monitored System Limit')).not.toBeInTheDocument();
    expect(screen.queryByText('Effective Monitored System Limit')).not.toBeInTheDocument();
    expect(screen.queryByText('Continuity Capture')).not.toBeInTheDocument();
    expect(screen.queryByText('Monitored-system policy')).not.toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
    expect(screen.queryByText('0 / 10')).not.toBeInTheDocument();
  });

  it('hides over-policy fallback migration volume warnings on recognized self-hosted plans', async () => {
    mockEntitlements = {
      capabilities: ['relay'],
      limits: [
        {
          key: 'max_monitored_systems',
          limit: 10,
          current: 23,
          state: 'enforced',
        },
      ],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      plan_version: 'legacy_migration_fallback',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
      runtime: PRO_RUNTIME_IDENTITY,
    };

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Current plan: Pulse Pro')).toBeInTheDocument();
    });

    expect(screen.getByText('Core Monitoring')).toBeInTheDocument();
    expect(
      within(screen.getByText('Core Monitoring').parentElement as HTMLElement).getByText(
        'Included',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/identified 23 monitored systems/i)).not.toBeInTheDocument();
    expect(screen.queryByText('Legacy continuity verification pending')).not.toBeInTheDocument();
    expect(screen.queryByText('Monitored-system policy')).not.toBeInTheDocument();
    expect(screen.queryByText('Why is continuity still pending?')).not.toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
    expect(
      screen.queryByText(
        'New top-level additions are paused until this legacy continuity state is reviewed.',
      ),
    ).not.toBeInTheDocument();
  });

  it('does not surface captured fallback migration floors as self-hosted plan limits', async () => {
    mockEntitlements = {
      capabilities: ['relay'],
      limits: [
        {
          key: 'max_monitored_systems',
          limit: 23,
          current: 23,
          state: 'enforced',
        },
      ],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      plan_version: 'legacy_migration_fallback',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
      runtime: PRO_RUNTIME_IDENTITY,
    };

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Current plan: Pulse Pro')).toBeInTheDocument();
    });

    expect(screen.getByText('Core Monitoring')).toBeInTheDocument();
    expect(
      within(screen.getByText('Core Monitoring').parentElement as HTMLElement).getByText(
        'Included',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Legacy monitoring continuity')).not.toBeInTheDocument();
    expect(screen.queryByText(/observed legacy estate available/i)).not.toBeInTheDocument();
    expect(screen.queryByText('Legacy continuity')).not.toBeInTheDocument();
    expect(screen.queryByText('23 monitored systems')).not.toBeInTheDocument();
    expect(screen.queryByText('Monitored-system policy')).not.toBeInTheDocument();
    expect(
      screen.queryByText('Existing monitoring continues. Additional monitored systems are paused.'),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('link', { name: 'Review monitored systems' }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText('Plan Monitored System Limit')).not.toBeInTheDocument();
    expect(screen.queryByText('Effective Monitored System Limit')).not.toBeInTheDocument();
    expect(screen.queryByText('Grandfathered Floor')).not.toBeInTheDocument();
    expect(screen.queryByText('Included Monitored Systems')).not.toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
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
      runtime: PRO_RUNTIME_IDENTITY,
    };

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Core SSO (OIDC/SAML)')).toBeInTheDocument();
    });

    // Verify self-hosted displayable capabilities render with their expected labels.
    expect(screen.getByText('Pulse Patrol')).toBeInTheDocument();
    expect(screen.getByText('Patrol Investigates Issues')).toBeInTheDocument();
    expect(screen.getByText('Patrol Handles Safe Fixes')).toBeInTheDocument();
    expect(screen.getByText('Update Alerts')).toBeInTheDocument();
    expect(screen.getByText('Role-Based Access Control (RBAC)')).toBeInTheDocument();
    expect(screen.getByText('Audit Logging')).toBeInTheDocument();
    expect(screen.getByText('PDF/CSV Reporting')).toBeInTheDocument();
    expect(screen.getByText('Centralized Agent Profiles')).toBeInTheDocument();
    expect(screen.getByText('Pulse Relay (Remote Access)')).toBeInTheDocument();
    expect(screen.getByText('Pulse Mobile Pairing')).toBeInTheDocument();
    expect(screen.getByText('Push Notifications')).toBeInTheDocument();
    expect(screen.getByText('Extended Metric History')).toBeInTheDocument();
    expect(screen.queryByText('Multi-Tenant Mode')).not.toBeInTheDocument();
    expect(screen.queryByText('Kubernetes AI Analysis (Compatibility)')).not.toBeInTheDocument();
    expect(screen.queryByText('Multi-User Mode')).not.toBeInTheDocument();
    expect(screen.queryByText('White-Label Branding')).not.toBeInTheDocument();
    expect(screen.queryByText('Unlimited Instances')).not.toBeInTheDocument();
  });

  it('shows migration guidance when the pasted key looks like a legacy v5 license', async () => {
    renderPanel();

    fireEvent.click(screen.getByText('Manual key recovery'));

    fireEvent.input(screen.getByLabelText(/license key/i), {
      target: { value: 'header.payload.signature' },
    });

    expect(screen.getByText('Legacy v5 license detected')).toBeInTheDocument();
    expect(screen.getByText(/migrate this key automatically/i)).toBeInTheDocument();
  });

  it.each([
    {
      purchase: SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED,
      title: 'Pulse Pro is now active',
      actionLabel: 'Choose Patrol mode',
      actionHref: PATROL_CONTROL_STARTER_URL,
    },
    {
      purchase: SELF_HOSTED_PRO_BILLING_PURCHASE_CANCELLED,
      title: 'Checkout cancelled',
      actionLabel: 'View plans',
      actionHref: getSelfHostedPurchaseStartUrl(SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT),
      redirectedHref: getSelfHostedBillingHref('plan', {
        intent: SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
      }),
    },
    {
      purchase: SELF_HOSTED_PRO_BILLING_PURCHASE_EXPIRED,
      title: 'Upgrade return expired',
      actionLabel: 'View plans',
      actionHref: getSelfHostedPurchaseStartUrl(),
    },
    {
      purchase: SELF_HOSTED_PRO_BILLING_PURCHASE_FAILED,
      title: 'Plan needs attention',
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
          runtime: PRO_RUNTIME_IDENTITY,
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
          screen.getByText(
            /Checkout completed and Pulse Pro is active\. Choose Patrol mode\./i,
          ),
        ).toBeInTheDocument();
        expect(screen.getByText('Available now on this instance')).toBeInTheDocument();
        expect(screen.getAllByText('Patrol Handles Safe Fixes').length).toBeGreaterThan(0);
        expect(screen.getAllByRole('link', { name: 'Choose Patrol mode' })).toHaveLength(1);
        const patrolControlLink = screen.getByRole('link', { name: 'Choose Patrol mode' });
        expect(patrolControlLink).toHaveAttribute('href', PATROL_CONTROL_STARTER_URL);
        expect(patrolControlLink).not.toHaveAttribute('target');
        patrolControlLink.addEventListener('click', (event) => event.preventDefault());
        fireEvent.click(patrolControlLink);
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

  it('keeps purchase activation success on Patrol mode when history is already verified', async () => {
    mockEntitlements = {
      capabilities: ['relay', 'mobile_app', 'push_notifications', 'ai_autofix'],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
      runtime: PRO_RUNTIME_IDENTITY,
    };
    fetchAgentOperationsLoopStatusMock.mockResolvedValue(
      buildOperationsLoopStatus({
        nextAction: 'complete',
        progressLabel:
          'Patrol verified the issue outcome and recorded the governed action history.',
        patrolControlOperationsLoopStarterCount: 1,
        patrolControlCompletedOperationsLoopCount: 1,
        patrolControlResolvedOperationsLoopCount: 1,
        patrolControlValueState: 'verified',
        externalAgentReady: true,
      }),
    );
    useLocationMock.mockReturnValue({
      search: `?purchase=${SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED}`,
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    expect(screen.getByText('Pulse Pro is now active')).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByRole('link', { name: 'Choose Patrol mode' })).toBeInTheDocument();
    });

    expect(screen.queryByRole('link', { name: 'Patrol history' })).not.toBeInTheDocument();
    const patrolControlLink = screen.getByRole('link', { name: 'Choose Patrol mode' });
    expect(patrolControlLink).toHaveAttribute('href', PATROL_CONTROL_STARTER_URL);
    patrolControlLink.addEventListener('click', (event) => event.preventDefault());
    fireEvent.click(patrolControlLink);
  });

  it('returns self-hosted plan purchases to the plan surface instead of the legacy usage tab', async () => {
    mockEntitlements = {
      capabilities: ['relay', 'mobile_app', 'push_notifications', 'ai_autofix'],
      limits: [],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
      runtime: PRO_RUNTIME_IDENTITY,
    };
    useLocationMock.mockReturnValue({
      search: `?purchase=${SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED}&intent=${SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT}`,
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    expect(screen.getByText('Pulse Pro is now active')).toBeInTheDocument();
    expect(
      screen.getByText(
        /Checkout completed and Pulse Pro is active\. Choose Patrol mode\./i,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Select a plan')).not.toBeInTheDocument();
    expect(screen.getAllByText('Patrol Handles Safe Fixes').length).toBeGreaterThan(0);
    expect(screen.queryByText('Available plans')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Review plan' })).not.toBeInTheDocument();
  });

  it('does not treat checkout return parameters as entitlement truth', async () => {
    useLocationMock.mockReturnValue({
      search: `?purchase=${SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED}&intent=${SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT}`,
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    await waitFor(() => {
      expect(loadLicenseEntitlementsMock).toHaveBeenCalled();
    });

    expect(screen.queryByText('Pulse Pro is now active')).not.toBeInTheDocument();
    expect(
      screen.queryByText(
        /Checkout completed and Pulse Pro is active\. Choose Patrol mode\./i,
      ),
    ).not.toBeInTheDocument();
    expect(screen.getByText('Current plan: Community')).toBeInTheDocument();
    expect(screen.queryByText('Patrol Handles Safe Fixes')).not.toBeInTheDocument();
    expect(screen.queryByText('Available now on this instance')).not.toBeInTheDocument();
    expect(navigateMock).toHaveBeenCalledWith(SELF_HOSTED_PRO_BILLING_PLAN_HREF, {
      replace: true,
      scroll: false,
    });
  });

  it('opens recovery by default when the billing route requests the recovery detail', async () => {
    useLocationMock.mockReturnValue({
      search: '?details=recovery',
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    const recoveryDisclosure = screen.getAllByText('Manual key recovery')[0]?.closest('details');
    expect(recoveryDisclosure).toHaveAttribute('open');
  });

  it('redirects stale bounded billing usage routes back to the plan surface', async () => {
    mockEntitlements = {
      capabilities: ['relay'],
      limits: [{ key: 'max_monitored_systems', limit: 10, current: 5, state: 'ok' }],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      plan_version: 'legacy_migration_fallback',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
      runtime: PRO_RUNTIME_IDENTITY,
    };
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
    expect(screen.queryByText('Monitored Systems')).not.toBeInTheDocument();
    expect(
      document.getElementById(SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID),
    ).not.toBeInTheDocument();
    expect(document.getElementById(SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID)).toBeInTheDocument();
    expect(navigateMock).toHaveBeenCalledWith(SELF_HOSTED_PRO_BILLING_PLAN_ROUTE, {
      replace: true,
      scroll: false,
    });
    expect(screen.queryByRole('button', { name: 'Hide counting rules' })).not.toBeInTheDocument();
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

    expect(screen.getAllByText('Select a plan').length).toBeGreaterThan(0);
    expect(screen.getByText(/Community is active on this instance/i)).toBeInTheDocument();
    expect(screen.getAllByText('Watch-only Patrol').length).toBeGreaterThan(0);
    const compareLinks = screen.getAllByRole('link', { name: 'View plans' });
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

  it('does not expose billing usage subtabs for stale legacy capacity on recognized plans', async () => {
    mockEntitlements = {
      capabilities: ['relay'],
      limits: [{ key: 'max_monitored_systems', limit: 10, current: 5, state: 'ok' }],
      subscription_state: 'active',
      upgrade_reasons: [],
      tier: 'pro',
      plan_version: 'legacy_migration_fallback',
      licensed_email: 'owner@example.com',
      trial_eligible: false,
      runtime: PRO_RUNTIME_IDENTITY,
    };
    useLocationMock.mockReturnValue({
      search: '',
      pathname: '/settings/system/billing/plan',
      hash: '',
    });

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText('Current plan: Pulse Pro')).toBeInTheDocument();
    });

    expect(screen.queryByRole('tab', { name: 'Usage' })).not.toBeInTheDocument();
    expect(screen.queryByText('Monitored-system policy')).not.toBeInTheDocument();
    expect(navigateMock).not.toHaveBeenCalledWith(SELF_HOSTED_PRO_BILLING_USAGE_HREF, {
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
    expect(screen.getByText(/retry from this instance/i)).toBeInTheDocument();
  });

  it('shows reason-specific guidance when the v5 key is unsupported', async () => {
    mockEntitlements = {
      capabilities: [],
      limits: [],
      subscription_state: 'expired',
      upgrade_reasons: [],
      tier: 'free',
      trial_eligible: false,
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

  it('shows retrieve-license guidance when a v5 key has been superseded', async () => {
    mockEntitlements = {
      capabilities: [],
      limits: [],
      subscription_state: 'expired',
      upgrade_reasons: [],
      tier: 'free',
      trial_eligible: false,
      commercial_migration: {
        source: 'v5_license',
        state: 'failed',
        reason: 'exchange_stale_key',
        recommended_action: 'retrieve_current_key',
      },
    };

    renderPanel();

    const title = screen.getByText('v5 license migration needs attention');
    expect(title).toBeInTheDocument();
    expect(title.closest('.border-red-300')).not.toBeNull();
    expect(screen.getByText(/superseded by a renewal/i)).toBeInTheDocument();
    expect(screen.getByText(/pulserelay\.pro\/retrieve-license/i)).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /start 14-day pro trial/i }),
    ).not.toBeInTheDocument();
  });

  it('shows blocked-egress guidance after sustained exchange transport failure', async () => {
    mockEntitlements = {
      capabilities: [],
      limits: [],
      subscription_state: 'expired',
      upgrade_reasons: [],
      tier: 'free',
      trial_eligible: false,
      commercial_migration: {
        source: 'v5_license',
        state: 'pending',
        reason: 'exchange_connectivity_required',
        recommended_action: 'allow_license_egress',
        first_failed_at: 1_700_000_000,
      },
    };

    renderPanel();

    const title = screen.getByText('v5 license migration pending');
    expect(title).toBeInTheDocument();
    expect(title.closest('.border-amber-300')).not.toBeNull();
    expect(
      screen.getByText(/paid v6 features require periodic outbound HTTPS/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/allow outbound HTTPS to license\.pulserelay\.pro/i),
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
    expect(selfHostedCommercialRecoverySectionSource).toContain('FormTextarea');
    expect(selfHostedCommercialRecoverySectionSource).not.toContain('<textarea');
    expect(selfHostedCommercialRecoverySectionSource).toContain('@/components/shared/Button');
    expect(selfHostedCommercialRecoverySectionSource).toContain('variant="primary"');
    expect(selfHostedCommercialRecoverySectionSource).toContain('variant="outline"');
    expect(selfHostedCommercialRecoverySectionSource).toContain('size="settingsAction"');
    expect(selfHostedCommercialRecoverySectionSource).not.toContain(
      'min-h-10 sm:min-h-9 px-4 py-2.5 text-sm font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700 transition-colors disabled:opacity-60 disabled:cursor-not-allowed',
    );
    expect(selfHostedCommercialRecoverySectionSource).not.toContain(
      'min-h-10 sm:min-h-9 px-4 py-2.5 text-sm font-medium rounded-md border border-border text-base-content hover:bg-surface-hover transition-colors disabled:opacity-60 disabled:cursor-not-allowed',
    );
    expect(proLicensePanelSource).toContain('SELF_HOSTED_PRO_BILLING_PRESENTATION');
    expect(proLicensePanelSource).toContain('value={state.activeSection()}');
    expect(proLicensePanelSource).toContain('<Subtabs');
    expect(proLicensePanelSource).not.toContain('createSignal(');
    expect(proLicensePanelSource).not.toContain('useLocation()');
    expect(proLicensePanelStateSource).toContain('useLocation');
    expect(proLicensePanelStateSource).toContain('resolveSelfHostedBillingSection');
    expect(proLicensePanelStateSource).toContain('getSelfHostedBillingPlanIntent');
    expect(proLicensePanelStateSource).not.toContain('getSelfHostedBillingUsageDetail');
    expect(proLicensePanelStateSource).toContain('const setActiveSection = (section: string) => {');
    expect(proLicensePanelStateSource).toContain('loadLicenseEntitlements(true)');
    expect(proLicensePanelStateSource).toContain('fetchAgentOperationsLoopStatus');
    expect(proLicensePanelStateSource).toContain('loadCommercialPosture(true)');
    expect(proLicensePanelStateSource).toContain('loadRuntimeCapabilities(true)');
    expect(proLicensePanelStateSource).toContain('buildSelfHostedCommercialPlanModel');
    expect(proLicensePanelStateSource).toContain('getSelfHostedCurrentPlanPresentation({');
    expect(proLicensePanelStateSource).toContain('getSelfHostedCurrentPlanStatusPresentation');
    expect(proLicensePanelStateSource).toContain('getSelfHostedPlanComparisonPresentation({');
    expect(proLicensePanelStateSource).toContain('getSelfHostedActivationSuccessPresentation({');
    expect(proLicensePanelStateSource).not.toContain('runStartProTrialAction({');
    expect(proLicensePanelStateSource).not.toContain('startProTrial()');
    expect(proLicensePanelStateSource).toContain("'A license key is required'");
    expect(proLicensePanelSource).toContain('@/components/shared/Button');
    expect(proLicensePanelSource).toContain('variant="outline"');
    expect(proLicensePanelSource).toContain('size="settingsAction"');
    expect(proLicensePanelSource).not.toContain(
      'inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-3 py-2 text-sm font-medium rounded-md border border-border text-base-content hover:bg-surface-hover transition-colors disabled:opacity-60',
    );
    expect(proLicensePlanSectionSource).toContain('getLicenseStatusLoadingState');
    expect(proLicensePlanSectionSource).toContain('getNoActiveSelfHostedActivationState');
    expect(proLicensePlanSectionSource).not.toContain('getTrialEndedProLicenseNotice');
    expect(proLicensePlanSectionSource).toContain('currentPlanSummary.title');
    expect(proLicensePlanSectionSource).toContain('currentPlanSummary.supplementalBadges');
    expect(proLicensePlanSectionSource).toContain('currentPlanSummary.privateRuntimeAction');
    expect(proLicensePlanSectionSource).toContain('props.activationSuccessSummary');
    expect(proLicensePlanSectionSource).toContain('summary().actionUrl');
    expect(proLicensePlanSectionSource).toContain('ButtonLink');
    expect(proLicensePlanSectionSource).toContain('variant="warning"');
    expect(proLicensePlanSectionSource).toContain('size="settingsActionXs"');
    expect(proLicensePlanSectionSource).not.toContain(
      'mt-2 inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-3 py-2 text-xs font-medium rounded-md border border-amber-300 dark:border-amber-700 text-amber-800 dark:text-amber-200 hover:bg-amber-100 dark:hover:bg-amber-800 transition-colors disabled:opacity-60',
    );
    expect(proLicensePlanSectionSource).toContain('UpgradeButtonLink');
    expect(selfHostedCommercialRecoverySectionSource).toContain('ExternalTextLink');
    expect(selfHostedCommercialRecoverySectionSource).not.toContain('target="_blank"');
    expect(selfHostedCommercialRecoverySectionSource).not.toContain('rel="noopener noreferrer"');
    expect(proLicensePlanSectionSource).not.toContain(
      'inline-flex items-center gap-1 mt-3 min-h-10 sm:min-h-9 rounded-md border border-current/20 px-3 py-2 text-xs font-medium hover:bg-black/5 dark:hover:bg-white/5',
    );
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
    expect(proLicensePlanSectionSource).not.toContain('trialEndedNotice');
    expect(proLicensePlanSectionSource).not.toContain('props.trialEnded');
    expect(proLicensePlanSectionSource).toContain('planSelectionPrompt');
    expect(proLicensePlanSectionSource).not.toContain(
      "resolveSelfHostedPurchaseStartDestination('self_hosted_plan')",
    );
    expect(proLicensePlanSectionSource).not.toContain('Your Pro trial has ended');
    expect(proLicensePlanSectionSource).not.toContain('Turn alert noise into');
    expect(selfHostedCommercialRecoverySectionSource).toContain(
      'SELF_HOSTED_RECOVERY_PRESENTATION',
    );
    expect(selfHostedCommercialRecoverySectionSource).toContain('TERMS_DOC_URL');
    expect(selfHostedCommercialRecoverySectionSource).toContain('disclosureLabel');
    expect(selfHostedCommercialRecoverySectionSource).toContain('privateRuntimeNotice');
    expect(selfHostedCommercialRecoverySectionSource).toContain('recoverySectionTitle');
    expect(selfHostedCommercialRecoverySectionSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/TERMS.md',
    );
    expect(selfHostedCommercialRecoverySectionSource).not.toContain('Start 14-day Pro Trial');
    expect(selfHostedCommercialRecoverySectionSource).not.toContain('Legacy v5 license detected');
    expect(proLicensePanelSource).toContain('id={SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID}');
    expect(proLicensePanelSource).not.toContain('SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID');
  });
});
