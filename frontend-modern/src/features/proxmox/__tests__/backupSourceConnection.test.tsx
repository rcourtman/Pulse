import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { PBSBackup, PVEBackupsPayload } from '@/types/api';
import { protectionPolicy } from '@/utils/__fixtures__/protectionPosture';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
  identitySnapshot,
} from '../__fixtures__/backupIdentity';

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn() }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));

const guests = ['east', 'west'].map((instance) => ({
  ...identityGuest(`guest-${instance}`, instance, 'pve-1'),
  name: 'same-name',
  displayName: 'same-name',
}));
const emptyPVE: PVEBackupsPayload = {
  backupTasks: [],
  storageBackups: [],
  guestSnapshots: [],
};
const points = (connection = 'pbs-east'): PBSBackup[] => [
  identityPBS({ id: 'point-east', instance: connection }),
  identityPBS({ id: 'point-west', instance: 'pbs-west' }),
];
const response = (data: unknown) =>
  new Response(JSON.stringify({ data }), { headers: { 'Content-Type': 'application/json' } });

function mount(
  getPBS: () => PBSBackup[],
  pve: PVEBackupsPayload = emptyPVE,
  workloads = guests.slice(0, 1),
) {
  api.fetch.mockImplementation((url: string) =>
    Promise.resolve(response(url === '/api/backups/pbs' ? { backups: getPBS() } : pve)),
  );
  // A visible source must not invent canonical workload protection.
  api.json.mockResolvedValue({ data: [], policy: protectionPolicy });
  return render(() => (
    <Router>
      <Route
        path="/*"
        component={() => <ProxmoxBackupsTable emptyIcon={<span />} workloads={workloads} />}
      />
    </Router>
  ));
}

async function inventoryRows() {
  await screen.findAllByText('same-name');
  return Array.from(
    document.querySelectorAll<HTMLElement>('[data-proxmox-backup-row="recoverable"]'),
  );
}

const connectionOf = (row: HTMLElement) =>
  row.querySelector('[data-backup-source-connection]')?.textContent;

beforeEach(() => {
  vi.useFakeTimers({ now: BACKUP_IDENTITY_NOW, toFake: ['Date'] });
  vi.resetAllMocks();
  resetCreateNonSuspendingQueryCacheForTest();
  window.history.replaceState({}, '', '/proxmox/backups/date');
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.useRealTimers();
});

