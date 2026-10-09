import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const servedDocs = path.join(repoRoot, 'frontend-modern/public/docs');

function temperatureFAQ(): HTMLElement {
  const source = readFileSync(
    process.env.PULSE_TEMPERATURE_FAQ_TEST_INPUT ?? path.join(servedDocs, 'FAQ.md'),
    'utf8',
  );
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(source, 'FAQ');
  const heading = article.querySelector('#how-do-i-monitor-temperature');
  expect(heading).not.toBeNull();
  const section = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-6]$/.test(next.tagName)) break;
    section.appendChild(next.cloneNode(true));
  }
  return section;
}

const prose = (element: HTMLElement): string =>
  (element.textContent ?? '').replace(/\s+/g, ' ').trim();

describe('shipped temperature FAQ', () => {
  it('checks the actual host, agent, sensor and observation rather than inventing a reading', () => {
    const text = prose(temperatureFAQ());
    expect(text).toContain("affected host's active agent version, sensor and observation time");
    expect(text).toContain('A current Pulse server does not update every agent');
    expect(text).toContain('same host and sensor at the same time');
    expect(text).toContain('CPU/SoC temperature is not physical-disk SMART temperature');
    expect(text).toContain('A missing reading is unavailable, not zero');
  });

  it('uses existing Linux providers without making a package install or hardware scan mandatory', () => {
    const section = temperatureFAQ();
    const text = prose(section);
    expect(text).toContain('existing sensors -j output and recognised CPU/SoC thermal sysfs');
    expect(text).toContain('lm-sensors is not required for the CPU/SoC fallback');
    expect(text).toContain('monitored host, not inside the Pulse container');
    expect(section.querySelector('pre, ol')).toBeNull();
    expect(text).not.toMatch(/apt install|sensors-detect|--auto/);
  });

  it('defers disruptive setup and incident-time diagnostics instead of prescribing a retry', () => {
    const text = prose(temperatureFAQ());
    expect(text).toContain('Do not run hardware detection, bus scans, load drivers or reboot');
    expect(text).toContain("sensor setup belongs in that host's normal maintenance window");
    expect(text).toContain('During backups, freeze/thaw or an unresponsive-host incident');
    expect(text).toContain('defer setup and diagnostics');
  });

  it('routes passive checks and new installation to real rendered headings in the served guide', () => {
    const links = [...temperatureFAQ().querySelectorAll('a[href]')];
    expect(links.map((link) => link.getAttribute('href'))).toEqual([
      '/docs/TEMPERATURE_MONITORING#check-existing-linux-readings-safely',
      '/docs/TEMPERATURE_MONITORING#recommended-pulse-agent-proxmox',
      '/docs/TEMPERATURE_MONITORING',
    ]);
    const guide = document.createElement('article');
    guide.innerHTML = renderDocMarkdown(
      readFileSync(path.join(servedDocs, 'TEMPERATURE_MONITORING.md'), 'utf8'),
      'TEMPERATURE_MONITORING',
    );
    for (const link of links) {
      const fragment = link.getAttribute('href')!.split('#')[1];
      if (fragment) expect(guide.querySelector(`#${fragment}`)).not.toBeNull();
      expect(link.hasAttribute('data-doc-link')).toBe(true);
      expect(link.hasAttribute('target')).toBe(false);
    }
    const text = prose(guide);
    expect(text).toContain('protect the Pulse agent token');
    expect(text).toContain('over verified HTTPS');
    expect(text).toContain('--token-file');
    expect(text).toContain('limits the command to five seconds');
    expect(text).toContain('saves both output streams privately');
  });

  it('keeps an agent reading from becoming a reason to widen SSH access', () => {
    const text = prose(temperatureFAQ());
    expect(text).toContain('Recent usable agent temperature data does not also require SSH');
    expect(text).toContain('do not mount root SSH keys into Pulse or loosen an existing');
    expect(text).toContain('restricted key to repair a missing reading');
    expect(text).toContain('Other platforms have different providers and limits');
  });

  it('ships the same FAQ and destination guide as their canonical copies', () => {
    for (const name of ['FAQ', 'TEMPERATURE_MONITORING']) {
      expect(readFileSync(path.join(servedDocs, `${name}.md`), 'utf8')).toBe(
        readFileSync(path.join(repoRoot, 'docs', `${name}.md`), 'utf8'),
      );
    }
  });
});
