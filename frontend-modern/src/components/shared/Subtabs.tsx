import ChevronLeftIcon from 'lucide-solid/icons/chevron-left';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import {
  Component,
  createEffect,
  createSignal,
  For,
  JSX,
  onCleanup,
  onMount,
  Show,
  splitProps,
} from 'solid-js';
import { useActiveHorizontalRailItemVisibility } from './useActiveHorizontalRailItemVisibility';

export interface SubtabOption {
  value: string;
  label: JSX.Element;
  disabled?: boolean;
}

interface SubtabsProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onChange'> {
  value: string;
  onChange: (value: string) => void;
  tabs: SubtabOption[];
  ariaLabel: string;
  listClass?: string;
  tabClass?: string;
  /**
   * Optional content rendered on the right side of the same border-b row as the
   * tablist (e.g. a contextual range select on a drawer's History tab). When
   * absent, the shell renders only the tablist — preserving the original
   * single-row layout for existing callers.
   */
  trailing?: JSX.Element;
}

export const subtabsShellClass = 'border-b border-border';
export const subtabsListClass =
  'flex min-w-0 items-center gap-3 overflow-x-auto scrollbar-hide sm:gap-6';
export const subtabsRailClass = 'relative min-w-0 flex-1';
export const subtabsTrailingRowClass = 'flex flex-wrap items-center justify-between gap-3';
export const subtabButtonClass =
  'inline-flex min-h-9 shrink-0 select-none items-center whitespace-nowrap border-b-2 px-1 py-1 text-xs font-medium transition-colors sm:min-h-10 sm:py-2 sm:text-sm';
export const subtabButtonActiveClass = 'border-blue-600 text-base-content';
export const subtabButtonInactiveClass = 'border-transparent text-muted hover:text-base-content';
export const Subtabs: Component<SubtabsProps> = (props) => {
  let tablistRef: HTMLDivElement | undefined;
  const [hasOverflow, setHasOverflow] = createSignal(false);
  const [canScrollLeft, setCanScrollLeft] = createSignal(false);
  const [canScrollRight, setCanScrollRight] = createSignal(false);
  const [local, divProps] = splitProps(props, [
    'value',
    'onChange',
    'tabs',
    'ariaLabel',
    'class',
    'listClass',
    'tabClass',
    'trailing',
  ]);
  const activeItemVisibility = useActiveHorizontalRailItemVisibility({
    active: () => local.value,
    rail: () => tablistRef,
    activeSelector: '[role="tab"][aria-selected="true"]',
  });

  createEffect(() => {
    // Selection changes can alter the overflow controls after the shared rail
    // helper has moved horizontally. Do not use scrollIntoView here: it also
    // scrolls vertical ancestors and can pull an open inline drawer upwards
    // when live data recreates its tab strip.
    void local.value;
    queueMicrotask(() => {
      updateScrollControls();
    });
  });

  const updateScrollControls = () => {
    const rail = tablistRef;
    if (!rail) return;
    const maxScrollLeft = Math.max(0, rail.scrollWidth - rail.clientWidth);
    setHasOverflow(maxScrollLeft > 1);
    setCanScrollLeft(rail.scrollLeft > 1);
    setCanScrollRight(rail.scrollLeft < maxScrollLeft - 1);
  };

  onMount(() => {
    const rail = tablistRef;
    if (!rail) return;

    rail.addEventListener('scroll', updateScrollControls, { passive: true });
    window.addEventListener('resize', updateScrollControls, { passive: true });
    const resizeObserver =
      typeof ResizeObserver === 'function' ? new ResizeObserver(updateScrollControls) : undefined;
    resizeObserver?.observe(rail);
    updateScrollControls();

    onCleanup(() => {
      rail.removeEventListener('scroll', updateScrollControls);
      window.removeEventListener('resize', updateScrollControls);
      resizeObserver?.disconnect();
    });
  });

  const scrollTabs = (direction: -1 | 1) => {
    const rail = tablistRef;
    if (!rail) return;
    activeItemVisibility.markManualScrollIntent();
    rail.scrollBy({
      left: direction * Math.max(120, Math.round(rail.clientWidth * 0.7)),
      behavior: 'smooth',
    });
  };

  const tablist = () => (
    <div class={subtabsRailClass}>
      <div
        ref={tablistRef}
        role="tablist"
        aria-label={local.ariaLabel}
        class={`${subtabsListClass} ${hasOverflow() ? 'pr-10' : ''} ${local.listClass ?? ''}`.trim()}
      >
        <For each={local.tabs}>
          {(tab) => {
            const selected = () => local.value === tab.value;
            return (
              <button
                type="button"
                role="tab"
                aria-selected={selected()}
                tabIndex={selected() ? 0 : -1}
                disabled={tab.disabled}
                onClick={() => local.onChange(tab.value)}
                class={`${subtabButtonClass} ${
                  selected() ? subtabButtonActiveClass : subtabButtonInactiveClass
                } ${local.tabClass ?? ''}`.trim()}
              >
                {tab.label}
              </button>
            );
          }}
        </For>
      </div>
      <Show when={canScrollLeft()}>
        <button
          type="button"
          class="absolute inset-y-0 left-0 z-10 flex w-10 items-center justify-start bg-gradient-to-r from-surface via-surface to-transparent pl-1 text-muted hover:text-base-content focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-blue-500 sm:hidden"
          onClick={() => scrollTabs(-1)}
          aria-label={`${local.ariaLabel}: scroll left`}
        >
          <span class="flex h-7 w-7 items-center justify-center rounded-full border border-border bg-surface shadow-sm">
            <ChevronLeftIcon class="h-4 w-4" aria-hidden="true" />
          </span>
        </button>
      </Show>
      <Show when={canScrollRight()}>
        <button
          type="button"
          class="absolute inset-y-0 right-0 z-10 flex w-10 items-center justify-end bg-gradient-to-l from-surface via-surface to-transparent pr-1 text-muted hover:text-base-content focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-blue-500 sm:hidden"
          onClick={() => scrollTabs(1)}
          aria-label={`${local.ariaLabel}: scroll right`}
        >
          <span class="flex h-7 w-7 items-center justify-center rounded-full border border-border bg-surface shadow-sm">
            <ChevronRightIcon class="h-4 w-4" aria-hidden="true" />
          </span>
        </button>
      </Show>
    </div>
  );

  return (
    <div {...divProps} class={`${subtabsShellClass} ${local.class ?? ''}`.trim()}>
      {local.trailing ? (
        <div class={subtabsTrailingRowClass}>
          {tablist()}
          {local.trailing}
        </div>
      ) : (
        tablist()
      )}
    </div>
  );
};
