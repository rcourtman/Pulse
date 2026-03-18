import type { JSX } from 'solid-js';
import type { StatusIndicatorVariant } from '@/utils/status';

type StatusDotSize = 'xs' | 'sm' | 'md';

interface StatusDotProps {
  variant?: StatusIndicatorVariant;
  size?: StatusDotSize;
  pulse?: boolean;
  title?: string;
  ariaLabel?: string;
  ariaHidden?: boolean;
  class?: string;
}

const VARIANT_CLASSES: Record<StatusIndicatorVariant, string> = {
  success: 'bg-emerald-500 dark:bg-emerald-400',
  warning: 'bg-amber-500 dark:bg-amber-400',
  danger: 'bg-red-500 dark:bg-red-400',
  muted: 'bg-slate-400',
};

const SIZE_CLASSES: Record<StatusDotSize, string> = {
  xs: 'h-1.5 w-1.5',
  sm: 'h-2 w-2',
  md: 'h-2.5 w-2.5',
};

export function StatusDot(props: StatusDotProps): JSX.Element {
  // Use getters to maintain reactivity - props can change over time
  const variant = () => props.variant ?? 'muted';
  const size = () => props.size ?? 'sm';
  const ariaHidden = () => props.ariaHidden ?? !props.ariaLabel;

  const className = () =>
    [
      'inline-block rounded-full flex-shrink-0',
      SIZE_CLASSES[size()],
      VARIANT_CLASSES[variant()],
      props.pulse ? 'animate-pulse' : '',
      props.class ?? '',
    ]
      .filter(Boolean)
      .join(' ');

  return (
    <span
      class={className()}
      title={props.title}
      aria-label={props.ariaLabel}
      aria-hidden={ariaHidden()}
      role={props.ariaLabel ? 'img' : undefined}
    />
  );
}

export default StatusDot;
