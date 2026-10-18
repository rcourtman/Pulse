import { afterEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { BackupIndicator, BackupStatusCell } from '../GuestRowCells';

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ getBackupThresholds: () => undefined }),
}));

const completed = Date.parse('2026-10-05T06:00:00Z');
const now = Date.parse('2026-10-05T08:00:00Z');

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('backup evidence access', () => {
  it.each(['badge', 'indicator'] as const)(
    'regression: %s exposes completed-backup details through a native button',
    async (variant) => {
      vi.useFakeTimers();
      vi.setSystemTime(now);
      render(() =>
        variant === 'badge' ? (
          <BackupStatusCell lastBackup={completed} />
        ) : (
          <BackupIndicator lastBackup={completed} isTemplate={false} />
        ),
      );
      const trigger = screen.getByRole('button', {
        name: variant === 'badge' ? /^Backup status:/ : /^Last backup:/,
      });
      expect(trigger).toHaveAttribute('type', 'button');
      expect(trigger).toHaveAttribute('aria-haspopup', 'dialog');
      expect(trigger).toHaveAttribute('aria-expanded', 'false');
      fireEvent.click(trigger);
      const dialog = screen.getByRole('dialog', { name: 'Backup status details' });
      expect(dialog).toHaveTextContent('Last completed backup');
      expect(dialog).toHaveTextContent(
        new Date(completed).toLocaleDateString(undefined, {
          weekday: 'short',
          year: 'numeric',
          month: 'short',
          day: 'numeric',
        }),
      );
      expect(dialog).toHaveTextContent('2 hours ago');
      expect(trigger).toHaveAttribute('aria-expanded', 'true');
      await Promise.resolve();
      expect(screen.getByRole('button', { name: 'Close backup details' })).toHaveFocus();
      fireEvent.keyDown(document, { key: 'Escape' });
      expect(screen.queryByRole('dialog')).toBeNull();
      expect(trigger).toHaveFocus();
    },
  );

  it('keeps open details reactive without interpreting running or uncertainty as completion', () => {
    vi.useFakeTimers();
    vi.setSystemTime(now);
    const [value, setValue] = createSignal<string | number>(completed);
    const [running, setRunning] = createSignal(false);
    render(() => <BackupStatusCell lastBackup={value()} backupRunning={running()} />);
    fireEvent.click(screen.getByRole('button', { name: /^Backup status:/ }));
    const dialog = screen.getByRole('dialog');
    setRunning(true);
    expect(dialog).toHaveTextContent('Backup running now');
    expect(dialog).toHaveTextContent('Last completed backup');
    setValue('not-a-date');
    expect(dialog).toHaveTextContent('invalid timestamp');
    expect(dialog).not.toHaveTextContent('Last completed backup');
    expect(dialog).not.toHaveTextContent('No backup has ever');
    setValue(now + 60000);
    expect(dialog).toHaveTextContent('timestamp is in the future');
    expect(dialog).not.toHaveTextContent('Invalid Date');
    setValue(0);
    expect(dialog).toHaveTextContent('No completed backup yet');
    setRunning(false);
    expect(dialog).toHaveTextContent('No backup has ever been recorded');
    expect(dialog).not.toHaveTextContent('Backup running now');
    setValue(completed);
    expect(dialog).toHaveTextContent('2 hours ago');
    expect(dialog).not.toHaveTextContent('unavailable');
  });

  it('uses separate disclosures for adjacent guests and omits template indicators', () => {
    vi.useFakeTimers();
    vi.setSystemTime(now);
    render(() => (
      <>
        <BackupStatusCell lastBackup={completed} />
        <BackupStatusCell lastBackup={0} />
        <BackupIndicator lastBackup={completed} isTemplate={true} />
      </>
    ));
    const triggers = screen.getAllByRole('button', { name: /^Backup status:/ });
    expect(triggers).toHaveLength(2);
    fireEvent.click(triggers[0]);
    fireEvent.click(screen.getByRole('button', { name: 'Close backup details' }));
    fireEvent.click(triggers[1]);
    expect(screen.getAllByRole('dialog')).toHaveLength(1);
    expect(screen.getByRole('dialog')).toHaveTextContent('No backup has ever been recorded');
    expect(screen.getByRole('dialog')).not.toHaveTextContent('2 hours ago');
    expect(screen.queryByRole('button', { name: /^Last backup:/ })).toBeNull();
  });
});
