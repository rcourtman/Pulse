import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { beforeAll, describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const source = readFileSync(path.join(repoRoot, 'docs/TROUBLESHOOTING.md'), 'utf8');
const shipped = readFileSync(
  path.join(repoRoot, 'frontend-modern/public/docs/TROUBLESHOOTING.md'),
  'utf8',
);
const guide = createGuideFixture(() => shipped);

// Parse and sanitise the complete shipped guide once during suite setup;
// each assertion still receives an isolated full-document clone.
beforeAll(() => {
  guide('TROUBLESHOOTING');
});

function testScopeSection(): { article: HTMLElement; section: HTMLElement } {
  const article = guide('TROUBLESHOOTING');
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
  it('routes a refused HTTP upgrade to direct HTTPS without replacing saved identity or trust', () => {
    const article = guide('TROUBLESHOOTING');
    const heading = article.querySelector('#proxmox-or-pbs-rejects-an-http-to-https-redirect');
    expect(heading?.tagName).toBe('H3');
    const section = document.createElement('section');
    for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
      if (/^H[1-3]$/.test(next.tagName)) break;
      section.appendChild(next.cloneNode(true));
    }
    const text = prose(section);
    expect(text).toContain('infrastructure redirects must stay on the same origin');
    expect(text).toContain('HTTP → HTTPS changes the origin');
    expect(text).toContain('the intended endpoint supports HTTPS at that same hostname and port');
    expect(text).toContain('change only the scheme to https://');
    expect(text).toContain(
      'Keep the saved credentials, Verify SSL certificate and SSL Fingerprint (optional) unchanged',
    );
    expect(text).toContain('Do not delete and re-add the connection, rotate tokens, clear pins');
    expect(text).toContain("do not copy a redirect's destination blindly");
    expect(text).toContain('check the next ordinary poll for the affected members');
    expect(text).toContain(
      "A successful Test alone does not establish every member's polling recovery",
    );
    expect(text).toContain('Keep literal endpoints, credentials and full configuration private');
    expect(section.querySelector('pre')).toBeNull();
    for (const id of [
      'proxmox-or-pbs-certificate-validation-failed',
      'test-connection-succeeds-but-cluster-polling-fails',
      '-getting-help',
    ]) {
      expect(section.querySelector(`a[href="#${id}"]`), id).not.toBeNull();
      expect(article.querySelector(`[id="${id}"]`), id).not.toBeNull();
    }
    const entry = article.querySelector(
      'a[href="#proxmox-or-pbs-rejects-an-http-to-https-redirect"]',
    );
    expect(entry?.textContent).toBe('direct HTTPS recovery');
  });

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
    expect(help?.textContent).toBe('Getting Help');
    expect(help?.hasAttribute('target')).toBe(false);
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
