import {
  type Accessor,
  type Component,
  type JSX,
  Show,
  createSignal,
  createUniqueId,
} from 'solid-js';
import SlidersHorizontalIcon from 'lucide-solid/icons/sliders-horizontal';

import { FilterPopoverTrigger } from '@/components/shared/FilterToolbar';

export interface ViewOptionsDisclosureState {
  close: (restoreFocus?: boolean) => void;
  open: Accessor<boolean>;
  panelId: string;
  titleId: string;
  toggle: (trigger: HTMLButtonElement) => void;
}

export function createViewOptionsDisclosureState(): ViewOptionsDisclosureState {
  const [open, setOpen] = createSignal(false);
  const panelId = createUniqueId();
  const titleId = `${panelId}-title`;
  let triggerRef: HTMLButtonElement | undefined;

  const close = (restoreFocus = false) => {
    setOpen(false);
    if (restoreFocus) queueMicrotask(() => triggerRef?.focus());
  };

  return {
    close,
    open,
    panelId,
    titleId,
    toggle: (trigger) => {
      triggerRef = trigger;
      setOpen((value) => !value);
    },
  };
}

export const ViewOptionsDisclosureTrigger: Component<{
  state: ViewOptionsDisclosureState;
}> = (props) => (
  <FilterPopoverTrigger
    open={props.state.open()}
    onClick={(event) => props.state.toggle(event.currentTarget)}
    onKeyDown={(event) => {
      if (event.key !== 'Escape' || !props.state.open()) return;
      event.preventDefault();
      props.state.close(true);
    }}
    aria-expanded={props.state.open()}
    aria-controls={props.state.panelId}
    title="Show or hide table presentation settings"
  >
    <SlidersHorizontalIcon class="h-3.5 w-3.5" aria-hidden="true" />
    View
  </FilterPopoverTrigger>
);

interface ViewOptionsDisclosurePanelProps {
  children: JSX.Element;
  state: ViewOptionsDisclosureState;
  label?: string;
  description?: string;
}

export const ViewOptionsDisclosurePanel: Component<ViewOptionsDisclosurePanelProps> = (props) => (
  <Show when={props.state.open()}>
    <section
      id={props.state.panelId}
      role="region"
      aria-label={props.label ?? 'View preferences'}
      aria-labelledby={props.state.titleId}
      onKeyDown={(event) => {
        if (event.key !== 'Escape') return;
        event.preventDefault();
        props.state.close(true);
      }}
      class="rounded-md border border-border-subtle bg-surface-alt/40 p-3"
    >
      <div class="mb-3 border-b border-border-subtle pb-2">
        <div id={props.state.titleId} class="text-xs font-medium text-base-content">
          {props.label ?? 'View preferences'}
        </div>
        <div class="mt-0.5 text-[11px] leading-4 text-muted">
          {props.description ?? 'These choices are remembered for future visits.'}
        </div>
      </div>
      <div class="view-options-grid grid items-start gap-3">{props.children}</div>
    </section>
  </Show>
);
