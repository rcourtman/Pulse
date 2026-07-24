import { Match, Switch, type Component, type JSX } from 'solid-js';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import TriangleAlertIcon from 'lucide-solid/icons/triangle-alert';
import { classifyWebInterfaceUrl } from './webInterfaceUrlFieldModel';

export const WEB_INTERFACE_LINK_COLOR_CLASS =
  'text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300';

export interface WebInterfaceLinkProps {
  url?: string | null;
  ariaLabel: string;
  title?: string;
  class?: string;
  invalidAriaLabel?: string;
  invalidTitle?: string;
  invalidClass?: string;
  children: JSX.Element;
}

/**
 * Canonical external web-interface control.
 *
 * Persisted metadata is treated as untrusted at render time. Only absolute
 * HTTP(S) URLs become anchors; malformed and unsafe values remain visible as
 * an accessible warning, never as a navigable link.
 */
export const WebInterfaceLink: Component<WebInterfaceLinkProps> = (props) => {
  const classified = () => classifyWebInterfaceUrl(props.url);

  return (
    <Switch>
      <Match when={classified().status === 'valid' && classified()}>
        {(result) => (
          <a
            href={result().url}
            target="_blank"
            rel="noopener noreferrer"
            class={props.class}
            title={props.title ?? `Open ${result().url}`}
            aria-label={props.ariaLabel}
            onClick={(event) => event.stopPropagation()}
            onKeyDown={(event) => event.stopPropagation()}
          >
            {props.children}
          </a>
        )}
      </Match>
      <Match when={classified().status === 'invalid'}>
        <span
          role="img"
          class={
            props.invalidClass ??
            'inline-flex min-h-6 min-w-6 shrink-0 items-center justify-center rounded text-amber-600 dark:text-amber-400'
          }
          title={props.invalidTitle ?? 'Invalid web interface URL. Pulse will not open it.'}
          aria-label={props.invalidAriaLabel ?? 'Invalid web interface URL'}
          data-web-interface-url-state="invalid"
        >
          <TriangleAlertIcon class="h-3.5 w-3.5" aria-hidden="true" />
        </span>
      </Match>
    </Switch>
  );
};

export interface ResourceNameWithWebInterfaceLinkProps {
  name: string;
  url?: string | null;
  class?: string;
  nameClass?: string;
  title?: string;
  ariaLabel?: string;
  children?: JSX.Element;
}

/**
 * Keeps the resource name inert so row selection and disclosure semantics are
 * unchanged, then exposes web access as a distinct adjacent control.
 */
export const ResourceNameWithWebInterfaceLink: Component<ResourceNameWithWebInterfaceLinkProps> = (
  props,
) => (
  <span class={`inline-flex min-w-0 max-w-full items-center gap-1 ${props.class ?? ''}`}>
    <span class={props.nameClass} title={props.name}>
      {props.children ?? props.name}
    </span>
    <WebInterfaceLink
      url={props.url}
      ariaLabel={props.ariaLabel ?? `Open web interface for ${props.name}`}
      title={props.title}
      invalidAriaLabel={`Web interface URL for ${props.name} is invalid`}
      class={`-my-1 inline-flex min-h-6 min-w-6 shrink-0 items-center justify-center rounded transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 ${WEB_INTERFACE_LINK_COLOR_CLASS}`}
    >
      <ExternalLinkIcon class="h-3.5 w-3.5" aria-hidden="true" />
    </WebInterfaceLink>
  </span>
);

export default ResourceNameWithWebInterfaceLink;
