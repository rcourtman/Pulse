import { describe, it, expect, afterEach } from 'vitest';
import { render, cleanup } from '@solidjs/testing-library';

import { BackupStatusCell } from '../GuestRowCells';

const HOUR_MS = 60 * 60 * 1000;

const renderCell = (lastBackup: string | number | null, backupRunning?: boolean) => {
  const { container } = render(() => (
    <BackupStatusCell lastBackup={lastBackup} backupRunning={backupRunning} />
  ));
  const badge = container.querySelector('span[aria-label^="Backup status"]');
  if (!badge) throw new Error('backup badge not rendered');
  return badge as HTMLElement;
};

describe('BackupStatusCell', () => {
  afterEach(() => {
    cleanup();
  });

  // The healthy state is the overwhelming majority of rows. It carries the
  // verdict in the shield colour alone; the age would be repeated decoration.
  it('renders a bare green shield with no age text when the backup is fresh', () => {
    const badge = renderCell(Date.now() - 2 * HOUR_MS);

    expect(badge.textContent?.trim()).toBe('');
    expect(badge.className).toContain('text-green-600');
    expect(badge.className).not.toContain('rounded-full');
    expect(badge.className).not.toContain('border');
    expect(badge.querySelector('svg')).not.toBeNull();
  });

  it('keeps the age available to assistive tech even when it is not drawn', () => {
    const badge = renderCell(Date.now() - 2 * HOUR_MS);

    expect(badge.getAttribute('aria-label')).toMatch(/^Backup status: fresh, last backup /);
  });

  it('draws the age and pill chrome when the backup is stale', () => {
    const badge = renderCell(Date.now() - 48 * HOUR_MS);

    expect(badge.textContent?.trim()).not.toBe('');
    expect(badge.className).toContain('rounded-full');
    expect(badge.className).toContain('border-yellow-200');
  });

  it('keeps the age and amber pill chrome when the backup is overdue', () => {
    const badge = renderCell(Date.now() - 200 * HOUR_MS);

    expect(badge.textContent?.trim()).not.toBe('');
    expect(badge.className).toContain('rounded-full');
    expect(badge.className).toContain('border-yellow-200');
    expect(badge.className).not.toContain('border-red-200');
  });

  it('labels a guest that has never been backed up in red', () => {
    const badge = renderCell(null);

    expect(badge.textContent?.trim()).toBe('None');
    expect(badge.className).toContain('rounded-full');
    expect(badge.className).toContain('border-red-200');
    expect(badge.getAttribute('aria-label')).toBe('Backup status: no backup found');
  });

  // A backup that merely STARTED must never wear the green shield - the
  // repro was an in-flight PBS snapshot at 9% reading as a healthy backup
  //. Running is its own blue state, and the last COMPLETED age
  // stays available to assistive tech.
  it('shows a blue Running pill instead of the green shield while a backup runs', () => {
    const badge = renderCell(Date.now() - 2 * HOUR_MS, true);

    expect(badge.textContent?.trim()).toBe('Running');
    expect(badge.className).toContain('border-blue-200');
    expect(badge.className).not.toContain('text-green-600');
    expect(badge.getAttribute('aria-label')).toMatch(
      /^Backup status: backup running now, last completed backup /,
    );
  });

  it('shows Running rather than red None while a first backup runs', () => {
    const badge = renderCell(null, true);

    expect(badge.textContent?.trim()).toBe('Running');
    expect(badge.className).toContain('border-blue-200');
    expect(badge.className).not.toContain('border-red-200');
    expect(badge.getAttribute('aria-label')).toBe(
      'Backup status: backup running now, no completed backup yet',
    );
  });

  it('keeps the completed-backup age when a stale guest starts backing up', () => {
    const badge = renderCell(Date.now() - 200 * HOUR_MS, true);

    // The badge shows Running, not the age - but the age must survive in
    // the accessible label so "running" never erases how old the last
    // completed backup is.
    expect(badge.textContent?.trim()).toBe('Running');
    expect(badge.getAttribute('aria-label')).toContain('last completed backup');
  });
});
