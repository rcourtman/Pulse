import { Component, Show, createSignal, createEffect } from 'solid-js';
import { Portal } from 'solid-js/web';

export interface UrlEditPopoverProps {
    /** Whether the popover is visible */
    isOpen: boolean;
    /** Current URL value being edited */
    value: string;
    /** Position of the popover (fixed positioning) */
    position: { top: number; left: number } | null;
    /** Whether the URL is currently being saved */
    isSaving?: boolean;
    /** Whether there's an existing URL that can be deleted */
    hasExistingUrl?: boolean;
    /** Placeholder text for the input */
    placeholder?: string;
    /** Help text shown below the input */
    helpText?: string;
    /** Called when the input value changes */
    onValueChange: (value: string) => void;
    /** Called when save is requested */
    onSave: () => void;
    /** Called when cancel is requested */
    onCancel: () => void;
    /** Called when delete is requested (only if hasExistingUrl is true) */
    onDelete?: () => void;
}

/**
 * A reusable URL editing popover component that provides a consistent
 * experience across all tables for adding/editing custom URLs.
 * 
 * Uses fixed positioning to escape overflow clipping from parent containers.
 */
export const UrlEditPopover: Component<UrlEditPopoverProps> = (props) => {
    let inputRef: HTMLInputElement | undefined;

    // Focus input when popover opens
    createEffect(() => {
        if (props.isOpen && inputRef) {
            // Use setTimeout to ensure the element is in the DOM
            setTimeout(() => {
                inputRef?.focus();
                inputRef?.select();
            }, 0);
        }
    });

    const handleKeyDown = (e: KeyboardEvent) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            props.onSave();
        } else if (e.key === 'Escape') {
            e.preventDefault();
            props.onCancel();
        }
    };

    return (
        <Show when={props.isOpen && props.position}>
            <Portal mount={document.body}>
                <div
                    data-url-editor
                    class="fixed z-[9999] bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-xl p-3 min-w-[300px]"
                    style={{ top: `${props.position!.top}px`, left: `${props.position!.left}px` }}
                    onClick={(e) => e.stopPropagation()}
                >
                    <div class="flex items-center gap-2">
                        <input
                            ref={inputRef}
                            type="url"
                            class="flex-1 text-sm px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors"
                            placeholder={props.placeholder ?? 'https://example.com'}
                            value={props.value}
                            onInput={(e) => props.onValueChange(e.currentTarget.value)}
                            onKeyDown={handleKeyDown}
                            disabled={props.isSaving}
                        />

                        {/* Save button */}
                        <button
                            type="button"
                            class="p-2 text-green-600 hover:text-green-700 dark:text-green-400 dark:hover:text-green-300 hover:bg-green-50 dark:hover:bg-green-900/20 rounded transition-colors disabled:opacity-50"
                            title="Save (Enter)"
                            disabled={props.isSaving}
                            onClick={props.onSave}
                        >
                            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                            </svg>
                        </button>

                        {/* Delete button - only show if there's an existing URL */}
                        <Show when={props.hasExistingUrl && props.onDelete}>
                            <button
                                type="button"
                                class="p-2 text-red-500 hover:text-red-600 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors disabled:opacity-50"
                                title="Remove URL"
                                disabled={props.isSaving}
                                onClick={props.onDelete}
                            >
                                <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                </svg>
                            </button>
                        </Show>

                        {/* Cancel button */}
                        <button
                            type="button"
                            class="p-2 text-gray-500 hover:text-gray-600 dark:text-gray-400 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors"
                            title="Cancel (Esc)"
                            onClick={props.onCancel}
                        >
                            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                        </button>
                    </div>

                    {/* Help text */}
                    <Show when={props.helpText}>
                        <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                            {props.helpText}
                        </p>
                    </Show>
                </div>
            </Portal>
        </Show>
    );
};

/**
 * Hook to manage URL editing state.
 * Provides all the state and handlers needed for the UrlEditPopover.
 */
export function createUrlEditState() {
    const [isEditing, setIsEditing] = createSignal(false);
    const [editingValue, setEditingValue] = createSignal('');
    const [isSaving, setIsSaving] = createSignal(false);
    const [position, setPosition] = createSignal<{ top: number; left: number } | null>(null);
    const [editingId, setEditingId] = createSignal<string | null>(null);

    const startEditing = (id: string, currentValue: string, event: MouseEvent) => {
        event.stopPropagation();
        event.preventDefault();

        const button = event.currentTarget as HTMLElement;
        const rect = button.getBoundingClientRect();

        // Popover dimensions (approximate)
        const popoverHeight = 80; // Approximate height of the popover
        const popoverWidth = 300; // min-w-[300px]

        // Get visual viewport dimensions (handles iOS Safari/Firefox address bar)
        const viewportHeight = window.visualViewport?.height ?? window.innerHeight;
        const viewportWidth = window.visualViewport?.width ?? window.innerWidth;
        const viewportOffsetTop = window.visualViewport?.offsetTop ?? 0;

        // Check if there's enough space below the button
        const spaceBelow = viewportHeight - (rect.bottom - viewportOffsetTop);
        const spaceAbove = rect.top - viewportOffsetTop;

        let top: number;
        if (spaceBelow >= popoverHeight + 8) {
            // Position below the button
            top = rect.bottom + 4;
        } else if (spaceAbove >= popoverHeight + 8) {
            // Flip above the button
            top = rect.top - popoverHeight - 4;
        } else {
            // Not enough space either way, position at top of viewport with small margin
            top = Math.max(8, viewportOffsetTop + 8);
        }

        // Ensure left position keeps popover within viewport
        let left = rect.left - 100;
        left = Math.max(8, left); // Don't go off left edge
        left = Math.min(left, viewportWidth - popoverWidth - 8); // Don't go off right edge

        setPosition({ top, left });
        setEditingValue(currentValue);
        setEditingId(id);
        setIsEditing(true);
    };

    const cancelEditing = () => {
        setIsEditing(false);
        setEditingValue('');
        setEditingId(null);
        setPosition(null);
    };

    const finishEditing = () => {
        setIsEditing(false);
        setEditingValue('');
        setEditingId(null);
        setPosition(null);
        setIsSaving(false);
    };

    return {
        isEditing,
        editingValue,
        setEditingValue,
        isSaving,
        setIsSaving,
        position,
        editingId,
        startEditing,
        cancelEditing,
        finishEditing,
    };
}
