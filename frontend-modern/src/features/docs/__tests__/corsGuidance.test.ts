import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const names = ['FAQ', 'TROUBLESHOOTING', 'CONFIGURATION', 'REVERSE_PROXY'];

function guide(name: string): HTMLElement {
  const root =
    names.includes(name) && process.env.PULSE_CORS_DOCS_TEST_DIR
      ? process.env.PULSE_CORS_DOCS_TEST_DIR
      : path.join(repoRoot, 'frontend-modern/public/docs');
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(readFileSync(path.join(root, `${name}.md`), 'utf8'), name);
  return article;
}

function section(name: string, id: string): HTMLElement {
  const heading = guide(name).querySelector(`#${id}`);
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

describe('shipped CORS guidance', () => {
  it('does not prescribe a wildcard for a proxy or login problem', () => {
    const text = prose(section('FAQ', 'cors-errors'));
    expect(text).toContain('same public origin, including behind a reverse proxy');
    expect(text).toContain('allow only its exact origin (scheme, host and port)');
    expect(text).toContain('* is not a login or proxy repair');
    expect(text).not.toContain('if you explicitly want all origins');
  });

  it('separates browser origin checks from platform connections and other failures', () => {
    const text = prose(section('TROUBLESHOOTING', 'cors-errors'));
    expect(text).toContain("not Pulse's connection to Proxmox, PBS or TrueNAS");
    expect(text).toContain("API request's origin and HTTP status");
    expect(text).toContain('certificate failure, login redirect or 401/403');
    expect(text).toContain('not a broader CORS allowlist');
    expect(text).toContain('do not expose the backend or add wildcard response headers');
  });

  it('describes exact origins, effective overrides and saved-value verification', () => {
    const element = section('TROUBLESHOOTING', 'cors-errors');
    const text = prose(element);
    expect(text).toContain('scheme, host and port');
    expect(text).toContain('Do not include a path, trailing slash or a hostname pattern');
    expect(text).toContain('Multiple exact origins are comma-separated');
    expect(text).toContain('removing that environment override does not erase');
    expect(text).toContain('verify the effective value and the ordinary browser request');
    expect(
      [...element.querySelectorAll('code')].some(
        (code) => code.textContent === 'https://app.example.com:8443',
      ),
    ).toBe(true);
  });

  it('keeps session credentials and security protections out of wildcard workarounds', () => {
    const text = prose(section('TROUBLESHOOTING', 'cors-errors'));
    expect(text).toContain('without credentialed browser access');
    expect(text).toContain('cannot fix a request that needs a browser session cookie');
    expect(text).toContain('Do not disable authentication, CSRF protection or TLS verification');
    expect(text).toContain('Iframe embedding and proxy authentication have separate settings');
    expect(text).toContain('empty policy grants no cross-origin browser permission');
  });

  it('does not ask for credential-bearing network exports or commands', () => {
    const element = section('TROUBLESHOOTING', 'cors-errors');
    const text = prose(element);
    expect(text).toContain(
      'Keep cookies, authorization headers, API tokens and full network exports private',
    );
    expect(text).toContain('redacted error, HTTP status and relevant origins');
    expect(text).toContain('not a credential-bearing request or HAR file');
    expect(element.querySelector('pre')).toBeNull();
  });

  it('makes the configuration reference and proxy guide agree with exact-origin policy', () => {
    const rows = [...guide('CONFIGURATION').querySelectorAll('tbody tr')];
    for (const key of ['allowedOrigins', 'ALLOWED_ORIGINS']) {
      const row = rows.find((candidate) => candidate.querySelector('td code')?.textContent === key);
      expect(row).toBeDefined();
      expect(prose(row as HTMLElement)).toContain('comma-separated exact origins');
      expect(prose(row as HTMLElement)).toMatch(
        /without credentials|without credentialed browser access/,
      );
    }
    expect(prose(guide('REVERSE_PROXY'))).toContain(
      'same-origin reverse proxy needs no CORS exception',
    );
  });

  it('resolves CORS and proxy links to real shipped headings', () => {
    const links = [
      ...section('FAQ', 'cors-errors').querySelectorAll('a[href]'),
      ...section('TROUBLESHOOTING', 'cors-errors').querySelectorAll('a[href]'),
      ...section('REVERSE_PROXY', 'before-configuring-the-proxy').querySelectorAll('a[href]'),
    ];
    expect(links.length).toBeGreaterThanOrEqual(4);
    for (const link of links) {
      const href = link.getAttribute('href')!;
      const match = /^\/docs\/([^#]+)(#.+)$/.exec(href);
      if (!match) continue;
      expect(guide(match[1]).querySelector(match[2]), href).not.toBeNull();
      expect(link.hasAttribute('data-doc-link')).toBe(true);
      expect(link.hasAttribute('target')).toBe(false);
    }
  });

  it('ships identical canonical and served guides', () => {
    for (const name of names) {
      expect(
        readFileSync(path.join(repoRoot, 'frontend-modern/public/docs', `${name}.md`), 'utf8'),
      ).toBe(readFileSync(path.join(repoRoot, 'docs', `${name}.md`), 'utf8'));
    }
  });
});
