import { A } from '@solidjs/router';
import CheckIcon from 'lucide-solid/icons/check';
import CopyIcon from 'lucide-solid/icons/copy';
import { JSX, Show, mergeProps, splitProps } from 'solid-js';
import {
  getActionIconButtonClass,
  getButtonClass,
  getCopyValueButtonClass,
  getDrawerHeaderActionButtonClass,
  getDrawerHeaderActionGroupClass,
  getDrawerHeaderIconButtonClass,
  type ActionIconButtonSize,
  type ActionIconButtonTone,
  type ButtonSize,
  type ButtonVariant,
  type CopyValueButtonSize,
  type CopyValueButtonVariant,
} from './buttonModel';
import { LoadingSpinner } from './LoadingSpinner';

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
  stopPropagation?: boolean;
  class?: string;
  children?: JSX.Element;
}

export interface ActionIconButtonProps extends Omit<
  JSX.ButtonHTMLAttributes<HTMLButtonElement>,
  'children'
> {
  label: string;
  tone?: ActionIconButtonTone;
  size?: ActionIconButtonSize;
  class?: string;
  children: JSX.Element;
}

export interface DrawerHeaderActionGroupProps extends JSX.HTMLAttributes<HTMLDivElement> {
  class?: string;
  children?: JSX.Element;
}

export interface DrawerHeaderActionButtonProps extends JSX.ButtonHTMLAttributes<HTMLButtonElement> {
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
        <LoadingSpinner size="md" tone="current" class="-ml-1 mr-2" />
      ) : null}
      {local.children}
    </button>
  );
}

export function DrawerHeaderActionGroup(props: DrawerHeaderActionGroupProps) {
  const [local, rest] = splitProps(props, ['class', 'children']);

  return (
    <div {...rest} class={getDrawerHeaderActionGroupClass(local.class)}>
      {local.children}
    </div>
  );
}

export function DrawerHeaderActionButton(props: DrawerHeaderActionButtonProps) {
  const merged = mergeProps({ type: 'button' as const }, props);
  const [local, rest] = splitProps(merged, ['class', 'children']);

  return (
    <button {...rest} class={getDrawerHeaderActionButtonClass(local.class)}>
      {local.children}
    </button>
  );
}

export function DrawerHeaderIconButton(props: DrawerHeaderActionButtonProps) {
  const merged = mergeProps({ type: 'button' as const }, props);
  const [local, rest] = splitProps(merged, ['class', 'children']);

  return (
    <button {...rest} class={getDrawerHeaderIconButtonClass(local.class)}>
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
    'stopPropagation',
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
      onClick={(event) => {
        if (local.stopPropagation) event.stopPropagation();
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

export function ActionIconButton(props: ActionIconButtonProps) {
  const merged = mergeProps(
    {
      tone: 'neutral' as ActionIconButtonTone,
      size: 'sm' as ActionIconButtonSize,
      type: 'button' as const,
    },
    props,
  );
  const [local, rest] = splitProps(merged, [
    'label',
    'tone',
    'size',
    'class',
    'children',
    'title',
    'aria-label',
  ]);

  return (
    <button
      {...rest}
      class={getActionIconButtonClass({
        tone: local.tone,
        size: local.size,
        class: local.class,
      })}
      title={local.title ?? local.label}
      aria-label={local['aria-label'] ?? local.label}
    >
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
