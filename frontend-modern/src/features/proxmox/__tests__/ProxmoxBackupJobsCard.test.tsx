import { fireEvent, render, screen, within } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { ProxmoxBackupJobsCard } from '../ProxmoxBackupJobsCard';
import type { BackupJobsDay, BackupJobsPayload } from '../proxmoxBackupJobs';

vi.mock('@/components/shared/hoverCapability', () => ({ supportsHoverTooltips: () => true }));

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
    failures: [],
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

  it("names the pills' window, and only that window, in the header", () => {
    const { container } = render(() => <ProxmoxBackupJobsCard jobs={payload()} />);
    const header = container.querySelector('[data-backup-jobs-last24h]')!;
    expect(header).toHaveTextContent(/^24hLast 24h/);
    // The chart's span is on its axis; a "14 days" title beside the 24-hour
    // pills read as if they were the chart's totals.
    expect(container).not.toHaveTextContent('14 days');
    expect(container.querySelector('[data-backup-jobs-incomplete]')).toBeNull();
  });

  it('says plainly when Pulse saw only part of the last 24 hours', () => {
    const { container } = render(() => (
      <ProxmoxBackupJobsCard
        jobs={payload({
          last24h: { historyState: 'partial', counts: counts(12), failures: [] },
        })}
      />
    ));
    const tag = container.querySelector('[data-backup-jobs-incomplete]');
    expect(tag).toHaveTextContent(/^incomplete data$/);
    expect(tag).toHaveAttribute(
      'title',
      'Pulse did not see the whole last 24 hours, so these counts may be low',
    );
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

  const failure = (workloadName: string, outcome = 'failed') => ({
    workloadId: workloadName,
    workloadName,
    instance: 'lab',
    at: '',
    outcome,
  });

  it('shows the day and names who failed when a bar is hovered or focused', async () => {
    render(() => (
      <ProxmoxBackupJobsCard
        jobs={payload({
          days: days({
            2: {
              counts: { ...counts(9, 2), warning: 1 },
              failures: [failure('web'), failure('db'), failure('cache', 'warning')],
            },
          }),
        })}
      />
    ));
    const bar = screen.getAllByRole('button')[2];
    expect(document.querySelector('[data-backup-jobs-tooltip]')).toBeNull();

    await fireEvent.mouseEnter(bar);
    const tip = document.querySelector('[data-backup-jobs-tooltip]')!;
    expect(tip).toHaveTextContent('2');
    expect(tip).toHaveTextContent('web, db');
    expect(tip).toHaveTextContent('cache');
    expect(tip).toHaveTextContent('ok');

    await fireEvent.mouseLeave(bar);
    expect(document.querySelector('[data-backup-jobs-tooltip]')).toBeNull();

    await fireEvent.focus(bar);
    expect(document.querySelector('[data-backup-jobs-tooltip]')).not.toBeNull();
    await fireEvent.blur(bar);
    expect(document.querySelector('[data-backup-jobs-tooltip]')).toBeNull();
  });

  it('reports a clicked day, marks it, and says who failed without needing hover', async () => {
    const onSelectDay = vi.fn();
    const jobs = payload({
      days: days({ 2: { counts: counts(9, 2), failures: [failure('web'), failure('db')] } }),
    });
    const { container } = render(() => (
      <ProxmoxBackupJobsCard jobs={jobs} selectedDay="2026-10-21" onSelectDay={onSelectDay} />
    ));
    const bars = screen.getAllByRole('button');
    expect(bars[2]).toHaveAttribute('aria-pressed', 'true');
    expect(bars[3]).toHaveAttribute('aria-pressed', 'false');
    expect(bars[3]).toHaveClass('opacity-40');
    expect(container.querySelector('[data-backup-jobs-chosen-day]')).toHaveTextContent(
      '2 failed: web, db',
    );

    // A clean day's answer is the list; a failed day's answer is this card.
    await fireEvent.click(bars[5]);
    expect(onSelectDay).toHaveBeenLastCalledWith('2026-10-24', { reveal: true });
    await fireEvent.click(bars[2]);
    expect(onSelectDay).toHaveBeenLastCalledWith('2026-10-21', { reveal: false });
  });

  it('leaves the bars undimmed when the chosen day is outside the 14 days', () => {
    const { container } = render(() => (
      <ProxmoxBackupJobsCard jobs={payload()} selectedDay="2026-01-01" />
    ));
    expect(screen.getAllByRole('button')[0]).not.toHaveClass('opacity-40');
    expect(container.querySelector('[data-backup-jobs-chosen-day]')).toBeNull();
  });
});
