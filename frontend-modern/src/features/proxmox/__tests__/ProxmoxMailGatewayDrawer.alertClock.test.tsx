import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { ProxmoxMailGatewayDrawer } from '../ProxmoxMailGatewayDrawer';

vi.mock('@/utils/apiClient', () => ({
  apiFetch: vi.fn(async () => new Response(JSON.stringify(null), { status: 200 })),
}));

const OPENED_AT = Date.parse('2026-10-07T10:00:00Z');

const gateway = {
  id: 'pmg-unified-eu',
  type: 'pmg',
  name: 'mail-gateway-eu',
  displayName: 'mail-gateway-eu',
  platformType: 'proxmox-pmg',
  sourceType: 'api',
  status: 'online',
  lastSeen: OPENED_AT - 60_000,
  canonicalIdentity: {
    primaryId: 'agent:pmg-main',
    aliases: ['pmg-main', 'mail-gateway-eu'],
  },
  pmg: { instanceId: 'pmg-main', nodeCount: 1, mailCountTotal: 10, queueTotal: 3 },
} as unknown as Resource;

// A Pulse agent on the gateway host raises threshold alerts the row matches
// through its identity aliases. The host stopped reporting a minute before the
// drawer opened, so the alert's live status never changes again.
const heldCpuAlert: Alert = {
  id: 'agent:pmg-main::metric-threshold:cpu',
  type: 'cpu',
  level: 'warning',
  resourceId: 'agent:pmg-main',
  resourceName: 'mail-gateway-eu',
  node: 'mail-gateway-eu',
  instance: 'pmg-main',
  message: 'Host cpu at 93.0%',
  value: 93,
  threshold: 90,
  startTime: '2026-10-07T09:40:00Z',
  lastSeen: new Date(OPENED_AT - 5 * 60_000).toISOString(),
  acknowledged: false,
  metricStatus: {
    phase: 'latched',
    value: 72,
    unit: '%',
    observedAt: new Date(OPENED_AT - 60_000).toISOString(),
    trigger: 90,
    recovery: 80,
  },
};

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('ProxmoxMailGatewayDrawer alert attention clock', () => {
  it('turns a held reading stale and keeps its ages moving while the drawer stays open', () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: OPENED_AT });

    render(() => <ProxmoxMailGatewayDrawer instanceRow={gateway} alerts={[heldCpuAlert]} />);
    const attention = within(screen.getByTestId('drawer-attention-section'));

    expect(attention.getByText('CPU 72% now, back under the 90% alert level')).toHaveAttribute(
      'title',
      expect.stringContaining('Last reading at or above 90%: 93%, 5 mins ago'),
    );

    // Past the ten-minute cut-off the reading is no longer "now".
    vi.advanceTimersByTime(10 * 60_000);
    expect(attention.queryByText(/now, back under/)).not.toBeInTheDocument();
    expect(attention.getByText('Last reading: CPU 72%, 11 mins ago')).toHaveAttribute(
      'title',
      expect.stringContaining('Last reading at or above 90%: 93%, 15 mins ago'),
    );

    vi.advanceTimersByTime(5 * 60_000);
    expect(attention.getByText('Last reading: CPU 72%, 16 mins ago')).toHaveAttribute(
      'title',
      expect.stringContaining('Last reading at or above 90%: 93%, 20 mins ago'),
    );
  });

  it('keeps a list of alerts without a live status in place across clock ticks', () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: OPENED_AT });
    // Mail Gateway alerts come from their own checks and carry no live status,
    // so their copy never depends on the clock.
    const messageAge: Alert = {
      id: 'pmg-main::pmg-main-oldest-message',
      type: 'message-age',
      level: 'warning',
      resourceId: 'pmg-main',
      resourceName: 'mail-gateway-eu',
      node: 'https://pmg.example:8006',
      instance: 'pmg-main',
      message: 'PMG mail-gateway-eu has messages queued for 41 minutes (threshold: 30 minutes)',
      value: 41,
      threshold: 30,
      startTime: '2026-10-07T09:40:00Z',
      lastSeen: '2026-10-07T09:59:00Z',
      acknowledged: false,
    };

    render(() => <ProxmoxMailGatewayDrawer instanceRow={gateway} alerts={[messageAge]} />);
    const attention = within(screen.getByTestId('drawer-attention-section'));
    const row = attention.getByText(messageAge.message).closest('li');
    expect(row).not.toBeNull();

    vi.advanceTimersByTime(5 * 60_000);
    expect(attention.getByText(messageAge.message).closest('li')).toBe(row);
  });
});
