import type { JSX } from 'solid-js';

export type SelectionCardTone = 'accent' | 'success';
export type SelectionCardGroupVariant = 'compact' | 'detail';

export interface SelectionCardOption<T extends string | number> {
  value: T;
  title: string;
  description?: string;
  icon?: (props: { active: boolean }) => JSX.Element;
  tone?: SelectionCardTone;
  disabled?: boolean;
}

export interface SelectionCardGroupProps<T extends string | number> {
  options: SelectionCardOption<T>[];
  value: T;
  onChange: (value: T) => void;
  class?: string;
  variant?: SelectionCardGroupVariant;
  disabled?: boolean;
}

const groupClassByVariant: Record<SelectionCardGroupVariant, string> = {
  compact: 'grid grid-cols-2 gap-2',
  detail: 'grid grid-cols-1 gap-3',
};

export function resolveSelectionCardGroupVariant(
  variant: SelectionCardGroupVariant | undefined,
): SelectionCardGroupVariant {
  return variant ?? 'detail';
}

export function resolveSelectionCardTone(tone: SelectionCardTone | undefined): SelectionCardTone {
  return tone ?? 'accent';
}

export function getSelectionCardGroupClass(
  variant: SelectionCardGroupVariant,
  className?: string,
): string {
  return `${groupClassByVariant[variant]} ${className ?? ''}`.trim();
}

function getSelectionCardActiveClass(tone: SelectionCardTone): string {
  if (tone === 'success') {
    return 'border-green-500 bg-green-50 dark:bg-green-900';
  }
  return 'border-blue-500 bg-blue-50 dark:bg-blue-900';
}

function getSelectionCardInactiveClass(variant: SelectionCardGroupVariant): string {
  if (variant === 'compact') {
    return 'border-border hover:border-blue-300';
  }
  return 'border-border hover:border-border';
}

export function getSelectionCardButtonClass(
  variant: SelectionCardGroupVariant,
  tone: SelectionCardTone,
  active: boolean,
  disabled: boolean,
): string {
  const base =
    variant === 'detail'
      ? 'p-4 rounded-md border-2 transition-all text-left'
      : 'p-3 rounded-md border-2 transition-all text-center';

  return [
    base,
    active ? getSelectionCardActiveClass(tone) : getSelectionCardInactiveClass(variant),
    disabled ? 'disabled:opacity-50 disabled:cursor-not-allowed' : '',
  ].join(' ');
}

export function getSelectionCardIconContainerClass(
  tone: SelectionCardTone,
  active: boolean,
): string {
  const activeClass =
    tone === 'success' ? 'bg-green-100 dark:bg-green-800' : 'bg-blue-100 dark:bg-blue-800';
  return ['p-2 rounded-md', active ? activeClass : 'bg-surface-alt'].join(' ');
}

export function getSelectionCardTitleClass(
  variant: SelectionCardGroupVariant,
  tone: SelectionCardTone,
  active: boolean,
): string {
  if (variant === 'compact') {
    return 'text-sm font-medium text-base-content';
  }
  if (!active) {
    return 'text-sm font-semibold text-base-content';
  }
  return tone === 'success'
    ? 'text-sm font-semibold text-green-900 dark:text-green-100'
    : 'text-sm font-semibold text-blue-900 dark:text-blue-100';
}

export function getSelectionCardDescriptionClass(variant: SelectionCardGroupVariant): string {
  return variant === 'compact' ? 'text-xs text-slate-500 mt-0.5' : 'text-xs text-muted';
}
