import { cleanup, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';
import { GuestDrawer } from '../GuestDrawer';
import { NodeGroupHeader } from '@/components/shared/NodeGroupHeader';
import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';

const makeGuest = (overrides: Partial<WorkloadGuest> = {}): WorkloadGuest => ({
  id: 'prod-pve-stale-101',
  name: 'db-primary',
  vmid: 101,
  instance: 'prod',
  node: 'pve-stale',
  type: 'qemu',
  status: 'running',
  cpu: 0.42,
  cpus: 4,
  memory: { total: 8 * 1024 ** 3, used: 6 * 1024 ** 3, free: 2 * 1024 ** 3, usage: 75 },
  disk: { total: 100 * 1024 ** 3, used: 48 * 1024 ** 3, free: 52 * 1024 ** 3, usage: 48 },
  networkIn: 120000,
  networkOut: 85000,
  diskRead: 64000,
  diskWrite: 192000,
  uptime: 432000,
  template: false,
  tags: [],
  lastBackup: 0,
  lock: '',
  lastSeen: '2026-11-28T02:42:00Z',
  ...overrides,
});

const makeNode = (overrides: Partial<Node> = {}): Node => ({
  id: 'prod-pve-stale',
  name: 'pve-stale',
  instance: 'prod',
  host: 'https://pve-stale.local:8006',
  type: 'node',
  status: 'degraded',
  connectionHealth: 'stale',
  cpu: 0.3,
  memory: { total: 64 * 1024 ** 3, used: 32 * 1024 ** 3, free: 32 * 1024 ** 3, usage: 50 },
  disk: { total: 500 * 1024 ** 3, used: 100 * 1024 ** 3, free: 400 * 1024 ** 3, usage: 20 },
  uptime: 864000,
  loadAverage: [0.5, 0.4, 0.3],
  kernelVersion: '6.8.12',
  pveVersion: '8.3.1',
  cpuInfo: { model: 'EPYC', cores: 16, sockets: 1, mhz: '2400' },
  lastSeen: '2026-11-28T02:42:00Z',
  isClusterMember: true,
  clusterName: 'prod',
  ...overrides,
});

afterEach(cleanup);

describe('GuestDrawer and NodeGroupHeader stale telemetry presentation', () => {
  it('shows stale warning header status and notice in GuestDrawer and clears reactively on recovery', () => {
    const stale = {
      label: 'Stale data',
      age: '18m',
      error: 'dial tcp 10.0.0.10:8006: i/o timeout',
      detail:
        'Proxmox has not updated this recently: dial tcp 10.0.0.10:8006: i/o timeout (last report 18m ago). Showing the last values received.',
    };
    const [guest, setGuest] = createSignal(makeGuest({ telemetryStale: stale }));
    render(() => <GuestDrawer guest={guest()} onClose={() => {}} />);

    const notice = screen.getByTestId('guest-telemetry-stale-notice');
    expect(notice).toBeVisible();
    expect(notice).toHaveTextContent('Readings are stale');
    expect(notice).toHaveTextContent(stale.detail);
    expect(screen.getByRole('img', { name: stale.detail })).toBeVisible();

    setGuest(makeGuest({ telemetryStale: undefined }));
    expect(screen.queryByTestId('guest-telemetry-stale-notice')).not.toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Running' })).toBeVisible();
  });

  it('shows a Stale data badge on NodeGroupHeader when node connection health is stale', () => {
    const [node, setNode] = createSignal(makeNode());
    const { container } = render(() => (
      <table>
        <tbody>
          <NodeGroupHeader node={node()} renderAs="tr" colspan={6} />
        </tbody>
      </table>
    ));

    const badge = container.querySelector('[data-node-group-stale]');
    expect(badge).not.toBeNull();
    expect(badge).toHaveTextContent('Stale data');

    setNode(makeNode({ status: 'online', connectionHealth: 'online' }));
    expect(container.querySelector('[data-node-group-stale]')).toBeNull();
  });
});
