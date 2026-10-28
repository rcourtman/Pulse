import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { protectionPolicy, protectionPosture } from '@/utils/__fixtures__/protectionPosture';
import type { ProtectionPosture } from '@/types/recovery';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import { identityArchive, identityGuest } from '../__fixtures__/backupIdentity';

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn(), width: 1200 }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => {}, width: () => api.width }),
}));

const guest = identityGuest('east-100', 'east', 'pve-1');
const evaluatedAt = '2026-10-28T17:45:12Z';
const pointAt = '2026-10-08T04:30:00Z';
const verifiedAt = '2026-10-09T05:00:00+02:00';
const freshArchiveAt = '2026-10-28T16:00:00Z';
const assessment = (extra: Partial<ProtectionPosture> = {}): ProtectionPosture => ({
  ...protectionPosture(guest.id),
  state: 'attention',
  freshness: 'stale',
  lastSuccessfulPointAt: pointAt,
  lastVerifiedAt: verifiedAt,
  explanation: 'The recorded successful point is older than the freshness window.',
  evaluatedAt,
  ...extra,
});
const reply = (data: ProtectionPosture[]) => ({ data, policy: protectionPolicy });
function mount(data = [assessment()]) {
  api.json.mockResolvedValue(reply(data));
  api.fetch.mockImplementation((url: string) =>
    Promise.resolve(
      new Response(
        JSON.stringify({
          data:
            url === '/api/backups/pbs'
              ? { backups: [] }
              : {
                  storageBackups: [
                    identityArchive({
                      time: freshArchiveAt,
                      ctime: Date.parse(freshArchiveAt) / 1000,
                    }),
                  ],
                  guestSnapshots: [],
                  backupTasks: [],
                },
        }),
      ),
    ),
  );
  render(() => (
    <Router>
      <Route
        path="/*"
        component={() => <ProxmoxBackupsTable emptyIcon={<span />} workloads={[guest]} />}
      />
    </Router>
  ));
}
async function open() {
  fireEvent.click(await screen.findByRole('button', { name: /Expand details for east-100/ }));
  const panel = await screen.findByTestId('proxmox-coverage-detail');
  await within(panel).findByText('Protection assessment', { exact: true });
  return panel;
}
const valueRow = (panel: HTMLElement, label: string) =>
  within(panel).getByText(label, { exact: true }).closest('tr')!;

beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers({
    now: Date.parse('2026-10-28T18:00:00Z'),
    toFake: ['Date', 'setInterval', 'clearInterval'],
  });
  resetCreateNonSuspendingQueryCacheForTest();
  api.width = 1200;
  window.history.replaceState({}, '', '/proxmox/backups/coverage');
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.useRealTimers();
});

