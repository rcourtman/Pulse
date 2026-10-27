import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown, stripLeadingTitle } from '../docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const source = readFileSync(path.join(root, 'docs/MAIL_GATEWAY.md'), 'utf8');
const shipped = readFileSync(
  path.join(root, 'frontend-modern/public/docs/MAIL_GATEWAY.md'),
  'utf8',
);
const guide = (): HTMLElement => {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(stripLeadingTitle(shipped), 'MAIL_GATEWAY');
  return article;
};
const prose = (element: Element): string => (element.textContent ?? '').replace(/\s+/g, ' ');

describe('published PMG collection guidance', () => {
  it('distinguishes shipped scope controls from the older release limitation', () => {
    const article = guide();
    const text = prose(article);
    expect(shipped).toBe(source);
    expect(text).toContain('Published v6.6.0 and current source respect these saved switches');
    expect(text).toContain('Published v6.5.0 does not contain this scope correction');
    expect(text).toContain('all four off does not mean no PMG requests');
    expect(text).toContain('Published v6.6.0 records downstream read failures');
    expect(article.querySelectorAll('table')[0].querySelectorAll('tbody tr')).toHaveLength(4);
  });

  it('explains the visible upgrade failure without inventing network or queue recovery', () => {
    const article = guide();
    const upgrade = [...article.querySelectorAll('p')].find((p) =>
      p.textContent?.startsWith('Upgraded from v6.5.0 to v6.6.0?'),
    )!;
    expect(prose(upgrade)).toContain('same failure counts as a failed ordinary poll');
    expect(prose(upgrade)).toContain('even after a successful version read or connection test');
    expect(prose(upgrade)).toContain('do not establish that your network or credentials changed');
    expect(prose(upgrade)).toContain('or that queue readings were correct before the upgrade');
    const scope = upgrade.nextElementSibling!;
    expect(prose(scope)).toContain('not by repairing it');
    expect(prose(scope)).toContain('Keep independent queue monitoring');
    expect(prose(scope)).toContain('retained alerts do not establish current queue health');
    expect(prose(scope)).toContain('Do not repeatedly toggle collection');
    const link = scope.querySelector('a[href="#collection-scope-and-older-releases"]');
    expect(link?.textContent).toBe('saved collection scope');
    expect(link?.hasAttribute('target')).toBe(false);
    expect(article.querySelector('#collection-scope-and-older-releases')?.tagName).toBe('H3');
    expect(scope.querySelector('pre')).toBeNull();
  });
});
