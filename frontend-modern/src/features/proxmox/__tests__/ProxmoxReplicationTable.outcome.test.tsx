import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';
import type { ReplicationJob } from '@/types/api';
import { ProxmoxReplicationTable } from '../ProxmoxReplicationTable';
import { parseReplicationInventory } from '../proxmoxReplicationInventory';

const job = (overrides: Partial<ReplicationJob> = {}): ReplicationJob => ({
  id: 'site-a-100-0',
  instance: 'site-a',
  jobId: '100-0',
  guestId: 100,
  guestName: 'database',
  sourceNode: 'pve-a',
  targetNode: 'pve-b',
  enabled: true,
  ...overrides,
});

const mount = (jobs: () => ReplicationJob[]) =>
  render(() => (
    <Router>
      <Route
        path="/*"
        component={() => (
          <ProxmoxReplicationTable
            jobs={jobs()}
            error={undefined}
            onRetry={() => undefined}
            emptyIcon={<span />}
            emptyTitle="No replication jobs"
            emptyDescription="Replication jobs appear here."
          />
        )}
      />
    </Router>
  ));

const inventory = (rows: ReplicationJob[]) =>
  parseReplicationInventory(
    JSON.parse(JSON.stringify({ data: rows, meta: { total: rows.length } })),
  );
const row = () => screen.getByText('100 (database)').closest('tr')!;
const details = async () => {
  await fireEvent.click(
    screen.getByRole('button', { name: 'Expand details for replication job 100-0' }),
  );
  return screen.getByTestId('replication-job-detail');
};

afterEach(cleanup);

