import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const shipped = (name: string) =>
  readFileSync(path.join(root, 'frontend-modern/public/docs', `${name}.md`), 'utf8');
const render = createGuideFixture(shipped);
const fragment = 'pbs-is-active-but-missing-from-monitoring';

function section(): HTMLElement {
  const heading = render('PBS').querySelector(`#${fragment}`);
  expect(heading).not.toBeNull();
  const element = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-3]$/.test(next.tagName)) break;
    element.appendChild(next.cloneNode(true));
  }
  return element;
}

const prose = (element: HTMLElement) =>
  (element.textContent ?? '').replace(/\s+/g, ' ').trim();

describe('missing API-only PBS help', () => {
  it('does not mistake connection polling health or an agent view for API recovery', () => {
    const text = prose(section());
    expect(text).toContain("connection's polling health");
    expect(text).toContain('does not establish that Pulse has a matching PBS resource');
    expect(text).toContain('A direct PBS API connection does not require a Pulse Agent');
    expect(text).toContain('adding one does not prove that the API path recovered');
  });

  it('separates the missing server, partial inventory and empty History without probes', () => {
    const element = section();
    const rows = element.querySelectorAll('tbody tr');
    expect(rows).toHaveLength(3);
    expect(prose(rows[0] as HTMLElement)).toContain('The PBS VM/LXC in Overview is a different resource');
    expect(prose(rows[0] as HTMLElement)).toContain('If Proxmox navigation is absent too');
    expect(prose(rows[1] as HTMLElement)).toContain('PVE passthrough backups do not establish');
    expect(prose(rows[2] as HTMLElement)).toContain('not the same as a missing server');
    expect(element.querySelector('pre')).toBeNull();
  });

  it('accepts existing evidence and unknown versions without transferring diagnosis', () => {
    const text = prose(section());
    expect(text).toContain('keep the Pulse server version separate from the PBS version');
    expect(text).toContain('unknown is valid');
    expect(text).toContain('Use consistent aliases for private server names');
    expect(text).toContain('Existing error text or screenshots are enough to start');
    expect(text).toContain('do not run Diagnostics, enable Debug or collect a full response');
    expect(text).toContain('Do not delete and re-add the connection, replace tokens, install an agent');
    expect(text).toContain('Keep certificate verification and existing access restrictions intact');
  });

  it('links the existing troubleshooting entry and safety/history guides to real headings', () => {
    const entry = render('TROUBLESHOOTING').querySelector(`a[href="/docs/PBS#${fragment}"]`);
    expect(entry).not.toBeNull();
    expect(prose(entry!.closest('li')!)).toContain('installing an agent does not prove API recovery');
    for (const link of section().querySelectorAll('a[href]')) {
      const href = link.getAttribute('href')!;
      const target = /^#(.+)$/.exec(href);
      const cross = /^\/docs\/([^#]+)#(.+)$/.exec(href);
      expect(target || cross, href).not.toBeNull();
      const article = render(cross ? cross[1] : 'PBS');
      expect(article.querySelector(`[id="${cross ? cross[2] : target![1]}"]`), href).not.toBeNull();
    }
    for (const name of ['PBS', 'TROUBLESHOOTING']) {
      expect(shipped(name)).toBe(readFileSync(path.join(root, 'docs', `${name}.md`), 'utf8'));
    }
  });
});
