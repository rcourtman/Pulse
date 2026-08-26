import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  bindWindowedPageScrollEvents,
  findWindowedPageScrollContainer,
  wheelDeltaInPixels,
} from './windowedPageScroll';
import windowedPageScrollSource from './windowedPageScroll.ts?raw';

afterEach(() => {
  document.body.replaceChildren();
  vi.restoreAllMocks();
});

describe('windowedPageScroll', () => {
  it('binds native scroll, wheel prewarming, and resize without intercepting touch', () => {
    const addEventListenerSpy = vi.spyOn(window, 'addEventListener');
    const removeEventListenerSpy = vi.spyOn(window, 'removeEventListener');
    const onScroll = vi.fn();
    const onWheel = vi.fn();
    const onResize = vi.fn();

    const dispose = bindWindowedPageScrollEvents({
      scrollTarget: window,
      onScroll,
      onWheel,
      onResize,
    });

    expect(addEventListenerSpy).toHaveBeenCalledWith('scroll', onScroll, { passive: true });
    expect(addEventListenerSpy).toHaveBeenCalledWith('wheel', onWheel, { passive: true });
    expect(addEventListenerSpy).toHaveBeenCalledWith('resize', onResize);
    expect(windowedPageScrollSource).not.toContain("addEventListener('touch");

    window.dispatchEvent(new Event('scroll'));
    window.dispatchEvent(new WheelEvent('wheel', { deltaY: 24 }));
    window.dispatchEvent(new Event('resize'));
    expect(onScroll).toHaveBeenCalledTimes(1);
    expect(onWheel).toHaveBeenCalledTimes(1);
    expect(onResize).toHaveBeenCalledTimes(1);

    dispose();
    expect(removeEventListenerSpy).toHaveBeenCalledWith('scroll', onScroll);
    expect(removeEventListenerSpy).toHaveBeenCalledWith('wheel', onWheel);
    expect(removeEventListenerSpy).toHaveBeenCalledWith('resize', onResize);
  });

  it('normalizes pixel, line, and page wheel deltas once for every consumer', () => {
    expect(wheelDeltaInPixels(new WheelEvent('wheel', { deltaY: 3 }), 800)).toBe(3);
    expect(wheelDeltaInPixels(new WheelEvent('wheel', { deltaMode: 1, deltaY: 3 }), 800)).toBe(48);
    expect(wheelDeltaInPixels(new WheelEvent('wheel', { deltaMode: 2, deltaY: 3 }), 800)).toBe(
      2_400,
    );
  });

  it('finds the single vertical page-scroller ancestor', () => {
    const scrollContainer = document.createElement('div');
    scrollContainer.style.overflowY = 'scroll';
    const child = document.createElement('div');
    scrollContainer.append(child);
    document.body.append(scrollContainer);

    expect(findWindowedPageScrollContainer(child)).toBe(scrollContainer);
  });
});
