import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { parsePBSBackupInventory } from '../proxmoxBackupInventory';
import { buildBackupServerRows } from '../ProxmoxBackupServersTable';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: () => <div data-testid="pbs-count-owner-detail">Existing owner detail</div>,
}));

const apiFetchMock = vi.hoisted(() => vi.fn());
vi.mock('@/utils/apiClient', () => ({ apiFetch: apiFetchMock, apiFetchJSON: vi.fn() }));

const server = (name: string, datastore = 'tank'): Resource => ({
  id: `resource-${name}`,
  name,
  displayName: `Display ${name}`,
  type: 'pbs',
  status: 'online',
  lastSeen: 1_793_300_000_000,
  platformId: 'proxmox-pbs',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  pbs: {
    instanceId: `pbs-${name}`,
    hostname: 'shared-host.example.test',
    connectionHealth: 'healthy',
    datastores: [{ name: datastore, total: 1000, used: 400 }],
  },
});

const entry = (instance: string | undefined, datastore = 'tank', suffix = '1') => ({
  id: JSON.stringify([instance, datastore, suffix]),
  instance,
  datastore,
  namespace: suffix,
  backupType: 'vm',
  vmid: '100',
  backupTime: '2026-10-30T01:00:00Z',
  files: ['index.json.blob'],
});
const inventory = (...entries: ReturnType<typeof entry>[]) => ({ data: { backups: entries } });
const rows = (servers: Resource[], payload: unknown) =>
  buildBackupServerRows(servers, parsePBSBackupInventory(payload).backups);

// No DOM marker dependency: the table's summary rows have count-column cells,
// whereas a detail row has only its spanning cell.
const displayedCounts = (table: HTMLElement) => {
  const index = [...table.querySelectorAll('th')].findIndex(
    (cell) => cell.getAttribute('aria-label') === 'Backups',
  );
  return [...table.querySelectorAll('tbody tr')]
    .filter((row) => row.querySelectorAll('td').length > index)
    .map((row) => row.querySelectorAll('td')[index].textContent?.trim());
};

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  apiFetchMock.mockReset();
  resetCreateNonSuspendingQueryCacheForTest();
  window.history.replaceState({}, '', '/');
});

describe('PBS server count identity', () => {
  it('does not borrow a namesake service-ID connection when the named reader has no entries', () => {
    const result = rows(
      [server('main'), server('pbs-main')],
      inventory(entry('pbs-main'), entry('pbs-main', 'tank', '2')),
    );
    expect(result.map((row) => [row.serverName, row.backupCount])).toEqual([
      ['main', 0],
      ['pbs-main', 2],
    ]);
  });

  it.each([
    ['a::b', 'c', 'a', 'b::c'],
    ['a', 'b::c', 'a::b', 'c'],
  ])('keeps delimiter-bearing identities separate (%s / %s)', (name, store, other, otherStore) => {
    const result = rows(
      [server(name, store), server(other, otherStore)],
      inventory(entry(name, store), entry(other, otherStore), entry(other, otherStore, '2')),
    );
    expect(result.find((row) => row.serverName === name)?.backupCount).toBe(1);
    expect(result.find((row) => row.serverName === other)?.backupCount).toBe(2);
  });

  it('keeps exact reader/store case, namespaces and explicit empty tuples', () => {
    const result = rows(
      [server('Main', 'Tank'), server('main', 'tank'), server('empty')],
      inventory(entry('Main', 'Tank'), entry('Main', 'Tank', '2'), entry('main', 'tank')),
    );
    expect(result.map((row) => [row.serverName, row.backupCount])).toEqual([
      ['empty', 0],
      ['main', 1],
      ['Main', 2],
    ]);
  });

  it('does not use display labels, hostnames or unattributed entries as reader identity', () => {
    const result = rows(
      [server('main')],
      inventory(entry('Display main'), entry('shared-host.example.test'), entry(undefined)),
    );
    expect(result[0].backupCount).toBe(0);
  });

  it('counts admitted entries rather than inventing a completion or verification filter', () => {
    const payload = inventory(entry('main'), entry('main', 'tank', '2'));
    Object.assign(payload.data.backups[1], { inProgress: true, verified: false });
    expect(rows([server('main')], payload)[0].backupCount).toBe(2);
  });

  it('keeps name-matched inventory even when an unrelated service-ID tuple also exists', () => {
    expect(
      rows(
        [server('main')],
        inventory(entry('main'), entry('pbs-main'), entry('pbs-main', 'tank', '2')),
      )[0].backupCount,
    ).toBe(1);
  });

  it('replaces exact server counts on polling, withdrawing failed reads without losing capacity', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] });
    vi.stubGlobal('scrollTo', vi.fn());
    window.history.replaceState({}, '', '/proxmox/backups/date');
    let payload = inventory(entry('pbs-main'), entry('pbs-main', 'tank', '2'));
    let failed = false;
    apiFetchMock.mockImplementation((url: string) =>
      Promise.resolve(
        new Response(
          JSON.stringify(
            url === '/api/backups/pbs'
              ? payload
              : { data: { storageBackups: [], guestSnapshots: [], backupTasks: [] } },
          ),
          { status: url === '/api/backups/pbs' && failed ? 403 : 200 },
        ),
      ),
    );
    const { container } = render(() => (
      <Router>
        <Route
          path="/*"
          component={() => (
            <ProxmoxBackupsTable
              emptyIcon={<span />}
              servers={[server('main'), server('pbs-main')]}
            />
          )}
        />
      </Router>
    ));
    await screen.findByRole('columnheader', { name: 'Backups' });
    const table = container.querySelector('[data-proxmox-backups-table="servers"]') as HTMLElement;
    await waitFor(() => expect(displayedCounts(table)).toEqual(['0', '2']));
    const disclosures = within(table).getAllByRole('button', { name: /details/i });
    await fireEvent.click(disclosures[0]);
    const detail = screen.getByTestId('pbs-count-owner-detail');

    // Only the query's actual 30-second polling is advanced; no direct call to
    // the count producer and no fabricated rendered rows in this consumer test.
    payload = inventory(entry('main'), entry('main', 'tank', '2'), entry('main', 'tank', '3'));
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() => expect(displayedCounts(table)).toEqual(['3', '0']));
    expect(screen.getByTestId('pbs-count-owner-detail')).toBe(detail);

    failed = true;
    await vi.advanceTimersByTimeAsync(30_000);
    await screen.findByRole('button', { name: 'Retry PBS inventory' });
    await waitFor(() => expect(displayedCounts(table)).toEqual(['Unavailable', 'Unavailable']));
    expect(within(table).getAllByText('40.0%')).toHaveLength(2);
    expect(within(table).getAllByText('Healthy')).toHaveLength(2);
    failed = false;
    payload = inventory();
    await fireEvent.click(screen.getByRole('button', { name: 'Retry PBS inventory' }));
    await waitFor(() => expect(displayedCounts(table)).toEqual(['0', '0']));
    expect(apiFetchMock.mock.calls.filter(([url]) => url === '/api/backups/pbs')).toHaveLength(4);
    expect(
      apiFetchMock.mock.calls.every(([url]) =>
        ['/api/backups/pbs', '/api/backups/pve'].includes(url),
      ),
    ).toBe(true);
  });
});
