import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';

import { NodeGroupHeader } from '@/components/shared/NodeGroupHeader';
import { NodeHardwareStrip } from '@/components/shared/NodeHardwareStrip';
import type { Node } from '@/types/api';

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
  it('renders node identity in the grouped workload header', () => {
    render(() => <NodeGroupHeader node={makeNode()} />);

    expect(screen.getByText('pve1')).toBeInTheDocument();
    expect(screen.getByText('homelab')).toBeInTheDocument();

    const link = screen.getByRole('link', { name: 'Open web interface for pve1' });
    expect(link).toHaveAttribute('href', 'https://pve1:8006');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('spans the table in one cell and leaves host stats to the platform host table', () => {
    const { container } = render(() => (
      <table>
        <tbody>
          <NodeGroupHeader node={makeNode()} renderAs="tr" colspan={9} />
        </tbody>
      </table>
    ));

    const cells = container.querySelectorAll('tr > td');
    expect(cells).toHaveLength(1);
    expect(cells[0]).toHaveAttribute('colspan', '9');
    expect(cells[0]).toHaveTextContent('pve1');
    expect(screen.queryByText('PVE 9.1.9')).not.toBeInTheDocument();
    expect(screen.queryByText(/°[CF]/)).not.toBeInTheDocument();
    expect(screen.queryByText('2h')).not.toBeInTheDocument();
  });

  it('shows what the node is as a hardware strip beside its identity', () => {
    render(() => (
      <NodeGroupHeader
        node={makeNode({
          cpuInfo: {
            model: 'Intel(R) Core(TM) i5-10600 CPU @ 3.30GHz',
            cores: 12,
            sockets: 1,
            mhz: '3300',
          },
          memory: { total: 15.3 * 1024 ** 3, used: 0, free: 0, usage: 0 },
          networkInterfaces: [
            { name: 'eno1', addresses: [] },
            { name: 'vmbr0', addresses: ['192.168.0.5/24'] },
          ],
        })}
      />
    ));

    const strip = screen.getByTestId('node-hardware-strip');
    expect(strip).toHaveTextContent('Intel Core i5-10600');
    expect(strip).toHaveTextContent('×12');
    expect(strip).toHaveTextContent('16 GB');
    expect(strip).toHaveTextContent('eno1');
    expect(strip).not.toHaveTextContent('vmbr0');
  });

  it('dims a port its agent reports without a link', () => {
    render(() => (
      <NodeHardwareStrip
        node={makeNode({
          networkInterfaces: [
            { name: 'enp3s0', addresses: [], speedMbps: 1000, linkUp: true },
            { name: 'enp1s0', addresses: [], linkUp: false },
          ],
        })}
      />
    ));
    const strip = screen.getByTestId('node-hardware-strip');
    const down = strip.querySelectorAll('[data-port-link="down"]');
    expect(down).toHaveLength(1);
    expect(down[0]).toHaveClass('opacity-40');
    expect(strip).toHaveTextContent('Network ports: enp3s0, 1 Gb/s, enp1s0, no link');
  });

  it('leaves the hardware strip out when the View menu hides it', () => {
    render(() => <NodeGroupHeader node={makeNode()} showHardware={false} />);
    expect(screen.getByText('pve1')).toBeInTheDocument();
    expect(screen.queryByTestId('node-hardware-strip')).not.toBeInTheDocument();
  });

  it('shows each port with its link speed and draws nothing for a node without hardware', () => {
    render(() => (
      <NodeHardwareStrip
        node={makeNode({
          networkInterfaces: [
            { name: 'enp1s0', addresses: [], speedMbps: 2500 },
            { name: 'wlp2s0', addresses: [] },
          ],
        })}
      />
    ));
    const strip = screen.getByTestId('node-hardware-strip');
    expect(strip).toHaveTextContent('2.5G');
    expect(strip).toHaveTextContent('Network ports: enp1s0, 2.5 Gb/s, wlp2s0 (Wi-Fi)');
    expect(strip.querySelectorAll('[data-port-link="down"]')).toHaveLength(0);

    const { container } = render(() => (
      <NodeHardwareStrip
        node={makeNode({
          cpuInfo: { model: 'Unknown', cores: 0, sockets: 0, mhz: '0' },
          memory: { total: 0, used: 0, free: 0, usage: 0 },
          networkInterfaces: [],
        })}
      />
    ));
    expect(container.innerHTML).toBe('');
  });
});
