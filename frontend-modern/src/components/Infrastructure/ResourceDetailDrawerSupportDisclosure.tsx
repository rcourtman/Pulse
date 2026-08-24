import { Show } from 'solid-js';
import type { Component, JSX } from 'solid-js';

interface ResourceDetailDrawerSupportDisclosureProps {
  title: string;
  summary?: string | null;
  expanded: boolean;
  onToggle: () => void;
  showLabel: string;
  hideLabel: string;
  children: JSX.Element;
  class?: string;
  buttonClass?: string;
  contentClass?: string;
  dataTestId?: string;
  // Rendered in the header row next to the toggle button, visible whether or
  // not the section is expanded — for actions that must not require a click
  // on the disclosure first (e.g. an open-web-interface link).
  headerExtra?: JSX.Element;
}

export const ResourceDetailDrawerSupportDisclosure: Component<
  ResourceDetailDrawerSupportDisclosureProps
> = (props) => {
  const summary = () => props.summary?.trim() ?? '';

  return (
    <div
      data-testid={props.dataTestId}
      class={`rounded border border-border bg-surface px-2 py-1.5 ${props.class ?? ''}`}
    >
      <div class="flex min-w-0 items-center justify-between gap-2">
        <div class="flex min-w-0 flex-1 items-baseline gap-2">
          <div class="shrink-0 text-[11px] font-medium uppercase tracking-wide text-base-content">
            {props.title}
          </div>
          <Show when={summary()}>
            <div class="truncate text-[10px] text-muted" title={summary()}>
              {summary()}
            </div>
          </Show>
        </div>

        <div class="flex min-w-0 items-center gap-2">
          {props.headerExtra}
          <button
            type="button"
            onClick={props.onToggle}
            class={
              props.buttonClass ??
              'inline-flex min-h-11 shrink-0 items-center rounded-md border border-border bg-surface px-2.5 py-1 text-[10px] font-medium text-base-content transition-colors hover:bg-base sm:min-h-0'
            }
          >
            {props.expanded ? props.hideLabel : props.showLabel}
          </button>
        </div>
      </div>

      <Show when={props.expanded}>
        <div class={props.contentClass ?? 'mt-3'}>{props.children}</div>
      </Show>
    </div>
  );
};
