import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ReplicationJob } from '@/types/api';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import {
  ProxmoxReplicationTable,
  REPLICATION_MOBILE_COLUMNS,
  REPLICATION_MOBILE_COLUMN_WIDTHS,
  compactReplicationNextSyncText,
  fetchReplicationJobs,
  formatMobileReplicationGuestLabel,
} from '../ProxmoxReplicationTable';
import replicationTableSource from '../ProxmoxReplicationTable.tsx?raw';

const replicationJob = (overrides: Partial<ReplicationJob> = {}): ReplicationJob => ({
  id: 'replication-100',
  instance: 'pve',
  jobId: '100-0',
  guestId: 100,
  guestName: 'web',
  sourceNode: 'pve-a',
  targetNode: 'pve-b',
  schedule: '*/15',
  enabled: true,
  lastSyncStatus: 'ok',
  lastSyncUnix: 1_700_000_000,
  lastSyncDurationSeconds: 125,
  failCount: 0,
  ...overrides,
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('ProxmoxReplicationTable', () => {
  const table = (jobs: ReplicationJob[]) => (
    <ProxmoxReplicationTable
      jobs={jobs}
      error={undefined}
      onRetry={() => undefined}
      emptyIcon={<span />}
      emptyTitle="No replication jobs"
      emptyDescription="Replication jobs appear here."
    />
  );
  const jobRow = () => screen.getByRole('cell', { name: '100 (web)', exact: true }).closest('tr')!;

  it.each([
    ['ok', { lastSyncStatus: 'ok' }, 'OK'],
    ['success alias', { lastSyncStatus: ' success ' }, 'OK'],
    ['completed alias', { lastSyncStatus: 'COMPLETED' }, 'OK'],
    ['zero-count error', { lastSyncStatus: 'ok', error: 'Sync failed', failCount: 0 }, 'Failed'],
    ['error without result', { lastSyncStatus: undefined, error: 'Sync failed' }, 'Failed'],
    ['failed attempts override ok', { lastSyncStatus: 'ok', failCount: 2 }, 'Failed'],
    ['failure result', { lastSyncStatus: ' Failed ' }, 'Failed'],
    ['error result', { lastSyncStatus: 'ERROR' }, 'Failed'],
    ['missing result', { lastSyncStatus: undefined }, 'Unknown'],
    ['empty result', { lastSyncStatus: '' }, 'Unknown'],
    ['blank result', { lastSyncStatus: '  ' }, 'Unknown'],
    ['future result', { lastSyncStatus: 'provider-future-state' }, 'Unknown'],
    ['running is not pending', { lastSyncStatus: 'running' }, 'Unknown'],
    ['explicit pending', { lastSyncStatus: ' PENDING ' }, 'Pending'],
    ['blank error is not failure', { lastSyncStatus: 'ok', error: '  ' }, 'OK'],
    [
      'disabled retains history',
      { enabled: false, lastSyncStatus: 'error', error: 'Old failure' },
      'Disabled',
    ],
  ] satisfies [string, Partial<ReplicationJob>, string][])(
    'interprets %s without inventing overall health or a pending operation',
    (_name, change, expected) => {
      render(() => table([replicationJob(change)]));
      expect(within(jobRow()).getByText(expected, { exact: true })).toBeVisible();
      expect(within(jobRow()).queryByText('Healthy', { exact: true })).toBeNull();
    },
  );

  it('keeps an absent result unreported despite a retained sync timestamp', async () => {
    render(() => table([replicationJob({ lastSyncStatus: undefined })]));
    await fireEvent.click(
      screen.getByRole('button', { name: 'Expand details for replication job 100-0' }),
    );
    expect(
      screen.getByText('Reported last result', { selector: 'dt' }).nextElementSibling,
    ).toHaveTextContent('Not reported');
    expect(within(jobRow()).getByText('Unknown', { exact: true })).toBeVisible();
  });

  it('keeps unknown counts and filtering aligned with the same live verdict', async () => {
    const [jobs, setJobs] = createSignal([
      replicationJob({ lastSyncStatus: undefined }),
      replicationJob({ id: 'other', jobId: '101-0', guestId: 101, guestName: 'other' }),
      replicationJob({
        id: 'error',
        jobId: '102-0',
        guestId: 102,
        guestName: 'error',
        error: 'Sync failed',
      }),
    ]);
    render(() => (
      <ProxmoxReplicationTable
        jobs={jobs()}
        error={undefined}
        onRetry={() => undefined}
        emptyIcon={<span />}
        emptyTitle="No replication jobs"
        emptyDescription="Replication jobs appear here."
      />
    ));
    const unknown = screen.getByRole('button', { name: 'Unknown, 1' });
    await fireEvent.click(unknown);
    expect(unknown).toHaveAttribute('aria-pressed', 'true');
    expect(within(jobRow()).getByText('Unknown', { exact: true })).toBeVisible();
    expect(screen.queryByText('101 (other)')).toBeNull();
    expect(screen.queryByText('102 (error)')).toBeNull();
    setJobs([replicationJob(), jobs()[1], jobs()[2]]);
    expect(screen.getByRole('button', { name: 'Unknown, 0' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByText('No replication jobs match current filters')).toBeVisible();
    await fireEvent.click(screen.getByRole('button', { name: 'OK, 2' }));
    expect(within(jobRow()).getByText('OK', { exact: true })).toBeVisible();
    expect(screen.getByText('101 (other)')).toBeVisible();
    expect(screen.queryByText('102 (error)')).toBeNull();
    await fireEvent.click(screen.getByRole('button', { name: 'Failed, 1' }));
    expect(screen.getByText('102 (error)')).toBeVisible();
    expect(screen.queryByText('100 (web)')).toBeNull();
  });

  it('explains unknown outcomes and preserves the plain-text provider result without hover', async () => {
    const raw = '<b>provider-future-state</b>';
    render(() => table([replicationJob({ lastSyncStatus: raw })]));
    expect(screen.getByText(/not a restore check or current guest health/)).toBeVisible();
    await fireEvent.click(
      screen.getByRole('button', { name: 'Expand details for replication job 100-0' }),
    );
    const result = screen.getByText('Reported last result', { selector: 'dt' }).nextElementSibling!;
    expect(result).toHaveTextContent(raw);
    expect(result.querySelector('b')).toBeNull();
    expect(screen.getByText(/This does not mean a sync is pending/)).toBeVisible();
  });

  it('does not lose the error behind an OK result and zero failure count', async () => {
    render(() => table([replicationJob({ error: 'Source snapshot unavailable', failCount: 0 })]));
    expect(within(jobRow()).getByText('Failed', { exact: true })).toBeVisible();
    await fireEvent.click(
      screen.getByRole('button', { name: 'Expand details for replication job 100-0' }),
    );
    expect(document.querySelector('[data-replication-job-error]')).toHaveTextContent(
      'Source snapshot unavailable',
    );
    expect(screen.getByText(/previous successful sync does not override/)).toBeVisible();
    expect(
      screen.getByText('Reported last result', { selector: 'dt' }).nextElementSibling,
    ).toHaveTextContent('ok');
  });

  it('uses five readable columns and compact time direction across phone widths', () => {
    expect(REPLICATION_MOBILE_COLUMNS).toEqual([
      'guest',
      'status',
      'route',
      'lastSync',
      'nextSync',
    ]);
    expect(REPLICATION_MOBILE_COLUMN_WIDTHS).toEqual({
      guest: 40,
      status: 16,
      route: 21.5,
      lastSync: 9.5,
      nextSync: 13,
    });
    expect(compactReplicationNextSyncText('34m overdue', 'overdue')).toBe('-34m');
    expect(compactReplicationNextSyncText('in 8m', 'normal')).toBe('8m');
    expect(formatMobileReplicationGuestLabel(replicationJob())).toBe('100 web');
  });

  it('removes phone tracking and excess padding from the tight last-sync column', () => {
    expect(replicationTableSource).toContain('px-[2px]! tracking-normal!');
  });

  it('renders replication duration cells through the shared duration format', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_700_000_300_000);

    render(() => (
      <ProxmoxReplicationTable
        jobs={[replicationJob()]}
        error={undefined}
        onRetry={() => undefined}
        emptyIcon={<span />}
        emptyTitle="No replication jobs"
        emptyDescription="Replication jobs appear here."
      />
    ));

    expect(screen.getByText('100 (web)')).toBeInTheDocument();
    expect(screen.getByText('100 (web)').closest('tr')).toHaveClass('h-8');
    expect(screen.getByText('2m 5s')).toBeInTheDocument();
  });

  it('preserves explicit replication duration labels from the backend', () => {
    render(() => (
      <ProxmoxReplicationTable
        jobs={[
          replicationJob({
            lastSyncDurationSeconds: undefined,
            lastSyncDurationHuman: 'backend duration',
          }),
        ]}
        error={undefined}
        onRetry={() => undefined}
        emptyIcon={<span />}
        emptyTitle="No replication jobs"
        emptyDescription="Replication jobs appear here."
      />
    ));

    expect(screen.getByText('backend duration')).toBeInTheDocument();
  });

  it('reveals the whole error from the row disclosure at desktop width', async () => {
    const error =
      "command 'zfs send -- rpool/data/vm-100-disk-0@__replicate_100-0__' failed: exit code 255";
    render(() => (
      <ProxmoxReplicationTable
        jobs={[replicationJob({ lastSyncStatus: 'error', failCount: 3, error })]}
        error={undefined}
        onRetry={() => undefined}
        emptyIcon={<span />}
        emptyTitle="No replication jobs"
        emptyDescription="Replication jobs appear here."
      />
    ));

    expect(document.querySelector('[data-replication-job-error]')).toBeNull();
    await fireEvent.click(
      screen.getByRole('button', { name: 'Expand details for replication job 100-0' }),
    );

    const detail = document.querySelector('[data-replication-job-error]');
    expect(detail).toHaveTextContent(error);
    // The expansion wraps; only the one-line row cell truncates.
    expect(detail?.className).toContain('wrap-break-word');
    expect(detail?.className).not.toContain('truncate');
  });

  it('gives the error column more room than the 8% that clipped it', () => {
    expect(replicationTableSource).toContain("error: 'w-[13%]'");
    expect(replicationTableSource).not.toContain("error: 'w-[8%]'");
    expect(replicationTableSource).not.toContain('max-w-[18rem] truncate');
  });

  it('matches jobs by their error text', () => {
    expect(replicationTableSource).toMatch(/job\.lastSyncStatus,\s*job\.error,\s*job\.comment,/);
  });
});

describe('fetchReplicationJobs', () => {
  it('passes the caller abort signal to the network request', async () => {
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(new Response(JSON.stringify({ data: [], meta: { total: 0 } })));
    const controller = new AbortController();

    await expect(fetchReplicationJobs(controller.signal)).resolves.toEqual([]);

    const init = fetchSpy.mock.calls.find(([url]) =>
      String(url).includes('/api/replication/jobs'),
    )?.[1];
    expect(init?.signal).toBe(controller.signal);
  });

  it('keeps the HTTP status on a failed read so access denials are recognised', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ message: 'Insufficient permissions' }), { status: 403 }),
    );

    await expect(fetchReplicationJobs()).rejects.toMatchObject({ status: 403 });
  });
});

