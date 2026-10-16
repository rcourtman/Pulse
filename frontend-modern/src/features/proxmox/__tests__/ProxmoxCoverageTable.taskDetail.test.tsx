import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';
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

afterEach(cleanup);

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
