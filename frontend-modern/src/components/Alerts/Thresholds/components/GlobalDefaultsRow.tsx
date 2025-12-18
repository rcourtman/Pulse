/**
 * GlobalDefaultsRow Component
 *
 * Displays and allows editing of global default thresholds for a resource type.
 * Shows at the top of each section with editable threshold badges.
 */

import { Component, Show, For, createSignal, createEffect, createMemo } from 'solid-js';
import Settings from 'lucide-solid/icons/settings';
import RotateCcw from 'lucide-solid/icons/rotate-ccw';
import Check from 'lucide-solid/icons/check';
import X from 'lucide-solid/icons/x';
import { ThresholdBadge } from './ThresholdBadge';
import type { ThresholdColumn, ThresholdValues } from '../types';

export interface GlobalDefaultsRowProps {
    /** Current default values */
    defaults: ThresholdValues;
    /** Factory defaults for reset */
    factoryDefaults?: ThresholdValues;
    /** Column definitions */
    columns: ThresholdColumn[];
    /** Called when defaults change */
    onUpdateDefaults: (
        value: ThresholdValues | ((prev: ThresholdValues) => ThresholdValues)
    ) => void;
    /** Called when changes are made */
    setHasUnsavedChanges: (value: boolean) => void;
    /** Called to reset to factory defaults */
    onResetDefaults?: () => void;
    /** Whether to show connectivity/offline settings */
    showOfflineSettings?: boolean;
    /** Current offline alert state */
    disableConnectivity?: boolean;
    offlineSeverity?: 'warning' | 'critical';
    onSetOfflineState?: (state: 'off' | 'warning' | 'critical') => void;
    /** Whether all resources of this type are disabled */
    globalDisabled?: boolean;
    onToggleGlobalDisabled?: () => void;
    /** Title override */
    title?: string;
}

