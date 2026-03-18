import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

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

vi.mock('@/stores/license', () => ({
  entitlements: mockEntitlements,
  getLimit: mockGetLimit,
  getUpgradeActionUrlOrFallback: vi.fn(() => '/pricing?feature=max_monitored_systems'),
  hasMigrationGap: mockHasMigrationGap,
  legacyConnections: mockLegacyConnections,
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
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('stays hidden for non-urgent pure v6 installs', async () => {
    const mod = await import('../MonitoredSystemLimitWarningBanner');
    render(() => <mod.MonitoredSystemLimitWarningBanner />);

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
    render(() => <mod.MonitoredSystemLimitWarningBanner />);

    expect(screen.queryByText(/Monitored systems:/i)).not.toBeInTheDocument();
  });

  it('keeps urgent limit warnings visible even without migration gap', async () => {
    mockGetLimit.mockReturnValue({ key: 'max_monitored_systems', limit: 6, current: 5, state: 'warning' });

    const mod = await import('../MonitoredSystemLimitWarningBanner');
    render(() => <mod.MonitoredSystemLimitWarningBanner />);

    expect(screen.getByText('Monitored systems: 5/6')).toBeInTheDocument();
    expect(screen.getByText('Upgrade to add more')).toBeInTheDocument();
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
    render(() => <mod.MonitoredSystemLimitWarningBanner />);

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
  });
});
