import type { JSX } from 'solid-js';

export interface FilterOption<T extends string | number> {
  value: T;
  label: string;
  ariaLabel?: string;
  title?: string;
  compactLabel?: string;
  leading?: JSX.Element;
  visualLabel?: JSX.Element;
  icon?: (props: { class?: string }) => JSX.Element;
  tone?: FilterButtonGroupOptionTone;
  disabled?: boolean;
}

export type FilterButtonGroupVariant =
  'default' | 'settings' | 'compact' | 'prominent' | 'segmented';
export type FilterButtonGroupOptionTone =
  'default' | 'info' | 'success' | 'warning' | 'danger' | 'muted';

export interface FilterButtonGroupProps<T extends string | number> extends Omit<
  JSX.HTMLAttributes<HTMLDivElement>,
  'onChange'
> {
  options: FilterOption<T>[];
  value: T;
  onChange: (value: T) => void;
  ariaLabel?: string;
  label?: JSX.Element;
  variant?: FilterButtonGroupVariant;
  disabled?: boolean;
}

const groupClassByVariant: Record<FilterButtonGroupVariant, string> = {
  default: 'flex p-1 space-x-1 bg-surface-alt rounded-md overflow-x-auto scrollbar-hide',
  settings: 'flex items-center gap-1 bg-surface-alt rounded-md p-1 overflow-x-auto scrollbar-hide',
  compact:
    'inline-flex items-center gap-1 bg-surface-hover rounded-md p-0.5 ring-1 ring-border-subtle overflow-x-auto scrollbar-hide',
  prominent: 'grid grid-cols-1 gap-2',
  segmented:
    'flex items-center gap-1 rounded-md border border-border bg-base p-1 shadow-inner overflow-x-auto scrollbar-hide',
};

const labelClassByVariant: Partial<Record<FilterButtonGroupVariant, string>> = {
  compact: 'px-1.5 text-[9px] font-semibold uppercase tracking-wide text-muted',
  settings: 'px-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted',
  default: 'px-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted',
};

const activeToneClassByOptionTone: Record<FilterButtonGroupOptionTone, string> = {
  default: 'text-base-content ring-border-subtle',
  info: 'text-blue-700 dark:text-blue-300 ring-blue-200 dark:ring-blue-800',
  success: 'text-emerald-700 dark:text-emerald-300 ring-emerald-200 dark:ring-emerald-800',
  warning: 'text-amber-700 dark:text-amber-300 ring-amber-200 dark:ring-amber-800',
  danger: 'text-red-700 dark:text-red-300 ring-red-200 dark:ring-red-800',
  muted: 'text-muted ring-border-subtle',
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

export function getFilterButtonGroupLabelClass(variant: FilterButtonGroupVariant): string {
  return labelClassByVariant[variant] ?? labelClassByVariant.default!;
}

export function getFilterButtonGroupButtonClass(
  variant: FilterButtonGroupVariant,
  active: boolean,
  disabled: boolean,
  tone: FilterButtonGroupOptionTone = 'default',
): string {
  if (variant === 'settings') {
    return [
      'flex min-h-11 items-center justify-center gap-1.5 px-3 py-2 text-sm rounded-md transition-all whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 sm:min-h-9',
      active ? 'bg-surface text-base-content shadow-sm' : 'text-muted hover:text-base-content',
      disabled ? 'opacity-60 cursor-not-allowed' : '',
    ].join(' ');
  }

  if (variant === 'prominent') {
    return [
      'flex min-h-11 w-full items-center justify-center gap-2 rounded-md border px-4 py-2.5 text-sm font-medium transition-all whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 sm:min-h-10',
      active
        ? 'bg-blue-50 border-blue-500 text-blue-700 dark:bg-blue-900 dark:text-blue-300 dark:border-blue-500'
        : 'border-border text-base-content hover:bg-surface-alt',
      disabled ? 'opacity-60 cursor-not-allowed' : '',
    ].join(' ');
  }

  if (variant === 'compact') {
    return [
      'inline-flex min-h-11 min-w-11 items-center justify-center gap-1.5 rounded px-2.5 py-1 text-xs font-medium transition-all whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 sm:min-h-0 sm:min-w-0',
      active
        ? `bg-surface shadow-sm ring-1 ${activeToneClassByOptionTone[tone]}`
        : 'text-muted hover:bg-surface-hover hover:text-base-content',
      disabled ? 'opacity-60 cursor-not-allowed' : '',
    ].join(' ');
  }

  if (variant === 'segmented') {
    return [
      'min-h-11 flex-1 rounded-md px-2 py-1.5 text-xs font-semibold transition-all whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 sm:min-h-8',
      active
        ? 'bg-surface text-blue-600 shadow-sm dark:text-blue-400'
        : 'text-muted hover:text-base-content hover:bg-surface-hover',
      disabled ? 'opacity-50 cursor-not-allowed' : '',
    ].join(' ');
  }

  return [
    'flex min-h-11 flex-1 items-center justify-center gap-2 rounded-md px-3 py-2.5 text-sm font-medium transition-all whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 sm:min-h-0 sm:flex-none sm:justify-start sm:px-4 sm:py-2',
    active
      ? 'bg-surface border border-border text-blue-600 dark:text-blue-400 shadow-sm'
      : 'text-muted border border-transparent hover:text-base-content hover:bg-surface-hover',
    disabled ? 'opacity-60 cursor-not-allowed' : '',
  ].join(' ');
}

export function getFilterButtonGroupCompactLabel(
  option: Pick<FilterOption<string | number>, 'label' | 'compactLabel'>,
): string {
  if (option.compactLabel) return option.compactLabel;
  if (option.label.startsWith('All ')) return 'All';
  return option.label.split(' ').pop() ?? option.label;
}
