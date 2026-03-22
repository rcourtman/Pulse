import { For, Show } from 'solid-js';
import RotateCcw from 'lucide-solid/icons/rotate-ccw';

import { Card } from '@/components/shared/Card';
import { TogglePrimitive } from '@/components/shared/Toggle';
import { AlertResourceGroupHeader } from './AlertResourceGroupHeader';
import {
  getAlertResourceTableCustomBadgeLabel,
  getAlertResourceTableEditNotePlaceholder,
  getAlertResourceTableEmptyState,
  getAlertResourceTableMetricPlaceholder,
  getAlertResourceTableRevertToDefaultsLabel,
} from '@/utils/alertResourceTablePresentation';
import {
  alertResourceSupportsMetric,
  buildAlertResourceEditPayload,
  getAlertResourceEnabledDefault,
  getAlertResourceLabel,
  getAlertResourceMetricBounds,
  getAlertResourceMetricDisplayValue,
  getAlertResourceMetricStep,
  isAlertResourceMetricOverridden,
  normalizeAlertResourceMetricKey,
} from './alertResourceTableModel';
import type { Resource } from '@/features/alerts/thresholds/tableTypes';
import type { ResourceTableProps } from './ResourceTable';

interface AlertResourceTableMobileProps {
  table: ResourceTableProps;
  hasRows: () => boolean;
  hasCustomGlobalDefaults: () => boolean;
  setActiveMetricInput: (value: { resourceId: string; metric: string } | null) => void;
}

