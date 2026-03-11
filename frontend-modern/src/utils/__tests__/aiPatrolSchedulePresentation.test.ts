import { describe, expect, it } from 'vitest';
import {
  buildPatrolScheduleOptions,
  getPatrolScheduleLabel,
  PATROL_SCHEDULE_PRESETS,
} from '@/utils/aiPatrolSchedulePresentation';

describe('aiPatrolSchedulePresentation', () => {
  it('returns canonical patrol schedule presets', () => {
    expect(PATROL_SCHEDULE_PRESETS).toContainEqual({ value: 0, label: 'Disabled' });
    expect(PATROL_SCHEDULE_PRESETS).toContainEqual({ value: 360, label: '6 hours' });
  });

  it('formats patrol schedule labels canonically', () => {
    expect(getPatrolScheduleLabel(60)).toBe('1 hour');
    expect(getPatrolScheduleLabel(17)).toBe('17 min');
  });

  it('builds canonical schedule options including custom current values', () => {
    expect(buildPatrolScheduleOptions(360)).toEqual([...PATROL_SCHEDULE_PRESETS]);
    expect(buildPatrolScheduleOptions(17)).toContainEqual({ value: 17, label: '17 min' });
  });
});
