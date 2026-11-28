import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { GuestDrawer } from '../GuestDrawer';
import { NodeGroupHeader } from '@/components/shared/NodeGroupHeader';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';

vi.mock('@/api/charts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/charts')>();
  return {
    ...actual,
    ChartsAPI: {
      ...actual.ChartsAPI,
      getMetricsHistory: vi.fn().mockResolvedValue({
        resourceType: 'vm',
        resourceId: 'prod-pve-stale-101',
        range: '24h',
        metrics: {},
      }),
    },
  };
});

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
  memory: {
    total: 8 * 1024 ** 3,
    used: 6 * 1024 ** 3,
    free: 2 * 1024 ** 3,
    usage: 75,
    observation: {
      state: 'current',
      source: 'guest-agent-meminfo',
      observedAt: new Date().toISOString(),
    },
  },
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

beforeEach(() => {
  resetCreateNonSuspendingQueryCacheForTest();
});

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
});

describe('GuestDrawer and NodeGroupHeader stale telemetry presentation', () => {
  it('shows stale warning header status, defers platform metrics with static capacities, and clears reactively on recovery', async () => {
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

    const facts = screen.getByTestId('guest-drawer-facts');
    expect(facts).toHaveTextContent('8.00 GB RAM');
    expect(facts).not.toHaveTextContent('6.00 GB of 8.00 GB RAM');
    expect(facts).toHaveTextContent('100 GB disk');
    expect(facts).not.toHaveTextContent('48.0 GB of 100 GB disk');

    await waitFor(() => {
      const historyRegion = screen.getByTestId('guest-history-region');
      const cpuDeferred = historyRegion.querySelector('[data-history-deferred="cpu"]');
      expect(cpuDeferred).not.toBeNull();
      expect(cpuDeferred).toHaveTextContent('Last value received 18m ago.');
      expect(historyRegion.querySelector('[data-history-deferred="memory"]')).not.toBeNull();
      expect(historyRegion.querySelector('[data-history-deferred="disk"]')).not.toBeNull();
      expect(historyRegion.querySelector('[data-history-deferred="netin"]')).not.toBeNull();
      expect(historyRegion.querySelector('[data-history-deferred="netout"]')).not.toBeNull();
      expect(historyRegion.querySelector('[data-history-deferred="diskread"]')).not.toBeNull();
      expect(historyRegion.querySelector('[data-history-deferred="diskwrite"]')).not.toBeNull();
    });

    setGuest(makeGuest({ telemetryStale: undefined }));
    expect(screen.queryByTestId('guest-telemetry-stale-notice')).not.toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Running' })).toBeVisible();
    expect(facts).toHaveTextContent('6.00 GB of 8.00 GB RAM');
    expect(facts).toHaveTextContent('48.0 GB of 100 GB disk');
    await waitFor(() => {
      const historyRegion = screen.getByTestId('guest-history-region');
      expect(historyRegion.querySelector('[data-history-deferred="cpu"]')).toBeNull();
      expect(historyRegion.querySelector('[data-history-deferred="memory"]')).toBeNull();
      expect(historyRegion.querySelector('[data-history-deferred="disk"]')).toBeNull();
    });
  });

  it('keeps fresh linked-agent disk live while deferring stale hypervisor metrics', async () => {
    const stale = {
      label: 'Stale data',
      age: '18m',
      detail: 'No Proxmox report for 18m. Showing the last values received.',
    };
    render(() => (
      <GuestDrawer
        guest={makeGuest({
          telemetryStale: stale,
          disksFromAgent: true,
          agentStale: false,
          agentKind: 'pulse',
        })}
        onClose={() => {}}
      />
    ));

    const facts = screen.getByTestId('guest-drawer-facts');
    expect(facts).toHaveTextContent('8.00 GB RAM');
    expect(facts).not.toHaveTextContent('6.00 GB of 8.00 GB RAM');
    expect(facts).toHaveTextContent('48.0 GB of 100 GB disk');

    await waitFor(() => {
      const historyRegion = screen.getByTestId('guest-history-region');
      expect(historyRegion.querySelector('[data-history-deferred="cpu"]')).not.toBeNull();
      expect(historyRegion.querySelector('[data-history-deferred="memory"]')).not.toBeNull();
      expect(historyRegion.querySelector('[data-history-deferred="disk"]')).toBeNull();
    });
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
