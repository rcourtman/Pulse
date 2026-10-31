import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const read = (file: string) => readFileSync(path.resolve(process.cwd(), '..', file), 'utf8');
const article = (name: string) => {
  const result = document.createElement('article');
  result.innerHTML = renderDocMarkdown(read(`docs/${name}.md`), name);
  return result;
};
const anchor = 'installation-did-not-reach-a-usable-server';
const section = () => {
  const heading = article('INSTALL').querySelector(`#${anchor}`);
  expect(heading?.tagName).toBe('H2');
  const result = document.createElement('section');
  for (
    let node = heading!.nextElementSibling;
    node && node.tagName !== 'H2';
    node = node.nextElementSibling
  ) {
    result.appendChild(node.cloneNode(true));
  }
  return result;
};
const prose = (element: HTMLElement) => element.textContent!.replace(/\s+/g, ' ').trim();

describe('installation startup guidance', () => {
  it('reaches the startup distinction before setup, from both existing entry paths', () => {
    const install = article('INSTALL');
    const heading = install.querySelector(`#${anchor}`)!;
    const entry = install.querySelector(`a[href="#${anchor}"]`)!;
    expect(entry).not.toBeNull();
    expect(entry.compareDocumentPosition(heading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    // Inspect heading order rather than relying on the emoji slug spelling.
    expect([...install.querySelectorAll('h2')].findIndex((node) => node === heading)).toBeLessThan(
      [...install.querySelectorAll('h2')].findIndex((node) =>
        node.textContent!.includes('First-Time Setup'),
      ),
    );
    expect(
      article('TROUBLESHOOTING').querySelector(`a[href="/docs/INSTALL#${anchor}"]`),
    ).not.toBeNull();
  });

  it('offers only a bounded property read, in the affected server rather than the hypervisor', () => {
    const rendered = section();
    const code = rendered.querySelectorAll('pre code');
    expect(code).toHaveLength(1);
    // Keep this read fully visible at 320px, including in WebKit where a
    // non-focusable code block cannot be scrolled with an external keyboard.
    for (const line of code[0].textContent!.trim().split('\n')) {
      expect(line.length).toBeLessThanOrEqual(32);
    }
    expect(
      code[0]
        .textContent!.replace(/\\\n\s*/g, ' ')
        .trim()
        .split(/\s+/),
    ).toEqual([
      'timeout',
      '-k',
      '1',
      '5',
      'systemctl',
      'show',
      'pulse.service',
      '--no-pager',
      '--property=LoadState',
      '--property=ActiveState',
      '--property=SubState',
      '--property=Result',
    ]);
    expect(prose(rendered)).toContain('inside the Pulse LXC, not on its Proxmox host');
    expect(prose(rendered)).toContain('do not substitute pulse-agent.service');
    expect(prose(rendered)).toContain('not a container-health check');
    expect(prose(rendered)).toContain('not environment values or log content');
  });

  it('does not confuse active, unavailable and ready or prescribe a second disruptive attempt', () => {
    const copy = prose(section());
    for (const distinction of [
      'A running LXC does not establish that Pulse is running inside it',
      'do not boot it merely to run the check',
      'a wrong unit name or incomplete service installation',
      'an unavailable read, not evidence of an inactive or healthy service',
      'does not establish application readiness or explain an earlier stop',
      'successful sign-in and current monitoring are separate observations',
      'Do not remove authentication or change firewall/TLS settings',
      'Do not recreate containers, remove volumes or change images',
      'Keep configuration, encryption keys, persistent data and rollback copies intact',
      'unknowns are valid',
      'another install attempt is not required',
    ])
      expect(copy).toContain(distinction);
  });

  it('links to real recovery and private log guidance and keeps shipped guides identical', () => {
    for (const [name, fragment] of [
      ['TROUBLESHOOTING', 'inspect-notification-logs'],
      ['TROUBLESHOOTING', 'i-forgot-my-password'],
      ['RECOVERY', ''],
    ]) {
      const href = `/docs/${name}${fragment ? `#${fragment}` : ''}`;
      expect(section().querySelector(`a[href="${href}"]`)).not.toBeNull();
      if (fragment) expect(article(name).querySelector(`#${fragment}`)).not.toBeNull();
    }
    for (const name of ['INSTALL', 'TROUBLESHOOTING']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
  });
});
