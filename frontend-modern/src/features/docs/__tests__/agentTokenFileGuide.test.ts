import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const guide = readFileSync(path.join(repoRoot, 'docs/UNIFIED_AGENT.md'), 'utf8');
const id = 'token-file-format-is-not-shell-syntax';

function renderGuide(): HTMLElement {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(guide, 'UNIFIED_AGENT');
  return article;
}

describe('agent token-file help in the shipped Markdown renderer', () => {
  it('makes the preparation link reach the local format heading, not an external page', () => {
    const article = renderGuide();
    const link = Array.from(article.querySelectorAll('a')).find(
      (anchor) => anchor.textContent === 'token-file format guidance',
    );
    expect(link?.getAttribute('href')).toBe(`/docs/UNIFIED_AGENT#${id}`);
    expect(link?.hasAttribute('data-doc-link')).toBe(true);
    expect(link?.hasAttribute('target')).toBe(false);
    expect(article.querySelector(`#${id}`)?.textContent).toBe('Token-file format is not shell syntax');
    expect(guide).toBe(
      readFileSync(path.join(repoRoot, 'frontend-modern/public/docs/UNIFIED_AGENT.md'), 'utf8'),
    );
  });

  it('renders all four forms as a table with headers and keeps private, unknown evidence valid', () => {
    const article = renderGuide();
    const heading = article.querySelector(`#${id}`)!;
    const parts: Element[] = [];
    for (let element = heading.nextElementSibling; element; element = element.nextElementSibling) {
      if (/^H[1-4]$/.test(element.tagName)) break;
      parts.push(element);
    }
    const table = parts.flatMap((element) => [...element.querySelectorAll('table')])[0];
    expect(table).toBeDefined();
    expect([...table.querySelectorAll('th')].map((cell) => cell.textContent)).toEqual([
      'File form',
      'What the agent reads',
    ]);
    const rows = [...table.querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(4);
    for (const [index, phrase] of [
      'does not prove it matches',
      'The quotes remain part',
      'The prefix remains part',
      'format and cause remain unknown',
    ].entries()) {
      expect(rows[index].textContent).toContain(phrase);
    }
    const text = parts.map((element) => element.textContent).join(' ').replace(/\s+/g, ' ');
    expect(text).toContain('not the value, length, hint, screenshot or file');
    expect(text).toContain('Do not print, source, upload or edit the installed token file');
    expect(text).toContain('not proof of accepted reporting');
    expect(parts.some((element) => element.matches('pre') || element.querySelector('pre'))).toBe(false);
  });
});
