import { Show, type Component } from 'solid-js';
import type { SummaryScopePresentation } from './summaryScopePresentation';

interface SummaryScopeBarProps {
  class?: string;
  clearLabel?: string;
  onClear?: () => void;
  scope: SummaryScopePresentation;
  testId?: string;
}

export const SummaryScopeBar: Component<SummaryScopeBarProps> = (props) => {
  const helperText = () => {
    if (props.scope.contextLabel) {
      return `Within ${props.scope.contextLabel}`;
    }
    return null;
  };

  return (
    <div
      data-testid={props.testId}
      data-summary-scope-mode={props.scope.mode}
      data-summary-scope-variant="fallback"
      class={`flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 px-0 py-0.5 ${props.class ?? ''}`.trim()}
    >
      <span class="shrink-0 text-[11px] font-semibold tracking-[0.01em] text-muted">
        Scoped to
      </span>
      <span
        class="min-w-0 truncate text-xs font-medium text-base-content"
        title={props.scope.label}
      >
        {props.scope.label}
      </span>
      <Show when={helperText()}>
        {(text) => (
          <>
            <span aria-hidden="true" class="shrink-0 text-[11px] text-muted/70">
              •
            </span>
            <span class="min-w-0 truncate text-[11px] text-muted" title={text()}>
              {text()}
            </span>
          </>
        )}
      </Show>
      <Show when={props.onClear}>
        <button
          type="button"
          aria-label={props.clearLabel ?? 'Clear pinned scope'}
          class="ml-auto shrink-0 text-[11px] font-medium text-muted transition-colors hover:text-base-content focus-visible:text-base-content"
          onClick={() => props.onClear?.()}
        >
          Clear
        </button>
      </Show>
    </div>
  );
};

export default SummaryScopeBar;
