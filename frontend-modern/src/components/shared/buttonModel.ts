export type ButtonVariant =
  | 'primary'
  | 'primaryFlat'
  | 'warning'
  | 'info'
  | 'success'
  | 'successOutline'
  | 'successGhost'
  | 'secondary'
  | 'danger'
  | 'dangerOutline'
  | 'dangerGhost'
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
  info: 'border border-blue-200 bg-blue-50 text-blue-700 hover:bg-blue-100 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200',
  success: 'border border-transparent bg-emerald-600 text-white shadow-sm hover:bg-emerald-700',
  successOutline:
    'border border-emerald-300 bg-white text-emerald-900 hover:bg-emerald-100 dark:border-emerald-700 dark:bg-emerald-950 dark:text-emerald-100 dark:hover:bg-emerald-800',
  successGhost:
    'border border-transparent bg-transparent text-emerald-900 hover:bg-emerald-100 dark:text-emerald-100 dark:hover:bg-emerald-800',
  secondary: 'border border-border bg-surface text-base-content shadow-sm hover:bg-surface-hover',
  danger: 'border border-transparent bg-rose-600 text-white shadow-sm hover:bg-rose-700',
  dangerOutline:
    'border border-rose-300 bg-transparent text-rose-700 hover:bg-rose-50 dark:border-rose-900 dark:text-rose-300 dark:hover:bg-rose-950',
  dangerGhost:
    'border border-transparent bg-transparent text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900',
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
export type ActionIconButtonTone = 'neutral' | 'muted' | 'accent' | 'success' | 'danger';
export type ActionIconButtonSize = 'xs' | 'sm' | 'md';

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

export const ACTION_ICON_BUTTON_BASE_CLASS =
  'inline-flex shrink-0 items-center justify-center rounded-md transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-50';

export const ACTION_ICON_BUTTON_TONE_CLASSES: Record<ActionIconButtonTone, string> = {
  neutral: 'text-base-content hover:bg-surface-hover hover:text-base-content',
  muted: 'text-muted hover:bg-surface-hover hover:text-base-content',
  accent:
    'bg-blue-50 text-blue-600 hover:bg-blue-100 hover:text-blue-700 dark:bg-blue-900 dark:text-blue-400 dark:hover:bg-blue-950 dark:hover:text-blue-300',
  success:
    'bg-green-50 text-green-600 hover:bg-green-100 hover:text-green-700 dark:bg-green-900 dark:text-green-400 dark:hover:bg-green-950 dark:hover:text-green-300',
  danger:
    'text-red-600 hover:bg-red-50 hover:text-red-700 dark:text-red-400 dark:hover:bg-red-950 dark:hover:text-red-300',
};

export const ACTION_ICON_BUTTON_SIZE_CLASSES: Record<ActionIconButtonSize, string> = {
  xs: 'h-6 w-6',
  sm: 'h-7 w-7',
  md: 'h-8 w-8',
};

export type ActionIconButtonClassOptions = {
  tone?: ActionIconButtonTone;
  size?: ActionIconButtonSize;
  class?: string;
};

export const getActionIconButtonClass = (options: ActionIconButtonClassOptions = {}): string =>
  [
    ACTION_ICON_BUTTON_BASE_CLASS,
    ACTION_ICON_BUTTON_TONE_CLASSES[options.tone ?? 'neutral'],
    ACTION_ICON_BUTTON_SIZE_CLASSES[options.size ?? 'sm'],
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
