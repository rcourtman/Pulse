import { describe, expect, it } from 'vitest';
import {
  docAssetUrlForPath,
  docRouteForPath,
  extractDocTitle,
  normalizeDocPath,
  renderDocMarkdown,
  resolveDocLink,
  stripLeadingTitle,
  wrapTables,
} from '../docMarkdown';

describe('normalizeDocPath', () => {
  it('strips the docs prefix, leading slashes, and the .md suffix', () => {
    expect(normalizeDocPath('/docs/INSTALL.md')).toBe('INSTALL');
    expect(normalizeDocPath('docs/i18n/de/README.md')).toBe('i18n/de/README');
    expect(normalizeDocPath('INSTALL')).toBe('INSTALL');
    expect(normalizeDocPath('  /FAQ.md  ')).toBe('FAQ');
  });
});

describe('doc URLs', () => {
  it('separates the viewer route from the raw asset', () => {
    expect(docRouteForPath('INSTALL.md')).toBe('/docs/INSTALL');
    expect(docAssetUrlForPath('INSTALL')).toBe('/docs/INSTALL.md');
    expect(docAssetUrlForPath('i18n/de/README.md')).toBe('/docs/i18n/de/README.md');
  });
});

describe('resolveDocLink', () => {
  it('resolves sibling documents', () => {
    expect(resolveDocLink('README', 'INSTALL.md')).toBe('/docs/INSTALL');
    expect(resolveDocLink('README', './FAQ.md')).toBe('/docs/FAQ');
  });

  it('resolves relative to the containing document', () => {
    expect(resolveDocLink('i18n/de/README', '../README.md')).toBe('/docs/i18n/README');
    expect(resolveDocLink('i18n/de/README', 'CONFIGURATION.md')).toBe(
      '/docs/i18n/de/CONFIGURATION',
    );
  });

  // Repo docs are authored for GitHub, where docs/ sits inside the repository,
  // so they climb out of the docs directory. Under the shipped root there is
  // nothing above it and those files ship flat, so the climb clamps.
  it('clamps links that climb above the docs root', () => {
    expect(resolveDocLink('README', '../SECURITY.md')).toBe('/docs/SECURITY');
    expect(resolveDocLink('README', '../../TERMS.md')).toBe('/docs/TERMS');
  });

  // Documents shipped from the repository root, such as CONTRIBUTING.md, link
  // as docs/X.md because that is the path from the root. They ship flat into
  // the docs root, so the prefix has to collapse rather than nest.
  it('collapses the docs/ prefix used by root-level documents', () => {
    expect(resolveDocLink('CONTRIBUTING', 'docs/AI_TRANSPARENCY.md')).toBe('/docs/AI_TRANSPARENCY');
    expect(resolveDocLink('CONTRIBUTING', 'SECURITY.md')).toBe('/docs/SECURITY');
  });

  it('preserves fragments', () => {
    expect(resolveDocLink('README', 'CONFIGURATION.md#api-tokens')).toBe(
      '/docs/CONFIGURATION#api-tokens',
    );
  });

  it('ignores anything that is not an intra-doc markdown link', () => {
    expect(resolveDocLink('README', 'https://example.com/page.md')).toBeNull();
    expect(resolveDocLink('README', 'https://github.com/rcourtman/Pulse')).toBeNull();
    expect(resolveDocLink('README', '#section')).toBeNull();
    expect(resolveDocLink('README', 'mailto:someone@example.com')).toBeNull();
  });
});

