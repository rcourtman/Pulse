import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { COVERAGE_COLUMNS } from '@/features/proxmox/proxmoxBackupsTablePresentation';
import { createGuideFixture } from './guideFixture';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const guide = createGuideFixture((name) =>
  name === 'RECOVERY' && process.env.PULSE_RECOVERY_GUIDE_TEST_INPUT
    ? readFileSync(process.env.PULSE_RECOVERY_GUIDE_TEST_INPUT, 'utf8')
    : read(`frontend-modern/public/docs/${name}.md`),
);
const fragment = '#backup-task-results-and-backup-age';
const section = () => {
  const heading = guide('RECOVERY').querySelector(fragment);
  expect(heading?.textContent).toBe('Backup task results and backup age');
  const result = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-3]$/.test(next.tagName)) break;
    result.appendChild(next.cloneNode(true));
  }
  return result;
};
const prose = () => section().textContent!.replace(/\s+/g, ' ');

describe('backup task evidence guidance', () => {
  it('keeps both discovery links usable and all three shipped mirrors equal', () => {
    for (const name of ['PBS', 'TROUBLESHOOTING']) {
      const link = guide(name).querySelector(`a[href="/docs/RECOVERY${fragment}"]`);
      expect(link?.textContent).toBe('task results and backup age');
    }
    for (const name of ['RECOVERY', 'PBS', 'TROUBLESHOOTING']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
  });

  it('separates the actual Coverage columns and preserves failure beside a recent backup', () => {
    const rows = [...section().querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(3);
    for (const id of ['latest', 'task', 'posture']) {
      const column = COVERAGE_COLUMNS.find((value) => value.id === id)!;
      expect(rows.some((row) => row.firstElementChild?.textContent?.includes(column.label))).toBe(
        true,
      );
    }
    const text = prose();
    for (const distinction of [
      'A recent backup and a failed task can both be genuine',
      'not the result of the latest task',
      'A guest snapshot does not fill this column',
      'A location filter does not recalculate this assessment',
      'Failed does not erase an earlier completed backup',
      'a recent backup age does not cancel the failure either',
      'Running is not a new completed recovery point',
      'OK is a task result, not verification',
    ]) {
      expect(text).toContain(distinction);
    }
  });

  it('explains uncertain ordering without turning absent evidence into a successful task', () => {
    const text = prose();
    for (const distinction of [
      'Latest task: Unknown',
      'missing, future or tied start times',
      'Possible latest task',
      'Its number is not a time order',
      'do not choose an OK task over a failed one',
      'None or a dash means no matching recent task is shown',
      'not that no task ran in the provider',
      'An Unknown status is not OK',
      'Not reported under Error is not proof that there was no error',
      'Status, Started, Duration, Error and Task ID',
      'same owning installation and guest',
    ]) {
      expect(text).toContain(distinction);
    }
  });

  it('keeps comparison passive, private and distinct from the guest-safety procedure', () => {
    const text = prose();
    for (const boundary of [
      'Use the records already available',
      'Do not run another backup, verification, restore or guest-agent probe',
      'restart monitoring, clear History or change retention',
      'Keep full task IDs, private paths and unredacted errors private',
      'stop this display comparison',
      'independent thaw, fresh successful writes to every covered filesystem and workload liveness',
    ]) {
      expect(text).toContain(boundary);
    }
    expect(section().querySelector('pre')).toBeNull();
    const safety = '#backup-safety';
    expect(section().querySelector(`a[href="/docs/VM_DISK_MONITORING${safety}"]`)).not.toBeNull();
    expect(guide('VM_DISK_MONITORING').querySelector(safety)).not.toBeNull();
  });
});
