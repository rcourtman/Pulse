import { marked } from 'marked';
import DOMPurify from 'dompurify';

// Shipped documentation lives at /docs/<path>.md as a static asset. The
// rendered viewer is the same path without the .md suffix, which cannot
// collide because every shipped file ends in .md and anything else falls
// through to the SPA.
export const DOC_ROUTE_PREFIX = '/docs';

export const docRouteForPath = (docPath: string): string =>
  `${DOC_ROUTE_PREFIX}/${normalizeDocPath(docPath)}`;

export const docAssetUrlForPath = (docPath: string): string =>
  `${DOC_ROUTE_PREFIX}/${normalizeDocPath(docPath)}.md`;

/** Strips a leading slash, a docs prefix, and a trailing .md suffix. */
export function normalizeDocPath(raw: string): string {
  let value = (raw ?? '').trim();
  value = value.replace(/^\/+/, '');
  if (value.toLowerCase().startsWith('docs/')) value = value.slice('docs/'.length);
  value = value.replace(/\.md$/i, '');
  value = value.replace(/^\/+|\/+$/g, '');
  return value;
}

/**
 * Resolves a relative markdown link against the document containing it.
 *
 * Repo docs are written for GitHub, where docs/ sits inside the repository, so
 * they contain links such as `../SECURITY.md` that climb above the docs root.
 * Under the shipped /docs root there is nothing above, and those files are
 * shipped flat alongside the rest, so a climb that escapes the root is clamped
 * to it rather than left dangling.
 */
export function resolveDocLink(currentDocPath: string, href: string): string | null {
  const trimmed = (href ?? '').trim();
  if (!trimmed || !/\.md(?:[?#].*)?$/i.test(trimmed)) return null;
  if (/^[a-z][a-z0-9+.-]*:/i.test(trimmed)) return null; // absolute URL, not ours

  const [pathPart, fragment = ''] = splitFragment(trimmed);
  const baseSegments = normalizeDocPath(currentDocPath).split('/').slice(0, -1);
  const segments = pathPart.startsWith('/') ? [] : [...baseSegments];

  for (const segment of pathPart.replace(/^\/+/, '').split('/')) {
    if (!segment || segment === '.') continue;
    if (segment === '..') {
      segments.pop();
      continue;
    }
    segments.push(segment);
  }

  const resolved = normalizeDocPath(segments.join('/'));
  if (!resolved) return null;
  return `${docRouteForPath(resolved)}${fragment}`;
}

function splitFragment(value: string): [string, string] {
  const index = value.search(/[?#]/);
  if (index < 0) return [value, ''];
  return [value.slice(0, index), value.slice(index)];
}

/**
 * Renders shipped documentation markdown to sanitised HTML.
 *
 * Deliberately not the AI chat's renderMarkdown: that one sets `breaks: true`
 * globally, which turns every hard-wrapped line in a normal document into a
 * visible break, and pins hrefs to http/https/mailto, which strips the
 * relative links that hold the documentation set together. Options are passed
 * per parse call here so neither renderer can disturb the other.
 */
export function renderDocMarkdown(markdown: string, currentDocPath: string): string {
  const source = typeof markdown === 'string' ? markdown : '';
  const rawHtml = marked.parse(source, { gfm: true, breaks: false, async: false }) as string;

  const sanitized = DOMPurify.sanitize(rawHtml, {
    ALLOWED_TAGS: [
      'p',
      'br',
      'strong',
      'em',
      'b',
      'i',
      'u',
      'del',
      'code',
      'pre',
      'blockquote',
      'ul',
      'ol',
      'li',
      'h1',
      'h2',
      'h3',
      'h4',
      'h5',
      'h6',
      'a',
      'hr',
      'table',
      'thead',
      'tbody',
      'tr',
      'th',
      'td',
      'span',
      'div',
      'img',
      'details',
      'summary',
    ],
    // Relative hrefs must survive so intra-document links keep working; they
    // are rewritten to viewer routes below. 'class' stays out for the same
    // UI-redressing reason the chat renderer excludes it.
    ALLOWED_ATTR: ['href', 'src', 'alt', 'title', 'id', 'align'],
    ALLOWED_URI_REGEXP: /^(?:https?:|mailto:|[^a-z0-9+.-]|[^:]*$)/i,
  });

  return rewriteDocLinks(sanitized, currentDocPath);
}

/**
 * Wraps tables in their own horizontal scroll container.
 *
 * Making the table itself `display: block; overflow-x: auto` is not enough:
 * its rows still lay out past the block box, so a wide table pushes the whole
 * page into horizontal scrolling on a phone. A wrapping element is what
 * actually contains the overflow.
 */
export function wrapTables(container: HTMLElement): void {
  container.querySelectorAll('table').forEach((table) => {
    const parent = table.parentElement;
    if (parent?.dataset?.docTableScroll !== undefined) return;
    const wrapper = window.document.createElement('div');
    wrapper.dataset.docTableScroll = '';
    wrapper.className = 'max-w-full overflow-x-auto';
    table.replaceWith(wrapper);
    wrapper.appendChild(table);
  });
}

/**
 * Points intra-documentation links at the viewer instead of the raw asset, so
 * following one renders the next document rather than downloading its source.
 */
export function rewriteDocLinks(html: string, currentDocPath: string): string {
  if (typeof document === 'undefined') return html;
  const container = document.createElement('div');
  container.innerHTML = html;
  wrapTables(container);

  container.querySelectorAll('a[href]').forEach((anchor) => {
    const href = anchor.getAttribute('href') ?? '';
    const resolved = resolveDocLink(currentDocPath, href);
    if (resolved) {
      anchor.setAttribute('href', resolved);
      anchor.setAttribute('data-doc-link', '');
      anchor.removeAttribute('target');
      anchor.removeAttribute('rel');
      return;
    }
    if (/^https?:/i.test(href)) {
      anchor.setAttribute('target', '_blank');
      anchor.setAttribute('rel', 'noopener noreferrer');
    }
  });

  return container.innerHTML;
}

/** First level-one heading, used as the document title. */
export function extractDocTitle(markdown: string, fallback: string): string {
  const index = leadingTitleLineIndex(markdown);
  if (index < 0) return fallback;
  return /^#\s+(.+?)\s*$/.exec((markdown ?? '').split('\n')[index])![1].trim();
}

/**
 * Removes the leading level-one heading, which the viewer promotes into the
 * page header. Leaving it in renders the document title twice.
 */
export function stripLeadingTitle(markdown: string): string {
  const lines = (markdown ?? '').split('\n');
  const index = leadingTitleLineIndex(markdown);
  if (index < 0) return markdown ?? '';
  lines.splice(index, 1);
  while (lines.length && lines[0].trim() === '') lines.shift();
  return lines.join('\n');
}

/**
 * Index of the document's own title line, meaning a level-one heading that
 * appears before any other content. A `# ` further down belongs to the body.
 */
function leadingTitleLineIndex(markdown: string): number {
  const lines = (markdown ?? '').split('\n');
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index];
    if (line.trim() === '') continue;
    return /^#\s+.+/.test(line) ? index : -1;
  }
  return -1;
}
