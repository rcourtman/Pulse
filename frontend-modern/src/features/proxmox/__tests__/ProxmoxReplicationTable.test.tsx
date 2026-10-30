import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import { Route, Router } from '@solidjs/router';
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
  readReplicationSyncTime,
} from '../ProxmoxReplicationTable';
import replicationTableSource from '../ProxmoxReplicationTable.tsx?raw';
import {
  ReplicationInventoryFormatError,
  parseReplicationInventory,
} from '../proxmoxReplicationInventory';

// Record drawers link to the object they are about, which needs a router.
const renderInRouter = (component: () => JSX.Element) =>
  render(() => (
    <Router>
      <Route path="/*" component={component} />
    </Router>
  ));

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
  it.each([
    ['2026-10-30T10:00:00Z', Date.parse('2026-10-30T10:00:00Z')],
    ['2026-10-30T11:00:00+01:00', Date.parse('2026-10-30T10:00:00Z')],
    ['2024-02-29T10:00:00.123456789Z', Date.parse('2024-02-29T10:00:00.123Z')],
    [1_793_354_400, 1_793_354_400_000],
    [1_793_354_400_000, 1_793_354_400_000],
  ])('preserves supported explicit-zone and numeric sync times (%s)', (time, ms) => {
    expect(readReplicationSyncTime(time, undefined)).toEqual({ state: 'known', ms });
  });

  it.each([
    '2026-02-30T10:00:00Z',
    '2026-10-30T10:00:00',
    '2026-10-30',
    'October 30, 2026 10:00:00 GMT',
    '2026-10-30T24:00:00Z',
    '2026-10-30T10:00:00+25:00',
    'unreadable',
    ' ',
    -1,
    NaN,
    Infinity,
    8_640_000_000_000_001,
  ])('keeps unreadable chronology unknown (%s)', (time) => {
    expect(readReplicationSyncTime(time, undefined)).toEqual({ state: 'invalid' });
  });

  it('keeps absent/zero fields distinct and retains Unix companion precedence', () => {
    for (const time of [undefined, '', 0]) {
      expect(readReplicationSyncTime(time, 0)).toEqual({ state: 'missing' });
    }
    expect(readReplicationSyncTime('unreadable', 1_793_354_400)).toEqual({
      state: 'known',
      ms: 1_793_354_400_000,
    });
    for (const unix of [-1, NaN, Infinity, 8_640_000_000_001]) {
      expect(readReplicationSyncTime('2026-10-30T10:00:00Z', unix)).toEqual({ state: 'invalid' });
    }
    expect(readReplicationSyncTime('2026-10-30T10:00:00Z', 0)).toEqual({
      state: 'known',
      ms: 1_793_354_400_000,
    });
  });

  it.each(['2026-02-30T10:00:00Z', '2026-10-30T10:00:00', 'unreadable', -1, 8_640_000_000_000_001])(
    'renders invalid parsed inventory dates as unknown in cells and disclosure (%s)',
    async (time) => {
      vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-10-30T11:00:00Z'));
      const data = [replicationJob({ lastSyncUnix: 0, lastSyncTime: time, nextSyncTime: time })];
      const jobs = parseReplicationInventory({ data, meta: { total: 1 } });
      renderInRouter(() => (
        <ProxmoxReplicationTable
          jobs={jobs}
          error={undefined}
          onRetry={() => undefined}
          emptyIcon={<span />}
          emptyTitle="No replication jobs"
          emptyDescription="Replication jobs appear here."
        />
      ));
      expect(screen.getByLabelText('Last sync time unknown')).toBeInTheDocument();
      expect(screen.getByLabelText('Next sync time unknown')).toBeInTheDocument();
      // Chronology does not erase the job or reclassify the provider's status.
      expect(screen.getByText('100 (web)')).toBeInTheDocument();
      await fireEvent.click(
        screen.getByRole('button', { name: 'Expand details for replication job 100-0' }),
      );
      const panel = screen.getByTestId('replication-job-detail');
      expect(
        within(panel).getByText('Last sync time unknown · next sync time unknown'),
      ).toBeInTheDocument();
      expect(
        within(panel).queryByText(/Not synced yet|Last synced|overdue|just now/),
      ).not.toBeInTheDocument();
      expect(within(panel).getAllByText('Unknown')).toHaveLength(2);
      expect(within(panel).getByRole('img', { name: 'Healthy' })).toBeInTheDocument();
    },
  );

  it('does not turn a future last sync into a just-now success or suppress a valid next sync', async () => {
    const now = Date.parse('2026-10-30T11:00:00Z');
    vi.spyOn(Date, 'now').mockReturnValue(now);
    const future = (now + 10 * 60_000) / 1000;
    const jobs = [replicationJob({ lastSyncUnix: future, nextSyncUnix: future })];
    renderInRouter(() => (
      <ProxmoxReplicationTable
        jobs={jobs}
        error={undefined}
        onRetry={() => undefined}
        emptyIcon={<span />}
        emptyTitle="No replication jobs"
        emptyDescription="Replication jobs appear here."
      />
    ));
    expect(screen.getByLabelText('Last sync time unknown')).toBeInTheDocument();
    expect(screen.queryByText('just now')).not.toBeInTheDocument();
    await fireEvent.click(
      screen.getByRole('button', { name: 'Expand details for replication job 100-0' }),
    );
    expect(screen.getByText('Last sync time unknown · next sync in 10m')).toBeInTheDocument();
  });

  it('describes omitted last-sync evidence as unreported, not as proof a job never ran', async () => {
    renderInRouter(() => (
      <ProxmoxReplicationTable
        jobs={[replicationJob({ lastSyncUnix: 0 })]}
        error={undefined}
        onRetry={() => undefined}
        emptyIcon={<span />}
        emptyTitle="No replication jobs"
        emptyDescription="Replication jobs appear here."
      />
    ));
    await fireEvent.click(
      screen.getByRole('button', { name: 'Expand details for replication job 100-0' }),
    );
    const panel = screen.getByTestId('replication-job-detail');
    expect(within(panel).getByText('No last sync reported')).toBeInTheDocument();
    expect(within(panel).getByText('Not reported')).toBeInTheDocument();
    expect(within(panel).queryByText('Not synced yet')).not.toBeInTheDocument();
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

    renderInRouter(() => (
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
    renderInRouter(() => (
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
    renderInRouter(() => (
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

    // A failed job's drawer is headed by its guest and states why it failed.
    const panel = screen.getByTestId('replication-job-detail');
    expect(panel).toHaveAttribute('data-inline-detail-for', 'replication-100');
    expect(screen.getByRole('heading', { level: 2, name: /web/ })).toBeInTheDocument();
    const detail = document.querySelector('[data-replication-job-error]');
    expect(detail).toHaveTextContent(error);
    // The drawer wraps the error; only the one-line row cell truncates.
    expect(detail?.parentElement?.className).toContain('wrap-anywhere');
    expect(detail?.className).not.toContain('truncate');
    // The error is the summary, so it is not repeated as a section.
    expect(within(panel).queryByText('Error', { selector: 'th' })).not.toBeInTheDocument();
  });

  it('keys the disclosure to the job and links the drawer to the replicated guest', async () => {
    renderInRouter(() => (
      <ProxmoxReplicationTable
        jobs={[replicationJob({ guestType: 'lxc', guestNode: 'pve-a' })]}
        error={undefined}
        onRetry={() => undefined}
        emptyIcon={<span />}
        emptyTitle="No replication jobs"
        emptyDescription="Replication jobs appear here."
      />
    ));

    const toggle = screen.getByRole('button', { name: 'Expand details for replication job 100-0' });
    expect(toggle).toHaveAttribute('aria-controls', 'replication-job-detail-replication-100');
    await fireEvent.click(toggle);

    expect(document.getElementById('replication-job-detail-replication-100')).not.toBeNull();
    const link = screen.getByRole('link', { name: 'Open web' });
    expect(link).toHaveAttribute('href', '/proxmox/overview?resource=pve%3Apve-a%3A100');
    const panel = screen.getByTestId('replication-job-detail');
    for (const section of ['Job', 'Route', 'Schedule']) {
      expect(within(panel).getByText(section, { selector: 'th' })).toBeInTheDocument();
    }

    await fireEvent.click(screen.getByRole('button', { name: 'Collapse web details' }));
    expect(screen.queryByTestId('replication-job-detail')).not.toBeInTheDocument();
  });

  it('leads a pending job drawer with its error and omits the link without a guest', async () => {
    renderInRouter(() => (
      <ProxmoxReplicationTable
        jobs={[
          replicationJob({
            lastSyncStatus: 'pending',
            error: 'target unreachable',
            guestId: undefined,
            guestName: undefined,
          }),
        ]}
        error={undefined}
        onRetry={() => undefined}
        emptyIcon={<span />}
        emptyTitle="No replication jobs"
        emptyDescription="Replication jobs appear here."
      />
    ));

    await fireEvent.click(
      screen.getByRole('button', { name: 'Expand details for replication job 100-0' }),
    );
    const sections = [...document.querySelectorAll('[data-testid="replication-job-detail"] th')];
    expect(sections[0]).toHaveTextContent('Error');
    expect(
      within(screen.getByTestId('replication-job-detail')).getByText('target unreachable'),
    ).toBeInTheDocument();
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
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
  it.each([
    '{}',
    '{"data":null,"meta":{"total":0}}',
    '{"data":[null],"meta":{"total":1}}',
    '{"data":[],"meta":{"total":1}}',
    '{"data":[{"id":"private-invalid-row"}',
  ])('rejects a malformed HTTP 200 response without leaking its body (%#)', async (body) => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(body));
    await expect(fetchReplicationJobs()).rejects.toMatchObject({
      name: 'ReplicationInventoryFormatError',
      message: new ReplicationInventoryFormatError().message,
    });
  });

  it('returns the complete typed HTTP inventory without changing job state', async () => {
    const data = [
      replicationJob(),
      replicationJob({ id: 'site-b-100', instance: 'site-b', enabled: false }),
    ];
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ data, meta: { total: 2 } })),
    );
    await expect(fetchReplicationJobs()).resolves.toEqual(data);
  });

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
    renderInRouter(() => (
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
      screen.getByText(label, { selector: 'td' }).nextElementSibling as HTMLElement;

    expect(detail('Last sync')).toHaveTextContent('5m ago');
    expect(detail('Next sync')).toHaveTextContent('in 10m');
    expect(screen.getByText('Last synced 5m ago · next sync in 10m')).toBeInTheDocument();

    twoHoursLater();

    expect(detail('Last sync')).toHaveTextContent('2h ago');
    expect(detail('Next sync')).toHaveTextContent('1h overdue');
    expect(detail('Next sync')).toHaveClass('text-rose-700');
    expect(screen.getByText('Last synced 2h ago · next sync 1h overdue')).toBeInTheDocument();
  });
});
