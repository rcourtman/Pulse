import { Show, createSignal, type Component, type JSX } from 'solid-js';
import { DetailSectionTable, type DetailSection } from './DetailSectionTable';

interface TechnicalDetailsDisclosureProps {
  children?: JSX.Element;
  sections?: DetailSection[];
  title?: string;
  subtitle: string;
  dataTestId: string;
  class?: string;
  contentClass?: string;
}

interface TechnicalDetailsSectionProps {
  children?: JSX.Element;
  sections?: DetailSection[];
  dataTestId: string;
  class?: string;
  contentClass?: string;
}

const TechnicalDetailsContent: Component<
  Pick<TechnicalDetailsSectionProps, 'children' | 'sections' | 'contentClass'>
> = (props) => (
  <Show when={props.sections} fallback={props.children}>
    {(sections) => (
      <DetailSectionTable
        sections={sections()}
        class={props.contentClass ?? 'overflow-hidden rounded border border-border bg-surface'}
      />
    )}
  </Show>
);

export const TechnicalDetailsSection: Component<TechnicalDetailsSectionProps> = (props) => (
  <div data-testid={props.dataTestId} class={props.class}>
    <TechnicalDetailsContent
      sections={props.sections}
      contentClass={props.contentClass}
      children={props.children}
    />
  </div>
);

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
        <div class={props.contentClass ?? 'mt-2 border-t border-border pt-2'}>
          <TechnicalDetailsContent sections={props.sections} children={props.children} />
        </div>
      </Show>
    </details>
  );
};
