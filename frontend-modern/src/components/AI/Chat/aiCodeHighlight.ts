import { logger } from '@/utils/logger';
import type { HLJSApi } from 'highlight.js';

// Lazy-loaded so highlight.js bills to an async chunk, never the entry
// bundle. Grammar set is the infra vocabulary Assistant answers actually
// contain; everything else renders unhighlighted rather than pulling in the
// full grammar pack.
let hljsPromise: Promise<HLJSApi | null> | undefined;

const loadHighlighter = (): Promise<HLJSApi | null> => {
  if (!hljsPromise) {
    hljsPromise = (async () => {
      try {
        const [core, bash, json, yaml, ini, sql, dockerfile, nginx, plaintext] =
          await Promise.all([
            import('highlight.js/lib/core'),
            import('highlight.js/lib/languages/bash'),
            import('highlight.js/lib/languages/json'),
            import('highlight.js/lib/languages/yaml'),
            import('highlight.js/lib/languages/ini'),
            import('highlight.js/lib/languages/sql'),
            import('highlight.js/lib/languages/dockerfile'),
            import('highlight.js/lib/languages/nginx'),
            import('highlight.js/lib/languages/plaintext'),
          ]);
        const hljs = core.default;
        hljs.registerLanguage('bash', bash.default);
        hljs.registerAliases(['sh', 'shell', 'zsh', 'console'], { languageName: 'bash' });
        hljs.registerLanguage('json', json.default);
        hljs.registerLanguage('yaml', yaml.default);
        hljs.registerAliases(['yml'], { languageName: 'yaml' });
        hljs.registerLanguage('ini', ini.default);
        hljs.registerAliases(['toml', 'conf'], { languageName: 'ini' });
        hljs.registerLanguage('sql', sql.default);
        hljs.registerLanguage('dockerfile', dockerfile.default);
        hljs.registerLanguage('nginx', nginx.default);
        hljs.registerLanguage('plaintext', plaintext.default);
        return hljs;
      } catch (error) {
        logger.debug('[AICodeHighlight] Failed to load highlighter', { error });
        return null;
      }
    })();
  }
  return hljsPromise;
};

const fenceLanguage = (code: Element): string => {
  for (const cls of Array.from(code.classList)) {
    if (cls.startsWith('language-')) return cls.slice('language-'.length).toLowerCase();
  }
  return '';
};

// Highlights fenced code blocks inside a settled (non-streaming) markdown
// container. Runs over the sanitized DOM, so highlight spans never pass
// through DOMPurify and the 'class' allowlist stays closed to the model.
// Idempotent per block via data-highlighted.
export const highlightSettledCodeBlocks = async (container: HTMLElement): Promise<void> => {
  const blocks = Array.from(container.querySelectorAll('pre code')).filter(
    (code) => !(code as HTMLElement).dataset.highlighted,
  );
  if (blocks.length === 0) return;

  const hljs = await loadHighlighter();
  if (!hljs) return;

  for (const code of blocks) {
    const element = code as HTMLElement;
    if (element.dataset.highlighted) continue;
    const text = element.textContent || '';
    if (!text.trim()) continue;
    const language = fenceLanguage(element);
    try {
      const result =
        language && hljs.getLanguage(language)
          ? hljs.highlight(text, { language, ignoreIllegals: true })
          : hljs.highlightAuto(text, ['bash', 'json', 'yaml', 'ini']);
      element.innerHTML = result.value;
      element.dataset.highlighted = 'true';
      element.classList.add('hljs-highlighted');
    } catch (error) {
      logger.debug('[AICodeHighlight] Failed to highlight block', { error });
      element.dataset.highlighted = 'true';
    }
  }
};
