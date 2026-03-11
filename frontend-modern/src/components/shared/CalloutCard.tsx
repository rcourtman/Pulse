import { mergeProps, splitProps, type JSX } from 'solid-js';
import { Card } from '@/components/shared/Card';

type CalloutTone = 'danger' | 'info' | 'success' | 'warning';

interface CalloutCardProps extends JSX.HTMLAttributes<HTMLDivElement> {
  tone?: CalloutTone;
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

export function CalloutCard(props: CalloutCardProps) {
  const merged = mergeProps({ padding: 'md' as const, tone: 'info' as CalloutTone }, props);
  const [local, rest] = splitProps(merged, [
    'tone',
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
      <div class="flex flex-col gap-4 sm:flex-row">
        <div classList={{ hidden: !local.icon }} class={`h-fit shrink-0 rounded-md p-3 ${iconClassByTone[local.tone]}`}>
          {local.icon}
        </div>
        <div class="min-w-0 space-y-2">
          <div classList={{ hidden: !local.title }} class={titleClassByTone[local.tone]}>
            {local.title}
          </div>
          <div classList={{ hidden: !local.description }} class={descriptionClassByTone[local.tone]}>
            {local.description}
          </div>
          {local.children}
        </div>
      </div>
    </Card>
  );
}

export default CalloutCard;
