import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { createStore } from 'solid-js/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { ProxmoxNodesTable } from '../ProxmoxNodesTable';

const nodeDrawerMock = vi.hoisted(() => vi.fn());
const activeAlertsMock = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const getMetricThresholdsMock = vi.hoisted(() => vi.fn());
const temperatureGaugeMock = vi.hoisted(() => vi.fn());
const metricHistoryMock = vi.hoisted(() => vi.fn());

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    width: () => 1280,
  }),
}));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts: activeAlertsMock.value }),
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    detectionEnabled: () => true,
    getMetricThresholds: getMetricThresholdsMock,
  }),
}));

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div data-testid="responsive-metric-cell" />,
}));

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: (props: { cacheInclusiveLabel?: string }) => (
    <div data-testid="stacked-memory-bar" data-cache-inclusive-label={props.cacheInclusiveLabel} />
  ),
}));

vi.mock('@/components/Workloads/StackedDiskBar', () => ({
  StackedDiskBar: () => <div data-testid="stacked-disk-bar" />,
}));

vi.mock('@/components/Workloads/MetricMiniSparkline', () => ({
  MetricMiniSparkline: () => <div data-testid="metric-mini-sparkline" />,
}));

vi.mock('@/components/shared/TemperatureGauge', () => ({
  TemperatureGauge: (props: {
    value: number;
    thresholds?: unknown;
    alertSeverity?: string | null;
    title?: string;
  }) => {
    temperatureGaugeMock({
      value: props.value,
      thresholds: props.thresholds,
      alertSeverity: props.alertSeverity,
      title: props.title,
    });
    return <div data-testid="temperature-gauge" title={props.title} />;
  },
}));

vi.mock('@/components/Workloads/useWorkloadTableMetricHistory', () => ({
  useWorkloadTableMetricHistory: (options: unknown) => {
    metricHistoryMock(options);
    return {
      getNodeMetricSeries: () => [],
    };
  },
}));

vi.mock('@/components/Workloads/NodeDrawer', () => ({
  NodeDrawer: (props: { node: { name: string }; temperatureThresholds?: unknown }) => {
    nodeDrawerMock(props);
    return <div data-testid="node-drawer">{props.node.name}</div>;
  },
}));

const makeNodeResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'agent:pve-node-1',
  name: 'pve-node-1',
  displayName: 'pve-node-1',
  platformId: 'homelab',
  platformType: 'proxmox-pve',
  sourceType: 'hybrid',
  status: 'online',
  type: 'agent',
  lastSeen: 1_700_000_000_000,
  cpu: { current: 42 },
  memory: { total: 8_000, used: 3_200, free: 4_800, current: 40 },
  disk: { total: 10_000, used: 4_500, free: 5_500, current: 45 },
  proxmox: {
    clusterName: 'homelab',
    nodeName: 'pve-node-1',
    pveVersion: 'pve-manager/9.1.9/ee7bad0a3d1546c9',
  },
  ...overrides,
});

const makeNodeResources = (count: number): Resource[] =>
  Array.from({ length: count }, (_, index) =>
    makeNodeResource({
      id: `agent:pve-node-${index + 1}`,
      name: `pve-node-${index + 1}`,
      displayName: `pve-node-${index + 1}`,
      proxmox: {
        clusterName: 'homelab',
        nodeName: `pve-node-${index + 1}`,
      },
    }),
  );

beforeEach(() => {
  vi.clearAllMocks();
  getMetricThresholdsMock.mockReturnValue({ warning: 80, critical: 85 });
  activeAlertsMock.value = {};
});

afterEach(() => {
  // The header sort persists per table via localStorage; clear it so one
  // test's sort choice cannot leak into the next render.
  window.localStorage.clear();
  cleanup();
  vi.useRealTimers();
});

