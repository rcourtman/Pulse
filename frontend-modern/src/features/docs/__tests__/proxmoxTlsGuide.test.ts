import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown, stripLeadingTitle } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const source = readFileSync(path.join(repoRoot, 'docs/TROUBLESHOOTING.md'), 'utf8');
const shipped = readFileSync(
  path.join(repoRoot, 'frontend-modern/public/docs/TROUBLESHOOTING.md'),
  'utf8',
);

function testScopeSection(): { article: HTMLElement; section: HTMLElement } {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(stripLeadingTitle(shipped), 'TROUBLESHOOTING');
  const heading = article.querySelector('#test-connection-succeeds-but-cluster-polling-fails');
  expect(heading?.tagName).toBe('H4');
  const section = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-4]$/.test(next.tagName)) break;
    section.appendChild(next.cloneNode(true));
  }
  return { article, section };
}

const prose = (element: HTMLElement): string =>
  (element.textContent ?? '').replace(/\s+/g, ' ').trim();

describe('Proxmox TLS troubleshooting guide', () => {
  it('distinguishes the primary connection test from saved-member polling and agent reports', () => {
    const { section } = testScopeSection();
    const text = prose(section);
    expect(text).toContain(
      'Test Connection checks the primary endpoint with the selected TLS settings',
    );
    expect(text).toContain('not the saved per-member trust used by background polling');
    expect(text).toContain(
      'Changing or clearing the primary SSL Fingerprint does not by itself replace saved member pins, even for that same endpoint',
    );
    expect(text).toContain(
      'does not establish that ordinary polling has recovered, or that every member now uses CA validation',
    );
    expect(text).toContain(
      "an API connection's TLS result also does not establish whether a Pulse agent is reporting successfully",
    );
  });

  it('offers a safe help route instead of repeated trust edits or recovery experiments', () => {
    const { article, section } = testScopeSection();
    const text = prose(section);
    expect(text).toContain("the failing members' existing errors as separate evidence");
    expect(text).toContain('independently verified primary-pin correction');
    expect(text).toContain('do not keep clearing pins, copy that pin to peers or restart Pulse');
    expect(text).toContain('Preserve verification and saved trust');
    expect(text).toContain('leave it intact and use Getting Help');
    expect(text).toContain(
      'keep literal endpoints, fingerprints, credentials and full configuration private',
    );
    expect(text).toContain('Do not edit encrypted files, run Diagnostics, force rediscovery');
    expect(section.querySelector('pre, code')).toBeNull();
    const help = section.querySelector('a[href="#-getting-help"]');
    expect(help?.hasAttribute('data-doc-link')).toBe(true);
    expect(article.querySelector('#-getting-help')?.tagName).toBe('H2');
    expect(
      section.querySelector('a[href="/docs/VM_DISK_MONITORING#backup-safety"]'),
    ).not.toBeNull();
  });

  it('ships the same explanation without displacing the existing trust and renewal guidance', () => {
    expect(shipped).toBe(source);
    const { article } = testScopeSection();
    for (const id of [
      'proxmox-or-pbs-certificate-validation-failed',
      'certificate-renewal-and-saved-fingerprints',
      'cluster-members-and-conflicting-saved-trust',
    ]) {
      expect(article.querySelector(`[id="${id}"]`), id).not.toBeNull();
    }
    expect(prose(article)).toContain('Keep certificate verification enabled.');
    expect(prose(article)).toContain("do not copy the primary's fingerprint to all members");
  });
});
