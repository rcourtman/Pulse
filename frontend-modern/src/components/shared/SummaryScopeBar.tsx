import { Show, type Component } from 'solid-js';
import type { SummaryScopePresentation } from './summaryScopePresentation';

interface SummaryScopeBarProps {
  class?: string;
  onReset?: () => void;
  resetLabel?: string;
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
      data-summary-scope-variant="inline"
      class={`inline-flex min-w-0 max-w-full items-center gap-1.5 overflow-hidden ${props.class ?? ''}`.trim()}
    >
      <span class="shrink-0 text-xs text-muted">
        Pinned to
      </span>
      <span
        class="min-w-0 max-w-[16rem] truncate text-xs font-medium text-base-content"
        title={props.scope.label}
      >
        {props.scope.label}
      </span>
      <Show when={helperText()}>
        {(text) => (
          <>
            <span aria-hidden="true" class="shrink-0 text-xs text-muted/70">
              •
            </span>
            <span class="min-w-0 max-w-[10rem] truncate text-xs text-muted" title={text()}>
              {text()}
            </span>
          </>
        )}
      </Show>
      <Show when={props.onReset}>
        <button
          type="button"
          aria-label={props.resetLabel ?? 'Reset pinned scope'}
          class="shrink-0 text-xs font-medium text-muted transition-colors hover:text-base-content focus-visible:text-base-content"
          onClick={() => props.onReset?.()}
        >
          Reset
        </button>
      </Show>
    </div>
  );
};

export default SummaryScopeBar;