describe('restore-point source connections in real inventory consumers', () => {
  it.each(['pbs', 'archive', 'snapshot'] as const)(
    'distinguishes same-named %s repositories and workloads in rows and full details',
    async (kind) => {
      const pve = {
        ...emptyPVE,
        storageBackups:
          kind === 'archive'
            ? ['east', 'west'].map((instance) =>
                identityArchive({ id: `file-${instance}`, instance, storage: 'main' }),
              )
            : [],
        guestSnapshots:
          kind === 'snapshot'
            ? ['east', 'west'].map((instance) =>
                identitySnapshot({ id: `snapshot-${instance}`, instance }),
              )
            : [],
      };
      mount(
        () => (kind === 'pbs' ? points() : []),
        pve,
        kind === 'pbs' ? guests.slice(0, 1) : guests,
      );
      const rows = await inventoryRows();
      expect(rows).toHaveLength(2);
      for (const connection of kind === 'pbs' ? ['pbs-east', 'pbs-west'] : ['east', 'west']) {
        const row = rows.find((item) => connectionOf(item) === `Connection: ${connection}`)!;
        expect(row).toBeDefined();
        fireEvent.click(within(row).getByRole('button', { name: /expand details/i }));
        const detail = screen.getByTestId('proxmox-recoverable-detail');
        const connectionRow = within(detail).getByText('Connection').closest('tr')!;
        expect(within(connectionRow).getByText(connection)).toBeInTheDocument();
        expect(
          within(detail).getByText(
            kind === 'pbs' ? 'main / pve-1' : kind === 'archive' ? 'main' : 'pve-1',
          ),
        ).toBeInTheDocument();
        fireEvent.click(within(row).getByRole('button', { name: /collapse details/i }));
      }
      expect(api.fetch).toHaveBeenCalledTimes(2);
    },
  );

  it('qualifies each Coverage restore-evidence entry with its own connection, not guest placement', async () => {
    window.history.replaceState({}, '', '/proxmox/backups/coverage');
    mount(() => points());
    const toggle = await screen.findByRole('button', { name: /expand details for same-name/i });
    fireEvent.click(toggle);
    const detail = screen.getByTestId('proxmox-coverage-detail');
    const origins = Array.from(detail.querySelectorAll('[data-backup-source-connection]'));
    expect(origins.map((item) => item.textContent)).toEqual([
      'Connection: pbs-east',
      'Connection: pbs-west',
    ]);
    expect(origins.every((item) => item.classList.contains('wrap-anywhere'))).toBe(true);
    for (const origin of origins) {
      const evidence = origin.closest('tr')!;
      expect(evidence).toHaveTextContent('main / pve-1');
      expect(evidence).toHaveTextContent('Verified');
    }
    expect(screen.getByRole('button', { name: '1 unknown' })).toBeInTheDocument();
    expect(api.fetch).toHaveBeenCalledTimes(2);
  });

  it('keeps a missing source explicit instead of borrowing the guest connection or namespace', async () => {
    mount(() => [identityPBS({ instance: undefined })]);
    const [row] = await inventoryRows();
    expect(connectionOf(row)).toBe('Connection: Unknown connection');
    fireEvent.click(within(row).getByRole('button', { name: /expand details/i }));
    const detail = screen.getByTestId('proxmox-recoverable-detail');
    const connectionRow = within(detail).getByText('Connection').closest('tr')!;
    expect(connectionRow).toHaveTextContent('Not reported');
    expect(connectionRow).not.toHaveTextContent('east');
    expect(connectionRow).not.toHaveTextContent('pve-1');
    expect(detail).toHaveTextContent('main / pve-1');
  });

  it('updates the same point and open detail through ordinary polling without replacing focus', async () => {
    vi.useFakeTimers({
      now: BACKUP_IDENTITY_NOW,
      toFake: ['Date', 'setInterval', 'clearInterval'],
    });
    let current = points();
    mount(() => current);
    const rows = await inventoryRows();
    const row = rows.find((item) => connectionOf(item) === 'Connection: pbs-east')!;
    expect(row).toBeDefined();
    const toggle = within(row).getByRole('button', { name: /expand details/i });
    fireEvent.click(toggle);
    toggle.focus();
    const detail = screen.getByTestId('proxmox-recoverable-detail');
    const origin = row.querySelector('[data-backup-source-connection]');
    current = points('pbs-renamed');
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() => expect(connectionOf(row)).toBe('Connection: pbs-renamed'));
    expect(row.querySelector('[data-backup-source-connection]')).toBe(origin);
    expect(screen.getByTestId('proxmox-recoverable-detail')).toBe(detail);
    expect(within(detail).getByText('pbs-renamed')).toBeInTheDocument();
    expect(within(detail).queryByText('pbs-east')).not.toBeInTheDocument();
    expect(document.activeElement).toBe(toggle);
    expect(api.fetch).toHaveBeenCalledTimes(4);
  });

  it('renders connection text literally, never as markup', async () => {
    const name = '<img data-source-injected src=x>';
    mount(() => [identityPBS({ instance: name })]);
    const [row] = await inventoryRows();
    expect(connectionOf(row)).toBe(`Connection: ${name}`);
    fireEvent.click(within(row).getByRole('button', { name: /expand details/i }));
    expect(
      within(screen.getByTestId('proxmox-recoverable-detail')).getByText(name),
    ).toBeInTheDocument();
    expect(document.querySelector('[data-source-injected]')).toBeNull();
  });
});
