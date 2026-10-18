import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');

function renderGuide(name: string): HTMLElement {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(
    readFileSync(path.join(repoRoot, 'frontend-modern/public/docs', `${name}.md`), 'utf8'),
    name,
  );
  return article;
}

function section(article: HTMLElement, id: string): HTMLElement {
  const heading = article.querySelector(`#${id}`);
  expect(heading, `missing permission guide section ${id}`).not.toBeNull();
  return followingContent(heading!);
}

function followingContent(heading: Element): HTMLElement {
  const content = document.createElement('div');
  for (let element = heading.nextElementSibling; element; element = element.nextElementSibling) {
    if (/^H[1-3]$/.test(element.tagName)) break;
    content.appendChild(element.cloneNode(true));
  }
  return content;
}

const prose = (element: HTMLElement): string =>
  (element.textContent ?? '').replace(/\s+/g, ' ').trim();

describe('shipped Proxmox permission diagnosis', () => {
  it('checks the configured user and token on the affected resource, not an administrator', () => {
    const content = prose(section(renderGuide('TROUBLESHOOTING'), 'check-permissions-proxmox'));
    expect(content).toContain(
      'user, realm and token ID configured for the affected Pulse connection',
    );
    expect(content).toContain('intersection of the user and token permissions');
    expect(content).toContain(
      'both must allow the required privilege on the affected resource path',
    );
    expect(content).toContain('inherited ACLs and their propagation');
    expect(content).toContain('administrator session or user-only permission listing');
    expect(content).toContain('does not prove the token has access');
  });

  it('removes the literal placeholder command that redirects and overwrites local files', () => {
    const content = section(renderGuide('TROUBLESHOOTING'), 'check-permissions-proxmox');
    // The old <user>@pam snippet passes bash -n, but redirects from `user`
    // into `@pam` instead of passing a user ID. Inspection needs no command.
    expect(content.querySelector('pre')).toBeNull();
    expect(prose(content)).not.toContain('pveum user permissions');
    expect(prose(content)).toContain('Inspect the existing denial');
  });

  it('preserves least privilege and avoids rotating credentials or probing a frozen guest', () => {
    const content = prose(section(renderGuide('TROUBLESHOOTING'), 'check-permissions-proxmox'));
    expect(content).toContain('Do not disable privilege separation');
    expect(content).toContain('grant Administrator');
    expect(content).toContain('add guest execution/write privileges');
    expect(content).toContain('Do not rerun setup or replace a token as a permissions test');
    expect(content).toContain('normal maintenance window, outside backups');
    expect(content).toContain('normal polling without manual guest-agent probes');
    expect(content).toContain('Keep token secrets and full permission listings private');
    expect(content).toContain('missing privilege and a redacted denial');
    expect(content).toContain('Sys.Modify is not a read-only monitoring privilege');
  });

  it('distinguishes active guest reads from passive inventory before recommending permissions', () => {
    const content = section(renderGuide('AGENT_SECURITY'), 'proxmox-deployment-choices');
    expect(prose(content)).toContain('API-only does not mean guest-agent-free');
    expect(prose(content)).toContain(
      'active requests on the channel used by freeze-enabled backups',
    );
    expect(prose(content)).toContain(
      'Read permissions and the absence of a host agent do not prove backup safety',
    );
    const warning = Array.from(content.querySelectorAll('p')).find((paragraph) =>
      paragraph.textContent?.includes('API-only does not mean guest-agent-free'),
    );
    const table = content.querySelector('table');
    expect(table).not.toBeNull();
    expect(warning?.compareDocumentPosition(table!)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(warning?.querySelector('a')?.getAttribute('href')).toBe(
      '/docs/VM_DISK_MONITORING#backup-safety',
    );
  });

  it('keeps the disk-specific entry point token-aware without promising a fresh reading', () => {
    const heading = Array.from(renderGuide('VM_DISK_MONITORING').querySelectorAll('h2')).find(
      (element) => element.textContent === '⚙️ Permissions',
    );
    expect(heading).toBeDefined();
    const content = followingContent(heading!);
    const text = prose(content);
    expect(text).toContain('intersection of user and token permissions');
    expect(text).toContain('on the affected VM');
    expect(text).toContain('Do not disable privilege separation or recreate the token');
    expect(text).toContain('does not prove disk freshness, responsiveness or thaw');
    expect(content.querySelector('a')?.getAttribute('href')).toBe(
      '/docs/TROUBLESHOOTING#check-permissions-proxmox',
    );
  });

  it('matches the existing generated token and ACL contract without running setup', () => {
    // Static binding only: neither this test nor the docs create a real token.
    const setup = readFileSync(
      path.join(repoRoot, 'internal/api/configapi/setup_script_render.go'),
      'utf8',
    );
    expect(setup).toContain('--privsep 1');
    expect(setup).toContain('pveum aclmod / -user pulse-monitor@pve -role PVEAuditor');
    expect(setup).toContain('pveum aclmod / -token "$PULSE_TOKEN_ID" -role PVEAuditor');
    for (const name of ['TROUBLESHOOTING', 'VM_DISK_MONITORING', 'AGENT_SECURITY']) {
      expect(readFileSync(path.join(repoRoot, 'docs', `${name}.md`))).toEqual(
        readFileSync(path.join(repoRoot, 'frontend-modern/public/docs', `${name}.md`)),
      );
    }
  });
});
