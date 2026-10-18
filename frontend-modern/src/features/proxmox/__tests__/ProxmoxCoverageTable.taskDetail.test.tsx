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
