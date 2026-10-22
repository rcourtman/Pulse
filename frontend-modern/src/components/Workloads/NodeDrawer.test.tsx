import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Alert, Node } from '@/types/api';
import { resetAIRuntimeState } from '@/stores/aiRuntimeState';

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
// Overview loads its charts lazily; warm the module so a cold transform does
// not outlast the tests' waits.
import './GuestDrawerHistory';

// Details is closed by default; a warning row is lifted out of it into
// Worth checking, so read the whole Overview with Details open.
const openTechnicalDetails = () => {
  const details = screen.getByTestId('node-technical-details') as HTMLDetailsElement;
  details.open = true;
  return within(screen.getByTestId('node-drawer-overview'));
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
    cpuTemperature: 65,
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

  it('opens the node web interface from the header, like the guest drawer', () => {
    const onClose = vi.fn();
    render(() => (
      <NodeDrawer
        node={makeNode()}
        onClose={onClose}
        webInterfaceUrl="https://pve-node-1.example:8006"
      />
    ));

    const open = screen.getByRole('link', { name: 'Open web interface for pve-node-1' });
    expect(open).toHaveAttribute('href', 'https://pve-node-1.example:8006');
    expect(open).toHaveTextContent('Open');
    fireEvent.click(open);
    expect(onClose).not.toHaveBeenCalled();
  });

  it('shows no Open action when the node has no web interface', () => {
    render(() => <NodeDrawer node={makeNode()} onClose={vi.fn()} webInterfaceUrl=" " />);
    expect(screen.queryByRole('link', { name: /Open web interface/ })).toBeNull();
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
    expect(screen.queryByRole('tab', { name: 'History' })).toBeNull();
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

    // GPU readings stay visible in Sensors rather than inside closed Details.
    const sensors = within(screen.getByTestId('node-drawer-sensors'));
    expect(sensors.getByText('Sensors')).toBeInTheDocument();
    expect(sensors.getByText('GPU 0')).toBeInTheDocument();
    expect(
      sensors.getByText('NVIDIA RTX A6000 · 63°C · 42% · 8.00 GB / 48.0 GB'),
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

  it('uses canonical history coordinates instead of the display resource identity', async () => {
    render(() => (
      <NodeDrawer
        node={makeNode({
          id: 'agent-canonical-id',
          linkedAgentId: '',
          metricsTarget: { resourceType: 'node', resourceId: 'cluster-pve-native' },
        })}
      />
    ));
    await waitFor(() =>
      expect(chartsApiMocks.getMetricsHistory).toHaveBeenCalledWith(
        expect.objectContaining({
          resourceType: 'node',
          resourceId: 'cluster-pve-native',
          range: '24h',
        }),
      ),
    );
    expect(
      screen.getAllByTestId('guest-history-group-chart').map((chart) => chart.dataset.historyGroup),
    ).toEqual(['utilization', 'network', 'thermals']);
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

const EMPTY_HISTORY = {
  resourceType: 'agent',
  resourceId: 'pve-node-1',
  range: '24h',
  start: 1,
  end: 3,
  metrics: {},
  source: 'store',
};

const utilizationChart = async () =>
  (await screen.findAllByTestId('guest-history-group-chart')).find(
    (chart) => chart.dataset.historyGroup === 'utilization',
  )!;

describe('NodeDrawer Overview charts', () => {
  it('lists Worth checking as one line under the alerts line, above the charts', async () => {
    render(() => (
      <NodeDrawer
        node={makeNode({
          pendingUpdatesStatus: 'unavailable',
          pendingUpdatesReason: 'source_unavailable',
        })}
      />
    ));
    const chart = await utilizationChart();
    const worth = screen.getByTestId('node-drawer-exceptions');
    expect(
      worth.compareDocumentPosition(chart) & window.Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    const row = within(worth).getByText('Updates').closest('tr')!;
    expect(row).toHaveTextContent('Unavailable · Proxmox source unavailable');
    expect(
      within(row)
        .getByText(/Proxmox source unavailable/)
        .closest('td'),
    ).toHaveClass('text-amber-700');
  });

  const guestAlert = makeAlert({
    id: 'guest-memory',
    resourceId: 'qemu-281',
    resourceName: 'msp-eval-lab',
    message: 'VM memory at 85.4%',
    threshold: 85,
  });

  it("draws an alert line only for the node's own alert, not a guest's on the same node", async () => {
    render(() => <NodeDrawer node={makeNode()} alerts={[guestAlert]} />);
    const chart = await utilizationChart();
    // The guest's alert is still listed under Needs attention.
    expect(screen.getByText('VM memory at 85.4%')).toBeInTheDocument();
    expect(chart.querySelector('[data-history-threshold]')).toBeNull();
  });

  it("draws the node's own memory alert on the utilization chart", async () => {
    render(() => <NodeDrawer node={makeNode()} alerts={[makeAlert(), guestAlert]} />);
    const chart = await utilizationChart();
    const line = chart.querySelector('[data-history-threshold="memory"]');
    expect(line).toHaveClass('text-amber-500');
    expect(line).toHaveTextContent('Memory alert at 90.0%');
  });

  it("does not present an offline node's last readings as live", async () => {
    chartsApiMocks.getMetricsHistory.mockResolvedValue(EMPTY_HISTORY);
    render(() => <NodeDrawer node={makeNode({ status: 'offline', cpuTemperature: 65 })} />);
    await waitFor(() => expect(chartsApiMocks.getMetricsHistory).toHaveBeenCalled());
    expect(screen.getByTestId('node-drawer-readings-withheld')).toHaveTextContent(
      'Node is offline. Showing its history only.',
    );
    await screen.findAllByText('No stored history in this range');
    expect(document.querySelector('[data-history-current]')).toBeNull();
    expect(document.querySelector('[data-history-last-known]')).toBeNull();
  });

  it('keeps NVMe and GPU sensor readings visible rather than inside closed Details', () => {
    render(() => (
      <NodeDrawer
        node={makeNode({
          temperature: {
            ...makeNode().temperature!,
            nvme: [{ device: 'nvme0', temp: 71 }],
          },
        })}
      />
    ));
    const sensors = screen.getByTestId('node-drawer-sensors');
    expect(sensors.closest('details')).toBeNull();
    expect(within(sensors).getByText(/nvme0 71/)).toBeInTheDocument();
  });

  it("uses the table's own-alert list when its id differs from the node record", async () => {
    // Proxmox node alerts are keyed on the cluster-qualified id (homelab-pve-node-1),
    // not the drawer node's id, so the table supplies the node's own alerts.
    const nodeAlert = makeAlert({
      resourceId: 'homelab-pve-node-1',
      level: 'critical',
      threshold: 95,
    });
    render(() => <NodeDrawer node={makeNode()} alerts={[nodeAlert]} ownAlerts={[nodeAlert]} />);
    const chart = await utilizationChart();
    expect(chart.querySelector('[data-history-threshold="memory"]')).toHaveClass('text-red-500');
  });

  it('states only the RAM size in the facts strip for retained node memory', () => {
    render(() => (
      <NodeDrawer
        node={makeNode({
          memory: {
            total: 8000,
            used: 3200,
            free: 4800,
            usage: 40,
            observation: { state: 'last-known', source: 'node-meminfo' },
          },
        })}
      />
    ));
    const facts = screen.getByTestId('node-drawer-facts');
    expect(facts).toHaveTextContent('RAM');
    expect(facts.textContent).not.toMatch(/ of .*RAM/);
  });

  it('labels retained node memory as last known instead of live', async () => {
    chartsApiMocks.getMetricsHistory.mockResolvedValue(EMPTY_HISTORY);
    render(() => (
      <NodeDrawer
        node={makeNode({
          memory: {
            total: 8000,
            used: 3200,
            free: 4800,
            usage: 40,
            observation: { state: 'last-known', source: 'node-meminfo' },
          },
        })}
      />
    ));
    const chart = await utilizationChart();
    await within(chart).findByText('No stored history in this range');
    expect(chart.querySelector('[data-history-last-known="memory"]')).toHaveTextContent(
      'Memory40.0%last known',
    );
    expect(chart.querySelector('[data-history-current="memory"]')).toBeNull();
  });

  it('does not show a filled-in zero for a node with no disk measurement', async () => {
    chartsApiMocks.getMetricsHistory.mockResolvedValue(EMPTY_HISTORY);
    render(() => (
      <NodeDrawer node={makeNode({ disk: { total: 0, used: 0, free: 0, usage: 0 } })} />
    ));
    const chart = await utilizationChart();
    await within(chart).findByText('No stored history in this range');
    expect(chart).toHaveTextContent('Disk-');
    expect(chart).not.toHaveTextContent('Disk0.0%');
    expect(screen.getByTestId('node-drawer-facts')).not.toHaveTextContent('root disk');
  });

  it("charts the backend's primary CPU temperature, the reading its history records", async () => {
    // The details carry a hotter core than the backend's primary reading. The
    // chart must fall back to the backend value: its history is recorded under
    // the backend rule, so a recomputed reading would disagree with it.
    chartsApiMocks.getMetricsHistory.mockResolvedValue(EMPTY_HISTORY);
    render(() => (
      <NodeDrawer
        node={makeNode({
          cpuTemperature: 44,
          temperature: {
            cpuPackage: 44,
            cpuMax: 51,
            cores: [{ core: 0, temp: 51 }],
            available: true,
            hasCPU: true,
            lastUpdate: new Date().toISOString(),
          },
        })}
      />
    ));
    const charts = await screen.findAllByTestId('guest-history-group-chart');
    const thermals = charts.find((chart) => chart.dataset.historyGroup === 'thermals')!;
    await within(thermals).findByText('No stored history in this range');
    const reading = thermals.querySelector('[data-history-current="temperature"]');
    expect(reading).toHaveTextContent('44°C');
    expect(reading?.textContent).not.toContain('51');
  });

  it('shows live readings on a healthy node before any history is stored', async () => {
    chartsApiMocks.getMetricsHistory.mockResolvedValue(EMPTY_HISTORY);
    render(() => (
      <NodeDrawer
        node={makeNode({
          cpu: 37,
          memory: { total: 8000, used: 4000, free: 4000, usage: 50 },
          disk: { total: 1000, used: 250, free: 750, usage: 25 },
        })}
      />
    ));
    const chart = await utilizationChart();
    await within(chart).findByText('No stored history in this range');
    expect(chart.querySelector('[data-history-current="cpu"]')).toHaveTextContent(
      'CPU37.0%current',
    );
    expect(chart.querySelector('[data-history-current="disk"]')).toHaveTextContent(
      'Disk25.0%current',
    );
    const facts = screen.getByTestId('node-drawer-facts');
    expect(facts).toHaveTextContent(/of 7\.81 KB RAM/);
    expect(facts).toHaveTextContent(/250 B of 1000 B root disk/);
  });

  it("states only an offline node's capacity, not a used amount that reads as live", async () => {
    render(() => <NodeDrawer node={makeNode({ status: 'offline' })} />);
    const facts = screen.getByTestId('node-drawer-facts');
    expect(facts).toHaveTextContent('RAM');
    expect(facts.textContent).not.toMatch(/ of .*RAM/);
  });

  it('tones a reading for an alert that carries no numeric trigger, without a line', async () => {
    const nodeAlert = makeAlert({ threshold: undefined, level: 'critical' });
    render(() => <NodeDrawer node={makeNode()} alerts={[nodeAlert]} ownAlerts={[nodeAlert]} />);
    const chart = await utilizationChart();
    await waitFor(() => expect(chart.querySelector('path[data-history-series]')).not.toBeNull());
    expect(chart.querySelector('[data-history-threshold]')).toBeNull();
    const memoryReading = chart.querySelector('span[class*="text-red-700"]');
    expect(memoryReading).not.toBeNull();
  });

  it('describes an alert trigger to assistive technology, not only in an SVG title', async () => {
    render(() => <NodeDrawer node={makeNode()} alerts={[makeAlert()]} ownAlerts={[makeAlert()]} />);
    const chart = await utilizationChart();
    const plot = within(chart).getByRole('img', { name: 'Utilization history' });
    await waitFor(() =>
      expect(plot).toHaveAccessibleDescription(/Memory warning alert at 90\.0%\./),
    );
  });

  it('opens the charts on 24 hours with the range picker beside them', async () => {
    render(() => <NodeDrawer node={makeNode()} />);
    await utilizationChart();
    expect(screen.getByTestId('guest-history-range-control')).toHaveValue('24h');
    fireEvent.change(screen.getByTestId('guest-history-range-control'), {
      target: { value: '7d' },
    });
    await waitFor(() =>
      expect(chartsApiMocks.getMetricsHistory.mock.calls.map(([params]) => params.range)).toEqual([
        '24h',
        '7d',
      ]),
    );
  });

  it('reads history once while Overview shows and stops reading behind Manage', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      render(() => <NodeDrawer node={makeNode()} />);
      await utilizationChart();
      await waitFor(() => expect(chartsApiMocks.getMetricsHistory).toHaveBeenCalledTimes(1));
      expect(chartsApiMocks.getMetricsHistory.mock.calls[0][0]).toMatchObject({ range: '24h' });
      fireEvent.click(screen.getByRole('tab', { name: 'Manage' }));
      await vi.advanceTimersByTimeAsync(120_000);
      expect(chartsApiMocks.getMetricsHistory).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });
});
