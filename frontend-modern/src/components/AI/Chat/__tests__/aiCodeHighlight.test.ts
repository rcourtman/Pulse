import { describe, expect, it } from 'vitest';
import { highlightSettledCodeBlocks } from '../aiCodeHighlight';

const makeContainer = (html: string): HTMLElement => {
  const container = document.createElement('div');
  container.innerHTML = html;
  document.body.appendChild(container);
  return container;
};

describe('aiCodeHighlight', () => {
  it('highlights a fenced bash block using the language hint', async () => {
    const container = makeContainer(
      '<pre><code class="language-bash">echo "hello" # comment</code></pre>',
    );
    await highlightSettledCodeBlocks(container);

    const code = container.querySelector('code') as HTMLElement;
    expect(code.dataset.highlighted).toBe('true');
    expect(code.classList.contains('hljs-highlighted')).toBe(true);
    expect(code.querySelector('.hljs-string')).not.toBeNull();
    expect(code.querySelector('.hljs-comment')).not.toBeNull();
    // Highlighting must not alter the visible text.
    expect(code.textContent).toBe('echo "hello" # comment');
  });

  it('is idempotent: a highlighted block is not reprocessed', async () => {
    const container = makeContainer('<pre><code class="language-json">{"a": 1}</code></pre>');
    await highlightSettledCodeBlocks(container);
    const firstPass = (container.querySelector('code') as HTMLElement).innerHTML;
    await highlightSettledCodeBlocks(container);
    expect((container.querySelector('code') as HTMLElement).innerHTML).toBe(firstPass);
  });

  it('falls back to auto-detection when the fence has no usable hint', async () => {
    const container = makeContainer(
      '<pre><code>{"name": "pulse", "ok": true, "count": 3}</code></pre>',
    );
    await highlightSettledCodeBlocks(container);
    const code = container.querySelector('code') as HTMLElement;
    expect(code.dataset.highlighted).toBe('true');
    expect(code.textContent).toBe('{"name": "pulse", "ok": true, "count": 3}');
  });

  it('leaves empty blocks alone', async () => {
    const container = makeContainer('<pre><code class="language-bash">   </code></pre>');
    await highlightSettledCodeBlocks(container);
    const code = container.querySelector('code') as HTMLElement;
    expect(code.dataset.highlighted).toBeUndefined();
  });
});
