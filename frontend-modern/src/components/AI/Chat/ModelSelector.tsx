import { Component, For, Show, createSignal, createMemo } from 'solid-js';
import { PROVIDER_DISPLAY_NAMES, getProviderFromModelId, groupModelsByProvider } from '../aiChatUtils';
import type { ModelInfo } from './types';

export interface ModelSelectorProps {
    models: ModelInfo[];
    selectedModel: string;
    defaultModelLabel?: string;
    chatOverrideModel?: string;
    chatOverrideLabel?: string;
    isLoading?: boolean;
    error?: string;
    onModelSelect: (modelId: string) => void;
    onRefresh?: () => void;
}

/**
 * Reusable model selector dropdown with notable model filtering.
 * Shows only recent/notable models by default with a toggle to reveal older models.
 */
export const ModelSelector: Component<ModelSelectorProps> = (props) => {
    const [isOpen, setIsOpen] = createSignal(false);
    const [showAllModels, setShowAllModels] = createSignal(false);
    const [searchQuery, setSearchQuery] = createSignal('');
    const [dropdownPosition, setDropdownPosition] = createSignal({ top: 0, right: 0 });
    let buttonRef: HTMLButtonElement | undefined;

    // Filter models by notable status (show only recent/notable models by default)
    const notableFilteredModels = createMemo(() => {
        if (showAllModels()) {
            return props.models;
        }
        const notable = props.models.filter(m => m.notable);
        return notable.length > 0 ? notable : props.models;
    });

    // Count hidden (older) models
    const hiddenModelCount = createMemo(() => {
        const notable = props.models.filter(m => m.notable);
        return props.models.length - notable.length;
    });

    // Apply search filter on top of notable filter
    const filteredModels = createMemo(() => {
        const query = searchQuery().trim().toLowerCase();
        const baseModels = notableFilteredModels();
        if (!query) return baseModels;
        return baseModels.filter((model) => {
            const provider = getProviderFromModelId(model.id);
            const providerName = PROVIDER_DISPLAY_NAMES[provider] || provider;
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

    // Check if typed query matches any model
    const customModelCandidate = createMemo(() => searchQuery().trim());
    const showCustomModelOption = createMemo(() => {
        const candidate = customModelCandidate();
        if (!candidate) return false;
        return !props.models.some((model) => model.id === candidate);
    });

    const updateDropdownPosition = () => {
        if (buttonRef) {
            const rect = buttonRef.getBoundingClientRect();
            setDropdownPosition({
                top: rect.bottom + 4, // 4px gap (mt-1)
                right: window.innerWidth - rect.right,
            });
        }
    };

    const handleToggle = () => {
        if (!isOpen()) {
            updateDropdownPosition();
        }
        setIsOpen(!isOpen());
    };

    const handleSelect = (modelId: string) => {
        props.onModelSelect(modelId);
        setIsOpen(false);
        setSearchQuery('');
    };

    const handleKeyDown = (e: KeyboardEvent) => {
        if (e.key !== 'Enter') return;
        e.preventDefault();
        const candidate = customModelCandidate();
        if (candidate) {
            handleSelect(candidate);
        }
    };

    const selectedLabel = createMemo(() => {
        const selected = props.selectedModel?.trim();
        if (!selected) {
            const fallback = props.defaultModelLabel;
            return fallback ? `Default (${fallback})` : 'Default';
        }
        const match = props.models.find((model) => model.id === selected);
        if (match) return match.name || match.id.split(':').pop() || match.id;
        return selected;
    });

    return (
        <div class="relative" data-dropdown>
            <button
                ref={buttonRef}
                onClick={handleToggle}
                class="flex items-center gap-1.5 px-2.5 py-1.5 text-[11px] text-slate-600 dark:text-slate-300 hover:text-slate-800 dark:hover:text-slate-100 rounded-lg border border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600 bg-white dark:bg-slate-800 transition-colors"
                title="Select model for this chat"
            >
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                </svg>
                <span class="max-w-[120px] truncate font-medium">{selectedLabel()}</span>
                <Show when={props.isLoading}>
                    <svg class="w-3 h-3 text-slate-400 animate-spin" fill="none" viewBox="0 0 24 24">
                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="3" />
                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                    </svg>
                </Show>
                <svg class="w-3 h-3 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
            </button>

            <Show when={isOpen()}>
                <div
                    class="fixed w-80 max-h-96 overflow-hidden bg-white dark:bg-slate-800 rounded-xl shadow-xl border border-slate-200 dark:border-slate-700 z-[9999]"
                    style={{ top: `${dropdownPosition().top}px`, right: `${dropdownPosition().right}px` }}
                >
                    {/* Search bar */}
                    <div class="flex items-center gap-2 px-3 py-2 border-b border-slate-200 dark:border-slate-700">
                        <input
                            type="text"
                            value={searchQuery()}
                            onInput={(e) => setSearchQuery(e.currentTarget.value)}
                            onKeyDown={handleKeyDown}
                            placeholder="Search or enter model ID"
                            class="flex-1 text-xs px-2 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-200 focus:outline-none focus:ring-2 focus:ring-purple-400/50"
                        />
                        <Show when={props.onRefresh}>
                            <button
                                type="button"
                                onClick={() => props.onRefresh?.()}
                                disabled={props.isLoading}
                                class="p-1.5 rounded-md text-slate-500 hover:text-slate-700 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-700 disabled:opacity-50"
                                title="Refresh models"
                            >
                                <svg class={`w-3.5 h-3.5 ${props.isLoading ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v6h6M20 20v-6h-6M5.32 9A7.5 7.5 0 0119 12.5M18.68 15A7.5 7.5 0 015 11.5" />
                                </svg>
                            </button>
                        </Show>
                    </div>

                    {/* Error message */}
                    <Show when={props.error}>
                        <div class="px-3 py-2 text-[11px] text-red-500 border-b border-slate-200 dark:border-slate-700">
                            {props.error}
                        </div>
                    </Show>

                    {/* Model list */}
                    <div class="max-h-72 overflow-y-auto py-1">
                        {/* Default option */}
                        <button
                            onClick={() => handleSelect('')}
                            class={`w-full px-3 py-2 text-left text-sm hover:bg-slate-50 dark:hover:bg-slate-700 ${!props.selectedModel ? 'bg-purple-50 dark:bg-purple-900/30' : ''}`}
                        >
                            <div class="font-medium text-slate-900 dark:text-slate-100">Default</div>
                            <div class="text-[11px] text-slate-500 dark:text-slate-400">
                                {props.defaultModelLabel ? `Use configured default model (${props.defaultModelLabel})` : 'Use configured default model'}
                            </div>
                        </button>

                        {/* Chat override option */}
                        <Show when={props.chatOverrideModel}>
                            <button
                                onClick={() => handleSelect(props.chatOverrideModel!)}
                                class={`w-full px-3 py-2 text-left text-sm hover:bg-slate-50 dark:hover:bg-slate-700 ${props.selectedModel === props.chatOverrideModel ? 'bg-purple-50 dark:bg-purple-900/30' : ''}`}
                            >
                                <div class="font-medium text-slate-900 dark:text-slate-100">Chat override</div>
                                <div class="text-[11px] text-slate-500 dark:text-slate-400">
                                    {props.chatOverrideLabel || props.chatOverrideModel}
                                </div>
                            </button>
                        </Show>

                        {/* Custom model option */}
                        <Show when={showCustomModelOption()}>
                            <button
                                onClick={() => handleSelect(customModelCandidate())}
                                class="w-full px-3 py-2 text-left text-sm hover:bg-slate-50 dark:hover:bg-slate-700"
                            >
                                <div class="font-medium text-slate-900 dark:text-slate-100">
                                    Use "{customModelCandidate()}"
                                </div>
                                <div class="text-[11px] text-slate-500 dark:text-slate-400">Custom model ID</div>
                            </button>
                        </Show>

                        {/* No results */}
                        <Show when={!props.isLoading && filteredModels().length === 0}>
                            <div class="px-3 py-4 text-center text-[11px] text-slate-500 dark:text-slate-400">
                                No matching models.
                            </div>
                        </Show>

                        {/* Grouped models */}
                        <For each={Array.from(groupModelsByProvider(filteredModels()).entries())}>
                            {([provider, providerModels]) => (
                                <>
                                    <div class="px-3 py-1.5 text-[11px] font-semibold text-slate-500 dark:text-slate-400 bg-slate-50 dark:bg-slate-700/50 sticky top-0">
                                        {PROVIDER_DISPLAY_NAMES[provider] || provider}
                                    </div>
                                    <For each={providerModels}>
                                        {(model) => (
                                            <button
                                                onClick={() => handleSelect(model.id)}
                                                class={`w-full px-3 py-2 text-left text-sm hover:bg-slate-50 dark:hover:bg-slate-700 ${props.selectedModel === model.id ? 'bg-purple-50 dark:bg-purple-900/30' : ''}`}
                                            >
                                                <div class="flex items-center gap-1.5">
                                                    <span class="font-medium text-slate-900 dark:text-slate-100">
                                                        {model.name || model.id.split(':').pop() || model.id}
                                                    </span>
                                                </div>
                                                <Show when={model.description}>
                                                    <div class="text-[11px] text-slate-500 dark:text-slate-400 line-clamp-2">
                                                        {model.description}
                                                    </div>
                                                </Show>
                                                <Show when={model.name && model.name !== model.id}>
                                                    <div class="text-[10px] text-slate-400 dark:text-slate-500">
                                                        {model.id}
                                                    </div>
                                                </Show>
                                            </button>
                                        )}
                                    </For>
                                </>
                            )}
                        </For>

                        {/* Toggle to show older models */}
                        <Show when={hiddenModelCount() > 0 && !searchQuery().trim()}>
                            <div class="border-t border-slate-200 dark:border-slate-700 mt-1 pt-1">
                                <button
                                    onClick={() => setShowAllModels(!showAllModels())}
                                    class="w-full px-3 py-2 text-left text-xs text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700 flex items-center gap-1.5"
                                >
                                    <svg class={`w-3 h-3 transition-transform ${showAllModels() ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                                    </svg>
                                    {showAllModels() ? 'Hide older models' : `Show ${hiddenModelCount()} older models`}
                                </button>
                            </div>
                        </Show>
                    </div>
                </div>
            </Show>
        </div>
    );
};
