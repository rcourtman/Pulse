import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { BackupTask } from '@/types/api';
import { ProxmoxCoverageTable } from '../ProxmoxCoverageTable';
import { buildProxmoxBackupRecoveryModel } from '../proxmoxBackupRecoveryModel';
import { parsePVEBackupInventory } from '../proxmoxBackupInventory';
import { BACKUP_IDENTITY_NOW, identityGuest, identityTask } from '../__fixtures__/backupIdentity';

const error = 'Backup failed: storage is unavailable.\n' + 'repository-path-'.repeat(20);

function mount(tasks: BackupTask[], width = 1365) {
  const [inventory, setInventory] = createSignal(tasks);
  const [expanded, setExpanded] = createSignal(new Set<string>());
  const rows = () =>
    buildProxmoxBackupRecoveryModel({
      nowMs: BACKUP_IDENTITY_NOW,
      workloads: [identityGuest('east-100', 'east', 'pve-1')],
      pbsBackups: [],
      archives: [],
      snapshots: [],
      // Exercise the same accepted API observation shape as the full table.
      tasks: parsePVEBackupInventory({
        data: { backupTasks: inventory(), storageBackups: [], guestSnapshots: [] },
      }).backupTasks,
    }).coverageRows;
  const { container } = render(() => (
    <ProxmoxCoverageTable
      rows={rows()}
      hasAnyRows
      emptyIcon={<span />}
      emptyTitle="No guests"
      emptyDescription="No guests"
      sortKey={() => 'workload'}
      sortDirection={() => 'asc'}
      onSort={() => undefined}
      expandedKeys={expanded()}
      onToggleExpand={(key) =>
        setExpanded((previous) => (previous.has(key) ? new Set() : new Set([key])))
      }
      showTaskColumn={inventory().length > 0}
      layoutWidth={() => width}
    />
  ));
  const toggle = container.querySelector<HTMLButtonElement>('button[aria-expanded]')!;
  return { container, toggle, setInventory };
}

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('expanded Coverage backup job evidence', () => {
  it.each([320, 640, 1365])(
    'keeps the native job ID, reported end and duration separate from observation time at %i px',
    (width) => {
      const id = 'UPID:pve-1:0000001A:0000002B:0000003C:vzdump:100:backup@pve:';
      const endTime = '2026-10-04T11:05:00Z';
      const observedAt = '2026-10-04T11:30:00Z';
      const { toggle } = mount([identityTask({ id, endTime, observedAt })], width);
      fireEvent.click(toggle);
      const detail = screen.getByRole('region', { name: 'Latest backup job' });
      const jobID = within(detail).getByText('Job ID').nextElementSibling;
      expect(jobID?.textContent).toBe(id);
      expect(jobID).toHaveClass('wrap-anywhere');
      expect(jobID).not.toHaveAttribute('title');
      expect(
        within(detail).getByText('Reported end').nextElementSibling?.querySelector('time'),
      ).toHaveAttribute('datetime', endTime);
      expect(within(detail).getByText('Recorded duration').nextElementSibling).toHaveTextContent(
        /^5m 0s$/,
      );
      expect(
        within(detail).getByText('Last observed').nextElementSibling?.querySelector('time'),
      ).toHaveAttribute('datetime', observedAt);
      expect(detail).toHaveTextContent('not evidence of guest thaw or filesystem writes');
    },
  );

  it.each([
    undefined,
    '',
    '0001-01-01T00:00:00Z',
    'not-a-date',
    '2026-10-04T10:59:59Z',
    '2026-10-05T11:05:00Z',
  ])('does not manufacture a job end or elapsed duration from %j', (endTime) => {
    const { toggle } = mount([identityTask({ endTime, status: 'running' })]);
    fireEvent.click(toggle);
    const detail = screen.getByRole('region', { name: 'Latest backup job' });
    const end = within(detail).getByText('Reported end').nextElementSibling;
    expect(end?.querySelector('time')).toBeNull();
    expect(end).toHaveTextContent(endTime ? `Unknown · reported: ${endTime}` : 'Not reported');
    expect(within(detail).getByText('Recorded duration').nextElementSibling).toHaveTextContent(
      /^Unknown$/,
    );
    expect(within(detail).getByText('Result').nextElementSibling).toHaveTextContent(/^Running$/);
  });

  it('retains a zero-second recorded interval without substituting a live elapsed time', () => {
    const { toggle } = mount([identityTask({ endTime: '2026-10-04T11:00:00Z' })]);
    fireEvent.click(toggle);
    const detail = screen.getByRole('region', { name: 'Latest backup job' });
    expect(within(detail).getByText('Recorded duration').nextElementSibling).toHaveTextContent(
      /^0s$/,
    );
  });

  it('replaces the exact job interval in an open row and leaves invalid provider text literal', () => {
    const { toggle, setInventory } = mount([identityTask({ id: 'native-old' })]);
    fireEvent.click(toggle);
    toggle.focus();
    const raw = '<img src="/unexpected"><script>unsafe()</script>';
    setInventory([identityTask({ id: 'native-new', endTime: raw })]);
    const detail = screen.getByRole('region', { name: 'Latest backup job' });
    expect(detail).toHaveTextContent('native-new');
    expect(detail).not.toHaveTextContent('native-old');
    expect(detail.querySelector('[data-backup-task-end]')?.textContent).toBe(
      `Unknown · reported: ${raw}`,
    );
    expect(detail.querySelector('[data-backup-task-duration]')).toHaveTextContent(/^Unknown$/);
    expect(detail.querySelector('img, script')).toBeNull();
    expect(toggle).toHaveFocus();
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    setInventory([identityTask({ id: 'native-final', endTime: '2026-10-04T11:10:00Z' })]);
    expect(detail.querySelector('[data-backup-task-end] time')).toHaveAttribute(
      'datetime',
      '2026-10-04T11:10:00Z',
    );
    expect(detail.querySelector('[data-backup-task-duration]')).toHaveTextContent(/^10m 0s$/);
    expect(detail).not.toHaveTextContent(raw);
    expect(toggle).toHaveFocus();
  });

  it.each([320, 640, 1365])(
    'identifies a raw provider ERROR as Failed and retains its full status at %i px',
    (width) => {
      const status = 'ERROR: archive unavailable\n' + 'repository-path-'.repeat(20);
      const { toggle, setInventory } = mount([identityTask({ status })], width);
      fireEvent.click(toggle);
      toggle.focus();
      const detail = screen.getByRole('region', { name: 'Latest backup job' });
      const result = within(detail).getByText('Result').nextElementSibling;
      expect(result).toHaveTextContent(/^Failed$/);
      expect(result).toHaveClass('text-red-600');
      const reported = within(detail).getByText('Reported status').nextElementSibling;
      expect(reported?.textContent).toBe(status);
      expect(reported).toHaveClass('whitespace-pre-wrap', 'wrap-anywhere');
      expect(within(detail).queryByText('Reported error')).not.toBeInTheDocument();
      setInventory([identityTask({ status: 'OK' })]);
      expect(result).toHaveTextContent(/^OK$/);
      expect(result).not.toHaveClass('text-red-600');
      expect(within(detail).queryByText('Reported status')).not.toBeInTheDocument();
      expect(detail).not.toHaveTextContent('archive unavailable');
      expect(toggle).toHaveFocus();
      expect(toggle).toHaveAttribute('aria-expanded', 'true');
    },
  );

  it('keeps a normalised raw failure literal, including markup-shaped provider text', () => {
    const status = 'ERROR: <img src="/unexpected" onerror="unsafe()"><script>unsafe()</script>';
    const { toggle } = mount([identityTask({ status })]);
    fireEvent.click(toggle);
    const detail = screen.getByRole('region', { name: 'Latest backup job' });
    expect(within(detail).getByText('Result').nextElementSibling).toHaveTextContent(/^Failed$/);
    expect(within(detail).getByText(status)).toBeVisible();
    expect(detail.querySelector('img, script')).toBeNull();
  });

  it('does not replace a separately reported error with the raw provider status', () => {
    const status = 'ERROR: task failed';
    const { toggle } = mount([identityTask({ status, error: 'archive unavailable' })]);
    fireEvent.click(toggle);
    const detail = screen.getByRole('region', { name: 'Latest backup job' });
    expect(within(detail).getByText('Reported status').nextElementSibling).toHaveTextContent(
      status,
    );
    expect(within(detail).getByText('Reported error').nextElementSibling).toHaveTextContent(
      'archive unavailable',
    );
  });

  it.each([320, 640, 1365])('exposes the selected job and full error at %i px', (width) => {
    const { toggle } = mount(
      [
        identityTask({
          id: 'old',
          error: 'Do not show an older error',
          startTime: '2026-10-03T11:00:00Z',
        }),
        identityTask({ status: 'failed', error }),
        identityTask({
          id: 'other',
          instance: 'west',
          status: 'failed',
          error: 'Other connection error',
        }),
      ],
      width,
    );
    expect(screen.queryByRole('region', { name: 'Latest backup job' })).not.toBeInTheDocument();
    fireEvent.click(toggle);
    const detail = screen.getByRole('region', { name: 'Latest backup job' });
    const job = within(detail);
    expect(job.getByText('Failed')).toBeVisible();
    expect(job.getByText('Reported error')).toBeVisible();
    const message = job.getByText(error.replace(/\s+/g, ' '));
    expect(message).toBeVisible();
    expect(message).toHaveClass('whitespace-pre-wrap', 'wrap-anywhere');
    expect(message).not.toHaveAttribute('title');
    expect(detail.querySelector('time')).toHaveAttribute('datetime', '2026-10-04T11:00:00Z');
    expect(detail).not.toHaveTextContent('Do not show an older error');
    expect(detail).not.toHaveTextContent('Other connection error');
  });

  it.each(['OK', 'running', 'stopped', 'WARNINGS: 1'])(
    'keeps %s literal without inventing an error',
    (status) => {
      const { toggle } = mount([identityTask({ status, error: ' \n ' })]);
      fireEvent.click(toggle);
      const detail = screen.getByRole('region', { name: 'Latest backup job' });
      expect(detail).toHaveTextContent(status === 'running' ? 'Running' : status);
      expect(within(detail).queryByText('Reported error')).not.toBeInTheDocument();
      expect(detail).not.toHaveTextContent('recoverable');
    },
  );

  it('renders provider text as text, not markup', () => {
    const raw = '<img src="/unexpected" onerror="alert(1)"><script>unsafe()</script>';
    const { toggle } = mount([identityTask({ status: 'failed', error: raw })]);
    fireEvent.click(toggle);
    const detail = screen.getByRole('region', { name: 'Latest backup job' });
    expect(within(detail).getByText(raw)).toBeVisible();
    expect(detail.querySelector('img, script')).toBeNull();
  });

  it('replaces the error in the same open workload without losing focus or stale text', () => {
    const { toggle, setInventory } = mount([identityTask({ status: 'failed', error })]);
    fireEvent.click(toggle);
    toggle.focus();
    expect(screen.getByRole('region', { name: 'Latest backup job' })).toHaveTextContent(
      'storage is unavailable',
    );
    setInventory([
      identityTask({ id: 'newer', status: 'running', startTime: '2026-10-04T11:30:00Z' }),
    ]);
    const detail = screen.getByRole('region', { name: 'Latest backup job' });
    expect(detail).toHaveTextContent('Running');
    expect(detail).not.toHaveTextContent('storage is unavailable');
    expect(toggle).toHaveFocus();
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
  });

  it.each(['invalid', 'future', 'conflict', 'empty'])(
    'does not select a misleading job for %s inventory',
    (kind) => {
      const tasks = kind === 'empty' ? [] : [identityTask()];
      if (kind !== 'empty')
        tasks.push(
          identityTask({
            id: 'uncertain',
            status: 'failed',
            error,
            startTime:
              kind === 'invalid'
                ? ''
                : kind === 'future'
                  ? '2026-10-05T11:00:00Z'
                  : '2026-10-04T11:00:00Z',
          }),
        );
      const { toggle } = mount(tasks);
      fireEvent.click(toggle);
      expect(screen.queryByRole('region', { name: 'Latest backup job' })).not.toBeInTheDocument();
      if (kind !== 'empty') expect(screen.getByText(/Latest backup job unknown:/)).toBeVisible();
    },
  );
});

