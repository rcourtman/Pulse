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
  default: 'bg-surface',
  muted: 'bg-base',
  info: 'bg-blue-50 dark:bg-blue-900',
  success: 'bg-emerald-50 dark:bg-emerald-900',
  warning: 'bg-amber-50 dark:bg-amber-900',
  danger: 'bg-red-50 dark:bg-red-900',
  card: 'bg-surface',
  glass: 'bg-base',
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

  const baseClass = 'rounded-md transition-shadow duration-200 max-w-full';
  const toneClass = toneClassMap[local.tone];
  const paddingClass = paddingClassMap[local.padding];
  const borderClass = local.border ? 'border border-border' : '';
  const hoverClass = local.hoverable ? 'hover:bg-surface-hover' : '';

  return (
    <div
      class={`${baseClass} ${toneClass} ${paddingClass} ${borderClass} ${hoverClass} ${local.class ?? ''}`.trim()}
      {...rest}
    />
  );
}

export default Card;
