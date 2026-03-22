import { For, Show } from 'solid-js';
import RotateCcw from 'lucide-solid/icons/rotate-ccw';

import { TogglePrimitive } from '@/components/shared/Toggle';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { ThresholdSlider } from '@/components/Dashboard/ThresholdSlider';
import { TableCell, TableRow } from '@/components/shared/Table';
import {
  getAlertResourceTableCustomBadgeLabel,
  getAlertResourceTableEditMetricTitle,
  getAlertResourceTableMetricInputTitle,
  getAlertResourceTableMetricPlaceholder,
  getAlertResourceTableOfflineStateOrder,
  getAlertResourceTableOfflineStatePresentation,
  getAlertResourceTableOverrideNotePlaceholder,
  getAlertResourceTableRevertToDefaultsLabel,
} from '@/utils/alertResourceTablePresentation';
import {
  ALERT_RESOURCE_TABLE_SLIDER_METRICS,
  alertResourceSupportsMetric,
  getAlertResourceLabel,
  getAlertResourceMetricBounds,
  getAlertResourceMetricDisplayValue,
  getAlertResourceMetricStep,
  isAlertResourceMetricOverridden,
  normalizeAlertResourceMetricKey,
  type AlertResourceThresholdMap,
} from './alertResourceTableModel';
import type { Alert } from '@/types/api';
import type { Resource } from '@/features/alerts/thresholds/tableTypes';

type OfflineState = 'off' | 'warning' | 'critical';

interface AlertResourceTableRowProps {
  resource: Resource;
  columns: string[];
  activeAlerts?: Record<string, Alert>;
  editingId: () => string | null;
  editingThresholds: () => Record<string, number | undefined>;
  setEditingThresholds: (value: Record<string, number | undefined>) => void;
  formatMetricValue: (metric: string, value: number | undefined) => string;
  hasActiveAlert: (resourceId: string, metric: string) => boolean;
  onEdit: (
    resourceId: string,
    thresholds: Record<string, number | undefined>,
    defaults: Record<string, number | undefined>,
    note: string | undefined,
  ) => void;
  onSaveEdit: (resourceId: string) => void;
  onCancelEdit: () => void;
  onRemoveOverride: (resourceId: string) => void;
  onToggleDisabled?: (resourceId: string) => void;
  onToggleNodeConnectivity?: (nodeId: string) => void;
  showOfflineAlertsColumn?: boolean;
  globalOfflineSeverity?: 'warning' | 'critical';
  onSetOfflineState?: (resourceId: string, state: OfflineState) => void;
  onToggleBackup?: (resourceId: string, forceState?: boolean) => void;
  onToggleSnapshot?: (resourceId: string, forceState?: boolean) => void;
  globalDisableFlag?: () => boolean;
  globalDisableOfflineFlag?: () => boolean;
  editingNote: () => string;
  setEditingNote: (value: string) => void;
  activeMetricInput: () => { resourceId: string; metric: string } | null;
  setActiveMetricInput: (value: { resourceId: string; metric: string } | null) => void;
  showBulkSelection?: boolean;
  selected?: boolean;
  onToggleSelection?: (checked: boolean) => void;
}

