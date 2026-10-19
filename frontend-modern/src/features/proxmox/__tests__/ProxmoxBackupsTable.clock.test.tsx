import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { recoveryDateKeyFromTimestamp } from '@/utils/recoveryDatePresentation';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import {
  identityArchive,
  identityGuest,
  identityPBS,
  identityTask,
} from '../__fixtures__/backupIdentity';

const api = vi.hoisted(() => ({ apiFetch: vi.fn(), apiFetchJSON: vi.fn() }));
vi.mock('@/utils/apiClient', () => api);

// Local calendar dates keep this test independent of the machine's time zone.
const start = new Date(2026, 9, 19, 23, 59, 50).getTime();
const iso = (time: number) => new Date(time).toISOString();
const day = (date: number) => new Date(2026, 9, date, 12).getTime();
const flush = () => vi.advanceTimersByTimeAsync(0);
let pve: unknown;
let pbs: unknown;
let denyPBS = false;

const mount = (view = 'date', query = '') => {
  window.history.replaceState({}, '', `/proxmox/backups/${view}${query}`);
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
const coverage = () =>
  document.querySelector('[data-proxmox-backup-row="coverage"]') as HTMLElement;
const rows = () => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]');
const activityCount = () =>
  screen
    .queryAllByRole('button', { name: /: \d+ backups?(, selected)?$/ })
    .reduce(
      (sum, el) => sum + Number(el.getAttribute('aria-label')!.match(/: (\d+) backups?/)![1]),
      0,
    );

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(start);
  vi.stubGlobal('scrollTo', vi.fn());
  denyPBS = false;
  pve = { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } };
  pbs = { data: { backups: [] } };
  api.apiFetch.mockImplementation((url: string) =>
    Promise.resolve(
      url.endsWith('/pbs') && denyPBS
        ? new Response('', { status: 403 })
        : new Response(JSON.stringify(url.endsWith('/pve') ? pve : pbs)),
    ),
  );
  api.apiFetchJSON.mockResolvedValue({
    data: [
      {
        subjectResourceId: 'east-100',
        state: 'protected',
        freshness: 'current',
        verification: 'verified',
        coverage: 'complete',
        explanation: 'Server-owned fixture verdict.',
        evaluatedAt: iso(start),
        providerStates: [],
        repositoryResourceIds: [],
        evidenceIds: [],
      },
    ],
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
  expect(vi.getTimerCount()).toBe(0);
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  api.apiFetch.mockReset();
  api.apiFetchJSON.mockReset();
  window.history.replaceState({}, '', '/');
});

