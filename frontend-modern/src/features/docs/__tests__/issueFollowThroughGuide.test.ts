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
    expect(prose(paragraphs[1])).toContain('carrying the same fix');
    expect(prose(paragraphs[1])).toContain(
      'limit on repeated unanswered contact prevents another reply',
    );
    expect(prose(paragraphs[1])).toContain(
      "later of that request and the stable release's publication",
    );
    expect(prose(paragraphs[1])).toContain(
      'A beta or release candidate alone never starts the timeout, nor does a fix commit',
    );
    const emphasis = paragraphs[2].querySelector('strong');
    expect(emphasis).not.toBeNull();
    expect(prose(emphasis!)).toBe('not confirmed resolution');
    expect(prose(paragraphs[2])).toContain('at least 14 days without any reply from the reporter');
    expect(prose(paragraphs[2])).toContain(
      'no other person reporting the same problem since that retest request',
    );
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

  it('keeps the disclosure and its linked release guide on the weekly, no-early-cut policy', () => {
    const article = renderGuide('AI_TRANSPARENCY', source);
    const text = prose(article);
    expect(text).toContain(
      'Stable publication is targeted seven days after the last stable release',
    );
    expect(text).toContain('one day earlier, reserving its full 24-hour soak');
    expect(text).toContain(
      'An important fix ships in the next beta, not through an early candidate or stable release',
    );
    expect(text).toContain('unless a release-blocker requires a fresh candidate');
    expect(text).not.toContain('Every 14 days');
    expect(text).not.toContain('An early cut can bring');
    expect(
      article.querySelector('a[href="/docs/RELEASE_PROCESS"]')?.hasAttribute('data-doc-link'),
    ).toBe(true);
    const releaseSource = readFileSync(path.join(shippedDocs, 'RELEASE_PROCESS.md'), 'utf8');
    expect(releaseSource).toBe(
      readFileSync(path.join(repoRoot, 'docs/RELEASE_PROCESS.md'), 'utf8'),
    );
    const release = renderGuide('RELEASE_PROCESS', releaseSource);
    expect(prose(release)).toContain('Nothing brings that cut forward');
    expect(prose(release)).toContain('an important fix ships in the next daily beta');
    expect(prose(release)).toContain('Only a release-blocker requires a fresh candidate');
    expect(prose(release)).not.toContain('can bring');
  });
});
