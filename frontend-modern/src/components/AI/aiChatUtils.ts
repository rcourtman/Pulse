import { marked } from 'marked';
import DOMPurify from 'dompurify';

// Configure marked for safe rendering.
//
// DOMPurify runs downstream of marked.parse and is the actual XSS gate, but
// we set marked options here to keep its output predictable rather than
// relying solely on the downstream sanitizer. Raw HTML coming out of an LLM
// would otherwise pass through marked unchanged (marked allows raw HTML by
// default since v9) and rely on DOMPurify to clean it. Defence-in-depth: a
// future DOMPurify-config edit that accidentally loosens the allowlist
// shouldn't suddenly mean script tags from the LLM hit the DOM.
marked.setOptions({
  breaks: true, // Convert \n to <br>
  gfm: true, // GitHub Flavored Markdown
});

let domPurifyConfigured = false;
const configureDOMPurify = () => {
  if (domPurifyConfigured) return;
  domPurifyConfigured = true;

  DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    const element = node as Element | null;
    if (!element || element.tagName !== 'A') return;
    element.setAttribute('target', '_blank');
    element.setAttribute('rel', 'noopener noreferrer');
  });
};

const coerceMarkdownInput = (content: unknown): string => {
  if (typeof content === 'string') return content;
  if (content && typeof content === 'object') {
    const record = content as Record<string, unknown>;
    if (typeof record.text === 'string') return record.text;
    if (typeof record.content === 'string') return record.content;
    try {
      return JSON.stringify(content);
    } catch {
      return String(content);
    }
  }
  return content == null ? '' : String(content);
};

// Helper to render markdown safely with XSS protection
// LLM output should NEVER be trusted - always sanitize before rendering as HTML
export const renderMarkdown = (content: unknown): string => {
  const normalized = coerceMarkdownInput(content);
  try {
    configureDOMPurify();
    const rawHtml = marked.parse(normalized) as string;
    // Sanitize to prevent XSS from malicious LLM output or injected content.
    //
    // Hardening notes:
    //  - ALLOWED_URI_REGEXP pins href values to http/https/mailto. Without
    //    this we rely on DOMPurify's internal default URI scheme filter,
    //    which strips javascript:/data: in current versions but is library-
    //    version dependent; an explicit allowlist regex makes the intent
    //    survive future updates.
    //  - 'class' was previously in ALLOWED_ATTR. The LLM has no legitimate
    //    reason to apply utility classes, and allowing arbitrary classes
    //    opens a UI-redressing surface (e.g. injecting "fixed inset-0 z-50
    //    bg-black" to overlay the page). Drop it.
    //  - 'target' and 'rel' are not in ALLOWED_ATTR because the
    //    afterSanitizeAttributes hook above sets them on every <a>; they
    //    don't need to be parsed in from the LLM-supplied HTML.
    const sanitized = DOMPurify.sanitize(rawHtml, {
      // Allow common formatting tags but block scripts, iframes, etc.
      ALLOWED_TAGS: [
        'p',
        'br',
        'strong',
        'em',
        'b',
        'i',
        'u',
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
      ],
      ALLOWED_ATTR: ['href'],
      ALLOWED_URI_REGEXP: /^(?:https?:|mailto:)/i,
      // Force all links to open in new tab and prevent opener attacks
      ADD_ATTR: ['target', 'rel'],
    });
    // Add application-owned scrolling only after sanitization. Model HTML
    // cannot supply classes or attributes that escape the message layout.
    const template = document.createElement('template');
    template.innerHTML = sanitized;
    for (const table of template.content.querySelectorAll('table')) {
      const region = document.createElement('div');
      region.className =
        'max-w-full overflow-x-auto focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-blue-500';
      region.tabIndex = 0;
      region.setAttribute('role', 'region');
      region.setAttribute('aria-label', 'Scrollable table');
      table.replaceWith(region);
      region.appendChild(table);
    }
    return template.innerHTML;
  } catch {
    // If parsing fails, escape HTML entities as fallback
    return normalized.replace(/[&<>"']/g, (char) => {
      const entities: Record<string, string> = {
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#39;',
      };
      return entities[char];
    });
  }
};
