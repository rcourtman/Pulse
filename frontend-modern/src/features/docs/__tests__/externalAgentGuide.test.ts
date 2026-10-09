import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const read = (file: string) => readFileSync(resolve(process.cwd(), '..', file), 'utf8');
const article = (name: string) => {
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(read(`docs/${name}.md`), name);
  return element;
};
const text = (element: Element) => element.textContent?.replace(/\s+/g, ' ').trim() ?? '';

describe('external-agent operator guidance', () => {
  it('puts read-only trust and private credentials before advertised controls', () => {
    expect(read('frontend-modern/public/docs/AGENT_SUBSTRATE.md')).toBe(
      read('docs/AGENT_SUBSTRATE.md'),
    );
    const guide = article('AGENT_SUBSTRATE');
    const warning = [...guide.querySelectorAll('strong')].find((node) =>
      text(node).includes('Start read-only'),
    );
    expect(warning).toBeDefined();
    const setup = guide.querySelector('#connect-without-granting-control');
    expect(setup?.nextElementSibling?.tagName).toBe('OL');
    expect(setup?.nextElementSibling?.querySelectorAll('li')).toHaveLength(5);
    expect(text(setup!.nextElementSibling!)).toContain('not the minimum for a read-only client');
    expect(text(setup!.nextElementSibling!)).toContain('project-shared .mcp.json');
    expect(text(guide)).toContain('only the protected fleet read checks token access');
    expect([...guide.querySelectorAll('pre')].map((node) => text(node))).toEqual([
      'pulse_api GET /api/agent/capabilities pulse_api GET /api/agent/fleet-context',
    ]);
  });

  it('shows terminal-event, partial-verification and legacy-probe limits', () => {
    const guide = article('AGENT_SUBSTRATE');
    const table = guide.querySelector('table');
    expect(table?.querySelectorAll('tbody tr')).toHaveLength(4);
    expect(text(table!)).toContain('unknown');
    expect(text(table!)).toContain('ran: false');
    expect(text(table!)).toContain('not every application or filesystem health requirement');
    expect(text(guide)).toContain('including failed or refused actions');
    expect(text(guide)).toContain('a change may already have happened');
    expect(text(guide)).toContain('Do not resend execution, invent another request ID');
    expect(text(guide)).toContain('Do not use it with a real token or production data');
    expect(text(guide)).not.toContain('so an agent can confirm an outcome without polling');
  });

  it('navigates all API boundaries and the local outcome heading', () => {
    const guide = article('AGENT_SUBSTRATE');
    const api = article('API');
    for (const fragment of [
      '-authentication',
      'resource-maintenance-and-operator-state',
      'unified-action-planning',
    ]) {
      const link = guide.querySelector(`a[href="/docs/API#${fragment}"]`);
      expect(link?.hasAttribute('data-doc-link')).toBe(true);
      expect(api.querySelector(`[id="${fragment}"]`)).not.toBeNull();
    }
    expect(guide.querySelector('#read-action-outcomes-without-repeating-them')).not.toBeNull();
  });
});
