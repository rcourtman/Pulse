import { describe, expect, it } from 'vitest';

import { normalizeChatMentionKeyPart, normalizeChatToolName } from '@/utils/chatIdentifiers';

describe('chatIdentifiers', () => {
  it('normalizes mention key parts for dedupe and matching', () => {
    expect(normalizeChatMentionKeyPart('  Agent-01  ')).toBe('agent-01');
    expect(normalizeChatMentionKeyPart('')).toBe('');
    expect(normalizeChatMentionKeyPart(undefined)).toBe('');
  });

  it('strips repeated pulse_ prefixes from tool names', () => {
    expect(normalizeChatToolName('pulse_pulse_get_logs')).toBe('get_logs');
    expect(normalizeChatToolName('pulse_get_logs')).toBe('get_logs');
    expect(normalizeChatToolName('get_logs')).toBe('get_logs');
    expect(normalizeChatToolName('  pulse_get_logs  ')).toBe('get_logs');
  });
});
