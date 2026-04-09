import { Show, type Component } from 'solid-js';
import {
  createSummaryInteractiveActionKeydownHandler,
  SUMMARY_ROW_ACTION_BUTTON_FOCUS_CLASS,
} from './summaryInteractionA11y';

type SummaryRowActionButtonProps =
  | {
      kind: 'disclosure';
      expanded: boolean;
      subjectLabel: string;
      controlsId?: string;
      class?: string;
      onAction: () => void;
      onPreviewClear?: () => void;
    }
  | {
      kind: 'scope';
      pressed: boolean;
      subjectLabel: string;
      class?: string;
      inactiveLabel?: string;
      activeLabel?: string;
      onAction: () => void;
      onPreviewClear?: () => void;
    };

const DISCLOSURE_BUTTON_CLASS = [
  'inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-muted transition-colors',
  'hover:bg-surface hover:text-base-content',
  SUMMARY_ROW_ACTION_BUTTON_FOCUS_CLASS,
].join(' ');

const SCOPE_BUTTON_CLASS = [
  'inline-flex shrink-0 items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] transition-colors',
  'border-border bg-surface text-muted hover:border-sky-200 hover:text-sky-700 dark:hover:border-sky-900/60 dark:hover:text-sky-300',
  SUMMARY_ROW_ACTION_BUTTON_FOCUS_CLASS,
].join(' ');

export const SummaryRowActionButton: Component<SummaryRowActionButtonProps> = (props) => {
  const disclosureProps = () => (props.kind === 'disclosure' ? props : null);
  const scopeProps = () => (props.kind === 'scope' ? props : null);
  const disclosureLabel = () =>
    disclosureProps()?.expanded ? `Collapse ${props.subjectLabel}` : `Expand ${props.subjectLabel}`;
  const scopeLabel = () =>
    scopeProps()?.pressed
      ? `Unpin summary scope for ${props.subjectLabel}`
      : `Pin summary scope for ${props.subjectLabel}`;

  return (
    <button
      type="button"
      data-row-action="true"
      class={`${props.kind === 'disclosure' ? DISCLOSURE_BUTTON_CLASS : SCOPE_BUTTON_CLASS} ${props.class ?? ''}`.trim()}
      aria-label={props.kind === 'disclosure' ? disclosureLabel() : scopeLabel()}
      aria-expanded={disclosureProps()?.expanded}
      aria-controls={disclosureProps()?.controlsId}
      aria-pressed={scopeProps()?.pressed}
      title={props.kind === 'disclosure' ? disclosureLabel() : scopeLabel()}
      onClick={(event) => {
        event.stopPropagation();
        props.onAction();
      }}
      onKeyDown={createSummaryInteractiveActionKeydownHandler({
        onAction: props.onAction,
        onPreviewClear: props.onPreviewClear,
      })}
    >
      <Show
        when={props.kind === 'scope'}
        fallback={
          <svg
            class={`h-3.5 w-3.5 transition-transform duration-150 ${disclosureProps()?.expanded ? 'rotate-90 text-base-content' : ''}`.trim()}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="2"
            aria-hidden="true"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M9 5l7 7-7 7"
            />
          </svg>
        }
      >
        <span
          class={`whitespace-nowrap ${scopeProps()?.pressed ? 'border-sky-200 bg-sky-50 px-0 py-0 text-sky-700 dark:border-sky-900/60 dark:bg-sky-950/40 dark:text-sky-300' : ''}`.trim()}
        >
          {scopeProps()?.pressed
            ? (scopeProps()?.activeLabel ?? 'Pinned')
            : (scopeProps()?.inactiveLabel ?? 'Scope')}
        </span>
      </Show>
    </button>
  );
};

export default SummaryRowActionButton;
