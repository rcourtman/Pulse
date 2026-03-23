import type { JSX } from 'solid-js';

export interface FilterOption<T extends string | number> {
  value: T;
  label: string;
  icon?: (props: { class?: string }) => JSX.Element;
  disabled?: boolean;
}

export type FilterButtonGroupVariant = 'default' | 'settings' | 'prominent';

export interface FilterButtonGroupProps<T extends string | number> {
  options: FilterOption<T>[];
  value: T;
  onChange: (value: T) => void;
  class?: string;
  variant?: FilterButtonGroupVariant;
  disabled?: boolean;
}

const groupClassByVariant: Record<FilterButtonGroupVariant, string> = {
  default: 'flex p-1 space-x-1 bg-surface-alt rounded-md overflow-x-auto scrollbar-hide',
  settings: 'flex items-center gap-1 bg-surface-alt rounded-md p-1 overflow-x-auto scrollbar-hide',
  prominent: 'grid grid-cols-1 gap-2',
};

export function resolveFilterButtonGroupVariant(
  variant: FilterButtonGroupVariant | undefined,
): FilterButtonGroupVariant {
  return variant ?? 'default';
}

export function getFilterButtonGroupClass(
  variant: FilterButtonGroupVariant,
  className?: string,
): string {
  return `${groupClassByVariant[variant]} touch-scroll ${className ?? ''}`.trim();
}

export function getFilterButtonGroupButtonClass(
  variant: FilterButtonGroupVariant,
  active: boolean,
  disabled: boolean,
): string {
  if (variant === 'settings') {
    return [
      'flex items-center justify-center gap-1.5 min-h-10 sm:min-h-9 px-3 py-2 text-sm rounded-md transition-all whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2',
      active ? 'bg-surface text-base-content shadow-sm' : 'text-muted hover:text-base-content',
      disabled ? 'opacity-60 cursor-not-allowed' : '',
    ].join(' ');
  }

  if (variant === 'prominent') {
    return [
      'w-full flex items-center justify-center gap-2 min-h-10 rounded-md border px-4 py-2.5 text-sm font-medium transition-all whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2',
      active
        ? 'bg-blue-50 border-blue-500 text-blue-700 dark:bg-blue-900 dark:text-blue-300 dark:border-blue-500'
        : 'border-border text-base-content hover:bg-surface-alt',
      disabled ? 'opacity-60 cursor-not-allowed' : '',
    ].join(' ');
  }

  return [
    'flex flex-1 justify-center sm:flex-none sm:justify-start items-center gap-2 px-3 sm:px-4 py-2.5 sm:py-2 text-sm font-medium rounded-md transition-all whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2',
    active
      ? 'bg-surface border border-border text-blue-600 dark:text-blue-400 shadow-sm'
      : 'text-muted border border-transparent hover:text-base-content hover:bg-surface-hover',
    disabled ? 'opacity-60 cursor-not-allowed' : '',
  ].join(' ');
}

export function getFilterButtonGroupCompactLabel(label: string): string {
  return label.split(' ').pop() ?? label;
}
