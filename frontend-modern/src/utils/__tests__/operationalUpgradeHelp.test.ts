import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const read = (path: string) => readFileSync(resolve(process.cwd(), '..', path), 'utf8');

const article = (name: string) => {
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(read(`docs/${name}.md`), name);
  return element;
};

describe('Operational Trust upgrade help', () => {
  it('ships the same recovery and passive-check guidance in the in-app document', () => {
    expect(read('frontend-modern/public/docs/OPERATIONAL_TRUST.md')).toBe(
      read('docs/OPERATIONAL_TRUST.md'),
    );
    const guide = article('OPERATIONAL_TRUST');
    expect(guide.querySelector('#read-only-checks-after-upgrading')?.textContent).toBe(
      'Read-only checks after upgrading',
    );
    const warning = [...guide.querySelectorAll('strong')].find((element) =>
      element.textContent?.includes('merely to validate an upgrade'),
    );
    expect(warning?.textContent?.replace(/\s+/g, ' ')).toBe(
      'Do not acknowledge, suppress, retry, dismiss or clear records, send a Test, or approve or run an action merely to validate an upgrade.',
    );
    expect(guide.textContent?.replace(/\s+/g, ' ')).toContain(
      'Retry can send a real notification, suppression changes active attention, and an offered Docker restart changes a workload.',
    );
  });

  it('keeps every recovery, safety and reporting link on an existing in-app guide and anchor', () => {
    const guide = article('OPERATIONAL_TRUST');
    for (const [name, fragment, label] of [
      ['MIGRATION', 'full-state-recovery', 'full-state recovery guidance'],
      ['VM_DISK_MONITORING', 'backup-safety', 'guest-safety precaution'],
      ['AUTO_UPDATE', 'rollback', 'update and rollback guidance'],
      ['TROUBLESHOOTING', 'test-succeeds-but-real-alerts-are-missing', 'notification checks'],
      ['TROUBLESHOOTING', '-getting-help', 'safe issue reporting'],
    ]) {
      const link = guide.querySelector(`a[href="/docs/${name}#${fragment}"]`);
      expect(link?.textContent).toBe(label);
      expect(link?.hasAttribute('data-doc-link')).toBe(true);
      expect(article(name).querySelector(`[id="${fragment}"]`)).not.toBeNull();
    }
  });

  it('does not turn an unobserved event or a failed read into an upgrade-success claim', () => {
    const text = article('OPERATIONAL_TRUST').textContent!.replace(/\s+/g, ' ');
    expect(text).toContain('An empty queue does not establish complete or healthy collection');
    expect(text).toContain('record it as unverified rather than manufacture one');
    expect(text).toContain('A failed read or an empty queue alone is not a reason to restore data');
    expect(text).not.toContain('exercise notification retry/dead-letter monitoring');
    expect(text).not.toContain('complete a review/approve/run/verify journey');
  });
});
