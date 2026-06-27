import { describe, it, expect } from 'vitest';
import { presentCapacityForecast } from '../patrolCapacityForecastPresentation';

describe('presentCapacityForecast', () => {
  it('returns null when no forecast is present', () => {
    expect(presentCapacityForecast(undefined)).toBeNull();
    expect(presentCapacityForecast(null)).toBeNull();
  });

  it('renders a critical urgency line when filling within days', () => {
    const out = presentCapacityForecast({ current_pct: 92, daily_change: 3, days_to_full: 3 });
    expect(out).not.toBeNull();
    expect(out!.direction).toBe('Filling up');
    expect(out!.detail).toBe('3 days to full at +3.0%/day');
    expect(out!.current).toBe('92% used');
    expect(out!.tone).toBe('critical');
  });

  it('renders a warning urgency line when filling within a month', () => {
    const out = presentCapacityForecast({ current_pct: 86, daily_change: 1.2, days_to_full: 11 });
    expect(out!.tone).toBe('warning');
    expect(out!.detail).toBe('~1 week to full at +1.2%/day');
  });

  it('renders info tone when fill is far out', () => {
    const out = presentCapacityForecast({ current_pct: 55, daily_change: 0.2, days_to_full: 200 });
    expect(out!.tone).toBe('info');
    expect(out!.direction).toBe('Filling up');
  });

  it('renders declining when usage is falling', () => {
    const out = presentCapacityForecast({ current_pct: 40, daily_change: -0.5, days_to_full: -1 });
    expect(out!.direction).toBe('Declining');
    expect(out!.tone).toBe('info');
    expect(out!.detail).toContain('falling');
  });

  it('renders stable when no meaningful trend', () => {
    const out = presentCapacityForecast({ current_pct: 60, daily_change: 0, days_to_full: -1 });
    expect(out!.direction).toBe('Stable');
    expect(out!.current).toBe('60% used');
  });
});
