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
    }
  | {
      kind: 'context';
      pressed: boolean;
      subjectLabel: string;
      class?: string;
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

const CONTEXT_BUTTON_CLASS = [
  'inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-muted transition-colors',
  'hover:bg-surface hover:text-sky-700 dark:hover:text-sky-300',
  SUMMARY_ROW_ACTION_BUTTON_FOCUS_CLASS,
].join(' ');

export const SummaryRowActionButton: Component<SummaryRowActionButtonProps> = (props) => {
  const disclosureLabel = () =>
    props.expanded ? `Collapse ${props.subjectLabel}` : `Expand ${props.subjectLabel}`;
  const scopeLabel = () =>
    props.pressed
      ? `Unpin summary scope for ${props.subjectLabel}`
      : `Pin summary scope for ${props.subjectLabel}`;
  const contextLabel = () =>
    props.pressed
      ? `Clear global context for ${props.subjectLabel}`
      : `Set global context to ${props.subjectLabel}`;

  return (
    <button
      type="button"
      data-row-action="true"
      class={`${props.kind === 'disclosure' ? DISCLOSURE_BUTTON_CLASS : props.kind === 'scope' ? SCOPE_BUTTON_CLASS : CONTEXT_BUTTON_CLASS} ${props.class ?? ''}`.trim()}
      aria-label={
        props.kind === 'disclosure'
          ? disclosureLabel()
          : props.kind === 'scope'
            ? scopeLabel()
            : contextLabel()
      }
      aria-expanded={props.kind === 'disclosure' ? props.expanded : undefined}
      aria-controls={props.kind === 'disclosure' ? props.controlsId : undefined}
      aria-pressed={props.kind !== 'disclosure' ? props.pressed : undefined}
      title={
        props.kind === 'disclosure'
          ? disclosureLabel()
          : props.kind === 'scope'
            ? scopeLabel()
            : contextLabel()
      }
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
        when={props.kind !== 'disclosure'}
        fallback={
          <svg
            class={`h-3.5 w-3.5 transition-transform duration-150 ${props.expanded ? 'rotate-90 text-base-content' : ''}`.trim()}
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
        <Show
          when={props.kind === 'scope'}
          fallback={
            <svg
              class={`h-3.5 w-3.5 ${props.pressed ? 'text-sky-600 dark:text-sky-300' : ''}`.trim()}
              fill={props.pressed ? 'currentColor' : 'none'}
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="1.8"
              aria-hidden="true"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M12 17.25V21m0-3.75L8.25 13.5m3.75 3.75L15.75 13.5M6 3.75h12a.75.75 0 01.75.75v1.086a3 3 0 01-.879 2.121l-3.242 3.243a3 3 0 00-.879 2.12V15a.75.75 0 01-.75.75h-2.5a.75.75 0 01-.75-.75v-1.257a3 3 0 00-.879-2.12L5.629 7.707A3 3 0 014.75 5.586V4.5A.75.75 0 015.5 3.75z"
              />
            </svg>
          }
        >
          <span
            class={`whitespace-nowrap ${props.pressed ? 'border-sky-200 bg-sky-50 px-0 py-0 text-sky-700 dark:border-sky-900/60 dark:bg-sky-950/40 dark:text-sky-300' : ''}`.trim()}
          >
            {props.pressed ? (props.activeLabel ?? 'Pinned') : (props.inactiveLabel ?? 'Scope')}
          </span>
        </Show>
      </Show>
    </button>
  );
};

export default SummaryRowActionButton;
