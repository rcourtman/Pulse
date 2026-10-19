import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { eventBus } from '@/stores/events';

const apiFetch = vi.hoisted(() => vi.fn());
vi.mock('@/utils/apiClient', () => ({
  apiFetch,
  apiFetchJSON: vi.fn(),
}));

// Leave enough time for the API client's existing two-minute Retry-After wait.
// This is one whole-read budget, not a timer restarted for headers and body.
const READ_LIMIT_MS = 150_000;
const emptyPVE = { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } };
const emptyPBS = { data: { backups: [] } };
const pbs = {
  data: {
    backups: [
      {
        id: 'pbs/main/ct/112',
        vmid: '112',
        backupType: 'ct',
        backupTime: '2026-10-19T01:00:00Z',
        instance: 'pbs',
        datastore: 'main',
        files: ['index.json.blob'],
      },
    ],
  },
};
const json = (payload: unknown) => new Response(JSON.stringify(payload));
const flush = () => vi.advanceTimersByTimeAsync(0);
const mount = () =>
  render(() => (
    <Router>
      <Route path="/*" component={() => <ProxmoxBackupsTable emptyIcon={<span />} />} />
    </Router>
  ));
const notice = (source: string) =>
  screen
    .getAllByRole('status')
    .find((node) => node.textContent?.includes(`${source} backup inventory is unavailable`));
const sourceCalls = (source: string) =>
  apiFetch.mock.calls.filter(([url]) => url === `/api/backups/${source}`);

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.useRealTimers();
  vi.restoreAllMocks();
  apiFetch.mockReset();
  window.history.replaceState({}, '', '/');
});

