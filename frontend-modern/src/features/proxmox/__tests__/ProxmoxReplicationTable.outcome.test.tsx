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
});
