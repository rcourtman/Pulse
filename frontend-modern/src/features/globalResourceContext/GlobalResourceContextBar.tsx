import { Show, type Component } from 'solid-js';
import { useGlobalResourceContext } from './GlobalResourceContext';

export const GlobalResourceContextBar: Component<{ class?: string }> = (props) => {
  const globalContext = useGlobalResourceContext();

  return (
    <Show when={globalContext.hasGlobalResourceContext()}>
      <div
        class={`flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 px-1 py-1 ${props.class ?? ''}`.trim()}
        data-global-resource-context="true"
      >
        <span class="shrink-0 text-[11px] font-semibold tracking-[0.01em] text-sky-700 dark:text-sky-300">
          Context
        </span>
        <span
          class="min-w-0 truncate text-sm font-medium text-base-content"
          title={globalContext.contextLabel() ?? undefined}
        >
          {globalContext.contextLabel()}
        </span>
        <span class="min-w-0 truncate text-xs text-muted">
          Scoped across platform views
        </span>
        <button
          type="button"
          aria-label="Clear global context"
          class="ml-auto shrink-0 text-xs font-medium text-muted transition-colors hover:text-base-content focus-visible:text-base-content"
          onClick={() => globalContext.clearGlobalResourceContext()}
        >
          Clear
        </button>
      </div>
    </Show>
  );
};

export default GlobalResourceContextBar;
