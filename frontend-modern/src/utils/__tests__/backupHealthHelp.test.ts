import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (relative: string): string => readFileSync(path.join(repoRoot, relative), 'utf8');
const guide = read('docs/PBS.md');
const heading = 'Backups are visible but Coverage says Unprotected';
const fragment = 'backups-are-visible-but-coverage-says-unprotected';
const render = (markdown: string, slug: string) => {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(markdown, slug);
  return article;
};

// The existing PBS protection-help tests cover its safety content. These
// checks cover the new entry points without maintaining a second guide.
describe('backup health help', () => {
  it('ships one authoritative PBS section in the same public guide mirrors', () => {
    const target = render(guide, 'PBS');
    expect(target.querySelector(`#${fragment}`)?.textContent).toBe(heading);
    expect(target.querySelectorAll(`[id="${fragment}"]`)).toHaveLength(1);
    expect(target.querySelector('#backup-health-disagrees-with-visible-pbs-backups')).toBeNull();
    for (const slug of ['PBS', 'RECOVERY', 'TROUBLESHOOTING']) {
      expect(read(`frontend-modern/public/docs/${slug}.md`)).toBe(read(`docs/${slug}.md`));
    }
  });

  it('routes every named health-help entry to the authoritative PBS check', () => {
    for (const slug of ['RECOVERY', 'TROUBLESHOOTING']) {
      // Guides can repeat the entry and other advice can share its destination.
      // Check every named entry without assuming document order or uniqueness.
      const links = Array.from(render(read(`docs/${slug}.md`), slug).querySelectorAll('a')).filter(
        (link) => link.textContent === 'backup health checks',
      );
      expect(links.length).toBeGreaterThan(0);
      for (const link of links) {
        expect(link.getAttribute('href')).toBe(`/docs/PBS#${fragment}`);
        expect(link.hasAttribute('data-doc-link')).toBe(true);
      }
    }
  });
});
