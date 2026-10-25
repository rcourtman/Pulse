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

// When a row's state has no column of its own, its words join the name on the
// same line rather than stacking under it, so the row stays one line tall.
// Only a state that needs attention (red or amber) shows its words; healthy
// and stopped already read in the dot's colour, and screen readers always get
// the words. Name and state keep their full width while the cell has room and
// shrink together, each to an ellipsis, only when it runs out.
export const getPlatformInlineStateClass = (attention: boolean): string =>
  attention ? 'min-w-0 truncate' : 'sr-only';

export const isPlatformAttentionVariant = (
  variant: StatusIndicatorVariant | null | undefined,
): boolean => variant === 'danger' || variant === 'warning';
