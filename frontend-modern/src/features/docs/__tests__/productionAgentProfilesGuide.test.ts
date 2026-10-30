import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const readDoc = (name: string) =>
  readFileSync(path.join(frontendRoot, '../docs', `${name}.md`), 'utf8');

function articleFor(name: string) {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(readDoc(name), name);
  return article;
}

function profileChoice(article: HTMLElement) {
  const heading = article.querySelector('#choose-a-linux-agent-profile');
  expect(heading?.textContent).toBe('Choose a Linux agent profile');
  const section = document.createElement('section');
  for (let node = heading!.nextElementSibling; node && node.tagName !== 'H2';) {
    section.append(node.cloneNode(true));
    node = node.nextElementSibling;
  }
  return section;
}

describe('Production guide agent-profile choice', () => {
  it('distinguishes the two choices in the actual rendered list', () => {
    const choice = profileChoice(articleFor('PRODUCTION_SECURITY'));
    const profiles = [...choice.querySelectorAll('li')];
    expect(profiles).toHaveLength(2);
    expect(profiles[0].querySelector('code')?.textContent).toBe(
      '--least-privilege --enable-privileged-helper',
    );
    expect(profiles[1].querySelector('code')?.textContent).toBe('--least-privilege');
    const typed = profiles[0].textContent!.replace(/\s+/g, ' ');
    expect(typed).toContain('no membership in the rootful Docker group');
    expect(typed).toContain('does not remove all root code');
    expect(typed).toContain('Do not combine it with --grant-smart or --grant-pct');
    const legacy = profiles[1].textContent!.replace(/\s+/g, ' ');
    expect(legacy).toContain('rootful Docker socket is root-equivalent');
    expect(legacy).toContain('NoNewPrivileges=false');
    expect(legacy).toContain('scoped sudo wrappers, not the typed helper');
  });

  it('keeps reduced telemetry, platform limits and migration separate from installation advice', () => {
    const choice = profileChoice(articleFor('PRODUCTION_SECURITY'));
    const text = choice.textContent!.replace(/\s+/g, ' ');
    expect(text).toContain('opt-in, not the default or full-telemetry parity');
    expect(text).toContain('not container stats, image-update checks or lifecycle actions');
    expect(text).toContain('do not substitute a rootful socket');
    expect(text).toContain('separately enabled action runner has its own credential');
    expect(text).toContain('not NAS appliances or non-systemd platforms');
    expect(text).toContain('Ordinary --update preserves the installed profile');
    expect(text).toContain('not a diagnostic step');
    // Profile flags are choices, not a second unsigned installer recipe.
    expect(choice.querySelector('pre')).toBeNull();
  });

  it('routes both the first comparison and rollout checklist to the profile decision', () => {
    const article = articleFor('PRODUCTION_SECURITY');
    const links = article.querySelectorAll('a[href="#choose-a-linux-agent-profile"]');
    expect(links).toHaveLength(2);
    expect(article.querySelector('#choose-a-linux-agent-profile')).not.toBeNull();
    expect(links[0].closest('table')).not.toBeNull();
    expect(links[1].closest('li')?.textContent).toContain(
      'Docker socket authority and reduced telemetry',
    );
  });

  it('links to real shipped profile and qualification headings, without losing backup safety', () => {
    const article = articleFor('PRODUCTION_SECURITY');
    const security = articleFor('AGENT_SECURITY');
    for (const anchor of [
      'least-privilege-agent-profile',
      'safe-profile-support-and-qualification-matrix',
    ]) {
      expect(article.querySelector(`a[href="/docs/AGENT_SECURITY#${anchor}"]`)).not.toBeNull();
      expect(security.querySelector(`#${anchor}`)).not.toBeNull();
    }
    const backup = article.querySelector('a[href="/docs/VM_DISK_MONITORING#backup-safety"]');
    expect(backup).not.toBeNull();
    expect(articleFor('VM_DISK_MONITORING').querySelector('#backup-safety')).not.toBeNull();
    expect(
      readFileSync(path.join(frontendRoot, 'public/docs/PRODUCTION_SECURITY.md'), 'utf8'),
    ).toBe(readDoc('PRODUCTION_SECURITY'));
  });
});
