import {
  Component,
  For,
  Show,
  createEffect,
  createMemo,
  createUniqueId,
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
  isPulseOwnedLocalModelRoute,
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
const TOP_CLEARANCE = 16;
const SEARCH_HEADER_HEIGHT = 52;
const ERROR_ROW_HEIGHT = 36;
const CUSTOM_RECENT_MODEL_DESCRIPTION = 'Recent custom model route';
const MODEL_ROUTE_PROVIDER_RE = /^[a-z][a-z0-9_-]*$/i;
const CURRENT_SELECTION_LABEL = 'Current';
const DEFAULT_OPTION_KEY = '__default__';
const SHOW_OLDER_MODELS_OPTION_KEY = '__show_older_models__';

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
  if (model && isPulseOwnedLocalModelRoute(model.id)) return '';
  if (!model?.name || model.name === model.id) return '';
  return model.id;
};

const CurrentSelectionBadge: Component = () => (
  <span class="shrink-0 rounded border border-blue-200 bg-blue-100 px-1.5 py-0.5 text-[10px] font-semibold uppercase text-blue-700 dark:border-blue-800 dark:bg-blue-950/60 dark:text-blue-200">
    {' '}
    {CURRENT_SELECTION_LABEL}{' '}
  </span>
);

const optionAriaLabel = (
  label: string,
  isSelected: boolean,
  details: Array<string | undefined> = [],
) =>
  [
    isSelected ? `${label}, ${CURRENT_SELECTION_LABEL}` : label,
    ...details.filter((detail): detail is string => Boolean(detail?.trim())),
  ].join('. ');

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
    bottom: 0,
    top: 0,
    left: 0,
    right: 0,
    maxHeight: DEFAULT_DROPDOWN_MAX_HEIGHT,
    listMaxHeight: DEFAULT_LIST_MAX_HEIGHT,
    placement: 'bottom' as 'bottom' | 'top',
  });
  let containerRef: HTMLDivElement | undefined;
  let buttonRef: HTMLButtonElement | undefined;
  let searchInputRef: HTMLInputElement | undefined;
  const pickerId = createUniqueId();
  const optionRefs = new Map<string, HTMLButtonElement>();
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
  const optionKeyForModel = (modelId: string) => `model:${modelId}`;
  const optionKeyForExtra = (optionId: string) => `extra:${optionId}`;
  const optionKeyForCustom = (modelId: string) => `custom:${modelId}`;
  const displayedOptionKeys = createMemo(() => {
    const keys: string[] = [];
    if (props.defaultOption) {
      keys.push(DEFAULT_OPTION_KEY);
    }
    for (const option of extraOptions()) {
      keys.push(optionKeyForExtra(option.id));
    }
    if (showCustomModelOption()) {
      keys.push(optionKeyForCustom(customModelCandidate()));
    }
    if (!searchQuery().trim()) {
      for (const section of modelSections()) {
        for (const entry of section.models) {
          keys.push(optionKeyForModel(entry.id));
        }
      }
    }
    for (const [, providerModels] of groupModelsByProvider(filteredModels()).entries()) {
      for (const model of providerModels) {
        keys.push(optionKeyForModel(model.id));
      }
    }
    if (hiddenModelCount() > 0 && !searchQuery().trim()) {
      keys.push(SHOW_OLDER_MODELS_OPTION_KEY);
    }
    return keys;
  });
  const currentOptionKey = createMemo(() => {
    const selected = selectedModel();
    if (!selected) {
      return props.defaultOption ? DEFAULT_OPTION_KEY : '';
    }
    if (extraOptions().some((option) => option.id === selected)) {
      return optionKeyForExtra(selected);
    }
    if (showCustomModelOption() && customModelCandidate() === selected) {
      return optionKeyForCustom(selected);
    }
    const modelKey = optionKeyForModel(selected);
    return displayedOptionKeys().includes(modelKey) ? modelKey : '';
  });
  const showOlderModelsOptionLabel = createMemo(() =>
    showAllModels() ? 'Hide older models' : `Show ${hiddenModelCount()} older models`,
  );

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
  const selectedBadge = createMemo(() => props.selectionBadge?.trim() || '');
  const selectedButtonLabel = createMemo(() => {
    const label = selectedLabel();
    const badge = selectedBadge();
    return badge ? `${label}, ${badge}` : label;
  });

  const dropdownStyle = createMemo(() => {
    const position = dropdownPosition();
    const base = {
      ...(position.placement === 'top'
        ? { bottom: `${position.bottom}px` }
        : { top: `${position.top}px` }),
      'max-height': `${position.maxHeight}px`,
    };
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
    const availableBelow = window.innerHeight - rect.bottom - bottomClearance;
    const availableAbove = rect.top - TOP_CLEARANCE;
    const placement =
      availableBelow < MIN_DROPDOWN_MAX_HEIGHT && availableAbove > availableBelow
        ? 'top'
        : 'bottom';
    const availableHeight = placement === 'top' ? availableAbove : availableBelow;
    const maxHeight = Math.max(
      MIN_DROPDOWN_MAX_HEIGHT,
      Math.min(DEFAULT_DROPDOWN_MAX_HEIGHT, availableHeight),
    );
    const nonListHeight = SEARCH_HEADER_HEIGHT + (props.error ? ERROR_ROW_HEIGHT : 0);
    setDropdownPosition({
      bottom: window.innerHeight - rect.top + 4,
      top: rect.bottom + 4,
      left: rect.left,
      right: window.innerWidth - rect.right,
      maxHeight,
      listMaxHeight: Math.max(80, maxHeight - nonListHeight),
      placement,
    });
  };

  const closePicker = () => {
    setIsOpen(false);
    setSearchQuery('');
  };

  const focusTriggerAfterClose = () => {
    const trigger = buttonRef;
    if (!trigger) return;
    window.setTimeout(() => trigger.focus(), 0);
  };

  const closePickerAndFocusTrigger = () => {
    closePicker();
    focusTriggerAfterClose();
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
  const isSelectedRoute = (modelId: string) => selectedModel() === modelId;
  const optionClass = (isSelected: boolean) =>
    `w-full px-3 py-2 text-left text-sm hover:bg-surface-hover ${
      isSelected ? 'bg-blue-50 dark:bg-blue-900' : ''
    }`;

  createEffect(() => {
    const visibleKeys = new Set(displayedOptionKeys());
    for (const key of optionRefs.keys()) {
      if (!visibleKeys.has(key)) {
        optionRefs.delete(key);
      }
    }
  });

  const focusOptionAtIndex = (index: number) => {
    const keys = displayedOptionKeys();
    if (keys.length === 0) return false;
    const nextIndex = ((index % keys.length) + keys.length) % keys.length;
    const target = optionRefs.get(keys[nextIndex]);
    if (!target) return false;
    target.focus();
    return true;
  };

  const initialOptionIndex = () => {
    const keys = displayedOptionKeys();
    if (keys.length === 0) return -1;
    const currentKey = currentOptionKey();
    const currentIndex = !searchQuery().trim() && currentKey ? keys.indexOf(currentKey) : -1;
    return currentIndex >= 0 ? currentIndex : 0;
  };

  const focusInitialOption = () => {
    const nextIndex = initialOptionIndex();
    return nextIndex >= 0 && focusOptionAtIndex(nextIndex);
  };

  const focusOptionFromSearchByOffset = (offset: number) => {
    const keys = displayedOptionKeys();
    const startIndex = initialOptionIndex();
    if (keys.length === 0 || startIndex < 0) return false;
    let nextIndex = startIndex + offset;
    if (nextIndex < 0) nextIndex = keys.length - 1;
    if (nextIndex >= keys.length) nextIndex = 0;
    return focusOptionAtIndex(nextIndex);
  };

  const consumePickerKey = (event: KeyboardEvent) => {
    event.preventDefault();
    event.stopPropagation();
  };

  const focusOptionRelativeTo = (optionKey: string, offset: number) => {
    const keys = displayedOptionKeys();
    const currentIndex = keys.indexOf(optionKey);
    if (currentIndex < 0) return false;
    let nextIndex = currentIndex + offset;
    if (nextIndex < 0) nextIndex = keys.length - 1;
    if (nextIndex >= keys.length) nextIndex = 0;
    return focusOptionAtIndex(nextIndex);
  };

  const handleSearchKeyDown = (event: KeyboardEvent) => {
    if (event.altKey || event.ctrlKey || event.metaKey) return;
    if (event.key === 'ArrowDown' && focusInitialOption()) {
      consumePickerKey(event);
      return;
    }
    if (event.key === 'ArrowUp' && focusOptionAtIndex(displayedOptionKeys().length - 1)) {
      consumePickerKey(event);
      return;
    }
    if (event.key === 'PageDown' && focusOptionFromSearchByOffset(10)) {
      consumePickerKey(event);
      return;
    }
    if (event.key === 'PageUp' && focusOptionFromSearchByOffset(-10)) {
      consumePickerKey(event);
      return;
    }
    if (event.key === 'Home' && focusOptionAtIndex(0)) {
      consumePickerKey(event);
      return;
    }
    if (event.key === 'End' && focusOptionAtIndex(displayedOptionKeys().length - 1)) {
      consumePickerKey(event);
      return;
    }
    if (event.key === 'Escape') {
      consumePickerKey(event);
      closePickerAndFocusTrigger();
      return;
    }
    if (event.key === 'Enter') {
      consumePickerKey(event);
      const candidate = customModelCandidate();
      if (candidate && (exactCandidateModel() || showCustomModelOption())) {
        handleSelect(candidate);
      }
    }
  };

  const handleOptionKeyDown = (
    event: KeyboardEvent & { currentTarget: HTMLButtonElement },
    optionKey: string,
  ) => {
    if (event.altKey || event.ctrlKey || event.metaKey) return;

    if (event.key === 'Enter' || event.key === ' ' || event.key === 'Spacebar') {
      consumePickerKey(event);
      event.currentTarget.click();
      return;
    }

    if (event.key === 'ArrowDown' && focusOptionRelativeTo(optionKey, 1)) {
      consumePickerKey(event);
      return;
    }
    if (event.key === 'ArrowUp' && focusOptionRelativeTo(optionKey, -1)) {
      consumePickerKey(event);
      return;
    }
    if (event.key === 'PageDown' && focusOptionRelativeTo(optionKey, 10)) {
      consumePickerKey(event);
      return;
    }
    if (event.key === 'PageUp' && focusOptionRelativeTo(optionKey, -10)) {
      consumePickerKey(event);
      return;
    }
    if (event.key === 'Home' && focusOptionAtIndex(0)) {
      consumePickerKey(event);
      return;
    }
    if (event.key === 'End' && focusOptionAtIndex(displayedOptionKeys().length - 1)) {
      consumePickerKey(event);
      return;
    }
    if (event.key === 'Escape') {
      consumePickerKey(event);
      closePickerAndFocusTrigger();
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
        aria-controls={isOpen() ? `${pickerId}-listbox` : undefined}
        aria-label={selectedButtonLabel()}
        class={props.buttonClass || DEFAULT_BUTTON_CLASS}
        title={props.title || 'Select model'}
      >
        <MonitorIcon class="h-3.5 w-3.5 shrink-0" />
        <span class={props.buttonLabelClass || DEFAULT_LABEL_CLASS}>{selectedLabel()}</span>
        <Show when={selectedBadge()}>
          {(badge) => (
            <>
              <span class="text-[10px] font-normal text-muted" aria-hidden="true">
                {' '}
                ·{' '}
              </span>
              <span class="text-[10px] font-normal text-muted">{badge()}</span>
            </>
          )}
        </Show>
        <Show when={props.isLoading}>
          <RefreshCwIcon class="h-3 w-3 shrink-0 animate-spin" />
        </Show>
        <ChevronDownIcon class="h-3 w-3 shrink-0" />
      </button>

      <Show when={isOpen()}>
        <div
          class={`fixed overflow-hidden rounded-md border border-border bg-surface shadow-sm z-[9999] ${props.dropdownClass || DEFAULT_DROPDOWN_CLASS}`}
          role="dialog"
          aria-label={props.title || 'Select model'}
          style={dropdownStyle()}
        >
          <div class="flex items-center gap-2 border-b border-border px-3 py-2">
            <SearchField
              value={searchQuery()}
              onChange={setSearchQuery}
              onKeyDown={handleSearchKeyDown}
              clearOnFocusedEscape={false}
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
                aria-label="Refresh models"
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
            id={`${pickerId}-listbox`}
            class="overflow-y-auto py-1"
            role="listbox"
            aria-label={props.title || 'Select model'}
            style={{ 'max-height': `${dropdownPosition().listMaxHeight}px` }}
          >
            <Show when={props.defaultOption}>
              <button
                type="button"
                ref={(button) => {
                  optionRefs.set(DEFAULT_OPTION_KEY, button);
                }}
                onClick={() => handleSelect('')}
                onKeyDown={(event) => handleOptionKeyDown(event, DEFAULT_OPTION_KEY)}
                role="option"
                aria-selected={!selectedModel()}
                aria-label={optionAriaLabel(props.defaultOption!.label, !selectedModel(), [
                  props.defaultOption!.description,
                ])}
                class={optionClass(!selectedModel())}
              >
                <div class="flex min-w-0 items-center gap-2">
                  <span class="min-w-0 flex-1 truncate font-medium text-base-content">
                    {props.defaultOption!.label}
                  </span>
                  <Show when={!selectedModel()}>
                    <CurrentSelectionBadge />
                  </Show>
                </div>
                <Show when={props.defaultOption!.description}>
                  <div class="text-[11px] text-muted">{props.defaultOption!.description}</div>
                </Show>
              </button>
            </Show>

            <For each={extraOptions()}>
              {(option) => (
                <button
                  type="button"
                  ref={(button) => {
                    optionRefs.set(optionKeyForExtra(option.id), button);
                  }}
                  onClick={() => handleSelect(option.id)}
                  onKeyDown={(event) => handleOptionKeyDown(event, optionKeyForExtra(option.id))}
                  role="option"
                  aria-selected={isSelectedRoute(option.id)}
                  aria-label={optionAriaLabel(option.label, isSelectedRoute(option.id), [
                    option.description,
                  ])}
                  class={optionClass(isSelectedRoute(option.id))}
                >
                  <div class="flex min-w-0 items-center gap-2">
                    <span class="min-w-0 flex-1 truncate font-medium text-base-content">
                      {option.label}
                    </span>
                    <Show when={isSelectedRoute(option.id)}>
                      <CurrentSelectionBadge />
                    </Show>
                  </div>
                  <Show when={option.description}>
                    <div class="text-[11px] text-muted">{option.description}</div>
                  </Show>
                </button>
              )}
            </For>

            <Show when={showCustomModelOption()}>
              <button
                type="button"
                ref={(button) => {
                  optionRefs.set(optionKeyForCustom(customModelCandidate()), button);
                }}
                onClick={() => handleSelect(customModelCandidate())}
                onKeyDown={(event) =>
                  handleOptionKeyDown(event, optionKeyForCustom(customModelCandidate()))
                }
                role="option"
                aria-selected={isSelectedRoute(customModelCandidate())}
                aria-label={optionAriaLabel(
                  `Use "${customModelCandidate()}"`,
                  isSelectedRoute(customModelCandidate()),
                  [props.customModelDescription || 'Custom model ID'],
                )}
                class={optionClass(isSelectedRoute(customModelCandidate()))}
              >
                <div class="flex min-w-0 items-center gap-2">
                  <span class="min-w-0 flex-1 truncate font-medium text-base-content">
                    Use "{customModelCandidate()}"
                  </span>
                  <Show when={isSelectedRoute(customModelCandidate())}>
                    <CurrentSelectionBadge />
                  </Show>
                </div>
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
                          ref={(button) => {
                            optionRefs.set(optionKeyForModel(entry.id), button);
                          }}
                          onClick={() => handleSelect(entry.id)}
                          onKeyDown={(event) =>
                            handleOptionKeyDown(event, optionKeyForModel(entry.id))
                          }
                          role="option"
                          aria-selected={isSelectedRoute(entry.id)}
                          aria-label={optionAriaLabel(
                            modelRouteLabel(entry),
                            isSelectedRoute(entry.id),
                            [modelRouteDescription(entry), modelRouteSecondaryId(entry)],
                          )}
                          class={optionClass(isSelectedRoute(entry.id))}
                        >
                          <div class="flex min-w-0 items-center gap-2">
                            <span class="min-w-0 flex-1 truncate font-medium text-base-content">
                              {modelRouteLabel(entry)}
                            </span>
                            <Show when={isSelectedRoute(entry.id)}>
                              <CurrentSelectionBadge />
                            </Show>
                          </div>
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
                    {(model) => {
                      const secondaryModelId = () =>
                        model.name &&
                        model.name !== model.id &&
                        !isPulseOwnedLocalModelRoute(model.id)
                          ? model.id
                          : undefined;
                      return (
                        <button
                          type="button"
                          ref={(button) => {
                            optionRefs.set(optionKeyForModel(model.id), button);
                          }}
                          onClick={() => handleSelect(model.id)}
                          onKeyDown={(event) =>
                            handleOptionKeyDown(event, optionKeyForModel(model.id))
                          }
                          role="option"
                          aria-selected={isSelectedRoute(model.id)}
                          aria-label={optionAriaLabel(
                            formatAIModelRouteLabel(model),
                            isSelectedRoute(model.id),
                            [model.description, secondaryModelId()],
                          )}
                          class={optionClass(isSelectedRoute(model.id))}
                        >
                          <div class="flex min-w-0 items-center gap-2">
                            <span class="min-w-0 flex-1 truncate font-medium text-base-content">
                              {formatAIModelRouteLabel(model)}
                            </span>
                            <Show when={isSelectedRoute(model.id)}>
                              <CurrentSelectionBadge />
                            </Show>
                          </div>
                          <Show when={model.description}>
                            <div class="line-clamp-2 text-[11px] text-muted">
                              {model.description}
                            </div>
                          </Show>
                          <Show when={secondaryModelId()}>
                            {(modelId) => <div class="text-[10px] text-muted">{modelId()}</div>}
                          </Show>
                        </button>
                      );
                    }}
                  </For>
                </>
              )}
            </For>

            <Show when={hiddenModelCount() > 0 && !searchQuery().trim()}>
              <div class="mt-1 border-t border-border pt-1">
                <button
                  type="button"
                  ref={(button) => {
                    optionRefs.set(SHOW_OLDER_MODELS_OPTION_KEY, button);
                  }}
                  onClick={() => setShowAllModels(!showAllModels())}
                  onKeyDown={(event) => handleOptionKeyDown(event, SHOW_OLDER_MODELS_OPTION_KEY)}
                  role="option"
                  aria-selected={false}
                  aria-label={showOlderModelsOptionLabel()}
                  class="flex w-full items-center gap-1.5 px-3 py-2 text-left text-xs text-muted hover:bg-surface-hover hover:text-base-content"
                >
                  <ChevronDownIcon
                    class={`h-3 w-3 transition-transform ${showAllModels() ? 'rotate-180' : ''}`}
                  />
                  {showOlderModelsOptionLabel()}
                </button>
              </div>
            </Show>
          </div>
        </div>
      </Show>
    </div>
  );
};
