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
  state: string;
};
const mockGetLimit = vi.hoisted(() =>
  vi.fn<(key: string) => MockLimitRecord | undefined>(() => undefined),
);
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
const mockLoadLicenseStatus = vi.hoisted(() => vi.fn());
const mockGetUpgradeActionDestination = vi.hoisted(() => vi.fn());
const mockGetUpgradeActionUrlOrFallback = vi.hoisted(() => vi.fn());

vi.mock('@/stores/license', () => ({
  entitlements: mockEntitlements,
  getLimit: mockGetLimit,
  getUpgradeActionDestination: (...args: unknown[]) => mockGetUpgradeActionDestination(...args),
  getUpgradeActionUrlOrFallback: (...args: unknown[]) => mockGetUpgradeActionUrlOrFallback(...args),
  hasMigrationGap: mockHasMigrationGap,
  legacyConnections: mockLegacyConnections,
  loadLicenseStatus: (...args: unknown[]) => mockLoadLicenseStatus(...args),
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
    mockGetLimit.mockReturnValue({ key: 'max_monitored_systems', limit: 6, current: 0, state: 'ok' });
    mockHasMigrationGap.mockReturnValue(false);
    mockLegacyConnections.mockReturnValue({
      proxmox_nodes: 0,
      docker_hosts: 0,
      kubernetes_clusters: 0,
    });
    mockLoadLicenseStatus.mockReset();
    mockLoadLicenseStatus.mockResolvedValue(undefined);
    mockGetUpgradeActionDestination.mockReset();
    mockGetUpgradeActionUrlOrFallback.mockReset();
    mockGetUpgradeActionDestination.mockReturnValue({
      href: '/settings/system/billing',
      external: false,
    });
    mockGetUpgradeActionUrlOrFallback.mockReturnValue('/settings/system/billing');
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
      'MONITORED_SYSTEM_LIMIT_LEARN_MORE_LABEL',
    );
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('createEffect');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('createMemo');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('loadLicenseStatus');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('trackUpgradeMetricEvent');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('legacyConnections()');

    expect(monitoredSystemLimitWarningBannerStateSource).toContain(
      'export function useMonitoredSystemLimitWarningBannerState',
    );
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('createEffect');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('createMemo');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('loadLicenseStatus');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('trackUpgradeMetricEvent');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('legacyConnections');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('handleUpgradeClick');

    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      "@/utils/monitoredSystemPresentation",
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'getMonitoredSystemMigrationMessage',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'getMonitoredSystemBannerToneClass',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'getMonitoredSystemLimitUpgradeLabel',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'getMonitoredSystemLimitInstallCollectorsLabel',
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

    expect(mockLoadLicenseStatus).toHaveBeenCalled();
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
    mockGetLimit.mockReturnValue({ key: 'max_monitored_systems', limit: 6, current: 5, state: 'warning' });

    const mod = await import('../MonitoredSystemLimitWarningBanner');
    render(() => (
      <Router>
        <Route path="/" component={mod.MonitoredSystemLimitWarningBanner} />
      </Router>
    ));

    expect(screen.getByText('Monitored systems: 5/6')).toBeInTheDocument();
    expect(screen.getByText('Upgrade to add more')).toBeInTheDocument();
    expect(screen.getByText('Upgrade to add more')).toHaveAttribute('href', '/settings/system/billing');
    expect(screen.queryByText('Install v6 collectors')).not.toBeInTheDocument();
  });

  it('keeps urgent limit warnings visible with migration context', async () => {
    mockHasMigrationGap.mockReturnValue(true);
    mockEntitlements.mockReturnValue({ overflow_days_remaining: 14 });
    mockGetLimit.mockReturnValue({ key: 'max_monitored_systems', limit: 6, current: 5, state: 'warning' });
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

    expect(screen.getByText('Monitored systems: 5/6')).toBeInTheDocument();
    expect(
      screen.getByText(
        /You also have 3 resources connected via API or legacy collectors \(2 Proxmox nodes, 1 Docker host\) that count once toward your monitored-system cap when the same top-level system is discovered canonically\./i,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('Install v6 collectors')).toHaveAttribute('href', '/settings');
    expect(
      screen.getByText('Includes 1 temporary onboarding slot \(14d remaining\)', { exact: false }),
    ).toBeInTheDocument();
    expect(screen.getByText('Upgrade to add more')).toHaveAttribute('href', '/settings/system/billing');
  });
});
