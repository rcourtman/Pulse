import { render, screen } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { DashboardOverview } from '@/hooks/useDashboardOverview';
import type { DashboardRecoverySummary } from '@/hooks/useDashboardRecovery';
import DashboardPage from '@/pages/Dashboard';

let unifiedLoading = false;
let unifiedResources: any[] = [];
let unifiedError: unknown = undefined;
let wsConnected = true;
let wsReconnecting = false;
const reconnectSpy = vi.fn();
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

vi.mock('@/App', () => ({
  useWebSocket: () => ({
    state: { resources: [] },
    activeAlerts: {},
    connected: () => wsConnected,
    reconnecting: () => wsReconnecting,
    reconnect: reconnectSpy,
    initialDataReceived: () => true,
  }),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: () => ({
    resources: () => unifiedResources,
    loading: () => unifiedLoading,
    error: () => unifiedError,
    refetch: vi.fn(),
    mutate: vi.fn(),
  }),
}));

vi.mock('@/hooks/useDashboardOverview', () => ({
  useDashboardOverview: () => () => overviewMock,
}));

vi.mock('@/hooks/useDashboardTrends', () => ({
  useDashboardTrends: () => () => ({
    infrastructure: {
      cpu: new Map(),
      memory: new Map(),
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
    unifiedLoading = false;
    unifiedResources = [];
    unifiedError = undefined;
    wsConnected = true;
    wsReconnecting = false;
    reconnectSpy.mockReset();
    overviewMock.storage.total = 0;
    overviewMock.storage.totalCapacity = 0;
    overviewMock.storage.totalUsed = 0;
    overviewMock.storage.warningCount = 0;
    overviewMock.storage.criticalCount = 0;
  });

  it('exports a default component function', () => {
    expect(typeof DashboardPage).toBe('function');
  });

  it('renders loading skeleton blocks when resources are loading', () => {
    unifiedLoading = true;

    render(() => <DashboardPage />);

    expect(screen.getByTestId('dashboard-loading')).toBeInTheDocument();
    expect(screen.getAllByTestId('dashboard-skeleton-block').length).toBeGreaterThan(0);
  });

  it('renders the governed storage and recovery dashboard panels', () => {
    unifiedResources = [{ id: 'resource-1' }];
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
