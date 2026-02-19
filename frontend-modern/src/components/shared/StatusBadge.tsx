import { JSX } from 'solid-js';

interface StatusBadgeProps {
  isEnabled: boolean;
  disabled?: boolean;
  size?: 'sm' | 'md';
  onToggle?: () => void;
  labelEnabled?: string;
  labelDisabled?: string;
  titleEnabled?: string;
  titleDisabled?: string;
  titleWhenDisabled?: string;
}

/**
 * StatusBadge - A shared component for enabled/disabled status badges
 * Used throughout the app for consistent styling of status indicators
 */
export function StatusBadge(props: StatusBadgeProps): JSX.Element {
  const {
    isEnabled,
    disabled = false,
    size = 'sm',
    onToggle,
    labelEnabled = 'Enabled',
    labelDisabled = 'Disabled',
    titleEnabled,
    titleDisabled,
    titleWhenDisabled,
  } = props;

  const basePadding = size === 'md' ? 'px-2.5 py-1' : 'px-2 py-0.5';
  const baseClasses = `inline-flex items-center justify-center ${basePadding} text-xs font-medium rounded-md transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-1 focus-visible:ring-blue-400`;
  const stateClasses = isEnabled
    ? 'bg-blue-50 text-blue-700 hover:bg-blue-100 dark:bg-blue-500/20 dark:text-blue-300 dark:hover:bg-blue-500/30'
    : 'bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-700 dark:text-slate-400 dark:hover:bg-slate-600';
  const disabledClasses = disabled
    ? 'opacity-60 cursor-not-allowed hover:bg-transparent dark:hover:bg-transparent'
    : '';

  const title = disabled
    ? titleWhenDisabled ?? titleDisabled ?? titleEnabled ?? ''
    : isEnabled
      ? titleEnabled ?? ''
      : titleDisabled ?? '';

  return (
    <button
      type="button"
      class={`${baseClasses} ${stateClasses} ${disabledClasses}`.trim()}
      onClick={() => {
        if (disabled) return;
        onToggle?.();
      }}
      disabled={disabled}
      aria-pressed={isEnabled}
      title={title}
    >
      {isEnabled ? labelEnabled : labelDisabled}
    </button>
  );
}

export default StatusBadge;
