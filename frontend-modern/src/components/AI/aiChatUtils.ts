import { marked } from 'marked';
import DOMPurify from 'dompurify';
import type { ModelInfo } from '@/types/ai';
export { getProviderFromModelId } from '@/utils/aiProviderPresentation';
import { getProviderFromModelId } from '@/utils/aiProviderPresentation';

// Group models by provider for grouped rendering
export function groupModelsByProvider(models: ModelInfo[]): Map<string, ModelInfo[]> {
  const grouped = new Map<string, ModelInfo[]>();

  for (const model of models) {
    const provider = getProviderFromModelId(model.id);
    const existing = grouped.get(provider) || [];
    existing.push(model);
    grouped.set(provider, existing);
  }

  return grouped;
}

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
    return DOMPurify.sanitize(rawHtml, {
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

// Helper to sanitize thinking/reasoning content for display
// Removes raw network errors with IP addresses that are not user-friendly
export const sanitizeThinking = (content: string): string => {
  // Replace raw TCP connection details like "write tcp 192.168.0.123:7655->192.168.0.134:58004: i/o timeout"
  // with friendlier messages
  // Replace "failed to send command: <raw error>" patterns first so follow-on replacements
  // don't partially sanitize the error and prevent the higher-level pattern from matching.
  let sanitized = content.replace(
    /failed to send command: write tcp [\d.:->\s]+/g,
    'failed to send command: connection error',
  );

  sanitized = sanitized.replace(
    /write tcp [\d.:]+->[\d.:]+: i\/o timeout/g,
    'connection timed out',
  );
  sanitized = sanitized.replace(/read tcp [\d.:]+: i\/o timeout/g, 'connection timed out');
  sanitized = sanitized.replace(/dial tcp [\d.:]+: connection refused/g, 'connection refused');
  return sanitized;
};

// Extract guest name from context if available
export const getGuestName = (context?: Record<string, unknown>): string | undefined => {
  if (!context) return undefined;
  if (typeof context.guestName === 'string') return context.guestName;
  if (typeof context.name === 'string') return context.name;
  return undefined;
};