describe('Coverage server assessment clocks through the parsed consumer', () => {
  it.each([1200, 320])(
    'keeps evaluation, recorded point and verification distinct at %ipx',
    async (width) => {
      api.width = width;
      mount();
      const panel = await open();
      for (const [label, instant] of [
        ['Evaluated', evaluatedAt],
        ['Latest successful point', pointAt],
        ['Last verified', verifiedAt],
      ]) {
        const time = valueRow(panel, label).querySelector('time')!;
        expect(time).toHaveAttribute('datetime', instant);
        expect(time).toHaveTextContent('UTC');
        expect(time).toHaveClass('whitespace-normal');
      }
      expect(valueRow(panel, 'Last verified')).toHaveTextContent('03:00:00');
      expect(panel).toHaveTextContent('Evaluation time is not a backup completion time.');
      expect(panel).toHaveTextContent('They do not verify a restore or guest responsiveness.');
      expect(panel.querySelector(`time[datetime="${freshArchiveAt}"]`)).toBeNull();
      expect(panel).toHaveTextContent('Attention');
      expect(panel).toHaveTextContent('PVE connection');
      expect(api.fetch).toHaveBeenCalledTimes(2);
      expect(api.json).toHaveBeenCalledTimes(1);
    },
  );

  it.each([undefined, null])(
    'does not replace an unreported point or verification with an artifact or evaluation (%s)',
    async (missing) => {
      mount([assessment({ lastSuccessfulPointAt: missing, lastVerifiedAt: missing })]);
      const panel = await open();
      for (const label of ['Latest successful point', 'Last verified']) {
        expect(valueRow(panel, label)).toHaveTextContent('Not reported');
        expect(valueRow(panel, label).querySelector('time')).toBeNull();
      }
      expect(panel.querySelectorAll('time')).toHaveLength(1);
      expect(panel).toHaveTextContent('Attention');
    },
  );

  it('keeps assessment clocks workload-wide when a location filter is selected and cleared', async () => {
    mount();
    const panel = await open();
    fireEvent.click(screen.getByRole('button', { name: 'Backup location: All locations' }));
    fireEvent.click(
      within(screen.getByRole('listbox', { name: 'Backup location' })).getByRole('option', {
        name: 'PVE file · east / local',
      }),
    );
    await screen.findByRole('note', { name: 'Restore evidence scope' });
    expect(valueRow(panel, 'Latest successful point').querySelector('time')).toHaveAttribute(
      'datetime',
      pointAt,
    );
    expect(panel).toHaveTextContent('independent of the current list and its filters');
    fireEvent.click(screen.getByRole('button', { name: 'Remove Backup location filter' }));
    await waitFor(() =>
      expect(screen.queryByRole('note', { name: 'Restore evidence scope' })).toBeNull(),
    );
    expect(valueRow(panel, 'Latest successful point').querySelector('time')).toHaveAttribute(
      'datetime',
      pointAt,
    );
    expect(screen.getByTestId('proxmox-coverage-detail')).toBe(panel);
    expect(api.fetch).toHaveBeenCalledTimes(2);
    expect(api.json).toHaveBeenCalledTimes(1);
  });

  it('updates the open assessment on ordinary polling and withdraws it on a failed read, then restores it with an isolated retry', async () => {
    mount();
    const panel = await open();
    const collapse = within(panel).getByRole('button', { name: /Collapse east-100 details/ });
    collapse.focus();
    const next = '2026-10-28T17:50:00Z';
    api.json.mockResolvedValue(reply([assessment({ evaluatedAt: next })]));
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() =>
      expect(valueRow(panel, 'Evaluated').querySelector('time')).toHaveAttribute('datetime', next),
    );
    expect(valueRow(panel, 'Latest successful point').querySelector('time')).toHaveAttribute(
      'datetime',
      pointAt,
    );
    expect(screen.getByTestId('proxmox-coverage-detail')).toBe(panel);
    expect(collapse).toHaveFocus();

    api.json.mockRejectedValue(new Error('synthetic unavailable assessment'));
    await vi.advanceTimersByTimeAsync(30_000);
    await screen.findByText(/Protection posture is unavailable/);
    expect(within(panel).queryByText('Protection assessment', { exact: true })).toBeNull();
    expect(panel.querySelectorAll('time')).toHaveLength(0);
    expect(panel).toHaveTextContent('Unknown');
    expect(collapse).toHaveFocus();

    const inventoryReads = api.fetch.mock.calls.length;
    api.json.mockResolvedValue(reply([assessment()]));
    fireEvent.click(screen.getByRole('button', { name: 'Retry protection posture' }));
    await within(panel).findByText('Protection assessment', { exact: true });
    expect(valueRow(panel, 'Evaluated').querySelector('time')).toHaveAttribute(
      'datetime',
      evaluatedAt,
    );
    expect(api.fetch).toHaveBeenCalledTimes(inventoryReads);
  });

  it.each(['2026-09-31T10:00:00Z', '2026-10-28T17:45:12', '<img src=x onerror=alert(1)>'])(
    'never renders a malformed reported assessment time (%s)',
    async (invalid) => {
      mount([assessment({ evaluatedAt: invalid })]);
      await screen.findByText(/Protection posture is unavailable/);
      fireEvent.click(await screen.findByRole('button', { name: /Expand details for east-100/ }));
      const panel = await screen.findByTestId('proxmox-coverage-detail');
      expect(within(panel).queryByText('Protection assessment', { exact: true })).toBeNull();
      expect(panel.querySelector('time')).toBeNull();
      expect(panel.querySelector('img')).toBeNull();
      expect(panel).toHaveTextContent('Unknown');
    },
  );
});
