import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { ProxmoxNodesTable } from '../ProxmoxNodesTable';

const nodeDrawerMock = vi.hoisted(() => vi.fn());

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    width: () => 1280,
  }),
}));

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div data-testid="responsive-metric-cell" />,
}));

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: () => <div data-testid="stacked-memory-bar" />,
}));

vi.mock('@/components/Workloads/StackedDiskBar', () => ({
  StackedDiskBar: () => <div data-testid="stacked-disk-bar" />,
}));

vi.mock('@/components/Workloads/MetricMiniSparkline', () => ({
  MetricMiniSparkline: () => <div data-testid="metric-mini-sparkline" />,
}));

vi.mock('@/components/shared/TemperatureGauge', () => ({
  TemperatureGauge: () => <div data-testid="temperature-gauge" />,
}));

vi.mock('@/components/Workloads/useWorkloadTableMetricHistory', () => ({
  useWorkloadTableMetricHistory: () => ({
    getNodeMetricSeries: () => [],
  }),
}));

vi.mock('@/components/Workloads/NodeDrawer', () => ({
  NodeDrawer: (props: { node: { name: string } }) => {
    nodeDrawerMock(props.node);
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

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
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

    const link = screen.getByRole('link', { name: 'Open pve-node-1 web interface' });
    expect(link).toHaveAttribute('href', 'https://pve.example.com:8006');
    expect(link).toHaveAttribute('target', '_blank');
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

    expect(screen.getByRole('link', { name: 'Open pve-node-1 web interface' })).toHaveAttribute(
      'href',
      'https://pve-node-1:8006',
    );
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
    expect(nodeDrawerMock).toHaveBeenCalledWith(expect.objectContaining({ name: 'pve-node-1' }));

    await fireEvent.click(row!);

    expect(row).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByTestId('node-drawer')).not.toBeInTheDocument();
  });
});