describe('unresolved Coverage job evidence', () => {
  const collected = () => screen.getByRole('region', { name: 'Collected backup jobs' });
  const raw = 'ERROR: archive unavailable\n' + 'repository-path-'.repeat(20);

  it.each(['', 'not-a-date', '1970-01-01T00:00:00Z', '2026-10-05T11:00:00Z'])(
    'keeps a failed job with start %j inspectable without claiming it is latest',
    (startTime) => {
      vi.useFakeTimers();
      vi.setSystemTime(BACKUP_IDENTITY_NOW);
      const { toggle } = mount([
        identityTask({ id: 'known-ok' }),
        identityTask({ id: 'unorderable', status: raw, startTime }),
      ]);
      fireEvent.click(toggle);
      expect(screen.queryByRole('region', { name: 'Latest backup job' })).not.toBeInTheDocument();
      const detail = collected();
      expect(detail).toHaveTextContent('do not establish which job is latest');
      expect(detail).toHaveTextContent('not a job timeline');
      expect(detail).toHaveTextContent(
        'does not prove backup usability, guest thaw or filesystem writes',
      );
      const jobs = detail.querySelectorAll('[data-backup-task-evidence]');
      expect(jobs).toHaveLength(2);
      expect(
        within(jobs[0] as HTMLElement).getByText('Result').nextElementSibling,
      ).toHaveTextContent(/^OK$/);
      expect(
        within(jobs[1] as HTMLElement).getByText('Result').nextElementSibling,
      ).toHaveTextContent(/^Failed$/);
      expect(
        within(jobs[1] as HTMLElement).getByText('Reported status').nextElementSibling?.textContent,
      ).toBe(raw);
      const started = jobs[1].querySelector('[data-backup-task-start-unknown]');
      expect(started).toHaveTextContent('Unknown');
      expect(started?.textContent).toContain(startTime || 'not reported');
      expect(started?.querySelector('time')).toBeNull();
    },
  );

  it.each([320, 640, 1365])(
    'retains conflicts and recorded pre-migration origins at %i px',
    (width) => {
      const markup = '<img src="/unexpected"><script>unsafe()</script>';
      const { toggle } = mount(
        [
          identityTask({ id: 'repeated' }),
          identityTask({ id: 'repeated', status: 'failed', error: markup }),
          identityTask({
            id: 'other-connection',
            instance: 'west',
            status: 'failed',
            error: 'wrong connection',
          }),
          identityTask({
            id: 'previous-node',
            node: 'pve-2',
            status: 'failed',
            error: 'Recorded on previous node',
          }),
        ],
        width,
      );
      fireEvent.click(toggle);
      const detail = collected();
      expect(screen.getByText(/same newest start time report different outcomes/)).toBeVisible();
      expect(detail.querySelectorAll('[data-backup-task-evidence]')).toHaveLength(3);
      expect(within(detail).getAllByText('repeated')).toHaveLength(2);
      expect(within(detail).getByText(markup)).toHaveClass('whitespace-pre-wrap', 'wrap-anywhere');
      expect(detail.querySelector('img, script')).toBeNull();
      expect(detail).not.toHaveTextContent('wrong connection');
      expect(detail).toHaveTextContent('Recorded on previous node');
      expect(within(detail).getByText('pve-2')).toBeVisible();
    },
  );

  it('withdraws uncertain evidence on ordinary replacement without closing or refocusing the row', () => {
    const { toggle, setInventory } = mount([
      identityTask({ id: 'known-ok' }),
      identityTask({ id: 'unknown-failed', status: raw, startTime: '' }),
    ]);
    fireEvent.click(toggle);
    toggle.focus();
    expect(collected()).toHaveTextContent('archive unavailable');
    setInventory([identityTask({ id: 'corrected', status: 'OK' })]);
    expect(screen.queryByRole('region', { name: 'Collected backup jobs' })).not.toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Latest backup job' })).toHaveTextContent('OK');
    expect(screen.queryByText(/archive unavailable/)).not.toBeInTheDocument();
    expect(toggle).toHaveFocus();
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    setInventory([]);
    expect(screen.queryByRole('region', { name: 'Latest backup job' })).not.toBeInTheDocument();
    expect(screen.queryByRole('region', { name: 'Collected backup jobs' })).not.toBeInTheDocument();
    expect(toggle).toHaveFocus();
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
  });
});

