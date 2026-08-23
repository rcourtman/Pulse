import { For, Show, createMemo } from 'solid-js';
import Check from 'lucide-solid/icons/check';
import Pencil from 'lucide-solid/icons/pencil';
import RotateCcw from 'lucide-solid/icons/rotate-ccw';
import Timer from 'lucide-solid/icons/timer';
import X from 'lucide-solid/icons/x';

import { ActionIconButton } from '@/components/shared/Button';
import { Card } from '@/components/shared/Card';
import { FormTextarea } from '@/components/shared/FormTextarea';
import { TogglePrimitive } from '@/components/shared/Toggle';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { AlertResourceGroupHeader } from './AlertResourceGroupHeader';
import { PlatformWindowedList } from '@/features/platformPage/PlatformWindowedList';
import {
  getAlertResourceTableCustomBadgeLabel,
  getAlertResourceTableEditNotePlaceholder,
  getAlertResourceTableEmptyState,
  getAlertResourceTableMetricOffToggleProps,
  getAlertResourceTableMetricPlaceholder,
  getAlertResourceTableOfflineStateOrder,
  getAlertResourceTableOfflineStatePresentation,
  getAlertResourceTableRevertToDefaultsLabel,
} from '@/utils/alertResourceTablePresentation';
import {
  ALERT_RESOURCE_METRIC_OFF_VALUE,
  alertResourceSupportsMetric,
  buildAlertResourceEditPayload,
  getAlertResourceEnabledDefault,
  getAlertResourceLabel,
  getAlertResourceMetricBounds,
  getAlertResourceMetricDisplayValue,
  getAlertResourceMetricStep,
  isAlertResourceMetricOff,
  isAlertResourceMetricOverridden,
  normalizeAlertResourceMetricKey,
  resolveAlertResourceMetricEnableValue,
} from './alertResourceTableModel';
import type { Resource } from '@/features/alerts/thresholds/tableTypes';
import type { OfflineState, ResourceTableProps } from './ResourceTable';

interface AlertResourceTableMobileProps {
  table: ResourceTableProps;
  hasRows: () => boolean;
  hasCustomGlobalDefaults: () => boolean;
  setActiveMetricInput: (value: { resourceId: string; metric: string } | null) => void;
}

type AlertResourceMobileItem =
  | { kind: 'group'; key: string; groupName: string }
  | { kind: 'resource'; key: string; resource: Resource };

