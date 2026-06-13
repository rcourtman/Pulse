import XIcon from 'lucide-solid/icons/x';
import { mergeProps, Show, splitProps, type JSX } from 'solid-js';
import { ActionIconButton } from './Button';

export type InlineNoticeTone = 'danger' | 'info' | 'success' | 'warning';
export type InlineNoticeLayout = 'inline' | 'banner';

interface InlineNoticeProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'class'> {
  tone?: InlineNoticeTone;
  layout?: InlineNoticeLayout;
  icon?: JSX.Element;
  actionHref?: string;
  actionLabel?: JSX.Element;
  actionIcon?: JSX.Element;
  actionAriaLabel?: string;
  actionOnClick?: () => void;
  onDismiss?: () => void;
  dismissLabel?: string;
  dismissTitle?: string;
  class?: string;
}

export const INLINE_NOTICE_BASE_CLASS = 'flex items-start gap-2 border px-3 py-2 text-sm';

export const INLINE_NOTICE_LAYOUT_CLASSES: Record<InlineNoticeLayout, string> = {
  inline: 'rounded-lg',
  banner: 'rounded-none border-x-0 border-t-0',
};

export const INLINE_NOTICE_TONE_CLASSES: Record<InlineNoticeTone, string> = {
  danger:
    'border-red-300 bg-red-50 text-red-800 dark:border-red-800/60 dark:bg-red-900/20 dark:text-red-200',
  info: 'border-blue-300 bg-blue-50 text-blue-800 dark:border-blue-800/60 dark:bg-blue-900/20 dark:text-blue-200',
  success:
    'border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-800/60 dark:bg-emerald-900/20 dark:text-emerald-200',
  warning:
    'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-200',
};

const INLINE_NOTICE_ICON_CLASS =
  'mt-0.5 inline-flex h-4 w-4 shrink-0 items-center justify-center [&>svg]:h-4 [&>svg]:w-4';

const INLINE_NOTICE_CONTENT_CLASS = 'min-w-0 flex-1 space-y-1';

const INLINE_NOTICE_ACTION_BASE_CLASS =
  'inline-flex items-center gap-1 text-xs font-semibold underline-offset-2 hover:underline';

export const INLINE_NOTICE_ACTION_TONE_CLASSES: Record<InlineNoticeTone, string> = {
  danger: 'text-red-900 dark:text-red-100',
  info: 'text-blue-900 dark:text-blue-100',
  success: 'text-emerald-900 dark:text-emerald-100',
  warning: 'text-amber-900 dark:text-amber-100',
};

const INLINE_NOTICE_ACTION_ICON_CLASS =
  'inline-flex h-3.5 w-3.5 items-center justify-center [&>svg]:h-3.5 [&>svg]:w-3.5';

export function InlineNotice(props: InlineNoticeProps) {
  const merged = mergeProps(
    { tone: 'warning' as InlineNoticeTone, layout: 'inline' as InlineNoticeLayout },
    props,
  );
  const [local, rest] = splitProps(merged, [
    'tone',
    'layout',
    'icon',
    'actionHref',
    'actionLabel',
    'actionIcon',
    'actionAriaLabel',
    'actionOnClick',
    'onDismiss',
    'dismissLabel',
    'dismissTitle',
    'children',
    'class',
  ]);

  return (
    <div
      class={`${INLINE_NOTICE_BASE_CLASS} ${INLINE_NOTICE_LAYOUT_CLASSES[local.layout]} ${
        INLINE_NOTICE_TONE_CLASSES[local.tone]
      } ${local.class ?? ''}`.trim()}
      {...rest}
    >
      <Show when={local.icon}>
        <span class={INLINE_NOTICE_ICON_CLASS} aria-hidden="true">
          {local.icon}
        </span>
      </Show>
      <div class={INLINE_NOTICE_CONTENT_CLASS}>
        <div>{local.children}</div>
        <Show when={local.actionLabel}>
          {(label) => (
            <Show
              when={local.actionHref}
              fallback={
                <Show when={local.actionOnClick}>
                  {(onClick) => (
                    <button
                      type="button"
                      aria-label={local.actionAriaLabel}
                      class={`${INLINE_NOTICE_ACTION_BASE_CLASS} ${
                        INLINE_NOTICE_ACTION_TONE_CLASSES[local.tone]
                      }`}
                      onClick={() => onClick()()}
                    >
                      <span>{label()}</span>
                      <Show when={local.actionIcon}>
                        <span class={INLINE_NOTICE_ACTION_ICON_CLASS} aria-hidden="true">
                          {local.actionIcon}
                        </span>
                      </Show>
                    </button>
                  )}
                </Show>
              }
            >
              {(href) => (
                <a
                  href={href()}
                  aria-label={local.actionAriaLabel}
                  class={`${INLINE_NOTICE_ACTION_BASE_CLASS} ${
                    INLINE_NOTICE_ACTION_TONE_CLASSES[local.tone]
                  }`}
                >
                  <span>{label()}</span>
                  <Show when={local.actionIcon}>
                    <span class={INLINE_NOTICE_ACTION_ICON_CLASS} aria-hidden="true">
                      {local.actionIcon}
                    </span>
                  </Show>
                </a>
              )}
            </Show>
          )}
        </Show>
      </div>
      <Show when={local.onDismiss}>
        {(onDismiss) => (
          <ActionIconButton
            type="button"
            label={local.dismissLabel ?? 'Dismiss notice'}
            title={local.dismissTitle ?? local.dismissLabel ?? 'Dismiss notice'}
            tone="muted"
            size="sm"
            class="-mr-1 -mt-1"
            onClick={() => onDismiss()()}
          >
            <XIcon class="h-4 w-4" aria-hidden="true" />
          </ActionIconButton>
        )}
      </Show>
    </div>
  );
}

export default InlineNotice;
