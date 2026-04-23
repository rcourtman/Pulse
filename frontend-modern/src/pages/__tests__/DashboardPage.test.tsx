import { fireEvent, render, screen } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { DashboardOverview } from '@/hooks/useDashboardOverview';
import type { DashboardRecoverySummary } from '@/hooks/useDashboardRecovery';
import DashboardPage from '@/pages/Dashboard';
import dashboardPageSource from '@/pages/Dashboard.tsx?raw';

let overviewLoading = false;
let overviewError: unknown = undefined;
let wsConnected = true;
let wsReconnecting = false;
const connectedInfrastructureMock: Array<{
  id: string;
  name: string;
  status: 'active' | 'ignored';
  healthStatus?: string;
  lastSeen?: number;
  surfaces: Array<{ id: string; kind: 'agent' | 'proxmox' | 'truenas'; label: string }>;
}> = [];
const reconnectSpy = vi.fn();
const navigateSpy = vi.hoisted(() => vi.fn());
const recoverySummaryMock: DashboardRecoverySummary = {
  totalProtected: 3,
  byOutcome: { success: 2, failed: 1 },
  latestEventTimestamp: Date.parse('2026-02-14T10:00:00.000Z'),
  hasData: true,
};

const overviewMock: DashboardOverview = {
  health: {
    totalResources: 0,
    byStatus: {},
    criticalAlerts: 0,
    warningAlerts: 0,
  },
  infrastructure: {
    total: 0,
    byStatus: {},
    byType: {},
    topCPU: [],
    topMemory: [],
  },
  workloads: {
    total: 0,
    running: 0,
    stopped: 0,
    byType: {},
  },
  storage: {
    total: 0,
    totalCapacity: 0,
    totalUsed: 0,
    warningCount: 0,
    criticalCount: 0,
  },
  alerts: {
    activeCritical: 0,
    activeWarning: 0,
    total: 0,
  },
  problemResources: [],
};

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    state: {
      resources: [],
      get connectedInfrastructure() {
        return connectedInfrastructureMock;
      },
    },
    activeAlerts: {},
    connected: () => wsConnected,
    reconnecting: () => wsReconnecting,
    reconnect: reconnectSpy,
    initialDataReceived: () => true,
  }),
}));

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/hooks/useDashboardOverview', () => ({
  useDashboardOverview: () => ({
    overview: () => overviewMock,
    loading: () => overviewLoading,
    error: () => overviewError,
    refetch: vi.fn(),
  }),
}));

vi.mock('@/hooks/useDashboardTrends', () => ({
  useDashboardTrends: () => () => ({
    infrastructure: {
      cpu: new Map(),
      memory: new Map(),
      emptyMessage: null,
    },
    storage: {
      capacity: null,
    },
    loading: false,
    error: null,
  }),
}));

vi.mock('@/hooks/useDashboardActions', () => ({
  useDashboardActions: () => ({
    pendingApprovals: () => [],
    unackedCriticalAlerts: () => [],
    findingsNeedingAttention: () => [],
    hasAnyActions: () => false,
    totalActionCount: () => 0,
  }),
}));

vi.mock('@/hooks/useDashboardRecovery', () => ({
  useDashboardRecovery: () => () => recoverySummaryMock,
}));

