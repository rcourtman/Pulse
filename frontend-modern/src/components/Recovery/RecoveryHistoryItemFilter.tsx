import { For, Show, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import type { Accessor, Component } from 'solid-js';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';

import { SearchField } from '@/components/shared/SearchField';
import {
  filterGroupClass,
  filterLabelClass,
  filterPanelDescriptionClass,
  filterPanelTitleClass,
  filterSelectClass,
} from '@/components/shared/FilterToolbar';

export interface RecoveryHistoryItemFilterOption {
  rollupId: string;
  label: string;
  secondaryLabel?: string | null;
  contextLabel?: string | null;
}

interface RecoveryHistoryItemFilterProps {
  options: Accessor<RecoveryHistoryItemFilterOption[]>;
  selectedRollupId: Accessor<string>;
  selectedLabel: Accessor<string | null>;
  onSelect: (rollupId: string) => void;
  onClear: () => void;
}

const OPTION_LIMIT = 40;

export const RecoveryHistoryItemFilter: Component<RecoveryHistoryItemFilterProps> = (props) => {
  const [isOpen, setIsOpen] = createSignal(false);
  const [query, setQuery] = createSignal('');
  const [activeIndex, setActiveIndex] = createSignal(0);
  let rootRef: HTMLDivElement | undefined;
  let searchInputRef: HTMLInputElement | undefined;

  const filteredOptions = createMemo(() => {
    const normalizedQuery = query().trim().toLowerCase();
    const selected = props.selectedRollupId().trim();
    const ranked = props
      .options()
      .filter((option) => {
        if (!normalizedQuery) return true;
        const haystack = [
          option.label,
          option.secondaryLabel || '',
          option.contextLabel || '',
          option.rollupId,
        ]
          .join(' ')
          .toLowerCase();
        return haystack.includes(normalizedQuery);
      })
      .sort((left, right) => {
        const leftSelected = left.rollupId === selected ? 1 : 0;
        const rightSelected = right.rollupId === selected ? 1 : 0;
        if (leftSelected !== rightSelected) return rightSelected - leftSelected;
        return left.label.localeCompare(right.label);
      });

    return ranked.slice(0, OPTION_LIMIT);
  });

  const close = () => {
    setIsOpen(false);
    setQuery('');
    setActiveIndex(0);
  };

  const handleClickOutside = (event: MouseEvent) => {
    const target = event.target as Node | null;
    if (target && rootRef?.contains(target)) return;
    close();
  };

  createEffect(() => {
    if (!isOpen()) return;
    document.addEventListener('mousedown', handleClickOutside);
    queueMicrotask(() => searchInputRef?.focus());
    onCleanup(() => document.removeEventListener('mousedown', handleClickOutside));
  });

  onCleanup(() => {
    document.removeEventListener('mousedown', handleClickOutside);
  });

  createEffect(() => {
    filteredOptions();
    setActiveIndex(0);
  });

  return (
    <div ref={rootRef} class={`relative ${filterGroupClass}`}>
      <span class={filterLabelClass}>Item</span>
      <button
        type="button"
        class={`${filterSelectClass} flex min-w-[12rem] max-w-[18rem] items-center gap-2 pr-2 text-left`}
        aria-haspopup="dialog"
        aria-expanded={isOpen()}
        aria-controls="recovery-history-item-filter-panel"
        aria-label="Item filter"
        data-testid="recovery-history-item-filter-trigger"
        onClick={() => {
          if (isOpen()) {
            close();
            return;
          }
          setIsOpen(true);
        }}
      >
        <span class="truncate">{props.selectedLabel() || 'Any Item'}</span>
        <ChevronDownIcon
          class={`ml-auto h-3.5 w-3.5 shrink-0 transition-transform ${isOpen() ? 'rotate-180' : ''}`}
        />
      </button>

      <Show when={isOpen()}>
        <div
          id="recovery-history-item-filter-panel"
          data-testid="recovery-history-item-filter-panel"
          class="absolute left-0 top-[calc(100%+0.5rem)] z-20 w-[min(28rem,calc(100vw-2rem))] rounded-md border border-border bg-surface p-3 shadow-lg"
          role="dialog"
          aria-label="Choose recovery item"
        >
          <div class="mb-3 flex items-start justify-between gap-3">
            <div>
              <div class={filterPanelTitleClass}>Filter by item</div>
              <div class={filterPanelDescriptionClass}>
                Pick an exact protected item or clear the selection.
              </div>
            </div>
            <Show when={props.selectedRollupId().trim() !== ''}>
              <button
                type="button"
                class="text-xs font-medium text-muted transition-colors hover:text-base-content"
                data-testid="recovery-history-item-filter-clear"
                onClick={() => {
                  props.onClear();
                  close();
                }}
              >
                Clear item
              </button>
            </Show>
          </div>

          <SearchField
            value={query()}
            onChange={setQuery}
            inputRef={(element) => {
              searchInputRef = element;
            }}
            onKeyDown={(event) => {
              const options = filteredOptions();
              if (event.key === 'Escape') {
                event.preventDefault();
                close();
                return;
              }
              if (!options.length) return;
              if (event.key === 'ArrowDown') {
                event.preventDefault();
                setActiveIndex((current) => Math.min(current + 1, options.length - 1));
                return;
              }
              if (event.key === 'ArrowUp') {
                event.preventDefault();
                setActiveIndex((current) => Math.max(current - 1, 0));
                return;
              }
              if (event.key === 'Enter') {
                event.preventDefault();
                const option = options[activeIndex()];
                if (!option) return;
                props.onSelect(option.rollupId);
                close();
              }
            }}
            placeholder="Search protected items..."
            inputClass="py-2 text-sm"
            clearOnFocusedEscape={false}
          />

          <div class="mt-3 max-h-[18rem] overflow-y-auto">
            <Show
              when={filteredOptions().length > 0}
              fallback={
                <div class="rounded-md border border-dashed border-border px-3 py-4 text-sm text-muted">
                  No matching protected items.
                </div>
              }
            >
              <div class="flex flex-col gap-1">
                <For each={filteredOptions()}>
                  {(option, index) => {
                    const isSelected = () => option.rollupId === props.selectedRollupId().trim();
                    const isActive = () => index() === activeIndex();
                    return (
                      <button
                        type="button"
                        data-testid={`recovery-history-item-filter-option-${option.rollupId}`}
                        class={`flex w-full flex-col rounded-md px-3 py-2 text-left transition-colors ${
                          isSelected()
                            ? 'bg-blue-50 text-base-content ring-1 ring-blue-200 dark:bg-blue-950/40 dark:ring-blue-900'
                            : isActive()
                              ? 'bg-surface-hover text-base-content'
                              : 'text-base-content hover:bg-surface-hover'
                        }`}
                        onClick={() => {
                          props.onSelect(option.rollupId);
                          close();
                        }}
                        onMouseEnter={() => setActiveIndex(index())}
                      >
                        <div class="flex items-center gap-2">
                          <span class="truncate font-medium">{option.label}</span>
                          <Show when={isSelected()}>
                            <span class="shrink-0 rounded-full bg-surface-alt px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
                              Selected
                            </span>
                          </Show>
                        </div>
                        <Show when={option.secondaryLabel || option.contextLabel}>
                          <div class="mt-0.5 flex flex-wrap gap-x-2 gap-y-1 text-xs text-muted">
                            <Show when={option.secondaryLabel}>
                              <span>{option.secondaryLabel}</span>
                            </Show>
                            <Show when={option.contextLabel}>
                              <span>{option.contextLabel}</span>
                            </Show>
                          </div>
                        </Show>
                      </button>
                    );
                  }}
                </For>
              </div>
            </Show>
          </div>
        </div>
      </Show>
    </div>
  );
};
