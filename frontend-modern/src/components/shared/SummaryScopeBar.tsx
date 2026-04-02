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

const badgeClassForMode = (mode: SummaryScopePresentation['mode']) => {
  switch (mode) {
    case 'preview':
      return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-300';
    case 'pinned':
      return 'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-900/60 dark:bg-sky-950/40 dark:text-sky-300';
    default:
      return 'border-border bg-surface text-muted';
  }
};

const labelForMode = (mode: SummaryScopePresentation['mode']) => {
  switch (mode) {
    case 'preview':
      return 'Preview';
    case 'pinned':
      return 'Pinned';
    default:
      return 'All';
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
      class={`rounded-md border border-border-subtle bg-surface-alt/60 px-3 py-2 ${props.class ?? ''}`.trim()}
    >
      <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
        <span class="text-[10px] font-semibold uppercase tracking-[0.16em] text-muted">
          Scope
        </span>
        <span
          class={`inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] ${badgeClassForMode(props.active.mode)}`.trim()}
        >
          {labelForMode(props.active.mode)}
        </span>
        <span class="min-w-0 truncate text-sm font-medium text-base-content">
          {props.active.label}
        </span>
        <Show when={helperText()}>
          {(text) => <span class="min-w-0 truncate text-xs text-muted">{text()}</span>}
        </Show>
        <Show when={props.onReset}>
          <button
            type="button"
            class="ml-auto inline-flex items-center rounded-md border border-border px-2 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface"
            onClick={() => props.onReset?.()}
          >
            {props.resetLabel ?? 'Reset pinned scope'}
          </button>
        </Show>
      </div>
    </div>
  );
};

export default SummaryScopeBar;
