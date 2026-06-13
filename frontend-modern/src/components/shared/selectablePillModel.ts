export type SelectablePillButtonSize = 'md';

export const SELECTABLE_PILL_BUTTON_BASE_CLASS =
  'inline-flex items-center justify-center rounded-full border font-semibold transition whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60';

export const SELECTABLE_PILL_BUTTON_SIZE_CLASSES: Record<SelectablePillButtonSize, string> = {
  md: 'min-h-10 px-3 py-2 text-sm sm:min-h-10',
};

export const SELECTABLE_PILL_BUTTON_ACTIVE_CLASS =
  'border-blue-500 bg-blue-600 text-white shadow-sm';

export const SELECTABLE_PILL_BUTTON_INACTIVE_CLASS =
  'border-border bg-surface text-base-content hover:border-blue-400 hover:text-blue-600 dark:hover:border-blue-400 dark:hover:text-blue-200';

export type SelectablePillButtonClassOptions = {
  active?: boolean;
  size?: SelectablePillButtonSize;
  class?: string;
};

export const getSelectablePillButtonClass = (
  options: SelectablePillButtonClassOptions = {},
): string =>
  [
    SELECTABLE_PILL_BUTTON_BASE_CLASS,
    SELECTABLE_PILL_BUTTON_SIZE_CLASSES[options.size ?? 'md'],
    options.active ? SELECTABLE_PILL_BUTTON_ACTIVE_CLASS : SELECTABLE_PILL_BUTTON_INACTIVE_CLASS,
    options.class,
  ]
    .filter(Boolean)
    .join(' ');
