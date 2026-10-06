import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { beforeAll, describe, expect, it } from 'vitest';

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

function renderedDocuments(): Map<string, HTMLElement> {
  return new Map(
    documentPaths(docsRoot).map((filename) => {
      const slug = path.relative(docsRoot, filename).replaceAll(path.sep, '/').slice(0, -3);
      const article = document.createElement('article');
      article.innerHTML = renderDocMarkdown(readFileSync(filename, 'utf8'), slug);
      return [slug, article];
    }),
  );
}

// A source-level heading scan accepts headings swallowed by an unclosed code
// fence. Check the HTML users receive, including nested shipped documents and
// their cross-document links, rather than maintaining a second slug algorithm.
describe('shipped document fragment destinations', () => {
  let documents: Map<string, HTMLElement>;
  // Render the 62-file corpus once, including the long security-review guide.
  // This is a bounded content audit, not a per-document rendering benchmark.
  beforeAll(() => {
    documents = renderedDocuments();
  }, 15_000);

  it('resolves every internal section link to exactly one rendered destination', () => {
    const broken: Array<{ source: string; link: string; target: string; matches: number }> = [];
    let checked = 0;
    for (const [slug, article] of documents) {
      for (const anchor of article.querySelectorAll<HTMLAnchorElement>('a[href]')) {
        const url = new URL(anchor.getAttribute('href')!, `${origin}/docs/${slug}`);
        if (url.origin !== origin || !url.pathname.startsWith('/docs/') || !url.hash) continue;
        const targetSlug = decodeURIComponent(url.pathname.slice('/docs/'.length));
        const fragment = decodeURIComponent(url.hash.slice(1));
        const target = documents.get(targetSlug);
        const matches = target
          ? [...target.querySelectorAll('[id]')].filter((element) => element.id === fragment).length
          : 0;
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
    }
    expect(checked).toBeGreaterThan(0);
    expect(broken).toEqual([]);
  });

  it('keeps the audit section and its response separate from the token regeneration example', () => {
    const api = documents.get('API')!;
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
