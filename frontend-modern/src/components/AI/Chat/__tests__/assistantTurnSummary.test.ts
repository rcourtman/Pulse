import { describe, expect, it } from 'vitest';
import {
  formatAssistantTurnDuration,
  getAssistantTurnSummary,
} from '../assistantTurnSummary';
import type { ChatMessage } from '../types';

const makeAssistantMessage = (overrides: Partial<ChatMessage> = {}): ChatMessage => ({
  id: 'asst-1',
  role: 'assistant',
  content: 'Done.',
  timestamp: new Date('2026-06-06T12:00:00Z'),
  completedAt: new Date('2026-06-06T12:00:03Z'),
  isStreaming: false,
  model: 'openrouter:qwen/qwen3.7-plus',
  tokens: { input: 500, output: 200 },
  ...overrides,
});

describe('assistantTurnSummary', () => {
  it('formats assistant turn duration for short and longer completed turns', () => {
    expect(
      formatAssistantTurnDuration(
        new Date('2026-06-06T12:00:00Z'),
        new Date('2026-06-06T12:00:00.400Z'),
      ),
    ).toBe('<1s');
    expect(
      formatAssistantTurnDuration(
        new Date('2026-06-06T12:00:00Z'),
        new Date('2026-06-06T12:01:05Z'),
      ),
    ).toBe('1m 5s');
    expect(
      formatAssistantTurnDuration(
        new Date('2026-06-06T12:00:00Z'),
        new Date('2026-06-06T11:59:59Z'),
      ),
    ).toBe('');
  });

  it('summarizes completed assistant route, duration, and billed usage', () => {
    expect(
      getAssistantTurnSummary(makeAssistantMessage(), {
        getModelRouteLabel: () => 'Qwen via OpenRouter',
      }),
    ).toEqual({
      label: 'Last turn: Qwen via OpenRouter · 3s · 700 tokens',
      title:
        'Last assistant turn summary: Model: Qwen via OpenRouter. Duration: 3s. Usage: 700 total, 500 input, 200 output',
    });
  });

  it('keeps route and duration visible when provider usage is missing', () => {
    expect(
      getAssistantTurnSummary(
        makeAssistantMessage({
          tokens: { input: 500, output: 0 },
        }),
        {
          getModelRouteLabel: () => 'DeepSeek direct',
        },
      ),
    ).toEqual({
      label: 'Last turn: DeepSeek direct · 3s',
      title: 'Last assistant turn summary: Model: DeepSeek direct. Duration: 3s',
    });
  });

  it('does not summarize active streaming or non-assistant turns', () => {
    expect(getAssistantTurnSummary(makeAssistantMessage({ isStreaming: true }))).toBeNull();
    expect(
      getAssistantTurnSummary(
        makeAssistantMessage({
          role: 'user',
          content: 'Question',
        }),
      ),
    ).toBeNull();
  });
});
