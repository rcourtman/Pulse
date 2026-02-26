import { describe, expect, it, vi } from 'vitest';

import * as utils from '@/components/AI/aiChatUtils';
import { marked } from 'marked';
import type { ModelInfo } from '@/types/ai';

describe('aiChatUtils', () => {
  describe('getProviderFromModelId', () => {
    it('uses explicit provider prefix when present', () => {
      expect(utils.getProviderFromModelId('openai:gpt-4o')).toBe('openai');
      expect(utils.getProviderFromModelId('anthropic:claude-3-5-sonnet')).toBe('anthropic');
    });

    it('detects provider from known model naming', () => {
      expect(utils.getProviderFromModelId('claude-3-5-sonnet')).toBe('anthropic');
      expect(utils.getProviderFromModelId('o3-mini')).toBe('openai');
      expect(utils.getProviderFromModelId('deepseek-r1')).toBe('deepseek');
      expect(utils.getProviderFromModelId('llama3.1')).toBe('ollama');
    });

    it('routes vendor-prefixed OpenRouter model names to openai', () => {
      expect(utils.getProviderFromModelId('google/gemini-2.5-flash-lite-preview-09-2025')).toBe('openai');
      expect(utils.getProviderFromModelId('meta-llama/llama-3-70b-instruct')).toBe('openai');
      expect(utils.getProviderFromModelId('anthropic/claude-3-opus')).toBe('openai');
      // OpenRouter free-tier suffix with colon
      expect(utils.getProviderFromModelId('google/gemini-2.0-flash:free')).toBe('openai');
    });

    it('does not misinterpret colons in model names as provider prefix', () => {
      // Ollama convention: "model:tag"
      expect(utils.getProviderFromModelId('llama3.2:latest')).toBe('ollama');
      expect(utils.getProviderFromModelId('deepseek:latest')).toBe('deepseek');
    });

    it('explicit provider prefix wins over slash detection', () => {
      expect(utils.getProviderFromModelId('ollama:hf.co/some/model')).toBe('ollama');
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
      expect(Array.from(grouped.keys()).sort()).toEqual(['anthropic', 'deepseek', 'ollama', 'openai']);
      expect(grouped.get('openai')?.map((m) => m.id)).toEqual(['openai:gpt-4o']);
      expect(grouped.get('anthropic')?.map((m) => m.id)).toEqual(['claude-3-5-sonnet']);
    });
  });

  describe('sanitizeThinking', () => {
    it('replaces raw tcp timeout and connection errors', () => {
      const input = [
        'write tcp 192.168.0.123:7655->192.168.0.134:58004: i/o timeout',
        'read tcp 10.0.0.1: i/o timeout',
        'dial tcp 127.0.0.1: connection refused',
        'failed to send command: write tcp 192.168.0.123:7655->192.168.0.134:58004: i/o timeout',
      ].join('\n');

      const output = utils.sanitizeThinking(input);
      expect(output).toContain('connection timed out');
      expect(output).toContain('connection refused');
      expect(output).toContain('failed to send command: connection error');
      expect(output).not.toContain('192.168.0.123');
      expect(output).not.toContain('10.0.0.1');
      expect(output).not.toContain('127.0.0.1');
    });
  });

  describe('getGuestName', () => {
    it('prefers guestName then falls back to name', () => {
      expect(utils.getGuestName(undefined)).toBeUndefined();
      expect(utils.getGuestName({})).toBeUndefined();
      expect(utils.getGuestName({ name: 'vm-101' })).toBe('vm-101');
      expect(utils.getGuestName({ guestName: 'container-202', name: 'ignored' })).toBe('container-202');
    });
  });

  describe('renderMarkdown', () => {
    it('sanitizes HTML and forces safe link attributes', () => {
      const output = utils.renderMarkdown(
        [
          'Hello',
          '',
          '[link](https://example.com)',
          '',
          '<script>alert("xss")</script>',
        ].join('\n')
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
  });
});