describe('replication outcome evidence', () => {
  it.each([undefined, '', ' ', 'unknown', 'future-state', 'syncing'])(
    'does not turn an unobserved or unsupported outcome into pending (%s)',
    async (lastSyncStatus) => {
      mount(() => inventory([job({ lastSyncStatus })]));
      expect(within(row()).getByText('Unknown')).toBeInTheDocument();
      expect(within(row()).getByLabelText('Failure count not reported')).toBeInTheDocument();
      expect(within(row()).queryByText('Pending')).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Unknown, 1' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Pending, 0' })).toBeInTheDocument();
      const panel = await details();
      expect(within(panel).getByRole('img', { name: 'Unknown' })).toBeInTheDocument();
      expect(
        within(panel).getByText(/The latest sync outcome is not reported/),
      ).toBeInTheDocument();
      expect(
        within(panel).getByText(/Unknown does not mean the job is pending or has no failures/),
      ).toBeInTheDocument();
      const outcome = within(panel).getByRole('row', { name: /^Reported outcome / });
      expect(outcome).toHaveTextContent(lastSyncStatus?.trim() || 'Not reported');
      const failures = within(panel).getByRole('row', { name: /^Failures / });
      expect(failures).toHaveTextContent('Not reported');
    },
  );

  it.each(['ok', 'success', 'completed', ' OK '])(
    'keeps successful omitted-zero wire counts (%s)',
    async (lastSyncStatus) => {
      mount(() => inventory([job({ lastSyncStatus })]));
      expect(within(row()).getByText('Healthy')).toBeInTheDocument();
      expect(within(row()).getByText('0')).toBeInTheDocument();
      const panel = await details();
      expect(within(panel).getByRole('row', { name: /^Failures / })).toHaveTextContent('0');
      expect(within(panel).queryByText(/The latest sync outcome/)).not.toBeInTheDocument();
    },
  );

  it.each([
    ['pending', 0, true, 'Pending', '0'],
    [' pending ', undefined, true, 'Pending', 'Not reported'],
    ['error', undefined, true, 'Failed', 'Not reported'],
    ['failed', 3, true, 'Failed', '3'],
    ['ok', 2, true, 'Failed', '2'],
    ['future-state', 2, true, 'Failed', '2'],
    [undefined, 0, true, 'Unknown', '0'],
    [undefined, undefined, false, 'Disabled', 'Not reported'],
    ['ok', undefined, false, 'Disabled', '0'],
  ] as const)(
    'preserves observed state and count independently (%s/%s/%s)',
    async (lastSyncStatus, failCount, enabled, status, failures) => {
      mount(() => inventory([job({ lastSyncStatus, failCount, enabled })]));
      expect(within(row()).getByText(status)).toBeInTheDocument();
      const panel = await details();
      expect(within(panel).getByRole('row', { name: /^Failures / })).toHaveTextContent(failures);
    },
  );

  it('separates pending/unknown counts and updates a selected filter on observation without losing a job', async () => {
    const rows = inventory([
      job(),
      job({
        id: 'site-a-101-0',
        jobId: '101-0',
        guestId: 101,
        guestName: 'new-worker',
        lastSyncStatus: 'pending',
        failCount: 0,
      }),
      job({
        id: 'site-a-102-0',
        jobId: '102-0',
        guestId: 102,
        guestName: 'web',
        lastSyncStatus: 'ok',
      }),
    ]);
    const [jobs, setJobs] = createSignal(rows);
    mount(jobs);
    await fireEvent.click(screen.getByRole('button', { name: 'Unknown, 1' }));
    expect(screen.getByText('100 (database)')).toBeInTheDocument();
    expect(screen.queryByText('101 (new-worker)')).not.toBeInTheDocument();
    const unknown = screen.getByRole('button', { name: 'Unknown, 1' });
    unknown.focus();
    setJobs(
      inventory(
        rows.map((entry) => (entry.guestId === 100 ? { ...entry, lastSyncStatus: 'ok' } : entry)),
      ),
    );
    expect(screen.getByRole('button', { name: 'Unknown, 0' })).toBe(unknown);
    expect(unknown).toHaveFocus();
    expect(unknown).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByText('No replication jobs match current filters')).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'All, 3' }));
    expect(screen.getByText('100 (database)')).toBeInTheDocument();
    expect(within(row()).getByText('Healthy')).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Pending, 1' }));
    expect(screen.getByText('101 (new-worker)')).toBeInTheDocument();
    expect(screen.queryByText('100 (database)')).not.toBeInTheDocument();
  });

  it.each([
    ['data', 'database', false],
    ['data', 'database', true],
    ['100-', '100-0', false],
    ['100-', '100-0', true],
  ] as const)(
    'completes %s to broad text %s before and after submission (namesake %s)',
    (draft, completed, hasNamesake) => {
      const primary = job({ lastSyncStatus: 'ok' });
      const mirror = job({ id: 'mirror-site-a-100-0', instance: 'mirror-site-a', failCount: 2 });
      const [jobs, setJobs] = createSignal(inventory(hasNamesake ? [primary, mirror] : [primary]));
      mount(jobs);
      const input = screen.getByPlaceholderText('Search jobs, guests, nodes');
      const expectResults = (count: number) => {
        expect(screen.getAllByText('100 (database)')).toHaveLength(count);
        expect(screen.getByRole('button', { name: `All, ${count}` })).toBeVisible();
      };

      input.focus();
      fireEvent.input(input, { target: { value: draft } });
      expectResults(hasNamesake ? 2 : 1);
      fireEvent.keyDown(input, { key: 'Tab' });
      expect(input).toHaveValue(completed);
      expectResults(hasNamesake ? 2 : 1);
      fireEvent.keyDown(input, { key: 'Enter' });
      expect(screen.getByRole('button', { name: `Remove search term ${completed}` })).toBeVisible();
      expectResults(hasNamesake ? 2 : 1);

      setJobs(inventory([primary, mirror]));
      expectResults(2);
    },
  );

  it('does not bind the shorter ID just because a shared prefix completed to it', () => {
    const shorter = job({ jobId: '100-1', id: 'site-a-100-1', lastSyncStatus: 'ok' });
    const longer = job({ jobId: '100-10', id: 'site-a-100-10', failCount: 2 });
    mount(() => inventory([shorter, longer]));
    const input = screen.getByPlaceholderText('Search jobs, guests, nodes');
    input.focus();
    fireEvent.input(input, { target: { value: 'site-a-100-' } });
    fireEvent.keyDown(input, { key: 'Tab' });
    expect(input).toHaveValue(shorter.id);
    expect(screen.getByRole('button', { name: 'All, 2' })).toBeVisible();
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(screen.getByRole('button', { name: `Remove search term ${shorter.id}` })).toBeVisible();
    expect(screen.getByRole('button', { name: 'All, 2' })).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: `Remove search term ${shorter.id}` }));

    fireEvent.input(input, { target: { value: '100-1 · site-a' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(screen.getByRole('button', { name: 'Remove search term 100-1 · site-a' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'All, 1' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'Healthy, 1' })).toBeVisible();
  });

  it('keeps a canonical ID broad when it also names another guest', () => {
    const primary = job({ lastSyncStatus: 'ok' });
    const other = job({
      jobId: '101-0',
      id: 'site-a-101-0',
      guestName: primary.id.toUpperCase(),
      failCount: 2,
    });
    mount(() => inventory([primary, other]));
    const input = screen.getByPlaceholderText('Search jobs, guests, nodes');
    fireEvent.input(input, { target: { value: primary.id } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(screen.getByRole('button', { name: `Remove search term ${primary.id}` })).toBeVisible();
    expect(screen.getByRole('button', { name: 'All, 2' })).toBeVisible();
  });

  it('completes each connection separately, keeps filters combinable, and never selects a namesake after refresh', () => {
    const primary = job({ lastSyncStatus: 'ok' });
    const mirror = job({
      id: 'Mirror, Site-a-100-0',
      instance: 'Mirror, Site-a',
      lastSyncStatus: 'error',
      failCount: 2,
      error: 'Target temporarily unavailable',
    });
    const [jobs, setJobs] = createSignal(inventory([primary, mirror]));
    mount(jobs);
    const input = screen.getByPlaceholderText('Search jobs, guests, nodes');
    const complete = (draft: string, completed: string) => {
      input.focus();
      fireEvent.input(input, { target: { value: draft } });
      fireEvent.keyDown(input, { key: 'Tab' });
      expect(input).toHaveValue(completed);
      fireEvent.keyDown(input, { key: 'Enter' });
    };

    complete('site-a-100-', primary.id);
    const onlyPrimary = screen.getByRole('button', {
      name: 'Expand details for replication job 100-0 on site-a',
    });
    expect(onlyPrimary).toBeVisible();
    expect(screen.getByRole('button', { name: 'Healthy, 1' })).toBeVisible();
    expect(screen.queryByText(mirror.instance, { exact: true })).not.toBeInTheDocument();

    fireEvent.input(input, { target: { value: '-mirror' } });
    expect(onlyPrimary).toBeVisible();
    complete('Mirror, Site-a-100-', mirror.id);
    expect(screen.getByRole('button', { name: 'All, 2' })).toBeVisible();
    const mirrorToggle = screen.getByRole('button', {
      name: 'Expand details for replication job 100-0 on Mirror, Site-a',
    });
    fireEvent.click(mirrorToggle);
    const panel = screen.getByTestId('replication-job-detail');
    expect(within(panel).getByRole('row', { name: /^Connection / })).toHaveTextContent(
      mirror.instance,
    );
    expect(within(panel).getByText(mirror.error!)).toBeVisible();
    fireEvent.click(
      screen.getByRole('button', {
        name: 'Collapse details for replication job 100-0 on Mirror, Site-a',
      }),
    );
    fireEvent.click(
      screen.getByRole('button', { name: 'Remove search term 100-0 · Mirror, Site-a' }),
    );

    setJobs(inventory([mirror]));
    expect(screen.getByText('No replication jobs match current filters')).toBeInTheDocument();
    expect(screen.queryByText('100 (database)')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Remove search term 100-0 · site-a' }));
    expect(screen.getByText('100 (database)')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Failed, 1' })).toBeVisible();
  });
});
