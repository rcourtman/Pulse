import { describe, expect, it } from 'vitest';
import { buildDisk } from '../resourceDetailMappers';

describe('root disk mapping preserves missing evidence', () => {
  it('does not manufacture any counters from an absent aggregate', () => {
    expect(buildDisk()).toMatchObject({
      total: undefined,
      used: undefined,
      free: undefined,
      usage: undefined,
    });
  });
  it('does not turn capacity or percent alone into empty used space', () => {
    expect(buildDisk({ current: 0, total: 100 })).toMatchObject({
      total: 100,
      used: undefined,
      free: undefined,
      usage: undefined,
    });
    expect(buildDisk({ current: 25 })).toMatchObject({
      total: undefined,
      used: undefined,
      free: undefined,
      usage: undefined,
    });
  });
  it('keeps explicit zero distinct from absent used bytes', () => {
    expect(buildDisk({ current: 0, total: 100, used: 0 })).toMatchObject({
      total: 100,
      used: 0,
      free: 100,
      usage: 0,
    });
  });
  it.each([-1, NaN, Infinity, 101])(
    'does not erase unavailable/invalid aggregate percent %s',
    (current) => {
      expect(buildDisk({ current, total: 100, used: 25 }).usage).toBe(-1);
    },
  );
});