describe('ProxmoxNodesTable', () => {
  it('uses the shared workload search to narrow the host preview and count', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={makeNodeResources(10)}
        guests={[]}
        search={() => 'pve-node-9'}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    expect(screen.getByText('Nodes').parentElement).toHaveTextContent('Nodes1of 10');
    expect(screen.getByText('pve-node-9')).toBeInTheDocument();
    expect(screen.queryByText('pve-node-1')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Show all 10 nodes' })).not.toBeInTheDocument();
  });

  it('keeps large estates bounded until the operator expands the node preview', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={makeNodeResources(14)}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    expect(screen.getAllByRole('row')).toHaveLength(13);
    const showAll = screen.getByRole('button', { name: 'Show all 14 nodes' });
    expect(showAll).toHaveAttribute('aria-expanded', 'false');
    expect(showAll.closest('[data-platform-table-preview-footer]')).toBeInTheDocument();
    expect(showAll.parentElement).toHaveTextContent('2 more below');
    expect(
      screen.getByRole('table').compareDocumentPosition(showAll) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();

    fireEvent.click(showAll);
    expect(screen.getAllByRole('row')).toHaveLength(15);
    expect(screen.getByRole('button', { name: 'Show fewer nodes' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );

    fireEvent.click(screen.getByRole('button', { name: 'Show fewer nodes' }));
    expect(screen.getAllByRole('row')).toHaveLength(13);
  });

  it('shows a threshold-sized estate without a continuation control', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={makeNodeResources(12)}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    expect(screen.getAllByRole('row')).toHaveLength(13);
    expect(screen.queryByRole('button', { name: 'Show all 12 nodes' })).not.toBeInTheDocument();
  });

  it('keeps the phone preview to six rows with its reveal control available', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={makeNodeResources(8)}
        guests={[]}
        layoutWidth={() => 390}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    expect(screen.getAllByRole('row')).toHaveLength(7);
    expect(screen.getByRole('button', { name: 'Show all 8 nodes' })).toBeVisible();
    expect(screen.getByText('2 more below')).toBeVisible();
  });

  it('names a single cluster once instead of repeating it in a column, even while searching', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={makeNodeResources(3)}
        guests={[]}
        topology={{ nodes: 3, clusters: 1, standalone: 0 }}
        search={() => 'pve-node-2'}
        inventoryCountsVisible={() => false}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    expect(screen.getByTestId('proxmox-single-cluster-label')).toHaveTextContent('Cluster homelab');
    expect(screen.queryByRole('columnheader', { name: /cluster/i })).not.toBeInTheDocument();
  });

  it('hides node and topology totals through the page-owned inventory preference', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={makeNodeResources(14)}
        guests={[]}
        topology={{ nodes: 14, clusters: 1, standalone: 0 }}
        inventoryCountsVisible={() => false}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    expect(screen.getByText('Nodes').parentElement).toHaveTextContent(/^Nodes$/);
    expect(screen.queryByText('1 cluster')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Show all nodes' })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Show all 14 nodes' })).not.toBeInTheDocument();
  });

  it('links each node to its PVE web interface without hijacking the row click', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={[
          makeNodeResource({
            proxmox: {
              clusterName: 'homelab',
              nodeName: 'pve-node-1',
              guestUrl: 'https://pve.example.com:8006',
            },
          }),
        ]}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    const link = screen.getByRole('link', { name: 'Open web interface for pve-node-1' });
    expect(link).toHaveAttribute('href', 'https://pve.example.com:8006');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveTextContent('');
    expect(screen.getByText('pve-node-1').closest('a')).toBeNull();
    expect(screen.queryByRole('columnheader', { name: 'Web' })).not.toBeInTheDocument();
    expect(screen.queryByTestId('proxmox-host-web-link')).not.toBeInTheDocument();
    expect(screen.queryByTestId('node-drawer')).not.toBeInTheDocument();
  });

  it('builds the canonical :8006 link when no URL metadata is present', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={[makeNodeResource()]}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    expect(screen.getByRole('link', { name: 'Open web interface for pve-node-1' })).toHaveAttribute(
      'href',
      'https://pve-node-1:8006',
    );
  });

  it('shows the configured display name without using it as the native web target', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={[
          makeNodeResource({
            id: 'production-pve1',
            name: 'Render East',
            displayName: 'Render East',
            proxmox: {
              clusterName: 'production',
              nodeIdentity: 'production-pve1',
              nodeName: 'pve1',
              nodeAliases: ['pve-old'],
              nodeDisplayName: 'Render East',
            },
          }),
        ]}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    const link = screen.getByRole('link', { name: 'Open web interface for Render East' });
    expect(screen.getByText('Render East')).toBeInTheDocument();
    expect(link).toHaveAttribute('href', 'https://pve1:8006');
  });

  it('keeps provider-native node identity without adding a phone subtitle', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={[
          makeNodeResource({
            id: 'production-pve1',
            name: 'West Production A',
            displayName: 'West Production A',
            proxmox: {
              clusterName: 'production',
              nodeIdentity: 'production-pve1',
              nodeName: 'pve1',
              nodeDisplayName: 'West Production A',
            },
          }),
        ]}
        guests={[]}
        layoutWidth={() => 390}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    expect(screen.getByText('West Production A')).toBeInTheDocument();
    expect(screen.getByTitle('West Production A · Proxmox node pve1')).toBeInTheDocument();
  });

  it('passes alert-backed temperature thresholds into the node temperature gauge', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={[makeNodeResource({ temperature: 76 })]}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    expect(getMetricThresholdsMock).toHaveBeenCalledWith(
      'node',
      'temperature',
      expect.arrayContaining(['agent:pve-node-1']),
    );
    expect(temperatureGaugeMock).toHaveBeenCalledWith(
      expect.objectContaining({
        value: 76,
        thresholds: { warning: 80, critical: 85 },
      }),
    );
  });

  it('keeps an open temperature alert visible on a reading that dipped under its trigger', () => {
    // minipc on 2026-10-06: the row warned "80.0°C" while the cell read a green 72°C.
    activeAlertsMock.value = {
      'cpu-alert': {
        id: 'cpu-alert',
        resourceId: 'agent:pve-node-1',
        type: 'cpu',
        level: 'critical',
        message: 'Node cpu at 97.0%',
        value: 97,
        acknowledged: false,
      },
      'temperature-alert': {
        id: 'temperature-alert',
        resourceId: 'agent:pve-node-1',
        type: 'temperature',
        level: 'warning',
        message: 'Node temperature at 80.0°C',
        value: 80,
        acknowledged: false,
        metricStatus: {
          phase: 'recovering',
          value: 72,
          unit: '°C',
          observedAt: new Date().toISOString(),
          trigger: 80,
          recovery: 75,
          recoveryDelaySeconds: 300,
        },
      },
    };

    render(() => (
      <ProxmoxNodesTable
        nodes={[makeNodeResource({ temperature: 72 })]}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    // The node's own temperature alert sets the tone; its CPU alert does not.
    expect(temperatureGaugeMock).toHaveBeenLastCalledWith(
      expect.objectContaining({
        value: 72,
        alertSeverity: 'warning',
        title: 'Temperature 72°C now, recovering. Clears after 5 minutes at 75°C or lower.',
      }),
    );
  });

  it('turns the held temperature reading stale on the shared clock while the row stays mounted', () => {
    const openedAt = Date.parse('2026-10-07T10:00:00Z');
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: openedAt });
    // The node stops reporting a minute before the page is read: its alert's
    // live status, and the row's data, never change again.
    activeAlertsMock.value = {
      'temperature-alert': {
        id: 'temperature-alert',
        resourceId: 'agent:pve-node-1',
        type: 'temperature',
        level: 'warning',
        message: 'Node temperature at 80.0°C',
        value: 80,
        lastSeen: new Date(openedAt - 5 * 60_000).toISOString(),
        acknowledged: false,
        metricStatus: {
          phase: 'latched',
          value: 76,
          unit: '°C',
          observedAt: new Date(openedAt - 60_000).toISOString(),
          trigger: 80,
          recovery: 75,
        },
      },
    };

    render(() => (
      <ProxmoxNodesTable
        nodes={[makeNodeResource({ temperature: 76 })]}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    const gauge = screen.getByTestId('temperature-gauge');
    expect(gauge).toHaveAttribute(
      'title',
      'Temperature 76°C now, back under the 80°C alert level. Stays open until it reaches 75°C or lower.',
    );

    vi.advanceTimersByTime(10 * 60_000);
    expect(gauge).toHaveAttribute(
      'title',
      'Last reading: Temperature 76°C, 11 mins ago. Stays open until it reaches 75°C or lower.',
    );

    vi.advanceTimersByTime(5 * 60_000);
    expect(gauge).toHaveAttribute(
      'title',
      'Last reading: Temperature 76°C, 16 mins ago. Stays open until it reaches 75°C or lower.',
    );
  });

  it('leaves the temperature tone to the reading once the alert is acknowledged', () => {
    activeAlertsMock.value = {
      'temperature-alert': {
        id: 'temperature-alert',
        resourceId: 'agent:pve-node-1',
        type: 'temperature',
        level: 'warning',
        message: 'Node temperature at 80.0°C',
        value: 80,
        acknowledged: true,
      },
    };

    render(() => (
      <ProxmoxNodesTable
        nodes={[makeNodeResource({ temperature: 72 })]}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    expect(temperatureGaugeMock).toHaveBeenLastCalledWith(
      expect.objectContaining({ alertSeverity: null, title: 'Node temperature at 80.0°C' }),
    );
  });

  it('keeps the Proxmox cache-inclusive memory comparison label on node rows', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={[
          makeNodeResource({
            memory: {
              total: 8_000,
              used: 3_200,
              free: 3_800,
              current: 40,
              cache: 1_000,
            } as Resource['memory'],
          }),
        ]}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    expect(screen.getByTestId('stacked-memory-bar')).toHaveAttribute(
      'data-cache-inclusive-label',
      'Shown in Proxmox',
    );
  });

  it('restores the v5 row signals: alert accent, pending updates badge, full uptime, offline dimming', () => {
    activeAlertsMock.value = {
      'alert-1': {
        id: 'alert-1',
        resourceId: 'agent:pve-node-1',
        level: 'critical',
        type: 'cpu',
        acknowledged: false,
      },
    };

    render(() => (
      <ProxmoxNodesTable
        nodes={[
          makeNodeResource({
            uptime: 187_200, // 2d 4h
            proxmox: {
              clusterName: 'homelab',
              nodeName: 'pve-node-1',
              pendingUpdates: 12,
            } as Resource['proxmox'],
          }),
          makeNodeResource({
            id: 'agent:pve-node-2',
            name: 'pve-node-2',
            displayName: 'pve-node-2',
            status: 'offline',
            proxmox: { clusterName: 'homelab', nodeName: 'pve-node-2' },
          }),
        ]}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    const alertedRow = screen.getByText('pve-node-1').closest('tr');
    expect(alertedRow).toHaveAttribute('data-workload-alert-accent', 'critical');
    expect(screen.getByTitle('12 pending apt updates')).toHaveTextContent('12 updates');
    expect(screen.getByText('2d 4h')).toBeInTheDocument();

    const offlineRow = screen.getByText('pve-node-2').closest('tr');
    expect(offlineRow?.className).toContain('opacity-60');
    expect(offlineRow).not.toHaveAttribute('data-workload-alert-accent');
    expect(offlineRow).toHaveTextContent('Offline');
  });

  it('does not present a retained stale update count as current in the node row', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={[
          makeNodeResource({
            proxmox: {
              clusterName: 'homelab',
              nodeName: 'pve-node-1',
              pendingUpdates: 12,
              pendingUpdatesStatus: 'stale',
              pendingUpdatesReason: 'source_unavailable',
            },
          }),
        ]}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    expect(screen.queryByTitle('12 pending apt updates')).not.toBeInTheDocument();
  });

  it('labels stale provider telemetry and does not present old metrics as current', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={[
          makeNodeResource({
            status: 'online',
            uptime: 187_200,
            temperature: 76,
            proxmox: {
              clusterName: 'homelab',
              nodeName: 'pve-node-1',
              connectionHealth: 'degraded',
            },
          }),
        ]}
        guests={[]}
        metricDisplayMode={() => 'sparklines'}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    const row = screen.getByText('pve-node-1').closest('tr');
    expect(row).toHaveTextContent('Stale');
    expect(row).toHaveTextContent('—');
    expect(row?.className).toContain('opacity-60');
    expect(screen.queryByTestId('metric-mini-sparkline')).not.toBeInTheDocument();
    expect(screen.queryByTestId('temperature-gauge')).not.toBeInTheDocument();
  });

  it('reads node history only, and only while Trends is on', () => {
    const [mode, setMode] = createSignal<'bars' | 'sparklines'>('bars');
    render(() => (
      <ProxmoxNodesTable
        nodes={[makeNodeResource()]}
        guests={[]}
        metricDisplayMode={mode}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    expect(metricHistoryMock).toHaveBeenCalledTimes(1);
    const options = metricHistoryMock.mock.calls[0][0] as {
      enabled: () => boolean;
      selectedNode?: unknown;
      series: string;
    };
    // Guest history belongs to the workloads table below; a second guest
    // reader here would poll the workloads route for rows it never draws.
    expect(options.series).toBe('nodes');
    expect(options.selectedNode).toBeUndefined();
    expect(options.enabled()).toBe(false);
    setMode('sparklines');
    expect(options.enabled()).toBe(true);
  });

  it('updates every availability signal when a live row becomes offline', () => {
    const [state, setState] = createStore({
      nodes: [makeNodeResource({ uptime: 187_200, temperature: 76 })],
    });

    render(() => (
      <ProxmoxNodesTable
        nodes={state.nodes}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    const row = screen.getByText('pve-node-1').closest('tr');
    expect(row).toHaveTextContent('2d 4h');
    expect(screen.getByTestId('temperature-gauge')).toBeInTheDocument();

    setState('nodes', 0, 'status', 'offline');
    setState('nodes', 0, 'proxmox', 'connectionHealth', 'error');

    expect(row).toHaveTextContent('Offline');
    expect(row).toHaveTextContent('—');
    expect(row).not.toHaveTextContent('2d 4h');
    expect(row?.className).toContain('opacity-60');
    expect(screen.queryByTestId('temperature-gauge')).not.toBeInTheDocument();
  });

  it('treats a live linked agent with an unavailable Proxmox provider as stale, not powered off', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={[
          makeNodeResource({
            status: 'online',
            uptime: 187_200,
            proxmox: {
              clusterName: 'homelab',
              nodeName: 'pve-node-1',
              connectionHealth: 'error',
            },
          }),
        ]}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    const row = screen.getByText('pve-node-1').closest('tr');
    expect(row).toHaveTextContent('Stale');
    expect(row).not.toHaveTextContent('Offline');
    expect(row).toHaveTextContent('—');
  });

  it('counts same-named-node guests only within the matching provider', () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={[
          makeNodeResource({
            id: 'site-a-node',
            platformId: 'site-a',
            proxmox: {
              instance: 'site-a',
              clusterName: 'production',
              nodeName: 'pve-1',
            },
          }),
          makeNodeResource({
            id: 'site-b-node',
            platformId: 'site-b',
            proxmox: {
              instance: 'site-b',
              clusterName: 'production',
              nodeName: 'pve-1',
            },
          }),
        ]}
        guests={[
          {
            id: 'site-b-vm',
            type: 'vm',
            name: 'site-b-vm',
            displayName: 'site-b-vm',
            platformId: 'site-b',
            platformType: 'proxmox-pve',
            sourceType: 'api',
            status: 'running',
            lastSeen: 1_700_000_000_000,
            proxmox: { instance: 'site-b', nodeName: 'pve-1', vmid: 101 },
          },
        ]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    const siteARow = document.querySelector('[data-proxmox-host-row="site-a-node"]');
    const siteBRow = document.querySelector('[data-proxmox-host-row="site-b-node"]');
    expect(siteARow?.querySelector('[data-proxmox-vm-count]')).toHaveTextContent('0');
    expect(siteBRow?.querySelector('[data-proxmox-vm-count]')).toHaveTextContent('1');
  });

  it('sorts rows from the column headers with metric columns defaulting to descending', async () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={[
          makeNodeResource({ cpu: { current: 10 } }),
          makeNodeResource({
            id: 'agent:pve-node-2',
            name: 'pve-node-2',
            displayName: 'pve-node-2',
            cpu: { current: 80 },
            proxmox: { clusterName: 'homelab', nodeName: 'pve-node-2' },
          }),
        ]}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    const rowNames = () =>
      screen
        .getAllByText(/pve-node-\d/)
        .map((el) => el.textContent)
        .filter((text) => text === 'pve-node-1' || text === 'pve-node-2');

    expect(rowNames()).toEqual(['pve-node-1', 'pve-node-2']);

    const cpuHeader = screen.getByText('CPU');
    await fireEvent.click(cpuHeader);
    expect(cpuHeader.closest('th')).toHaveAttribute('aria-sort', 'descending');
    expect(rowNames()).toEqual(['pve-node-2', 'pve-node-1']);

    await fireEvent.click(cpuHeader);
    expect(cpuHeader.closest('th')).toHaveAttribute('aria-sort', 'ascending');
    expect(rowNames()).toEqual(['pve-node-1', 'pve-node-2']);

    // Third click returns to the default (unsorted) order.
    await fireEvent.click(cpuHeader);
    expect(cpuHeader.closest('th')).not.toHaveAttribute('aria-sort');
    expect(rowNames()).toEqual(['pve-node-1', 'pve-node-2']);
  });

  it('selects guests from the row and reserves the chevron for node details', async () => {
    const onShowGuests = vi.fn();
    render(() => (
      <ProxmoxNodesTable
        nodes={[makeNodeResource()]}
        guests={[]}
        onShowGuests={onShowGuests}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    const row = screen.getByText('pve-node-1').closest('tr');
    expect(row).toBeTruthy();
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    expect(screen.queryByTestId('node-drawer')).not.toBeInTheDocument();

    await fireEvent.click(row!);
    expect(onShowGuests).toHaveBeenCalledTimes(1);
    expect(screen.queryByTestId('node-drawer')).not.toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Expand details for pve-node-1' }));
    expect(onShowGuests).toHaveBeenCalledTimes(1);

    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByTestId('node-drawer')).toHaveTextContent('pve-node-1');
    expect(nodeDrawerMock).toHaveBeenCalledWith(
      expect.objectContaining({
        node: expect.objectContaining({ name: 'pve-node-1' }),
        temperatureThresholds: { warning: 80, critical: 85 },
      }),
    );

    await fireEvent.click(screen.getByRole('button', { name: 'Collapse details for pve-node-1' }));
    expect(onShowGuests).toHaveBeenCalledTimes(1);

    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    expect(screen.queryByTestId('node-drawer')).not.toBeInTheDocument();
  });
});
