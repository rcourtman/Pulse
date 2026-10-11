import { createSignal } from 'solid-js';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { BackupIndicator, BackupStatusCell } from '../GuestRowCells';
import { useGuestRowState } from '../useGuestRowState';

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    getBackupThresholds: () => ({ freshHours: 1, staleHours: 2 }),
    getMetricThresholds: () => null,
  }),
}));
vi.mock('@/hooks/useAnomalies', () => ({ useAnomalyForMetric: () => () => null }));
vi.mock('@/hooks/useBreakpoint', () => ({ useBreakpoint: () => ({ isMobile: () => false }) }));

const start = Date.parse('2026-10-10T12:00:00Z');
const hour = 3_600_000;
const fakeClock = () =>
  vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: start });

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('mounted workload evidence clock', () => {
  it('ages both backup entry points past the configured boundaries without new evidence', () => {
    fakeClock();
    const lastBackup = start - hour;
    render(() => (
      <>
        <BackupStatusCell lastBackup={lastBackup} guestName="Database VM" />
        <BackupIndicator lastBackup={lastBackup} isTemplate={false} />
      </>
    ));
    const badge = screen.getByRole('button', { name: /^Backup status: fresh/ });
    const indicator = screen.getByRole('button', { name: /^Last backup:/ });
    expect(badge).toHaveClass('text-green-600');
    expect(indicator).toHaveClass('text-green-600');
    fireEvent.click(badge);
    const dialog = screen.getByRole('dialog');
    const time = dialog.querySelector('time');
    expect(dialog).toHaveTextContent('1 hour ago');

    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(badge).toHaveAccessibleName(/^Backup status: stale/);
    expect(badge).toHaveTextContent('1h');
    expect(indicator.className).not.toContain('green');
    expect(badge).toHaveClass('border-yellow-200');
    vi.advanceTimersByTime(hour);
    expect(badge).toHaveAccessibleName(/^Backup status: overdue/);
    expect(badge).toHaveTextContent('2h');
    expect(dialog).toHaveTextContent('2 hours ago');
    expect(dialog.querySelector('time')).toBe(time);
    expect(time).toHaveAttribute('datetime', new Date(lastBackup).toISOString());
    expect(screen.getByRole('button', { name: /^Backup status: overdue/ })).toBe(badge);
  });

  it('keeps running, invalid, absent and template semantics while the clock moves', () => {
    fakeClock();
    render(() => (
      <>
        <BackupStatusCell lastBackup={start - hour} backupRunning />
        <BackupStatusCell lastBackup="invalid" />
        <BackupStatusCell lastBackup={0} />
        <BackupIndicator lastBackup={start - hour} isTemplate />
      </>
    ));
    const running = screen.getByRole('button', { name: /^Backup status: backup running now/ });
    vi.advanceTimersByTime(hour);
    expect(running).toHaveTextContent('Running');
    expect(running).toHaveClass('border-blue-200');
    expect(running).toHaveAccessibleName(/last completed backup 2 hours ago/);
    expect(screen.getByRole('button', { name: /invalid timestamp/ })).toHaveTextContent('Unknown');
    expect(
      screen.getByRole('button', { name: 'Backup status: no backup found' }),
    ).toHaveTextContent('None');
    expect(screen.getAllByRole('button')).toHaveLength(3);
    cleanup();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('expires the existing attached probe and withdraws it without retaining a presentation', () => {
    fakeClock();
    const [guest, setGuest] = createSignal({
      id: 'lab:vm:100',
      type: 'vm',
      name: 'Database VM',
      status: 'running',
      availability: {
        enabled: true,
        available: true,
        protocol: 'https',
        path: '/health',
        correlationState: 'attached',
        lastChecked: new Date(start - 60_000).toISOString(),
        evidence: { validUntil: new Date(start + 10_000).toISOString() },
      },
    } as WorkloadGuest);
    let latest: ReturnType<typeof useGuestRowState>;
    render(() => {
      latest = useGuestRowState({
        get guest() {
          return guest();
        },
      });
      return (
        <span data-testid="probe" class={latest.availabilityPresentation()?.toneClassName}>
          {latest.availabilityPresentation()?.detailLabel}
        </span>
      );
    });
    const probe = screen.getByTestId('probe');
    expect(probe).toHaveTextContent('fresh - checked 1 min ago');
    expect(probe.className).toContain('emerald');
    vi.advanceTimersByTime(60_000);
    expect(probe).toHaveTextContent('stale - checked 2 mins ago');
    expect(probe.className).toContain('amber');
    expect(guest().availability?.available).toBe(true);
    setGuest({ ...guest(), availability: undefined });
    expect(latest!.availabilityPresentation()).toBeNull();
    expect(probe).toBeEmptyDOMElement();
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(latest!.availabilityPresentation()).toBeNull();
  });
});
