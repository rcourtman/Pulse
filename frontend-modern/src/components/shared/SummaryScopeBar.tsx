import { Show, type Component } from 'solid-js';
import type { SummaryScopePresentation } from './summaryScopePresentation';

interface SummaryScopeBarProps {
  active: SummaryScopePresentation;
  class?: string;
  idleHint?: string;
  pinned?: SummaryScopePresentation | null;
  resetLabel?: string;
  testId?: string;
  onReset?: () => void;
}

const leadClassForMode = (mode: SummaryScopePresentation['mode']) => {
  switch (mode) {
    case 'preview':
      return 'text-amber-700 dark:text-amber-300';
    case 'pinned':
      return 'text-sky-700 dark:text-sky-300';
    default:
      return 'text-muted';
  }
};

const leadLabelForMode = (mode: SummaryScopePresentation['mode']) => {
  switch (mode) {
    case 'preview':
      return 'Previewing';
    case 'pinned':
      return 'Pinned to';
    default:
      return 'Showing';
  }
};

export const SummaryScopeBar: Component<SummaryScopeBarProps> = (props) => {
  const helperText = () => {
    if (props.active.mode === 'preview' && props.pinned?.mode === 'pinned') {
      return `Pinned to ${props.pinned.label}`;
    }
    if (props.active.contextLabel) {
      return `Within ${props.active.contextLabel}`;
    }
    if (props.active.mode === 'all') {
      return props.idleHint ?? null;
    }
    return null;
  };

  return (
    <div
      data-testid={props.testId}
      data-summary-scope-mode={props.active.mode}
      class={`flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 px-1 py-1 ${props.class ?? ''}`.trim()}
    >
      <span
        class={`shrink-0 text-[11px] font-semibold tracking-[0.01em] ${leadClassForMode(props.active.mode)}`.trim()}
      >
        {leadLabelForMode(props.active.mode)}
      </span>
      <span class="min-w-0 truncate text-sm font-medium text-base-content" title={props.active.label}>
        {props.active.label}
      </span>
      <Show when={helperText()}>
        {(text) => (
          <>
            <span aria-hidden="true" class="shrink-0 text-xs text-muted/70">
              •
            </span>
            <span class="min-w-0 truncate text-xs text-muted" title={text()}>
              {text()}
            </span>
          </>
        )}
      </Show>
      <Show when={props.onReset}>
        <button
          type="button"
          aria-label={props.resetLabel ?? 'Reset pinned scope'}
          class="ml-auto shrink-0 text-xs font-medium text-muted transition-colors hover:text-base-content focus-visible:text-base-content"
          onClick={() => props.onReset?.()}
        >
          Reset
        </button>
      </Show>
    </div>
  );
};

export default SummaryScopeBar;
