import { describe, expect, it, vi } from 'vitest';
import { highlightSettledCodeBlocks } from '../aiCodeHighlight';

// Branch-coverage companion to aiCodeHighlight.test.ts. Targets the two
// functions highlightSettledCodeBlocks (exported) and loadHighlighter
// (module-private). loadHighlighter's branches are driven through the
// public highlightSettledCodeBlocks entry point, or via vi.doMock +
// dynamic import to obtain a fresh module instance with a controlled
// highlight.js core.

const makeContainer = (html: string): HTMLElement => {
  const container = document.createElement('div');
  container.innerHTML = html;
  document.body.appendChild(container);
  return container;
};

// ---------------------------------------------------------------------------
// highlightSettledCodeBlocks — ternary: `language && hljs.getLanguage(language)`
// The sibling suite covers language='' (falsy short-circuit) and a registered
// language (truthy getLanguage). This covers the remaining arm: language is
// truthy but getLanguage returns undefined (unregistered grammar).
// ---------------------------------------------------------------------------

describe('highlightSettledCodeBlocks branch coverage — real highlighter', () => {
  it('falls back to highlightAuto when the fence language is truthy but unregistered', async () => {
    // 'rust' is not in the registered grammar set (bash/json/yaml/ini/sql/
    // dockerfile/nginx/plaintext), so getLanguage('rust') returns undefined
    // and the ternary takes the highlightAuto arm.
    const container = makeContainer(
      '<pre><code class="language-rust">{"name": "test", "count": 3}</code></pre>',
    );
    await highlightSettledCodeBlocks(container);

    const code = container.querySelector('code') as HTMLElement;
    expect(code.dataset.highlighted).toBe('true');
    expect(code.classList.contains('hljs-highlighted')).toBe(true);
    // highlightAuto (subset includes json) detected JSON and applied spans.
    expect(code.innerHTML).toContain('<span');
    expect(code.textContent).toBe('{"name": "test", "count": 3}');
  });
});

// ---------------------------------------------------------------------------
// loadHighlighter — catch arm (import failure → null), cached-null on retry,
// and the downstream `if (!hljs) return` early exit in highlightSettledCodeBlocks.
// ---------------------------------------------------------------------------

describe('loadHighlighter branch coverage — mocked core failures', () => {
  it('catches when core.default is undefined, returns null, and caches the null promise', async () => {
    vi.resetModules();
    // Mock core to an object with no `default` — core.default is undefined,
    // so hljs.registerLanguage throws inside the try, the catch arm logs
    // and returns null, and highlightSettledCodeBlocks hits `if (!hljs)`.
    vi.doMock('highlight.js/lib/core', () => ({}));

    const { highlightSettledCodeBlocks: highlight } = await import('../aiCodeHighlight');

    const c1 = makeContainer(
      '<pre><code class="language-bash">echo "hello" # comment</code></pre>',
    );
    await highlight(c1);
    const code1 = c1.querySelector('code') as HTMLElement;
    // loadHighlighter catch → null → if (!hljs) return: block untouched.
    expect(code1.dataset.highlighted).toBeUndefined();
    expect(code1.classList.contains('hljs-highlighted')).toBe(false);
    expect(code1.innerHTML).toBe('echo "hello" # comment');

    // Second call: hljsPromise is cached (resolved with null) — the
    // if (!hljsPromise) false arm returns the cached null without retrying.
    const c2 = makeContainer('<pre><code class="language-json">{"a": 1}</code></pre>');
    await highlight(c2);
    const code2 = c2.querySelector('code') as HTMLElement;
    expect(code2.dataset.highlighted).toBeUndefined();
    expect(code2.innerHTML).toBe('{"a": 1}');
  });

  // -------------------------------------------------------------------------
  // highlightSettledCodeBlocks — catch arm around the highlight call.
  // A fake hljs whose highlight/highlightAuto throws exercises the catch
  // block: sets data-highlighted='true' but NOT the hljs-highlighted class
  // and leaves innerHTML untouched.
  // -------------------------------------------------------------------------

  it('marks a block highlighted without the class when hljs.highlight throws (language path)', async () => {
    vi.resetModules();
    const throwingHljs = {
      registerLanguage: vi.fn(),
      registerAliases: vi.fn(),
      getLanguage: vi.fn(() => ({ name: 'bash' })),
      highlight: vi.fn(() => {
        throw new Error('highlight boom');
      }),
      highlightAuto: vi.fn(),
    };
    vi.doMock('highlight.js/lib/core', () => ({ default: throwingHljs }));

    const { highlightSettledCodeBlocks: highlight } = await import('../aiCodeHighlight');
    const container = makeContainer(
      '<pre><code class="language-bash">echo hello</code></pre>',
    );
    await highlight(container);

    const code = container.querySelector('code') as HTMLElement;
    // Catch arm: data-highlighted set, class NOT added, innerHTML untouched.
    expect(code.dataset.highlighted).toBe('true');
    expect(code.classList.contains('hljs-highlighted')).toBe(false);
    expect(code.innerHTML).toBe('echo hello');
    expect(throwingHljs.highlight).toHaveBeenCalledTimes(1);
    expect(throwingHljs.highlightAuto).not.toHaveBeenCalled();
  });

  it('marks a block highlighted without the class when hljs.highlightAuto throws (auto path)', async () => {
    vi.resetModules();
    const throwingHljs = {
      registerLanguage: vi.fn(),
      registerAliases: vi.fn(),
      getLanguage: vi.fn(() => undefined),
      highlight: vi.fn(),
      highlightAuto: vi.fn(() => {
        throw new Error('auto boom');
      }),
    };
    vi.doMock('highlight.js/lib/core', () => ({ default: throwingHljs }));

    const { highlightSettledCodeBlocks: highlight } = await import('../aiCodeHighlight');
    // No language- class → fenceLanguage returns '' → falsy → highlightAuto.
    const container = makeContainer('<pre><code>plain text here</code></pre>');
    await highlight(container);

    const code = container.querySelector('code') as HTMLElement;
    expect(code.dataset.highlighted).toBe('true');
    expect(code.classList.contains('hljs-highlighted')).toBe(false);
    expect(code.innerHTML).toBe('plain text here');
    expect(throwingHljs.highlightAuto).toHaveBeenCalledTimes(1);
    expect(throwingHljs.highlight).not.toHaveBeenCalled();
  });
});
