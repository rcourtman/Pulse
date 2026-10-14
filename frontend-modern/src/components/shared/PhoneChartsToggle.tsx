import { Show, type Component } from 'solid-js';

// Below the `sm` breakpoint a stack of charts can push everything after it
// off the screen, so chart groups show their first chart (plus any carrying
// a problem) and fold the rest behind this one full-width button. Wider
// layouts always show every chart and never render it.
export const PhoneChartsToggle: Component<{
  /** Names of the charts folded away on a phone, in display order. */
  hiddenLabels: readonly string[];
  expanded: boolean;
  onToggle: () => void;
  /** Id of the element holding the folded charts. */
  controls: string;
  testId?: string;
}> = (props) => (
  <Show when={props.hiddenLabels.length > 0 || props.expanded}>
    <button
      type="button"
      class="flex min-h-11 w-full items-center justify-center gap-1 rounded-xs border border-border bg-surface px-3 text-xs font-medium text-base-content sm:hidden"
      aria-controls={props.controls}
      aria-expanded={props.expanded}
      data-testid={props.testId ?? 'history-phone-toggle'}
      onClick={() => props.onToggle()}
    >
      {props.expanded ? 'Show fewer charts' : `Show ${props.hiddenLabels.join(', ')}`}
    </button>
  </Show>
);
