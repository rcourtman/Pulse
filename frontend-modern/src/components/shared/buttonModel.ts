export type ButtonVariant =
  | 'primary'
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
  | 'md'
  | 'lg'
  | 'icon'
  | 'iconMd';

export const BUTTON_BASE_CLASS =
  'inline-flex items-center justify-center rounded-md font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed';

export const BUTTON_VARIANT_CLASSES: Record<ButtonVariant, string> = {
  primary: 'border border-transparent bg-blue-600 text-white shadow-sm hover:bg-blue-700',
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
