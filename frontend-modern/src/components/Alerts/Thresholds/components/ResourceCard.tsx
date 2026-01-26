/**
 * ResourceCard Component
 *
 * An expandable card for displaying and editing resource thresholds.
 * Replaces wide table rows with a compact, mobile-friendly design.
 */

import { Component, Show, For, createSignal, createMemo } from 'solid-js';
import ChevronDown from 'lucide-solid/icons/chevron-down';
import ChevronUp from 'lucide-solid/icons/chevron-up';
import Settings from 'lucide-solid/icons/settings';
import RotateCcw from 'lucide-solid/icons/rotate-ccw';
import Check from 'lucide-solid/icons/check';
import X from 'lucide-solid/icons/x';
import Bell from 'lucide-solid/icons/bell';
import BellOff from 'lucide-solid/icons/bell-off';

import ExternalLink from 'lucide-solid/icons/external-link';
import StickyNote from 'lucide-solid/icons/sticky-note';
import { ThresholdBadge, ThresholdBadgeGroup } from './ThresholdBadge';
import type { ThresholdResource, ThresholdColumn } from '../types';

export interface ResourceCardProps {
    /** The resource to display */
    resource: ThresholdResource;
    /** Column definitions for thresholds */
    columns: ThresholdColumn[];
    /** Whether this card is currently being edited */
    isEditing: boolean;
    /** Current editing threshold values */
    editingThresholds: Record<string, number | undefined>;
    /** Current editing note value */
    editingNote: string;
    /** Callbacks */
    onStartEdit: () => void;
    onSaveEdit: () => void;
    onCancelEdit: () => void;
    onUpdateThreshold: (metric: string, value: number | undefined) => void;
    onUpdateNote: (note: string) => void;
    onToggleDisabled?: () => void;
    onToggleConnectivity?: () => void;
    onSetOfflineState?: (state: 'off' | 'warning' | 'critical') => void;
    onRemoveOverride?: () => void;
    /** Format a threshold value for display */
    formatValue: (metric: string, value: number | undefined) => string;
    /** Check if there's an active alert for this resource/metric */
    hasActiveAlert: (resourceId: string, metric: string) => boolean;
    /** Whether to show offline alerts column */
    showOfflineAlerts?: boolean;
    /** Global defaults for comparison */
    globalDefaults?: Record<string, number | undefined>;
}

/**
 * Get status indicator color
 */
const getStatusColor = (status?: string): string => {
    switch (status?.toLowerCase()) {
        case 'running':
        case 'online':
            return 'bg-green-500';
        case 'stopped':
        case 'offline':
            return 'bg-red-500';
        case 'paused':
            return 'bg-yellow-500';
        default:
            return 'bg-gray-400';
    }
};

/**
 * Get human-readable status text
 */
const getStatusText = (status?: string): string => {
    switch (status?.toLowerCase()) {
        case 'running':
            return 'Running';
        case 'online':
            return 'Online';
        case 'stopped':
            return 'Stopped';
        case 'offline':
            return 'Offline';
        case 'paused':
            return 'Paused';
        default:
            return status || 'Unknown';
    }
};

