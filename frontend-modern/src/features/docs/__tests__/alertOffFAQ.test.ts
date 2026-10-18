import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const guide = (name: string): string =>
  readFileSync(path.join(root, 'frontend-modern/public/docs', `${name}.md`), 'utf8');
const article = (name: string): HTMLElement => {
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(guide(name), name);
  return element;
};
const section = (): HTMLElement => {
  const heading = article('FAQ').querySelector('#can-i-disable-alerts-for-specific-metrics');
  expect(heading).not.toBeNull();
  const element = document.createElement('section');
  for (
    let next = heading!.nextElementSibling;
    next && !/^H[1-6]$/.test(next.tagName);
    next = next.nextElementSibling
  ) {
    element.appendChild(next.cloneNode(true));
  }
  return element;
};

describe('shipped alert Off FAQ', () => {
  it('separates the alert check, resource overrides, notification queue and Patrol', () => {
    const element = section();
    const text = element.textContent!.replace(/\s+/g, ' ');
    expect(text).toContain('not every default has a per-resource control');
    expect(text).toContain('save, then reopen the editor');
    expect(element.querySelectorAll('li')).toHaveLength(4);
    for (const phrase of [
      'does not hide its readings, stop collection',
      'explicit threshold can remain active',
      'Already queued notifications',
      'Patrol findings are separate from threshold alerts',
      'when the corresponding alert default is Off',
      'Do not induce an alert, send another Test or retry the queue',
    ]) {
      expect(text).toContain(phrase);
    }
    expect(element.querySelector('pre')).toBeNull();
    expect(guide('FAQ')).toBe(readFileSync(path.join(root, 'docs/FAQ.md'), 'utf8'));
  });

  it('routes the two independent warning paths to real shipped headings', () => {
    const links = [...section().querySelectorAll('a')];
    expect(links).toHaveLength(2);
    expect(links.map((link) => link.textContent)).toEqual([
      'how Patrol works',
      'ordinary notification checks',
    ]);
    for (const link of links) {
      const [name, fragment] = link.getAttribute('href')!.replace('/docs/', '').split('#');
      expect(article(name).querySelector(`#${fragment}`)).not.toBeNull();
      expect(link.hasAttribute('data-doc-link')).toBe(true);
      expect(link.hasAttribute('target')).toBe(false);
    }
  });
});
