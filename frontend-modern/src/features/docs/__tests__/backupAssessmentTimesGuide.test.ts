import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const guide = createGuideFixture((name) => read(`frontend-modern/public/docs/${name}.md`));
const fragment = '#protection-assessment-times';
const section = () => {
  const heading = guide('RECOVERY').querySelector(fragment);
  expect(heading?.textContent).toBe('Protection assessment times');
  const result = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-3]$/.test(next.tagName)) break;
    result.appendChild(next.cloneNode(true));
  }
  return result;
};
const prose = () => section().textContent!.replace(/\s+/g, ' ');

describe('backup assessment time guidance', () => {
  it('routes the stale-protection symptom and archive comparison to the shipped explanation', () => {
    const link = guide('TROUBLESHOOTING').querySelector(`a[href="/docs/RECOVERY${fragment}"]`);
    expect(link?.textContent).toBe('recorded assessment times');
    expect(link?.hasAttribute('data-doc-link')).toBe(true);
    expect(link?.hasAttribute('target')).toBe(false);
    expect(guide('RECOVERY').querySelector(`a[href="${fragment}"]`)).not.toBeNull();
    for (const name of ['RECOVERY', 'TROUBLESHOOTING']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
  });

  it('explains all three assessment clocks without substituting task, collection or filter times', () => {
    const rows = [...section().querySelectorAll('ul > li')];
    expect(rows.map((row) => row.querySelector('strong')?.textContent)).toEqual([
      'Evaluated',
      'Latest successful point',
      'Last verified',
    ]);
    const text = prose();
    for (const distinction of [
      "UTC, not your browser's local timezone",
      'not when the backup completed or the provider last collected data',
      'A recent evaluation can use older or incomplete evidence',
      "not the latest task's result or necessarily Last backup",
      'These clocks are workload-wide',
      'not the assessment or its times',
      'compare the same instant with its timezone, not just the calendar date',
    ]) {
      expect(text).toContain(distinction);
    }
  });

  it('keeps missing, invalid and unavailable assessment evidence honest', () => {
    const text = prose();
    for (const distinction of [
      'Not reported means that clock was not supplied',
      'Unknown means its supplied value could not be interpreted',
      'Neither means zero, no backups or a failed verification',
      'Do not borrow another clock to fill it',
      'green connection does not establish a current assessment',
      'Older versions may not show these details',
    ]) {
      expect(text).toContain(distinction);
    }
  });

  it('uses passive comparison and preserves the independent guest-safety path', () => {
    const text = prose();
    expect(section().querySelector('pre')).toBeNull();
    expect(text).toContain('repeated names or VMIDs do not establish identity');
    expect(text).toContain('use existing provider records without rerunning collection');
    expect(text).toContain('do not run Diagnostics, another backup or a guest-agent probe');
    expect(text).toContain('restart monitoring or clear History');
    expect(text).toContain('not proof of a tested restore, application health or guest thaw');
    expect(text).toContain('does not establish safe workload recovery');
    const safety = section().querySelector('a[href="/docs/VM_DISK_MONITORING#backup-safety"]');
    expect(safety).not.toBeNull();
    expect(guide('VM_DISK_MONITORING').querySelector('#backup-safety')).not.toBeNull();
    const archive = section().querySelector('a[href="#local-pve-backups-are-newer-than-coverage"]');
    expect(archive).not.toBeNull();
    expect(guide('RECOVERY').querySelector(archive!.getAttribute('href')!)).not.toBeNull();
  });
});
