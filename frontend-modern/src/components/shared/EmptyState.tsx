import { JSX, Show, mergeProps, splitProps } from 'solid-js';
import { getEmptyStatePresentation, type EmptyStateTone } from '@/utils/emptyStatePresentation';

type EmptyStateProps = {
  icon?: JSX.Element;
  title: JSX.Element;
  description?: JSX.Element;
  actions?: JSX.Element;
  tone?: EmptyStateTone;
  align?: 'center' | 'left';
  variant?: 'framed' | 'panel';
} & JSX.HTMLAttributes<HTMLDivElement>;

export function EmptyState(props: EmptyStateProps) {
  const merged = mergeProps(
    { tone: 'default' as EmptyStateTone, align: 'center' as const, variant: 'framed' as const },
    props,
  );
  const [local, others] = splitProps(merged, [
    'icon',
    'title',
    'description',
    'actions',
    'tone',
    'align',
    'variant',
    'class',
  ]);

  const alignment = local.align;
  const tone = local.tone;
  const variant = local.variant;
  const presentation = getEmptyStatePresentation(tone);
  const containerClass = [
    'flex w-full animate-fade-in flex-col',
    variant === 'framed'
      ? 'rounded-md border border-dashed border-border bg-base px-6 py-10 sm:px-8 sm:py-16'
      : 'px-4 py-8',
    alignment === 'center' ? 'items-center text-center' : 'items-start text-left',
    local.class ?? '',
  ]
    .join(' ')
    .trim();
  const iconClass =
    variant === 'framed'
      ? `mb-6 flex h-16 w-16 items-center justify-center rounded-md sm:h-20 sm:w-20 ${presentation.iconClass}`
      : `mb-3 flex h-12 w-12 items-center justify-center rounded-md ${presentation.iconClass}`;
  const iconInnerClass = variant === 'framed' ? 'scale-125' : '';
  const titleClass =
    variant === 'framed'
      ? `mb-2 text-lg font-bold tracking-tight sm:text-xl ${presentation.titleClass}`
      : `mb-1 text-sm font-semibold ${presentation.titleClass}`;
  const descriptionClass =
    variant === 'framed'
      ? `mb-6 max-w-sm text-sm leading-relaxed sm:max-w-md ${presentation.descriptionClass}`
      : `mb-4 max-w-xl text-sm leading-6 ${presentation.descriptionClass}`;

  return (
    <div class={containerClass} {...others}>
      <Show when={local.icon}>
        <div class={iconClass}>
          <div class={iconInnerClass}>{local.icon}</div>
        </div>
      </Show>

      <h3 class={titleClass}>{local.title}</h3>

      <Show when={local.description}>
        <p class={descriptionClass}>{local.description}</p>
      </Show>

      <Show when={local.actions}>
        <div
          class={
            alignment === 'center'
              ? 'flex flex-col items-center justify-center w-full gap-3'
              : 'flex flex-col items-start w-full gap-3'
          }
        >
          {local.actions}
        </div>
      </Show>
    </div>
  );
}

export default EmptyState;