describe('mounted backup chronology', () => {
  it('rolls Count and Volume into the new local day without rereading inventory or clearing filters', async () => {
    pve = {
      data: {
        backupTasks: [],
        guestSnapshots: [],
        storageBackups: [
          identityArchive({ id: 'old', time: iso(day(13)), size: 2048 }),
          identityArchive({ id: 'recent', time: iso(day(19)), size: 1024 }),
        ],
      },
    };
    mount('date', '?source=archive&q=east-100');
    await flush();
    fireEvent.click(screen.getByRole('button', { name: '7d', exact: true }));
    expect(activityCount()).toBe(2);
    const oldDay = screen.getAllByRole('button', { name: /: 1 backup$/ })[0];
    fireEvent.click(oldDay);
    await flush();
    const selected = recoveryDateKeyFromTimestamp(day(13));
    expect(new URLSearchParams(window.location.search).get('day')).toBe(selected);
    expect(rows()).toHaveLength(1);

    await vi.advanceTimersByTimeAsync(RELATIVE_TIME_TICK_MS);
    expect(activityCount()).toBe(1);
    expect(screen.getAllByRole('button', { name: /: \d+ backups?/ })).toHaveLength(7);
    expect(new URLSearchParams(window.location.search).get('day')).toBe(selected);
    expect(screen.getByRole('button', { name: '7d', exact: true })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(rows()).toHaveLength(1); // Selected historical day is not silently reset.
    fireEvent.click(screen.getByRole('button', { name: 'Clear date filter' }));
    await flush();
    expect(rows()).toHaveLength(2);
    fireEvent.click(screen.getByRole('button', { name: 'Volume', exact: true }));
    expect(screen.getByRole('button', { name: /: 1.00 KB$/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /: 2.00 KB$/ })).not.toBeInTheDocument();
    expect(new URLSearchParams(window.location.search).get('source')).toBe('archive');
    expect(new URLSearchParams(window.location.search).get('q')).toBe('east-100');
    expect(api.apiFetch).toHaveBeenCalledTimes(2);
  });

  it('reconciles future backup/job times in both clock directions without renewing observations or rating protection locally', async () => {
    pve = {
      data: {
        guestSnapshots: [],
        storageBackups: [identityArchive({ time: iso(start - 120_000) })],
        backupTasks: [
          identityTask({
            startTime: iso(start + 5_000),
            endTime: iso(start + 10_000),
            observedAt: iso(start - 60_000),
          }),
        ],
      },
    };
    pbs = { data: { backups: [identityPBS({ backupTime: iso(start + 5_000) })] } };
    mount('coverage');
    await flush();
    expect(coverage()).toHaveTextContent('Protected');
    expect(within(coverage()).getByLabelText(/Unknown age/)).toBeInTheDocument();
    expect(coverage()).toHaveTextContent('Unknown');
    fireEvent.click(within(coverage()).getByRole('button', { name: /Expand/ }));
    const detail = document.querySelector('[data-inline-detail-for="resource:east-100"]')!;
    expect(detail).toHaveTextContent('Latest backup job unknown');

    await vi.advanceTimersByTimeAsync(RELATIVE_TIME_TICK_MS);
    expect(within(coverage()).queryByLabelText(/Unknown age/)).not.toBeInTheDocument();
    expect(coverage()).toHaveTextContent('OK');
    expect(detail).not.toHaveTextContent('Latest backup job unknown');
    expect(detail).toHaveTextContent('5s');
    expect(detail.querySelector('[data-backup-task-observed] time')).toHaveAttribute(
      'datetime',
      iso(start - 60_000),
    );
    expect(coverage()).toHaveTextContent('Protected');
    expect(detail).toHaveTextContent('Server-owned fixture verdict.');

    vi.setSystemTime(start - 300_000);
    await vi.advanceTimersByTimeAsync(RELATIVE_TIME_TICK_MS);
    expect(within(coverage()).getByLabelText(/Unknown age/)).toBeInTheDocument();
    expect(detail).toHaveTextContent('Latest backup job unknown');
    expect(coverage()).not.toHaveTextContent('OK');
    expect(coverage()).toHaveTextContent('Protected');

    vi.setSystemTime(start + 60_000);
    await vi.advanceTimersByTimeAsync(RELATIVE_TIME_TICK_MS);
    expect(within(coverage()).queryByLabelText(/Unknown age/)).not.toBeInTheDocument();
    expect(coverage()).toHaveTextContent('OK');
    expect(document.querySelector('[data-inline-detail-for="resource:east-100"]')).toBe(detail);
    expect(detail.querySelector('[data-backup-task-observed] time')).toHaveAttribute(
      'datetime',
      iso(start - 60_000),
    );
    expect(api.apiFetch).toHaveBeenCalledTimes(2);
  });

  it('never re-admits a denied source when time changes, and accepts a newly read point between clock ticks', async () => {
    pve = {
      data: {
        backupTasks: [],
        guestSnapshots: [],
        storageBackups: [identityArchive({ time: iso(start - 60_000) })],
      },
    };
    pbs = { data: { backups: [identityPBS({ backupTime: iso(start - 30_000) })] } };
    mount();
    await flush();
    expect(activityCount()).toBe(2);
    denyPBS = true;
    fireEvent.click(screen.getByRole('button', { name: 'Refresh backups', exact: true }));
    await flush();
    expect(activityCount()).toBe(1);
    expect(rows()).toHaveLength(1);
    vi.setSystemTime(start + 5_000);
    await vi.advanceTimersByTimeAsync(RELATIVE_TIME_TICK_MS);
    expect(activityCount()).toBe(1);
    expect(rows()).toHaveLength(1);
    const reads = api.apiFetch.mock.calls.length;
    vi.setSystemTime(Date.now() + 5_000); // New timestamp since the latest shared tick.
    pve = {
      data: {
        backupTasks: [],
        guestSnapshots: [],
        storageBackups: [identityArchive({ time: iso(Date.now()) })],
      },
    };
    fireEvent.click(screen.getByRole('button', { name: 'Refresh backups', exact: true }));
    await flush();
    expect(activityCount()).toBe(1);
    expect(document.querySelectorAll('[aria-label^="Unknown age"]')).toHaveLength(0);
    expect(api.apiFetch).toHaveBeenCalledTimes(reads + 2);
    expect(screen.getByText(/PBS backup inventory is unavailable/)).toHaveTextContent(
      'Access denied.',
    );
  });
});
