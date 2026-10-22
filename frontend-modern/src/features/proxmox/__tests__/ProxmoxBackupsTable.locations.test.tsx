import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
} from '../__fixtures__/backupIdentity';
import { eventBus } from '@/stores/events';

const { apiFetchMock, apiFetchJSONMock } = vi.hoisted(() => ({
  apiFetchMock: vi.fn(),
  apiFetchJSONMock: vi.fn(),
}));
vi.mock('@/utils/apiClient', () => ({ apiFetch: apiFetchMock, apiFetchJSON: apiFetchJSONMock }));
const response = (data: unknown) =>
  new Response(JSON.stringify({ data }), {
    headers: { 'Content-Type': 'application/json' },
  });
const lower = identityPBS({
  id: 'lower-store',
  datastore: 'main',
  namespace: '',
  backupTime: '2026-10-04T09:00:00Z',
});
const upper = identityPBS({
  id: 'upper-store',
  datastore: 'Main',
  namespace: '',
  backupTime: '2026-10-04T10:00:00Z',
});
let points = [lower, upper];
let files: ReturnType<typeof identityArchive>[] = [];
const open = (location = '', view = 'date') => {
  window.history.replaceState(
    {},
    '',
    `/proxmox/backups/${view}?location=${encodeURIComponent(location)}`,
  );
  return render(() => (
    <Router>
      <Route
        path="/*"
        component={() => (
          <ProxmoxBackupsTable
            emptyIcon={<span />}
            workloads={[identityGuest('east-100', 'east', 'pve-1')]}
          />
        )}
      />
    </Router>
  ));
};
const locations = () =>
  Array.from(document.querySelectorAll('[data-proxmox-backup-row="recoverable"]')).map(
    (row) => row.textContent,
  );
const select = async (label: string) => {
  const option = screen.getByRole('option', {
    name: `Backup location: ${label}`,
  }) as HTMLOptionElement;
  await fireEvent.change(screen.getByRole('combobox', { name: 'Filter' }), {
    target: { value: option.value },
  });
};

beforeEach(() => {
  vi.spyOn(Date, 'now').mockReturnValue(BACKUP_IDENTITY_NOW);
  vi.stubGlobal('scrollTo', vi.fn());
  points = [lower, upper];
  files = [];
  apiFetchMock.mockImplementation((url: string) =>
    Promise.resolve(
      response(
        url.endsWith('/pbs')
          ? { backups: points }
          : { backupTasks: [], storageBackups: files, guestSnapshots: [] },
      ),
    ),
  );
  apiFetchJSONMock.mockResolvedValue({
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
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  apiFetchMock.mockReset();
  apiFetchJSONMock.mockReset();
  window.history.replaceState({}, '', '/');
});

describe('Backups exact location filters', () => {
  it('selects the case-sensitive store through the real filter and retains it in Coverage', async () => {
    open();
    await screen.findByText('pbs-server / Main / (root)');
    await select('pbs-server / main');
    await waitFor(() => expect(locations()).toHaveLength(1));
    expect(locations()[0]).toContain('pbs-server / main / (root)');
    expect(screen.queryByText('pbs-server / Main / (root)')).not.toBeInTheDocument();
    await waitFor(() =>
      expect(new URLSearchParams(window.location.search).get('location')).toBe(
        'backup-location-v2:["pbs","pbs-server","main"]',
      ),
    );
    const saved = new URLSearchParams(window.location.search).get('location');
    await fireEvent.click(screen.getByRole('link', { name: /coverage/i }));
    await screen.findByRole('button', { name: /expand details for east-100/i });
    await fireEvent.click(screen.getByRole('button', { name: /expand details for east-100/i }));
    expect(screen.getByText('pbs-server / main / (root)')).toBeInTheDocument();
    expect(screen.queryByText('pbs-server / Main / (root)')).not.toBeInTheDocument();
    expect(new URLSearchParams(window.location.search).get('location')).toBe(saved);
    expect(apiFetchMock).toHaveBeenCalledTimes(2);
  });

  it('keeps an ambiguous old saved view explicitly empty until an exact location is selected', async () => {
    open('pbs:pbs-server:main');
    await screen.findByText(/saved backup location matches more than one source/);
    expect(locations()).toHaveLength(0);
    const chip = screen.getByRole('button', { name: 'Backup location: Ambiguous saved location' });
    await fireEvent.click(chip);
    await fireEvent.click(screen.getByRole('option', { name: 'pbs-server / Main' }));
    await waitFor(() => expect(locations()).toHaveLength(1));
    expect(locations()[0]).toContain('pbs-server / Main / (root)');
    expect(
      screen.queryByText(/saved backup location matches more than one source/),
    ).not.toBeInTheDocument();
  });

  it('preserves a unique old saved view but withdraws it when refresh reveals a collision', async () => {
    points = [lower];
    open('pbs:pbs-server:main');
    await screen.findByRole('button', { name: 'Backup location: pbs-server / main' });
    expect(locations()).toHaveLength(1);
    points = [lower, upper];
    await fireEvent.click(screen.getByRole('button', { name: 'Refresh backups' }));
    await screen.findByText(/saved backup location matches more than one source/);
    expect(locations()).toHaveLength(0);
    expect(new URLSearchParams(window.location.search).get('location')).toBe('pbs:pbs-server:main');
    await fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));
    await waitFor(() => expect(locations()).toHaveLength(2));
  });

  it('retains store-wide local-node saved views without pooling other stores', async () => {
    points = [];
    files = [
      identityArchive({ id: 'local-a', storageNode: 'node-a' }),
      identityArchive({ id: 'local-b', storageNode: 'node-b' }),
      identityArchive({ id: 'other', storage: 'other' }),
    ];
    open('archive:east:local');
    await screen.findByRole('button', { name: 'Backup location: east / local' });
    expect(locations()).toHaveLength(2);
    expect(locations().every((row) => row?.includes('east / local / node-'))).toBe(true);
    expect(screen.queryByText('east / other')).not.toBeInTheDocument();
  });

  it('explains a missing or hostile saved key without rendering its bytes or selecting all', async () => {
    const { container } = open('<img src=x onerror="alert(1)">');
    await screen.findByText(/saved backup location is not in the current inventory/);
    expect(locations()).toHaveLength(0);
    expect(
      screen.getByRole('button', { name: 'Backup location: Unavailable saved location' }),
    ).toBeInTheDocument();
    expect(container.querySelector('img')).toBeNull();
    expect(container.textContent).not.toContain('<img');
    await fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));
    await waitFor(() => expect(locations()).toHaveLength(2));
  });

  it('does not describe an unresolved saved scope as no backups when inventory is empty', async () => {
    points = [];
    open('pbs:old:store');
    await screen.findByText(/saved backup location is not in the current inventory/);
    expect(screen.getByText('Choose a backup location')).toBeInTheDocument();
    expect(screen.queryByText('No backups yet')).not.toBeInTheDocument();
  });

  it('does not restore a retired source from the query cache on organisation changes', async () => {
    open('backup-location-v2:["pbs","pbs-server","main"]');
    await waitFor(() => expect(locations()).toHaveLength(1));
    points = [identityPBS({ id: 'other-org', instance: 'other', datastore: 'main' })];
    eventBus.emit('org_switched', 'different-org');
    await screen.findByText(/saved backup location is not in the current inventory/);
    expect(locations()).toHaveLength(0);
  });
});
