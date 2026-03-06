import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { STORAGE_KEYS } from '@/utils/localStorage';

const mockEntitlements = vi.hoisted(() => vi.fn(() => ({ overflow_days_remaining: undefined })));
const mockGetLimit = vi.hoisted(() => vi.fn(() => undefined));
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
  getUpgradeActionUrlOrFallback: vi.fn(() => '/pricing?feature=max_agents'),
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

describe('AgentLimitWarningBanner', () => {
  beforeEach(() => {
    localStorage.clear();
    mockEntitlements.mockReturnValue({ overflow_days_remaining: undefined });
    mockGetLimit.mockReturnValue({ key: 'max_agents', limit: 6, current: 0, state: 'ok' });
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
    const mod = await import('../AgentLimitWarningBanner');
    render(() => <mod.AgentLimitWarningBanner />);

    expect(screen.queryByText(/Host Agents:/i)).not.toBeInTheDocument();
  });

  it('shows migration guidance when legacy connections exist', async () => {
    mockHasMigrationGap.mockReturnValue(true);
    mockLegacyConnections.mockReturnValue({
      proxmox_nodes: 2,
      docker_hosts: 1,
      kubernetes_clusters: 0,
    });

    const mod = await import('../AgentLimitWarningBanner');
    render(() => <mod.AgentLimitWarningBanner />);

    expect(screen.getByText('Host Agents: 0/6')).toBeInTheDocument();
    expect(
      screen.getByText(
        /You also have 3 resources connected via API or legacy agents \(2 Proxmox nodes, 1 Docker host\) that do not count toward Host Agents\./i,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('Install v6 agents')).toHaveAttribute('href', '/settings/infrastructure');
    expect(screen.getByLabelText('Dismiss agent migration notice')).toBeInTheDocument();
  });

  it('keeps urgent limit warnings visible even without migration gap', async () => {
    mockGetLimit.mockReturnValue({ key: 'max_agents', limit: 6, current: 5, state: 'warning' });

    const mod = await import('../AgentLimitWarningBanner');
    render(() => <mod.AgentLimitWarningBanner />);

    expect(screen.getByText('Host Agents: 5/6')).toBeInTheDocument();
    expect(screen.getByText('Upgrade to add more')).toBeInTheDocument();
    expect(screen.queryByText('Install v6 agents')).not.toBeInTheDocument();
  });

  it('persists dismissal for migration-only notices', async () => {
    mockHasMigrationGap.mockReturnValue(true);
    mockLegacyConnections.mockReturnValue({
      proxmox_nodes: 1,
      docker_hosts: 0,
      kubernetes_clusters: 0,
    });

    const mod = await import('../AgentLimitWarningBanner');
    render(() => <mod.AgentLimitWarningBanner />);

    fireEvent.click(screen.getByLabelText('Dismiss agent migration notice'));

    expect(screen.queryByText(/Host Agents:/i)).not.toBeInTheDocument();
    const orgId = sessionStorage.getItem(STORAGE_KEYS.ORG_ID) ?? 'default';
    expect(localStorage.getItem(`${STORAGE_KEYS.AGENT_MIGRATION_NOTICE_DISMISSED}:${orgId}`)).toBe(
      'true',
    );
  });
});
