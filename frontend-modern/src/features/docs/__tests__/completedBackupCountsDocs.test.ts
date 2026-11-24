import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
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
  it('ships the current labels and separates inventory, job and list scope', () => {
    expect(read('frontend-modern/public/docs/RECOVERY.md')).toBe(read('docs/RECOVERY.md'));
    const copy = text();
    // The restore points carry no per-day chart; the guide must not describe one.
    for (const label of ['Completed restore points per day', 'Known restore volume per day']) {
      expect(copy).not.toContain(label);
    }
    expect(copy).toContain(
      'covers the loaded inventory, not just the rows matching your list filters',
    );
    expect(copy).toContain('Backup jobs counts retained Proxmox tasks for current workloads');
    expect(copy).toContain("List filters don't change it");
    expect(copy).toContain('so daily totals can differ');
    expect(copy).toContain(
      'The list follows the selected day and the By date search, Node, Backup location, Type and Source filters, including Verified or Unverified',
    );
    expect(copy).toContain('a non-zero total there can sit beside an empty filtered list');
    expect(copy).toContain(
      'An unresolved or ambiguous saved backup location shows no matching points',
    );
    expect(copy).toContain('choose the intended location again or clear that filter');
    expect(copy).toContain(
      'None of these display controls changes collection, permissions, retention or protection policy',
    );
    expect(copy).toContain('Older versions may use different labels');
  });

  it('separates unrated rows from an unknown assessment and links to its evidence', () => {
    const link = section().querySelector('a[href="#protection-posture"]');
    const rating = link?.closest('li');
    const copy = rating?.textContent?.replace(/\s+/g, ' ');
    expect(link?.textContent).toBe('protection evidence');
    expect(guide('RECOVERY').querySelector('#protection-posture')).not.toBeNull();
    expect(copy).toContain('Unknown counts towards the total but not as protected');
    expect(copy).toContain('Checking means Pulse is still waiting for a protection assessment');
    expect(copy).toContain('Not evaluated means Pulse has no linked resource to assess');
    expect(copy).toContain('neither means a workload was deliberately excluded from backups');
    expect(copy).toContain('Unmatched backups are also outside this ratio');
    expect(copy).toContain(
      '2 / 2 protected (rated only) beside 1 checking does not tell you whether the third workload is protected',
    );
  });

  it('separates fresh running activity, guest results and stale readings', () => {
    const fragment = '#backup-task-results-and-backup-age';
    const link = section().querySelector(`a[href="${fragment}"]`);
    expect(link?.textContent).toBe('no guest result');
    const jobCopy = link?.closest('li')?.textContent?.replace(/\s+/g, ' ');
    expect(jobCopy).toContain("Select a bar to show that day's restore points in By date");
    expect(jobCopy).toContain('Use Clear day above the list or select the same bar again');
    expect(jobCopy).toContain('Running can mean a fresh read with no guest result.');
    expect(jobCopy).toContain('Stale reads count as Unknown.');
    expect(guide('RECOVERY').querySelector(fragment)?.textContent).toBe(
      'Backup task results and backup age',
    );
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
    expect(text()).toContain(
      'excludes both, even when an incomplete entry has a size or a verification flag',
    );
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
    expect(copy).toContain('An empty list is not proof that no backups exist');
    expect(copy).toContain('check any inventory loading or access warning first');
    expect(copy).toContain(
      'Backup list not confirmed can appear even when both inventories have loaded',
    );
    expect(copy).toContain('Wait if Pulse is still checking protection status');
    expect(copy).toContain(
      'Protection status is unavailable, choose Retry protection status to repeat just that read, not run a backup',
    );
    expect(copy).toContain('Counts ending in read cover only the restore points Pulse could list');
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
