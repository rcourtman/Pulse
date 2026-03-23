import { createSignal, onCleanup, onMount } from 'solid-js';
import {
  findNearestScrollableAncestor,
  isScrollToTopButtonVisible,
} from '@/components/shared/scrollToTopButtonModel';

export function useScrollToTopButtonState() {
  const [visible, setVisible] = createSignal(false);
  let sentinelRef: HTMLDivElement | undefined;
  let scroller: HTMLElement | null = null;

  const syncVisibility = () => {
    if (!scroller) {
      return;
    }
    setVisible(isScrollToTopButtonVisible(scroller.scrollTop));
  };

  onMount(() => {
    scroller = findNearestScrollableAncestor(sentinelRef);
    if (!scroller) {
      return;
    }

    scroller.addEventListener('scroll', syncVisibility, { passive: true });
    syncVisibility();
  });

  onCleanup(() => {
    scroller?.removeEventListener('scroll', syncVisibility);
  });

  return {
    scrollToTop: () => {
      scroller?.scrollTo({ top: 0, behavior: 'smooth' });
    },
    setSentinelRef: (element: HTMLDivElement) => {
      sentinelRef = element;
    },
    visible,
  };
}
