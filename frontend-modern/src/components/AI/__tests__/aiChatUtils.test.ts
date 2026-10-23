import { describe, expect, it, vi } from 'vitest';

import * as utils from '@/components/AI/aiChatUtils';
import DOMPurify from 'dompurify';
import { marked } from 'marked';

describe('aiChatUtils', () => {
  describe('renderMarkdown', () => {
    it('sanitizes HTML and forces safe link attributes', () => {
      const output = utils.renderMarkdown(
        ['Hello', '', '[link](https://example.com)', '', '<script>alert("xss")</script>'].join(
          '\n',
        ),
      );

      expect(output).toContain('Hello');
      expect(output).toContain('<a');
      expect(output).toContain('href="https://example.com"');
      expect(output).toContain('target="_blank"');
      expect(output).toContain('rel="noopener noreferrer"');
      expect(output).not.toContain('<script');
      expect(output).not.toContain('alert("xss")');
    });

    it('contains table overflow without trusting model layout attributes', () => {
      const output = utils.renderMarkdown(
        '<table class="fixed inset-0" style="position:fixed" onclick="alert(1)"><tr><th>Metric</th><th>Value</th></tr><tr><td>Memory</td><td>88%</td></tr></table>',
      );
      const root = document.createElement('div');
      root.innerHTML = output;
      const table = root.querySelector('table');
      const region = table?.parentElement;
      expect(region?.getAttribute('role')).toBe('region');
      expect(region?.getAttribute('aria-label')).toBe('Scrollable table');
      expect(region?.tabIndex).toBe(0);
      expect(region?.classList.contains('overflow-x-auto')).toBe(true);
      expect(table?.textContent).toBe('MetricValueMemory88%');
      expect(table?.hasAttribute('class')).toBe(false);
      expect(table?.hasAttribute('style')).toBe(false);
      expect(table?.hasAttribute('onclick')).toBe(false);
    });

    it('escapes HTML entities if markdown parsing fails', () => {
      const spy = vi.spyOn(marked, 'parse').mockImplementation(() => {
        throw new Error('boom');
      });

      const output = utils.renderMarkdown(`a&b <c> "d" 'e'`);
      expect(output).toBe('a&amp;b &lt;c&gt; &quot;d&quot; &#39;e&#39;');

      spy.mockRestore();
    });

    // Regression: drop `javascript:` href from LLM-supplied markdown links.
    // Relies on the explicit ALLOWED_URI_REGEXP pin so the behaviour survives
    // future DOMPurify upgrades that might relax internal scheme filtering.
    it('strips javascript: hrefs from links', () => {
      const output = utils.renderMarkdown('[click](javascript:alert(1))');
      expect(output).not.toContain('javascript:');
      expect(output).not.toContain('alert(1)');
    });

    // Regression: data: URIs are not in the allowlist either; including SVG
    // data URIs which some browsers will execute inline script from.
    it('strips data: hrefs from links', () => {
      const output = utils.renderMarkdown('[svg](data:image/svg+xml,<svg/>)');
      expect(output).not.toContain('data:');
    });

    // Regression: the LLM has no legitimate reason to apply arbitrary CSS
    // classes — used to be ALLOWED_ATTR; allowing them opens a UI-redress
    // surface (overlay attacks, hidden text, etc.).
    it('strips class attribute from LLM-supplied markup', () => {
      const output = utils.renderMarkdown('<div class="fixed inset-0 z-50 bg-black">overlay</div>');
      expect(output).not.toContain('class="fixed');
      expect(output).not.toContain('inset-0');
    });

    // No class survives, fence-language hints included: the carve-out that
    // kept them served the retired Assistant code highlighter.
    it('strips the fence-language class from code blocks too', () => {
      const output = utils.renderMarkdown(['```bash', 'df -h', '```'].join('\n'));
      expect(output).toContain('df -h');
      expect(output).not.toContain('class=');

      const hostile = utils.renderMarkdown(
        '<code class="fixed inset-0 z-50">x</code><code class="language-bash extra">y</code>',
      );
      expect(hostile).not.toContain('fixed');
      expect(hostile).not.toContain('language-bash');
    });

    // Regression: real http/https links still render and pick up the safe
    // target/rel attributes from the afterSanitizeAttributes hook.
    it('preserves https links and applies target/rel', () => {
      const output = utils.renderMarkdown('[ok](https://example.com)');
      expect(output).toContain('href="https://example.com"');
      expect(output).toContain('target="_blank"');
      expect(output).toContain('rel="noopener noreferrer"');
    });

    it('removes executable descendants from an advisory-shaped detached-subtree payload', () => {
      const output = utils.renderMarkdown(
        [
          '<footer>',
          '  <img src="x" onload="alert(1)" onerror="alert(2)">',
          '</footer>',
          '<a href="https://example.com">safe link</a>',
        ].join('\n'),
      );
      const template = document.createElement('template');
      template.innerHTML = output;

      expect(template.content.querySelector('footer, img, [onload], [onerror], script')).toBeNull();
      const link = template.content.querySelector('a');
      expect(link?.getAttribute('href')).toBe('https://example.com');
      expect(link?.getAttribute('target')).toBe('_blank');
      expect(link?.getAttribute('rel')).toBe('noopener noreferrer');
    });

    it.each(['beforeSanitizeElements', 'uponSanitizeElement', 'afterSanitizeAttributes'] as const)(
      'neutralizes a hook-detached IN_PLACE subtree through the %s hook',
      (hookName) => {
        // Configure the same singleton hooks used by renderMarkdown before
        // exercising DOMPurify's patched IN_PLACE hook-detachment path.
        utils.renderMarkdown('```ts\nconst safe = true;\n```');

        const root = document.createElement('div');
        root.innerHTML =
          '<section><img id="detached-tail" onerror="alert(1)"></section><div>safe</div>';
        const detachedTail = root.querySelector<HTMLImageElement>('#detached-tail');
        expect(detachedTail).not.toBeNull();

        const removeSection = (node: Node) => {
          if (node.nodeName === 'SECTION') (node as Element).remove();
        };
        if (hookName === 'beforeSanitizeElements') {
          DOMPurify.addHook('beforeSanitizeElements', removeSection);
        } else if (hookName === 'uponSanitizeElement') {
          DOMPurify.addHook('uponSanitizeElement', removeSection);
        } else {
          DOMPurify.addHook('afterSanitizeAttributes', removeSection);
        }

        try {
          const result = DOMPurify.sanitize(root, {
            ALLOWED_TAGS: ['div', 'section', '#text'],
            IN_PLACE: true,
          });

          expect(result).toBe(root);
          expect(root.innerHTML).toBe('<div>safe</div>');
          expect(detachedTail?.isConnected).toBe(false);
          expect(detachedTail?.getAttribute('onerror')).toBeNull();

          const probe = document.createElement('div');
          probe.innerHTML = `${root.outerHTML}${detachedTail?.outerHTML ?? ''}`;
          expect(probe.querySelector('[onerror], [onload], script')).toBeNull();
        } finally {
          if (hookName === 'beforeSanitizeElements') {
            DOMPurify.removeHook('beforeSanitizeElements', removeSection);
          } else if (hookName === 'uponSanitizeElement') {
            DOMPurify.removeHook('uponSanitizeElement', removeSection);
          } else {
            DOMPurify.removeHook('afterSanitizeAttributes', removeSection);
          }
        }
      },
    );
  });

  describe('Mention ID format contracts', () => {
    it('VM mention IDs follow vm:node:vmid format', () => {
      const id = 'vm:pve1:100';
      expect(id).toMatch(/^vm:[^:]+:\d+$/);
    });

    it('container mention IDs follow lxc:node:vmid format', () => {
      const id = 'lxc:pve1:200';
      expect(id).toMatch(/^lxc:[^:]+:\d+$/);
    });

    it('app-container mention IDs follow app-container:host:providerUid format', () => {
      const id = 'app-container:truenas-main:nextcloud';
      expect(id).toMatch(/^app-container:[^:]+:[^:]+$/);
    });

    it('node mention IDs follow node:instance:name format', () => {
      const id = 'node:pve1/pve:pve1';
      expect(id).toMatch(/^node:[^:]+:[^:]+$/);
    });

    it('agent mention IDs follow agent:id format', () => {
      const id = 'agent:agent-123';
      expect(id).toMatch(/^agent:[^:]+$/);
    });
  });
});
