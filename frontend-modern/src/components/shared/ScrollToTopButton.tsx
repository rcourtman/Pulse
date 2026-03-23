import ArrowUpIcon from 'lucide-solid/icons/arrow-up';
import {
  SCROLL_TO_TOP_BUTTON_ARIA_LABEL,
  getScrollToTopButtonClass,
} from '@/components/shared/scrollToTopButtonModel';
import { useScrollToTopButtonState } from '@/components/shared/useScrollToTopButtonState';

export function ScrollToTopButton() {
  const state = useScrollToTopButtonState();

  return (
    <>
      <div ref={state.setSentinelRef} class="hidden" />
      <button
        type="button"
        onClick={state.scrollToTop}
        class={getScrollToTopButtonClass(state.visible())}
        aria-label={SCROLL_TO_TOP_BUTTON_ARIA_LABEL}
      >
        <ArrowUpIcon class="h-4 w-4" />
      </button>
    </>
  );
}
