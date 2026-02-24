import { Component, Show, onCleanup, createEffect, createSignal } from 'solid-js';
import { SearchInput, type SearchInputProps } from '@/components/shared/SearchInput';

interface CollapsibleSearchInputProps extends Omit<SearchInputProps, 'autoFocus'> {
  triggerLabel?: string;
  fullWidthWhenExpanded?: boolean;
}

const isEditableTarget = (target: EventTarget | null): boolean => {
  if (!target || !(target instanceof HTMLElement)) return false;
  const tag = target.tagName.toLowerCase();
  if (tag === 'input' || tag === 'textarea' || tag === 'select') return true;
  if (target.isContentEditable) return true;
  if (target.getAttribute('role') === 'textbox') return true;
  return false;
};

export const CollapsibleSearchInput: Component<CollapsibleSearchInputProps> = (props) => {
  const [isExpanded, setIsExpanded] = createSignal(props.value().trim().length > 0);
  let rootRef: HTMLDivElement | undefined;
  let inputRef: HTMLInputElement | undefined;
  let suppressCollapse = false;

  const focusInput = (selectText = false) => {
    queueMicrotask(() => {
      if (!inputRef) return;
      inputRef.focus();
      if (selectText) {
        inputRef.select?.();
      }
    });
  };

  const expandSearch = (selectText = false) => {
    suppressCollapse = true;
    queueMicrotask(() => {
      suppressCollapse = false;
    });
    if (!isExpanded()) {
      setIsExpanded(true);
    }
    focusInput(selectText);
  };

  const collapseIfEmpty = () => {
    if (props.value().trim().length > 0) return;
    setIsExpanded(false);
  };

  createEffect(() => {
    if (props.value().trim().length > 0 && !isExpanded()) {
      setIsExpanded(true);
    }
  });

  const handleAutoFocusKey = (e: KeyboardEvent) => {
    if (isEditableTarget(e.target)) return;
    if (e.key === '/' || e.key.length !== 1 || e.ctrlKey || e.metaKey || e.altKey) return;
    if (props.onBeforeAutoFocus?.()) return;

    e.preventDefault();
    if (!isExpanded()) {
      setIsExpanded(true);
    }
    props.onChange(`${props.value()}${e.key}`);
    focusInput(false);
  };

  if (typeof document !== 'undefined') {
    document.addEventListener('keydown', handleAutoFocusKey);
    onCleanup(() => document.removeEventListener('keydown', handleAutoFocusKey));
  }

  const triggerLabel = () => props.triggerLabel ?? 'Search';
  const showExpanded = () => isExpanded() || props.value().trim().length > 0;
  const rootClass = () => {
    const baseClass = props.class ?? '';
    if (!props.fullWidthWhenExpanded) return baseClass;
    const layoutClass = showExpanded() ? 'order-last basis-full w-full' : 'shrink-0 md:ml-auto';
    return `${baseClass} ${layoutClass}`.trim();
  };

  return (
    <div
      ref={(el) => (rootRef = el)}
      class={rootClass()}
      onFocusOut={(e) => {
        if (suppressCollapse) return;
        const next = e.relatedTarget as Node | null;
        if (next && rootRef?.contains(next)) return;
        collapseIfEmpty();
      }}
    >
      <Show
        when={showExpanded()}
        fallback={
          <div class="inline-flex rounded-md bg-surface-hover p-0.5">
            <button
              type="button"
              onClick={() => expandSearch(false)}
              class="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 text-muted hover:text-base-content hover:bg-surface-hover"
              aria-label={props.title ?? props.placeholder ?? 'Open search'}
              title={`${props.placeholder ?? 'Search'} (/)`}
              data-global-search-trigger
            >
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                />
              </svg>
              <span>{triggerLabel()}</span>
            </button>
          </div>
        }
      >
        <SearchInput
          value={props.value}
          onChange={props.onChange}
          placeholder={props.placeholder}
          title={props.title}
          history={props.history}
          tips={props.tips}
          inputRef={(el) => {
            inputRef = el;
            props.inputRef?.(el);
          }}
          onBeforeAutoFocus={props.onBeforeAutoFocus}
          autoFocus={false}
        />
      </Show>
    </div>
  );
};
