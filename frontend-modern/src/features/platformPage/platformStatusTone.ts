import type { StatusIndicatorVariant } from '@/utils/status';

// A row's status words read in its dot's colour, without a second dot: red for
// down or failing, amber for a warning, muted for stopped or unknown, plain when
// healthy. Platform tables colour the cell behind a dot's verdict through this,
// so the dot and its words never disagree.
export const getPlatformStatusTextToneClass = (
  variant: StatusIndicatorVariant | null | undefined,
): string => {
  if (variant === 'danger') return 'font-medium text-red-600 dark:text-red-300';
  if (variant === 'warning') return 'font-medium text-amber-700 dark:text-amber-300';
  if (variant === 'muted') return 'text-muted';
  return '';
};
