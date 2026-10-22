import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import { identityArchive, identityGuest } from '../__fixtures__/backupIdentity';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn() }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));
const nodeKey = (connection: string, node: string) =>
  `backup-node-v1:${JSON.stringify([connection, node])}`;
const east = identityGuest('east-vm', 'east', 'pve-1', { nodeDisplayName: 'Shared' });
const west = identityGuest('west-vm', 'west', 'pve-1', { nodeDisplayName: 'Shared' });
const archives = [
  identityArchive({ id: 'east-file', instance: 'east', node: 'pve-1' }),
  identityArchive({ id: 'west-file', instance: 'west', node: 'pve-1' }),
];
const rows = (view = 'coverage') => [
  ...document.querySelectorAll(`[data-proxmox-backup-row="${view}"]`),
];
const mount = (guests = [east, west]) => {
  const [workloads, setWorkloads] = createSignal(guests);
  render(() => (
    <Router>
      <Route
        path="/*"
        component={() => <ProxmoxBackupsTable emptyIcon={<span />} workloads={workloads()} />}
      />
    </Router>
  ));
  return setWorkloads;
};
const url = (view: string, node = '') =>
  window.history.replaceState(
    {},
    '',
    `/proxmox/backups/${view}?keep=node-proof${node ? `&node=${encodeURIComponent(node)}` : ''}`,
  );

beforeEach(() => {
  resetCreateNonSuspendingQueryCacheForTest();
  api.fetch.mockImplementation((url: string) =>
    Promise.resolve(
      new Response(
        JSON.stringify({
          data: url.endsWith('/pve')
            ? { storageBackups: archives, guestSnapshots: [], backupTasks: [] }
            : { backups: [] },
        }),
        { status: 200 },
      ),
    ),
  );
  api.json.mockResolvedValue({
    data: [],
    policy: {
      freshnessWindowSeconds: 604800,
      verificationWindowSeconds: 604800,
      requireVerification: true,
    },
  });
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.clearAllMocks();
  window.history.replaceState({}, '', '/');
});

describe('source-scoped backup Node filter', () => {
  it.each(['coverage', 'date'])(
    'refuses an ambiguous old display-name URL in %s instead of pooling sources',
    async (view) => {
      url(view, 'Shared');
      mount();
      await screen.findByText(/The saved node matches more than one/);
      expect(rows(view === 'date' ? 'recoverable' : 'coverage')).toHaveLength(0);
      expect(
        screen.getByRole('button', { name: 'Node: Ambiguous saved node' }),
      ).toBeInTheDocument();
      expect(window.location.search).toContain('node=Shared');
    },
  );
  it.each(['coverage', 'date'])(
    'keeps an exact connection/native-node selection isolated in %s',
    async (view) => {
      url(view, nodeKey('east', 'pve-1'));
      mount();
      await waitFor(() =>
        expect(rows(view === 'date' ? 'recoverable' : 'coverage')).toHaveLength(1),
      );
      expect(rows(view === 'date' ? 'recoverable' : 'coverage')[0]).toHaveTextContent('east-vm');
      expect(rows(view === 'date' ? 'recoverable' : 'coverage')[0]).not.toHaveTextContent(
        'west-vm',
      );
      expect(
        screen.getByRole('button', { name: 'Node: east / Shared (pve-1)' }),
      ).toBeInTheDocument();
    },
  );
  it('offers distinct source labels and keeps the same exact selection through display renames and navigation', async () => {
    url('coverage');
    const set = mount();
    await waitFor(() => expect(rows()).toHaveLength(2));
    const filter = screen.getByRole('combobox', { name: 'Filter' }) as HTMLSelectElement;
    const options = [...filter.options].filter((option) => option.textContent?.startsWith('Node:'));
    expect(options.map((option) => option.textContent)).toEqual([
      'Node: east / Shared (pve-1)',
      'Node: west / Shared (pve-1)',
    ]);
    await fireEvent.change(filter, { target: { value: options[0].value } });
    await waitFor(() => expect(rows()).toHaveLength(1));
    set([west, { ...east, proxmox: { ...east.proxmox!, nodeDisplayName: 'Renamed' } }]);
    await screen.findByRole('button', { name: 'Node: east / Renamed (pve-1)' });
    expect(rows()[0]).toHaveTextContent('east-vm');
    await fireEvent.click(screen.getByRole('link', { name: 'By date' }));
    await waitFor(() => expect(rows('recoverable')).toHaveLength(1));
    expect(rows('recoverable')[0]).toHaveTextContent('east-vm');
    expect(new URLSearchParams(window.location.search).get('node')).toBe(nodeKey('east', 'pve-1'));
    expect(new URLSearchParams(window.location.search).get('keep')).toBe('node-proof');
  });
  it('retains a unique old name bookmark and clears the filter without clearing unrelated URL state', async () => {
    url('coverage', 'Shared');
    mount([east]);
    await waitFor(() => expect(rows()).toHaveLength(1));
    await fireEvent.click(screen.getByRole('button', { name: 'Node: east / Shared (pve-1)' }));
    const selected = within(screen.getByRole('listbox', { name: 'Node' })).getByRole('option', {
      name: 'east / Shared (pve-1)',
    });
    expect(selected).toHaveAttribute('aria-selected', 'true');
    await fireEvent.keyDown(screen.getByRole('textbox', { name: 'Filter Node values' }), {
      key: 'Enter',
    });
    expect(new URLSearchParams(window.location.search).get('node')).toBe(nodeKey('east', 'pve-1'));
    await fireEvent.click(screen.getByRole('button', { name: 'Remove Node filter' }));
    await waitFor(() =>
      expect(new URLSearchParams(window.location.search).has('node')).toBe(false),
    );
    expect(new URLSearchParams(window.location.search).get('keep')).toBe('node-proof');
  });
  it.each(['coverage', 'date'])(
    'does not call an unavailable saved node an empty backup estate in %s',
    async (view) => {
      api.fetch.mockImplementation(() =>
        Promise.resolve(
          new Response(
            JSON.stringify({
              data: { storageBackups: [], guestSnapshots: [], backupTasks: [], backups: [] },
            }),
            { status: 200 },
          ),
        ),
      );
      url(view, nodeKey('gone', 'pve-1'));
      mount([]);
      await screen.findByText('Choose a node');
      expect(
        screen.getByText(/The saved node is not in the current inventory/),
      ).toBeInTheDocument();
      expect(screen.queryByText('No backups yet')).not.toBeInTheDocument();
    },
  );

  it('leaves all guests and points visible with no Node filter', async () => {
    url('date');
    mount();
    await waitFor(() => expect(rows('recoverable')).toHaveLength(2));
    expect(screen.queryByText(/The saved node/)).not.toBeInTheDocument();
  });
});
