import { JSX, Show, mergeProps, splitProps } from 'solid-js';
import { getEmptyStatePresentation, type EmptyStateTone } from '@/utils/emptyStatePresentation';

type EmptyStateProps = {
  icon?: JSX.Element;
  title: JSX.Element;
  description?: JSX.Element;
  actions?: JSX.Element;
  tone?: EmptyStateTone;
  align?: 'center' | 'left';
} & JSX.HTMLAttributes<HTMLDivElement>;

export function EmptyState(props: EmptyStateProps) {
  const merged = mergeProps({ tone: 'default' as EmptyStateTone, align: 'center' as const }, props);
  const [local, others] = splitProps(merged, [
    'icon',
    'title',
    'description',
    'actions',
    'tone',
    'align',
    'class',
  ]);

  const alignment = local.align;
  const tone = local.tone;
  const presentation = getEmptyStatePresentation(tone);
  const containerClass = [
    'flex flex-col py-10 px-6 sm:py-16 sm:px-8 w-full animate-fade-in',
    'bg-base border border-dashed border-border rounded-md',
    alignment === 'center' ? 'items-center text-center' : 'items-start text-left',
    local.class ?? '',
  ]
    .join(' ')
    .trim();

  return (
    <div class={containerClass} {...others}>
      <Show when={local.icon}>
        <div
          class={`w-16 h-16 sm:w-20 sm:h-20 mb-6 rounded-md flex items-center justify-center ${presentation.iconClass}`}
        >
          <div class="scale-125">{local.icon}</div>
        </div>
      </Show>

      <h3 class={`text-lg sm:text-xl font-bold tracking-tight mb-2 ${presentation.titleClass}`}>
        {local.title}
      </h3>

      <Show when={local.description}>
        <p
          class={`text-sm max-w-sm sm:max-w-md ${presentation.descriptionClass} mb-6 leading-relaxed`}
        >
          {local.description}
        </p>
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
