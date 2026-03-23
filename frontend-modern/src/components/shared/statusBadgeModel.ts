export type StatusBadgeSize = 'sm' | 'md';

export interface StatusBadgeProps {
  isEnabled: boolean;
  disabled?: boolean;
  size?: StatusBadgeSize;
  onToggle?: () => void;
  labelEnabled?: string;
  labelDisabled?: string;
  titleEnabled?: string;
  titleDisabled?: string;
  titleWhenDisabled?: string;
}

const STATUS_BADGE_BASE_CLASS =
  'inline-flex items-center justify-center text-xs font-medium rounded-md transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-1 focus-visible:ring-blue-400';
const STATUS_BADGE_PADDING_BY_SIZE: Record<StatusBadgeSize, string> = {
  sm: 'px-2 py-0.5',
  md: 'px-2.5 py-1',
};
const STATUS_BADGE_ENABLED_CLASS =
  'bg-blue-50 text-blue-700 hover:bg-blue-100 dark:bg-blue-500 dark:text-blue-300 dark:hover:bg-blue-500';
const STATUS_BADGE_DISABLED_STATE_CLASS = 'text-muted hover:bg-surface-hover';
const STATUS_BADGE_INTERACTION_DISABLED_CLASS =
  'opacity-60 cursor-not-allowed hover:bg-transparent dark:hover:bg-transparent';

export function resolveStatusBadgeSize(size: StatusBadgeSize | undefined): StatusBadgeSize {
  return size ?? 'sm';
}

export function getStatusBadgeClass(
  size: StatusBadgeSize,
  isEnabled: boolean,
  disabled: boolean,
): string {
  return [
    STATUS_BADGE_BASE_CLASS,
    STATUS_BADGE_PADDING_BY_SIZE[size],
    isEnabled ? STATUS_BADGE_ENABLED_CLASS : STATUS_BADGE_DISABLED_STATE_CLASS,
    disabled ? STATUS_BADGE_INTERACTION_DISABLED_CLASS : '',
  ].join(' ').trim();
}

export function getStatusBadgeLabel(props: StatusBadgeProps): string {
  return props.isEnabled ? (props.labelEnabled ?? 'Enabled') : (props.labelDisabled ?? 'Disabled');
}

export function getStatusBadgeTitle(props: StatusBadgeProps, disabled: boolean): string {
  if (disabled) {
    return props.titleWhenDisabled ?? props.titleDisabled ?? props.titleEnabled ?? '';
  }
  return props.isEnabled ? (props.titleEnabled ?? '') : (props.titleDisabled ?? '');
}
