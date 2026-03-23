import { Component, For, Show } from 'solid-js';
import {
  FilterActionButton,
  FilterToolbarPanel,
  filterUtilityBadgeClass,
} from '@/components/shared/FilterToolbar';
import {
  COLUMN_PICKER_BUTTON_LABEL,
  COLUMN_PICKER_BUTTON_TITLE,
  COLUMN_PICKER_EMPTY_LABEL,
  COLUMN_PICKER_PANEL_TITLE,
  COLUMN_PICKER_RESET_LABEL,
  getColumnPickerOptionTextClass,
} from '@/components/shared/columnPickerModel';
import {
  type ColumnPickerProps,
  useColumnPickerState,
} from '@/components/shared/useColumnPickerState';

export const ColumnPicker: Component<ColumnPickerProps> = (props) => {
  const state = useColumnPickerState(props);

  return (
    <div ref={state.setContainerRef} class="relative shrink-0">
      <FilterActionButton
        onClick={state.toggleOpen}
        active={state.isOpen()}
        class="whitespace-nowrap"
        title={COLUMN_PICKER_BUTTON_TITLE}
      >
        <svg
          class="w-3.5 h-3.5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 4v16M15 4v16M4 9h16M4 15h16" />
        </svg>
        <span>{COLUMN_PICKER_BUTTON_LABEL}</span>
        <Show when={state.hiddenCount() > 0}>
          <span class={filterUtilityBadgeClass}>{state.hiddenCount()}</span>
        </Show>
      </FilterActionButton>

      <Show when={state.isOpen()}>
        <FilterToolbarPanel class="top-[calc(100%+0.25rem)] z-50 w-56 p-0">
          <div class="px-3 py-2 border-b border-border-subtle">
            <div class="flex items-center justify-between">
              <span class="text-xs font-medium text-base-content">{COLUMN_PICKER_PANEL_TITLE}</span>
              <Show when={state.showReset()}>
                <button
                  type="button"
                  onClick={state.handleResetClick}
                  class="text-[10px] text-blue-600 dark:text-blue-400 hover:underline"
                >
                  {COLUMN_PICKER_RESET_LABEL}
                </button>
              </Show>
            </div>
          </div>

          <div class="max-h-64 overflow-y-auto py-1">
            <For each={props.columns}>
              {(column) => (
                <label class="flex items-center gap-2.5 px-3 py-2 cursor-pointer hover:bg-surface-hover transition-colors">
                  <input
                    type="checkbox"
                    checked={state.isColumnChecked(column.id)}
                    onChange={() => state.handleColumnToggle(column.id)}
                    class="w-3.5 h-3.5 rounded border-border text-blue-600 focus:ring-blue-500 focus:ring-offset-0 dark:checked:bg-blue-600"
                  />
                  <span class={getColumnPickerOptionTextClass(state.isColumnChecked(column.id))}>
                    {column.label}
                  </span>
                </label>
              )}
            </For>
          </div>

          <Show when={props.columns.length === 0}>
            <div class="px-3 py-4 text-xs text-muted text-center">
              {COLUMN_PICKER_EMPTY_LABEL}
            </div>
          </Show>
        </FilterToolbarPanel>
      </Show>
    </div>
  );
};