export function AlertResourceTableMobile(props: AlertResourceTableMobileProps) {
  const getThresholds = (resource: Resource, isEditing: boolean) =>
    isEditing ? props.table.editingThresholds() : (resource.thresholds ?? {});

  const getDisplayValue = (resource: Resource, metric: string, isEditing: boolean) =>
    getAlertResourceMetricDisplayValue(
      resource,
      metric,
      props.table.editingThresholds(),
      isEditing,
    );

  const startEditing = (resource: Resource, metric?: string, event?: MouseEvent) => {
    event?.stopPropagation();
    if (resource.editable === false) {
      return;
    }
    if (metric) {
      props.setActiveMetricInput({ resourceId: resource.id, metric });
    }
    const payload = buildAlertResourceEditPayload(resource);
    props.table.onEdit(resource.id, payload.thresholds, payload.defaults, payload.note);
  };

  const cancelEditing = () => {
    props.table.onCancelEdit();
    props.setActiveMetricInput(null);
  };

  const saveEditing = (resourceId: string) => {
    props.table.onSaveEdit(resourceId);
    props.setActiveMetricInput(null);
  };

  const MetricValueWithHeat = (metricProps: {
    resourceId: string;
    metric: string;
    value: number;
    isOverridden: boolean;
  }) => {
    const isDisabledMetric = metricProps.value <= 0;
    const displayText = isDisabledMetric
      ? 'Off'
      : props.table.formatMetricValue(metricProps.metric, metricProps.value);

    return (
      <div
        class={`flex items-center justify-center gap-1 ${isDisabledMetric ? 'opacity-60' : ''}`.trim()}
        title={isDisabledMetric ? 'Disabled (no alerts for this metric)' : ''}
      >
        <span
          class={`text-sm ${
            isDisabledMetric
              ? 'text-muted italic'
              : metricProps.isOverridden
                ? 'text-base-content font-bold'
                : 'text-base-content'
          }`}
        >
          {displayText}
        </span>
        <Show when={props.table.hasActiveAlert(metricProps.resourceId, metricProps.metric)}>
          <div class="w-1.5 h-1.5 rounded-full bg-red-500 animate-pulse" title="Active alert" />
        </Show>
      </div>
    );
  };

  return (
    <div class="space-y-4">
      <Show
        when={
          props.table.globalDefaults &&
          props.table.setGlobalDefaults &&
          props.table.setHasUnsavedChanges
        }
      >
        <Card
          padding="sm"
          class="border border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900"
        >
          <div class="flex justify-between items-center mb-3">
            <div class="flex items-center gap-2">
              <span class="font-semibold text-sm">Global Defaults</span>
              <Show when={props.hasCustomGlobalDefaults()}>
                <span class="text-[10px] px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                  {getAlertResourceTableCustomBadgeLabel()}
                </span>
              </Show>
            </div>
            <Show when={props.table.onToggleGlobalDisable}>
              <TogglePrimitive
                size="sm"
                checked={!props.table.globalDisableFlag?.()}
                onToggle={() => {
                  props.table.onToggleGlobalDisable?.();
                  props.table.setHasUnsavedChanges?.(true);
                }}
              />
            </Show>
          </div>

          <div class="grid grid-cols-2 gap-2">
            <For
              each={props.table.columns.filter((column) => {
                const metric = normalizeAlertResourceMetricKey(column);
                return metric !== 'backup' && metric !== 'snapshot';
              })}
            >
              {(column) => {
                const metric = normalizeAlertResourceMetricKey(column);
                const bounds = getAlertResourceMetricBounds(metric);
                const value = () => props.table.globalDefaults?.[metric] ?? 0;
                const isOff = () => value() === -1;

                return (
                  <div class="p-2 bg-surface rounded border border-border-subtle flex flex-col gap-1">
                    <span class="text-[10px] uppercase text-slate-500 font-medium">{column}</span>
                    <div class="relative">
                      <input
                        type="number"
                        min={bounds.min}
                        max={bounds.max}
                        step={getAlertResourceMetricStep(metric)}
                        value={isOff() ? '' : value()}
                        placeholder={getAlertResourceTableMetricPlaceholder(isOff())}
                        disabled={isOff()}
                        class={`w-full text-sm p-1 rounded border text-center ${isOff() ? 'bg-surface-hover' : ' border-border'}`}
                        onInput={(e) => {
                          const nextValue = parseFloat(e.currentTarget.value);
                          props.table.setGlobalDefaults?.((prev) => ({
                            ...prev,
                            [metric]: Number.isNaN(nextValue) ? 0 : nextValue,
                          }));
                          props.table.setHasUnsavedChanges?.(true);
                        }}
                      />
                      <Show when={isOff()}>
                        <button
                          type="button"
                          class="absolute inset-0 w-full"
                          onClick={() => {
                            props.table.setGlobalDefaults?.((prev) => ({
                              ...prev,
                              [metric]: getAlertResourceEnabledDefault(metric),
                            }));
                            props.table.setHasUnsavedChanges?.(true);
                          }}
                          aria-label={`Enable ${column} default`}
                        />
                      </Show>
                    </div>
                  </div>
                );
              }}
            </For>
          </div>
        </Card>
      </Show>

      <For
        each={
          props.table.groupedResources
            ? Object.entries(props.table.groupedResources).sort(([a], [b]) => a.localeCompare(b))
            : [['default', props.table.resources || []] as [string, Resource[]]]
        }
      >
        {([groupName, resources]) => (
          <div class="space-y-2">
            <Show when={props.table.groupedResources}>
              <div class="px-1 font-medium text-xs text-slate-500 uppercase mt-4 mb-1">
                <AlertResourceGroupHeader
                  groupKey={groupName}
                  meta={props.table.groupHeaderMeta?.[groupName]}
                />
              </div>
            </Show>
            <For each={resources as Resource[]}>
              {(resource) => {
                const isEditing = () => props.table.editingId() === resource.id;
                const thresholds = () => getThresholds(resource, isEditing());
                const displayValue = (metric: string) =>
                  getDisplayValue(resource, metric, isEditing());
                const isOverridden = (metric: string) =>
                  isAlertResourceMetricOverridden(resource, metric);

                return (
                  <Card
                    padding="sm"
                    class={`flex flex-col gap-3 transition-opacity ${resource.disabled || props.table.globalDisableFlag?.() ? 'opacity-60' : ''}`}
                  >
                    <div class="flex items-center justify-between">
                      <div class="flex items-center gap-3 min-w-0">
                        <Show when={props.table.onToggleDisabled}>
                          <div class="shrink-0 scale-90 origin-left">
                            <TogglePrimitive
                              size="sm"
                              checked={
                                !(props.table.globalDisableFlag?.() ?? false) && !resource.disabled
                              }
                              disabled={props.table.globalDisableFlag?.() ?? false}
                              onToggle={() =>
                                !(props.table.globalDisableFlag?.() ?? false) &&
                                props.table.onToggleDisabled?.(resource.id)
                              }
                            />
                          </div>
                        </Show>

                        <div class="min-w-0 truncate">
                          <div class="font-medium text-sm truncate">
                            {getAlertResourceLabel(resource)}
                          </div>
                          <Show when={resource.subtitle}>
                            <div class="text-xs text-slate-500 truncate">{resource.subtitle}</div>
                          </Show>
                        </div>
                      </div>

                      <div class="flex gap-1 shrink-0">
                        <Show when={!isEditing() && resource.type !== 'dockerHost'}>
                          <button
                            type="button"
                            onClick={() => startEditing(resource)}
                            class="p-1.5 bg-blue-50 dark:bg-blue-900 text-blue-600 rounded"
                            aria-label={`Edit thresholds for ${getAlertResourceLabel(resource)}`}
                          >
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"
                              />
                            </svg>
                          </button>
                        </Show>
                        <Show when={isEditing()}>
                          <button
                            type="button"
                            onClick={cancelEditing}
                            class="p-1.5 bg-surface-hover text-slate-600 rounded"
                            aria-label="Cancel threshold edits"
                          >
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M6 18L18 6M6 6l12 12"
                              />
                            </svg>
                          </button>
                          <button
                            type="button"
                            onClick={() => saveEditing(resource.id)}
                            class="p-1.5 bg-green-50 dark:bg-green-900 text-green-600 rounded"
                            aria-label={`Save threshold edits for ${getAlertResourceLabel(resource)}`}
                          >
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M5 13l4 4L19 7"
                              />
                            </svg>
                          </button>
                        </Show>
                        <Show
                          when={
                            resource.hasOverride ||
                            (resource.type === 'agent' && resource.disableConnectivity)
                          }
                        >
                          <button
                            type="button"
                            onClick={() => props.table.onRemoveOverride(resource.id)}
                            class="p-1.5 bg-surface-alt hover:text-muted rounded transition-colors"
                            aria-label={`Revert to defaults for ${getAlertResourceLabel(resource)}`}
                            title={getAlertResourceTableRevertToDefaultsLabel()}
                          >
                            <RotateCcw class="w-4 h-4" />
                          </button>
                        </Show>
                      </div>
                    </div>

                    <Show when={isEditing()}>
                      <textarea
                        class="w-full text-xs p-2 rounded border border-border bg-surface-alt"
                        rows={2}
                        placeholder={getAlertResourceTableEditNotePlaceholder()}
                        value={props.table.editingNote()}
                        onInput={(e) => props.table.setEditingNote(e.currentTarget.value)}
                      />
                    </Show>

                    <div class="grid grid-cols-2 gap-2 text-sm border-t pt-2">
                      <For each={props.table.columns}>
                        {(column) => {
                          const metric = normalizeAlertResourceMetricKey(column);
                          if (!alertResourceSupportsMetric(resource.type, metric)) return null;

                          const isDisabled = () => thresholds()?.[metric] === -1;
                          const bounds = getAlertResourceMetricBounds(metric);

                          return (
                            <div class="flex justify-between items-center p-1.5 bg-surface-alt rounded">
                              <span class="text-[10px] uppercase font-bold tracking-wider">
                                {column.replace(/mb\/s|%|°c/gi, '').trim()}
                              </span>

                              <Show
                                when={isEditing()}
                                fallback={
                                  <button
                                    type="button"
                                    onClick={(e) => startEditing(resource, metric, e)}
                                    class="font-mono text-xs font-medium cursor-pointer rounded px-1 -mx-1 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
                                    aria-label={`Edit ${column} threshold for ${getAlertResourceLabel(resource)}`}
                                  >
                                    <MetricValueWithHeat
                                      resourceId={resource.id}
                                      metric={metric}
                                      value={displayValue(metric)}
                                      isOverridden={isOverridden(metric)}
                                    />
                                  </button>
                                }
                              >
                                <input
                                  type="number"
                                  min={bounds.min}
                                  max={bounds.max}
                                  value={thresholds()?.[metric] ?? ''}
                                  placeholder={getAlertResourceTableMetricPlaceholder(
                                    isDisabled(),
                                  )}
                                  class="w-16 text-right text-xs p-1 rounded border border-border bg-surface"
                                  onInput={(e) => {
                                    const nextValue = parseFloat(e.currentTarget.value);
                                    props.table.setEditingThresholds({
                                      ...props.table.editingThresholds(),
                                      [metric]: Number.isNaN(nextValue)
                                        ? undefined
                                        : nextValue,
                                    });
                                  }}
                                />
                              </Show>
                            </div>
                          );
                        }}
                      </For>
                    </div>
                  </Card>
                );
              }}
            </For>
          </div>
        )}
      </For>
      <Show when={props.hasRows() === false}>
        <div class="text-center p-8 text-slate-500 text-sm italic bg-surface-alt rounded-md">
          {getAlertResourceTableEmptyState(props.table.emptyMessage)}
        </div>
      </Show>
    </div>
  );
}