describe('ProxmoxReplicationTable relative times', () => {
  const start = Date.parse('2026-10-04T12:00:00Z');
  // Last synced 5 minutes before the table opens, next sync due 10 minutes after.
  const job = () =>
    replicationJob({
      lastSyncUnix: (start - 5 * 60_000) / 1000,
      nextSyncUnix: (start + 10 * 60_000) / 1000,
    });
  const renderTable = () =>
    render(() => (
      <ProxmoxReplicationTable
        jobs={[job()]}
        error={undefined}
        onRetry={() => undefined}
        emptyIcon={<span />}
        emptyTitle="No replication jobs"
        emptyDescription="Replication jobs appear here."
      />
    ));
  // Two hours pass with no new job data; the next clock tick re-reads them.
  const twoHoursLater = () => {
    vi.setSystemTime(start + 2 * 60 * 60_000);
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
  };

  afterEach(() => vi.useRealTimers());

  it('keeps the compact Last sync and Next sync moving while the job does not change', () => {
    vi.useFakeTimers({ now: start });
    vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(400);
    renderTable();

    expect(screen.getByText('5m')).toBeInTheDocument();
    expect(screen.getByText('10m')).toBeInTheDocument();

    twoHoursLater();

    // A stalled scheduler leaves nextSync unchanged, so the countdown must
    // turn overdue on its own.
    expect(screen.getByText('2h')).toBeInTheDocument();
    expect(screen.getByText('-1h')).toHaveClass('text-red-600');
    expect(screen.queryByText('10m')).not.toBeInTheDocument();
  });

  it('keeps the expanded Last sync and Next sync moving while the job does not change', async () => {
    vi.useFakeTimers({ now: start });
    renderTable();
    await fireEvent.click(
      screen.getByRole('button', { name: 'Expand details for replication job 100-0' }),
    );
    const detail = (label: string) =>
      screen.getByText(label, { selector: 'dt' }).nextElementSibling as HTMLElement;

    expect(detail('Last sync')).toHaveTextContent('5m ago');
    expect(detail('Next sync')).toHaveTextContent('in 10m');

    twoHoursLater();

    expect(detail('Last sync')).toHaveTextContent('2h ago');
    expect(detail('Next sync')).toHaveTextContent('1h overdue');
    expect(detail('Next sync')).toHaveClass('text-red-600');
  });
});
