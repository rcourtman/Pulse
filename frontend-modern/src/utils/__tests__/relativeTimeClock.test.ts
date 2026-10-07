import { createRoot } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { RELATIVE_TIME_TICK_MS, useRelativeTimeNow } from '@/utils/relativeTimeClock';

afterEach(() => {
  vi.useRealTimers();
});

describe('useRelativeTimeNow', () => {
  it('ticks while read inside a component and stops when nothing reads it', () => {
    vi.useFakeTimers({ now: new Date('2026-10-06T10:00:00Z') });

    const { now, dispose } = createRoot((dispose) => ({ now: useRelativeTimeNow(), dispose }));
    expect(now()).toBe(Date.parse('2026-10-06T10:00:00Z'));

    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(now()).toBe(Date.parse('2026-10-06T10:00:30Z'));

    dispose();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('reads the wall clock between ticks, so a late reader never trails it', () => {
    vi.useFakeTimers({ now: new Date('2026-10-06T10:00:00Z') });

    const first = createRoot((dispose) => ({ now: useRelativeTimeNow(), dispose }));
    // A second cell mounts 20s after the clock started, before the next tick.
    vi.setSystemTime(new Date('2026-10-06T10:00:20Z'));
    const second = createRoot((dispose) => ({ now: useRelativeTimeNow(), dispose }));

    expect(second.now()).toBe(Date.parse('2026-10-06T10:00:20Z'));
    expect(first.now()).toBe(second.now());

    first.dispose();
    second.dispose();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('reads the time directly outside a reactive owner', () => {
    vi.useFakeTimers({ now: new Date('2026-10-06T10:00:00Z') });
    const now = useRelativeTimeNow();
    vi.setSystemTime(new Date('2026-10-06T12:00:00Z'));
    expect(now()).toBe(Date.parse('2026-10-06T12:00:00Z'));
    expect(vi.getTimerCount()).toBe(0);
  });
});
