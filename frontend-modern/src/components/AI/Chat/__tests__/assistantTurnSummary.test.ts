import { describe, expect, it } from 'vitest';
import {
  formatAssistantTurnDuration,
  formatSessionCostUSD,
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

  it('shows how full the context window was when the done event carries the limit', () => {
    expect(
      getAssistantTurnSummary(
        makeAssistantMessage({
          tokens: { input: 8200, output: 300, contextLimit: 131072 },
        }),
        {
          getModelRouteLabel: () => 'Qwen via OpenRouter',
        },
      ),
    ).toEqual({
      label: 'Last turn: Qwen via OpenRouter · 3s · 8,500 tokens (6% of context)',
      title:
        'Last assistant turn summary: Model: Qwen via OpenRouter. Duration: 3s. Usage: 8,500 total, 8,200 input, 300 output, context 8,200 of 131,072 (6%)',
    });
  });

  it('formats session cost with a sub-cent floor and hides non-costs', () => {
    expect(formatSessionCostUSD(0.1234)).toBe('$0.12');
    expect(formatSessionCostUSD(1.5)).toBe('$1.50');
    expect(formatSessionCostUSD(0.0042)).toBe('<$0.01');
    expect(formatSessionCostUSD(0)).toBe('');
    expect(formatSessionCostUSD(undefined)).toBe('');
  });

  it('appends the estimated session cost when the done event carries it', () => {
    const summary = getAssistantTurnSummary(
      makeAssistantMessage({
        tokens: { input: 500, output: 200, sessionCostUsd: 0.1234 },
      }),
      { getModelRouteLabel: () => 'Qwen via OpenRouter' },
    );
    expect(summary?.label).toBe('Last turn: Qwen via OpenRouter · 3s · 700 tokens · $0.12 session');
    expect(summary?.title).toContain('Estimated session cost: $0.12');
  });

  it('omits session cost when pricing is unknown or free', () => {
    const summary = getAssistantTurnSummary(makeAssistantMessage(), {
      getModelRouteLabel: () => 'Qwen via OpenRouter',
    });
    expect(summary?.label).not.toContain('session');
    expect(summary?.title).not.toContain('session cost');
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