describe('Coverage job observation age', () => {
  const observedAt = '2026-10-04T11:15:00Z';
  const lastObserved = () =>
    screen.getByRole('region', { name: 'Latest backup job' }).querySelector('dt:last-of-type')
      ?.nextElementSibling;

  it.each([320, 640, 1365])(
    'shows source age and full time at %i px, not a fresh job verdict',
    (width) => {
      vi.useFakeTimers();
      vi.setSystemTime(BACKUP_IDENTITY_NOW);
      const { toggle, container } = mount([identityTask({ status: 'warning', observedAt })], width);
      expect(container.querySelector('[data-backup-task-observed]')).toHaveTextContent(
        'Job observed 45m ago',
      );
      fireEvent.click(toggle);
      expect(lastObserved()).toHaveTextContent('45m ago');
      expect(lastObserved()).toHaveTextContent('2026');
      expect(lastObserved()?.querySelector('time')).toHaveAttribute('datetime', observedAt);
      expect(screen.getByRole('region', { name: 'Latest backup job' })).toHaveTextContent(
        'not when the job finished',
      );
      vi.advanceTimersByTime(2 * 60 * 60 * 1000);
      expect(container.querySelector('[data-backup-task-observed]')).toHaveTextContent(
        'Job observed 2h ago',
      );
      expect(lastObserved()).toHaveTextContent('2h ago');
      expect(container.querySelectorAll('[data-backup-task-observed]')).toHaveLength(2);
      expect(screen.getByRole('region', { name: 'Latest backup job' })).toHaveTextContent(
        'Warning',
      );
    },
  );

  it.each([
    undefined,
    '',
    '0001-01-01T00:00:00Z',
    'invalid',
    '2027-01-01T00:00:00Z',
    '<img src="/unexpected">',
  ])('keeps observation %j unknown, without a misleading just-now age or markup', (observedAt) => {
    vi.useFakeTimers();
    vi.setSystemTime(BACKUP_IDENTITY_NOW);
    const { toggle, container } = mount([identityTask({ observedAt })]);
    expect(screen.getByText('Job observation unknown')).toBeVisible();
    fireEvent.click(toggle);
    expect(lastObserved()).toHaveTextContent(/^Unknown$/);
    expect(lastObserved()?.querySelector('time')).toBeNull();
    expect(container.querySelector('img')).toBeNull();
    expect(screen.queryByText(/Job observed 0s ago/)).not.toBeInTheDocument();
  });

  it('retains the original source age through a same-open refresh, then replaces or withdraws it', () => {
    vi.useFakeTimers();
    vi.setSystemTime(BACKUP_IDENTITY_NOW);
    const { toggle, setInventory } = mount([identityTask({ observedAt })]);
    fireEvent.click(toggle);
    toggle.focus();
    vi.advanceTimersByTime(2 * 60 * 60 * 1000);
    setInventory([identityTask({ observedAt })]);
    expect(lastObserved()).toHaveTextContent('2h ago');
    expect(lastObserved()?.querySelector('time')).toHaveAttribute('datetime', observedAt);
    const fresh = new Date(Date.now() - 60_000).toISOString();
    setInventory([identityTask({ observedAt: fresh })]);
    expect(lastObserved()).toHaveTextContent('1m ago');
    expect(lastObserved()?.querySelector('time')).toHaveAttribute('datetime', fresh);
    setInventory([identityTask()]);
    expect(lastObserved()).toHaveTextContent(/^Unknown$/);
    expect(lastObserved()?.querySelector('time')).toBeNull();
    expect(toggle).toHaveFocus();
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
  });
});
