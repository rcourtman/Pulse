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
    expect(AI_COST_EMPTY_STATE).toBe('Usage data will appear here once activity is recorded.');
    expect(AI_COST_DAILY_USD_EMPTY_STATE).toBe(
      'Daily cost trend will appear here once activity is recorded.',
    );
    expect(AI_COST_DAILY_TOKEN_EMPTY_STATE).toBe(
      'Daily token trend will appear here once activity is recorded.',
    );
    expect(getAICostLoadingState()).toEqual({
      text: 'Loading usage data…',
    });
  });
});
