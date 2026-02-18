/**
 * Returns consistent CSS classes for segmented toggle buttons used in filter bars.
 * Includes flex/icon layout, spacing, animation, and selected/unselected visual states.
 *
 * Usage:
 *   <button class={segmentedButtonClass(isSelected)}>Label</button>
 *   <button class={segmentedButtonClass(isSelected, isDisabled)}>Label</button>
 */
export const segmentedButtonClass = (selected: boolean, disabled = false): string => {
  const base =
    'inline-flex items-center gap-1.5 px-2 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95';
  if (disabled) {
    return `${base} text-gray-400 dark:text-gray-600 cursor-not-allowed`;
  }
  if (selected) {
    return `${base} bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600`;
  }
  return `${base} text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50`;
};
