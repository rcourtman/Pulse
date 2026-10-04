import { afterEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { getBackupInfo } from '@/utils/format';
import { BackupIndicator, BackupStatusCell } from '../GuestRowCells';
import { getGuestDrawerBackupPresentation } from '../guestDrawerModel';

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ getBackupThresholds: () => undefined }),
}));

const now = new Date('2026-10-04T08:00:00Z');
const future = now.getTime() + 60_000;
const invalidMessage = 'Backup time unavailable: invalid timestamp.';
const futureMessage =
  'Backup time unavailable: timestamp is in the future. Check the Proxmox and browser clocks.';

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('backup time evidence', () => {
  it.each([
    ['malformed', 'not-a-date', invalidMessage],
    ['NaN', NaN, invalidMessage],
    ['infinity', Infinity, invalidMessage],
    ['negative', -1, invalidMessage],
    ['outside Date range', 8.64e15 + 1, invalidMessage],
    ['future numeric', future, futureMessage],
    ['future ISO', new Date(future).toISOString(), futureMessage],
  ] as const)(
    'regression: %s is uncertainty, never freshness or absence',
    (_name, value, message) => {
      const info = getBackupInfo(value, undefined, now);
      expect(info).toEqual({ status: 'unknown', ageMs: null, ageFormatted: message });
      const drawer = getGuestDrawerBackupPresentation(value, undefined, now);
      expect(drawer.ageLabel).toBe(message);
      expect(drawer.ageClass).not.toContain('green');
      expect(drawer.ageClass).not.toContain('red');
      expect(drawer.dateLabel).toBe('Unknown');
    },
  );

  it.each([NaN, Infinity, new Date(NaN), 8.64e15 + 1])(
    'regression: an unusable current time %s cannot confer freshness',
    (clock) => {
      expect(getBackupInfo(now.getTime() - 3600_000, undefined, clock)).toEqual({
        status: 'unknown',
        ageMs: null,
        ageFormatted: 'Backup age unavailable: invalid current time.',
      });
    },
  );

  it.each([null, undefined, '', 0])('control: absence sentinel %s stays missing', (value) => {
    expect(getBackupInfo(value, undefined, now)).toEqual({
      status: 'never',
      ageMs: null,
      ageFormatted: 'Never',
    });
  });

  it.each([
    [0, 'fresh'],
    [24 * 3600_000, 'fresh'],
    [24 * 3600_000 + 1, 'stale'],
    [72 * 3600_000, 'stale'],
    [72 * 3600_000 + 1, 'overdue'],
  ] as const)('control: a valid age of %s retains %s', (age, status) => {
    const value = now.getTime() - age;
    expect(getBackupInfo(value, undefined, now).status).toBe(status);
    expect(getBackupInfo(new Date(value).toISOString(), undefined, now).status).toBe(status);
  });

  it('control: valid policy-relative thresholds stay in effect', () => {
    expect(
      getBackupInfo(now.getTime() - 15 * 24 * 3600_000, { freshHours: 16 * 24 }, now).status,
    ).toBe('fresh');
  });

  it.each([false, true])(
    'regression: row and indicator keep uncertainty while running=%s',
    (running) => {
      vi.useFakeTimers();
      vi.setSystemTime(now);
      const [timestamp, setTimestamp] = createSignal<string | number>(future);
      render(() => (
        <div>
          <BackupStatusCell lastBackup={timestamp()} backupRunning={running} />
          <BackupIndicator lastBackup={timestamp()} backupRunning={running} isTemplate={false} />
        </div>
      ));
      const badge = screen.getByLabelText(/^Backup status:/);
      expect(badge).toHaveTextContent(running ? 'Running' : 'Unknown');
      expect(badge).toHaveAccessibleName(new RegExp('timestamp is in the future'));
      expect(badge.className).not.toContain('green');
      const indicator = screen.getByLabelText(
        running ? /^Backup running now · backup time unavailable/ : /^Backup time unavailable/,
      );
      expect(indicator.className).not.toContain('green');

      setTimestamp('invalid');
      expect(badge).toHaveAccessibleName(new RegExp('invalid timestamp'));
      expect(badge).not.toHaveAccessibleName(/no completed backup/);
      setTimestamp(now.getTime() - 3600_000);
      expect(badge).not.toHaveAccessibleName(/unavailable/);
      expect(badge.className).toContain(running ? 'blue' : 'green');
      setTimestamp(0);
      expect(badge).toHaveTextContent(running ? 'Running' : 'None');
      expect(badge).toHaveAccessibleName(/no (completed )?backup/);
    },
  );
});
