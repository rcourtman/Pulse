import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const publicDocs = path.resolve(process.cwd(), 'public/docs');
const read = (name: string) => readFileSync(path.join(publicDocs, `${name}.md`), 'utf8');
const guide = createGuideFixture(read);

describe('shipped documentation index', () => {
  it('routes every local index link to a shipped guide and an existing fragment', () => {
    const links = [...guide('README').querySelectorAll<HTMLAnchorElement>('a[data-doc-link]')];
    expect(links.length).toBeGreaterThan(40);
    const missing: string[] = [];
    for (const link of links) {
      const url = new URL(link.getAttribute('href')!, 'https://pulse.invalid');
      const target = decodeURIComponent(url.pathname.slice('/docs/'.length));
      if (!existsSync(path.join(publicDocs, `${target}.md`))) {
        missing.push(`${link.textContent}: ${target}.md is not shipped`);
      } else if (url.hash && !guide(target).querySelector(`[id="${url.hash.slice(1)}"]`)) {
        missing.push(`${link.textContent}: ${target}${url.hash} is absent`);
      }
    }
    expect(missing).toEqual([]);
  });

  it('uses labelled online references rather than unshipped routes or draft release packets', () => {
    const article = guide('README');
    for (const [label, url] of [
      [
        'Demo screenshots (GitHub)',
        'https://github.com/rcourtman/Pulse/blob/main/docs/SCREENSHOTS.md',
      ],
      ['Published release notes (GitHub)', 'https://github.com/rcourtman/Pulse/releases'],
    ]) {
      const link = [...article.querySelectorAll('a')].find(
        (anchor) => anchor.textContent === label,
      );
      expect(link?.getAttribute('href')).toBe(url);
      expect(link?.getAttribute('target')).toBe('_blank');
      expect(link?.getAttribute('rel')).toBe('noopener noreferrer');
      expect(link?.hasAttribute('data-doc-link')).toBe(false);
    }
    expect(article.textContent?.replace(/\s+/g, ' ')).toContain(
      'open online resources in a new tab and need network access',
    );
    expect(article.textContent?.replace(/\s+/g, ' ')).toContain(
      'current content can differ from your installed version',
    );
  });

  it('keeps the repository and shipped index identical', () => {
    expect(read('README')).toBe(
      readFileSync(path.resolve(process.cwd(), '../docs/README.md'), 'utf8'),
    );
  });
});
