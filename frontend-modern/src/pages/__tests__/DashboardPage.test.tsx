import { render, screen } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import type { DashboardOverview } from '@/hooks/useDashboardOverview';
import DashboardPage from '@/pages/Dashboard';

let wsInitialDataReceived = true;
let wsConnected = true;
let wsReconnecting = false;
let wsResources: Resource[] = [];
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
    state: { resources: wsResources },
    activeAlerts: {},
    connected: () => wsConnected,
    reconnecting: () => wsReconnecting,
    reconnect: reconnectSpy,
    initialDataReceived: () => wsInitialDataReceived,
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

describe('Dashboard page module contract', () => {
  beforeEach(() => {
    wsInitialDataReceived = true;
    wsConnected = true;
    wsReconnecting = false;
    wsResources = [];
    reconnectSpy.mockReset();
  });

  it('exports a default component function', () => {
    expect(typeof DashboardPage).toBe('function');
  });

  it('renders loading skeleton blocks before initial data is received', () => {
    wsInitialDataReceived = false;

    render(() => <DashboardPage />);

    expect(screen.getByTestId('dashboard-loading')).toBeInTheDocument();
    expect(screen.getAllByTestId('dashboard-skeleton-block').length).toBeGreaterThan(0);
  });
});
