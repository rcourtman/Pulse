import { describe, expect, it } from 'vitest';
import {
  AI_COST_DAILY_TOKEN_EMPTY_STATE,
  AI_COST_DAILY_USD_EMPTY_STATE,
  AI_COST_EMPTY_STATE,
  getAICostLoadingState,
  getAICostRangeButtonClass,
} from '@/utils/aiCostPresentation';

describe('aiCostPresentation', () => {
  it('returns canonical AI cost range button classes', () => {
    expect(getAICostRangeButtonClass(true)).toContain('inline-flex items-center');
    expect(getAICostRangeButtonClass(true)).toContain('bg-surface');
    expect(getAICostRangeButtonClass(false, true)).toContain('cursor-not-allowed');
  });

  it('exports canonical AI cost empty-state copy', () => {
    expect(AI_COST_EMPTY_STATE).toBe('No usage data yet.');
    expect(AI_COST_DAILY_USD_EMPTY_STATE).toBe('No daily USD trend yet.');
    expect(AI_COST_DAILY_TOKEN_EMPTY_STATE).toBe('No daily token trend yet.');
    expect(getAICostLoadingState()).toEqual({
      text: 'Loading usage…',
    });
  });
});
