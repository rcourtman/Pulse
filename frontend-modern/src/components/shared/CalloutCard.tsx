import { mergeProps, splitProps, type JSX } from 'solid-js';
import { Card } from '@/components/shared/Card';

type CalloutTone = 'danger' | 'info' | 'success' | 'warning';
type CalloutScale = 'default' | 'compact';

interface CalloutCardProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'title'> {
  tone?: CalloutTone;
  scale?: CalloutScale;
  title?: JSX.Element;
  description?: JSX.Element;
  icon?: JSX.Element;
  padding?: 'sm' | 'md' | 'lg';
}

const toneClassByTone: Record<CalloutTone, string> = {
  danger:
    'border border-red-200 bg-red-50 text-red-900 dark:border-red-800 dark:bg-red-900 dark:text-red-100',
  info: 'border border-blue-200 bg-blue-50 text-blue-900 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-100',
  success:
    'border border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-800 dark:bg-emerald-900 dark:text-emerald-100',
  warning:
    'border border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-100',
};

const iconClassByTone: Record<CalloutTone, string> = {
  danger: 'bg-red-100 text-red-600 dark:bg-red-900 dark:text-red-300',
  info: 'bg-blue-100 text-blue-600 dark:bg-blue-900 dark:text-blue-300',
  success: 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900 dark:text-emerald-300',
  warning: 'bg-amber-100 text-amber-600 dark:bg-amber-900 dark:text-amber-300',
};

const titleClassByTone: Record<CalloutTone, string> = {
  danger: 'text-lg font-semibold text-red-900 dark:text-red-100',
  info: 'text-lg font-semibold text-blue-900 dark:text-blue-100',
  success: 'text-lg font-semibold text-emerald-900 dark:text-emerald-100',
  warning: 'text-lg font-semibold text-amber-900 dark:text-amber-100',
};

const descriptionClassByTone: Record<CalloutTone, string> = {
  danger: 'text-sm leading-relaxed text-red-800 dark:text-red-200',
  info: 'text-sm leading-relaxed text-blue-800 dark:text-blue-200',
  success: 'text-sm leading-relaxed text-emerald-800 dark:text-emerald-200',
  warning: 'text-sm leading-relaxed text-amber-800 dark:text-amber-200',
};

const layoutClassByScale: Record<CalloutScale, string> = {
  default: 'flex flex-col gap-4 sm:flex-row',
  compact: 'flex items-start gap-3',
};

const iconSizeClassByScale: Record<CalloutScale, string> = {
  default: 'rounded-md p-3',
  compact: 'rounded-md p-2',
};

const contentClassByScale: Record<CalloutScale, string> = {
  default: 'min-w-0 space-y-2',
  compact: 'min-w-0 space-y-1',
};

const titleClassByScaleAndTone: Record<CalloutScale, Record<CalloutTone, string>> = {
  default: titleClassByTone,
  compact: {
    danger: 'text-sm font-semibold text-red-900 dark:text-red-100',
    info: 'text-sm font-semibold text-blue-900 dark:text-blue-100',
    success: 'text-sm font-semibold text-emerald-900 dark:text-emerald-100',
    warning: 'text-sm font-semibold text-amber-900 dark:text-amber-100',
  },
};

const descriptionClassByScaleAndTone: Record<CalloutScale, Record<CalloutTone, string>> = {
  default: descriptionClassByTone,
  compact: {
    danger: 'text-xs leading-relaxed text-red-800 dark:text-red-200',
    info: 'text-xs leading-relaxed text-blue-800 dark:text-blue-200',
    success: 'text-xs leading-relaxed text-emerald-800 dark:text-emerald-200',
    warning: 'text-xs leading-relaxed text-amber-800 dark:text-amber-200',
  },
};

export function CalloutCard(props: CalloutCardProps) {
  const merged = mergeProps(
    { padding: 'md' as const, tone: 'info' as CalloutTone, scale: 'default' as CalloutScale },
    props,
  );
  const [local, rest] = splitProps(merged, [
    'tone',
    'scale',
    'padding',
    'title',
    'description',
    'icon',
    'children',
    'class',
  ]);

  return (
    <Card
      padding={local.padding}
      border={false}
      class={`${toneClassByTone[local.tone]} ${local.class ?? ''}`.trim()}
      {...rest}
    >
      <div class={layoutClassByScale[local.scale]}>
        <div
          classList={{ hidden: !local.icon }}
          class={`h-fit shrink-0 ${iconSizeClassByScale[local.scale]} ${iconClassByTone[local.tone]}`}
        >
          {local.icon}
        </div>
        <div class={contentClassByScale[local.scale]}>
          <div
            classList={{ hidden: !local.title }}
            class={titleClassByScaleAndTone[local.scale][local.tone]}
          >
            {local.title}
          </div>
          <div
            classList={{ hidden: !local.description }}
            class={descriptionClassByScaleAndTone[local.scale][local.tone]}
          >
            {local.description}
          </div>
          {local.children}
        </div>
      </div>
    </Card>
  );
}

export default CalloutCard;
