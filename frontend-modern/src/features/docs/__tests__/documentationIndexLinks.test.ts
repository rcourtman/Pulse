import { readFileSync, existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const shipped = path.join(root, 'frontend-modern/public/docs');
const source = readFileSync(
  process.env.PULSE_DOC_INDEX_TEST_INPUT ?? path.join(shipped, 'README.md'),
  'utf8',
);
const index = document.createElement('article');
index.innerHTML = renderDocMarkdown(source, 'README');
const links = [...index.querySelectorAll<HTMLAnchorElement>('a[data-doc-link]')];

describe('the shipped documentation index', () => {
  it('keeps the repository and installed entrypoints identical', () => {
    expect(readFileSync(path.join(root, 'docs/README.md'), 'utf8')).toBe(
      readFileSync(path.join(shipped, 'README.md'), 'utf8'),
    );
  });

  // Exercise actual rendered destinations, including ../ paths flattened by
  // the production renderer. A repository file alone is not a shipped asset.
  it.each(links.map((link) => [link.textContent, link.getAttribute('href')!] as const))(
    '%s resolves to documentation included in the installation',
    (_label, href) => {
      const url = new URL(href, 'https://pulse.example.invalid');
      const relative = decodeURIComponent(url.pathname.slice('/docs/'.length));
      expect(existsSync(path.join(shipped, `${relative}.md`)), href).toBe(true);
    },
  );

  it.each([
    ['Published release notes on GitHub', 'https://github.com/rcourtman/Pulse/releases'],
    ['Product screenshots on GitHub', 'https://github.com/rcourtman/Pulse/blob/main/docs/SCREENSHOTS.md'],
  ])('%s explicitly leaves the installed viewer', (label, href) => {
    const link = [...index.querySelectorAll('a')].find((item) => item.textContent === label);
    expect(link?.getAttribute('href')).toBe(href);
    expect(link?.hasAttribute('data-doc-link')).toBe(false);
    expect(link?.getAttribute('target')).toBe('_blank');
    expect(link?.getAttribute('rel')?.split(' ')).toEqual(
      expect.arrayContaining(['noopener', 'noreferrer']),
    );
  });

  it('distinguishes published notes from source drafts without pinning a version', () => {
    expect(index.textContent?.replace(/\s+/g, ' ')).toContain(
      "A release packet in the source repository can be an unpublished draft; use the published release's notes before choosing an update.",
    );
  });
});
