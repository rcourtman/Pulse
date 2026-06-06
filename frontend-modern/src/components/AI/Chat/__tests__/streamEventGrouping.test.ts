import { describe, it, expect } from 'vitest';
import { groupStreamEventsForDisplay } from '../streamEventGrouping';
import type { StreamDisplayEvent } from '../types';

const content = (text: string): StreamDisplayEvent => ({ type: 'content', content: text });
const thinking = (text: string): StreamDisplayEvent => ({ type: 'thinking', thinking: text });

describe('groupStreamEventsForDisplay', () => {
  it('merges consecutive content into one block', () => {
    const grouped = groupStreamEventsForDisplay([content('Hello '), content('world')]);
    expect(grouped).toHaveLength(1);
    expect(grouped[0]).toMatchObject({ type: 'content', content: 'Hello world' });
  });

  it('keeps one content block when reasoning interleaves with the answer', () => {
    // Reasoning models via OpenRouter interleave reasoning + answer tokens.
    // Each content delta must NOT become its own block, or whitespace and
    // markdown structure are destroyed.
    const events: StreamDisplayEvent[] = [
      thinking('Let me '),
      content("Here's "),
      thinking('check the '),
      content('the table:\n\n'),
      thinking('numbers.'),
      content('| Metric | Value |\n'),
      content('| --- | --- |\n'),
      content('| CPU | 0.1% |'),
    ];
    const grouped = groupStreamEventsForDisplay(events);

    const contentBlocks = grouped.filter((e) => e.type === 'content');
    const thinkingBlocks = grouped.filter((e) => e.type === 'thinking');
    expect(contentBlocks).toHaveLength(1);
    expect(thinkingBlocks).toHaveLength(1);

    // The answer is one coherent markdown document with whitespace intact.
    expect(contentBlocks[0].content).toBe(
      "Here's the table:\n\n| Metric | Value |\n| --- | --- |\n| CPU | 0.1% |",
    );
    expect(thinkingBlocks[0].thinking).toBe('Let me check the numbers.');
    // Reasoning arrived first, so the thinking block leads.
    expect(grouped[0].type).toBe('thinking');
  });

  it('keeps content separated across a tool boundary so order is preserved', () => {
    const tool: StreamDisplayEvent = {
      type: 'tool',
      tool: { name: 'get_status', input: '{}', output: 'ok', success: true },
    };
    const grouped = groupStreamEventsForDisplay([
      content('Let me check.'),
      tool,
      content('All healthy.'),
    ]);

    expect(grouped.map((e) => e.type)).toEqual(['content', 'tool', 'content']);
    expect(grouped[0].content).toBe('Let me check.');
    expect(grouped[2].content).toBe('All healthy.');
  });

  it('keeps content separated across a model-switch boundary', () => {
    const modelSwitch: StreamDisplayEvent = {
      type: 'model_switch',
      model: 'openrouter:deepseek/deepseek-v4-pro',
    };
    const grouped = groupStreamEventsForDisplay([
      content('OpenRouter was unavailable.'),
      modelSwitch,
      content('Trying the gateway route now.'),
    ]);

    expect(grouped.map((e) => e.type)).toEqual(['content', 'model_switch', 'content']);
    expect(grouped[0].content).toBe('OpenRouter was unavailable.');
    expect(grouped[2].content).toBe('Trying the gateway route now.');
  });

  it('skips empty deltas', () => {
    const grouped = groupStreamEventsForDisplay([content(''), thinking(''), content('hi')]);
    expect(grouped).toHaveLength(1);
    expect(grouped[0]).toMatchObject({ type: 'content', content: 'hi' });
  });
});