export const GlobalDefaultsRow: Component<GlobalDefaultsRowProps> = (props) => {
    const [isEditing, setIsEditing] = createSignal(false);
    const [editingValues, setEditingValues] = createSignal<ThresholdValues>({});

    // Check if current defaults differ from factory defaults
    const hasCustomDefaults = createMemo(() => {
        if (!props.factoryDefaults) return false;
        return props.columns.some((col) => {
            const current = props.defaults[col.key];
            const factory = props.factoryDefaults?.[col.key];
            return current !== factory;
        });
    });

    // Start editing
    const startEditing = () => {
        setEditingValues({ ...props.defaults });
        setIsEditing(true);
    };

    // Save changes
    const saveEdit = () => {
        props.onUpdateDefaults(editingValues());
        props.setHasUnsavedChanges(true);
        setIsEditing(false);
    };

    // Cancel editing
    const cancelEdit = () => {
        setEditingValues({});
        setIsEditing(false);
    };

    // Update a single threshold value
    const updateValue = (metric: string, value: number | undefined) => {
        setEditingValues((prev) => ({
            ...prev,
            [metric]: value,
        }));
    };

    // Handle input change
    const handleInput = (metric: string, e: Event) => {
        const input = e.target as HTMLInputElement;
        const value = input.value === '' ? undefined : Number(input.value);
        updateValue(metric, value);
    };

    // Get the display value for a metric
    const getDisplayValue = (metric: string) => {
        if (isEditing()) {
            return editingValues()[metric];
        }
        return props.defaults[metric];
    };

    return (
        <div
            class={`
        rounded-lg border-2 border-dashed transition-all duration-200
        ${isEditing()
                    ? 'border-blue-400 bg-blue-50/50 dark:bg-blue-950/20'
                    : 'border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/50'
                }
      `}
        >
            <div class="px-4 py-3">
                <div class="flex items-center justify-between gap-4 mb-3">
                    {/* Title */}
                    <div class="flex items-center gap-2">
                        <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300">
                            {props.title || 'Global Defaults'}
                        </h4>
                        <Show when={hasCustomDefaults()}>
                            <span class="px-1.5 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-400">
                                Modified
                            </span>
                        </Show>
                        <Show when={props.globalDisabled}>
                            <span class="px-1.5 py-0.5 rounded text-xs font-medium bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-400">
                                All Disabled
                            </span>
                        </Show>
                    </div>

                    {/* Actions */}
                    <div class="flex items-center gap-2">
                        {/* Toggle all disabled */}
                        <Show when={props.onToggleGlobalDisabled}>
                            <button
                                type="button"
                                onClick={props.onToggleGlobalDisabled}
                                class={`
                  px-2 py-1 text-xs font-medium rounded transition-colors
                  ${props.globalDisabled
                                        ? 'bg-yellow-100 text-yellow-700 hover:bg-yellow-200 dark:bg-yellow-900/40 dark:text-yellow-400'
                                        : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-400'
                                    }
                `}
                                title={props.globalDisabled ? 'Enable all alerts' : 'Disable all alerts'}
                            >
                                {props.globalDisabled ? 'Enable All' : 'Disable All'}
                            </button>
                        </Show>

                        {/* Reset to factory defaults */}
                        <Show when={props.onResetDefaults && hasCustomDefaults() && !isEditing()}>
                            <button
                                type="button"
                                onClick={props.onResetDefaults}
                                class="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-gray-600 hover:text-gray-800 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700 rounded transition-colors"
                                title="Reset to factory defaults"
                            >
                                <RotateCcw class="w-3 h-3" />
                                Reset
                            </button>
                        </Show>

                        {/* Edit/Save/Cancel */}
                        <Show
                            when={isEditing()}
                            fallback={
                                <button
                                    type="button"
                                    onClick={startEditing}
                                    class="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-blue-600 hover:text-blue-700 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-900/30 rounded transition-colors"
                                >
                                    <Settings class="w-3 h-3" />
                                    Edit Defaults
                                </button>
                            }
                        >
                            <button
                                type="button"
                                onClick={cancelEdit}
                                class="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-gray-600 hover:text-gray-800 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700 rounded transition-colors"
                            >
                                <X class="w-3 h-3" />
                                Cancel
                            </button>
                            <button
                                type="button"
                                onClick={saveEdit}
                                class="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-white bg-blue-600 hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-600 rounded transition-colors"
                            >
                                <Check class="w-3 h-3" />
                                Save
                            </button>
                        </Show>
                    </div>
                </div>

                {/* Threshold Values */}
                <div class="flex flex-wrap items-center gap-4">
                    {/* Threshold badges/inputs */}
                    <div class="flex flex-wrap items-center gap-2">
                        <For each={props.columns}>
                            {(column) => {
                                const metric = column.key;
                                const value = getDisplayValue(metric);
                                const factoryValue = props.factoryDefaults?.[metric];
                                const isModified = factoryValue !== undefined && value !== factoryValue;

                                return (
                                    <Show
                                        when={isEditing()}
                                        fallback={
                                            <ThresholdBadge
                                                metric={metric}
                                                value={value}
                                                defaultValue={factoryValue}
                                                isOverridden={isModified}
                                                onClick={startEditing}
                                                size="md"
                                                showLabel={true}
                                                label={column.label.replace(' %', '').replace(' °C', '').replace(' MB/s', '')}
                                            />
                                        }
                                    >
                                        <div class="flex items-center gap-1.5">
                                            <label class="text-xs font-medium text-gray-600 dark:text-gray-400">
                                                {column.label.replace(' %', '').replace(' °C', '').replace(' MB/s', '')}
                                            </label>
                                            <div class="relative">
                                                <input
                                                    type="number"
                                                    value={editingValues()[metric] ?? ''}
                                                    onInput={(e) => handleInput(metric, e)}
                                                    placeholder={factoryValue !== undefined ? String(factoryValue) : '—'}
                                                    class="
                            w-16 px-2 py-1 text-sm text-center rounded-md border border-gray-300 dark:border-gray-600
                            bg-white dark:bg-gray-900
                            focus:border-blue-500 focus:ring-1 focus:ring-blue-500/20
                            placeholder:text-gray-400
                          "
                                                />
                                            </div>
                                        </div>
                                    </Show>
                                );
                            }}
                        </For>
                    </div>

                    {/* Offline alerts settings */}
                    <Show when={props.showOfflineSettings && props.onSetOfflineState}>
                        <div class="flex items-center gap-2 pl-4 border-l border-gray-300 dark:border-gray-600">
                            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
                                Offline:
                            </span>
                            <For each={['off' as const, 'warning' as const, 'critical' as const]}>
                                {(state) => {
                                    const currentState = props.disableConnectivity
                                        ? 'off'
                                        : props.offlineSeverity || 'warning';
                                    const isActive = currentState === state;

                                    return (
                                        <button
                                            type="button"
                                            onClick={() => props.onSetOfflineState?.(state)}
                                            class={`
                        px-2 py-0.5 rounded text-xs font-medium transition-all
                        ${isActive
                                                    ? state === 'off'
                                                        ? 'bg-gray-200 text-gray-700 dark:bg-gray-600 dark:text-gray-200'
                                                        : state === 'warning'
                                                            ? 'bg-yellow-200 text-yellow-800 dark:bg-yellow-800/50 dark:text-yellow-300'
                                                            : 'bg-red-200 text-red-800 dark:bg-red-800/50 dark:text-red-300'
                                                    : 'bg-gray-100 text-gray-500 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-400 dark:hover:bg-gray-600'
                                                }
                      `}
                                        >
                                            {state === 'off' ? 'Off' : state === 'warning' ? 'Warn' : 'Crit'}
                                        </button>
                                    );
                                }}
                            </For>
                        </div>
                    </Show>
                </div>
            </div>
        </div>
    );
};

export default GlobalDefaultsRow;
