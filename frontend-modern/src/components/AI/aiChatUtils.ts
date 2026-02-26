import { marked } from 'marked';
import DOMPurify from 'dompurify';
import type { ModelInfo } from '@/types/ai';

// Provider display names for grouped model selection
export const PROVIDER_DISPLAY_NAMES: Record<string, string> = {
  anthropic: 'Anthropic',
  openai: 'OpenAI',
  deepseek: 'DeepSeek',
  gemini: 'Google Gemini',
  ollama: 'Ollama',
};

// Known provider prefixes — only these are treated as explicit "provider:model" delimiters.
// This avoids misinterpreting colons in model names like "llama3.2:latest" or "model:free".
const KNOWN_PROVIDERS = ['anthropic', 'openai', 'deepseek', 'gemini', 'ollama'];

// Parse provider from model ID (format: "provider:model-name")
export function getProviderFromModelId(modelId: string): string {
  // Check for explicit known provider prefix (e.g. "openai:gpt-4o")
  const colonIndex = modelId.indexOf(':');
  if (colonIndex > 0) {
    const prefix = modelId.substring(0, colonIndex);
    if (KNOWN_PROVIDERS.includes(prefix)) {
      return prefix;
    }
  }
  // Vendor-prefixed names like "google/gemini-*" or "meta-llama/llama-*" are
  // OpenRouter model IDs routed through the OpenAI-compatible provider.
  if (modelId.includes('/')) {
    return 'openai';
  }
  // Strip colon suffix for detection (e.g. "llama3.2:latest" → "llama3.2")
  const name = colonIndex > 0 ? modelId.substring(0, colonIndex) : modelId;
  // Default detection for models without prefix
  if (name.startsWith('claude') || name.startsWith('opus') || name.startsWith('sonnet') || name.startsWith('haiku')) {
    return 'anthropic';
  }
  if (name.startsWith('gpt') || name.startsWith('o1') || name.startsWith('o3') || name.startsWith('o4')) {
    return 'openai';
  }
  if (name.startsWith('deepseek')) {
    return 'deepseek';
  }
  if (name.startsWith('gemini')) {
    return 'gemini';
  }
  return 'ollama';
}

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

// Configure marked for safe rendering
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
    // Sanitize to prevent XSS from malicious LLM output or injected content
    return DOMPurify.sanitize(rawHtml, {
      // Allow common formatting tags but block scripts, iframes, etc.
      ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'b', 'i', 'u', 'code', 'pre', 'blockquote',
        'ul', 'ol', 'li', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'a', 'hr', 'table',
        'thead', 'tbody', 'tr', 'th', 'td', 'span', 'div'],
      ALLOWED_ATTR: ['href', 'target', 'rel', 'class'],
      // Force all links to open in new tab and prevent opener attacks
      ADD_ATTR: ['target', 'rel'],
    });
  } catch {
    // If parsing fails, escape HTML entities as fallback
    return normalized.replace(/[&<>"']/g, (char) => {
      const entities: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
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
    'failed to send command: connection error'
  );

  sanitized = sanitized.replace(
    /write tcp [\d.:]+->[\d.:]+: i\/o timeout/g,
    'connection timed out'
  );
  sanitized = sanitized.replace(
    /read tcp [\d.:]+: i\/o timeout/g,
    'connection timed out'
  );
  sanitized = sanitized.replace(
    /dial tcp [\d.:]+: connection refused/g,
    'connection refused'
  );
  return sanitized;
};

// Extract guest name from context if available
export const getGuestName = (context?: Record<string, unknown>): string | undefined => {
  if (!context) return undefined;
  if (typeof context.guestName === 'string') return context.guestName;
  if (typeof context.name === 'string') return context.name;
  return undefined;
};
