import { Show, createSignal, type Component, type JSX } from 'solid-js';

interface TechnicalDetailsDisclosureProps {
  children: JSX.Element;
  title?: string;
  subtitle: string;
  dataTestId: string;
  class?: string;
  contentClass?: string;
}

export const TechnicalDetailsDisclosure: Component<TechnicalDetailsDisclosureProps> = (props) => {
  const [expanded, setExpanded] = createSignal(false);

  return (
    <details
      data-testid={props.dataTestId}
      class={props.class ?? 'rounded border border-border bg-surface px-2 py-1.5'}
      onToggle={(event) => setExpanded(event.currentTarget.open)}
    >
      <summary class="cursor-pointer list-none text-[11px] font-medium text-base-content">
        {props.title ?? 'Technical details'}
        <span class="ml-2 font-normal text-muted">{props.subtitle}</span>
      </summary>
      <Show when={expanded()}>
        <div class={props.contentClass ?? 'mt-2 border-t border-border pt-2'}>{props.children}</div>
      </Show>
    </details>
  );
};
