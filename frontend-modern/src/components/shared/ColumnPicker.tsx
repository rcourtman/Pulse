import { Component, Show, For, createSignal, onCleanup, createEffect } from 'solid-js';
import type { ColumnDef } from '@/hooks/useColumnVisibility';

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
  const hiddenCount = () => props.columns.filter(c => props.isHidden(c.id)).length;

  return (
    <div ref={containerRef} class="relative shrink-0">
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen())}
        class={`inline-flex items-center gap-1.5 whitespace-nowrap px-2.5 py-1.5 text-xs font-medium rounded-md transition-all
          ${isOpen()
            ? 'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300'
            : 'bg-surface-hover text-muted hover:bg-surface-hover'
          }`}
        title="Choose which columns to display"
      >
        {/* Columns icon */}
        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 4v16M15 4v16M4 9h16M4 15h16" />
        </svg>
        <span>Columns</span>
        <Show when={hiddenCount() > 0}>
          <span class="ml-0.5 inline-flex items-center whitespace-nowrap rounded-full bg-slate-200 px-1.5 py-0.5 text-[10px] font-semibold text-slate-700 dark:bg-slate-600 dark:text-slate-200">
            {hiddenCount()} hidden
          </span>
        </Show>
      </button>

      <Show when={isOpen()}>
        <div
          class="absolute right-0 mt-1 w-56 rounded-md border border-slate-200 bg-white shadow-sm z-50 dark:border-slate-700 dark:bg-slate-800"
        >
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
                  <label
                    class="flex items-center gap-2.5 px-3 py-2 cursor-pointer hover:bg-surface-hover transition-colors"
                  >
                    <input
                      type="checkbox"
                      checked={isChecked()}
                      onChange={() => props.onToggle(col.id)}
                      class="w-3.5 h-3.5 rounded border-slate-300 text-blue-600 focus:ring-blue-500 focus:ring-offset-0 dark:border-slate-600 dark:bg-slate-700 dark:checked:bg-blue-600"
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
        </div>
      </Show>
    </div>
  );
};
