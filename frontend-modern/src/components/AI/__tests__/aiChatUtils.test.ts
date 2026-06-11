import { describe, expect, it, vi } from 'vitest';

import * as utils from '@/components/AI/aiChatUtils';
import { marked } from 'marked';
import type { ModelInfo } from '@/types/ai';

describe('aiChatUtils', () => {
  describe('getProviderFromModelId', () => {
    it('uses explicit provider prefix when present', () => {
      expect(utils.getProviderFromModelId('openai:gpt-4o')).toBe('openai');
      expect(utils.getProviderFromModelId('openrouter:openai/gpt-4o-mini')).toBe('openrouter');
      expect(utils.getProviderFromModelId('anthropic:claude-3-5-sonnet')).toBe('anthropic');
    });

    it('detects provider from known model naming', () => {
      expect(utils.getProviderFromModelId('claude-3-5-sonnet')).toBe('anthropic');
      expect(utils.getProviderFromModelId('o3-mini')).toBe('openai');
      expect(utils.getProviderFromModelId('anthropic/claude-sonnet-4.5')).toBe('openrouter');
      expect(utils.getProviderFromModelId('deepseek-r1')).toBe('deepseek');
      expect(utils.getProviderFromModelId('llama3.1')).toBe('ollama');
    });

    it('handles odd strings without a provider prefix', () => {
      // colon at index 0 should not be treated as a provider prefix
      expect(utils.getProviderFromModelId(':gpt-4o')).toBe('openai');
      expect(utils.getProviderFromModelId(':unknown-model')).toBe('ollama');
    });
  });

  describe('groupModelsByProvider', () => {
    it('groups models by detected provider', () => {
      const models: ModelInfo[] = [
        { id: 'openai:gpt-4o', name: 'GPT-4o' },
        { id: 'claude-3-5-sonnet', name: 'Claude 3.5 Sonnet' },
        { id: 'deepseek-r1', name: 'DeepSeek R1' },
        { id: 'ollama:llama3.1', name: 'Llama 3.1' },
      ];

      const grouped = utils.groupModelsByProvider(models);
      expect(Array.from(grouped.keys()).sort()).toEqual([
        'anthropic',
        'deepseek',
        'ollama',
        'openai',
      ]);
      expect(grouped.get('openai')?.map((m) => m.id)).toEqual(['openai:gpt-4o']);
      expect(grouped.get('anthropic')?.map((m) => m.id)).toEqual(['claude-3-5-sonnet']);
    });

    it('prefers the server-supplied provider over the id heuristic (#1320)', () => {
      const models: ModelInfo[] = [
        // The id heuristic would MIS-detect these (gpt -> openai, claude ->
        // anthropic); the server-supplied provider must win.
        { id: 'gpt-oss-20b', name: 'GPT-OSS 20B', provider: 'ollama' },
        { id: 'my-claude-clone', name: 'Clone', provider: 'ollama' },
      ];

      const grouped = utils.groupModelsByProvider(models);
      expect(grouped.get('ollama')?.map((m) => m.id).sort()).toEqual([
        'gpt-oss-20b',
        'my-claude-clone',
      ]);
      expect(grouped.has('openai')).toBe(false);
      expect(grouped.has('anthropic')).toBe(false);
    });
  });

  describe('sanitizeThinking', () => {
    it('replaces raw tcp timeout and connection errors', () => {
      const input = [
        'write tcp 192.0.2.10:7655->198.51.100.20:58004: i/o timeout',
        'read tcp 10.0.0.1: i/o timeout',
        'dial tcp 127.0.0.1: connection refused',
        'failed to send command: write tcp 192.0.2.10:7655->198.51.100.20:58004: i/o timeout',
      ].join('\n');

      const output = utils.sanitizeThinking(input);
      expect(output).toContain('connection timed out');
      expect(output).toContain('connection refused');
      expect(output).toContain('failed to send command: connection error');
      expect(output).not.toContain('192.0.2.10');
      expect(output).not.toContain('10.0.0.1');
      expect(output).not.toContain('127.0.0.1');
    });
  });

  describe('getGuestName', () => {
    it('prefers guestName then falls back to name', () => {
      expect(utils.getGuestName(undefined)).toBeUndefined();
      expect(utils.getGuestName({})).toBeUndefined();
      expect(utils.getGuestName({ name: 'vm-101' })).toBe('vm-101');
      expect(utils.getGuestName({ guestName: 'container-202', name: 'ignored' })).toBe(
        'container-202',
      );
    });
  });

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
      const output = utils.renderMarkdown(
        '<div class="fixed inset-0 z-50 bg-black">overlay</div>',
      );
      expect(output).not.toContain('class="fixed');
      expect(output).not.toContain('inset-0');
    });

    // The one class carve-out: marked's fence-language hint on <code>, so
    // the lazy syntax highlighter can pick a grammar. Pattern-pinned.
    it('keeps the language-x class on fenced code blocks only', () => {
      const output = utils.renderMarkdown(['```bash', 'df -h', '```'].join('\n'));
      expect(output).toContain('language-bash');

      const hostile = utils.renderMarkdown(
        '<code class="fixed inset-0 z-50">x</code><code class="language-bash extra">y</code>',
      );
      expect(hostile).not.toContain('fixed');
      // Multi-class values fail the ^language-x$ pattern and are dropped whole.
      expect(hostile).not.toContain('language-bash extra');

      const nonCode = utils.renderMarkdown('<div class="language-bash">z</div>');
      expect(nonCode).not.toContain('class=');
    });

    // Regression: real http/https links still render and pick up the safe
    // target/rel attributes from the afterSanitizeAttributes hook.
    it('preserves https links and applies target/rel', () => {
      const output = utils.renderMarkdown('[ok](https://example.com)');
      expect(output).toContain('href="https://example.com"');
      expect(output).toContain('target="_blank"');
      expect(output).toContain('rel="noopener noreferrer"');
    });
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
