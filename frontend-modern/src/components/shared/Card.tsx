import { JSX, splitProps, mergeProps } from 'solid-js';

type Tone = 'default' | 'muted' | 'info' | 'success' | 'warning' | 'danger' | 'card' | 'glass';
type Padding = 'none' | 'sm' | 'md' | 'lg';

type CardProps = {
  tone?: Tone;
  padding?: Padding;
  hoverable?: boolean;
  border?: boolean;
} & JSX.HTMLAttributes<HTMLDivElement>;

const toneClassMap: Record<Tone, string> = {
  default: 'bg-white dark:bg-slate-800',
  muted: 'bg-slate-50 dark:bg-slate-900',
  info: 'bg-blue-50 dark:bg-blue-900',
  success: 'bg-emerald-50 dark:bg-emerald-900',
  warning: 'bg-amber-50 dark:bg-amber-900',
  danger: 'bg-red-50 dark:bg-red-900',
  card: 'bg-white dark:bg-slate-800 shadow-sm',
  glass: 'bg-slate-50 dark:bg-slate-900',
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

  const baseClass = 'rounded-md shadow-sm transition-shadow duration-200 max-w-full';
  const toneClass = toneClassMap[local.tone];
  const paddingClass = paddingClassMap[local.padding];
  const borderClass = local.border ? 'border border-slate-200 dark:border-slate-700' : '';
  const hoverClass = local.hoverable ? 'hover:shadow-sm' : '';

  return (
    <div
      class={`${baseClass} ${toneClass} ${paddingClass} ${borderClass} ${hoverClass} ${local.class ?? ''}`.trim()}
      {...rest}
    />
  );
}

export default Card;
