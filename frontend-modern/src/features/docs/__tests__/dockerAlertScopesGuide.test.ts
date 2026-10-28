import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown, stripLeadingTitle } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');

function guide(name: string): HTMLElement {
  const source = readFileSync(path.join(repoRoot, `docs/${name}.md`), 'utf8');
  const shipped = readFileSync(
    path.join(repoRoot, `frontend-modern/public/docs/${name}.md`),
    'utf8',
  );
  expect(shipped).toBe(source);
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(stripLeadingTitle(shipped), name);
  return article;
}

function scopeSection(): HTMLElement {
  const article = guide('DOCKER');
  const heading = article.querySelector('#runtime-and-container-alert-scopes');
  expect(heading?.tagName).toBe('H2');
  const section = document.createElement('section');
  for (let next = heading!.nextElementSibling; next && next.tagName !== 'H2';) {
    const following = next.nextElementSibling;
    section.appendChild(next);
    next = following;
  }
  return section;
}

const prose = (element: Element): string => element.textContent!.replace(/\s+/g, ' ').trim();

describe('shipped Docker alert scope guidance', () => {
  it('routes the existing metric FAQ to the real Docker scope heading', () => {
    const link = guide('FAQ').querySelector(
      'a[href="/docs/DOCKER#runtime-and-container-alert-scopes"]',
    );
    expect(link?.hasAttribute('data-doc-link')).toBe(true);
    expect(link?.hasAttribute('target')).toBe(false);
    expect(guide('DOCKER').querySelector('#runtime-and-container-alert-scopes')).not.toBeNull();
  });

  it('keeps runtime connectivity, one-container Off and machine maintenance distinct', () => {
    const section = scopeSection();
    const rows = [...section.querySelectorAll('li')].map(prose);
    expect(rows).toHaveLength(3);
    expect(rows[0]).toContain('does not turn off alerts for its containers');
    expect(rows[0]).toContain('not a runtime-and-children control');
    expect(rows[1]).toContain('intended runtime/host group, not by name alone');
    expect(rows[1]).toContain('Other containers and the machine keep their own rules');
    expect(rows[1]).toContain('Offline Alerts leaves its other checks enabled');
    expect(rows[2]).toContain('also suppresses wanted machine alerts');
    expect(prose(section)).toContain('does not automatically cover containers added later');
    // This explanation must not require an unsafe command or a new diagnostics run.
    expect(section.querySelector('pre, form, input, button')).toBeNull();
    expect(prose(section)).toContain(
      'do not stop a workload, induce an alert or send another Test',
    );
    expect(prose(section)).toContain('not queue cancellation or proof of recovery');
  });

  it('keeps notification and Patrol follow-through in the shipped viewer', () => {
    const section = scopeSection();
    for (const [name, id] of [
      ['TROUBLESHOOTING', 'test-succeeds-but-real-alerts-are-missing'],
      ['FAQ', 'can-i-disable-alerts-for-specific-metrics'],
    ]) {
      const link = section.querySelector(`a[href="/docs/${name}#${id}"]`);
      expect(link, name).not.toBeNull();
      expect(link?.hasAttribute('data-doc-link'), name).toBe(true);
      expect(link?.hasAttribute('target'), name).toBe(false);
      expect(guide(name).querySelector(`[id="${id}"]`), name).not.toBeNull();
    }
  });
});
