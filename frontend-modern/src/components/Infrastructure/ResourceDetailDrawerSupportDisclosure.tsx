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

        <button
          type="button"
          onClick={props.onToggle}
          class={
            props.buttonClass ??
            'inline-flex items-center rounded-md border border-border bg-surface px-2.5 py-1 text-[10px] font-medium text-base-content transition-colors hover:bg-base'
          }
        >
          {props.expanded ? props.hideLabel : props.showLabel}
        </button>
      </div>

      <Show when={props.expanded}>
        <div class={props.contentClass ?? 'mt-3'}>{props.children}</div>
      </Show>
    </div>
  );
};
