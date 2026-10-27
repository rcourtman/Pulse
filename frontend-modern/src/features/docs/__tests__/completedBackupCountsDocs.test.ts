import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import {
  buildBackupActivityTimeline,
  getBackupActivityUnknownCount,
} from '@/features/proxmox/proxmoxBackupActivityPresentation';
import { COVERAGE_COLUMNS } from '@/features/proxmox/proxmoxBackupsTablePresentation';
import {
  isCompletedRecoveryArtifact,
  selectWorkloadRecoveryArtifacts,
  type RecoverableArtifact,
} from '@/features/proxmox/proxmoxBackupRecoveryModel';
import { createGuideFixture } from './guideFixture';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const guide = createGuideFixture((name) => read(`docs/${name}.md`));
const section = () => {
  const article = guide('RECOVERY');
  const heading = article.querySelector('#why-proxmox-backup-counts-differ');
  expect(heading?.textContent).toBe('Why Proxmox backup counts differ');
  const result = document.createElement('section');
  for (
    let next = heading!.nextElementSibling;
    next && next.tagName !== 'H2';
    next = next.nextElementSibling
  ) {
    result.appendChild(next.cloneNode(true));
  }
  return result;
};
const text = () => section().textContent!.replace(/\s+/g, ' ');
const now = new Date('2026-10-24T12:00:00Z');
const point = (id: string, overrides: Partial<RecoverableArtifact> = {}): RecoverableArtifact => ({
  id,
  nativeId: id,
  sourceKind: 'pbs',
  sourceLabel: 'PBS',
  workload: { key: 'fixture', type: 'vm', typeLabel: 'VM', label: 'Fixture VM', vmid: '100' },
  createdAt: '2026-10-24T10:00:00Z',
  createdMs: Date.parse('2026-10-24T10:00:00Z'),
  location: 'fixture / (root)',
  detail: '',
  protected: false,
  ...overrides,
});

describe('completed backup count guidance', () => {
  it('ships the current visible labels and separates inventory, chart and list scope', () => {
    expect(read('frontend-modern/public/docs/RECOVERY.md')).toBe(read('docs/RECOVERY.md'));
    const copy = text();
    const tableSource = read('frontend-modern/src/features/proxmox/ProxmoxBackupsTable.tsx');
    for (const label of ['Completed restore points per day', 'Known restore volume per day']) {
      expect(copy).toContain(label);
      expect(tableSource).toContain(label);
    }
    expect(copy).not.toContain('Completed restore volume per day');
    expect(tableSource).toContain('aria-label="Clear date filter"');
    expect(copy).toContain(
      'covers the loaded inventory, not just the rows matching your list filters',
    );
    expect(copy).toContain(
      'The chart follows the search, node, type, location, source and entry-state filters',
    );
    expect(copy).toContain('Selecting a day narrows only the list, not the other chart days');
    expect(tableSource).toContain(
      'Activity follows the backup filters. Selecting a day narrows only the list.',
    );
    expect(copy).toContain('Selecting a chart day filters the list');
    expect(copy).toContain('Clear date filter');
    expect(copy).toContain(
      'This changes only the display, not collection, permissions or retention',
    );
    expect(copy).toContain('Older versions may use different labels');
  });

  it('keeps incomplete entries out of completed totals even with size and verification evidence', () => {
    const entries = [
      point('completed', { size: 100, verified: false }),
      point('undated', { sourceKind: 'archive', createdAt: '', createdMs: undefined }),
      point('snapshot', { sourceKind: 'snapshot' }),
      point('running', { running: true, protected: true, verified: true, size: 500 }),
      point('failed', { failed: true, protected: true, verified: true, size: 500 }),
    ];
    const completed = entries.filter(isCompletedRecoveryArtifact);
    expect(entries).toHaveLength(5);
    expect(completed.map((entry) => entry.id)).toEqual(['completed', 'undated', 'snapshot']);
    const classify = (entry: RecoverableArtifact) =>
      isCompletedRecoveryArtifact(entry) ? entry.sourceKind : null;
    const dated = buildBackupActivityTimeline(7, entries, (entry) => entry.createdMs, classify, {
      now,
    });
    const volume = buildBackupActivityTimeline(7, entries, (entry) => entry.createdMs, classify, {
      now,
      getValue: (entry) => entry.size ?? 0,
    });
    expect(dated.points.reduce((sum, day) => sum + day.total, 0)).toBe(2);
    expect(volume.points.reduce((sum, day) => sum + day.total, 0)).toBe(100);
    expect(volume.points.reduce((sum, day) => sum + getBackupActivityUnknownCount(day), 0)).toBe(1);
    const copy = text();
    expect(copy).toContain(
      'excludes both, even when an incomplete entry has a size or a verification flag',
    );
    expect(copy).toContain(
      'An undated completed point can count in Backup health but not in a daily column',
    );
    expect(copy).toContain('no reported size can count as a point without contributing volume');
    expect(copy).toContain('It remains selectable in Volume and counts normally in Count');
    expect(copy).toContain('Amber marks days with unknown sizes, not zero volume');
    expect(copy).toContain('its known volume is a lower bound, not a complete total');
    expect(copy).toContain('not PBS datastore usage or deduplicated physical space');
  });

  it('does not substitute a snapshot or older backup when chronology is unknown', () => {
    const known = point('known');
    const snapshot = point('snapshot', { sourceKind: 'snapshot' });
    const snapshotOnly = selectWorkloadRecoveryArtifacts([snapshot], now.getTime());
    expect(snapshotOnly.latestBackup).toBeUndefined();
    expect(snapshotOnly.latestSnapshot?.id).toBe('snapshot');
    for (const createdAt of ['', '2026-10-25T10:00:00Z']) {
      const unknown = point('unknown', { createdAt, createdMs: undefined });
      const selected = selectWorkloadRecoveryArtifacts([known, snapshot, unknown], now.getTime());
      expect(selected.latestBackup).toBeUndefined();
      expect(selected.ageUnknown?.backup).toBe(true);
    }
    expect(text()).toContain('They are not independent backups');
    const latestLabel = COVERAGE_COLUMNS.find((column) => column.id === 'latest')!.label;
    expect(text()).toContain(`do not fill Coverage's ${latestLabel}`);
    expect(text()).toContain('age stays unknown rather than choosing an older dated backup');
  });

  it('keeps display checks passive and links to shipped safety and evidence sections', () => {
    const copy = text();
    expect(copy).toContain('A blank chart is not proof that no backups exist');
    expect(copy).toContain('check any inventory loading or access warning first');
    expect(copy).toContain('Completed is not Verified, a tested restore or guest thaw');
    expect(copy).toContain(
      'Do not run a new backup, verification or restore, delete entries, or clear History',
    );
    expect(section().querySelector('pre')).toBeNull();
    for (const [name, anchor] of [
      ['PBS', 'backups-are-visible-but-coverage-says-unprotected'],
      ['VM_DISK_MONITORING', 'backup-safety'],
    ]) {
      expect(section().querySelector(`a[href="/docs/${name}#${anchor}"]`)).not.toBeNull();
      expect(guide(name).querySelector(`[id="${anchor}"]`)).not.toBeNull();
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
  });
});
