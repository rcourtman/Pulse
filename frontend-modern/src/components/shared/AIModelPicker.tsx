import {
  Component,
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  onMount,
} from 'solid-js';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';
import MonitorIcon from 'lucide-solid/icons/monitor';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import { SearchField } from '@/components/shared/SearchField';
import type { ModelInfo } from '@/types/ai';
import { AI_CHAT_MODEL_SELECTOR_EMPTY_STATE } from '@/utils/aiChatPresentation';
import {
  formatAIModelRouteLabel,
  getAIProviderDisplayName,
  getProviderFromModelId,
} from '@/utils/aiProviderPresentation';

type AIModelPickerDefaultOption = {
  label: string;
  description?: string;
};

export type AIModelPickerExtraOption = {
  id: string;
  label: string;
  description?: string;
  hidden?: boolean;
};

export type AIModelPickerModelSection = {
  title: string;
  modelIds: string[];
};

export interface AIModelPickerProps {
  models: ModelInfo[];
  selectedModel: string;
  onModelSelect: (modelId: string) => void;
  defaultOption?: AIModelPickerDefaultOption;
  extraOptions?: AIModelPickerExtraOption[];
  modelSections?: AIModelPickerModelSection[];
  emptySelectionLabel?: string;
  selectionBadge?: string;
  title?: string;
  searchPlaceholder?: string;
  emptyState?: string;
  customModelDescription?: string;
  disabled?: boolean;
  isLoading?: boolean;
  error?: string;
  onRefresh?: () => void;
  openRequest?: number;
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
const DEFAULT_DROPDOWN_MAX_HEIGHT = 384;
const DEFAULT_LIST_MAX_HEIGHT = 288;
const MIN_DROPDOWN_MAX_HEIGHT = 120;
const MOBILE_BOTTOM_CLEARANCE = 88;
const DESKTOP_BOTTOM_CLEARANCE = 16;
const SEARCH_HEADER_HEIGHT = 52;
const ERROR_ROW_HEIGHT = 36;
const CUSTOM_RECENT_MODEL_DESCRIPTION = 'Recent custom model route';
const MODEL_ROUTE_PROVIDER_RE = /^[a-z][a-z0-9_-]*$/i;

type ResolvedModelRoute = {
  id: string;
  model?: ModelInfo;
};

const isExplicitModelRoute = (modelId: string) => {
  const candidate = modelId.trim();
  if (!candidate || /\s/.test(candidate) || candidate.includes('://')) {
    return false;
  }
  const separator = candidate.indexOf(':');
  if (separator <= 0 || separator === candidate.length - 1) {
    return false;
  }
  const provider = candidate.slice(0, separator);
  const model = candidate.slice(separator + 1);
  return MODEL_ROUTE_PROVIDER_RE.test(provider) && Boolean(model.trim()) && !model.startsWith('/');
};

const modelRouteLabel = (entry: ResolvedModelRoute) =>
  entry.model ? formatAIModelRouteLabel(entry.model) : formatAIModelRouteLabel(entry.id);

const modelRouteDescription = (entry: ResolvedModelRoute) =>
  entry.model?.description || (!entry.model ? CUSTOM_RECENT_MODEL_DESCRIPTION : '');

const modelRouteSecondaryId = (entry: ResolvedModelRoute) => {
  const model = entry.model;
  if (!model?.name || model.name === model.id) return '';
  return model.id;
};

function groupModelsByProvider(models: ModelInfo[]): Map<string, ModelInfo[]> {
  const grouped = new Map<string, ModelInfo[]>();

  for (const model of models) {
    // Prefer the server-supplied provider; fall back to the id heuristic for
    // models that predate the field or have an opaque id (#1320).
    const provider = model.provider?.trim() || getProviderFromModelId(model.id);
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
  const [dropdownPosition, setDropdownPosition] = createSignal({
    top: 0,
    left: 0,
    right: 0,
    maxHeight: DEFAULT_DROPDOWN_MAX_HEIGHT,
    listMaxHeight: DEFAULT_LIST_MAX_HEIGHT,
  });
  let containerRef: HTMLDivElement | undefined;
  let buttonRef: HTMLButtonElement | undefined;
  let searchInputRef: HTMLInputElement | undefined;
  let lastOpenRequest = props.openRequest || 0;

  const selectedModel = createMemo(() => props.selectedModel?.trim() || '');
  const modelsById = createMemo(() => new Map(props.models.map((model) => [model.id, model])));
  const notableModels = createMemo(() => props.models.filter((model) => model.notable));
  const shouldFilterToNotable = createMemo(() => notableModels().length > 0);
  const extraOptions = createMemo(() =>
    (props.extraOptions || []).filter((option) => option.id.trim() && !option.hidden),
  );
  const modelSections = createMemo(() => {
    const seen = new Set<string>();
    return (props.modelSections || [])
      .map((section) => {
        const models = section.modelIds.flatMap((modelId): ResolvedModelRoute[] => {
          const id = modelId.trim();
          if (!id || seen.has(id)) {
            return [];
          }
          const model = modelsById().get(id);
          if (!model && !isExplicitModelRoute(id)) {
            return [];
          }
          seen.add(id);
          return [{ id, model }];
        });
        return {
          title: section.title,
          models,
        };
      })
      .filter((section) => section.models.length > 0);
  });
  const sectionModelIds = createMemo(
    () => new Set(modelSections().flatMap((section) => section.models.map((model) => model.id))),
  );

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
    const sectionIds = sectionModelIds();
    return props.models.filter(
      (model) => !model.notable && model.id !== selected && !sectionIds.has(model.id),
    ).length;
  });

