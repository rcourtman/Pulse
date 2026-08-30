import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Alert, Node } from '@/types/api';
import { resetAIRuntimeState, syncAIRuntimeSettings } from '@/stores/aiRuntimeState';

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

const openTechnicalDetails = () => {
  return within(screen.getByTestId('node-technical-details'));
};

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

function makeAlert(overrides: Partial<Alert> = {}): Alert {
  return {
    id: 'alert-memory',
    type: 'memory',
    level: 'warning',
    resourceId: 'agent:pve-node-1',
    resourceName: 'pve-node-1',
    node: 'pve-node-1',
    instance: 'homelab',
    message: 'Node memory at 90.1%',
    value: 90.1,
    threshold: 90,
    startTime: new Date().toISOString(),
    acknowledged: false,
    ...overrides,
  };
}

beforeEach(() => {
  resetAIRuntimeState();
  syncAIRuntimeSettings({ discovery_enabled: true } as Parameters<typeof syncAIRuntimeSettings>[0]);
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
  resetAIRuntimeState();
  vi.clearAllMocks();
});

describe('NodeDrawer', () => {
  it('collapses from the full shared drawer header surface', async () => {
    const onClose = vi.fn();
    render(() => <NodeDrawer node={makeNode()} onClose={onClose} />);

    await fireEvent.click(screen.getByRole('button', { name: 'Collapse pve-node-1 details' }));

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('does not expose Discovery when the feature is disabled', () => {
    syncAIRuntimeSettings({ discovery_enabled: false } as Parameters<
      typeof syncAIRuntimeSettings
    >[0]);

    render(() => (
      <NodeDrawer
        node={makeNode()}
        discoveryTarget={{ agentId: 'pve-node-1', hostname: 'pve-node-1' }}
      />
    ));

    expect(screen.queryByRole('tab', { name: 'Discovery' })).toBeNull();
  });

  it('shows a guest-drawer style Proxmox node overview with detailed node context', () => {
    render(() => (
      <NodeDrawer
        node={makeNode({
          networkInterfaces: [
            { name: 'eno1', addresses: [] },
            {
              name: 'vmbr0',
              addresses: ['192.168.10.21/24', 'fd42:7065:6c73::21/64'],
            },
          ],
        })}
      />
    ));
    const technical = openTechnicalDetails();

    expect(screen.getByText('Overview')).toBeInTheDocument();
    expect(screen.getByText('History')).toBeInTheDocument();
    expect(screen.getByText('Manage')).toBeInTheDocument();
    expect(technical.queryByText('System')).toBeNull();
    expect(technical.getAllByText('Platform').length).toBeGreaterThan(0);
    expect(technical.getByText('Hardware')).toBeInTheDocument();
    expect(technical.getByText('Telemetry')).toBeInTheDocument();
    expect(technical.getByText('Network')).toBeInTheDocument();
    expect(technical.getByText('eno1')).toBeInTheDocument();
    expect(technical.getByText('vmbr0')).toBeInTheDocument();
    expect(technical.getByText('192.168.10.21/24 / fd42:7065:6c73::21/64')).toBeInTheDocument();
    expect(technical.getByText('Ryzen')).toBeInTheDocument();
    expect(technical.getByText('6.8.0')).toBeInTheDocument();
    expect(technical.getByText('8')).toBeInTheDocument();
    expect(screen.getAllByText('PVE 9.1.9').length).toBeGreaterThan(0);
    expect(technical.getByText('CPU low')).toBeInTheDocument();
    expect(technical.getByText('CPU record')).toBeInTheDocument();
    expect(technical.getByText('Temp monitor')).toBeInTheDocument();
  });

  it('colors an operator-significant thermal row from configured thresholds', () => {
    render(() => (
      <NodeDrawer
        node={makeNode({
          temperature: {
            cpuPackage: 86,
            cpuMax: 86,
            cpuMin: 50,
            cpuMaxRecord: 86,
            available: true,
            hasCPU: true,
            lastUpdate: new Date().toISOString(),
          },
        })}
        temperatureThresholds={{ warning: 80, critical: 85 }}
      />
    ));

    expect(openTechnicalDetails().getAllByText('86°C')[0].closest('td')).toHaveClass(
      'text-rose-700',
    );
  });

  it('shows typed NVIDIA telemetry reported by the linked PVE agent', () => {
    render(() => (
      <NodeDrawer
        node={makeNode({
          linkedAgentId: 'pve-agent',
          sensors: {
            temperatureCelsius: { gpu_nvidia_0: 63 },
            gpu: [
              {
                id: '0',
                name: 'NVIDIA RTX A6000',
                temperatureCelsius: 63,
                utilizationPercent: 42,
                memoryUsedBytes: 8 * 1024 * 1024 * 1024,
                memoryTotalBytes: 48 * 1024 * 1024 * 1024,
              },
            ],
          },
        })}
      />
    ));

    const technical = openTechnicalDetails();
    expect(technical.getByText('GPU')).toBeInTheDocument();
    expect(technical.getByText('GPU 0')).toBeInTheDocument();
    expect(
      technical.getByText('NVIDIA RTX A6000 · 63°C · 42% · 8.00 GB / 48.0 GB'),
    ).toBeInTheDocument();
  });

  it('puts the active alert problem before context and visible inventory', () => {
    render(() => (
      <NodeDrawer
        node={makeNode()}
        alerts={[
          {
            id: 'alert-disk',
            type: 'disk',
            level: 'warning',
            resourceId: 'agent:pve-node-1',
            resourceName: 'pve-node-1',
            node: 'pve-node-1',
            instance: 'homelab',
            message: 'Root disk usage is above 85%',
            value: 88,
            threshold: 85,
            startTime: new Date().toISOString(),
            acknowledged: false,
          },
        ]}
      />
    ));

    expect(screen.getByText('Needs attention')).toBeInTheDocument();
    expect(screen.getByText('Root disk usage is above 85%')).toBeInTheDocument();
    expect(screen.getAllByText('PVE 9.1.9').length).toBeGreaterThan(0);
    expect(screen.getByTestId('node-technical-details').querySelector('table')).toBeTruthy();
  });

  it('identifies child resources and reveals every active alert in place', async () => {
    render(() => (
      <NodeDrawer
        node={makeNode()}
        alerts={[
          makeAlert(),
          makeAlert({
            id: 'alert-vm-memory',
            resourceId: 'homelab-pve-node-1-100',
            resourceName: 'media-vm',
            message: 'VM memory at 90.2%',
            value: 90.2,
          }),
          makeAlert({
            id: 'alert-vm-disk',
            type: 'disk',
            resourceId: 'homelab-pve-node-1-100-c',
            resourceName: 'media-vm (C:)',
            message: 'VM disk at 90.2%',
            value: 90.2,
          }),
          makeAlert({
            id: 'alert-backup',
            type: 'backup-age',
            level: 'info',
            resourceId: 'homelab-pve-node-1-101',
            resourceName: 'archive-vm',
            message: 'Latest backup is 8 days old',
            value: 8,
            threshold: 7,
          }),
        ]}
      />
    ));

    const attention = within(screen.getByTestId('drawer-attention-section'));
    expect(attention.getByText('4 active')).toBeInTheDocument();
    expect(attention.getByText('pve-node-1')).toBeInTheDocument();
    expect(attention.getByText('media-vm')).toBeInTheDocument();
    expect(attention.getByText('media-vm (C:)')).toBeInTheDocument();
    expect(attention.queryByText('archive-vm')).not.toBeInTheDocument();

    const revealButton = attention.getByRole('button', { name: 'Show 1 more alert' });
    expect(revealButton).toHaveAttribute('aria-expanded', 'false');
    await fireEvent.click(revealButton);

    expect(attention.getByText('archive-vm')).toBeInTheDocument();
    expect(attention.getByText('Backup Age')).toBeInTheDocument();
    expect(attention.getByText('Info')).toBeInTheDocument();
    expect(attention.getByRole('button', { name: 'Show fewer alerts' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
  });

  it('renders API-only node history without unavailable disk throughput', async () => {
    render(() => (
      <NodeDrawer
        node={makeNode({
          id: 'homelab-pve-node-1',
          type: 'node',
          linkedAgentId: '',
        })}
      />
    ));

    await fireEvent.click(screen.getByText('History'));

    await waitFor(() => {
      expect(chartsApiMocks.getMetricsHistory).toHaveBeenCalledWith(
        expect.objectContaining({
          resourceType: 'node',
          resourceId: 'homelab-pve-node-1',
          range: '24h',
        }),
      );
    });

    const charts = screen.getAllByTestId('guest-history-group-chart');
    expect(charts).toHaveLength(3);
    expect(charts.map((chart) => chart.dataset.historyGroup)).toEqual([
      'utilization',
      'network',
      'thermals',
    ]);

    const thermalChart = charts[2];
    expect(within(thermalChart).getByText('Thermals')).toBeInTheDocument();
    expect(thermalChart).toHaveTextContent('65°C');
    expect(screen.getByTestId('guest-history-range-control')).toBeInTheDocument();
  });

  it('keeps agent history and disk throughput when a Unified Agent is linked', async () => {
    render(() => <NodeDrawer node={makeNode({ linkedAgentId: 'agent:pve-node-1' })} />);

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
    expect(charts.map((chart) => chart.dataset.historyGroup)).toEqual([
      'utilization',
      'network',
      'disk-io',
      'thermals',
    ]);
  });
});
