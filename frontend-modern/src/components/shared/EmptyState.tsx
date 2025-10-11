import { JSX, Show, mergeProps, splitProps } from 'solid-js';
import { SectionHeader } from '@/components/shared/SectionHeader';

type EmptyStateTone = 'default' | 'info' | 'success' | 'warning' | 'danger';

type EmptyStateProps = {
  icon?: JSX.Element;
  title: JSX.Element;
  description?: JSX.Element;
  actions?: JSX.Element;
  tone?: EmptyStateTone;
  align?: 'center' | 'left';
} & JSX.HTMLAttributes<HTMLDivElement>;

const titleToneClass: Record<EmptyStateTone, string> = {
  default: '',
  info: 'text-blue-700 dark:text-blue-300',
  success: 'text-green-700 dark:text-green-300',
  warning: 'text-amber-700 dark:text-amber-300',
  danger: 'text-red-700 dark:text-red-300',
};

const descriptionToneClass: Record<EmptyStateTone, string> = {
  default: '',
  info: 'text-blue-600 dark:text-blue-300',
  success: 'text-green-600 dark:text-green-300',
  warning: 'text-amber-600 dark:text-amber-300',
  danger: 'text-red-600 dark:text-red-300',
};

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
  const containerClass = [
    'flex flex-col gap-3',
    alignment === 'center' ? 'items-center text-center' : 'items-start text-left',
    local.class ?? '',
  ]
    .join(' ')
    .trim();

  return (
    <div class={containerClass} {...others}>
      <Show when={local.icon}>
        <div class={alignment === 'center' ? 'flex justify-center' : ''}>{local.icon}</div>
      </Show>
      <SectionHeader
        align={alignment}
        title={local.title}
        description={local.description}
        size={alignment === 'center' ? 'sm' : 'md'}
        class={alignment === 'center' ? 'items-center' : 'items-start'}
        titleClass={titleToneClass[tone]}
        descriptionClass={`text-xs ${descriptionToneClass[tone]}`.trim()}
      />
      <Show when={local.actions}>
        <div
          class={
            alignment === 'center'
              ? 'mt-2 flex flex-col items-center gap-2'
              : 'mt-2 flex flex-col gap-2'
          }
        >
          {local.actions}
        </div>
      </Show>
    </div>
  );
}

export default EmptyState;
