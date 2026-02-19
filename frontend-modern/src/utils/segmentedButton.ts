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
    return `${base} text-slate-400 dark:text-slate-600 cursor-not-allowed`;
  }
  if (selected) {
    return `${base} bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600`;
  }
  return `${base} text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100 hover:bg-slate-50 dark:hover:bg-slate-600/50`;
};
