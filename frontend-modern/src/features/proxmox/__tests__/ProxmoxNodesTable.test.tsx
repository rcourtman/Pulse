import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createStore } from 'solid-js/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { ProxmoxNodesTable } from '../ProxmoxNodesTable';

const nodeDrawerMock = vi.hoisted(() => vi.fn());
const activeAlertsMock = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const getMetricThresholdsMock = vi.hoisted(() => vi.fn());
const temperatureGaugeMock = vi.hoisted(() => vi.fn());

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
  TemperatureGauge: (props: { value: number; thresholds?: unknown }) => {
    temperatureGaugeMock({ value: props.value, thresholds: props.thresholds });
    return <div data-testid="temperature-gauge" />;
  },
}));

vi.mock('@/components/Workloads/useWorkloadTableMetricHistory', () => ({
  useWorkloadTableMetricHistory: () => ({
    getNodeMetricSeries: () => [],
  }),
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
});

describe('ProxmoxNodesTable', () => {
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
            // pendingUpdates rides the raw proxmox payload; the narrow
            // ResourceProxmoxMeta type does not declare it yet.
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

  it('opens the host details drawer from the host-owned top table row', async () => {
    render(() => (
      <ProxmoxNodesTable
        nodes={[makeNodeResource()]}
        guests={[]}
        emptyIcon={<span />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="No nodes"
      />
    ));

    const row = screen.getByText('pve-node-1').closest('tr');
    expect(row).toBeTruthy();
    expect(row).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByTestId('node-drawer')).not.toBeInTheDocument();

    await fireEvent.click(row!);

    expect(row).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByTestId('node-drawer')).toHaveTextContent('pve-node-1');
    expect(nodeDrawerMock).toHaveBeenCalledWith(
      expect.objectContaining({
        node: expect.objectContaining({ name: 'pve-node-1' }),
        temperatureThresholds: { warning: 80, critical: 85 },
      }),
    );

    await fireEvent.click(row!);

    expect(row).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByTestId('node-drawer')).not.toBeInTheDocument();
  });
});
