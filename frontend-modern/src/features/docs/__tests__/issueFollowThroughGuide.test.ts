import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown, stripLeadingTitle } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const shippedDocs = path.join(repoRoot, 'frontend-modern/public/docs');
const source = readFileSync(path.join(shippedDocs, 'AI_TRANSPARENCY.md'), 'utf8');

function renderGuide(name: string, markdown: string): HTMLElement {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(stripLeadingTitle(markdown), name);
  return article;
}

function followThrough(): HTMLElement {
  const article = renderGuide('AI_TRANSPARENCY', source);
  const heading = article.querySelector('#issue-resolution-and-retesting');
  expect(heading?.tagName).toBe('H2');
  const section = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (next.tagName === 'H2') break;
    section.appendChild(next.cloneNode(true));
  }
  return section;
}

const prose = (element: HTMLElement): string =>
  (element.textContent ?? '').replace(/\s+/g, ' ').trim();

describe('public issue follow-through guidance', () => {
  it('keeps resolution, stable-release retest and timeout distinct in rendered paragraphs', () => {
    const paragraphs = followThrough().querySelectorAll('p');
    expect(paragraphs).toHaveLength(4);
    expect(prose(paragraphs[0])).toContain('no reported symptom left unresolved');
    expect(prose(paragraphs[1])).toContain('start with that stable-release retest reply');
    expect(prose(paragraphs[1])).toContain('not the fix commit, a beta or a release candidate');
    const emphasis = paragraphs[2].querySelector('strong');
    expect(emphasis).not.toBeNull();
    expect(prose(emphasis!)).toBe('not confirmed resolution');
    expect(prose(paragraphs[2])).toContain('neither the reporter nor another person');
    expect(prose(paragraphs[2])).toContain('an unfixed or unexplained symptom');
  });

  it('keeps the existing issue and safe evidence route usable without new collection', () => {
    const section = followThrough();
    expect(prose(section)).toContain('original issue rather than filing it again');
    expect(prose(section)).toContain('unknown details are valid and credentials stay private');
    expect(prose(section)).toContain('do not repeat an outage, update or backup');
    expect(section.querySelector('pre, form, input, button')).toBeNull();
    const link = section.querySelector('a');
    expect(link?.getAttribute('href')).toBe('/docs/TROUBLESHOOTING#collect-diagnostics-safely');
    expect(link?.hasAttribute('data-doc-link')).toBe(true);
    expect(link?.hasAttribute('target')).toBe(false);
    const target = renderGuide(
      'TROUBLESHOOTING',
      readFileSync(path.join(shippedDocs, 'TROUBLESHOOTING.md'), 'utf8'),
    );
    expect(target.querySelector('#collect-diagnostics-safely')?.tagName).toBe('H3');
  });

  it('ships the same explanation at the standing triage footer destination', () => {
    expect(source).toBe(readFileSync(path.join(repoRoot, 'docs/AI_TRANSPARENCY.md'), 'utf8'));
    const article = renderGuide('AI_TRANSPARENCY', source);
    expect(article.querySelector('#how-changes-land-and-ship')).not.toBeNull();
    expect(article.querySelector('#authority-and-responsibility')).not.toBeNull();
  });
});