  const filteredModels = createMemo(() => {
    const query = searchQuery().trim().toLowerCase();
    if (!query) {
      const sectionIds = sectionModelIds();
      return visibleUnsearchedModels().filter((model) => !sectionIds.has(model.id));
    }
    return props.models.filter((model) => {
      const provider = model.provider?.trim() || getProviderFromModelId(model.id);
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
    if (!isExplicitModelRoute(candidate)) {
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
      return formatAIModelRouteLabel(match);
    }
    return formatAIModelRouteLabel(selected);
  });

  const dropdownStyle = createMemo(() => {
    const position = dropdownPosition();
    const base = { top: `${position.top}px`, 'max-height': `${position.maxHeight}px` };
    if (props.align === 'left') {
      return { ...base, left: `${position.left}px` };
    }
    return { ...base, right: `${position.right}px` };
  });

  const updateDropdownPosition = () => {
    if (!buttonRef) {
      return;
    }
    const rect = buttonRef.getBoundingClientRect();
    const bottomClearance =
      window.innerWidth < 1024 ? MOBILE_BOTTOM_CLEARANCE : DESKTOP_BOTTOM_CLEARANCE;
    const availableHeight = window.innerHeight - rect.bottom - bottomClearance;
    const maxHeight = Math.max(
      MIN_DROPDOWN_MAX_HEIGHT,
      Math.min(DEFAULT_DROPDOWN_MAX_HEIGHT, availableHeight),
    );
    const nonListHeight = SEARCH_HEADER_HEIGHT + (props.error ? ERROR_ROW_HEIGHT : 0);
    setDropdownPosition({
      top: rect.bottom + 4,
      left: rect.left,
      right: window.innerWidth - rect.right,
      maxHeight,
      listMaxHeight: Math.max(80, maxHeight - nonListHeight),
    });
  };

  const closePicker = () => {
    setIsOpen(false);
    setSearchQuery('');
  };

  const focusSearchInput = () => {
    queueMicrotask(() => searchInputRef?.focus());
  };

  const openPicker = () => {
    updateDropdownPosition();
    setSearchQuery('');
    setIsOpen(true);
    focusSearchInput();
  };

  const handleToggle = () => {
    if (props.disabled) {
      return;
    }
    if (!isOpen()) {
      openPicker();
      return;
    }
    closePicker();
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

  createEffect(() => {
    const request = props.openRequest || 0;
    if (request <= 0 || request === lastOpenRequest) {
      return;
    }
    lastOpenRequest = request;
    queueMicrotask(openPicker);
  });

  const hasVisibleListOptions = createMemo(
    () =>
      Boolean(props.defaultOption) ||
      extraOptions().length > 0 ||
      modelSections().length > 0 ||
      showCustomModelOption(),
  );

  onMount(() => {
    const handlePointerDown = (event: MouseEvent) => {
      if (isOpen() && containerRef && !containerRef.contains(event.target as Node)) {
        closePicker();
      }
    };
    const handleViewportChange = () => {
      if (isOpen()) {
        updateDropdownPosition();
      }
    };
    document.addEventListener('mousedown', handlePointerDown);
    window.addEventListener('resize', handleViewportChange);
    onCleanup(() => {
      document.removeEventListener('mousedown', handlePointerDown);
      window.removeEventListener('resize', handleViewportChange);
    });
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
        <Show when={props.selectionBadge}>
          <span class="text-[10px] font-normal text-muted">{props.selectionBadge}</span>
        </Show>
        <Show when={props.isLoading}>
          <RefreshCwIcon class="h-3 w-3 shrink-0 animate-spin" />
        </Show>
        <ChevronDownIcon class="h-3 w-3 shrink-0" />
      </button>

      <Show when={isOpen()}>
        <div
          class={`fixed overflow-hidden rounded-md border border-border bg-surface shadow-sm z-[9999] ${props.dropdownClass || DEFAULT_DROPDOWN_CLASS}`}
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
              inputRef={(el) => {
                searchInputRef = el;
              }}
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

          <div
            class="overflow-y-auto py-1"
            role="listbox"
            style={{ 'max-height': `${dropdownPosition().listMaxHeight}px` }}
          >
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

            <For each={extraOptions()}>
              {(option) => (
                <button
                  type="button"
                  onClick={() => handleSelect(option.id)}
                  class={`w-full px-3 py-2 text-left text-sm hover:bg-surface-hover ${selectedModel() === option.id ? 'bg-blue-50 dark:bg-blue-900' : ''}`}
                >
                  <div class="font-medium text-base-content">{option.label}</div>
                  <Show when={option.description}>
                    <div class="text-[11px] text-muted">{option.description}</div>
                  </Show>
                </button>
              )}
            </For>

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

            <Show
              when={
                !props.isLoading &&
                filteredModels().length === 0 &&
                (Boolean(searchQuery().trim()) || !hasVisibleListOptions())
              }
            >
              <div class="px-3 py-4 text-center text-[11px] text-muted">
                {props.emptyState || DEFAULT_EMPTY_STATE}
              </div>
            </Show>

            <Show when={!searchQuery().trim()}>
              <For each={modelSections()}>
                {(section) => (
                  <>
                    <div class="sticky top-0 bg-surface-alt px-3 py-1.5 text-[11px] font-semibold text-muted">
                      {section.title}
                    </div>
                    <For each={section.models}>
                      {(entry) => (
                        <button
                          type="button"
                          onClick={() => handleSelect(entry.id)}
                          class={`w-full px-3 py-2 text-left text-sm hover:bg-surface-hover ${selectedModel() === entry.id ? 'bg-blue-50 dark:bg-blue-900' : ''}`}
                        >
                          <div class="font-medium text-base-content">{modelRouteLabel(entry)}</div>
                          <Show when={modelRouteDescription(entry)}>
                            <div class="line-clamp-2 text-[11px] text-muted">
                              {modelRouteDescription(entry)}
                            </div>
                          </Show>
                          <Show when={modelRouteSecondaryId(entry)}>
                            {(modelId) => <div class="text-[10px] text-muted">{modelId()}</div>}
                          </Show>
                        </button>
                      )}
                    </For>
                  </>
                )}
              </For>
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
                          {formatAIModelRouteLabel(model)}
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
