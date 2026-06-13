import { splitProps, type JSX } from 'solid-js';

export type ExternalTextLinkVariant =
  | 'inline'
  | 'inlineSubtle'
  | 'muted'
  | 'compact'
  | 'compactAction'
  | 'compactInherit'
  | 'inlineAction';

interface ExternalTextLinkProps extends Omit<
  JSX.AnchorHTMLAttributes<HTMLAnchorElement>,
  'href' | 'rel' | 'target'
> {
  href: string;
  variant?: ExternalTextLinkVariant;
  preserveOpener?: boolean;
  class?: string;
}

export const EXTERNAL_TEXT_LINK_REL = 'noopener noreferrer';

export const EXTERNAL_TEXT_LINK_BASE_CLASS =
  'underline-offset-2 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60';

export const EXTERNAL_TEXT_LINK_VARIANT_CLASSES: Record<ExternalTextLinkVariant, string> = {
  inline: 'text-blue-600 hover:underline dark:text-blue-400',
  inlineSubtle: 'text-blue-600 hover:underline dark:text-blue-300',
  muted: 'text-muted hover:text-base-content hover:underline',
  compact:
    'inline-flex min-h-10 items-center rounded px-1 py-1 text-sm text-blue-600 hover:underline dark:text-blue-400 sm:min-h-9',
  compactAction:
    'inline-flex min-h-10 items-center rounded px-1 text-xs font-medium text-blue-700 hover:underline dark:text-blue-300 sm:min-h-9',
  compactInherit: 'inline-flex min-h-10 items-center rounded px-1 underline sm:min-h-9',
  inlineAction:
    'inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium text-blue-600 hover:underline dark:text-blue-300',
};

export const getExternalTextLinkRel = (preserveOpener = false) =>
  preserveOpener ? undefined : EXTERNAL_TEXT_LINK_REL;

export function ExternalTextLink(props: ExternalTextLinkProps) {
  const [local, rest] = splitProps(props, [
    'href',
    'variant',
    'preserveOpener',
    'class',
    'children',
  ]);
  const variant = () => local.variant ?? 'inline';

  return (
    <a
      {...rest}
      href={local.href}
      target="_blank"
      rel={getExternalTextLinkRel(local.preserveOpener)}
      class={`${EXTERNAL_TEXT_LINK_BASE_CLASS} ${EXTERNAL_TEXT_LINK_VARIANT_CLASSES[variant()]} ${
        local.class ?? ''
      }`.trim()}
    >
      {local.children}
    </a>
  );
}

export default ExternalTextLink;