describe('Dashboard page module contract', () => {
  beforeEach(() => {
    overviewLoading = false;
    overviewError = undefined;
    wsConnected = true;
    wsReconnecting = false;
    reconnectSpy.mockReset();
    navigateSpy.mockReset();
    overviewMock.health.totalResources = 0;
    overviewMock.infrastructure.total = 0;
    overviewMock.infrastructure.byStatus = {};
    overviewMock.workloads.total = 0;
    overviewMock.workloads.running = 0;
    overviewMock.workloads.stopped = 0;
    overviewMock.problemResources = [];
    overviewMock.storage.total = 0;
    overviewMock.storage.totalCapacity = 0;
    overviewMock.storage.totalUsed = 0;
    overviewMock.storage.warningCount = 0;
    overviewMock.storage.criticalCount = 0;
    overviewMock.alerts.activeCritical = 0;
    overviewMock.alerts.activeWarning = 0;
    overviewMock.alerts.total = 0;
    connectedInfrastructureMock.length = 0;
  });

  it('exports a default component function', () => {
    expect(typeof DashboardPage).toBe('function');
  });

  it('routes the alerts dashboard widget through the alert-owned surface', () => {
    expect(dashboardPageSource).toContain("from '@/components/Alerts/RecentAlertsPanel'");
    expect(dashboardPageSource).toContain('return <RecentAlertsPanel alerts={alertsList()} />;');
    expect(dashboardPageSource).not.toContain('criticalCount={overview().alerts.activeCritical}');
    expect(dashboardPageSource).not.toContain('warningCount={overview().alerts.activeWarning}');
  });

  it('routes dashboard overview panels through the dashboard overview feature owner', () => {
    expect(dashboardPageSource).toContain("from '@/features/dashboardOverview'");
    expect(dashboardPageSource).not.toContain("from '@/components/Dashboard/RelayOnboardingCard'");
    expect(dashboardPageSource).not.toContain('<RelayOnboardingCard />');
    expect(dashboardPageSource).toContain(
      'ActionRequiredPanel,\n  DashboardCustomizer,\n  EstateSummaryPanel,\n  KPIStrip,\n  ProblemResourcesTable,\n  TrendCharts,',
    );
  });

  it('routes storage and recovery dashboard widgets through storage-recovery owners', () => {
    expect(dashboardPageSource).toContain(
      "from '@/components/Recovery/DashboardRecoveryStatusPanel'",
    );
    expect(dashboardPageSource).toContain("from '@/components/Storage/DashboardStoragePanel'");
    expect(dashboardPageSource).toContain(
      'const overviewState = useDashboardOverview(alertsList);',
    );
    expect(dashboardPageSource).not.toContain("cacheKey: 'all-resources'");
  });

  it('routes dashboard trend hydration through the shared dashboard resources snapshot', () => {
    expect(dashboardPageSource).toContain(
      'const trends = useDashboardTrends(overview, trendRange);',
    );
    expect(dashboardPageSource).not.toContain('const dashboardResources = useUnifiedResources');
  });

  it('renders loading skeleton blocks when resources are loading', () => {
    overviewLoading = true;

    render(() => <DashboardPage />);

    expect(screen.getByTestId('dashboard-loading')).toBeInTheDocument();
    expect(screen.getAllByTestId('dashboard-skeleton-block').length).toBeGreaterThan(0);
  });

  it('routes the empty dashboard state to infrastructure install', () => {
    render(() => <DashboardPage />);

    expect(
      screen.getByRole('heading', { name: 'Connect your first infrastructure source' }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'The dashboard appears after Pulse receives its first monitored system. Add a Pulse Agent or platform API source from Infrastructure setup, then this page becomes the live estate overview.',
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Add infrastructure source' }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=agent');
  });

  it('renders the governed storage and recovery dashboard panels', () => {
    overviewMock.health.totalResources = 4;
    overviewMock.infrastructure.total = 4;
    overviewMock.infrastructure.byStatus = { online: 4 };
    overviewMock.storage.total = 4;
    overviewMock.storage.totalCapacity = 4000;
    overviewMock.storage.totalUsed = 2000;
    overviewMock.storage.warningCount = 1;
    overviewMock.storage.criticalCount = 1;

    render(() => <DashboardPage />);

    expect(screen.queryByTestId('relay-onboarding-card')).toBeNull();
    expect(screen.getByRole('heading', { name: 'Recovery Status' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: /Storage/ })).toBeInTheDocument();
    expect(screen.getByText('Last recovery point over 24 hours ago')).toBeInTheDocument();
    expect(screen.getAllByText(/1\.95 KB \/ 3\.91 KB/i)).toHaveLength(2);
  });

  it('renders estate orientation before detailed dashboard problem rows', () => {
    overviewMock.health.totalResources = 5;
    overviewMock.infrastructure.total = 4;
    overviewMock.infrastructure.byStatus = { online: 4 };
    overviewMock.workloads.total = 9;
    overviewMock.workloads.running = 7;
    connectedInfrastructureMock.push(
      {
        id: 'homelab',
        name: 'homelab',
        status: 'active',
        healthStatus: 'online',
        lastSeen: Date.now(),
        surfaces: [
          { id: 'agent:homelab', kind: 'agent', label: 'Host telemetry' },
          { id: 'proxmox:homelab', kind: 'proxmox', label: 'Proxmox VE data' },
        ],
      },
      {
        id: 'nas',
        name: 'nas',
        status: 'active',
        healthStatus: 'degraded',
        lastSeen: Date.now(),
        surfaces: [{ id: 'truenas:nas', kind: 'truenas', label: 'TrueNAS API data' }],
      },
    );
    overviewMock.problemResources = [
      {
        resource: {
          id: 'storage-1',
          type: 'storage',
          name: 'Storage 1',
          displayName: 'Storage 1',
          platformId: 'storage-1',
          platformType: 'truenas',
          sourceType: 'api',
          status: 'offline',
        } as DashboardOverview['problemResources'][number]['resource'],
        problems: ['Offline'],
        worstValue: 200,
      },
    ];

    render(() => <DashboardPage />);

    const estateHeading = screen.getByRole('heading', { name: 'Connected infrastructure' });
    const problemHeading = screen.getByRole('heading', { name: 'Problem Resources' });

    expect(screen.getByText('1 system needs attention')).toBeInTheDocument();
    expect(screen.getAllByText(/1 resource issue below/).length).toBeGreaterThan(0);
    expect(screen.getByText('1 system needs review; details below')).toBeInTheDocument();
    expect(screen.getByText('2 active')).toBeInTheDocument();
    expect(screen.getByText(/Proxmox/)).toBeInTheDocument();
    expect(screen.getByText(/TrueNAS/)).toBeInTheDocument();
    expect(
      estateHeading.compareDocumentPosition(problemHeading) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });

  it('keeps healthy system copy distinct from resource and alert issues', () => {
    overviewMock.health.totalResources = 5;
    overviewMock.infrastructure.total = 1;
    overviewMock.infrastructure.byStatus = { online: 1 };
    overviewMock.alerts.activeWarning = 2;
    overviewMock.alerts.total = 2;
    connectedInfrastructureMock.push({
      id: 'homelab',
      name: 'homelab',
      status: 'active',
      healthStatus: 'online',
      lastSeen: Date.now(),
      surfaces: [{ id: 'agent:homelab', kind: 'agent', label: 'Host telemetry' }],
    });
    overviewMock.problemResources = [
      {
        resource: {
          id: 'container-1',
          type: 'app-container',
          name: 'Container 1',
          displayName: 'Container 1',
          platformId: 'container-1',
          platformType: 'docker',
          sourceType: 'api',
          status: 'offline',
          lastSeen: Date.now(),
        } as DashboardOverview['problemResources'][number]['resource'],
        problems: ['Offline'],
        worstValue: 200,
      },
    ];

    render(() => <DashboardPage />);

    expect(screen.getByText('1 system reporting')).toBeInTheDocument();
    expect(screen.getAllByText(/1 resource issue and 2 alerts below/).length).toBeGreaterThan(0);
    expect(screen.getByText('Resource issues and alerts listed below')).toBeInTheDocument();
    expect(screen.queryByText('No dashboard issues found')).toBeNull();
  });

  it('keeps the KPI strip above problem resources so the dashboard snapshot reads before detail', () => {
    overviewMock.health.totalResources = 5;
    overviewMock.infrastructure.total = 5;
    overviewMock.infrastructure.byStatus = { online: 5 };
    overviewMock.workloads.total = 12;
    overviewMock.workloads.running = 9;
    overviewMock.problemResources = [
      {
        resource: {
          id: 'storage-1',
          type: 'storage',
          name: 'Storage 1',
          displayName: 'Storage 1',
          platformId: 'storage-1',
          platformType: 'truenas',
          sourceType: 'api',
          status: 'offline',
        } as DashboardOverview['problemResources'][number]['resource'],
        problems: ['Offline'],
        worstValue: 200,
      },
    ];

    render(() => <DashboardPage />);

    const kpiLabel = screen.getByText('Infrastructure');
    const problemHeading = screen.getByRole('heading', { name: 'Problem Resources' });

    expect(
      kpiLabel.compareDocumentPosition(problemHeading) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });
});
