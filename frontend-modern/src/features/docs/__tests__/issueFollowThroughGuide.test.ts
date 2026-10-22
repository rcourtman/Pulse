import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown, stripLeadingTitle } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const shippedDocs = path.join(repoRoot, 'frontend-modern/public/docs');
const source = readFileSync(
  process.env.PULSE_ISSUE_GUIDE_TEST_INPUT ?? path.join(shippedDocs, 'AI_TRANSPARENCY.md'),
  'utf8',
);

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
    expect(paragraphs).toHaveLength(5);
    expect(prose(paragraphs[0])).toContain('no reported symptom left unresolved');
    expect(prose(paragraphs[1])).toContain('start with that stable-release retest reply');
    expect(prose(paragraphs[1])).toContain('not the fix commit');
    const emphasis = paragraphs[3].querySelector('strong');
    expect(emphasis).not.toBeNull();
    expect(prose(emphasis!)).toBe('not confirmed resolution');
    expect(prose(paragraphs[3])).toContain('at least 14 days without any reply from the reporter');
    expect(prose(paragraphs[3])).toContain(
      'no other person reporting the same problem since that retest request',
    );
    expect(prose(paragraphs[3])).toContain('an unfixed or unexplained symptom');
  });

  it('gives earlier preview requests a stable-bound clock without another reminder', () => {
    const paragraph = followThrough().querySelectorAll('p')[2];
    expect(prose(paragraph)).toContain('earlier retest request named a beta or release candidate');
    expect(prose(paragraph)).toContain('carrying the same fix');
    expect(prose(paragraph)).toContain('repeated unanswered follow-ups');
    expect(prose(paragraph.querySelector('strong')!)).toBe(
      "later of that request and the stable release's publication",
    );
    expect(prose(paragraph)).toContain(
      'stable release contains the fix for every reported symptom',
    );
    expect(prose(paragraph)).toContain(
      'A beta or release candidate alone never starts this period',
    );
    expect(prose(paragraph)).toContain('another unanswered reminder is not required');
  });

  it('keeps the fixed weekly target and beta access consistent with release help', () => {
    const article = renderGuide('AI_TRANSPARENCY', source);
    const releaseHelp = renderGuide(
      'RELEASE_PROCESS',
      readFileSync(path.join(shippedDocs, 'RELEASE_PROCESS.md'), 'utf8'),
    );
    for (const guide of [article, releaseHelp]) {
      const text = prose(guide);
      expect(text).toContain('seven days after the last stable release');
      expect(text).toContain('one day earlier');
      expect(text).toContain('full 24-hour soak');
      expect(text).toContain('Preparation and qualification can move publication later');
      expect(text).toContain(
        'An important fix does not bring the weekly candidate or stable release forward',
      );
      expect(text).toContain('published beta');
      expect(text).not.toMatch(/(?:early cut|important fix) can bring/i);
      expect(text).not.toContain('can bring that cut forward');
      expect(text).not.toContain('Every 14 days after the last stable release');
    }
    const releaseText = prose(releaseHelp);
    expect(releaseText).toContain("Work merged during the soak reaches the next cycle's betas");
    expect(releaseText).toContain('Only a candidate regression replaces a soaking candidate');
    expect(releaseText).toContain('an unrelated fix does not restart its soak');
    expect(releaseText).toContain('regression from the previous stable release or a failed release attempt');
    expect(article.querySelector('a[href="/docs/RELEASE_PROCESS"]')).not.toBeNull();
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
