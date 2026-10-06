import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');

function renderGuide(name: string): HTMLElement {
  // An exact-parent input proves that the former advice fails these checks
  // without altering the committed source used by the proof.
  const input = process.env[`PULSE_${name}_GUIDE_TEST_INPUT`];
  const source = readFileSync(
    input ?? path.join(repoRoot, 'frontend-modern/public/docs', `${name}.md`),
    'utf8',
  );
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(source, name);
  return article;
}

function unavailableSection(article: HTMLElement): HTMLElement {
  const heading = article.querySelector('#truenas-service-unavailable');
  expect(heading).not.toBeNull();
  const section = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-4]$/.test(next.tagName)) break;
    section.appendChild(next.cloneNode(true));
  }
  return section;
}

const prose = (section: HTMLElement): string =>
  (section.textContent ?? '').replace(/\s+/g, ' ').trim();

describe('shipped TrueNAS error diagnosis', () => {
  it.each(['TRUENAS', 'TROUBLESHOOTING'])(
    '%s routes local service errors away from appliance credentials and live tests',
    (name) => {
      const section = unavailableSection(renderGuide(name));
      const text = prose(section);
      expect(text).toContain('truenas_unavailable');
      expect(text).toContain('500');
      expect(text).toContain('503');
      expect(text).toContain('connection-management handler or configuration persistence');
      expect(text).toContain('bounded Pulse');
      expect(text).toMatch(/Do not rotate the (?:TrueNAS )?key, recreate/);
      expect(text).toContain('weaken TLS verification');
      expect(section.querySelector('pre')).toBeNull();
      expect(text).not.toContain('then use Test Connection');
      expect(text).not.toContain('Use Test Connection in Pulse.');
    },
  );

  it('keeps explicit opt-out and an actual failed probe distinct from unavailable storage', () => {
    const text = prose(unavailableSection(renderGuide('TRUENAS')));
    expect(text).toContain('truenas_disabled / HTTP 404');
    expect(text).toContain('PULSE_ENABLE_TRUENAS');
    expect(text).toContain('not an appliance outage');
    expect(text).toContain('override an intentional opt-out');
    expect(text).toContain('truenas_connection_failed / HTTP 400');
    expect(text).toContain('a live connection test failed');
    expect(text).toContain('Keep HTTPS and certificate verification enabled');
    expect(text).toContain('recognized CORE 13 systems use legacy REST');
  });

  it('uses existing browser evidence and does not require replay or public credentials', () => {
    const text = prose(unavailableSection(renderGuide('TRUENAS')));
    expect(text).toContain('existing failed request');
    expect(text).toContain('Developer tools → Network');
    expect(text).toContain('do not repeat a save, delete or test');
    expect(text).toContain('request path, status/code and a manually redacted error');
    expect(text).toContain('not the full response, configuration, key or session cookie');
    expect(text).toContain('on a separate connection, not inventory or metric collection');
    expect(text).toContain('instead of changing credentials');
  });

  it('takes both entry points to real error and polling headings in the shipped viewer', () => {
    const guide = renderGuide('TRUENAS');
    const troubleshooting = unavailableSection(renderGuide('TROUBLESHOOTING'));
    const errorLink = troubleshooting.querySelector<HTMLAnchorElement>(
      'a[href="/docs/TRUENAS#truenas-service-unavailable"]',
    );
    expect(errorLink?.textContent).toBe('TrueNAS error diagnosis');
    expect(guide.querySelector(new URL(errorLink!.href).hash)?.textContent).toBe(
      '"TrueNAS service unavailable"',
    );
    expect(
      troubleshooting.querySelector('a[href="/docs/TRUENAS#stale-truenas-data"]'),
    ).not.toBeNull();
    const pollingLink = unavailableSection(guide).querySelector('a[href="#stale-truenas-data"]');
    expect(pollingLink?.textContent).toBe('polling checks');
    expect(guide.querySelector('#stale-truenas-data')?.textContent).toBe('Stale TrueNAS data');
  });
});
