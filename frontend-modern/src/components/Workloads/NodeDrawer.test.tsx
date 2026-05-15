import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Node } from '@/types/api';

const chartsApiMocks = vi.hoisted(() => ({
  getMetricsHistory: vi.fn(),
}));

vi.mock('@/api/charts', async () => {
  const actual = await vi.importActual<typeof import('@/api/charts')>('@/api/charts');
  return {
    ...actual,
    ChartsAPI: {
      getMetricsHistory: chartsApiMocks.getMetricsHistory,
    },
  };
});

vi.mock('@/stores/license', () => ({
  isRangeLocked: () => false,
  loadRuntimeCapabilities: vi.fn(),
  maxHistoryDays: () => 90,
}));

import { NodeDrawer } from './NodeDrawer';

const makeHistoryPoints = (base: number) => [
  { timestamp: 1, value: base, min: base, max: base },
  { timestamp: 2, value: base + 5, min: base + 5, max: base + 5 },
  { timestamp: 3, value: base + 10, min: base + 10, max: base + 10 },
];

function makeNode(overrides: Partial<Node> = {}): Node {
  return {
    id: 'agent:pve-node-1',
    name: 'pve-node-1',
    instance: 'homelab',
    host: 'pve-node-1',
    status: 'online',
    type: 'agent',
    cpu: 0.42,
    memory: { total: 8000, used: 3200, free: 4800, usage: 40 },
    disk: { total: 10000, used: 4500, free: 5500, usage: 45 },
    networkIn: 1200,
    networkOut: 800,
    diskRead: 400,
    diskWrite: 300,
    uptime: 3600,
    loadAverage: [0.5],
    kernelVersion: '6.8.0',
    pveVersion: 'pve-manager/9.1.9',
    cpuInfo: { model: 'Ryzen', cores: 8, sockets: 1, mhz: '3200' },
    temperature: {
      cpuPackage: 62.5,
      cpuMax: 65,
      cpuMin: 40,
      cpuMaxRecord: 72,
      available: true,
      hasCPU: true,
      lastUpdate: new Date().toISOString(),
    },
    temperatureMonitoringEnabled: true,
    lastSeen: new Date().toISOString(),
    connectionHealth: 'online',
    isClusterMember: true,
    clusterName: 'homelab',
    linkedAgentId: '',
    ...overrides,
  };
}

beforeEach(() => {
  chartsApiMocks.getMetricsHistory.mockResolvedValue({
    resourceType: 'agent',
    resourceId: 'pve-node-1',
    range: '24h',
    start: 1,
    end: 3,
    metrics: {
      cpu: makeHistoryPoints(10),
      memory: makeHistoryPoints(20),
      disk: makeHistoryPoints(30),
      netin: makeHistoryPoints(1000),
      netout: makeHistoryPoints(2000),
      diskread: makeHistoryPoints(3000),
      diskwrite: makeHistoryPoints(4000),
    },
    source: 'store',
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('NodeDrawer', () => {
  it('shows a guest-drawer style Proxmox node overview with detailed node context', () => {
    render(() => <NodeDrawer node={makeNode()} />);

    expect(screen.getByText('Overview')).toBeInTheDocument();
    expect(screen.getByText('History')).toBeInTheDocument();
    expect(screen.getByText('System')).toBeInTheDocument();
    expect(screen.getByText('Platform')).toBeInTheDocument();
    expect(screen.getByText('Hardware')).toBeInTheDocument();
    expect(screen.getByText('Telemetry')).toBeInTheDocument();
    expect(screen.getByText('Ryzen')).toBeInTheDocument();
    expect(screen.getByText('6.8.0')).toBeInTheDocument();
    expect(screen.getByText('8')).toBeInTheDocument();
    expect(screen.getByText('PVE 9.1.9')).toBeInTheDocument();
    expect(screen.getByText('65°C')).toBeInTheDocument();
    expect(screen.getByText('Temp monitor')).toBeInTheDocument();
  });

  it('renders node-only thermal history without requiring a table temperature column', async () => {
    render(() => <NodeDrawer node={makeNode()} />);

    await fireEvent.click(screen.getByText('History'));

    await waitFor(() => {
      expect(chartsApiMocks.getMetricsHistory).toHaveBeenCalledWith(
        expect.objectContaining({
          resourceType: 'agent',
          resourceId: 'pve-node-1',
          range: '24h',
        }),
      );
    });

    const charts = screen.getAllByTestId('guest-history-group-chart');
    expect(charts).toHaveLength(4);
    expect(charts.map((chart) => chart.dataset.historyGroup)).toEqual([
      'utilization',
      'network',
      'disk-io',
      'thermals',
    ]);

    const thermalChart = charts[3];
    expect(within(thermalChart).getByText('Thermals')).toBeInTheDocument();
    expect(thermalChart).toHaveTextContent('65°C');
    expect(screen.getByTestId('guest-history-range-control')).toBeInTheDocument();
  });
});
