import { Component, Show } from 'solid-js';
import { SearchInput } from '@/components/shared/SearchInput';
import {
  type CollapsibleSearchInputProps,
} from './collapsibleSearchInputModel';
import { useCollapsibleSearchInputState } from './useCollapsibleSearchInputState';

export type { CollapsibleSearchInputProps } from './collapsibleSearchInputModel';

export const CollapsibleSearchInput: Component<CollapsibleSearchInputProps> = (props) => {
  const collapsible = useCollapsibleSearchInputState(props);

  return (
    <div
      ref={collapsible.setRootRef}
      class={collapsible.rootClass()}
      onFocusOut={collapsible.handleFocusOut}
    >
      <Show
        when={collapsible.showExpanded()}
        fallback={
          <div class="inline-flex rounded-md bg-surface-hover p-0.5">
            <button
              type="button"
              onClick={() => collapsible.expandSearch(false)}
              class="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 text-muted hover:text-base-content hover:bg-surface-hover"
              aria-label={props.title ?? props.placeholder ?? 'Open search'}
              title={`${props.placeholder ?? 'Search'} (/)`}
            >
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                />
              </svg>
              <span>{collapsible.triggerLabel()}</span>
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
            collapsible.setInputRef(el);
            props.inputRef?.(el);
          }}
          onBeforeAutoFocus={props.onBeforeAutoFocus}
          typeToSearch={false}
        />
      </Show>
    </div>
  );
};
