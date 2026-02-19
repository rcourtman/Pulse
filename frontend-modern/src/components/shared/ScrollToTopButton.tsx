import { createSignal, onCleanup, onMount } from 'solid-js';
import ArrowUpIcon from 'lucide-solid/icons/arrow-up';

const SCROLL_THRESHOLD = 400;

/**
 * Floating "back to top" button that appears after scrolling down.
 * Auto-discovers its nearest scrollable ancestor so it works regardless
 * of whether the page uses window scroll or a scrollable div.
 */
export function ScrollToTopButton() {
  const [visible, setVisible] = createSignal(false);
  let sentinel: HTMLDivElement | undefined;
  let scroller: HTMLElement | null = null;

  const findScroller = (): HTMLElement | null => {
    let el: HTMLElement | null = sentinel ?? null;
    while (el) {
      const { overflowY } = getComputedStyle(el);
      if ((overflowY === 'auto' || overflowY === 'scroll') && el.scrollHeight > el.clientHeight) {
        return el;
      }
      el = el.parentElement;
    }
    return null;
  };

  const onScroll = () => {
    if (!scroller) return;
    setVisible(scroller.scrollTop > SCROLL_THRESHOLD);
  };

  const scrollToTop = () => {
    scroller?.scrollTo({ top: 0, behavior: 'smooth' });
  };

  onMount(() => {
    scroller = findScroller();
    if (scroller) {
      scroller.addEventListener('scroll', onScroll, { passive: true });
      onScroll();
    }
  });

  onCleanup(() => {
    if (scroller) {
      scroller.removeEventListener('scroll', onScroll);
    }
  });

  return (
    <>
      <div ref={sentinel} class="hidden" />
      <button
        type="button"
        onClick={scrollToTop}
        class={`fixed bottom-6 right-6 z-30 flex h-9 w-9 items-center justify-center rounded-full bg-slate-800 text-white shadow-sm transition-all duration-200 hover:bg-slate-700 dark:bg-slate-600 dark:hover:bg-slate-500 md:bottom-6 bottom-20 ${
          visible() ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-2 pointer-events-none'
        }`}
        aria-label="Scroll to top"
      >
        <ArrowUpIcon class="h-4 w-4" />
      </button>
    </>
  );
}
