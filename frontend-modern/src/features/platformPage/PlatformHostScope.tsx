import { Show, type Component, type JSX } from 'solid-js';
import { revealElementInViewport } from '@/components/shared/contextualFocus';
import { SUMMARY_ROW_ACTION_BUTTON_FOCUS_CLASS } from '@/components/shared/summaryInteractionA11y';
import { TABLE_CARD_HEADER_CLEAR_BUTTON_CLASS } from '@/components/shared/TableCardHeader';

// A platform overview pairs a host table with the workloads those hosts run
// (Proxmox nodes and guests, Docker hosts and containers, vSphere hosts and
// VMs). Clicking a host row narrows the workload table to that host and a
// second click clears it; the row's chevron opens the host's drawer. These
// pieces keep that interaction one shape on every platform.

/** A host row's name, as the control that narrows the workload table to it. */
export const PlatformHostScopeNameButton: Component<{
  /** The host's full name, for the accessible label and tooltip. */
  name: string;
  /** Plural workload noun: "guests", "containers", "VMs". */
  workloadNoun: string;
  /** The id of the section holding the workload table. */
  controlsId: string;
  selected: boolean;
  onSelect: () => void;
  /** What the cell shows, when it differs from `name`. */
  children?: JSX.Element;
}> = (props) => (
  <button
    type="button"
    class={`block max-w-full truncate rounded-xs text-left ${SUMMARY_ROW_ACTION_BUTTON_FOCUS_CLASS}`}
    aria-label={`Show ${props.workloadNoun} on ${props.name}`}
    aria-controls={props.controlsId}
    aria-pressed={props.selected}
    title={
      props.selected
        ? `Clear ${props.workloadNoun} filter for ${props.name}`
        : `Show ${props.workloadNoun} on ${props.name}`
    }
    onClick={(event) => {
      event.stopPropagation();
      props.onSelect();
    }}
  >
    {props.children ?? props.name}
  </button>
);

/**
 * The workload table's heading, which names the host it is narrowed to and
 * offers the way back. Selecting a host moves focus here and reveals it, so a
 * keyboard or screen-reader user lands on the result of their click.
 */
export function createPlatformHostScopeFocus() {
  let heading: HTMLHeadingElement | undefined;
  return {
    setHeading: (element: HTMLHeadingElement) => {
      heading = element;
    },
    /** Focus the heading and scroll it into view if it is off screen. */
    reveal: () => {
      heading?.focus({ preventScroll: true });
      if (heading) revealElementInViewport({ element: heading, bottomPadding: 96 });
    },
    /** Focus the heading without scrolling, after the scope is cleared. */
    focus: () => heading?.focus({ preventScroll: true }),
  };
}

export const PlatformHostScopedTableTitle: Component<{
  /** Workload noun as a title: "Guests", "Containers", "VMs". */
  noun: string;
  /** The selected host's name, or null when the table is not narrowed. */
  hostLabel: string | null;
  /** Rows shown, when the page counts them. */
  count?: number | null;
  /** The clear button's label: "Show all nodes", "Show all hosts". */
  clearLabel: string;
  onClear: () => void;
  headingId: string;
  headingRef: (element: HTMLHeadingElement) => void;
}> = (props) => (
  <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
    <h2
      ref={props.headingRef}
      id={props.headingId}
      tabIndex={-1}
      class="inline-flex flex-wrap items-center gap-1.5 rounded-xs focus:outline-hidden focus-visible:ring-2 focus-visible:ring-primary/40"
    >
      {props.hostLabel ? `${props.noun} on ${props.hostLabel}` : props.noun}
      <Show when={typeof props.count === 'number'}>
        <span class="font-semibold tabular-nums text-base-content">{props.count}</span>
      </Show>
    </h2>
    <Show when={props.hostLabel}>
      <button
        type="button"
        class={`${TABLE_CARD_HEADER_CLEAR_BUTTON_CLASS} min-h-6`}
        onClick={props.onClear}
      >
        {props.clearLabel}
      </button>
    </Show>
  </div>
);
