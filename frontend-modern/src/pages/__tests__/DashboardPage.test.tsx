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
    state: { resources: [] },
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
    overviewMock.storage.total = 0;
    overviewMock.storage.totalCapacity = 0;
    overviewMock.storage.totalUsed = 0;
    overviewMock.storage.warningCount = 0;
    overviewMock.storage.criticalCount = 0;
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
    expect(dashboardPageSource).toContain(
      'ActionRequiredPanel,\n  DashboardCustomizer,\n  KPIStrip,\n  ProblemResourcesTable,\n  TrendCharts,',
    );
  });

  it('routes storage and recovery dashboard widgets through storage-recovery owners', () => {
    expect(dashboardPageSource).toContain(
      "from '@/components/Recovery/DashboardRecoveryStatusPanel'",
    );
    expect(dashboardPageSource).toContain("from '@/components/Storage/DashboardStoragePanel'");
    expect(dashboardPageSource).toContain('const overviewState = useDashboardOverview(alertsList);');
    expect(dashboardPageSource).not.toContain("cacheKey: 'all-resources'");
  });

  it('routes dashboard trend hydration through the shared dashboard resources snapshot', () => {
    expect(dashboardPageSource).toContain('const trends = useDashboardTrends(overview, trendRange);');
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

    expect(screen.getByRole('heading', { name: 'No resources yet' })).toBeInTheDocument();
    expect(
      screen.getByText(
        'Start by opening Settings → Infrastructure → Install on a host and connecting the first system you want Pulse to monitor. Your dashboard overview will appear here once that system starts reporting.',
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Open infrastructure install' }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/install');
  });

  it('renders the governed storage and recovery dashboard panels', () => {
    overviewMock.health.totalResources = 4;
    overviewMock.storage.total = 4;
    overviewMock.storage.totalCapacity = 4000;
    overviewMock.storage.totalUsed = 2000;
    overviewMock.storage.warningCount = 1;
    overviewMock.storage.criticalCount = 1;

    render(() => <DashboardPage />);

    expect(screen.getByRole('heading', { name: 'Recovery Status' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: /Storage/ })).toBeInTheDocument();
    expect(screen.getByText('Last recovery point over 24 hours ago')).toBeInTheDocument();
    expect(screen.getAllByText(/1\.95 KB \/ 3\.91 KB/i)).toHaveLength(2);
  });
});
