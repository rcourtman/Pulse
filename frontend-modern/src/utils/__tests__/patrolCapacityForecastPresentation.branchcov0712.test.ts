import { describe, it, expect } from 'vitest';
import type { CapacityForecast } from '@/api/patrol';
import { presentCapacityForecast } from '../patrolCapacityForecastPresentation';

describe('formatDays (exercised via presentCapacityForecast)', () => {
  it("returns '1 day' for an exact single day", () => {
    const out = presentCapacityForecast({
      current_pct: 99,
      daily_change: 0.5,
      days_to_full: 1,
    });
    expect(out).toStrictEqual({
      direction: 'Filling up',
      detail: '1 day to full at +0.50%/day',
      current: '99% used',
      tone: 'critical',
    });
  });

  it("formats '~N weeks' for days in the 14-29 range", () => {
    const out = presentCapacityForecast({
      current_pct: 70,
      daily_change: 1,
      days_to_full: 20,
    });
    expect(out!.detail).toBe('~3 weeks to full at +1.0%/day');
  });

  it('uses weeks rounding at the 14-day boundary', () => {
    const out = presentCapacityForecast({
      current_pct: 65,
      daily_change: 1,
      days_to_full: 14,
    });
    expect(out!.detail).toBe('~2 weeks to full at +1.0%/day');
  });

  it("formats '~1 month' when months round down to 1", () => {
    const out = presentCapacityForecast({
      current_pct: 60,
      daily_change: 0.5,
      days_to_full: 44,
    });
    expect(out!.detail).toBe('~1 month to full at +0.50%/day');
  });

  it("formats '~N months' for months between 2 and 11", () => {
    const out = presentCapacityForecast({
      current_pct: 40,
      daily_change: 0.2,
      days_to_full: 90,
    });
    expect(out!.detail).toBe('~3 months to full at +0.20%/day');
  });

  it("returns 'over a year' when months reach 12 or beyond", () => {
    const out = presentCapacityForecast({
      current_pct: 30,
      daily_change: 0.1,
      days_to_full: 400,
    });
    expect(out!.detail).toBe('over a year to full at +0.10%/day');
    expect(out!.tone).toBe('info');
  });
});

describe('formatRate (exercised via presentCapacityForecast)', () => {
  it("omits the '+' sign and uses two decimals for a zero rate", () => {
    const out = presentCapacityForecast({
      current_pct: 80,
      daily_change: 0,
      days_to_full: 5,
    });
    expect(out!.detail).toBe('5 days to full at 0.00%/day');
  });

  it("uses '+' with two decimals for a small positive fractional rate", () => {
    const out = presentCapacityForecast({
      current_pct: 75,
      daily_change: 0.5,
      days_to_full: 6,
    });
    expect(out!.detail).toBe('6 days to full at +0.50%/day');
  });

  it('uses no sign and one decimal for a rate <= -1 (negative fill projection)', () => {
    const out = presentCapacityForecast({
      current_pct: 50,
      daily_change: -2,
      days_to_full: 10,
    });
    expect(out!.detail).toBe('~1 week to full at -2.0%/day');
  });
});

describe('presentCapacityForecast', () => {
  it('returns null for malformed falsy non-null inputs (defensive !fc guard)', () => {
    expect(presentCapacityForecast(0 as unknown as CapacityForecast)).toBeNull();
    expect(presentCapacityForecast('' as unknown as CapacityForecast)).toBeNull();
  });

  it('defaults daily_change to 0 via ?? when it is null', () => {
    const out = presentCapacityForecast({
      current_pct: 80,
      days_to_full: 5,
      daily_change: null,
    } as unknown as CapacityForecast);
    expect(out!.detail).toBe('5 days to full at 0.00%/day');
    expect(out!.tone).toBe('critical');
  });

  it('defaults daily_change to 0 via ?? when it is undefined', () => {
    const out = presentCapacityForecast({
      current_pct: 80,
      days_to_full: 5,
    } as unknown as CapacityForecast);
    expect(out!.detail).toBe('5 days to full at 0.00%/day');
  });

  it('rounds current_pct to the nearest whole percent (half-up)', () => {
    const out = presentCapacityForecast({
      current_pct: 86.5,
      daily_change: 0,
      days_to_full: -1,
    });
    expect(out!.current).toBe('87% used');
    expect(out!.direction).toBe('Stable');
  });

  it('assigns critical tone at the days<=7 boundary (days=7)', () => {
    const out = presentCapacityForecast({
      current_pct: 90,
      daily_change: 1,
      days_to_full: 7,
    });
    expect(out!.tone).toBe('critical');
    expect(out!.detail).toBe('~1 week to full at +1.0%/day');
  });

  it('assigns warning tone just above the critical boundary (days=8)', () => {
    const out = presentCapacityForecast({
      current_pct: 85,
      daily_change: 1,
      days_to_full: 8,
    });
    expect(out!.tone).toBe('warning');
  });

  it('assigns warning tone at the days<=30 boundary (days=30)', () => {
    const out = presentCapacityForecast({
      current_pct: 70,
      daily_change: 0.5,
      days_to_full: 30,
    });
    expect(out!.tone).toBe('warning');
    expect(out!.detail).toBe('~1 month to full at +0.50%/day');
  });

  it('assigns info tone just above the warning boundary (days=31)', () => {
    const out = presentCapacityForecast({
      current_pct: 65,
      daily_change: 0.5,
      days_to_full: 31,
    });
    expect(out!.tone).toBe('info');
    expect(out!.detail).toBe('~1 month to full at +0.50%/day');
  });

  it('renders Declining just below the -0.1 threshold', () => {
    const out = presentCapacityForecast({
      current_pct: 45,
      daily_change: -0.11,
      days_to_full: -1,
    });
    expect(out!.direction).toBe('Declining');
    expect(out!.detail).toBe('usage falling +0.11%/day');
  });

  it('renders Stable at exactly -0.1 (boundary not < -0.1)', () => {
    const out = presentCapacityForecast({
      current_pct: 45,
      daily_change: -0.1,
      days_to_full: -1,
    });
    expect(out!.direction).toBe('Stable');
    expect(out!.detail).toBe('no clear fill trend');
  });

  it('renders Declining with a one-decimal absolute rate for a large drop', () => {
    const out = presentCapacityForecast({
      current_pct: 20,
      daily_change: -3,
      days_to_full: -1,
    });
    expect(out).toStrictEqual({
      direction: 'Declining',
      detail: 'usage falling +3.0%/day',
      current: '20% used',
      tone: 'info',
    });
  });
});
