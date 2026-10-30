import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const readDoc = (name: string) =>
  readFileSync(path.join(frontendRoot, '../docs', `${name}.md`), 'utf8');
const source = readDoc('UNIFIED_AGENT');

function observerSection() {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(source, 'UNIFIED_AGENT');
  const heading = article.querySelector('#observer-destinations');
  expect(heading).not.toBeNull();
  const section = document.createElement('section');
  for (let node = heading!.nextElementSibling; node && node.id !== 'advanced-flags';) {
    section.append(node.cloneNode(true));
    node = node.nextElementSibling;
  }
  return section;
}

describe('Agent observer trust and API-access choice', () => {
  it('renders a copyable explicit telemetry-only recipe with separate credentials and trust', () => {
    const section = observerSection();
    const fences = section.querySelectorAll('pre code');
    expect(fences).toHaveLength(1);
    const config = JSON.parse(fences[0].textContent!);
    expect(config.version).toBe(1);
    expect(config.observers).toHaveLength(1);
    expect(config.observers[0]).toMatchObject({
      url: 'https://pulse-dev.example.test',
      tokenFile: '/etc/pulse-agent/dev-observer.token',
      provisionProxmox: false,
    });
    expect(config.observers[0].serverFingerprint).toMatch(/^[0-9a-f]{64}$/);
    expect(config.observers[0].token).toBeUndefined();
    expect(config.observers[0].insecureSkipVerify).toBeUndefined();
    expect(config.observers[0].allowPlaintextHTTP).toBeUndefined();
    const text = section.textContent!.replace(/\s+/g, ' ');
    expect(text).toContain('placeholder, not a trusted certificate');
    expect(text).toContain('Report-only is a control boundary, not a redaction boundary');
  });

  it('keeps the omitted-field default and irreversible-access limits beside the choice', () => {
    const section = observerSection();
    const rows = [...section.querySelectorAll('li')];
    expect(rows).toHaveLength(2);
    expect(rows[0].textContent).toContain('"provisionProxmox": false');
    expect(rows[1].textContent).toContain('true, or the field omitted');
    const text = section.textContent!.replace(/\s+/g, ' ');
    expect(text).toContain('Omitting the field defaults to true, not telemetry only');
    expect(text).toContain('register its credentials with that Pulse server');
    expect(text).toContain('not revocation of a previously issued Proxmox token');
    expect(text).toContain('not a reduced or anonymised copy');
    expect(text).toContain('primary server\'s HTTP consent does not authorise an observer');
  });

  it('retains real setup, safe-reporting and backup links through the consuming renderer', () => {
    const section = observerSection();
    const targets = [
      ['UNIFIED_AGENT', 'private-file-installation-linux-macos-and-nas'],
      ['UNIFIED_AGENT', 'agent-lookup-returns-http-401-or-403'],
      ['VM_DISK_MONITORING', 'backup-safety'],
    ];
    for (const [doc, anchor] of targets) {
      const href = doc === 'UNIFIED_AGENT' ? `#${anchor}` : `/docs/${doc}#${anchor}`;
      expect(section.querySelector(`a[href="${href}"]`)).not.toBeNull();
      const article = document.createElement('article');
      article.innerHTML = renderDocMarkdown(readDoc(doc), doc);
      expect(article.querySelector(`#${anchor}`)).not.toBeNull();
    }
    expect(readFileSync(path.join(frontendRoot, 'public/docs/UNIFIED_AGENT.md'), 'utf8')).toBe(
      source,
    );
  });
});
