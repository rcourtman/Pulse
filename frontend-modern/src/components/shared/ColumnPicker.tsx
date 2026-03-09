import { Component, Show, For, createSignal, onCleanup, createEffect } from 'solid-js';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import {
  FilterActionButton,
  FilterToolbarPanel,
  filterUtilityBadgeClass,
} from '@/components/shared/FilterToolbar';

interface ColumnPickerProps {
  /** Columns that can be toggled */
  columns: ColumnDef[];
  /** Check if a column is currently hidden */
  isHidden: (id: string) => boolean;
  /** Toggle a column's visibility */
  onToggle: (id: string) => void;
  /** Reset all columns to visible */
  onReset?: () => void;
}

export const ColumnPicker: Component<ColumnPickerProps> = (props) => {
  const [isOpen, setIsOpen] = createSignal(false);
  let containerRef: HTMLDivElement | undefined;

  // Close on click outside
  const handleClickOutside = (e: MouseEvent) => {
    if (containerRef && !containerRef.contains(e.target as Node)) {
      setIsOpen(false);
    }
  };

  createEffect(() => {
    if (isOpen()) {
      document.addEventListener('mousedown', handleClickOutside);
    } else {
      document.removeEventListener('mousedown', handleClickOutside);
    }
  });

  onCleanup(() => {
    document.removeEventListener('mousedown', handleClickOutside);
  });

  // Count how many are hidden
  const hiddenCount = () => props.columns.filter((c) => props.isHidden(c.id)).length;
  return (
    <div ref={containerRef} class="relative shrink-0">
      <FilterActionButton
        onClick={() => setIsOpen(!isOpen())}
        active={isOpen()}
        class="whitespace-nowrap"
        title="Choose which columns to display"
      >
        {/* Columns icon */}
        <svg
          class="w-3.5 h-3.5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 4v16M15 4v16M4 9h16M4 15h16" />
        </svg>
        <span>Columns</span>
        <Show when={hiddenCount() > 0}>
          <span class={filterUtilityBadgeClass}>{hiddenCount()}</span>
        </Show>
      </FilterActionButton>

      <Show when={isOpen()}>
        <FilterToolbarPanel class="top-[calc(100%+0.25rem)] z-50 w-56 p-0">
          <div class="px-3 py-2 border-b border-border-subtle">
            <div class="flex items-center justify-between">
              <span class="text-xs font-medium text-base-content">Show Columns</span>
              <Show when={props.onReset && hiddenCount() > 0}>
                <button
                  type="button"
                  onClick={() => props.onReset?.()}
                  class="text-[10px] text-blue-600 dark:text-blue-400 hover:underline"
                >
                  Show all
                </button>
              </Show>
            </div>
          </div>

          <div class="max-h-64 overflow-y-auto py-1">
            <For each={props.columns}>
              {(col) => {
                const isChecked = () => !props.isHidden(col.id);
                return (
                  <label class="flex items-center gap-2.5 px-3 py-2 cursor-pointer hover:bg-surface-hover transition-colors">
                    <input
                      type="checkbox"
                      checked={isChecked()}
                      onChange={() => props.onToggle(col.id)}
                      class="w-3.5 h-3.5 rounded border-border text-blue-600 focus:ring-blue-500 focus:ring-offset-0 dark:checked:bg-blue-600"
                    />
                    <span class={`text-sm ${isChecked() ? 'text-base-content' : 'text-muted'}`}>
                      {col.label}
                    </span>
                  </label>
                );
              }}
            </For>
          </div>

          <Show when={props.columns.length === 0}>
            <div class="px-3 py-4 text-xs text-muted text-center">
              No columns available to toggle
            </div>
          </Show>
        </FilterToolbarPanel>
      </Show>
    </div>
  );
};
