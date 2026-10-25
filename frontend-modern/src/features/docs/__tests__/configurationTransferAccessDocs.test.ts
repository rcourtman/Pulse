import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const repoRoot = path.resolve(frontendRoot, '..');
const readDoc = (name: string) => readFileSync(path.join(repoRoot, 'docs', `${name}.md`), 'utf8');

function articleFor(name: string) {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(readDoc(name), name);
  return article;
}

function accessSection() {
  const article = articleFor('MIGRATION');
  const heading = article.querySelector('#confirm-organisation-and-access');
  expect(heading).not.toBeNull();
  const section = document.createElement('section');
  for (
    let node = heading!.nextElementSibling;
    node && node.tagName !== 'H2';
    node = node.nextElementSibling
  ) {
    section.append(node.cloneNode(true));
  }
  return section;
}

const textOf = (node: Element) => node.textContent!.replace(/\s+/g, ' ');

describe('Configuration transfer access guidance', () => {
  it('places target checks before transfer and links them from the first step', () => {
    const article = articleFor('MIGRATION');
    const headings = [...article.querySelectorAll('h2')].map((heading) => heading.id);
    expect(headings.indexOf('confirm-organisation-and-access')).toBeLessThan(
      headings.indexOf('transfer-the-included-configuration'),
    );
    const firstStep = article.querySelector('#transfer-the-included-configuration + ol > li');
    expect(firstStep?.querySelector('a')?.getAttribute('href')).toBe(
      '#confirm-organisation-and-access',
    );
    const text = textOf(accessSection());
    expect(text).toContain('source and destination installation');
    expect(text).toContain('Org Switcher');
    expect(text).toContain('its ID in Settings → Organization → Overview');
    expect(text).toContain('Keep that selection unchanged until the operation finishes');
    expect(text).toContain('not an archive of every organisation');
  });

  it('keeps session management authority and the two API scopes distinct', () => {
    const section = accessSection();
    const rows = [...section.querySelectorAll('tbody tr')].map((row) =>
      [...row.querySelectorAll('td')].map((cell) => cell.textContent),
    );
    expect(rows).toEqual([
      [
        'Signed-in browser session, default organisation',
        'Instance administrator',
        'Instance administrator',
      ],
      [
        'Signed-in browser session, selected non-default organisation',
        'Organisation owner or admin',
        'Organisation owner or admin',
      ],
      ['API token for the selected organisation', 'settings:read', 'settings:write'],
    ]);
    const text = textOf(section);
    expect(text).toContain('A viewer or editor session is not sufficient');
    expect(text).toContain('trusted-proxy identity must also have administrator authority');
    expect(text).toContain('API token takes precedence over a browser session');
    expect(text).toContain("does not enlarge that token's scopes or organisation binding");
  });

  it('separates decryption from authority and disclaims membership and token remapping', () => {
    const text = textOf(accessSection());
    expect(text).toContain('The archive passphrase is not a sign-in credential');
    expect(text).toContain('does not grant permission to export or replace settings');
    expect(text).toContain('provider and SSO secrets');
    expect(text).toContain('Import does not create organisations or transfer their members');
    expect(text).toContain(
      'import does not rebind them to a different destination organisation ID',
    );
    const article = articleFor('MIGRATION');
    expect(textOf(article)).toContain(
      'Organisation definitions, members, invitations and resource shares',
    );
  });

  it('explains denials without inviting bypasses or blind import retries', () => {
    const article = articleFor('MIGRATION');
    const failure = [...article.querySelectorAll('li')].find((item) =>
      textOf(item).startsWith('401 or 403 when exporting or importing:'),
    );
    expect(failure).toBeDefined();
    const text = textOf(failure!);
    expect(text).toContain('access failure, not evidence of a wrong archive passphrase');
    expect(text).toContain('Do not disable authentication, change organisation headers to bypass');
    expect(text).toContain('or repeat an import just to test permissions');
    expect(text).toContain('inspect the affected configuration before submitting it again');
  });

  it('renders the access checklist links from Recovery and API in the existing viewer', () => {
    for (const name of ['RECOVERY', 'API']) {
      const link = articleFor(name).querySelector(
        'a[href="/docs/MIGRATION#confirm-organisation-and-access"]',
      );
      expect(link, name).not.toBeNull();
      expect(link?.hasAttribute('data-doc-link')).toBe(true);
    }
    const section = accessSection();
    for (const [name, fragment] of [
      ['API', 'export-configuration'],
      ['MULTI_TENANT', 'organizations'],
    ]) {
      expect(section.querySelector(`a[href="/docs/${name}#${fragment}"]`)).not.toBeNull();
      expect(articleFor(name).querySelector(`#${fragment}`)).not.toBeNull();
    }
  });

  it('ships byte-identical migration, recovery and API documents', () => {
    for (const name of ['MIGRATION', 'RECOVERY', 'API']) {
      expect(readFileSync(path.join(frontendRoot, 'public/docs', `${name}.md`), 'utf8')).toBe(
        readDoc(name),
      );
    }
  });
});
