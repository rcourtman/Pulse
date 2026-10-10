import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { NodeDrawerOverview } from '@/components/Workloads/NodeDrawerOverview';
import type { Alert, Node } from '@/types/api';

const OPENED_AT = Date.parse('2026-10-07T10:00:00Z');

const node = {
  id: 'homelab-minipc',
  name: 'minipc',
  instance: 'homelab',
  status: 'online',
  type: 'node',
  cpu: 0,
  memory: { total: 1024, used: 256, free: 768, usage: 25 },
  disk: { total: 1024, used: 256, free: 768, usage: 25 },
  uptime: 3600,
  loadAverage: [],
  kernelVersion: '6.8.12',
  pveVersion: 'pve-manager/9.0.1',
  cpuInfo: { model: 'CPU', cores: 4, sockets: 1, mhz: '2400' },
  lastSeen: '2026-10-07T09:59:00Z',
  connectionHealth: 'healthy',
} as unknown as Node;

// The node stopped reporting a minute before the drawer opened: the alert's
// live status, and every prop the drawer receives, never change again.
const heldTemperatureAlert: Alert = {
  id: 'homelab-minipc::metric-threshold:temperature',
  type: 'temperature',
  level: 'warning',
  resourceId: 'homelab-minipc',
  resourceName: 'minipc',
  node: 'minipc',
  instance: 'homelab',
  message: 'Node temperature at 80.0°C',
  value: 80,
  threshold: 80,
  startTime: '2026-10-07T09:40:00Z',
  lastSeen: new Date(OPENED_AT - 5 * 60_000).toISOString(),
  acknowledged: false,
  metricStatus: {
    phase: 'latched',
    value: 76,
    unit: '°C',
    observedAt: new Date(OPENED_AT - 60_000).toISOString(),
    trigger: 80,
    recovery: 75,
    recoveryDelaySeconds: 300,
  },
};

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('NodeDrawerOverview alert attention clock', () => {
  it('turns a held reading stale and keeps its ages moving while the drawer stays open', () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: OPENED_AT });

    render(() => <NodeDrawerOverview node={node} alerts={[heldTemperatureAlert]} />);
    const attention = within(screen.getByTestId('drawer-attention-section'));

    const live = attention.getByText('Temperature 76°C now, back under the 80°C alert level');
    expect(live).toHaveAttribute(
      'title',
      expect.stringContaining('Last reading at or above 80°C: 80°C, 5 mins ago'),
    );

    // Past the ten-minute cut-off the reading is no longer "now".
    vi.advanceTimersByTime(10 * 60_000);
    expect(attention.queryByText(/now, back under/)).not.toBeInTheDocument();
    const stale = attention.getByText('Last reading: Temperature 76°C, 11 mins ago');
    expect(stale).toHaveAttribute(
      'title',
      expect.stringContaining('Last reading at or above 80°C: 80°C, 15 mins ago'),
    );
    expect(
      attention.getByText(
        'Stays open until it reaches 75°C or lower and stays there for 5 minutes.',
      ),
    ).toBeInTheDocument();

    vi.advanceTimersByTime(5 * 60_000);
    expect(attention.getByText('Last reading: Temperature 76°C, 16 mins ago')).toHaveAttribute(
      'title',
      expect.stringContaining('Last reading at or above 80°C: 80°C, 20 mins ago'),
    );
  });
});
