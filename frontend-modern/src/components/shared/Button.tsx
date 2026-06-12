import { A } from '@solidjs/router';
import { JSX, Show, mergeProps, splitProps } from 'solid-js';
import { getButtonClass, type ButtonSize, type ButtonVariant } from './buttonModel';

export interface ButtonProps extends JSX.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  isLoading?: boolean;
  class?: string;
}

export interface ButtonLinkProps extends JSX.AnchorHTMLAttributes<HTMLAnchorElement> {
  href: string;
  variant?: ButtonVariant;
  size?: ButtonSize;
  class?: string;
  hardNavigation?: boolean;
}

export function Button(props: ButtonProps) {
  const merged = mergeProps(
    { variant: 'secondary' as ButtonVariant, size: 'md' as ButtonSize, type: 'button' as const },
    props,
  );
  const [local, rest] = splitProps(merged, [
    'variant',
    'size',
    'isLoading',
    'class',
    'children',
    'disabled',
  ]);

  return (
    <button
      class={getButtonClass({
        variant: local.variant,
        size: local.size,
        class: local.class,
      })}
      disabled={local.disabled || local.isLoading}
      {...rest}
    >
      {local.isLoading ? (
        <svg
          class="animate-spin -ml-1 mr-2 h-4 w-4 text-current"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle
            class="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            stroke-width="4"
          ></circle>
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          ></path>
        </svg>
      ) : null}
      {local.children}
    </button>
  );
}

export function ButtonLink(props: ButtonLinkProps) {
  const merged = mergeProps(
    { variant: 'secondary' as ButtonVariant, size: 'md' as ButtonSize },
    props,
  );
  const [local, rest] = splitProps(merged, [
    'variant',
    'size',
    'class',
    'children',
    'href',
    'hardNavigation',
    'rel',
    'target',
  ]);
  const className = () =>
    getButtonClass({
      variant: local.variant,
      size: local.size,
      class: local.class,
    });
  const useNativeAnchor = () =>
    Boolean(
      local.hardNavigation ||
      local.target === '_blank' ||
      /^(https?:|mailto:|tel:)/.test(local.href),
    );
  const rel = () => local.rel ?? (local.target === '_blank' ? 'noopener noreferrer' : undefined);

  return (
    <Show
      when={useNativeAnchor()}
      fallback={
        <A {...rest} href={local.href} class={className()}>
          {local.children}
        </A>
      }
    >
      <a {...rest} href={local.href} class={className()} target={local.target} rel={rel()}>
        {local.children}
      </a>
    </Show>
  );
}

export default Button;
