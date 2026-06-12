import { A } from '@solidjs/router';
import CheckIcon from 'lucide-solid/icons/check';
import CopyIcon from 'lucide-solid/icons/copy';
import { JSX, Show, mergeProps, splitProps } from 'solid-js';
import {
  getButtonClass,
  getCopyValueButtonClass,
  type ButtonSize,
  type ButtonVariant,
  type CopyValueButtonSize,
  type CopyValueButtonVariant,
} from './buttonModel';

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
  preserveOpener?: boolean;
}

export interface CommandCopyButtonProps extends Omit<
  ButtonProps,
  'children' | 'isLoading' | 'size' | 'variant'
> {
  label?: string;
}

export interface CopyValueButtonProps extends Omit<
  JSX.ButtonHTMLAttributes<HTMLButtonElement>,
  'children' | 'onClick' | 'value'
> {
  value?: string | null;
  copied?: boolean;
  onCopyValue: (value: string) => void | Promise<void>;
  label: string;
  variant?: CopyValueButtonVariant;
  size?: CopyValueButtonSize;
  class?: string;
  children?: JSX.Element;
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

export function CommandCopyButton(props: CommandCopyButtonProps) {
  const merged = mergeProps({ label: 'Copy command', type: 'button' as const }, props);
  const [local, rest] = splitProps(merged, ['label', 'class', 'title', 'aria-label']);

  return (
    <Button
      {...rest}
      variant="ghost"
      size="icon"
      class={[
        'absolute right-2 top-2 min-h-10 min-w-10 bg-surface-hover text-muted hover:text-base-content sm:min-h-9 sm:min-w-9',
        local.class,
      ]
        .filter(Boolean)
        .join(' ')}
      title={local.title ?? local.label}
      aria-label={local['aria-label'] ?? local.label}
    >
      <CopyIcon class="h-4 w-4" />
    </Button>
  );
}

export function CopyValueButton(props: CopyValueButtonProps) {
  const merged = mergeProps(
    {
      variant: 'neutral' as CopyValueButtonVariant,
      size: 'md' as CopyValueButtonSize,
      type: 'button' as const,
    },
    props,
  );
  const [local, rest] = splitProps(merged, [
    'value',
    'copied',
    'onCopyValue',
    'label',
    'variant',
    'size',
    'class',
    'children',
    'disabled',
    'title',
    'aria-label',
  ]);
  const trimmedValue = () => (local.value ?? '').trim();

  return (
    <button
      {...rest}
      type="button"
      class={getCopyValueButtonClass({
        variant: local.variant,
        size: local.size,
        class: local.class,
      })}
      disabled={local.disabled || !trimmedValue()}
      onClick={() => {
        const value = trimmedValue();
        if (!value) return;
        void local.onCopyValue(value);
      }}
      title={local.title ?? local.label}
      aria-label={local['aria-label'] ?? local.label}
    >
      {local.children}
      <Show when={local.copied} fallback={<CopyIcon class="h-3.5 w-3.5" aria-hidden="true" />}>
        <CheckIcon class="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" aria-hidden="true" />
      </Show>
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
    'preserveOpener',
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
  const rel = () =>
    local.rel ??
    (local.target === '_blank' && !local.preserveOpener ? 'noopener noreferrer' : undefined);

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
