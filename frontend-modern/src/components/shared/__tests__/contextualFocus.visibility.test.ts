import { afterEach, describe, expect, it, vi } from 'vitest';
import { revealElementInViewport } from '../contextualFocus';

const target = (top: number, bottom: number) => {
  const element = document.createElement('h2');
  element.getBoundingClientRect = () => ({ top, bottom }) as DOMRect;
  return element;
};

afterEach(() => vi.restoreAllMocks());

describe('conditional contextual reveal', () => {
  it('does not move a visible heading even when extra result space is requested', () => {
    const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
    expect(
      revealElementInViewport({
        element: target(window.innerHeight - 30, window.innerHeight - 10),
        bottomPadding: 96,
      }),
    ).toBe(false);
    expect(scroll).not.toHaveBeenCalled();
  });

  it('reveals an off-screen heading with room for its first results', () => {
    const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
    expect(
      revealElementInViewport({
        element: target(window.innerHeight + 40, window.innerHeight + 60),
        bottomPadding: 96,
      }),
    ).toBe(true);
    expect(scroll).toHaveBeenCalledWith({ top: window.scrollY + 156, behavior: 'instant' });
  });

  it('uses the clipped scroll-container viewport instead of moving the document', () => {
    const scroller = document.createElement('div');
    scroller.style.overflowY = 'auto';
    Object.defineProperties(scroller, {
      clientHeight: { value: 300 },
      scrollHeight: { value: 1000 },
    });
    scroller.getBoundingClientRect = () => ({ top: 100, bottom: 400 }) as DOMRect;
    scroller.scrollTop = 25;
    scroller.scrollTo = vi.fn();
    const element = target(450, 470);
    scroller.append(element);
    expect(revealElementInViewport({ element })).toBe(true);
    expect(scroller.scrollTo).toHaveBeenCalledWith({ top: 95, behavior: 'instant' });
  });
});
