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
      class={props.class ?? 'rounded border border-dashed border-border bg-surface-hover p-3'}
    >
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div class="text-[11px] font-medium uppercase tracking-wide text-base-content">
            {props.title}
          </div>
          <Show when={summary()}>
            <div class="mt-1 text-[10px] text-base-content">{summary()}</div>
          </Show>
        </div>

        <div class="flex min-w-0 items-center gap-2">
          {props.headerExtra}
          <button
            type="button"
            onClick={props.onToggle}
            class={
              props.buttonClass ??
              'inline-flex shrink-0 items-center rounded-md border border-border bg-surface px-2.5 py-1 text-[10px] font-medium text-base-content transition-colors hover:bg-base'
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
