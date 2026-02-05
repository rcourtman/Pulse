import { JSX, splitProps, mergeProps } from 'solid-js';

type Tone = 'default' | 'muted' | 'info' | 'success' | 'warning' | 'danger' | 'glass';
type Padding = 'none' | 'sm' | 'md' | 'lg';

type CardProps = {
  tone?: Tone;
  padding?: Padding;
  hoverable?: boolean;
  border?: boolean;
} & JSX.HTMLAttributes<HTMLDivElement>;

const toneClassMap: Record<Tone, string> = {
  default: 'bg-white dark:bg-gray-800',
  muted: 'bg-gray-50 dark:bg-gray-800/80',
  info: 'bg-blue-50/70 dark:bg-blue-900/20',
  success: 'bg-green-50/70 dark:bg-green-900/20',
  warning: 'bg-amber-50/80 dark:bg-amber-900/20',
  danger: 'bg-red-50/80 dark:bg-red-900/20',
  glass: 'glass',
};

const paddingClassMap: Record<Padding, string> = {
  none: 'p-0',
  sm: 'p-2 sm:p-3',
  md: 'p-3 sm:p-4',
  lg: 'p-4 sm:p-6',
};

export function Card(props: CardProps) {
  const merged = mergeProps(
    { tone: 'default' as Tone, padding: 'md' as Padding, hoverable: false, border: true },
    props,
  );
  const [local, rest] = splitProps(merged, ['tone', 'padding', 'hoverable', 'border', 'class']);

  const baseClass = 'rounded-lg shadow-sm transition-shadow duration-200 max-w-full';
  const toneClass = toneClassMap[local.tone];
  const paddingClass = paddingClassMap[local.padding];
  const borderClass = local.border ? 'border border-gray-200 dark:border-gray-700' : '';
  const hoverClass = local.hoverable ? 'hover:shadow-md' : '';

  return (
    <div
      class={`${baseClass} ${toneClass} ${paddingClass} ${borderClass} ${hoverClass} ${local.class ?? ''}`.trim()}
      {...rest}
    />
  );
}

export default Card;
