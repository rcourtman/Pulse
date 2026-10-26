import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { afterAll, describe, expect, it } from 'vitest';

import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const docsRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../public/docs');
const origin = 'https://docs.invalid';

function documentPaths(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const filename = path.join(directory, entry.name);
    if (entry.isDirectory()) return documentPaths(filename);
    return entry.isFile() && entry.name.endsWith('.md') ? [filename] : [];
  });
}

// A source-level heading scan accepts headings swallowed by an unclosed code
// fence. Check the HTML users receive, including nested shipped documents and
// their cross-document links, rather than maintaining a second slug algorithm.
describe.sequential('shipped document fragment destinations', () => {
  const filenames = new Map(
    documentPaths(docsRoot).sort().map((filename) => [
      path.relative(docsRoot, filename).replaceAll(path.sep, '/').slice(0, -3),
      filename,
    ]),
  );
  const documents = new Map<string, { article: HTMLElement; ids: Map<string, number> }>();
  let checked = 0;

  const renderedDocument = (slug: string) => {
    const cached = documents.get(slug);
    if (cached) return cached;
    const filename = filenames.get(slug);
    if (!filename) return undefined;
    const article = document.createElement('article');
    article.innerHTML = renderDocMarkdown(readFileSync(filename, 'utf8'), slug);
    const ids = new Map<string, number>();
    for (const element of article.querySelectorAll('[id]')) {
      ids.set(element.id, (ids.get(element.id) ?? 0) + 1);
    }
    const rendered = { article, ids };
    documents.set(slug, rendered);
    return rendered;
  };

  afterAll(() => documents.clear());

  // Bound each source document, not the aggregate cost of the entire corpus.
  // Targets still use the real renderer and are cached once, including their
  // duplicate-ID counts. No content, sanitizer or fragment check is skipped.
  it.each([...filenames.keys()])(
    'resolves every internal section link in %s to exactly one rendered destination',
    (slug) => {
      const { article } = renderedDocument(slug)!;
      const broken: Array<{ source: string; link: string; target: string; matches: number }> = [];
      for (const anchor of article.querySelectorAll<HTMLAnchorElement>('a[href]')) {
        const url = new URL(anchor.getAttribute('href')!, `${origin}/docs/${slug}`);
        if (url.origin !== origin || !url.pathname.startsWith('/docs/') || !url.hash) continue;
        const targetSlug = decodeURIComponent(url.pathname.slice('/docs/'.length));
        const fragment = decodeURIComponent(url.hash.slice(1));
        const matches = renderedDocument(targetSlug)?.ids.get(fragment) ?? 0;
        checked++;
        if (matches !== 1) {
          broken.push({
            source: slug,
            link: anchor.textContent?.replace(/\s+/g, ' ').trim() ?? '',
            target: url.pathname + url.hash,
            matches,
          });
        }
      }
      expect(broken).toEqual([]);
    },
    15_000,
  );

  it('covers the complete shipped corpus and checks real internal section links', () => {
    expect(documents.size).toBe(filenames.size);
    expect(checked).toBeGreaterThan(0);
  });

  it('keeps the audit section and its response separate from the token regeneration example', () => {
    const { article: api } = renderedDocument('API')!;
    expect(api.querySelector('#-audit-log-pro')?.textContent).toBe('🧾 Audit Log (Pro)');
    expect(api.querySelector('#list-audit-events')?.textContent).toBe('List Audit Events');
    const examples = [...api.querySelectorAll('pre code')].map((code) => code.textContent ?? '');
    const tokenExamples = examples.filter((code) => code.includes('New API token generated'));
    expect(tokenExamples).toHaveLength(1);
    expect(tokenExamples[0]).not.toContain('Audit Log');
    expect(JSON.parse(tokenExamples[0])).toMatchObject({ success: true, token: 'raw-token' });
    const auditExamples = examples.filter((code) => code.includes('"persistentLogging": true'));
    expect(auditExamples).toHaveLength(1);
    expect(JSON.parse(auditExamples[0])).toMatchObject({ total: 1, persistentLogging: true });
  });
});
