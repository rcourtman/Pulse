import { cleanup, fireEvent, render, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { PBSBackup } from '@/types/api';
import type { Resource, ResourcePBSDatastore } from '@/types/resource';
import { canonicalizeRealtimeResource } from '@/utils/resourceStateAdapters';
import { buildBackupServerRows, ProxmoxBackupServersTable } from '../ProxmoxBackupServersTable';

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
    expect(within(cell).getByRole('button', { name: new RegExp(label, 'i') })).toBeInTheDocument();
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
    'explains %s capacity failures in the same open, named datastore disclosure',
    async (transport) => {
      const present = (store: ResourcePBSDatastore): Resource => {
        const resource = server(store);
        if (transport === 'direct') return resource;
        const { pbs, ...base } = resource;
        return canonicalizeRealtimeResource({ ...base, platformData: { sources: ['pbs'], pbs } });
      };
      const [servers, setServers] = createSignal([
        present(datastore({ status: 'unavailable', error: 'PRIVATE_PROVIDER_ERROR_SENTINEL' })),
      ]);
      const { container } = render(() => (
        <ProxmoxBackupServersTable servers={servers()} layoutWidth={() => 320} />
      ));
      const button = within(usedCell(container)).getByRole('button', {
        name: 'Explain unavailable capacity for pbs-capacity / tank',
      });
      expect(button).toHaveAttribute('type', 'button');
      expect(button).toHaveAttribute('aria-expanded', 'false');
      const click = new MouseEvent('click', { bubbles: true });
      const stop = vi.spyOn(click, 'stopPropagation');
      button.dispatchEvent(click);
      expect(stop).toHaveBeenCalledTimes(1);
      expect(button).toHaveAttribute('aria-expanded', 'true');
      const region = within(container).getByRole('region', {
        name: 'Datastore capacity for pbs-capacity / tank',
      });
      expect(region.closest('td')).toHaveAttribute('id', button.getAttribute('aria-controls'));
      expect(region).toHaveTextContent('Datastore capacity is unavailable.');
      expect(region).toHaveTextContent('This does not mean the datastore is empty.');
      expect(region).toHaveTextContent('Server reachability does not confirm a successful');
      expect(region).toHaveTextContent('Backup server: pbs-capacity');
      expect(region).toHaveTextContent('Datastore: tank');
      expect(region.textContent).not.toMatch(/40\.0%|0 B/);
      button.focus();
      setServers([present(datastore({ total: 0, used: 0, usagePercent: 0 }))]);
      await waitFor(() => expect(region).toHaveTextContent('Datastore capacity is unknown.'));
      expect(within(usedCell(container)).getByRole('button')).toBe(button);
      expect(button).toHaveFocus();
      expect(button).toHaveAttribute('aria-expanded', 'true');
      expect(within(container).getByRole('region')).toBe(region);
      expect(container.innerHTML).not.toContain('PRIVATE_PROVIDER_ERROR_SENTINEL');
      setServers([present(datastore({ used: 0, usagePercent: 0 }))]);
      await waitFor(() => expect(region).toHaveTextContent('Used: 0.0%'));
      expect(region).not.toHaveTextContent('does not mean the datastore is empty');
      expect(within(container).getByRole('region')).toBe(region);
      expect(within(usedCell(container)).queryByRole('button')).toBeNull();
    },
  );

  it('keeps each datastore explanation distinct and treats reported names as text', () => {
    const resource = server(datastore({ status: 'unavailable' }));
    const secondName = '<img src=x onerror=alert(1)>' + 'long-name-'.repeat(20);
    resource.pbs!.datastores!.push(datastore({ name: secondName, status: 'unknown' }));
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={[resource]} layoutWidth={() => 1200} />
    ));
    const open = (name: string) => {
      fireEvent.click(
        within(container).getByRole('button', {
          name: `Explain ${name === 'tank' ? 'unavailable' : 'unknown'} capacity for pbs-capacity / ${name}`,
        }),
      );
      return within(container).getByRole('region', {
        name: `Datastore capacity for pbs-capacity / ${name}`,
      });
    };
    expect(open('tank')).toHaveTextContent('Datastore capacity is unavailable.');
    const region = open(secondName);
    expect(region).toHaveTextContent(secondName);
    expect(region).toHaveTextContent('Datastore capacity is unknown.');
    expect(region.querySelector('img')).toBeNull();
    expect(within(container).getAllByRole('region')).toHaveLength(1);
    const details = region.closest('td')!;
    expect(details.colSpan).toBe(container.querySelector('thead tr')!.children.length);
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

