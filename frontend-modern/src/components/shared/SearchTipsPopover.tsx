import { Component, Show, For } from 'solid-js';
import {
  getSearchTipsPopoverButtonLabel,
  getSearchTipsPopoverId,
  getSearchTipsPopoverPositionClass,
  getSearchTipsPopoverTitle,
  getSearchTipsPopoverTriggerClass,
  getSearchTipsPopoverTriggerVariant,
  shouldSearchTipsPopoverOpenOnHover,
  type SearchTipsPopoverProps,
} from './searchTipsPopoverModel';
import { useSearchTipsPopoverState } from './useSearchTipsPopoverState';

export type { SearchTip, SearchTipsPopoverProps } from './searchTipsPopoverModel';

export const SearchTipsPopover: Component<SearchTipsPopoverProps> = (props) => {
  const triggerVariant = () => getSearchTipsPopoverTriggerVariant(props.triggerVariant);
  const buttonLabel = () => getSearchTipsPopoverButtonLabel(props.buttonLabel);
  const title = () => getSearchTipsPopoverTitle(props.title);
  const popoverId = () => getSearchTipsPopoverId(props.popoverId);
  const positionClass = () => getSearchTipsPopoverPositionClass(props.align);
  const triggerClass = () => getSearchTipsPopoverTriggerClass(triggerVariant());
  const openOnHover = () => shouldSearchTipsPopoverOpenOnHover(props.openOnHover);
  const state = useSearchTipsPopoverState({
    buttonLabel,
    openOnHover,
  });

  return (
    <div
      class={`relative ${props.class ?? ''}`}
      onMouseEnter={openOnHover() ? state.handleMouseEnter : undefined}
      onMouseLeave={openOnHover() ? state.handleMouseLeave : undefined}
    >
      <button
        ref={state.setTriggerRef}
        type="button"
        class={triggerClass()}
        onClick={state.handleClick}
        onFocus={state.handleMouseEnter}
        onBlur={state.handleBlur}
        aria-expanded={state.isOpen()}
        aria-controls={popoverId()}
        aria-label={buttonLabel()}
      >
        {triggerVariant() === 'icon' ? (
          <>
            <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span class="sr-only">{buttonLabel()}</span>
          </>
        ) : (
          buttonLabel()
        )}
      </button>

      <Show when={state.isOpen()}>
        <div
          ref={state.setPopoverRef}
          id={popoverId()}
          role="dialog"
          aria-label={title()}
          class={`absolute ${positionClass()} z-50 mt-2 w-72 overflow-hidden rounded-md border bg-surface text-left shadow-sm`}
        >
          <div class="flex items-center justify-between border-b border-border-subtle px-3 py-2">
            <span class="text-sm font-semibold text-base-content">{title()}</span>
            <button
              type="button"
              class="rounded p-1 transition-colors hover:text-muted"
              onClick={state.close}
              aria-label="Close search tips"
            >
              <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
          <div class="px-3 py-3 text-xs text-muted">
            <Show when={props.intro}>
              <p class="mb-3 text-[11px] uppercase tracking-wide text-muted">{props.intro}</p>
            </Show>
            <div class="space-y-2">
              <For each={props.tips}>
                {(tip) => (
                  <div class="flex items-start gap-2">
                    <code class="rounded bg-surface-alt px-2 py-0.5 font-mono text-[11px] text-base-content">
                      {tip.code}
                    </code>
                    <span class="text-[12px] leading-snug text-muted">{tip.description}</span>
                  </div>
                )}
              </For>
            </div>
            <Show when={props.footerText || props.footerHighlight}>
              <div class="mt-3 rounded-md bg-blue-50 px-3 py-2 text-[11px] text-blue-700 dark:bg-blue-900 dark:text-blue-200">
                <Show when={props.footerHighlight}>
                  <code class="mr-1 rounded bg-blue-100 px-1 py-0.5 font-mono text-[11px] text-blue-700 dark:bg-blue-800 dark:text-blue-100">
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
