export type ButtonVariant =
  | 'primary'
  | 'primaryFlat'
  | 'warning'
  | 'success'
  | 'successOutline'
  | 'successGhost'
  | 'secondary'
  | 'danger'
  | 'dangerOutline'
  | 'ghost'
  | 'outline';
export type ButtonSize =
  | 'xs'
  | 'sm'
  | 'mdCompact'
  | 'settingsAction'
  | 'settingsActionXs'
  | 'md'
  | 'lg'
  | 'icon'
  | 'iconMd';

export const BUTTON_BASE_CLASS =
  'inline-flex items-center justify-center rounded-md font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed';

export const BUTTON_VARIANT_CLASSES: Record<ButtonVariant, string> = {
  primary: 'border border-transparent bg-blue-600 text-white shadow-sm hover:bg-blue-700',
  primaryFlat: 'border border-transparent bg-blue-600 text-white hover:bg-blue-700',
  warning:
    'border border-amber-300 bg-amber-100 text-amber-800 hover:bg-amber-200 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-100 dark:hover:bg-amber-800',
  success: 'border border-transparent bg-emerald-600 text-white shadow-sm hover:bg-emerald-700',
  successOutline:
    'border border-emerald-300 bg-white text-emerald-900 hover:bg-emerald-100 dark:border-emerald-700 dark:bg-emerald-950 dark:text-emerald-100 dark:hover:bg-emerald-800',
  successGhost:
    'border border-transparent bg-transparent text-emerald-900 hover:bg-emerald-100 dark:text-emerald-100 dark:hover:bg-emerald-800',
  secondary: 'border border-border bg-surface text-base-content shadow-sm hover:bg-surface-hover',
  danger: 'border border-transparent bg-rose-600 text-white shadow-sm hover:bg-rose-700',
  dangerOutline:
    'border border-rose-300 bg-transparent text-rose-700 hover:bg-rose-50 dark:border-rose-900 dark:text-rose-300 dark:hover:bg-rose-950',
  ghost: 'border border-transparent bg-transparent text-base-content hover:bg-surface-hover',
  outline: 'border border-border bg-transparent text-base-content hover:bg-surface-hover',
};

export const BUTTON_SIZE_CLASSES: Record<ButtonSize, string> = {
  xs: 'px-2.5 py-1 text-xs',
  sm: 'px-2.5 py-1.5 text-xs',
  mdCompact: 'px-3 py-2 text-sm',
  settingsAction: 'min-h-10 px-3 py-2 text-sm sm:min-h-9',
  settingsActionXs: 'min-h-10 px-3 py-2 text-xs sm:min-h-9',
  md: 'px-4 py-2 text-sm',
  lg: 'px-6 py-3 text-base',
  icon: 'p-2',
  iconMd: 'h-9 w-9 p-0',
};

export type ButtonClassOptions = {
  variant?: ButtonVariant;
  size?: ButtonSize;
  class?: string;
};

export const getButtonClass = (options: ButtonClassOptions = {}): string =>
  [
    BUTTON_BASE_CLASS,
    BUTTON_VARIANT_CLASSES[options.variant ?? 'secondary'],
    BUTTON_SIZE_CLASSES[options.size ?? 'md'],
    options.class,
  ]
    .filter(Boolean)
    .join(' ');

export type CopyValueButtonVariant = 'neutral' | 'ghost' | 'accent' | 'chip';
export type CopyValueButtonSize = 'xs' | 'sm' | 'md' | 'lg' | 'chip';

export const COPY_VALUE_BUTTON_BASE_CLASS =
  'inline-flex shrink-0 items-center justify-center rounded transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50';

export const COPY_VALUE_BUTTON_VARIANT_CLASSES: Record<CopyValueButtonVariant, string> = {
  neutral:
    'border border-border bg-surface text-muted hover:bg-surface-hover hover:text-base-content',
  ghost: 'text-muted hover:bg-surface-hover hover:text-base-content',
  accent: 'text-blue-700 hover:bg-blue-100 dark:text-blue-200 dark:hover:bg-blue-950',
  chip: 'bg-surface-alt text-base-content hover:bg-surface-hover',
};

export const COPY_VALUE_BUTTON_SIZE_CLASSES: Record<CopyValueButtonSize, string> = {
  xs: 'min-h-5 min-w-5',
  sm: 'min-h-6 min-w-6',
  md: 'min-h-7 min-w-7',
  lg: 'min-h-8 min-w-8 rounded-md',
  chip: 'gap-1 px-1.5 py-0.5 text-[10px]',
};

export type CopyValueButtonClassOptions = {
  variant?: CopyValueButtonVariant;
  size?: CopyValueButtonSize;
  class?: string;
};

export const getCopyValueButtonClass = (options: CopyValueButtonClassOptions = {}): string =>
  [
    COPY_VALUE_BUTTON_BASE_CLASS,
    COPY_VALUE_BUTTON_VARIANT_CLASSES[options.variant ?? 'neutral'],
    COPY_VALUE_BUTTON_SIZE_CLASSES[options.size ?? 'md'],
    options.class,
  ]
    .filter(Boolean)
    .join(' ');

export const DRAWER_HEADER_ACTION_GROUP_CLASS = 'flex shrink-0 items-center gap-1.5';

export const DRAWER_HEADER_ACTION_BUTTON_CLASS =
  'inline-flex h-8 items-center gap-1.5 rounded border border-border bg-surface px-2 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-wait disabled:opacity-60';

export const DRAWER_HEADER_ICON_BUTTON_CLASS =
  'inline-flex h-8 w-8 items-center justify-center rounded-md hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500';

export const getDrawerHeaderActionGroupClass = (className?: string): string =>
  [DRAWER_HEADER_ACTION_GROUP_CLASS, className].filter(Boolean).join(' ');

export const getDrawerHeaderActionButtonClass = (className?: string): string =>
  [DRAWER_HEADER_ACTION_BUTTON_CLASS, className].filter(Boolean).join(' ');

export const getDrawerHeaderIconButtonClass = (className?: string): string =>
  [DRAWER_HEADER_ICON_BUTTON_CLASS, className].filter(Boolean).join(' ');
