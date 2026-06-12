import { render, screen } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it } from 'vitest';

import { NodeGroupHeader } from '@/components/shared/NodeGroupHeader';
import type { Node } from '@/types/api';
import { temperatureStore } from '@/utils/temperature';

const makeNode = (overrides: Partial<Node> = {}): Node => ({
  id: 'cluster-a-pve1',
  name: 'pve1',
  displayName: 'pve1',
  instance: 'cluster-a',
  host: 'https://pve1:8006',
  status: 'online',
  type: 'pve',
  cpu: 0,
  memory: { total: 1, used: 0, free: 1, usage: 0 },
  disk: { total: 1, used: 0, free: 1, usage: 0 },
  uptime: 7200,
  loadAverage: [0, 0, 0],
  kernelVersion: '6.8.0',
  pveVersion: 'pve-manager/9.1.9/test',
  cpuInfo: { model: 'test', cores: 1, sockets: 1, mhz: '1' },
  temperature: {
    available: true,
    cpuPackage: 62,
    lastUpdate: '2026-01-01T00:00:00Z',
  },
  lastSeen: '2026-01-01T00:00:00Z',
  connectionHealth: 'online',
  isClusterMember: true,
  clusterName: 'homelab',
  ...overrides,
});

describe('NodeGroupHeader', () => {
  beforeEach(() => {
    temperatureStore.setUnit('celsius');
  });

  it('renders compact infrastructure facts in the grouped workload header', () => {
    render(() => <NodeGroupHeader node={makeNode()} />);

    expect(screen.getByText('pve1')).toBeInTheDocument();
    expect(screen.getByText('homelab')).toBeInTheDocument();
    expect(screen.getByText('PVE 9.1.9')).toBeInTheDocument();
    expect(screen.getByText('62°C')).toBeInTheDocument();
    expect(screen.getByText('2h')).toBeInTheDocument();

    const link = screen.getByRole('link', { name: 'Open web interface for pve1' });
    expect(link).toHaveAttribute('href', 'https://pve1:8006');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });
});
