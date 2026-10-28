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

  it('routes required warning and scope guidance to real shipped headings', () => {
    const links = [...section().querySelectorAll('a')];
    // Require each existing job, but allow another useful link when its
    // destination also passes the real-viewer checks below.
    for (const [label, href] of [
      ['how Patrol works', '/docs/AI#how-patrol-works'],
      [
        'ordinary notification checks',
        '/docs/TROUBLESHOOTING#test-succeeds-but-real-alerts-are-missing',
      ],
      [
        'runtime, container and machine alert scopes',
        '/docs/DOCKER#runtime-and-container-alert-scopes',
      ],
    ]) {
      const matches = links.filter((link) => link.textContent === label);
      expect(matches, label).toHaveLength(1);
      expect(matches[0].getAttribute('href'), label).toBe(href);
    }
    for (const link of links) {
      const route = /^\/docs\/([^#]+)#(.+)$/.exec(link.getAttribute('href') ?? '');
      expect(route).not.toBeNull();
      expect(article(route![1]).querySelector(`[id="${route![2]}"]`)).not.toBeNull();
      expect(link.hasAttribute('data-doc-link')).toBe(true);
      expect(link.hasAttribute('target')).toBe(false);
    }
  });
});
