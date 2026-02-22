/**
 * Returns consistent CSS classes for segmented toggle buttons used in filter bars.
 * Includes flex/icon layout, spacing, animation, and selected/unselected visual states.
 *
 * Usage:
 * <button class={segmentedButtonClass(isSelected)}>Label</button>
 * <button class={segmentedButtonClass(isSelected, isDisabled)}>Label</button>
 */
export const segmentedButtonClass = (selected: boolean, disabled = false): string => {
    const base =
        'inline-flex items-center gap-1.5 px-2 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95';
    if (disabled) {
        return `${base} text-muted cursor-not-allowed`;
    }
    if (selected) {
        return `${base} bg-surface text-base-content shadow-sm ring-1 ring-border`;
    }
    return `${base} text-muted hover:text-base-content hover:bg-surface-hover`;
};
