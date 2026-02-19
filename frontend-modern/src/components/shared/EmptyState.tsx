import { JSX, Show, mergeProps, splitProps } from 'solid-js';

type EmptyStateTone = 'default' | 'info' | 'success' | 'warning' | 'danger';

type EmptyStateProps = {
  icon?: JSX.Element;
  title: JSX.Element;
  description?: JSX.Element;
  actions?: JSX.Element;
  tone?: EmptyStateTone;
  align?: 'center' | 'left';
} & JSX.HTMLAttributes<HTMLDivElement>;

const iconBgClass: Record<EmptyStateTone, string> = {
  default: 'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400',
  info: 'bg-blue-50 dark:bg-blue-900 text-blue-500',
  success: 'bg-green-50 dark:bg-green-900 text-green-500',
  warning: 'bg-amber-50 dark:bg-amber-900 text-amber-500',
  danger: 'bg-red-50 dark:bg-red-900 text-red-500',
};

const titleToneClass: Record<EmptyStateTone, string> = {
  default: 'text-slate-900 dark:text-white',
  info: 'text-blue-700 dark:text-blue-300',
  success: 'text-green-700 dark:text-green-300',
  warning: 'text-amber-700 dark:text-amber-300',
  danger: 'text-red-700 dark:text-red-300',
};

const descriptionToneClass: Record<EmptyStateTone, string> = {
  default: 'text-slate-500 dark:text-slate-400',
  info: 'text-blue-600/80 dark:text-blue-300/80',
  success: 'text-green-600/80 dark:text-green-300/80',
  warning: 'text-amber-600/80 dark:text-amber-300/80',
  danger: 'text-red-600/80 dark:text-red-300/80',
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
    'flex flex-col py-10 px-6 sm:py-16 sm:px-8 w-full animate-fade-in',
    alignment === 'center' ? 'items-center text-center' : 'items-start text-left',
    local.class ?? '',
  ]
    .join(' ')
    .trim();

  return (
    <div class={containerClass} {...others}>
      <Show when={local.icon}>
        <div class={`w-16 h-16 sm:w-20 sm:h-20 mb-6 rounded-lg flex items-center justify-center ${iconBgClass[tone]}`}>
          <div class="scale-125">
            {local.icon}
          </div>
        </div>
      </Show>

      <h3 class={`text-lg sm:text-xl font-bold tracking-tight mb-2 ${titleToneClass[tone]}`}>
        {local.title}
      </h3>

      <Show when={local.description}>
        <p class={`text-sm max-w-sm sm:max-w-md ${descriptionToneClass[tone]} mb-6 leading-relaxed`}>
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