export function AlertResourceTableMobile(props: AlertResourceTableMobileProps) {
  const mobileItems = createMemo<AlertResourceMobileItem[]>(() => {
    if (!props.table.groupedResources) {
      return (props.table.resources ?? []).map((resource) => ({
        kind: 'resource' as const,
        key: `resource:${resource.id}`,
        resource,
      }));
    }

    const items: AlertResourceMobileItem[] = [];
    for (const [groupName, resources] of Object.entries(props.table.groupedResources).sort(
      ([left], [right]) => left.localeCompare(right),
    )) {
      items.push({ kind: 'group', key: `group:${groupName}`, groupName });
      for (const resource of resources) {
        items.push({ kind: 'resource', key: `resource:${resource.id}`, resource });
      }
    }
    return items;
  });

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

  const nextOfflineState = (state: OfflineState): OfflineState => {
    const order = getAlertResourceTableOfflineStateOrder();
    const idx = order.indexOf(state);
    return order[(idx + 1) % order.length];
  };

  const renderOfflineStateButton = (
    state: OfflineState,
    disabled: boolean,
    onToggle: () => void,
  ) => {
    const config = getAlertResourceTableOfflineStatePresentation(state);

    return (
      <button
        type="button"
        class={`inline-flex min-h-11 items-center justify-center px-2 py-0.5 text-xs font-medium rounded transition-colors duration-150 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400 focus-visible:ring-offset-1 sm:min-h-0 ${config.className} ${disabled ? 'opacity-60 cursor-not-allowed pointer-events-none' : ''}`.trim()}
        disabled={disabled}
        onClick={() => {
          if (disabled) return;
          onToggle();
        }}
        title={config.title}
      >
        {config.label}
      </button>
    );
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
                // An unset default and a stored 0 are both disabled to the alert
                // engine, so neither may render as On.
                const isOff = () => isAlertResourceMetricOff(value());

                return (
                  <div class="p-2 bg-surface rounded border border-border-subtle flex flex-col gap-1">
                    <span class="text-[10px] uppercase text-slate-500 font-medium">{column}</span>
                    <div class="flex items-center gap-1.5">
                      <div class="relative flex-1">
                        <input
                          type="number"
                          min={bounds.min}
                          max={bounds.max}
                          step={getAlertResourceMetricStep(metric)}
                          value={isOff() ? '' : value()}
                          placeholder={getAlertResourceTableMetricPlaceholder(isOff())}
                          disabled={isOff()}
                          class={`min-h-11 w-full rounded border p-1 text-center text-sm sm:min-h-0 ${isOff() ? 'bg-surface-hover' : ' border-border'}`}
                          onInput={(e) => {
                            const nextValue = parseFloat(e.currentTarget.value);
                            // An empty box is mid-edit, not a disable request:
                            // coercing it to 0 used to disable the metric in the
                            // engine while the card still claimed On. Use the Off
                            // toggle to disable.
                            if (Number.isNaN(nextValue)) {
                              return;
                            }
                            if (nextValue < bounds.min || nextValue > bounds.max) {
                              return;
                            }
                            props.table.setGlobalDefaults?.((prev) => ({
                              ...prev,
                              [metric]: nextValue,
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
                      <StatusBadge
                        isEnabled={!isOff()}
                        onToggle={() => {
                          props.table.setGlobalDefaults?.((prev) => ({
                            ...prev,
                            [metric]: isOff()
                              ? getAlertResourceEnabledDefault(metric)
                              : ALERT_RESOURCE_METRIC_OFF_VALUE,
                          }));
                          props.table.setHasUnsavedChanges?.(true);
                        }}
                        {...getAlertResourceTableMetricOffToggleProps()}
                      />
                    </div>
                  </div>
                );
              }}
            </For>
            <Show when={props.table.showOfflineAlertsColumn}>
              <div class="p-2 bg-surface rounded border border-border-subtle flex flex-col gap-1">
                <span class="text-[10px] uppercase text-slate-500 font-medium">Offline alerts</span>
                <div class="flex items-center min-h-11 sm:min-h-0">
                  <Show
                    when={props.table.onSetGlobalOfflineState}
                    fallback={
                      <Show
                        when={props.table.onToggleGlobalDisableOffline}
                        fallback={
                          <span class="text-sm text-slate-400" aria-hidden="true">
                            -
                          </span>
                        }
                      >
                        <StatusBadge
                          isEnabled={!(props.table.globalDisableOfflineFlag?.() ?? false)}
                          onToggle={() => {
                            props.table.onToggleGlobalDisableOffline?.();
                            props.table.setHasUnsavedChanges?.(true);
                          }}
                          labelEnabled="On"
                          labelDisabled="Off"
                          titleEnabled="Offline alerts currently enabled by default. Click to disable."
                          titleDisabled="Offline alerts currently disabled by default. Click to enable."
                        />
                      </Show>
                    }
                  >
                    {(() => {
                      const disabledGlobally = props.table.globalDisableFlag?.() ?? false;
                      const defaultDisabled = props.table.globalDisableOfflineFlag?.() ?? false;
                      const defaultSeverity = props.table.globalOfflineSeverity ?? 'warning';
                      const state: OfflineState = defaultDisabled
                        ? 'off'
                        : defaultSeverity === 'critical'
                          ? 'critical'
                          : 'warning';

                      return renderOfflineStateButton(state, disabledGlobally, () => {
                        if (disabledGlobally) return;
                        props.table.onSetGlobalOfflineState?.(nextOfflineState(state));
                      });
                    })()}
                  </Show>
                </div>
              </div>
            </Show>
          </div>
        </Card>
      </Show>

      <PlatformWindowedList
        items={mobileItems}
        estimatedItemHeight={240}
        enableThreshold={18}
        windowSize={24}
      >
        {(item) => {
          if (item.kind === 'group') {
            return (
              <div class="px-1 pt-4 pb-1 font-medium text-xs text-slate-500 uppercase">
                <AlertResourceGroupHeader
                  groupKey={item.groupName}
                  meta={props.table.groupHeaderMeta?.[item.groupName]}
                />
              </div>
            );
          }

          const resource = item.resource;
          const isEditing = () => props.table.editingId() === resource.id;
          const thresholds = () => getThresholds(resource, isEditing());
          const displayValue = (metric: string) => getDisplayValue(resource, metric, isEditing());
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
                    <ActionIconButton
                      onClick={() => startEditing(resource)}
                      label={`Edit thresholds for ${getAlertResourceLabel(resource)}`}
                      tone="accent"
                      size="sm"
                      class="min-h-11 min-w-11"
                    >
                      <Pencil class="w-4 h-4" aria-hidden="true" />
                    </ActionIconButton>
                  </Show>
                  <Show when={!isEditing() && props.table.onConfigureResourceIntent}>
                    <ActionIconButton
                      onClick={() => {
                        const preferredMetric = ['cpu', 'memory', 'disk'].find(
                          (metric) =>
                            props.table.columns.some(
                              (column) => normalizeAlertResourceMetricKey(column) === metric,
                            ) && alertResourceSupportsMetric(resource.type, metric),
                        );
                        props.table.onConfigureResourceIntent?.(
                          resource.id,
                          preferredMetric ? `metric.${preferredMetric}` : 'state.offline',
                        );
                      }}
                      label={`Configure alert delay for ${getAlertResourceLabel(resource)}`}
                      title="Configure individual alert delay"
                      tone="neutral"
                      size="sm"
                      class="min-h-11 min-w-11"
                    >
                      <Timer class="w-4 h-4" aria-hidden="true" />
                    </ActionIconButton>
                  </Show>
                  <Show when={isEditing()}>
                    <ActionIconButton
                      onClick={cancelEditing}
                      label="Cancel threshold edits"
                      tone="muted"
                      size="sm"
                      class="min-h-11 min-w-11"
                    >
                      <X class="w-4 h-4" aria-hidden="true" />
                    </ActionIconButton>
                    <ActionIconButton
                      onClick={() => saveEditing(resource.id)}
                      label={`Save threshold edits for ${getAlertResourceLabel(resource)}`}
                      tone="success"
                      size="sm"
                      class="min-h-11 min-w-11"
                    >
                      <Check class="w-4 h-4" aria-hidden="true" />
                    </ActionIconButton>
                  </Show>
                  <Show
                    when={
                      resource.hasOverride ||
                      (resource.type === 'agent' && resource.disableConnectivity)
                    }
                  >
                    <ActionIconButton
                      onClick={() => props.table.onRemoveOverride(resource.id)}
                      label={`Revert to defaults for ${getAlertResourceLabel(resource)}`}
                      title={getAlertResourceTableRevertToDefaultsLabel()}
                      tone="neutral"
                      size="sm"
                      class="min-h-11 min-w-11"
                    >
                      <RotateCcw class="w-4 h-4" aria-hidden="true" />
                    </ActionIconButton>
                  </Show>
                </div>
              </div>

              <Show when={isEditing()}>
                <FormTextarea
                  label="Override note"
                  labelClass="sr-only"
                  fieldBaseClass="w-full"
                  textareaBaseClass="w-full text-xs p-2 rounded border border-border bg-surface-alt"
                  rows={2}
                  placeholder={getAlertResourceTableEditNotePlaceholder()}
                  value={props.table.editingNote()}
                  onInput={(e) => props.table.setEditingNote(e.currentTarget.value)}
                />
              </Show>

              <div
                class={`grid gap-2 text-sm border-t pt-2 ${isEditing() ? 'grid-cols-1' : 'grid-cols-2'}`}
              >
                <For each={props.table.columns}>
                  {(column) => {
                    const metric = normalizeAlertResourceMetricKey(column);
                    if (!alertResourceSupportsMetric(resource.type, metric)) return null;

                    // Backup and snapshot carry day-based configs, not
                    // trigger/clear thresholds. Routing them through the
                    // numeric editor persisted {trigger, clear} into the
                    // backup block, which the backend reads as an all-zero
                    // disabled config (#1126). Render the same on/off
                    // toggle the desktop rows use instead.
                    if (metric === 'backup' || metric === 'snapshot') {
                      const config = metric === 'backup' ? resource.backup : resource.snapshot;
                      const onToggle =
                        metric === 'backup'
                          ? props.table.onToggleBackup
                          : props.table.onToggleSnapshot;
                      if (!onToggle) return null;
                      const titlePrefix = metric === 'backup' ? 'Backup' : 'Snapshot';

                      return (
                        <div class="flex justify-between items-center p-1.5 bg-surface-alt rounded">
                          <span class="text-[10px] uppercase font-bold tracking-wider">
                            {column}
                          </span>
                          <StatusBadge
                            isEnabled={config?.enabled ?? true}
                            onToggle={() => onToggle(resource.id)}
                            titleEnabled={`${titlePrefix} alerts enabled. Click to disable for this resource.`}
                            titleDisabled={`${titlePrefix} alerts disabled. Click to enable for this resource.`}
                          />
                        </div>
                      );
                    }

                    const isDisabled = () => isAlertResourceMetricOff(thresholds()?.[metric]);
                    const inheritedDefault = () => resource.defaults?.[metric];
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
                              class="min-h-11 min-w-11 font-mono text-xs font-medium cursor-pointer rounded px-1 -mx-1 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
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
                          <div class="flex items-center gap-1.5 [&_button]:min-h-11">
                            <input
                              type="number"
                              min={bounds.min}
                              max={bounds.max}
                              value={isDisabled() ? '' : (thresholds()?.[metric] ?? '')}
                              placeholder={getAlertResourceTableMetricPlaceholder(isDisabled())}
                              class="min-h-11 w-16 text-right text-xs p-1 rounded border border-border bg-surface"
                              onInput={(e) => {
                                const nextValue = parseFloat(e.currentTarget.value);
                                if (
                                  !Number.isNaN(nextValue) &&
                                  (nextValue < bounds.min || nextValue > bounds.max)
                                ) {
                                  return;
                                }
                                props.table.setEditingThresholds({
                                  ...props.table.editingThresholds(),
                                  [metric]: Number.isNaN(nextValue) ? undefined : nextValue,
                                });
                              }}
                            />
                            <StatusBadge
                              isEnabled={!isDisabled()}
                              onToggle={() =>
                                props.table.setEditingThresholds({
                                  ...props.table.editingThresholds(),
                                  [metric]: isDisabled()
                                    ? resolveAlertResourceMetricEnableValue(
                                        inheritedDefault(),
                                        metric,
                                      )
                                    : ALERT_RESOURCE_METRIC_OFF_VALUE,
                                })
                              }
                              {...getAlertResourceTableMetricOffToggleProps()}
                            />
                          </div>
                        </Show>
                      </div>
                    );
                  }}
                </For>
                <Show when={props.table.showOfflineAlertsColumn}>
                  {(() => {
                    const supportsTriState =
                      typeof props.table.onSetOfflineState === 'function' &&
                      (resource.type === 'guest' || resource.type === 'dockerContainer');
                    if (!supportsTriState && !props.table.onToggleNodeConnectivity) {
                      return null;
                    }
                    const disabledGlobally = () => props.table.globalDisableFlag?.() ?? false;

                    return (
                      <div class="flex justify-between items-center p-1.5 bg-surface-alt rounded">
                        <span class="text-[10px] uppercase font-bold tracking-wider">Offline</span>
                        <Show
                          when={supportsTriState}
                          fallback={
                            <StatusBadge
                              isEnabled={
                                !(props.table.globalDisableOfflineFlag?.() ?? false) &&
                                !resource.disableConnectivity
                              }
                              disabled={disabledGlobally()}
                              onToggle={() => {
                                if (disabledGlobally()) return;
                                props.table.onToggleNodeConnectivity?.(resource.id);
                              }}
                              titleEnabled="Offline alerts enabled. Click to disable for this resource."
                              titleDisabled="Offline alerts disabled. Click to enable for this resource."
                              titleWhenDisabled="Offline alerts controlled globally"
                            />
                          }
                        >
                          {(() => {
                            const defaultDisabled =
                              props.table.globalDisableOfflineFlag?.() ?? false;
                            const defaultSeverity = props.table.globalOfflineSeverity ?? 'warning';

                            let state: OfflineState;
                            if (resource.disableConnectivity) {
                              state = 'off';
                            } else if (resource.poweredOffSeverity) {
                              state = resource.poweredOffSeverity;
                            } else if (defaultDisabled) {
                              state = 'off';
                            } else {
                              state = defaultSeverity === 'critical' ? 'critical' : 'warning';
                            }

                            return renderOfflineStateButton(state, disabledGlobally(), () => {
                              if (disabledGlobally()) return;
                              props.table.onSetOfflineState?.(resource.id, nextOfflineState(state));
                            });
                          })()}
                        </Show>
                      </div>
                    );
                  })()}
                </Show>
              </div>
            </Card>
          );
        }}
      </PlatformWindowedList>
      <Show when={props.hasRows() === false}>
        <div class="text-center p-8 text-slate-500 text-sm italic bg-surface-alt rounded-md">
          {getAlertResourceTableEmptyState(props.table.emptyMessage)}
        </div>
      </Show>
    </div>
  );
}
