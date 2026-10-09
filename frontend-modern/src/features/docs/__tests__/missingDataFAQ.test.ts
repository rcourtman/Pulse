import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');

function renderGuide(name: string): HTMLElement {
  const input = process.env[`PULSE_${name}_GUIDE_TEST_INPUT`];
  const source = readFileSync(
    input ?? path.join(repoRoot, 'frontend-modern/public/docs', `${name}.md`),
    'utf8',
  );
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(source, name);
  return article;
}

function section(id: string): HTMLElement {
  const heading = renderGuide('FAQ').querySelector(`#${id}`);
  expect(heading).not.toBeNull();
  const result = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-6]$/.test(next.tagName)) break;
    result.appendChild(next.cloneNode(true));
  }
  return result;
}

const prose = (element: HTMLElement): string =>
  (element.textContent ?? '').replace(/\s+/g, ' ').trim();

describe('shipped missing-data FAQ', () => {
  it('separates missing navigation, current readings and stored History', () => {
    const rows = section('no-data-showing').querySelectorAll('tbody tr');
    expect(rows).toHaveLength(3);
    expect(prose(rows[0] as HTMLElement)).toContain('Whole page or resource');
    expect(prose(rows[0] as HTMLElement)).toContain('platform page, not just Machines');
    expect(prose(rows[1] as HTMLElement)).toContain('during ordinary polling');
    expect(prose(rows[1] as HTMLElement)).toContain('particular missing reading');
    expect(prose(rows[2] as HTMLElement)).toContain('selected resource and time range');
    expect(prose(rows[2] as HTMLElement)).toContain('samples were stored for that chart');
  });

  it('does not mistake connection liveness or a successful test for fresh metrics', () => {
    const text = prose(section('no-data-showing'));
    expect(text).toContain('Last seen or successful Test Connection');
    expect(text).toContain('does not prove that every reading is fresh');
    expect(text).toContain('Missing or unavailable is not zero');
    expect(text).toContain('access error, not on a missing chart alone');
  });

  it('routes each symptom to a real heading in a shipped guide', () => {
    const sections = [section('no-data-showing'), section('connection-refused')];
    const links = sections.flatMap((element) => [...element.querySelectorAll('a[href]')]);
    expect(links).toHaveLength(10);
    expect(
      sections[0].querySelector(
        'a[href="/docs/TROUBLESHOOTING#replication-jobs-are-pending-stale-or-missing"]',
      ),
    ).not.toBeNull();
    for (const link of links) {
      const href = link.getAttribute('href')!;
      const match = /^\/docs\/([^#]+)(#.+)$/.exec(href);
      expect(match, href).not.toBeNull();
      expect(renderGuide(match![1]).querySelector(match![2]), href).not.toBeNull();
      expect(link.hasAttribute('data-doc-link'), href).toBe(true);
      expect(link.hasAttribute('target'), href).toBe(false);
    }
  });

  it('preserves identity and backup safety while keeping collection bounded and private', () => {
    const element = section('no-data-showing');
    const text = prose(element);
    expect(text).toContain('Do not replace credentials, delete connections or re-enrol agents');
    expect(text).toContain('backup, freeze/thaw or an unresponsive-host incident');
    expect(text).toContain('defer setup, live tests and Run Diagnostics');
    expect(text).toContain('bounded excerpt from the original incident');
    expect(text).toContain('review it locally before sharing');
    expect(element.querySelector('pre')).toBeNull();
    expect(text).not.toMatch(/journalctl|docker logs|port 8006/);
  });

  it('keeps a refused platform request separate from a refused Pulse browser connection', () => {
    const element = section('connection-refused');
    const text = prose(element);
    expect(text).toContain("browser's connection to Pulse from Pulse's connection");
    expect(text).toContain('actual service, listening port and proxy');
    expect(text).toContain(
      "changing Pulse's listening port does not repair the platform connection",
    );
    expect(element.querySelector('pre')).toBeNull();
  });

  it('makes the connection guide diagnose the affected path without weakening access or TLS', () => {
    const heading = renderGuide('TROUBLESHOOTING').querySelector('#connection-refused');
    expect(heading).not.toBeNull();
    const element = document.createElement('section');
    for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
      if (/^H[1-6]$/.test(next.tagName)) break;
      element.appendChild(next.cloneNode(true));
    }
    const text = prose(element);
    expect(text).toContain('Your browser cannot open Pulse');
    expect(text).toContain('Pulse opens, but a monitored platform request is refused');
    expect(text).toContain('network path from the Pulse server');
    expect(text).toContain('not an assumed Proxmox port');
    expect(text).toContain('not broadly open firewall access or expose Pulse directly');
    expect(text).toContain('not an authentication response (401/403)');
    expect(text).toContain('do not replace credentials or disable TLS verification');
    expect(element.querySelector('a[href="/docs/FAQ#no-data-showing"]')).not.toBeNull();
    expect(element.querySelector('pre')).toBeNull();
  });
});