const countServer = (name: string, instanceId = `pbs-${name}`, id = instanceId): Resource => ({
  id,
  type: 'pbs',
  name,
  displayName: name,
  platformId: id,
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_792_000_000_000,
  pbs: {
    instanceId,
    connectionHealth: 'healthy',
    datastores: [
      { name: 'tank', total: 1000, used: 400, available: 600, status: 'available' },
      { name: 'offsite', total: 1000, used: 200, available: 800, status: 'available' },
    ],
  },
});
const countBackup = (
  instance: string,
  datastore = 'tank',
  id = `${instance}-${datastore}`,
): PBSBackup => ({
  id,
  instance,
  datastore,
  namespace: '',
  backupType: 'vm',
  vmid: '100',
  backupTime: '2026-10-19T01:00:00Z',
  size: 100,
  files: ['manifest.blob'],
  protected: false,
  verified: true,
});
const countValues = (servers: Resource[], backups: PBSBackup[]) =>
  Object.fromEntries(
    buildBackupServerRows(servers, backups).map((row) => [row.key, row.backupCount]),
  );

describe('PBS backup count source ownership', () => {
  it('does not lend a current connection name to another server through its legacy ID alias', () => {
    const servers = [countServer('backup'), countServer('pbs-backup')];
    const backups = [countBackup('pbs-backup')];
    expect(countValues(servers, backups)).toEqual({
      'pbs-backup:offsite': 0,
      'pbs-backup:tank': 0,
      'pbs-pbs-backup:offsite': 0,
      'pbs-pbs-backup:tank': 1,
    });
    expect(countValues([...servers].reverse(), [...backups].reverse())).toEqual(
      countValues(servers, backups),
    );
  });

  it('does not fill a missing datastore from another source even when its own source has entries', () => {
    expect(
      countValues(
        [countServer('backup'), countServer('pbs-backup')],
        [countBackup('backup', 'offsite'), countBackup('pbs-backup', 'tank')],
      ),
    ).toEqual({
      'pbs-backup:offsite': 1,
      'pbs-backup:tank': 0,
      'pbs-pbs-backup:offsite': 0,
      'pbs-pbs-backup:tank': 1,
    });
  });

  it('keeps an unclaimed, unique legacy instance-ID source usable', () => {
    expect(
      countValues([countServer('backup')], [countBackup('pbs-backup')])['pbs-backup:tank'],
    ).toBe(1);
  });

  it('prefers the whole exact-name inventory over a legacy alias without adding them', () => {
    expect(
      countValues(
        [countServer('backup')],
        [countBackup('backup', 'offsite'), countBackup('pbs-backup', 'tank')],
      ),
    ).toEqual({ 'pbs-backup:offsite': 1, 'pbs-backup:tank': 0 });
  });

  it('does not duplicate entries across servers with the same configured source name', () => {
    const actual = countValues(
      [countServer('duplicate', 'pbs-a'), countServer('duplicate', 'pbs-b')],
      [countBackup('duplicate')],
    );
    expect(Object.values(actual)).toEqual([undefined, undefined, undefined, undefined]);
  });

  it('does not attribute an ambiguous legacy ID to either unnamed server', () => {
    expect(
      Object.values(
        countValues(
          [countServer('', 'legacy', 'a'), countServer('', 'legacy', 'b')],
          [countBackup('legacy')],
        ),
      ),
    ).toEqual([undefined, undefined, undefined, undefined]);
  });

  it('keeps real empty, missing identity and unrelated inventory distinct', () => {
    expect(
      countValues([countServer('backup')], [countBackup('unrelated')])['pbs-backup:tank'],
    ).toBe(0);
    expect(countValues([countServer('', '', 'missing')], [])['missing:tank']).toBeUndefined();
  });

  it('exposes the same scoped count in the compact disclosure through loading, failure and recovery', async () => {
    const [backups, setBackups] = createSignal([countBackup('pbs-backup')]);
    const [state, setState] = createSignal<'available' | 'loading' | 'unavailable'>('available');
    const { container } = render(() => (
      <ProxmoxBackupServersTable
        servers={[countServer('backup'), countServer('pbs-backup')]}
        backups={backups()}
        backupInventoryState={state()}
        layoutWidth={() => 320}
      />
    ));
    const toggle = within(
      container.querySelector('td[title="backup · tank"]') as HTMLElement,
    ).getByRole('button', { name: 'Expand details for backup' });
    fireEvent.click(toggle);
    const region = within(container).getByRole('region', {
      name: 'Datastore capacity for backup / tank',
    });
    expect(region).toHaveTextContent('Backup entries: 0');
    toggle.focus();
    setState('loading');
    await waitFor(() => expect(region).toHaveTextContent('Backup entries: Loading'));
    setState('unavailable');
    await waitFor(() => expect(region).toHaveTextContent('Backup entries: Unavailable'));
    setBackups([countBackup('backup'), countBackup('pbs-backup')]);
    setState('available');
    await waitFor(() => expect(region).toHaveTextContent('Backup entries: 1'));
    expect(toggle).toHaveFocus();
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
  });
});