describe('Proxmox backup inventory read deadline', () => {
  it.each(['pve', 'pbs'] as const)(
    'releases a stalled %s header read, retires late data and permits only explicit retry',
    async (source) => {
      vi.useFakeTimers();
      let finish!: (response: Response) => void;
      apiFetch.mockImplementation((url: string) => {
        if (url === `/api/backups/${source}` && sourceCalls(source).length === 1) {
          // Deliberately ignores abort, as a late client/fixture may do.
          return new Promise<Response>((resolve) => (finish = resolve));
        }
        return Promise.resolve(json(url.endsWith('/pve') ? emptyPVE : emptyPBS));
      });
      mount();
      await flush();
      await vi.advanceTimersByTimeAsync(READ_LIMIT_MS - 1);
      expect(notice(source.toUpperCase())).toBeUndefined();
      expect(screen.getByRole('button', { name: 'Reading backups…' })).toHaveAttribute(
        'aria-disabled',
        'true',
      );

      await vi.advanceTimersByTimeAsync(1);
      expect(notice(source.toUpperCase())).toHaveTextContent(
        'The inventory read timed out. Missing restore points do not mean no backups exist.',
      );
      expect(sourceCalls(source)[0][1].signal.aborted).toBe(true);
      expect(screen.getByRole('button', { name: 'Refresh backups' })).toHaveAttribute(
        'aria-disabled',
        'false',
      );
      expect(screen.queryByText('No backups yet')).not.toBeInTheDocument();
      expect(sourceCalls(source)).toHaveLength(1);

      // A late answer must not clear the warning or invent a new inventory.
      finish(json(source === 'pve' ? emptyPVE : pbs));
      await flush();
      expect(notice(source.toUpperCase())).toBeInTheDocument();
      expect(document.querySelectorAll('[data-proxmox-backup-row="recoverable"]')).toHaveLength(0);
      fireEvent.click(
        screen.getByRole('button', { name: `Retry ${source.toUpperCase()} inventory` }),
      );
      await flush();
      expect(notice(source.toUpperCase())).toBeUndefined();
      expect(sourceCalls(source)).toHaveLength(2);
      expect(sourceCalls(source === 'pve' ? 'pbs' : 'pve')).toHaveLength(1);
      expect(screen.getByText('No backups yet')).toBeInTheDocument();
    },
  );

  it.each(['pve', 'pbs'] as const)(
    'counts time awaiting %s headers against the same body-read deadline',
    async (source) => {
      vi.useFakeTimers();
      let headers!: (response: Response) => void;
      let body!: (payload: unknown) => void;
      apiFetch.mockImplementation((url: string) =>
        url.endsWith(`/${source}`)
          ? new Promise<Response>((resolve) => (headers = resolve))
          : Promise.resolve(json(url.endsWith('/pve') ? emptyPVE : emptyPBS)),
      );
      mount();
      await flush();
      await vi.advanceTimersByTimeAsync(120_000);
      const response = json({});
      vi.spyOn(response, 'json').mockImplementation(
        () => new Promise((resolve) => (body = resolve)),
      );
      headers(response);
      await flush();
      await vi.advanceTimersByTimeAsync(30_000);
      expect(notice(source.toUpperCase())).toHaveTextContent('The inventory read timed out.');
      body(source === 'pve' ? emptyPVE : pbs);
      await flush();
      expect(notice(source.toUpperCase())).toBeInTheDocument();
      expect(sourceCalls(source)).toHaveLength(1);
    },
  );

  it('withdraws timed-out source points while keeping independent evidence through retry and denial', async () => {
    vi.useFakeTimers();
    let pbsRead = 0;
    let finishRetry!: (response: Response) => void;
    apiFetch.mockImplementation((url: string) => {
      if (url.endsWith('/pve'))
        return Promise.resolve(
          json({
            data: {
              ...emptyPVE.data,
              storageBackups: [
                { id: 'archive', vmid: 112, type: 'ct', time: '2026-10-19T01:00:00Z' },
              ],
            },
          }),
        );
      pbsRead++;
      if (pbsRead === 1) return Promise.resolve(json(pbs));
      if (pbsRead === 2) return new Promise(() => {});
      return new Promise((resolve) => (finishRetry = resolve));
    });
    mount();
    await flush();
    const rows = () => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]');
    expect(rows()).toHaveLength(2);
    fireEvent.click(screen.getByRole('button', { name: 'Refresh backups' }));
    await vi.advanceTimersByTimeAsync(READ_LIMIT_MS);
    expect(notice('PBS')).toBeInTheDocument();
    expect(rows()).toHaveLength(1);
    fireEvent.click(screen.getByRole('button', { name: 'Retry PBS inventory' }));
    await flush();
    expect(within(notice('PBS')!).getByRole('button')).toBeDisabled();
    expect(rows()).toHaveLength(1);
    finishRetry(new Response('', { status: 403 }));
    await flush();
    expect(rows()).toHaveLength(1);
    expect(notice('PBS')).toHaveTextContent('Access denied.');
    expect(notice('PBS')).not.toHaveTextContent('timed out');
  });

  it('clears deadline timers and rejects prior-org data when the source owner changes or unmounts', async () => {
    vi.useFakeTimers();
    const pending: { signal: AbortSignal; finish: (response: Response) => void }[] = [];
    apiFetch.mockImplementation(
      (_url: string, { signal }: { signal: AbortSignal }) =>
        new Promise<Response>((finish) => pending.push({ signal, finish })),
    );
    const view = mount();
    await flush();
    expect(pending).toHaveLength(2);
    eventBus.emit('org_switched', 'next-org');
    await flush();
    expect(pending).toHaveLength(4);
    expect(pending.slice(0, 2).every(({ signal }) => signal.aborted)).toBe(true);
    pending[0].finish(json(emptyPVE));
    pending[1].finish(json(pbs));
    await flush();
    expect(document.querySelectorAll('[data-proxmox-backup-row="recoverable"]')).toHaveLength(0);
    view.unmount();
    expect(pending.every(({ signal }) => signal.aborted)).toBe(true);
    await flush();
    expect(vi.getTimerCount()).toBe(0);
  });
});
