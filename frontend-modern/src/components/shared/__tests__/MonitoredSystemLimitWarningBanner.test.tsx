import { cleanup, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import monitoredSystemLimitWarningBannerSource from '@/components/shared/MonitoredSystemLimitWarningBanner.tsx?raw';
import monitoredSystemLimitWarningBannerModelSource from '@/components/shared/monitoredSystemLimitWarningBannerModel.ts?raw';
import monitoredSystemLimitWarningBannerStateSource from '@/components/shared/useMonitoredSystemLimitWarningBannerState.ts?raw';

type MockEntitlements = {
  overflow_days_remaining?: number;
};
const mockEntitlements = vi.hoisted(() =>
  vi.fn<() => MockEntitlements>(() => ({ overflow_days_remaining: undefined })),
);
type MockLimitRecord = {
  key: string;
  limit: number;
  current: number;
  current_available?: boolean;
  current_unavailable_reason?: string;
  state: string;
};
const mockGetLimit = vi.hoisted(() =>
  vi.fn<(key: string) => MockLimitRecord | undefined>(() => undefined),
);
const mockGetMonitoredSystemCapacity = vi.hoisted(() => vi.fn(() => undefined));
const mockHasMigrationGap = vi.hoisted(() => vi.fn(() => false));
const mockLegacyConnections = vi.hoisted(() =>
  vi.fn(() => ({
    proxmox_nodes: 0,
    docker_hosts: 0,
    kubernetes_clusters: 0,
  })),
);
const mockTrackUpgradeMetricEvent = vi.hoisted(() => vi.fn());
const mockTrackUpgradeClicked = vi.hoisted(() => vi.fn());
const mockLoadRuntimeLicenseStatus = vi.hoisted(() => vi.fn());
const mockPresentationPolicyHidesCommercialSurfaces = vi.hoisted(() => vi.fn(() => false));
const mockGetUpgradeActionDestination = vi.hoisted(() => vi.fn());
const mockGetUpgradeActionUrlOrFallback = vi.hoisted(() => vi.fn());

vi.mock('@/stores/license', () => ({
  getRuntimeLimit: (key: string) => mockGetLimit(key),
  getRuntimeMonitoredSystemCapacity: () => mockGetMonitoredSystemCapacity(),
  loadRuntimeCapabilities: (force?: boolean) => mockLoadRuntimeLicenseStatus(force),
}));

vi.mock('@/stores/licenseCommercial', () => ({
  commercialOverflowDaysRemaining: () => mockEntitlements().overflow_days_remaining ?? null,
  getUpgradeActionDestination: (key: string) => mockGetUpgradeActionDestination(key),
  getUpgradeActionUrlOrFallback: (key: string) => mockGetUpgradeActionUrlOrFallback(key),
  hasMigrationGap: mockHasMigrationGap,
  legacyConnections: mockLegacyConnections,
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesCommercialSurfaces: () => mockPresentationPolicyHidesCommercialSurfaces(),
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  UPGRADE_METRIC_EVENTS: {
    LIMIT_WARNING_SHOWN: 'limit_warning_shown',
  },
  trackUpgradeMetricEvent: mockTrackUpgradeMetricEvent,
  trackUpgradeClicked: mockTrackUpgradeClicked,
}));

describe('MonitoredSystemLimitWarningBanner', () => {
  beforeEach(() => {
    localStorage.clear();
    mockEntitlements.mockReturnValue({ overflow_days_remaining: undefined });
    mockGetLimit.mockReturnValue({
      key: 'max_monitored_systems',
      limit: 6,
      current: 0,
      state: 'ok',
    });
    mockGetMonitoredSystemCapacity.mockReset();
    mockGetMonitoredSystemCapacity.mockReturnValue(undefined);
    mockHasMigrationGap.mockReturnValue(false);
    mockLegacyConnections.mockReturnValue({
      proxmox_nodes: 0,
      docker_hosts: 0,
      kubernetes_clusters: 0,
    });
    mockPresentationPolicyHidesCommercialSurfaces.mockReset();
    mockPresentationPolicyHidesCommercialSurfaces.mockReturnValue(false);
    mockLoadRuntimeLicenseStatus.mockReset();
    mockLoadRuntimeLicenseStatus.mockResolvedValue(undefined);
    mockGetUpgradeActionDestination.mockReset();
    mockGetUpgradeActionUrlOrFallback.mockReset();
    mockGetUpgradeActionDestination.mockReturnValue({
      href: '/settings/system/billing',
      external: false,
    });
    mockGetUpgradeActionUrlOrFallback.mockReturnValue(
      '/settings/system/billing/plan?intent=self_hosted_plan',
    );
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('keeps monitored system limit warning banner on shell, runtime, and model owners', () => {
    expect(monitoredSystemLimitWarningBannerSource).toContain(
      'useMonitoredSystemLimitWarningBannerState',
    );
    expect(monitoredSystemLimitWarningBannerSource).toContain(
      'MONITORED_SYSTEM_LIMIT_REVIEW_POLICY_LABEL',
    );
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('createEffect');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('createMemo');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('loadRuntimeCapabilities');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('trackUpgradeMetricEvent');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('legacyConnections()');

    expect(monitoredSystemLimitWarningBannerStateSource).toContain(
      'export function useMonitoredSystemLimitWarningBannerState',
    );
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('createEffect');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('createMemo');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('loadRuntimeCapabilities');
    expect(monitoredSystemLimitWarningBannerStateSource).not.toContain('loadCommercialPosture');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain(
      'presentationPolicyHidesCommercialSurfaces',
    );
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('trackUpgradeMetricEvent');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('hasMigrationGap');
    expect(monitoredSystemLimitWarningBannerStateSource).not.toContain(
      'scopeSelfHostedBillingDestination',
    );
    expect(monitoredSystemLimitWarningBannerStateSource).not.toContain(
      'SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT',
    );
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('reviewPolicyDestination');
    expect(monitoredSystemLimitWarningBannerStateSource).not.toContain('handleUpgradeClick');

    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      '@/utils/monitoredSystemPresentation',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'getMonitoredSystemBannerToneClass',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain(
      'getMonitoredSystemLimitUpgradeLabel',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'getMonitoredSystemLimitInstallCollectorsLabel',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'isMonitoredSystemLimitUsageAvailable',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain(
      'current_available !== false',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain('Upgrade to add more');
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain('Install v6 collectors');
  });

  it('stays hidden for non-urgent pure v6 installs', async () => {
    const mod = await import('../MonitoredSystemLimitWarningBanner');
    render(() => (
      <Router>
        <Route path="/" component={mod.MonitoredSystemLimitWarningBanner} />
      </Router>
    ));

    expect(mockLoadRuntimeLicenseStatus).toHaveBeenCalled();
    expect(screen.queryByText(/Monitored systems:/i)).not.toBeInTheDocument();
  });

  it('shows migration guidance when legacy connections exist', async () => {
    mockHasMigrationGap.mockReturnValue(true);
    mockLegacyConnections.mockReturnValue({
      proxmox_nodes: 2,
      docker_hosts: 1,
      kubernetes_clusters: 0,
    });

    const mod = await import('../MonitoredSystemLimitWarningBanner');
    render(() => (
      <Router>
        <Route path="/" component={mod.MonitoredSystemLimitWarningBanner} />
      </Router>
    ));

    expect(screen.queryByText(/Monitored systems:/i)).not.toBeInTheDocument();
  });

  it('keeps urgent limit warnings visible even without migration gap', async () => {
    mockGetLimit.mockReturnValue({
      key: 'max_monitored_systems',
      limit: 6,
      current: 5,
      state: 'warning',
    });

    const mod = await import('../MonitoredSystemLimitWarningBanner');
    render(() => (
      <Router>
        <Route path="/" component={mod.MonitoredSystemLimitWarningBanner} />
      </Router>
    ));

    expect(screen.getByText('1 remaining. 5 monitored, 6 included.')).toBeInTheDocument();
    expect(screen.getByText('Review policy')).toHaveAttribute(
      'href',
      '/settings/system/billing/usage',
    );
    expect(screen.queryByText('Review options')).not.toBeInTheDocument();
    expect(screen.queryByText('Install v6 collectors')).not.toBeInTheDocument();
  });

  it('stays hidden while canonical monitored-system usage is unavailable', async () => {
    mockGetLimit.mockReturnValue({
      key: 'max_monitored_systems',
      limit: 6,
      current: 6,
      current_available: false,
      current_unavailable_reason: 'supplemental_inventory_unsettled',
      state: 'enforced',
    });

    const mod = await import('../MonitoredSystemLimitWarningBanner');
    render(() => (
      <Router>
        <Route path="/" component={mod.MonitoredSystemLimitWarningBanner} />
      </Router>
    ));

    expect(screen.queryByText(/Monitored systems:/i)).not.toBeInTheDocument();
    expect(screen.queryByText('Review options')).not.toBeInTheDocument();
    expect(mockTrackUpgradeMetricEvent).not.toHaveBeenCalled();
  });

  it('keeps urgent limit warnings visible with migration context', async () => {
    mockHasMigrationGap.mockReturnValue(true);
    mockEntitlements.mockReturnValue({ overflow_days_remaining: 14 });
    mockGetLimit.mockReturnValue({
      key: 'max_monitored_systems',
      limit: 6,
      current: 5,
      state: 'warning',
    });
    mockLegacyConnections.mockReturnValue({
      proxmox_nodes: 2,
      docker_hosts: 1,
      kubernetes_clusters: 0,
    });

    const mod = await import('../MonitoredSystemLimitWarningBanner');
    render(() => (
      <Router>
        <Route path="/" component={mod.MonitoredSystemLimitWarningBanner} />
      </Router>
    ));

    expect(screen.getByText('1 remaining. 5 monitored, 6 included.')).toBeInTheDocument();
    expect(screen.getByText('Install v6 collectors')).toHaveAttribute('href', '/settings');
    expect(screen.getByText('Review policy')).toHaveAttribute(
      'href',
      '/settings/system/billing/usage',
    );
    expect(screen.queryByText('Review options')).not.toBeInTheDocument();
  });

  it('stays hidden in demo mode even when usage is urgent', async () => {
    mockPresentationPolicyHidesCommercialSurfaces.mockReturnValue(true);
    mockGetLimit.mockReturnValue({
      key: 'max_monitored_systems',
      limit: 6,
      current: 5,
      state: 'warning',
    });

    const mod = await import('../MonitoredSystemLimitWarningBanner');
    render(() => (
      <Router>
        <Route path="/" component={mod.MonitoredSystemLimitWarningBanner} />
      </Router>
    ));

    expect(
      screen.queryByText('1 remaining. 5 monitored, 6 included.'),
    ).not.toBeInTheDocument();
    expect(mockTrackUpgradeMetricEvent).not.toHaveBeenCalled();
  });
});
