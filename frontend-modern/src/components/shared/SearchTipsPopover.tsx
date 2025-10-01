import { Component, Show, createEffect, createSignal, onCleanup } from 'solid-js';

interface SearchTip {
  code: string;
  description: string;
}

interface SearchTipsPopoverProps {
  buttonLabel?: string;
  title?: string;
  intro?: string;
  tips: SearchTip[];
  footerText?: string;
  footerHighlight?: string;
  popoverId?: string;
  align?: 'left' | 'right';
  class?: string;
}

export const SearchTipsPopover: Component<SearchTipsPopoverProps> = (props) => {
  const [open, setOpen] = createSignal(false);
  let popoverRef: HTMLDivElement | undefined;
  let triggerRef: HTMLButtonElement | undefined;

  const close = () => setOpen(false);

  createEffect(() => {
    if (!open()) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      const insidePopover = popoverRef?.contains(target) ?? false;
      const onTrigger = triggerRef?.contains(target) ?? false;

      if (!insidePopover && !onTrigger) {
        close();
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        close();
      }
    };

    window.addEventListener('pointerdown', handlePointerDown);
    window.addEventListener('keydown', handleKeyDown);

    onCleanup(() => {
      window.removeEventListener('pointerdown', handlePointerDown);
      window.removeEventListener('keydown', handleKeyDown);
    });
  });

  const popoverPositionClass = props.align === 'left' ? 'left-0' : 'right-0';
  const popoverId = props.popoverId ?? 'search-tips-popover';

  return (
    <div class={`relative ${props.class ?? ''}`}>
      <button
        ref={(el) => (triggerRef = el)}
        type="button"
        class="rounded-md border border-gray-200 px-2.5 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500/40 focus:ring-offset-1 focus:ring-offset-white dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700 dark:focus:ring-blue-400/40 dark:focus:ring-offset-gray-900"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open()}
        aria-controls={popoverId}
      >
        {props.buttonLabel ?? 'Search tips'}
      </button>

      <Show when={open()}>
        <div
          ref={(el) => (popoverRef = el)}
          id={popoverId}
          role="dialog"
          aria-label={props.title ?? 'Search tips'}
          class={`absolute ${popoverPositionClass} z-50 mt-2 w-72 overflow-hidden rounded-lg border border-gray-200 bg-white text-left shadow-xl dark:border-gray-600 dark:bg-gray-800`}
        >
          <div class="flex items-center justify-between border-b border-gray-100 px-3 py-2 dark:border-gray-700">
            <span class="text-sm font-semibold text-gray-900 dark:text-gray-100">
              {props.title ?? 'Search tips'}
            </span>
            <button
              type="button"
              class="rounded p-1 text-gray-400 transition-colors hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300"
              onClick={close}
              aria-label="Close search tips"
            >
              <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div class="px-3 py-3 text-xs text-gray-600 dark:text-gray-300">
            <Show when={props.intro}>
              <p class="mb-3 text-[11px] uppercase tracking-wide text-gray-400 dark:text-gray-500">
                {props.intro}
              </p>
            </Show>
            <div class="space-y-2">
              {props.tips.map((tip) => (
                <div class="flex items-start gap-2">
                  <code class="rounded bg-gray-100 px-2 py-0.5 font-mono text-[11px] text-gray-700 dark:bg-gray-700 dark:text-gray-100">
                    {tip.code}
                  </code>
                  <span class="text-[12px] leading-snug text-gray-500 dark:text-gray-400">
                    {tip.description}
                  </span>
                </div>
              ))}
            </div>
            <Show when={props.footerText || props.footerHighlight}>
              <div class="mt-3 rounded-md bg-blue-50 px-3 py-2 text-[11px] text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                <Show when={props.footerHighlight}>
                  <code class="mr-1 rounded bg-blue-100 px-1 py-0.5 font-mono text-[11px] text-blue-700 dark:bg-blue-800/60 dark:text-blue-100">
                    {props.footerHighlight}
                  </code>
                </Show>
                {props.footerText}
              </div>
            </Show>
          </div>
        </div>
      </Show>
    </div>
  );
};

export default SearchTipsPopover;
