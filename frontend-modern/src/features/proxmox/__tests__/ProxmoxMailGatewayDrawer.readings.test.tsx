import { cleanup, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { apiFetch } from '@/utils/apiClient';
import { ProxmoxMailGatewayDrawer } from '../ProxmoxMailGatewayDrawer';

vi.mock('@/utils/apiClient', () => ({ apiFetch: vi.fn() }));
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});

const resource = {
  id: 'unified-1',
  type: 'pmg',
  name: 'gateway',
  status: 'warning',
  pmg: { instanceId: 'pmg-gateway', connectionHealth: 'healthy' },
} as Resource;

function serve(nodes: unknown[]) {
  vi.mocked(apiFetch).mockImplementation(
    async () =>
      new Response(
        JSON.stringify({
          data: [
            {
              id: 'pmg-gateway',
              name: 'gateway',
              status: 'online',
              connectionHealth: 'healthy',
              nodes,
            },
          ],
          meta: { total: 1 },
        }),
        { status: 200 },
      ),
  );
}

describe('PMG detail preserves missing readings and resource health', () => {
  it('does not replace the stale/degraded resource state with an online full instance', async () => {
    serve([{ name: 'mail-a', status: 'online' }]);
    render(() => <ProxmoxMailGatewayDrawer instanceRow={resource} />);
    await screen.findByText('mail-a');
    expect(apiFetch).toHaveBeenCalledWith('/api/pmg/instances?id=pmg-gateway');
    expect(screen.getByRole('img', { name: 'Degraded' })).toHaveClass('bg-amber-500');
    expect(screen.queryByRole('img', { name: 'Healthy' })).toBeNull();
  });

  it('renders an uncollected queue as a dash but preserves measured zero and partial fields', async () => {
    serve([
      { name: 'missing-queue', status: 'online' },
      {
        name: 'zero-queue',
        status: 'online',
        queueStatus: { total: 0, active: 0, deferred: 0, hold: 0, incoming: 0 },
      },
      { name: 'partial-queue', status: 'online', queueStatus: { total: 5, active: 5 } },
    ]);
    render(() => <ProxmoxMailGatewayDrawer instanceRow={resource} />);
    await screen.findByText('missing-queue');
    for (const [name, expected] of [
      ['missing-queue', '—'],
      ['zero-queue', '00/0/0/0'],
      ['partial-queue', '55/—/—/—'],
    ]) {
      const row = screen.getByText(name).closest('tr')!;
      expect(within(row).getAllByRole('cell')[4].textContent?.trim()).toBe(expected);
    }
    expect(screen.queryByText(/undefined|NaN/)).toBeNull();
  });

  it('keeps the same open drawer truthful through critical attention and recovery', async () => {
    serve([{ name: 'mail-a', status: 'online' }]);
    const critical = {
      id: 'collection',
      type: 'connection-degraded',
      level: 'critical',
      message: 'Collection failed',
      acknowledged: false,
    } as Alert;
    const [state, setState] = createSignal<Resource>({ ...resource, status: 'online' });
    const [alerts, setAlerts] = createSignal<Alert[]>([critical]);
    render(() => <ProxmoxMailGatewayDrawer instanceRow={state()} alerts={alerts()} />);
    await screen.findByText('mail-a');
    expect(screen.getByRole('img', { name: 'Needs attention' })).toHaveClass('bg-red-500');
    setState(resource);
    setAlerts([]);
    expect(screen.getByRole('img', { name: 'Degraded' })).toBeInTheDocument();
    setState({ ...resource, status: 'online' });
    expect(screen.getByRole('img', { name: 'Healthy' })).toBeInTheDocument();
    await waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(3));
  });
});
