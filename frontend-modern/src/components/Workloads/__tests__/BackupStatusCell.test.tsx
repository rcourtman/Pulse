import { describe, it, expect, afterEach } from 'vitest';
import { render, cleanup } from '@solidjs/testing-library';

import { BackupStatusCell } from '../GuestRowCells';

const HOUR_MS = 60 * 60 * 1000;

const renderCell = (lastBackup: string | number | null) => {
  const { container } = render(() => <BackupStatusCell lastBackup={lastBackup} />);
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

  it('draws the age and pill chrome when the backup is critical', () => {
    const badge = renderCell(Date.now() - 200 * HOUR_MS);

    expect(badge.textContent?.trim()).not.toBe('');
    expect(badge.className).toContain('rounded-full');
    expect(badge.className).toContain('border-red-200');
  });

  it('labels a guest that has never been backed up', () => {
    const badge = renderCell(null);

    expect(badge.textContent?.trim()).toBe('None');
    expect(badge.className).toContain('rounded-full');
    expect(badge.getAttribute('aria-label')).toBe('Backup status: never');
  });
});
