import { Show, type Component, type JSX } from 'solid-js';
import { asTrimmedString } from '@/utils/stringUtils';

export interface WebInterfaceNameLinkProps {
  name: string;
  url?: string | null;
  class?: string;
  fallbackClass?: string;
  title?: string;
  ariaLabel?: string;
  children?: JSX.Element;
}

export const WebInterfaceNameLink: Component<WebInterfaceNameLinkProps> = (props) => {
  const url = () => asTrimmedString(props.url) ?? '';
  const label = () => props.children ?? props.name;
  const fallbackClass = () => props.fallbackClass ?? props.class ?? '';
  const linkClass = () => props.class ?? '';
  const linkTitle = () => props.title ?? `Open ${url()}`;
  const ariaLabel = () => props.ariaLabel ?? `Open web interface for ${props.name}`;

  return (
    <Show
      when={url()}
      fallback={
        <span class={fallbackClass()} title={props.name}>
          {label()}
        </span>
      }
    >
      {(href) => (
        <a
          href={href()}
          target="_blank"
          rel="noopener noreferrer"
          class={linkClass()}
          title={linkTitle()}
          aria-label={ariaLabel()}
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          {label()}
        </a>
      )}
    </Show>
  );
};

export default WebInterfaceNameLink;
