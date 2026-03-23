import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ScrollToTopButton } from './ScrollToTopButton';
import scrollToTopButtonSource from './ScrollToTopButton.tsx?raw';
import scrollToTopButtonModelSource from './scrollToTopButtonModel.ts?raw';
import scrollToTopButtonStateSource from './useScrollToTopButtonState.ts?raw';

describe('ScrollToTopButton', () => {
  afterEach(() => {
    cleanup();
  });

  it('keeps scroll-to-top button on shell, runtime, and model owners', () => {
    expect(scrollToTopButtonSource).toContain('useScrollToTopButtonState');
    expect(scrollToTopButtonSource).toContain('SCROLL_TO_TOP_BUTTON_ARIA_LABEL');
    expect(scrollToTopButtonSource).toContain('getScrollToTopButtonClass');
    expect(scrollToTopButtonSource).not.toContain('createSignal');
    expect(scrollToTopButtonSource).not.toContain('onMount');
    expect(scrollToTopButtonSource).not.toContain('scrollHeight');
    expect(scrollToTopButtonSource).not.toContain('SCROLL_THRESHOLD');

    expect(scrollToTopButtonStateSource).toContain('export function useScrollToTopButtonState');
    expect(scrollToTopButtonStateSource).toContain('createSignal');
    expect(scrollToTopButtonStateSource).toContain('onMount');
    expect(scrollToTopButtonStateSource).toContain('addEventListener');
    expect(scrollToTopButtonStateSource).toContain('scrollTo({ top: 0, behavior: \'smooth\' })');
    expect(scrollToTopButtonStateSource).toContain('findNearestScrollableAncestor');

    expect(scrollToTopButtonModelSource).toContain('SCROLL_TO_TOP_BUTTON_THRESHOLD');
    expect(scrollToTopButtonModelSource).toContain('SCROLL_TO_TOP_BUTTON_ARIA_LABEL');
    expect(scrollToTopButtonModelSource).toContain('findNearestScrollableAncestor');
    expect(scrollToTopButtonModelSource).toContain('isScrollToTopButtonVisible');
    expect(scrollToTopButtonModelSource).toContain('getScrollToTopButtonClass');
  });

  it('shows after scrolling past the threshold and scrolls the container to top', async () => {
    const scroller = document.createElement('div');
    scroller.style.overflowY = 'auto';
    Object.defineProperty(scroller, 'scrollHeight', { value: 1000, configurable: true });
    Object.defineProperty(scroller, 'clientHeight', { value: 200, configurable: true });
    Object.defineProperty(scroller, 'scrollTop', { value: 0, writable: true, configurable: true });
    scroller.scrollTo = vi.fn();
    document.body.appendChild(scroller);

    render(() => <ScrollToTopButton />, { container: scroller });

    const button = screen.getByRole('button', { name: 'Scroll to top' });
    expect(button.className).toContain('pointer-events-none');

    scroller.scrollTop = 500;
    fireEvent.scroll(scroller);
    expect(button.className).toContain('opacity-100');

    fireEvent.click(button);
    expect(scroller.scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'smooth' });
  });
});
