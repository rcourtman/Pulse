import { cleanup, render, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource, ResourcePBSDatastore } from '@/types/resource';
import { canonicalizeRealtimeResource } from '@/utils/resourceStateAdapters';
import { ProxmoxBackupServersTable } from '../ProxmoxBackupServersTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: () => <div>Details</div>,
}));

afterEach(cleanup);

const datastore = (patch: Record<string, unknown> = {}): ResourcePBSDatastore =>
  ({
    name: 'tank',
    total: 1000,
    used: 400,
    available: 600,
    status: 'available',
    deduplicationFactor: 2,
    ...patch,
  }) as ResourcePBSDatastore;

const server = (store?: ResourcePBSDatastore, health = 'healthy'): Resource => ({
  id: 'pbs-capacity',
  type: 'pbs',
  name: 'pbs-capacity',
  displayName: 'pbs-capacity',
  status: 'online',
  platformId: 'pbs-capacity',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  lastSeen: 1_790_000_000_000,
  pbs: { instanceId: 'pbs-capacity', connectionHealth: health, datastores: store ? [store] : [] },
});

function usedCell(container: HTMLElement) {
  const table = container.querySelector('table')!;
  const index = Array.from(table.querySelectorAll('th')).findIndex(
    (head) => head.textContent === 'Used',
  );
  return table.querySelector('tbody tr')!.querySelectorAll('td')[index] as HTMLElement;
}

describe('PBS datastore capacity evidence', () => {
  it.each([
    [
      'provider failure zeros',
      { total: 0, used: 0, available: 0, usagePercent: 0, status: 'unavailable' },
      'Unavailable',
    ],
    ['retained failure values', { status: 'unavailable', usagePercent: 40 }, 'Unavailable'],
    [
      'explicit error with healthy status',
      { error: 'PRIVATE_PROVIDER_ERROR_SENTINEL' },
      'Unavailable',
    ],
    ['unknown status', { status: 'unknown', usagePercent: 0 }, 'Unknown'],
    ['future status', { status: 'new-provider-state' }, 'Unknown'],
    ['zero total', { total: 0, used: 0, usagePercent: 0 }, 'Unknown'],
    ['missing used', { used: undefined }, 'Unknown'],
    ['negative used', { used: -10 }, 'Unknown'],
    ['nonfinite total', { total: Infinity }, 'Unknown'],
    ['invalid authoritative percent', { usagePercent: NaN }, 'Unknown'],
    ['negative authoritative percent', { usagePercent: -1 }, 'Unknown'],
    ['null authoritative percent', { usagePercent: null }, 'Unknown'],
    ['string authoritative percent', { usagePercent: '0' }, 'Unknown'],
    ['overflowed ratio', { used: Number.MAX_VALUE, total: Number.MIN_VALUE }, 'Unknown'],
  ])('shows %s as unknown evidence rather than healthy usage', async (_name, patch, label) => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={[server(datastore(patch))]} layoutWidth={() => 1200} />
    ));
    const cell = usedCell(container);
    await waitFor(() => expect(within(cell).getByText(label)).toBeInTheDocument());
    expect(cell.textContent).not.toMatch(/%|0 B/);
    expect(cell.querySelector('.bg-emerald-500')).toBeNull();
    expect(cell.querySelector('[title]')?.getAttribute('title')).toContain(
      'does not mean the datastore is empty',
    );
    expect(container.textContent).not.toContain('2.0×');
    expect(container.innerHTML).not.toContain('PRIVATE_PROVIDER_ERROR_SENTINEL');
    expect(within(container).getByText('Healthy')).toBeInTheDocument();
  });

  it('withdraws retained capacity and dedup when the PBS connection is offline', () => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable
        servers={[server(datastore(), 'offline')]}
        layoutWidth={() => 1200}
      />
    ));
    expect(usedCell(container).textContent).toBe('Unavailable');
    expect(container.textContent).not.toContain('2.0×');
  });

  it.each([
    [0, '0.0%', 'bg-emerald-500'],
    [750, '75.0%', 'bg-amber-500'],
    [900, '90.0%', 'bg-red-500'],
    [1100, '110.0%', 'bg-red-500'],
  ])('keeps measured usage %s and the existing warning thresholds', async (used, label, tone) => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={[server(datastore({ used }))]} layoutWidth={() => 1200} />
    ));
    const cell = usedCell(container);
    await waitFor(() => expect(within(cell).getByText(label)).toBeInTheDocument());
    expect(cell.querySelector(`.${tone}`)).not.toBeNull();
    expect(container.textContent).toContain('2.0×');
  });

  it('keeps a server without datastore inventory distinct from an unknown named store', () => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={[server()]} layoutWidth={() => 1200} />
    ));
    expect(usedCell(container).textContent).toBe('No datastore data');
  });

  it.each(['direct', 'canonical'] as const)(
    'reconciles same-identity %s failure and recovery without remounting',
    async (transport) => {
      const present = (store: ResourcePBSDatastore): Resource => {
        const resource = server(store);
        if (transport === 'direct') return resource;
        const { pbs, ...base } = resource;
        return canonicalizeRealtimeResource({ ...base, platformData: { sources: ['pbs'], pbs } });
      };
      const [servers, setServers] = createSignal([present(datastore())]);
      const { container } = render(() => (
        <ProxmoxBackupServersTable servers={servers()} layoutWidth={() => 1200} />
      ));
      const cell = usedCell(container);
      await waitFor(() => expect(cell.textContent).toContain('40.0%'));
      setServers([
        present(datastore({ total: 0, used: 0, usagePercent: 0, status: 'unavailable' })),
      ]);
      await waitFor(() => expect(cell.textContent).toBe('Unavailable'));
      expect(usedCell(container)).toBe(cell);
      setServers([present(datastore({ used: 0, usagePercent: 0 }))]);
      await waitFor(() => expect(cell.textContent).toContain('0.0%'));
      expect(usedCell(container)).toBe(cell);
      setServers([present(datastore({ used: 950, usagePercent: 95 }))]);
      await waitFor(() => expect(cell.textContent).toContain('95.0%'));
      expect(cell.querySelector('.bg-red-500')).not.toBeNull();
    },
  );
});
