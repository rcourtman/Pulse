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
    // Keep the native fragment href, but use the same explicit re-activation
    // path as file-qualified links. Docs navigation tests cover repeat focus.
    expect(link?.getAttribute('href')).toBe(`#${id}`);
    expect(link?.hasAttribute('data-doc-link')).toBe(true);
    expect(link?.hasAttribute('target')).toBe(false);
    expect(article.querySelector(`#${id}`)?.textContent).toBe(
      'Token-file format is not shell syntax',
    );
    expect(guide).toBe(
      readFileSync(path.join(repoRoot, 'frontend-modern/public/docs/UNIFIED_AGENT.md'), 'utf8'),
    );
  });

  it('renders every token-file form and keeps damaged bare values distinct from valid reporting', () => {
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
    const expectedForms = [
      ['Bare token, possibly followed by a newline', 'does not prove it matches'],
      ['Token surrounded by single or double quotes', 'The quotes remain part'],
      ['Assignment prefix such as PULSE_TOKEN= or export', 'The prefix remains part'],
      [
        'Duplicated or missing characters, or whitespace inside a bare value',
        "trimming the file's ends does not repair them",
      ],
      ['Unknown', 'format and cause remain unknown'],
    ];
    expect(rows.map((row) => row.querySelector('td')?.textContent)).toEqual(
      expectedForms.map(([form]) => form),
    );
    for (const [form, phrase] of expectedForms) {
      const row = rows.find((item) => item.querySelector('td')?.textContent === form)!;
      expect(row.querySelectorAll('td')[1].textContent).toContain(phrase);
    }
    const text = parts
      .map((element) => element.textContent)
      .join(' ')
      .replace(/\s+/g, ' ');
    expect(text).toContain('not the value, length, hint, screenshot or file');
    expect(text).toContain('Do not print, source, upload or edit the installed token file');
    expect(text).toContain('not proof of accepted reporting');
    expect(text).toContain('duplicated or changed middle characters can leave both ends matching');
    expect(text).toContain('matching hint do not prove full equality');
    expect(text).toContain('Do not apply this shape check to other token families');
    expect(text).toContain('rotate a working credential because it looks different');
    expect(parts.some((element) => element.matches('pre') || element.querySelector('pre'))).toBe(
      false,
    );
  });
});