export function AlertResourceTableRow(props: AlertResourceTableRowProps) {
  const isEditing = () => props.editingId() === props.resource.id;
  const thresholds = () => (isEditing() ? props.editingThresholds() : (props.resource.thresholds ?? {}));
  const resourceLabel = () => getAlertResourceLabel(props.resource);
  const displayValue = (metric: string) =>
    getAlertResourceMetricDisplayValue(
      props.resource,
      metric,
      props.editingThresholds(),
      isEditing(),
    );
  const isOverridden = (metric: string) =>
    isAlertResourceMetricOverridden(props.resource, metric);

  const getThresholds = (): AlertResourceThresholdMap => thresholds();

  const startEditing = (metric?: string, event?: MouseEvent) => {
    event?.stopPropagation();
    if (props.resource.editable === false) {
      return;
    }
    if (metric) {
      props.setActiveMetricInput({ resourceId: props.resource.id, metric });
    }
    props.onEdit(
      props.resource.id,
      props.resource.thresholds ? { ...props.resource.thresholds } : {},
      props.resource.defaults ? { ...props.resource.defaults } : {},
      typeof props.resource.note === 'string' ? props.resource.note : undefined,
    );
  };

  const cancelEditing = () => {
    props.onCancelEdit();
    props.setActiveMetricInput(null);
  };

  const saveEditing = () => {
    props.onSaveEdit(props.resource.id);
    props.setActiveMetricInput(null);
  };

  const updateEditingThreshold = (metric: string, value: number | undefined) => {
    props.setEditingThresholds({
      ...props.editingThresholds(),
      [metric]: value,
    });
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
      : props.formatMetricValue(metricProps.metric, metricProps.value);

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
        <Show when={props.hasActiveAlert(metricProps.resourceId, metricProps.metric)}>
          <div class="w-1.5 h-1.5 rounded-full bg-red-500 animate-pulse" title="Active alert" />
        </Show>
      </div>
    );
  };

  const renderToggleBadge = (config: {
    isEnabled: boolean;
    disabled?: boolean;
    size?: 'sm' | 'md';
    onToggle?: () => void;
    labelEnabled?: string;
    labelDisabled?: string;
    titleEnabled?: string;
    titleDisabled?: string;
    titleWhenDisabled?: string;
  }) => <StatusBadge {...config} />;

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
        class={`inline-flex items-center justify-center px-2 py-0.5 text-xs font-medium rounded transition-colors duration-150 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400 focus-visible:ring-offset-1 ${config.className} ${disabled ? 'opacity-60 cursor-not-allowed pointer-events-none' : ''}`.trim()}
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

  return (
    <TableRow
      class={`transition-colors ${props.resource.disabled || props.globalDisableFlag?.() ? 'opacity-40' : ''} ${props.resource.hasOverride ? 'bg-sky-50/50 hover:bg-sky-50/80 dark:bg-sky-900/20 dark:hover:bg-sky-900/30 border-l-[3px] border-l-sky-400 dark:border-l-sky-500' : 'hover:bg-surface-hover'}`}
    >
      <Show when={props.showBulkSelection}>
        <TableCell class="p-1 px-2 text-center align-middle border-r border-border">
          <input
            type="checkbox"
            checked={props.selected}
            onChange={(e) => props.onToggleSelection?.(e.currentTarget.checked)}
            class="rounded border-border text-sky-600 focus:ring-sky-500 transition-shadow cursor-pointer"
            aria-label={`Select ${resourceLabel()}`}
          />
        </TableCell>
      </Show>

      <TableCell class="p-1 px-2 text-center align-middle">
        <Show when={props.onToggleDisabled}>
          {(() => {
            const globallyDisabled = props.globalDisableFlag?.() ?? false;
            const isChecked = !globallyDisabled && !props.resource.disabled;
            return (
              <div class="flex items-center justify-center">
                <TogglePrimitive
                  size="sm"
                  checked={isChecked}
                  disabled={globallyDisabled}
                  onToggle={() => !globallyDisabled && props.onToggleDisabled?.(props.resource.id)}
                  class="my-[1px]"
                  title={
                    globallyDisabled
                      ? 'Alerts disabled globally'
                      : props.resource.disabled
                        ? 'Click to enable alerts'
                        : 'Click to disable alerts'
                  }
                  ariaLabel={
                    isChecked
                      ? 'Alerts enabled for this resource'
                      : 'Alerts disabled for this resource'
                  }
                />
              </div>
            );
          })()}
        </Show>
      </TableCell>

      <TableCell class="p-1 px-2">
        <div class="flex items-center gap-2 min-w-0">
          <Show
            when={props.resource.type === 'agent'}
            fallback={
              <span
                class={`text-sm font-medium truncate flex-nowrap ${props.resource.disabled ? 'text-slate-500 ' : 'text-base-content'}`}
              >
                {resourceLabel()}
              </span>
            }
          >
            <div class="flex items-center gap-3 min-w-0" title={props.resource.status || undefined}>
              <Show
                when={props.resource.host}
                fallback={
                  <span
                    class={`text-sm font-medium truncate flex-nowrap ${props.resource.disabled ? 'text-slate-500 ' : 'text-base-content'}`}
                  >
                    {resourceLabel()}
                  </span>
                }
              >
                {(host) => (
                  <a
                    href={host() as string}
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={(e) => e.stopPropagation()}
                    class={`text-sm font-medium truncate flex-nowrap transition-colors duration-150 ${
                      props.resource.disabled
                        ? 'text-slate-500 '
                        : 'text-base-content hover:text-sky-600 dark:hover:text-sky-400'
                    }`}
                    title={`Open ${resourceLabel()} web interface`}
                  >
                    {resourceLabel()}
                  </a>
                )}
              </Show>
              <Show when={props.resource.clusterName}>
                <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">
                  {props.resource.clusterName}
                </span>
              </Show>
            </div>
          </Show>
          <Show when={props.resource.type === 'storage' && props.resource.node}>
            <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300">
              {props.resource.node}
            </span>
          </Show>
          <Show when={props.resource.subtitle}>
            <span class="text-xs text-muted">{props.resource.subtitle as string}</span>
          </Show>
          <Show when={props.resource.hasOverride || props.resource.disableConnectivity}>
            <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
              {getAlertResourceTableCustomBadgeLabel()}
            </span>
          </Show>
        </div>
        <Show when={isEditing()}>
          <div class="mt-2 w-full">
            <label class="sr-only" for={`note-${props.resource.id}`}>
              Override note
            </label>
            <textarea
              id={`note-${props.resource.id}`}
              class="w-full rounded border border-border bg-surface px-2 py-1 text-xs text-base-content focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              rows={2}
              placeholder={getAlertResourceTableOverrideNotePlaceholder()}
              value={props.editingNote()}
              onInput={(e) => props.setEditingNote(e.currentTarget.value)}
            />
          </div>
        </Show>
        <Show when={!isEditing() && props.resource.note}>
          <p class="mt-2 text-xs italic text-muted break-words">{props.resource.note as string}</p>
        </Show>
      </TableCell>

      <For each={props.columns}>
        {(column) => {
          const metric = normalizeAlertResourceMetricKey(column);
          const bounds = getAlertResourceMetricBounds(metric);
          const isDisabled = () => getThresholds()?.[metric] === -1;
          const isSpecialToggle = metric === 'backup' || metric === 'snapshot';

          if (isSpecialToggle) {
            const config = metric === 'backup' ? props.resource.backup : props.resource.snapshot;
            const isEnabled = config?.enabled ?? true;
            const onToggle = metric === 'backup' ? props.onToggleBackup : props.onToggleSnapshot;
            const titlePrefix = metric === 'backup' ? 'Backup' : 'Snapshot';

            return (
              <TableCell class="p-1 px-2 text-center align-middle">
                <Show
                  when={onToggle}
                  fallback={<span class="text-sm text-slate-400">-</span>}
                >
                  <div class="flex items-center justify-center">
                    <StatusBadge
                      isEnabled={isEnabled}
                      onToggle={() => onToggle?.(props.resource.id)}
                      titleEnabled={`${titlePrefix} alerts enabled. Click to disable for this resource.`}
                      titleDisabled={`${titlePrefix} alerts disabled. Click to enable for this resource.`}
                    />
                  </div>
                </Show>
              </TableCell>
            );
          }

          const openMetricEditor = (e: MouseEvent) => {
            startEditing(metric, e);
          };

          return (
            <TableCell class="p-1 px-2 text-center align-middle">
              <Show
                when={alertResourceSupportsMetric(props.resource.type, metric)}
                fallback={<span class="text-sm text-muted">-</span>}
              >
                <Show
                  when={isEditing()}
                  fallback={
                    <div
                      onClick={openMetricEditor}
                      class="cursor-pointer hover:bg-surface-hover rounded px-1 py-0.5 transition-colors"
                      title={getAlertResourceTableEditMetricTitle()}
                    >
                      <MetricValueWithHeat
                        resourceId={props.resource.id}
                        metric={metric}
                        value={displayValue(metric)}
                        isOverridden={isOverridden(metric)}
                      />
                    </div>
                  }
                >
                  <div class="flex w-full items-center justify-center gap-3">
                    <Show when={ALERT_RESOURCE_TABLE_SLIDER_METRICS.has(metric)}>
                      {(() => {
                        const isTemperatureMetric =
                          metric === 'temperature' || metric === 'diskTemperature';
                        const sliderMin = Math.max(0, bounds.min);
                        const sliderMax = isTemperatureMetric
                          ? Math.max(sliderMin, bounds.max > 0 ? bounds.max : 120)
                          : bounds.max;
                        const defaultSliderValue = () => {
                          if (metric === 'disk') return 90;
                          if (metric === 'memory') return 85;
                          if (metric === 'temperature') return 80;
                          if (metric === 'diskTemperature') return 55;
                          return 80;
                        };
                        const currentSliderValue = () => {
                          const editingVal = props.editingThresholds()?.[metric];
                          if (typeof editingVal === 'number' && editingVal >= 0) {
                            return Math.round(editingVal);
                          }
                          const currentDisplayValue = displayValue(metric);
                          if (
                            typeof currentDisplayValue === 'number' &&
                            currentDisplayValue >= 0
                          ) {
                            return Math.round(currentDisplayValue);
                          }
                          return defaultSliderValue();
                        };

                        return (
                          <div class="w-36">
                            <ThresholdSlider
                              value={Math.max(
                                sliderMin,
                                Math.min(sliderMax, currentSliderValue()),
                              )}
                              onChange={(val) => {
                                props.setEditingThresholds({
                                  ...props.editingThresholds(),
                                  [metric]: val,
                                });
                              }}
                              type={
                                isTemperatureMetric
                                  ? 'temperature'
                                  : (metric as 'cpu' | 'memory' | 'disk')
                              }
                              min={sliderMin}
                              max={sliderMax}
                              disabled={isDisabled()}
                            />
                          </div>
                        );
                      })()}
                    </Show>
                    <div class="flex items-center justify-center">
                      <input
                        type="number"
                        min={bounds.min}
                        max={bounds.max}
                        step={getAlertResourceMetricStep(metric)}
                        value={getThresholds()?.[metric] ?? ''}
                        placeholder={getAlertResourceTableMetricPlaceholder(isDisabled())}
                        title={getAlertResourceTableMetricInputTitle(isDisabled())}
                        ref={(el) => {
                          if (
                            isEditing() &&
                            props.activeMetricInput()?.resourceId === props.resource.id &&
                            props.activeMetricInput()?.metric === metric
                          ) {
                            queueMicrotask(() => {
                              el.focus();
                              el.select();
                            });
                          }
                        }}
                        onInput={(e) => {
                          const raw = e.currentTarget.value;
                          if (raw === '') {
                            updateEditingThreshold(metric, undefined);
                            return;
                          }
                          const val = parseFloat(raw);
                          if (!Number.isNaN(val)) {
                            updateEditingThreshold(metric, val);
                          }
                        }}
                        onBlur={() => {
                          if (props.editingId() === props.resource.id) {
                            saveEditing();
                          }
                        }}
                        class={`w-16 px-2 py-0.5 text-sm text-center border rounded ${
                          isDisabled()
                            ? 'bg-surface-alt text-slate-400 border-border'
                            : ' text-base-content border-border'
                        }`}
                      />
                    </div>
                  </div>
                </Show>
              </Show>
            </TableCell>
          );
        }}
      </For>

      <Show when={props.showOfflineAlertsColumn}>
        <TableCell class="p-1 px-2 text-center align-middle">
          {(() => {
            const disabledGlobally = props.globalDisableFlag?.() ?? false;
            const supportsTriState =
              typeof props.onSetOfflineState === 'function' &&
              (props.resource.type === 'guest' || props.resource.type === 'dockerContainer');

            if (supportsTriState) {
              const defaultDisabled = props.globalDisableOfflineFlag?.() ?? false;
              const defaultSeverity = props.globalOfflineSeverity ?? 'warning';

              let state: OfflineState;
              if (props.resource.disableConnectivity) {
                state = 'off';
              } else if (props.resource.poweredOffSeverity) {
                state = props.resource.poweredOffSeverity;
              } else if (defaultDisabled) {
                state = 'off';
              } else {
                state = defaultSeverity === 'critical' ? 'critical' : 'warning';
              }

              return renderOfflineStateButton(state, disabledGlobally, () => {
                if (disabledGlobally) return;
                const next = nextOfflineState(state);
                props.onSetOfflineState?.(props.resource.id, next);
              });
            }

            if (!props.onToggleNodeConnectivity) {
              return <span class="text-sm text-slate-400">-</span>;
            }

            const globalOfflineDisabled = props.globalDisableOfflineFlag?.() ?? false;
            return renderToggleBadge({
              isEnabled: !globalOfflineDisabled && !props.resource.disableConnectivity,
              disabled: disabledGlobally,
              onToggle: () => {
                if (disabledGlobally) return;
                props.onToggleNodeConnectivity?.(props.resource.id);
              },
              titleEnabled:
                'Offline alerts enabled. Click to disable for this resource.',
              titleDisabled:
                'Offline alerts disabled. Click to enable for this resource.',
              titleWhenDisabled: 'Offline alerts controlled globally',
            });
          })()}
        </TableCell>
      </Show>

      <TableCell class="p-1 px-2">
        <div class="flex items-center justify-center gap-1">
          <Show
            when={!isEditing()}
            fallback={
              <button
                type="button"
                onClick={cancelEditing}
                class="p-1 hover:text-muted"
                title="Cancel editing"
                aria-label="Cancel editing"
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
            }
          >
            {(() => {
              const showEdit =
                props.resource.type !== 'dockerHost' && props.resource.editable !== false;
              const showRevert =
                props.resource.hasOverride ||
                ((props.resource.type === 'agent' || props.resource.type === 'dockerHost') &&
                  props.resource.disableConnectivity);

              return (
                <Show when={showEdit || showRevert} fallback={<span class="text-xs text-muted">—</span>}>
                  <Show when={showEdit}>
                    <button
                      type="button"
                      onClick={() => startEditing()}
                      class="p-1 text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                      title="Edit thresholds"
                      aria-label={`Edit thresholds for ${resourceLabel()}`}
                    >
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                        />
                      </svg>
                    </button>
                  </Show>
                  <Show when={showRevert}>
                    <button
                      type="button"
                      onClick={() => props.onRemoveOverride(props.resource.id)}
                      class="p-1 hover:text-base-content transition-colors"
                      title={getAlertResourceTableRevertToDefaultsLabel()}
                      aria-label={`Revert to defaults for ${resourceLabel()}`}
                    >
                      <RotateCcw class="w-4 h-4" />
                    </button>
                  </Show>
                </Show>
              );
            })()}
          </Show>
        </div>
      </TableCell>
    </TableRow>
  );
}
