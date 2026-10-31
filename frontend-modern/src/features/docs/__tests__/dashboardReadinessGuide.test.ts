import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const read = (file: string) => readFileSync(path.resolve(process.cwd(), '..', file), 'utf8');
const anchor = 'health-is-green-but-the-dashboard-is-stalled';
const guide = (name: string) => read(`docs/${name}.md`);
const article = (markdown: string, name: string) => {
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(markdown, name);
  return element;
};
// Render the real section, not an invented diagnostic page or source-code spelling.
const section = (name: string, heading: string) => {
  const markdown = guide(name).split(`### ${heading}\n`)[1]?.split('\n### ')[0];
  expect(markdown, heading).toBeDefined();
  return article(`### ${heading}\n${markdown}`, name);
};
const text = (node: Element) => node.textContent!.replace(/\s+/g, ' ').trim();
const stalled = () => section('TROUBLESHOOTING', 'Health is green but the dashboard is stalled');

describe('dashboard readiness guidance', () => {
  it('offers a real symptom entry and working two-way API links', () => {
    const entry = guide('TROUBLESHOOTING').split('\n## ⚡ Quick Fixes')[0];
    const rendered = article(entry, 'TROUBLESHOOTING');
    const links = rendered.querySelectorAll(`a[href="#${anchor}"]`);
    expect(links).toHaveLength(1);
    expect(text(links[0].closest('li')!)).toMatch(/Connecting to Pulse.*stops updating/);
    expect(stalled().querySelector(`#${anchor}`)?.tagName).toBe('H3');
    expect(stalled().querySelector('a[href="/docs/API#system-health"]')).not.toBeNull();
    const health = section('API', 'System Health');
    expect(health.querySelector('#system-health')?.tagName).toBe('H3');
    expect(health.querySelector(`a[href="/docs/TROUBLESHOOTING#${anchor}"]`)).not.toBeNull();
  });

  it('separates public liveness and dependency presence from usable dashboard delivery', () => {
    const health = section('API', 'System Health');
    const copy = text(health);
    expect(copy).toMatch(/GET \/api\/health.*HEAD/);
    expect(copy).toMatch(/Public.*200.*503/);
    expect(copy).toMatch(/Liveness is not end-to-end readiness/);
    expect(copy).toMatch(
      /do not establish.*authentication.*polling.*dashboard state.*notification/,
    );
    expect(copy).toMatch(/websocket: true is not proof.*initial state/);
    expect(copy).toMatch(/upgrade or welcome message is not that proof/);
    expect(copy).toMatch(/container health badge.*inherits these limits/);
    const example = JSON.parse(health.querySelector('pre code')!.textContent!);
    expect(example.dependencies).toEqual({ monitor: true, scheduler: true, websocket: true });
    expect(example.status).toBe('healthy');
  });

  it('keeps connection, authentication, initial state and advancing readings distinct', () => {
    const rendered = stalled();
    const rows = [...rendered.querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(3);
    expect(text(rows[0])).toMatch(/page does not open.*another address, proxy or earlier time/);
    expect(text(rows[1])).toMatch(
      /Sign-in succeeds.*Authentication worked.*does not establish receipt of initial state/,
    );
    expect(text(rows[1])).toMatch(
      /does not identify a lock owner.*proxy, browser-cache or credential fault/,
    );
    expect(text(rows[2])).toMatch(/readings stop advancing.*collection or delivery continues/);
    expect(text(rows[2])).toMatch(/current value.*retained value.*stored History/);
    expect(rendered.querySelector('a[href="/docs/FAQ#no-data-showing"]')).not.toBeNull();
    expect(
      rendered.querySelector('a[href="/docs/INSTALL#installation-did-not-reach-a-usable-server"]'),
    ).not.toBeNull();
    expect(rendered.querySelector('a[href="#connection-refused"]')).not.toBeNull();
  });

  it('keeps failed proxy upgrades separate from accepted transport without state', () => {
    const proxy = section('REVERSE_PROXY', 'Other Issues');
    expect(proxy.querySelector(`a[href="/docs/TROUBLESHOOTING#${anchor}"]`)).not.toBeNull();
    expect(text(proxy)).toMatch(/already-observed upgrade failed.*Upgrade.*Connection/);
    expect(text(proxy)).toMatch(/HTTP 101 or welcome message without subsequent state is a different failure/);
    expect(text(proxy)).toMatch(/changing those headers is not an established remedy/);
    expect(text(proxy)).toMatch(/Keep authentication, TLS and origin checks enabled/);
    expect(text(proxy)).toMatch(/do not expose the backend to test a bypass/);
    expect(read('frontend-modern/public/docs/REVERSE_PROXY.md')).toBe(guide('REVERSE_PROXY'));
  });

  it('uses existing evidence without extra probes, reset advice or credential-bearing exports', () => {
    const rendered = stalled();
    const copy = text(rendered);
    expect(rendered.querySelector('pre')).toBeNull();
    expect(copy).toMatch(/original failure, not a new diagnostic run/);
    expect(copy).toMatch(/when already available/);
    expect(copy).toMatch(
      /cookies, headers, query values, WebSocket frames, full responses.*exports private/,
    );
    expect(copy).toMatch(/do not use Copy as cURL/);
    expect(copy).toMatch(/unavailable observation is unknown, not success/);
    expect(copy).toMatch(
      /Do not restart, reinstall, clear browser storage, sign out, bypass the proxy, disable TLS or run Diagnostics/,
    );
    expect(copy).toMatch(
      /Do not send repeated state requests, induce a freeze or take a process-terminating dump/,
    );
    expect(copy).toMatch(/Preserve existing configuration, credentials and logs/);
    expect(copy).toMatch(/guidance is not a runtime fix/);
    expect(rendered.querySelector('a[href="#-getting-help"]')).not.toBeNull();
    for (const name of ['API', 'TROUBLESHOOTING']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(guide(name));
    }
  });
});
