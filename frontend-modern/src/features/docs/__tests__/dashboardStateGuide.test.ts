import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const docs =
  process.env.PULSE_DASHBOARD_HELP_DIR || path.join(repoRoot, 'frontend-modern/public/docs');
const fragment = 'dashboard-stays-on-connecting-to-pulse-or-stops-updating';
const guide = (name: string): HTMLElement => {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(readFileSync(path.join(docs, `${name}.md`), 'utf8'), name);
  return article;
};
const section = (): HTMLElement => {
  const heading = guide('REVERSE_PROXY').querySelector(`#${fragment}`);
  expect(heading).not.toBeNull();
  const result = document.createElement('section');
  for (
    let node = heading!.nextElementSibling;
    node && node.tagName !== 'H3';
    node = node.nextElementSibling
  )
    result.appendChild(node.cloneNode(true));
  return result;
};
const text = (element: HTMLElement): string =>
  (element.textContent || '').replace(/\s+/g, ' ').trim();

describe('dashboard connection and state-delivery help', () => {
  it('provides a shipped symptom route before password recovery', () => {
    const article = guide('TROUBLESHOOTING');
    const link = article.querySelector(`a[href="/docs/REVERSE_PROXY#${fragment}"]`);
    expect(link).not.toBeNull();
    expect(text(link!.closest('li')!)).toContain('no repeat outage, restart or raw network dump');
    expect(link!.hasAttribute('data-doc-link')).toBe(true);
    expect(
      link!.compareDocumentPosition(article.querySelector('a[href$="#i-forgot-my-password"]')!) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it('does not equate login, health or socket welcome with state delivery', () => {
    const prose = text(section());
    expect(prose).toContain('does not prove that the dashboard received fresh monitoring state');
    expect(prose).toContain(
      '101 upgrade or welcome message establishes a connection, not state delivery',
    );
    expect(prose).toContain('A green healthcheck does not clear it');
  });

  it('separates rejected upgrades, absent state and already-observed independent comparisons', () => {
    const rows = [...section().querySelectorAll('li')].map(text);
    expect(rows).toHaveLength(3);
    expect(rows[0]).toContain('proxy, session or origin checks');
    expect(rows[1]).toContain('only welcome arrives or readings stop changing');
    expect(rows[2]).toContain('do not ask someone to repeat it');
    expect(rows[2]).toContain('recovery after restart does not establish its cause');
    expect(rows[2]).toContain("Do not publish Pulse's backend port or bypass HTTPS");
  });

  it('keeps credentials and bulk diagnostics private and avoids a lock-holder diagnosis', () => {
    const prose = text(section());
    expect(prose).toContain(
      'Keep cookies, tokens, full WebSocket payloads, HAR files, profiles and goroutine dumps private',
    );
    expect(prose).toContain(
      'A stack showing one waiting goroutine does not identify the lock holder or prove a deadlock',
    );
    expect(prose).toContain('Unknown is valid');
    expect(section().querySelector('pre')).toBeNull();
  });

  it('preserves working access, TLS and backup safety rather than inducing another failure', () => {
    const prose = text(section());
    expect(prose).toContain(
      'without weakening authentication, certificate verification or origin restrictions',
    );
    expect(prose).toContain('Keep any working administrator session');
    expect(prose).toContain(
      'Do not repeatedly reload, sign out, restart Pulse, enable Debug, reset configuration or run Diagnostics',
    );
    expect(prose).toContain('a healthy container does not establish guest or filesystem recovery');
  });

  it('resolves the existing help and safety links to shipped headings', () => {
    const links = [...section().querySelectorAll('a[href]')];
    expect(links).toHaveLength(2);
    for (const link of links) {
      const match = /^\/docs\/([^#]+)#(.+)$/.exec(link.getAttribute('href')!);
      expect(match).not.toBeNull();
      expect(guide(match![1]).querySelector(`[id="${match![2]}"]`)).not.toBeNull();
      expect(link.hasAttribute('data-doc-link')).toBe(true);
    }
    expect(
      guide('REVERSE_PROXY').querySelector(`a[href="/docs/REVERSE_PROXY#${fragment}"]`),
    ).not.toBeNull();
  });

  it('ships the exact canonical Markdown', () => {
    for (const name of ['REVERSE_PROXY', 'TROUBLESHOOTING'])
      expect(
        readFileSync(path.join(repoRoot, 'frontend-modern/public/docs', `${name}.md`), 'utf8'),
      ).toBe(readFileSync(path.join(repoRoot, 'docs', `${name}.md`), 'utf8'));
  });
});
