export const SCROLL_TO_TOP_BUTTON_THRESHOLD = 400;
export const SCROLL_TO_TOP_BUTTON_ARIA_LABEL = 'Scroll to top';
export const SCROLL_TO_TOP_BUTTON_BASE_CLASS =
  'fixed bottom-6 right-6 z-30 flex h-9 w-9 items-center justify-center rounded-full bg-surface text-base-content shadow-sm transition-all duration-200 hover:bg-surface-hover border border-border md:bottom-6 bottom-20';
export const SCROLL_TO_TOP_BUTTON_VISIBLE_CLASS = 'opacity-100 translate-y-0';
export const SCROLL_TO_TOP_BUTTON_HIDDEN_CLASS = 'opacity-0 translate-y-2 pointer-events-none';

export function findNearestScrollableAncestor(
  sentinel: HTMLElement | undefined,
  getStyle: (element: Element) => CSSStyleDeclaration = getComputedStyle,
): HTMLElement | null {
  let element: HTMLElement | null = sentinel ?? null;
  while (element) {
    const { overflowY } = getStyle(element);
    if ((overflowY === 'auto' || overflowY === 'scroll') && element.scrollHeight > element.clientHeight) {
      return element;
    }
    element = element.parentElement;
  }
  return null;
}

export function isScrollToTopButtonVisible(scrollTop: number): boolean {
  return scrollTop > SCROLL_TO_TOP_BUTTON_THRESHOLD;
}

export function getScrollToTopButtonClass(visible: boolean): string {
  return `${SCROLL_TO_TOP_BUTTON_BASE_CLASS} ${
    visible ? SCROLL_TO_TOP_BUTTON_VISIBLE_CLASS : SCROLL_TO_TOP_BUTTON_HIDDEN_CLASS
  }`;
}
