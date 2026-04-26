import { Component, For, Show, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';
import MonitorIcon from 'lucide-solid/icons/monitor';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import { SearchField } from '@/components/shared/SearchField';
import type { ModelInfo } from '@/types/ai';
import { AI_CHAT_MODEL_SELECTOR_EMPTY_STATE } from '@/utils/aiChatPresentation';
import { getAIProviderDisplayName, getProviderFromModelId } from '@/utils/aiProviderPresentation';

type AIModelPickerDefaultOption = {
  label: string;
  description?: string;
};

export interface AIModelPickerProps {
  models: ModelInfo[];
  selectedModel: string;
  onModelSelect: (modelId: string) => void;
  defaultOption?: AIModelPickerDefaultOption;
  emptySelectionLabel?: string;
  title?: string;
  searchPlaceholder?: string;
  emptyState?: string;
  customModelDescription?: string;
  disabled?: boolean;
  isLoading?: boolean;
  error?: string;
  onRefresh?: () => void;
  align?: 'left' | 'right';
  buttonClass?: string;
  buttonLabelClass?: string;
  dropdownClass?: string;
}

const DEFAULT_BUTTON_CLASS =
  'inline-flex items-center gap-1.5 rounded-md border border-border bg-surface px-2.5 py-1.5 text-[11px] text-muted transition-colors hover:border-border hover:text-base-content disabled:cursor-not-allowed disabled:opacity-60';

const DEFAULT_LABEL_CLASS = 'max-w-[90px] truncate font-medium sm:max-w-[180px]';
const DEFAULT_DROPDOWN_CLASS = 'w-80';
const DEFAULT_EMPTY_STATE = AI_CHAT_MODEL_SELECTOR_EMPTY_STATE;

function groupModelsByProvider(models: ModelInfo[]): Map<string, ModelInfo[]> {
  const grouped = new Map<string, ModelInfo[]>();

  for (const model of models) {
    const provider = getProviderFromModelId(model.id);
    const existing = grouped.get(provider) || [];
    existing.push(model);
    grouped.set(provider, existing);
  }

  return grouped;
}

export const AIModelPicker: Component<AIModelPickerProps> = (props) => {
  const [isOpen, setIsOpen] = createSignal(false);
  const [showAllModels, setShowAllModels] = createSignal(false);
  const [searchQuery, setSearchQuery] = createSignal('');
  const [dropdownPosition, setDropdownPosition] = createSignal({ top: 0, left: 0, right: 0 });
  let containerRef: HTMLDivElement | undefined;
  let buttonRef: HTMLButtonElement | undefined;

  const selectedModel = createMemo(() => props.selectedModel?.trim() || '');
  const notableModels = createMemo(() => props.models.filter((model) => model.notable));
  const shouldFilterToNotable = createMemo(() => notableModels().length > 0);

  const visibleUnsearchedModels = createMemo(() => {
    if (showAllModels() || !shouldFilterToNotable()) {
      return props.models;
    }
    const selected = selectedModel();
    const current = props.models.find((model) => model.id === selected && !model.notable);
    return current ? [...notableModels(), current] : notableModels();
  });

  const hiddenModelCount = createMemo(() => {
    if (!shouldFilterToNotable()) {
      return 0;
    }
    const selected = selectedModel();
    return props.models.filter((model) => !model.notable && model.id !== selected).length;
  });

  const filteredModels = createMemo(() => {
    const query = searchQuery().trim().toLowerCase();
    if (!query) {
      return visibleUnsearchedModels();
    }
    return props.models.filter((model) => {
      const provider = getProviderFromModelId(model.id);
      const providerName = getAIProviderDisplayName(provider) || provider;
      const modelName = model.name || '';
      return (
        model.id.toLowerCase().includes(query) ||
        modelName.toLowerCase().includes(query) ||
        (model.description || '').toLowerCase().includes(query) ||
        provider.toLowerCase().includes(query) ||
        providerName.toLowerCase().includes(query)
      );
    });
  });

  const customModelCandidate = createMemo(() => searchQuery().trim());
  const exactCandidateModel = createMemo(() => {
    const candidate = customModelCandidate();
    if (!candidate) {
      return undefined;
    }
    return props.models.find((model) => model.id === candidate);
  });
  const showCustomModelOption = createMemo(() => {
    const candidate = customModelCandidate();
    if (!candidate) {
      return false;
    }
    if (!candidate.includes(':')) {
      return false;
    }
    return !props.models.some((model) => model.id === candidate);
  });

  const selectedLabel = createMemo(() => {
    const selected = selectedModel();
    if (!selected) {
      return props.emptySelectionLabel || props.defaultOption?.label || 'Select a model';
    }
    const match = props.models.find((model) => model.id === selected);
    if (match) {
      return match.name || match.id.split(':').pop() || match.id;
    }
    return selected;
  });

  const dropdownStyle = createMemo(() => {
    const position = dropdownPosition();
    if (props.align === 'left') {
      return { top: `${position.top}px`, left: `${position.left}px` };
    }
    return { top: `${position.top}px`, right: `${position.right}px` };
  });

  const updateDropdownPosition = () => {
    if (!buttonRef) {
      return;
    }
    const rect = buttonRef.getBoundingClientRect();
    setDropdownPosition({
      top: rect.bottom + 4,
      left: rect.left,
      right: window.innerWidth - rect.right,
    });
  };

  const closePicker = () => {
    setIsOpen(false);
    setSearchQuery('');
  };

  const handleToggle = () => {
    if (props.disabled) {
      return;
    }
    if (!isOpen()) {
      updateDropdownPosition();
    }
    setIsOpen(!isOpen());
  };

  const handleSelect = (modelId: string) => {
    props.onModelSelect(modelId);
    closePicker();
  };

  const handleKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'Enter') {
      return;
    }
    event.preventDefault();
    const candidate = customModelCandidate();
    if (candidate && (exactCandidateModel() || showCustomModelOption())) {
      handleSelect(candidate);
    }
  };

  onMount(() => {
    const handlePointerDown = (event: MouseEvent) => {
      if (isOpen() && containerRef && !containerRef.contains(event.target as Node)) {
        closePicker();
      }
    };
    document.addEventListener('mousedown', handlePointerDown);
    onCleanup(() => document.removeEventListener('mousedown', handlePointerDown));
  });

  return (
    <div ref={containerRef} class="relative" data-ai-model-picker>
      <button
        ref={buttonRef}
        type="button"
        onClick={handleToggle}
        disabled={props.disabled}
        aria-haspopup="listbox"
        aria-expanded={isOpen()}
        class={props.buttonClass || DEFAULT_BUTTON_CLASS}
        title={props.title || 'Select model'}
      >
        <MonitorIcon class="h-3.5 w-3.5 shrink-0" />
        <span class={props.buttonLabelClass || DEFAULT_LABEL_CLASS}>{selectedLabel()}</span>
        <Show when={props.isLoading}>
          <RefreshCwIcon class="h-3 w-3 shrink-0 animate-spin" />
        </Show>
        <ChevronDownIcon class="h-3 w-3 shrink-0" />
      </button>

      <Show when={isOpen()}>
        <div
          class={`fixed max-h-96 overflow-hidden rounded-md border border-border bg-surface shadow-sm z-[9999] ${props.dropdownClass || DEFAULT_DROPDOWN_CLASS}`}
          style={dropdownStyle()}
        >
          <div class="flex items-center gap-2 border-b border-border px-3 py-2">
            <SearchField
              value={searchQuery()}
              onChange={setSearchQuery}
              onKeyDown={handleKeyDown}
              placeholder={props.searchPlaceholder || 'Search or enter model ID'}
              class="flex-1"
              inputClass="py-1.5 text-xs focus:ring-blue-400"
            />
            <Show when={props.onRefresh}>
              <button
                type="button"
                onClick={() => props.onRefresh?.()}
                disabled={props.isLoading}
                class="rounded-md p-1.5 text-muted hover:bg-surface-hover hover:text-base-content disabled:cursor-not-allowed disabled:opacity-50"
                title="Refresh models"
              >
                <RefreshCwIcon class={`h-3.5 w-3.5 ${props.isLoading ? 'animate-spin' : ''}`} />
              </button>
            </Show>
          </div>

          <Show when={props.error}>
            <div class="border-b border-border px-3 py-2 text-[11px] text-red-500">
              {props.error}
            </div>
          </Show>

          <div class="max-h-72 overflow-y-auto py-1" role="listbox">
            <Show when={props.defaultOption}>
              <button
                type="button"
                onClick={() => handleSelect('')}
                class={`w-full px-3 py-2 text-left text-sm hover:bg-surface-hover ${!selectedModel() ? 'bg-blue-50 dark:bg-blue-900' : ''}`}
              >
                <div class="font-medium text-base-content">{props.defaultOption!.label}</div>
                <Show when={props.defaultOption!.description}>
                  <div class="text-[11px] text-muted">{props.defaultOption!.description}</div>
                </Show>
              </button>
            </Show>

            <Show when={showCustomModelOption()}>
              <button
                type="button"
                onClick={() => handleSelect(customModelCandidate())}
                class="w-full px-3 py-2 text-left text-sm hover:bg-surface-hover"
              >
                <div class="font-medium text-base-content">Use "{customModelCandidate()}"</div>
                <div class="text-[11px] text-muted">
                  {props.customModelDescription || 'Custom model ID'}
                </div>
              </button>
            </Show>

            <Show when={!props.isLoading && filteredModels().length === 0}>
              <div class="px-3 py-4 text-center text-[11px] text-muted">
                {props.emptyState || DEFAULT_EMPTY_STATE}
              </div>
            </Show>

            <For each={Array.from(groupModelsByProvider(filteredModels()).entries())}>
              {([provider, providerModels]) => (
                <>
                  <div class="sticky top-0 bg-surface-alt px-3 py-1.5 text-[11px] font-semibold text-muted">
                    {getAIProviderDisplayName(provider) || provider}
                  </div>
                  <For each={providerModels}>
                    {(model) => (
                      <button
                        type="button"
                        onClick={() => handleSelect(model.id)}
                        class={`w-full px-3 py-2 text-left text-sm hover:bg-surface-hover ${selectedModel() === model.id ? 'bg-blue-50 dark:bg-blue-900' : ''}`}
                      >
                        <div class="font-medium text-base-content">
                          {model.name || model.id.split(':').pop() || model.id}
                        </div>
                        <Show when={model.description}>
                          <div class="line-clamp-2 text-[11px] text-muted">{model.description}</div>
                        </Show>
                        <Show when={model.name && model.name !== model.id}>
                          <div class="text-[10px] text-muted">{model.id}</div>
                        </Show>
                      </button>
                    )}
                  </For>
                </>
              )}
            </For>

            <Show when={hiddenModelCount() > 0 && !searchQuery().trim()}>
              <div class="mt-1 border-t border-border pt-1">
                <button
                  type="button"
                  onClick={() => setShowAllModels(!showAllModels())}
                  class="flex w-full items-center gap-1.5 px-3 py-2 text-left text-xs text-muted hover:bg-surface-hover hover:text-base-content"
                >
                  <ChevronDownIcon
                    class={`h-3 w-3 transition-transform ${showAllModels() ? 'rotate-180' : ''}`}
                  />
                  {showAllModels()
                    ? 'Hide older models'
                    : `Show ${hiddenModelCount()} older models`}
                </button>
              </div>
            </Show>
          </div>
        </div>
      </Show>
    </div>
  );
};
