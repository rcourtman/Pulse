import { render, screen, within } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';
import { ProxmoxBackupJobsCard } from '../ProxmoxBackupJobsCard';
import type { BackupJobsDay, BackupJobsPayload } from '../proxmoxBackupJobs';

const counts = (success = 0, failed = 0, running = 0) => ({
  success,
  failed,
  warning: 0,
  running,
  unknown: 0,
});

const days = (overrides: Partial<Record<number, Partial<BackupJobsDay>>> = {}): BackupJobsDay[] =>
  Array.from({ length: 14 }, (_, i) => ({
    date: `2026-10-${String(19 + i).padStart(2, '0')}`,
    historyState: 'complete' as const,
    counts: counts(10),
    ...overrides[i],
  }));

const payload = (overrides: Partial<BackupJobsPayload> = {}): BackupJobsPayload => ({
  historyState: 'complete',
  timezone: 'UTC',
  last24h: {
    historyState: 'complete',
    counts: counts(456, 35, 33),
    failures: [
      { workloadId: 'vm-1', workloadName: 'web', instance: 'lab', at: '', outcome: 'failed' },
      { workloadId: 'vm-2', workloadName: 'db', instance: 'lab', at: '', outcome: 'failed' },
      { workloadId: 'vm-3', workloadName: 'cache', instance: 'lab', at: '', outcome: 'warning' },
    ],
  },
  days: days(),
  ...overrides,
});

describe('ProxmoxBackupJobsCard', () => {
  it('shows the last 24 hours as outcome pills and names failures on hover', () => {
    const { container } = render(() => <ProxmoxBackupJobsCard jobs={payload()} />);
    const pills = container.querySelectorAll('[data-backup-jobs-pill]');
    expect([...pills].map((pill) => pill.textContent)).toEqual([
      '456 ok',
      '33 running',
      '35 failed',
    ]);
    // Warnings are not named as failures.
    expect(container.querySelector('[data-backup-jobs-pill="failed"]')).toHaveAttribute(
      'title',
      'web, db',
    );
  });

  it('draws one bar per day and outlines days Pulse did not fully observe', () => {
    render(() => (
      <ProxmoxBackupJobsCard
        jobs={payload({
          historyState: 'partial',
          days: days({
            0: { historyState: 'partial', counts: counts() },
            13: { counts: counts(3, 1) },
          }),
        })}
      />
    ));
    const list = within(screen.getByRole('list', { name: 'Backups per day, last 14 days' }));
    const items = list.getAllByRole('listitem');
    expect(items).toHaveLength(14);
    expect(items[0]).toHaveAttribute('data-backup-jobs-day', 'partial');
    expect(items[0]).toHaveClass('border-dashed');
    expect(items[13]).toHaveAttribute('data-backup-jobs-day', 'observed');
    expect(items[13]).toHaveTextContent('3 ok, 1 failed');
    expect(items[0]).toHaveTextContent('no backups (not fully observed)');
  });

  it('says so when no backup ran in the last 24 hours', () => {
    const { container } = render(() => (
      <ProxmoxBackupJobsCard
        jobs={payload({ last24h: { historyState: 'complete', counts: counts(), failures: [] } })}
      />
    ));
    expect(container.querySelector('[data-backup-jobs-pill]')).toHaveTextContent('0 ok');
  });

  it('never presents unreadable history as counts', () => {
    const { container } = render(() => (
      <ProxmoxBackupJobsCard
        jobs={payload({
          historyState: 'unavailable',
          last24h: { historyState: 'unavailable', counts: counts(), failures: [] },
          days: days(
            Object.fromEntries(
              Array.from({ length: 14 }, (_, i) => [
                i,
                { historyState: 'unavailable', counts: counts() },
              ]),
            ),
          ),
        })}
      />
    ));
    expect(container).toHaveTextContent('History unavailable');
    expect(container.querySelector('[data-backup-jobs-pill]')).toBeNull();
  });
});
