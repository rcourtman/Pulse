import { render, screen } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { DashboardOverview } from '@/hooks/useDashboardOverview';
import DashboardPage from '@/pages/Dashboard';

let unifiedLoading = false;
let unifiedResources: any[] = [];
let unifiedError: unknown = undefined;
let wsConnected = true;
let wsReconnecting = false;
const reconnectSpy = vi.fn();

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

vi.mock('@/hooks/useDashboardRecovery', () => ({
  useDashboardRecovery: () => () => ({
    totalProtected: 0,
    byOutcome: {},
    latestEventTimestamp: null,
    hasData: false,
  }),
}));

describe('Dashboard page module contract', () => {
  beforeEach(() => {
    unifiedLoading = false;
    unifiedResources = [];
    unifiedError = undefined;
    wsConnected = true;
    wsReconnecting = false;
    reconnectSpy.mockReset();
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
});
