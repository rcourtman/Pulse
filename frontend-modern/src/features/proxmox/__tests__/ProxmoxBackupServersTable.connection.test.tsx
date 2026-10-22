import { cleanup, fireEvent, render, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { canonicalizeRealtimeResource } from '@/utils/resourceStateAdapters';
import { buildBackupServerRows, ProxmoxBackupServersTable } from '../ProxmoxBackupServersTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: () => <div>Details</div>,
}));
afterEach(cleanup);

const server = (status: Resource['status'] = 'online', health = 'healthy'): Resource => ({
  id: 'pbs-status',
  type: 'pbs',
  name: 'pbs-status',
  displayName: 'pbs-status',
  platformId: 'pbs-status',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  status,
  lastSeen: Date.parse('2026-10-22T01:00:00Z'),
  cpu: { current: 12 },
  memory: { current: 40, total: 10000, used: 4000 },
  uptime: 7200,
  pbs: {
    instanceId: 'pbs-status',
    connectionHealth: health,
    datastores: [
      {
        name: 'tank',
        total: 10000,
        used: 4000,
        available: 6000,
        usagePercent: 40,
        status: 'available',
        deduplicationFactor: 2,
      },
    ],
  },
});
const present = (resource: Resource, transport: 'direct' | 'canonical'): Resource => {
  if (transport === 'direct') return resource;
  const { pbs, ...base } = resource;
  return canonicalizeRealtimeResource({ ...base, platformData: { sources: ['pbs'], pbs } });
};
function cell(container: HTMLElement, heading: string): HTMLElement {
  const index = [...container.querySelectorAll('thead th')].findIndex((h) =>
    h.textContent?.trim().endsWith(heading),
  );
  expect(index).toBeGreaterThanOrEqual(0);
  return container.querySelector('tbody tr')!.querySelectorAll('td')[index] as HTMLElement;
}

describe.each(['direct', 'canonical'] as const)('PBS primary offline state (%s)', (transport) => {
  it.each(['healthy', 'ok', '', 'error'])(
    'never promotes offline plus %s health to reachable',
    (health) => {
      const resource = present(server('offline', health), transport);
      const [row] = buildBackupServerRows([resource]);
      expect(row.online).toBe(false);
      expect(row.connectionLabel).toBe('Offline');
      expect(row.resource.status).toBe('offline');
      expect(row.resource.pbs?.connectionHealth).toBe(health);
      expect(row.datastore?.used).toBe(4000); // Retain evidence; do not rewrite it to zero.
    },
  );

  it('withdraws live metrics/capacity without losing an open datastore, then recovers in place', async () => {
    const [servers, setServers] = createSignal([present(server(), transport)]);
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={servers()} layoutWidth={() => 1200} />
    ));
    const row = container.querySelector('tbody tr')!;
    expect(cell(container, 'Status')).toHaveTextContent('Healthy');
    expect(cell(container, 'Used')).toHaveTextContent('40.0%');
    expect(cell(container, 'CPU')).toHaveTextContent('12.0%');
    fireEvent.click(
      within(row as HTMLElement).getByRole('button', { name: 'Expand details for pbs-status' }),
    );
    const region = within(container).getByRole('region', {
      name: 'Datastore capacity for pbs-status / tank',
    });
    const toggle = within(row as HTMLElement).getByRole('button', {
      name: 'Collapse details for pbs-status',
    });
    toggle.focus();
    setServers([present(server('offline'), transport)]);
    await waitFor(() => expect(cell(container, 'Status')).toHaveTextContent('Offline'));
    expect(container.querySelector('tbody tr')).toBe(row);
    expect(within(container).getByRole('region')).toBe(region);
    expect(toggle).toHaveFocus();
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(cell(container, 'Used')).toHaveTextContent('Unavailable');
    expect(cell(container, 'Used')).not.toHaveTextContent('%');
    for (const heading of ['CPU', 'Memory', 'Uptime', 'Dedup'])
      expect(cell(container, heading)).toHaveTextContent('—');
    expect(region).toHaveTextContent('Datastore capacity is unavailable.');
    expect(region).not.toHaveTextContent('Used:');
    expect(container.textContent).not.toContain('2.0×');
    setServers([present(server(), transport)]);
    await waitFor(() => expect(cell(container, 'Used')).toHaveTextContent('40.0%'));
    expect(cell(container, 'Status')).toHaveTextContent('Healthy');
    expect(within(container).getByRole('region')).toBe(region);
    expect(region).toHaveTextContent('Used: 40.0%');
    expect(toggle).toHaveFocus();
    expect(cell(container, 'CPU')).toHaveTextContent('12.0%');
    expect(cell(container, 'Dedup')).toHaveTextContent('2.0×');
  });

  it('does not mistake a reachable storage warning for a connection failure', () => {
    const resource = server('warning');
    resource.pbs!.datastores![0] = {
      ...resource.pbs!.datastores![0],
      used: 9500,
      available: 500,
      usagePercent: 95,
    };
    const { container } = render(() => (
      <ProxmoxBackupServersTable
        servers={[present(resource, transport)]}
        layoutWidth={() => 1200}
      />
    ));
    expect(cell(container, 'Status')).toHaveTextContent('Healthy');
    expect(cell(container, 'Used')).toHaveTextContent('95.0%');
    expect(cell(container, 'Used').querySelector('.bg-red-500')).not.toBeNull();
  });

  it('keeps an explicit failed connection authoritative over the online carrier', () => {
    const [row] = buildBackupServerRows([present(server('online', 'error'), transport)]);
    expect(row.online).toBe(false);
    expect(row.connectionLabel).toBe('Error');
  });
});