describe('renderDocMarkdown', () => {
  it('does not turn hard-wrapped prose into visible line breaks', () => {
    // The AI chat renderer sets breaks: true, which would put a <br> between
    // these two lines. Documentation is hard-wrapped, so that would break
    // every paragraph in the shipped set.
    const html = renderDocMarkdown('A wrapped\nparagraph.', 'README');
    expect(html).not.toContain('<br');
    expect(html).toContain('A wrapped\nparagraph.');
  });

  it('renders standard markdown structure', () => {
    const html = renderDocMarkdown('# Title\n\n- one\n- two\n', 'README');
    expect(html).toContain('<h1');
    expect(html).toContain('<li>one</li>');
  });

  it('points intra-doc links at the viewer and marks them for routing', () => {
    const html = renderDocMarkdown('[Install](INSTALL.md)', 'README');
    expect(html).toContain('href="/docs/INSTALL"');
    expect(html).toContain('data-doc-link');
    expect(html).not.toContain('target="_blank"');
  });

  it('opens external links in a new tab', () => {
    const html = renderDocMarkdown('[Site](https://example.com)', 'README');
    expect(html).toContain('target="_blank"');
    expect(html).toContain('rel="noopener noreferrer"');
  });

  it('strips scripts and event handlers', () => {
    const html = renderDocMarkdown(
      '<script>alert(1)</script>\n\n<img src=x onerror="alert(1)">\n\n[x](javascript:alert(1))',
      'README',
    );
    expect(html).not.toContain('<script');
    expect(html).not.toContain('onerror');
    expect(html).not.toContain('javascript:');
  });

  it('neutralizes an advisory-shaped nested resource payload before link rewriting', () => {
    const html = renderDocMarkdown(
      [
        '<footer>',
        '  <img src="x" onload="alert(1)" onerror="alert(2)">',
        '</footer>',
        '<div><a href="INSTALL.md">safe link</a></div>',
      ].join('\n'),
      'README',
    );
    const template = document.createElement('template');
    template.innerHTML = html;

    expect(template.content.querySelector('footer, [onload], [onerror], script')).toBeNull();
    const image = template.content.querySelector('img');
    expect(image?.getAttribute('src')).toBe('x');
    const link = template.content.querySelector('a');
    expect(link?.getAttribute('href')).toBe('/docs/INSTALL');
    expect(link?.hasAttribute('data-doc-link')).toBe(true);
  });
});

describe('wrapTables', () => {
  // Making the table itself display:block; overflow-x:auto is not enough, its
  // rows still lay out past the block box and push the page into horizontal
  // scrolling on a phone. A wrapping element is what contains the overflow.
  it('gives each table its own scroll container', () => {
    const container = window.document.createElement('div');
    container.innerHTML = '<table><tbody><tr><td>a</td></tr></tbody></table>';
    wrapTables(container);
    const wrapper = container.querySelector('[data-doc-table-scroll]');
    expect(wrapper).not.toBeNull();
    expect(wrapper?.className).toContain('overflow-x-auto');
    expect(wrapper?.querySelector('table')).not.toBeNull();
  });

  it('does not wrap a table twice', () => {
    const container = window.document.createElement('div');
    container.innerHTML = '<table><tbody><tr><td>a</td></tr></tbody></table>';
    wrapTables(container);
    wrapTables(container);
    expect(container.querySelectorAll('[data-doc-table-scroll]').length).toBe(1);
  });
});

describe('stripLeadingTitle', () => {
  it('removes the title the viewer promotes into the page header', () => {
    expect(stripLeadingTitle('# Title\n\nBody text')).toBe('Body text');
  });

  it('leaves a level-one heading that is not the document title', () => {
    const markdown = 'Intro paragraph\n\n# Later heading\n';
    expect(stripLeadingTitle(markdown)).toBe(markdown);
  });

  it('leaves documents with no heading untouched', () => {
    expect(stripLeadingTitle('Just body')).toBe('Just body');
  });
});

describe('extractDocTitle', () => {
  it('uses the first level-one heading', () => {
    expect(extractDocTitle('# Installation Guide\n\nBody', 'fallback')).toBe('Installation Guide');
  });

  it('falls back when there is no heading', () => {
    expect(extractDocTitle('Body only', 'INSTALL')).toBe('INSTALL');
  });

  it('does not treat a mid-document heading as the title', () => {
    expect(extractDocTitle('Intro\n\n# Later heading', 'FAQ')).toBe('FAQ');
  });
});
