import { Component, For, Show, createUniqueId } from 'solid-js';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';
import HashIcon from 'lucide-solid/icons/hash';
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
  COLUMN_PICKER_RESPONSIVE_NOTE,
  COLUMN_PICKER_MANUAL_WIDTH_NOTE,
  COLUMN_PICKER_RESET_LABEL,
  COLUMN_PICKER_RESET_WIDTHS_LABEL,
  getColumnPickerOptionTextClass,
} from '@/components/shared/columnPickerModel';
import {
  type ColumnPickerProps,
  useColumnPickerState,
} from '@/components/shared/useColumnPickerState';

export const ColumnPicker: Component<ColumnPickerProps> = (props) => {
  const state = useColumnPickerState(props);
  const contentId = createUniqueId();

  const content = () => (
    <>
      <div class="border-b border-border-subtle px-3 py-2">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-base-content">{COLUMN_PICKER_PANEL_TITLE}</span>
          <div class="flex items-center gap-2">
            <Show when={state.showResetWidths()}>
              <button
                type="button"
                onClick={state.handleResetWidthsClick}
                class="text-[10px] text-blue-600 hover:underline dark:text-blue-400"
              >
                {COLUMN_PICKER_RESET_WIDTHS_LABEL}
              </button>
            </Show>
            <Show when={state.showReset()}>
              <button
                type="button"
                onClick={state.handleResetClick}
                class="text-[10px] text-blue-600 hover:underline dark:text-blue-400"
              >
                {COLUMN_PICKER_RESET_LABEL}
              </button>
            </Show>
          </div>
        </div>
        <p class="mt-1 text-[10px] leading-4 text-muted">
          {state.showResetWidths()
            ? COLUMN_PICKER_MANUAL_WIDTH_NOTE
            : COLUMN_PICKER_RESPONSIVE_NOTE}
        </p>
      </div>

      <div
        class={`max-h-64 overflow-y-auto py-1 ${props.inline ? 'column-picker-inline-options' : ''}`.trim()}
      >
        <For each={props.columns}>
          {(column) => (
            <label class="flex cursor-pointer items-center gap-2.5 px-3 py-2 transition-colors hover:bg-surface-hover">
              <input
                type="checkbox"
                checked={state.isColumnChecked(column.id)}
                onChange={() => state.handleColumnToggle(column.id)}
                class="h-3.5 w-3.5 rounded border-border text-blue-600 focus:ring-blue-500 focus:ring-offset-0 dark:checked:bg-blue-600"
              />
              <span class={getColumnPickerOptionTextClass(state.isColumnChecked(column.id))}>
                {column.label}
              </span>
            </label>
          )}
        </For>
      </div>

      <Show when={props.columns.length === 0}>
        <div class="px-3 py-4 text-center text-xs text-muted">{COLUMN_PICKER_EMPTY_LABEL}</div>
      </Show>
    </>
  );

  return (
    <div ref={state.setContainerRef} class={`relative shrink-0 ${props.inline ? 'w-full' : ''}`}>
      <FilterActionButton
        onClick={state.toggleOpen}
        active={state.isOpen()}
        class={`${props.inline ? 'w-full justify-between text-base-content' : ''} whitespace-nowrap`}
        title={COLUMN_PICKER_BUTTON_TITLE}
        aria-expanded={state.isOpen()}
        aria-controls={contentId}
      >
        <span class="inline-flex items-center gap-1.5">
          <HashIcon class="h-3.5 w-3.5" />
          <span>{COLUMN_PICKER_BUTTON_LABEL}</span>
          <Show when={state.hiddenCount() > 0}>
            <span class={filterUtilityBadgeClass}>{state.hiddenCount()} hidden</span>
          </Show>
        </span>
        <ChevronDownIcon
          class={`h-3.5 w-3.5 transition-transform ${state.isOpen() ? 'rotate-180' : ''}`}
          aria-hidden="true"
        />
      </FilterActionButton>

      <Show when={state.isOpen()}>
        <Show
          when={props.inline}
          fallback={
            <FilterToolbarPanel
              id={contentId}
              widthClass="w-56 max-w-[calc(100vw-2rem)]"
              class="top-[calc(100%+0.25rem)] z-50 p-0"
            >
              {content()}
            </FilterToolbarPanel>
          }
        >
          <div
            id={contentId}
            class="mt-2 overflow-hidden rounded-md border border-border-subtle bg-surface"
          >
            {content()}
          </div>
        </Show>
      </Show>
    </div>
  );
};
