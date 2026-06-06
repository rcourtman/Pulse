import { describe, expect, it } from 'vitest';
import { getAssistantAnswerText, getLastAssistantAnswerText } from '../assistantAnswerText';
import type { ChatMessage } from '../types';

const makeMessage = (overrides: Partial<ChatMessage>): ChatMessage => ({
  id: 'message-1',
  role: 'assistant',
  content: '',
  timestamp: new Date('2026-06-06T12:00:00Z'),
  ...overrides,
});

describe('assistantAnswerText', () => {
  it('returns the latest assistant message text without transcript framing', () => {
    const messages = [
      makeMessage({ id: 'assistant-1', content: 'Earlier answer' }),
      makeMessage({ id: 'user-1', role: 'user', content: 'next question' }),
      makeMessage({ id: 'assistant-2', content: 'Latest answer' }),
    ];

    expect(getLastAssistantAnswerText(messages)).toBe('Latest answer');
  });

  it('uses streamed content events when the message body has not been compacted yet', () => {
    const message = makeMessage({
      content: '',
      isStreaming: true,
      streamEvents: [
        { type: 'workflow_status', workflowStatus: { message: 'Checking inventory' } },
        { type: 'content', content: 'Streaming answer part one.' },
        { type: 'content', content: 'Streaming answer part two.' },
      ],
    });

    expect(getAssistantAnswerText(message)).toBe(
      'Streaming answer part one.\n\nStreaming answer part two.',
    );
  });

  it('does not fall back to an older answer when the current assistant message has no text', () => {
    const messages = [
      makeMessage({ id: 'assistant-1', content: 'Earlier answer' }),
      makeMessage({
        id: 'assistant-2',
        content: '',
        streamEvents: [
          {
            type: 'tool',
            tool: {
              name: 'pulse_read',
              input: '{}',
              output: '',
              success: true,
            },
          },
        ],
      }),
    ];

    expect(getLastAssistantAnswerText(messages)).toBe('');
  });

  it('keeps raw tool-call artifacts out of copied assistant answers', () => {
    const message = makeMessage({
      content: 'Visible answer\n\n{"name":"pulse_read","arguments":{"command":"ls"}}',
    });

    expect(getAssistantAnswerText(message)).toBe('Visible answer');
  });
});
