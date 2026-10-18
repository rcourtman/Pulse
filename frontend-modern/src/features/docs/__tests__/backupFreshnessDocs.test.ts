import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createRoot } from 'solid-js';
import { describe, expect, it } from 'vitest';
import { getBackupInfo } from '@/utils/format';
import { ALERT_THRESHOLDS_SECTION_TITLE_BACKUPS } from '@/utils/alertThresholdsPresentation';
import { useThresholdsRecoveryDefaultsState } from '@/features/alerts/thresholds/hooks/useThresholdsRecoveryDefaultsState';
import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import { createGuideFixture } from './guideFixture';

const read = (name: string) =>
  readFileSync(resolve(process.cwd(), '../docs', `${name}.md`), 'utf8');
const article = createGuideFixture(read);
const text = (node: Element) => node.textContent!.replace(/\s+/g, ' ').trim();
const now = new Date('2026-10-12T08:00:00Z');
const hour = 3_600_000;

describe('backup-cycle help matches the existing controls', () => {
  it('names the actual Recovery group and separates display hours from alert days', () => {
    const guide = article('PBS');
    const heading = guide.querySelector('#backup-age-colours-for-a-longer-backup-cycle');
    expect(heading).not.toBeNull();
    expect(text(guide)).toContain(
      `Alerts → Thresholds → Proxmox, then expand ${ALERT_THRESHOLDS_SECTION_TITLE_BACKUPS}`,
    );
    expect(text(guide)).toContain('Global Defaults');
    const scroll = heading!.nextElementSibling!.nextElementSibling!.nextElementSibling!;
    expect(scroll.hasAttribute('data-doc-table-scroll')).toBe(true);
    const table = scroll.querySelector('table')!;
    expect(table).not.toBeNull();
    expect(table.tagName).toBe('TABLE');
    expect([...table.querySelectorAll('tbody tr td:first-child')].map(text)).toEqual([
      'Fresh Hours',
      'Stale Hours',
      'Warning Days / Critical Days',
    ]);
    expect(text(table)).toContain("not the shield's freshness window");
    expect(readFileSync(resolve(process.cwd(), 'public/docs/PBS.md'), 'utf8')).toBe(read('PBS'));
  });

  it('reproduces the one-day badge and validates the documented three-day boundary', () => {
    const completed = now.getTime() - 25 * hour;
    expect(getBackupInfo(completed, undefined, now).status).toBe('stale');
    const thresholds = { freshHours: 72, staleHours: 96 };
    expect(getBackupInfo(completed, thresholds, now).status).toBe('fresh');
    for (const [hours, expected] of [
      [72, 'fresh'],
      [73, 'stale'],
      [96, 'stale'],
      [97, 'overdue'],
    ] as const) {
      expect(getBackupInfo(now.getTime() - hours * hour, thresholds, now).status).toBe(expected);
    }
    expect(getBackupInfo(completed, thresholds, now).ageMs).toBe(25 * hour);
    expect(text(article('PBS'))).toContain('Fresh Hours = 72');
    expect(text(article('PBS'))).toContain('Stale Hours = 96');
  });

  it('does not turn missing, future or invalid evidence into a fresh backup', () => {
    const thresholds = { freshHours: 72, staleHours: 96 };
    expect(getBackupInfo(null, thresholds, now).status).toBe('never');
    expect(getBackupInfo('invalid', thresholds, now).status).toBe('unknown');
    expect(getBackupInfo(now.getTime() + hour, thresholds, now).status).toBe('unknown');
    const guide = article('PBS');
    expect(text(guide)).toContain('missing or unknown backup evidence is not made healthy');
    expect(
      guide.querySelector('a[href="#backups-are-visible-but-coverage-says-unprotected"]'),
    ).not.toBeNull();
    expect(text(guide)).toContain(
      'Do not run another backup, change retention, disable alerts or clear History',
    );
  });

  it('retains the stable-version caution while the repaired editor preserves Off', () => {
    const config = {
      enabled: true,
      warningDays: 3,
      criticalDays: 0,
      freshHours: 72,
      staleHours: 96,
    };
    const result = createRoot((dispose) => {
      const state = useThresholdsRecoveryDefaultsState({
        backupDefaults: () => config,
        snapshotDefaults: () => ({ enabled: false, warningDays: 0, criticalDays: 0 }),
      } as ThresholdsTableProps);
      const sanitized = state.sanitizeBackupConfig(config);
      dispose();
      return sanitized;
    });
    expect(result).toMatchObject(config);
    const guide = text(article('PBS'));
    expect(guide).toContain('v6.5.0 caution:');
    expect(guide).toContain('do not save threshold edits on that version');
    expect(guide).toContain('Cancel rather than changing alert policy');
  });
});
