import { describe, expect, it } from 'vitest';
import { renderDocMarkdown, resolveDocLink } from '../docMarkdown';

// The stored-XSS annotation on the post-sanitization href setter must be
// assessed at that setter, not just with a script-tag test. Both arguments
// are treated as untrusted here, including the document route itself.
const currentPaths = [
  'README',
  'i18n/de/README',
  '../../README',
  '//untrusted.invalid/README',
  'https://untrusted.invalid/README',
  'javascript:globalThis.__docSecurityProbe=1/README',
  'data:text/html,<script>probe()</script>/README',
  '..\\..\\untrusted.invalid/README',
  '%2e%2e/%2e%2e/README',
  'README"><img src=x onerror="probe()">/README',
];
const relativeLinks = [
  'INSTALL.md',
  './FAQ.md',
  '../SECURITY.md#reporting',
  '../../TERMS.md?return=javascript:probe()',
  '/docs/CONFIGURATION.md#api-tokens',
  '//untrusted.invalid/INSTALL.md',
  '\\untrusted.invalid\\INSTALL.md',
  'java\tscript:probe.md',
  'java\nscript:probe.md',
  '%6aavascript%3aprobe.md',
  '%2e%2e/%2e%2e/INSTALL.md',
  '%2f%2funtrusted.invalid/INSTALL.md',
  'INSTALL.md#javascript:probe()',
  'INSTALL.md#"><img src=x onerror="probe()">',
  'INSTALL.md?x=" onfocus="probe()',
  'INSTALL.md#%22%3E%3Cscript%3Eprobe()%3C/script%3E',
];
const bases = ['http://pulse.invalid/', 'https://pulse.invalid/prefix/'];

function articleAt(baseUrl: string, html: string): HTMLDivElement {
  // Use the DOM's URL properties as well as URL(), without altering the test
  // runner's own base URL or enabling script execution in a fixture.
  const owner = document.implementation.createHTMLDocument();
  const base = owner.createElement('base');
  base.href = baseUrl;
  owner.head.appendChild(base);
  const article = owner.createElement('div');
  article.innerHTML = html;
  owner.body.appendChild(article);
  return article;
}

function linkHtml(href: string): string {
  const anchor = document.createElement('a');
  anchor.setAttribute('href', href);
  anchor.textContent = 'Document';
  return anchor.outerHTML;
}

describe('documentation rewritten-link security boundary', () => {
  it.each(bases)('keeps resolved URLs non-executable and same-origin at %s', (base) => {
    for (const currentPath of currentPaths) {
      for (const href of relativeLinks) {
        const label = JSON.stringify({ currentPath, href, base });
        const resolved = resolveDocLink(currentPath, href);
        expect(resolved, label).not.toBeNull();
        expect(resolved, label).toMatch(/^\/docs\//);
        const url = new URL(resolved!, base);
        expect(url.protocol, label).toBe(new URL(base).protocol);
        expect(url.origin, label).toBe(new URL(base).origin);
      }
    }
  });

  it.each(bases)('keeps the actual rendered href setter safe at %s', (base) => {
    let rewritten = 0;
    for (const currentPath of currentPaths) {
      for (const href of relativeLinks) {
        const label = JSON.stringify({ currentPath, href, base });
        const article = articleAt(base, renderDocMarkdown(linkHtml(href), currentPath));
        const anchor = article.querySelector('a')!;
        expect(anchor, label).not.toBeNull();
        expect(anchor.textContent, label).toBe('Document');
        // DOMPurify may remove an obfuscated scheme before the rewrite. If
        // an href survives, the emitted DOM, not only the resolver, is safe.
        if (!anchor.hasAttribute('href')) continue;
        rewritten += 1;
        expect(anchor.hasAttribute('data-doc-link'), label).toBe(true);
        expect(anchor.getAttribute('href'), label).toMatch(/^\/docs\//);
        expect(anchor.protocol, label).toBe(new URL(base).protocol);
        expect(anchor.origin, label).toBe(new URL(base).origin);
        expect(article.querySelector('script, svg, [onerror], [onfocus]'), label).toBeNull();
      }
    }
    // Prevent a sanitizer that removes every link from passing vacuously.
    expect(rewritten).toBeGreaterThanOrEqual(120);
  });

  it('rejects explicit executable schemes rather than treating them as documents', () => {
    for (const href of [
      'javascript:probe.md',
      'JaVaScRiPt:probe.md#fragment',
      'data:text/html,probe.md',
      'vbscript:probe.md',
      'file:probe.md',
      'blob:probe.md',
      'https://untrusted.invalid/probe.md',
      'mailto:probe.md',
    ]) {
      expect(resolveDocLink('README', href), href).toBeNull();
    }
  });

  it('removes entity- and whitespace-obfuscated executable hrefs before rewriting', () => {
    for (const href of [
      'javascript:probe.md',
      'jav&#x61;script:probe.md',
      'java&#x09;script:probe.md',
      'java&#x0a;script:probe.md',
      'java&#x0d;script:probe.md',
      'data:text/html,probe.md',
      'vbscript:probe.md',
    ]) {
      // Deliberately raw HTML: entities must pass through the real parser,
      // not through linkHtml's escaping or a replacement sanitizer.
      const article = articleAt(
        bases[1],
        renderDocMarkdown(`<a href="${href}">Document</a>`, 'README'),
      );
      const anchor = article.querySelector('a')!;
      expect(anchor.textContent, href).toBe('Document');
      expect(anchor.hasAttribute('href'), href).toBe(false);
      expect(anchor.hasAttribute('data-doc-link'), href).toBe(false);
    }
  });

  it('keeps HTML inert through sanitization, link mutation and a second HTML parse', () => {
    const article = articleAt(
      bases[1],
      renderDocMarkdown(
        [
          '<script>globalThis.__docSecurityProbe=1</script>',
          '<img src="x" onerror="globalThis.__docSecurityProbe=2">',
          '<svg><a href="javascript:probe()">bad</a></svg>',
          '<div><a href="INSTALL.md#&quot;&gt;&lt;img src=x onerror=probe()&gt;" onclick="probe()">Install</a></div>',
          '<iframe srcdoc="<script>probe()</script>"></iframe>',
        ].join('\n'),
        'README',
      ),
    );
    expect(article.querySelector('script, svg, iframe, [onclick], [onerror]')).toBeNull();
    const link = article.querySelector<HTMLAnchorElement>('a[data-doc-link]')!;
    expect(link.textContent).toBe('Install');
    expect(link.getAttribute('href')).toBe('/docs/INSTALL#"><img src=x onerror=probe()>');
    expect(link.origin).toBe(new URL(bases[1]).origin);
    expect(article.querySelectorAll('img')).toHaveLength(1);
  });
});
