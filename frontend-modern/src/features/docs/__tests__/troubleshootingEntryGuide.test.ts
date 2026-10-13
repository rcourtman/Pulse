import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown, stripLeadingTitle } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const shippedDocs = path.join(repoRoot, 'frontend-modern/public/docs');
const source = readFileSync(path.join(shippedDocs, 'TROUBLESHOOTING.md'), 'utf8');
const renderedGuides = new Map<string, HTMLElement>();

function renderGuide(name: string): HTMLElement {
  // Several destinations share a long guide. Render each immutable document
  // once, rather than making seventeen navigation checks parse it repeatedly.
  // entry() clones its section, so the cached article is never modified.
  const cached = renderedGuides.get(name);
  if (cached) return cached;
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(
    stripLeadingTitle(readFileSync(path.join(shippedDocs, `${name}.md`), 'utf8')),
    name,
  );
  renderedGuides.set(name, article);
  return article;
}

function entry(): HTMLElement {
  const article = renderGuide('TROUBLESHOOTING');
  const heading = article.querySelector('h2');
  expect(heading?.id).toBe('choose-the-affected-workflow');
  const section = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (next.tagName === 'H2') break;
    section.appendChild(next.cloneNode(true));
  }
  return section;
}

const prose = (element: HTMLElement): string =>
  (element.textContent ?? '').replace(/\s+/g, ' ').trim();

describe('troubleshooting workflow entry', () => {
  it('puts guest incident safety before the symptom links and any collection recipe', () => {
    const section = entry();
    const link = section.querySelector('a');
    expect(link?.getAttribute('href')).toBe('/docs/VM_DISK_MONITORING#backup-safety');
    expect(section.querySelectorAll('li')).toHaveLength(10);
    expect(section.querySelector('pre')).toBeNull();
    const beforeList = [...section.children]
      .slice(
        0,
        [...section.children].findIndex((element) => element.tagName === 'UL'),
      )
      .map((element) => prose(element as HTMLElement))
      .join(' ');
    expect(beforeList).toContain("an earlier guest request's state is uncertain");
    expect(beforeList).toContain(
      'Do not run Diagnostics, force guest-agent probes or repeat a backup',
    );
    expect(beforeList).toContain('responsive console alone does not establish safe recovery');
  });

  it('keeps server updates, workload updates and reporting acceptance distinct', () => {
    const section = entry();
    const rows = section.querySelectorAll('li');
    expect(prose(rows[1] as HTMLElement)).toContain(
      'An update of a monitored container is a different job',
    );
    const updateLinks = [...rows[1].querySelectorAll('a')].map((link) => link.getAttribute('href'));
    expect(updateLinks).toEqual([
      '/docs/AUTO_UPDATE#update-failed',
      '/docs/DOCKER#check-a-failed-or-pending-workload-update',
    ]);
    expect(prose(rows[1] as HTMLElement)).toContain(
      'Preserve the current state and rollback evidence',
    );
    expect(prose(rows[2] as HTMLElement)).toContain('Installer success is not an accepted report');
    expect(prose(rows[3] as HTMLElement)).toContain('A stopped new command does not diagnose the existing agent');
    expect(prose(rows[3] as HTMLElement)).toContain('do not repeat removal or installation');
    expect(rows[3].querySelector('a')?.getAttribute('href')).toBe(
      '/docs/UNIFIED_AGENT#copied-agent-command-fails-before-installation',
    );
    expect(prose(rows[4] as HTMLElement)).toContain(
      'a working Pulse Agent view does not prove API recovery',
    );
    expect(prose(rows[7] as HTMLElement)).toContain(
      'Guest usage and database size are different measurements',
    );
  });

  it('routes every entry to an existing shipped heading in the production viewer', () => {
    const links = [...entry().querySelectorAll('a[href]')];
    expect(links).toHaveLength(17);
    for (const link of links) {
      const href = link.getAttribute('href')!;
      const match = /^\/docs\/([^#]+)#(.+)$/.exec(href);
      expect(match, href).not.toBeNull();
      const target = renderGuide(match![1]).querySelector(`[id="${match![2]}"]`);
      expect(target, href).not.toBeNull();
      expect(target?.tagName, href).toMatch(/^H[1-6]$/);
      expect(link.hasAttribute('data-doc-link'), href).toBe(true);
      expect(link.hasAttribute('target'), href).toBe(false);
    }
  });

  it('keeps the source and shipped entry identical and preserves existing anchors', () => {
    const canonical = readFileSync(path.join(repoRoot, 'docs/TROUBLESHOOTING.md'), 'utf8');
    expect(source).toBe(canonical);
    const article = renderGuide('TROUBLESHOOTING');
    for (const id of [
      'i-forgot-my-password',
      'memory-use-keeps-growing',
      'collect-diagnostics-safely',
      'recovery-mode',
      '-getting-help',
    ]) {
      expect(article.querySelector(`[id="${id}"]`), id).not.toBeNull();
    }
    expect(prose(entry())).toContain('unknown or unavailable details are valid');
    expect(prose(entry())).toContain('Deleting the public copy does not revoke a credential');
    expect(prose(entry())).toContain('Do not trigger an alert or replay the queue');
  });
});
