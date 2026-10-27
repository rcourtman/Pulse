import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { PBSBackup } from '@/types/api';
import { protectionPolicy } from '@/utils/__fixtures__/protectionPosture';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import { BACKUP_IDENTITY_NOW, identityGuest, identityPBS } from '../__fixtures__/backupIdentity';

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn() }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));

const guest = {
  ...identityGuest('vm-kept', 'east', 'pve-1'),
  name: 'kept-guest',
  displayName: 'kept-guest',
};
const point = (fields: Partial<PBSBackup> = {}) => identityPBS({ protected: true, ...fields });
const jsonResponse = (data: unknown) =>
  new Response(JSON.stringify({ data }), { headers: { 'Content-Type': 'application/json' } });

function mount(getPoint: () => PBSBackup) {
  api.fetch.mockImplementation((url: string) =>
    Promise.resolve(
      jsonResponse(
        url === '/api/backups/pbs'
          ? { backups: [getPoint()] }
          : { backupTasks: [], guestSnapshots: [], storageBackups: [] },
      ),
    ),
  );
  // No canonical assessment: a retained/verified file must not invent one.
  api.json.mockResolvedValue({ data: [], policy: protectionPolicy });
  return render(() => (
    <Router>
      <Route
        path="/*"
        component={() => <ProxmoxBackupsTable emptyIcon={<span />} workloads={[guest]} />}
      />
    </Router>
  ));
}

async function restorePointRow() {
  await screen.findAllByText('kept-guest');
  return document.querySelector('[data-proxmox-backup-row="recoverable"]') as HTMLElement;
}

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

describe('prune protection is independent of backup verification', () => {
  it.each([
    [false, 'Unverified', 'bg-amber-100', 'Not verified'],
    [true, 'Verified', 'bg-emerald-100', 'Verified'],
    [undefined, 'Kept', 'bg-blue-100', 'Not reported'],
  ] as const)(
    'keeps the verification result visible when a retained point reports %s',
    async (verified, label, tone, verification) => {
      mount(() => point({ verified }));
      const row = await restorePointRow();
      expect(within(row).getByText(label)).toHaveClass(tone);
      expect(within(row).queryByText('Protected')).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: '1 unknown' })).toBeInTheDocument();
      await fireEvent.click(within(row).getByRole('button', { name: /expand details/i }));
      const detail = screen.getByTestId('proxmox-recoverable-detail');
      expect(within(detail).getByText('Prune protection')).toBeInTheDocument();
      expect(within(detail).getByText('Kept when pruning')).toBeInTheDocument();
      expect(within(detail).getAllByText(verification).length).toBeGreaterThan(0);
      if (verified === undefined) {
        expect(within(detail).queryByText('Verified')).not.toBeInTheDocument();
      }
    },
  );

  it.each([
    [false, 'Unverified', 'bg-amber-100'],
    [true, 'Verified', 'bg-emerald-100'],
  ] as const)('[control] preserves non-retained verification %s', async (verified, label, tone) => {
    mount(() => point({ protected: false, verified }));
    const row = await restorePointRow();
    expect(within(row).getByText(label)).toHaveClass(tone);
  });

  it('[control] does not promote a retained in-progress point to verified', async () => {
    mount(() => point({ inProgress: true, verified: true }));
    const row = await restorePointRow();
    expect(within(row).getByText('Running')).toHaveClass('bg-blue-100');
    expect(within(row).queryByText('Verified')).not.toBeInTheDocument();
  });

  it('finds the visible Kept state and retains the older protected search alias', async () => {
    mount(() => point({ verified: undefined }));
    await restorePointRow();
    const search = screen.getByPlaceholderText(/Search backups by workload/);
    for (const query of ['kept', 'protected']) {
      fireEvent.input(search, { target: { value: query } });
      await waitFor(() => {
        const row = document.querySelector('[data-proxmox-backup-row="recoverable"]');
        expect(row).toHaveTextContent('kept-guest');
      });
    }
    expect(api.fetch).toHaveBeenCalledTimes(2);
  });

  it('updates a retained point and its open drawer without dropping focus', async () => {
    vi.useFakeTimers({
      now: BACKUP_IDENTITY_NOW,
      toFake: ['Date', 'setInterval', 'clearInterval'],
    });
    let current = point({ verified: false });
    mount(() => current);
    const row = await restorePointRow();
    const badge = within(row).getByText('Unverified');
    const disclosure = within(row).getByRole('button', { name: /expand details/i });
    fireEvent.click(disclosure);
    disclosure.focus();
    const panel = screen.getByTestId('proxmox-recoverable-detail');

    current = point({ verified: true });
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() => expect(within(row).getByText('Verified')).toBe(badge));
    expect(badge).toHaveClass('bg-emerald-100');
    expect(screen.getByTestId('proxmox-recoverable-detail')).toBe(panel);
    expect(within(panel).getByText('Kept when pruning')).toBeInTheDocument();
    expect(within(panel).queryByText('Not verified')).not.toBeInTheDocument();
    expect(document.activeElement).toBe(disclosure);
    expect(api.fetch).toHaveBeenCalledTimes(4);
  });
});
