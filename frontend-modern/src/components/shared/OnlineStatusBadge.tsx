import { Component } from 'solid-js';
import type { StatusIndicatorVariant } from '@/utils/status';

interface OnlineStatusBadgeProps {
  variant: StatusIndicatorVariant;
  label: string;
  class?: string;
}

const variantClasses: Record<StatusIndicatorVariant, string> = {
  success: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
  warning: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
  danger: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
  muted: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400',
};

/**
 * OnlineStatusBadge - A pill-shaped badge showing online/offline/degraded status
 * Used in tables to show resource status (nodes, guests, containers, etc.)
 */
export const OnlineStatusBadge: Component<OnlineStatusBadgeProps> = (props) => {
  return (
    <span
      class={`inline-flex items-center justify-center rounded-full px-2 py-0.5 text-[10px] font-semibold whitespace-nowrap ${variantClasses[props.variant]} ${props.class || ''}`}
    >
      {props.label}
    </span>
  );
};
