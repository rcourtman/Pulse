import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { BackupIndicator, BackupStatusCell } from '../GuestRowCells';
import { getGuestDrawerBackupPresentation } from '../guestDrawerModel';

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ getBackupThresholds: () => undefined }),
}));

const now = Date.parse('2026-10-12T02:00:00Z');
const completed = Date.parse('2026-10-12T00:30:00Z');
const iso = '2026-10-12T00:30:00.000Z';
const label = '2026-10-12 00:30:00 UTC';
let clockIntervals: MockInstance<typeof globalThis.setInterval>;
let clockClears: MockInstance<typeof globalThis.clearInterval>;

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(now);
  clockIntervals = vi.spyOn(globalThis, 'setInterval');
  clockClears = vi.spyOn(globalThis, 'clearInterval');
});
afterEach(() => {
  cleanup();
  // Observe ownership directly, rather than counting unrelated test-DOM
  // scheduled work: the shared clock must clear each interval it started.
  expect(clockIntervals).toHaveBeenCalledTimes(1);
  for (const interval of clockIntervals.mock.results) {
    expect(clockClears).toHaveBeenCalledWith(interval.value);
  }
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe('one explicit completion time across backup evidence surfaces', () => {
  it.each(['badge', 'indicator'] as const)(
    'regression: %s shows the same labelled instant as the drawer, not unlabelled local time',
    (variant) => {
      render(() =>
        variant === 'badge' ? (
          <BackupStatusCell lastBackup={completed} />
        ) : (
          <BackupIndicator lastBackup={completed} isTemplate={false} />
        ),
      );
      fireEvent.click(screen.getByRole('button', { name: /^(Backup status|Last backup):/ }));
      const time = screen.getByRole('dialog').querySelector('time')!;
      expect(time).toHaveAttribute('datetime', iso);
      expect(time.textContent).toBe(label);
      expect(time.textContent).toBe(
        getGuestDrawerBackupPresentation(completed, undefined, now).dateLabel,
      );
    },
  );

  it.each(['2026-10-11T17:30:00-07:00', '2026-10-12T09:30:00+09:00', '2026-10-12T00:30:00Z'])(
    'regression: equivalent producer time %s retains one UTC date and instant',
    (value) => {
      render(() => <BackupStatusCell lastBackup={value} />);
      fireEvent.click(screen.getByRole('button', { name: /^Backup status:/ }));
      const time = screen.getByRole('dialog').querySelector('time')!;
      expect(time).toHaveAttribute('datetime', iso);
      expect(time.textContent).toBe(label);
      expect(time.textContent).toBe(
        getGuestDrawerBackupPresentation(value, undefined, now).dateLabel,
      );
    },
  );

  it('keeps a mounted disclosure tied to the completion, not activity or the age clock', () => {
    const [value, setValue] = createSignal<string | number>(completed);
    const [running, setRunning] = createSignal(false);
    render(() => <BackupStatusCell lastBackup={value()} backupRunning={running()} />);
    fireEvent.click(screen.getByRole('button', { name: /^Backup status:/ }));
    const dialog = screen.getByRole('dialog');
    const time = () => dialog.querySelector('time');
    expect(time()?.textContent).toBe(label);
    setRunning(true);
    expect(dialog).toHaveTextContent('Backup running now');
    vi.advanceTimersByTime(60_000);
    expect(time()).toHaveAttribute('datetime', iso);
    expect(time()?.textContent).toBe(label);
    expect(dialog).toHaveTextContent('1 hour ago');
    setRunning(false);
    expect(time()).toHaveAttribute('datetime', iso);
    setValue('2026-10-12T01:45:00Z');
    expect(time()?.textContent).toBe('2026-10-12 01:45:00 UTC');
    expect(time()).toHaveAttribute('datetime', '2026-10-12T01:45:00.000Z');
    expect(dialog).not.toHaveTextContent(label);
  });

  it('withdraws unknown/future/absent times and re-admits only the supplied completion', () => {
    const [value, setValue] = createSignal<string | number>(completed);
    render(() => <BackupStatusCell lastBackup={value()} backupRunning />);
    fireEvent.click(screen.getByRole('button', { name: /^Backup status:/ }));
    const dialog = screen.getByRole('dialog');
    for (const invalid of ['invalid', NaN, Infinity, -1, 8.64e15 + 1]) {
      setValue(invalid);
      expect(dialog.querySelector('time')).toBeNull();
      expect(dialog).toHaveTextContent('invalid timestamp');
      expect(dialog).not.toHaveTextContent('Invalid Date');
    }
    setValue(0);
    expect(dialog.querySelector('time')).toBeNull();
    expect(dialog).toHaveTextContent('No completed backup yet');
    setValue(now + RELATIVE_TIME_TICK_MS);
    expect(dialog.querySelector('time')).toBeNull();
    expect(dialog).toHaveTextContent('timestamp is in the future');
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(dialog.querySelector('time')).toHaveAttribute('datetime', '2026-10-12T02:00:30.000Z');
    expect(dialog.querySelector('time')?.textContent).toBe('2026-10-12 02:00:30 UTC');
    expect(dialog).toHaveTextContent('Backup running now');
    expect(dialog).toHaveTextContent('0s ago');
  });
});
