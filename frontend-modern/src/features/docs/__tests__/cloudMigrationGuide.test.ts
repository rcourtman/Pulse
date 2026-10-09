import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const read = (file: string) => readFileSync(resolve(process.cwd(), '..', file), 'utf8');
const article = (name: string) => {
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(read(`docs/${name}.md`), name);
  return element;
};
const text = (element: Element) => element.textContent?.replace(/\s+/g, ' ') ?? '';

describe('Cloud configuration-transfer help', () => {
  it('ships the same scope warning before either migration direction', () => {
    expect(read('frontend-modern/public/docs/CLOUD.md')).toBe(read('docs/CLOUD.md'));
    const guide = article('CLOUD');
    const migration = guide.querySelector('#migrating-tofrom-cloud');
    const warning = migration?.nextElementSibling;
    expect(warning?.querySelector('strong')).not.toBeNull();
    expect(text(warning!)).toContain('not the full installation');
    expect(text(warning!)).toContain('agent inventory and enrolment state');
    expect(text(warning!)).toContain('TrueNAS, vSphere and Machine Availability');
    expect(text(guide)).not.toContain('fully portable');
  });

  it('keeps both ordered moves and their single-active, identity and recovery checks readable', () => {
    const guide = article('CLOUD');
    for (const name of ['Self-Hosted → Cloud', 'Cloud → Self-Hosted']) {
      const heading = [...guide.querySelectorAll('h3')].find((node) => text(node) === name);
      expect(heading?.nextElementSibling?.tagName).toBe('OL');
      expect(heading?.nextElementSibling?.querySelectorAll('li')).toHaveLength(5);
      expect(text(heading!.nextElementSibling!)).toContain('destination-local administrator access');
      expect(text(heading!.nextElementSibling!)).toContain('API-token records');
    }
    expect(text(guide)).toContain('single-active cutover before importing');
    expect(text(guide)).toContain('stop before importing');
    expect(text(guide)).toContain('rather than creating a replacement token');
    expect(text(guide)).toContain('settings may already have been written');
    expect(text(guide)).toContain('Do not induce alerts, replay a queue or run a workload action');
    expect(text(guide)).toContain('not proof of full recovery');
    expect(guide.querySelector('pre')).toBeNull();
  });

  it('routes every migration/retarget link to an existing guide and fragment', () => {
    const guide = article('CLOUD');
    for (const [name, fragment] of [
      ['MIGRATION', 'configuration-transfer'],
      ['MIGRATION', 'full-state-recovery'],
      ['MIGRATION', 'cut-over-and-verify'],
      ['UNIFIED_AGENT', 'moving-pulse-to-a-new-address'],
    ]) {
      const link = guide.querySelector(`a[href="/docs/${name}#${fragment}"]`);
      expect(link).not.toBeNull();
      expect(link?.hasAttribute('data-doc-link')).toBe(true);
      expect(article(name).querySelector(`[id="${fragment}"]`)).not.toBeNull();
    }
    expect(guide.querySelector('a[href="#verify-the-move-before-retiring-the-source"]')).not.toBeNull();
    expect(guide.querySelector('#verify-the-move-before-retiring-the-source')).not.toBeNull();
  });
});