export const ResourceCard: Component<ResourceCardProps> = (props) => {
    const [isExpanded, setIsExpanded] = createSignal(false);

    // When editing starts, expand the card
    const expanded = createMemo(() => props.isEditing || isExpanded());

    // Determine which metrics to show in collapsed view
    const primaryMetrics = createMemo(() => {
        // Show first 3-4 metrics in collapsed view
        return props.columns.slice(0, 4).map((col) => col.key);
    });

    // Check if resource has any custom overrides
    const hasCustomSettings = () => props.resource.hasOverride;

    // Get the effective threshold value (editing or current)
    const getEffectiveValue = (metric: string) => {
        if (props.isEditing) {
            return props.editingThresholds[metric];
        }
        return props.resource.thresholds[metric] ?? props.resource.defaults[metric];
    };

    // Check if a metric is overridden from defaults
    const isOverridden = (metric: string) => {
        const current = props.resource.thresholds[metric];
        const defaultVal = props.resource.defaults[metric];
        return current !== undefined && current !== defaultVal;
    };

    // Handle expanding/collapsing
    const toggleExpand = () => {
        if (!props.isEditing) {
            setIsExpanded(!isExpanded());
        }
    };

    // Handle input change for threshold
    const handleThresholdInput = (metric: string, e: Event) => {
        const input = e.target as HTMLInputElement;
        const value = input.value === '' ? undefined : Number(input.value);
        props.onUpdateThreshold(metric, value);
    };

    return (
        <div
            class={`
        rounded-lg border transition-all duration-200
        ${props.isEditing
                    ? 'border-blue-400 ring-2 ring-blue-400/20 dark:ring-blue-400/10 bg-blue-50/50 dark:bg-blue-950/20'
                    : 'border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800'
                }
        ${props.resource.disabled
                    ? 'opacity-60'
                    : ''
                }
      `}
            data-testid={`resource-card-${props.resource.id}`}
        >
            {/* Collapsed Header */}
            <div
                class={`
          flex items-center justify-between gap-3 px-4 py-3
          ${!props.isEditing ? 'cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/30' : ''}
          transition-colors duration-150
        `}
                onClick={!props.isEditing ? toggleExpand : undefined}
            >
                {/* Left: Status + Name + Badges */}
                <div class="flex items-center gap-3 min-w-0 flex-1">
                    {/* Status indicator */}
                    <span
                        class={`flex-shrink-0 w-2.5 h-2.5 rounded-full ${getStatusColor(props.resource.status)}`}
                        title={getStatusText(props.resource.status)}
                    />

                    {/* Resource info */}
                    <div class="min-w-0 flex-1">
                        <div class="flex items-center gap-2 flex-wrap">
                            {/* Name */}
                            <span class="font-medium text-gray-900 dark:text-gray-100 truncate">
                                {props.resource.displayName || props.resource.name}
                            </span>

                            {/* Type badge */}
                            <Show when={props.resource.resourceType}>
                                <span class="flex-shrink-0 px-1.5 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400">
                                    {props.resource.resourceType}
                                </span>
                            </Show>

                            {/* Node badge for storage resources */}
                            <Show when={props.resource.type === 'storage' && props.resource.node}>
                                <span class="flex-shrink-0 px-1.5 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300">
                                    {props.resource.node}
                                </span>
                            </Show>

                            {/* Custom badge */}
                            <Show when={hasCustomSettings()}>
                                <span class="flex-shrink-0 px-1.5 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-400">
                                    Custom
                                </span>
                            </Show>

                            {/* Has note indicator */}
                            <Show when={props.resource.note}>
                                <span title={props.resource.note}>
                                    <StickyNote class="w-3.5 h-3.5 text-yellow-500" />
                                </span>
                            </Show>

                            {/* Disabled badge */}
                            <Show when={props.resource.disabled}>
                                <span class="flex-shrink-0 px-1.5 py-0.5 rounded text-xs font-medium bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-400">
                                    Alerts Off
                                </span>
                            </Show>
                        </div>

                        {/* Subtitle with node/instance info */}
                        <Show when={props.resource.node || props.resource.vmid}>
                            <p class="text-xs text-gray-500 dark:text-gray-400 truncate mt-0.5">
                                <Show when={props.resource.vmid}>
                                    ID {props.resource.vmid}
                                </Show>
                                <Show when={props.resource.vmid && props.resource.node}> • </Show>
                                <Show when={props.resource.node}>
                                    {props.resource.node}
                                </Show>
                            </p>
                        </Show>
                    </div>
                </div>

                {/* Center: Threshold badges (collapsed view) */}
                <Show when={!expanded()}>
                    <div class="hidden sm:flex items-center gap-1 flex-shrink-0">
                        <ThresholdBadgeGroup
                            thresholds={props.resource.thresholds}
                            defaults={props.resource.defaults}
                            metrics={primaryMetrics()}
                            hasActiveAlert={(metric) => props.hasActiveAlert(props.resource.id, metric)}
                            size="sm"
                            maxVisible={4}
                        />
                    </div>
                </Show>

                {/* Right: Actions */}
                <div class="flex items-center gap-2 flex-shrink-0">
                    {/* Alert toggle */}
                    <Show when={props.onToggleDisabled && !props.isEditing}>
                        <button
                            type="button"
                            onClick={(e) => {
                                e.stopPropagation();
                                props.onToggleDisabled?.();
                            }}
                            class={`
                p-1.5 rounded-md transition-colors
                ${props.resource.disabled
                                    ? 'text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                                    : 'text-green-600 hover:text-green-700 hover:bg-green-50 dark:text-green-400 dark:hover:bg-green-900/30'
                                }
              `}
                            title={props.resource.disabled ? 'Enable alerts' : 'Disable alerts'}
                        >
                            <Show when={props.resource.disabled} fallback={<Bell class="w-4 h-4" />}>
                                <BellOff class="w-4 h-4" />
                            </Show>
                        </button>
                    </Show>

                    {/* Expand/Collapse */}
                    <button
                        type="button"
                        onClick={(e) => {
                            e.stopPropagation();
                            if (!props.isEditing) {
                                setIsExpanded(!isExpanded());
                            }
                        }}
                        class="p-1.5 rounded-md text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:text-gray-300 dark:hover:bg-gray-700 transition-colors"
                        title={expanded() ? 'Collapse' : 'Expand to edit'}
                    >
                        <Show when={expanded()} fallback={<ChevronDown class="w-4 h-4" />}>
                            <ChevronUp class="w-4 h-4" />
                        </Show>
                    </button>
                </div>
            </div>

            {/* Expanded Content */}
            <Show when={expanded()}>
                <div class="px-4 pb-4 pt-2 border-t border-gray-200 dark:border-gray-700 space-y-4">
                    {/* Threshold Grid */}
                    <div>
                        <h4 class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2">
                            Thresholds
                        </h4>
                        <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
                            <For each={props.columns}>
                                {(column) => {
                                    const metric = column.key;
                                    const value = getEffectiveValue(metric);
                                    const defaultVal = props.resource.defaults[metric];
                                    const hasAlert = props.hasActiveAlert(props.resource.id, metric);

                                    return (
                                        <div class="space-y-1">
                                            <label class="text-xs font-medium text-gray-600 dark:text-gray-400">
                                                {column.label}
                                            </label>
                                            <Show
                                                when={props.isEditing}
                                                fallback={
                                                    <ThresholdBadge
                                                        metric={metric}
                                                        value={value}
                                                        defaultValue={defaultVal}
                                                        isOverridden={isOverridden(metric)}
                                                        hasAlert={hasAlert}
                                                        onClick={props.onStartEdit}
                                                        size="md"
                                                    />
                                                }
                                            >
                                                <div class="relative">
                                                    <input
                                                        type="number"
                                                        value={props.editingThresholds[metric] ?? ''}
                                                        onInput={(e) => handleThresholdInput(metric, e)}
                                                        placeholder={defaultVal !== undefined ? String(defaultVal) : 'Off'}
                                                        class={`
                              w-full px-2.5 py-1.5 text-sm rounded-md border
                              bg-white dark:bg-gray-900
                              ${hasAlert
                                                                ? 'border-red-400 ring-1 ring-red-400'
                                                                : 'border-gray-300 dark:border-gray-600'
                                                            }
                              focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20
                              placeholder:text-gray-400 dark:placeholder:text-gray-500
                            `}
                                                    />
                                                    <Show when={column.unit}>
                                                        <span class="absolute right-2.5 top-1/2 -translate-y-1/2 text-xs text-gray-400">
                                                            {column.unit}
                                                        </span>
                                                    </Show>
                                                </div>
                                            </Show>
                                        </div>
                                    );
                                }}
                            </For>
                        </div>
                    </div>

                    {/* Offline Alerts Section */}
                    <Show when={props.showOfflineAlerts && props.onSetOfflineState}>
                        <div>
                            <h4 class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2">
                                Offline Alerts
                            </h4>
                            <div class="flex items-center gap-2">
                                <For each={['off' as const, 'warning' as const, 'critical' as const]}>
                                    {(state) => {
                                        const currentState = props.resource.disableConnectivity
                                            ? 'off'
                                            : props.resource.poweredOffSeverity || 'warning';
                                        const isActive = currentState === state;

                                        const colors = {
                                            off: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400',
                                            warning: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-400',
                                            critical: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-400',
                                        };

                                        return (
                                            <button
                                                type="button"
                                                onClick={() => props.onSetOfflineState?.(state)}
                                                class={`
                          px-3 py-1.5 rounded-md text-sm font-medium transition-all
                          ${isActive
                                                        ? `${colors[state]} ring-2 ring-offset-2 ring-blue-500 dark:ring-offset-gray-800`
                                                        : 'bg-gray-50 text-gray-500 hover:bg-gray-100 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700'
                                                    }
                        `}
                                            >
                                                {state === 'off' ? 'Off' : state === 'warning' ? 'Warning' : 'Critical'}
                                            </button>
                                        );
                                    }}
                                </For>
                            </div>
                        </div>
                    </Show>

                    {/* Note Field */}
                    <Show when={props.isEditing}>
                        <div>
                            <label class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">
                                Note
                            </label>
                            <textarea
                                value={props.editingNote}
                                onInput={(e) => props.onUpdateNote(e.currentTarget.value)}
                                placeholder="Add a note about this resource..."
                                rows={2}
                                class="
                  w-full mt-1 px-3 py-2 text-sm rounded-md border border-gray-300 dark:border-gray-600
                  bg-white dark:bg-gray-900
                  focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20
                  placeholder:text-gray-400 dark:placeholder:text-gray-500
                  resize-none
                "
                            />
                        </div>
                    </Show>

                    {/* Action Buttons */}
                    <div class="flex items-center justify-between pt-2 border-t border-gray-200 dark:border-gray-700">
                        <div class="flex items-center gap-2">
                            {/* Host link */}
                            <Show when={props.resource.host}>
                                <a
                                    href={props.resource.host}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    class="inline-flex items-center gap-1 px-2 py-1 text-xs text-blue-600 hover:text-blue-700 dark:text-blue-400"
                                >
                                    <ExternalLink class="w-3 h-3" />
                                    Open
                                </a>
                            </Show>

                            {/* Reset to defaults */}
                            <Show when={hasCustomSettings() && props.onRemoveOverride}>
                                <button
                                    type="button"
                                    onClick={() => props.onRemoveOverride?.()}
                                    class="inline-flex items-center gap-1 px-2 py-1 text-xs text-gray-600 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
                                >
                                    <RotateCcw class="w-3 h-3" />
                                    Reset
                                </button>
                            </Show>
                        </div>

                        <Show
                            when={props.isEditing}
                            fallback={
                                <button
                                    type="button"
                                    onClick={props.onStartEdit}
                                    class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-blue-600 hover:text-blue-700 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-900/30 rounded-md transition-colors"
                                >
                                    <Settings class="w-4 h-4" />
                                    Edit
                                </button>
                            }
                        >
                            <div class="flex items-center gap-2">
                                <button
                                    type="button"
                                    onClick={props.onCancelEdit}
                                    class="inline-flex items-center gap-1 px-3 py-1.5 text-sm font-medium text-gray-600 hover:text-gray-800 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700 rounded-md transition-colors"
                                >
                                    <X class="w-4 h-4" />
                                    Cancel
                                </button>
                                <button
                                    type="button"
                                    onClick={props.onSaveEdit}
                                    class="inline-flex items-center gap-1 px-3 py-1.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-600 rounded-md transition-colors"
                                >
                                    <Check class="w-4 h-4" />
                                    Save
                                </button>
                            </div>
                        </Show>
                    </div>
                </div>
            </Show>
        </div>
    );
};

export default ResourceCard;
