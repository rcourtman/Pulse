import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { GuestDrawer } from './GuestDrawer';

vi.mock('../Discovery/DiscoveryTab', () => ({
  DiscoveryTab: () => <div data-testid="discovery-tab">Discovery content</div>,
}));

vi.mock('./DiskList', () => ({
  DiskList: () => <div data-testid="disk-list" />,
}));

vi.mock('../shared/HistoryChart', () => ({
  HistoryChart: () => <div data-testid="history-chart" />,
}));

vi.mock('@/stores/license', () => ({
  hasFeature: () => true,
}));

describe('GuestDrawer discovery activation', () => {
  afterEach(() => {
    cleanup();
  });

  it('does not mount DiscoveryTab until the discovery tab is opened', async () => {
    render(() => (
      <GuestDrawer
        guest={
          {
            id: 'pve1:node1:100',
            instance: 'pve1',
            node: 'node1',
            vmid: 100,
            name: 'vm100',
            type: 'qemu',
            status: 'running',
            cpus: 2,
            uptime: 3600,
            memory: {
              total: 8 * 1024 * 1024 * 1024,
              used: 2 * 1024 * 1024 * 1024,
              free: 6 * 1024 * 1024 * 1024,
              usage: 25,
            },
            disks: [],
            networkInterfaces: [],
            ipAddresses: [],
            osName: 'Ubuntu',
            osVersion: '24.04',
            agentVersion: '1.0',
          } as any
        }
        metricsKey="vm:pve1:node1:100"
        onClose={() => undefined}
      />
    ));

    expect(screen.queryByTestId('discovery-tab')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Discovery' }));

    expect(await screen.findByTestId('discovery-tab')).toBeInTheDocument();
  });
});
