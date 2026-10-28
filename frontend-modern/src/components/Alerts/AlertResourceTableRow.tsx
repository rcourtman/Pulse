import { For, Show } from 'solid-js';
import Pencil from 'lucide-solid/icons/pencil';
import RotateCcw from 'lucide-solid/icons/rotate-ccw';
import Timer from 'lucide-solid/icons/timer';
import X from 'lucide-solid/icons/x';

import { ActionIconButton } from '@/components/shared/Button';
import { FormTextarea } from '@/components/shared/FormTextarea';
import { TogglePrimitive } from '@/components/shared/Toggle';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { ThresholdSlider } from '@/components/Workloads/ThresholdSlider';
import { TableCell, TableRow } from '@/components/shared/Table';
import { ResourceNameWithWebInterfaceLink } from '@/components/shared/WebInterfaceLink';
import { getPlatformTableCellClassForKind } from '@/features/platformPage/sharedPlatformPage';
import {
  getAlertResourceTableCustomBadgeLabel,
  getAlertResourceTableEditMetricTitle,
  getAlertResourceTableOfflineStateOrder,
  getAlertResourceTableOfflineStatePresentation,
  getAlertResourceTableOverrideNotePlaceholder,
  getAlertResourceTableRevertToDefaultsLabel,
} from '@/utils/alertResourceTablePresentation';
import {
  ALERT_RESOURCE_TABLE_SLIDER_METRICS,
  alertResourceSupportsMetric,
  getAlertResourceColumnKind,
  getAlertResourceLabel,
  getAlertResourceInitialEditMetric,
  getAlertResourceMetricBounds,
  getAlertResourceMetricDisplayValue,
  getAlertResourceRelaxedTriggerSummary,
  isAlertResourceMetricOff,
  isAlertResourceMetricOverridden,
  normalizeAlertResourceMetricKey,
  type AlertResourceThresholdMap,
} from './alertResourceTableModel';
import type { Alert } from '@/types/api';
import type { AlertIntentSignal } from '@/api/alertIntentPolicies';
import type { Resource } from '@/features/alerts/thresholds/tableTypes';
import { AlertResourceMetricEditor } from './AlertResourceMetricEditor';
import { AlertResourceRelaxedTriggers } from './AlertResourceRelaxedTriggers';

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
  showAlertsColumn?: boolean;
  onToggleDisabled?: (resourceId: string) => void;
  onToggleNodeConnectivity?: (nodeId: string) => void;
  showOfflineAlertsColumn?: boolean;
  globalOfflineSeverity?: 'warning' | 'critical';
  onSetOfflineState?: (resourceId: string, state: OfflineState) => void;
  onToggleBackup?: (resourceId: string, forceState?: boolean) => void;
  onToggleSnapshot?: (resourceId: string, forceState?: boolean) => void;
  onConfigureResourceIntent?: (resourceId: string, signal: AlertIntentSignal) => void;
  globalDisableFlag?: () => boolean;
  globalDisableOfflineFlag?: () => boolean;
  editingNote: () => string;
  setEditingNote: (value: string) => void;
  activeMetricInput: () => { resourceId: string; metric: string } | null;
  isActiveLayout: () => boolean;
  setActiveMetricInput: (value: { resourceId: string; metric: string } | null) => void;
  showBulkSelection?: boolean;
  selected?: boolean;
  onToggleSelection?: (checked: boolean) => void;
}

export function AlertResourceTableRow(props: AlertResourceTableRowProps) {
  const isEditing = () => props.editingId() === props.resource.id;
  const thresholds = () =>
    isEditing() ? props.editingThresholds() : (props.resource.thresholds ?? {});
  const resourceLabel = () => getAlertResourceLabel(props.resource);
  const displayValue = (metric: string) =>
    getAlertResourceMetricDisplayValue(
      props.resource,
      metric,
      props.editingThresholds(),
      isEditing(),
    );
  const isOverridden = (metric: string) => isAlertResourceMetricOverridden(props.resource, metric);
  const relaxedTriggerSummary = () =>
    props.globalDisableFlag?.()
      ? null
      : getAlertResourceRelaxedTriggerSummary(
          props.resource,
          props.editingThresholds(),
          isEditing(),
        );
  const preferredIntentSignal = (): AlertIntentSignal => {
    for (const metric of ['cpu', 'memory', 'disk'] as const) {
      if (
        props.columns.some((column) => normalizeAlertResourceMetricKey(column) === metric) &&
        alertResourceSupportsMetric(props.resource.type, metric)
      ) {
        return `metric.${metric}`;
      }
    }
    return 'state.offline';
  };

  const getThresholds = (): AlertResourceThresholdMap => thresholds();

  let hasEditingFocus = false;

  const startEditing = (metric?: string, event?: MouseEvent) => {
    event?.stopPropagation();
    if (props.resource.editable === false) {
      return;
    }
    hasEditingFocus = false;
    const initialMetric =
      metric ?? getAlertResourceInitialEditMetric(props.columns, props.resource.type);
    if (initialMetric) {
      props.setActiveMetricInput({ resourceId: props.resource.id, metric: initialMetric });
    }
    props.onEdit(
      props.resource.id,
      props.resource.thresholds ? { ...props.resource.thresholds } : {},
      props.resource.defaults ? { ...props.resource.defaults } : {},
      typeof props.resource.note === 'string' ? props.resource.note : undefined,
    );
  };

  const cancelEditing = () => {
    hasEditingFocus = false;
    props.onCancelEdit();
    props.setActiveMetricInput(null);
  };

  const saveEditing = () => {
    hasEditingFocus = false;
    props.onSaveEdit(props.resource.id);
    props.setActiveMetricInput(null);
  };

  const updateEditingThreshold = (metric: string, value: number | undefined) => {
    props.setEditingThresholds({
      ...props.editingThresholds(),
      [metric]: value,
    });
  };

  // The value a metric cell shows, which its accessible name repeats so a
  // screen reader hears the current threshold and voice control can say it.
  const metricValueText = (metric: string, value: number) =>
    value <= 0 ? 'Off' : props.formatMetricValue(metric, value);

  const MetricValueWithHeat = (metricProps: {
    resourceId: string;
    metric: string;
    value: number;
    isOverridden: boolean;
  }) => {
    const isDisabledMetric = metricProps.value <= 0;
    const displayText = metricValueText(metricProps.metric, metricProps.value);

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
    ariaLabel?: string;
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
    // What the button sets; its visible state word ends the accessible name.
    name: string,
  ) => {
    const config = getAlertResourceTableOfflineStatePresentation(state);

    return (
      <button
        type="button"
        class={`inline-flex items-center justify-center px-2 py-0.5 text-xs font-medium rounded-sm transition-colors duration-150 focus:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-400 focus-visible:ring-offset-1 ${config.className} ${disabled ? 'opacity-60 cursor-not-allowed pointer-events-none' : ''}`.trim()}
        disabled={disabled}
        onClick={() => {
          if (disabled) return;
          onToggle();
        }}
        title={config.title}
        aria-label={`${name}: ${config.label}`}
      >
        {config.label}
      </button>
    );
  };

  return (
    <TableRow
      onFocusIn={() => {
        if (isEditing()) hasEditingFocus = true;
      }}
      onFocusOut={(event) => {
        // Removing this layout during a resize transfers the draft to cards.
        // It is not the user leaving the editor to save a row.
        if (!props.isActiveLayout()) return;
        if (!hasEditingFocus || props.editingId() !== props.resource.id) return;
        const nextFocus = event.relatedTarget;
        if (nextFocus instanceof Node && event.currentTarget.contains(nextFocus)) return;
        saveEditing();
      }}
      class={`transition-colors ${props.resource.disabled || props.globalDisableFlag?.() ? 'opacity-40' : ''} ${props.resource.hasOverride ? 'bg-sky-50/50 hover:bg-sky-50/80 dark:bg-sky-900/20 dark:hover:bg-sky-900/30 border-l-[3px] border-l-sky-400 dark:border-l-sky-500' : 'hover:bg-surface-hover'}`}
    >
      <Show when={props.showBulkSelection}>
        <TableCell
          class={`${getPlatformTableCellClassForKind('badge')} align-middle border-r border-border`}
        >
          <input
            type="checkbox"
            checked={props.selected}
            onChange={(e) => props.onToggleSelection?.(e.currentTarget.checked)}
            class="rounded-sm border-border text-sky-600 focus:ring-sky-500 transition-shadow cursor-pointer"
            aria-label={`Select ${resourceLabel()}`}
          />
        </TableCell>
      </Show>

      <Show when={props.showAlertsColumn !== false}>
        <TableCell class={`${getPlatformTableCellClassForKind('badge')} align-middle`}>
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
                    onToggle={() =>
                      !globallyDisabled && props.onToggleDisabled?.(props.resource.id)
                    }
                    class="-my-0.5"
                    title={
                      globallyDisabled
                        ? 'Alerts disabled globally'
                        : props.resource.disabled
                          ? 'Click to enable alerts'
                          : 'Click to disable alerts'
                    }
                    // The switch role carries on/off; the name says whose alerts.
                    ariaLabel={`Alerts for ${resourceLabel()}`}
                  />
                </div>
              );
            })()}
          </Show>
        </TableCell>
      </Show>

      <TableCell class={getPlatformTableCellClassForKind('name')}>
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
              <ResourceNameWithWebInterfaceLink
                name={resourceLabel()}
                url={props.resource.host as string | null | undefined}
                class="min-w-0"
                nameClass={`truncate text-sm font-medium ${
                  props.resource.disabled ? 'text-slate-500' : 'text-base-content'
                }`}
                title={`Open ${resourceLabel()} web interface`}
              />
              <Show when={props.resource.clusterName}>
                <span class="rounded-sm px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900/25 dark:text-blue-300">
                  {props.resource.clusterName}
                </span>
              </Show>
            </div>
          </Show>
          <Show when={props.resource.type === 'storage' && props.resource.node}>
            <span class="rounded-sm px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-amber-100 text-amber-700 dark:bg-amber-900/25 dark:text-amber-300">
              {props.resource.node}
            </span>
          </Show>
          <Show when={props.resource.subtitle}>
            <span class="text-xs text-muted">{props.resource.subtitle as string}</span>
          </Show>
          <Show when={props.resource.hasOverride || props.resource.disableConnectivity}>
            <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/25 text-blue-700 dark:text-blue-300 rounded-sm">
              {getAlertResourceTableCustomBadgeLabel()}
            </span>
          </Show>
          {/* The reading note joins the name on its line and ends in an
              ellipsis, with the whole note in its title, so the row stays
              one line tall instead of wrapping a paragraph under the name. */}
          <Show when={props.resource.readingMessage}>
            <span
              class="min-w-0 truncate text-xs text-muted"
              title={props.resource.readingMessage}
              data-alert-resource-reading-message
            >
              {props.resource.readingMessage}
            </span>
          </Show>
        </div>
        <Show when={isEditing()}>
          <FormTextarea
            id={`note-${props.resource.id}`}
            label="Override note"
            labelClass="sr-only"
            fieldBaseClass="mt-2 w-full"
            textareaBaseClass="w-full rounded-sm border border-border bg-surface px-2 py-1 text-xs text-base-content focus:border-blue-500 focus:outline-hidden focus:ring-1 focus:ring-blue-500"
            rows={2}
            placeholder={getAlertResourceTableOverrideNotePlaceholder()}
            value={props.editingNote()}
            onInput={(e) => props.setEditingNote(e.currentTarget.value)}
          />
        </Show>
        <Show when={relaxedTriggerSummary()}>
          {(summary) => (
            <AlertResourceRelaxedTriggers summary={summary()} class="mt-1 text-xs text-muted" />
          )}
        </Show>
        <Show when={!isEditing() && props.resource.note}>
          <p class="mt-2 text-xs italic text-muted wrap-break-word">
            {props.resource.note as string}
          </p>
        </Show>
      </TableCell>

      <For each={props.columns}>
        {(column) => {
          const metric = normalizeAlertResourceMetricKey(column);
          const bounds = getAlertResourceMetricBounds(metric);
          const isDisabled = () =>
            isAlertResourceMetricOff(getThresholds()?.[metric], props.resource.defaults?.[metric]);
          const isSpecialToggle = metric === 'backup' || metric === 'snapshot';

          if (isSpecialToggle) {
            const config = metric === 'backup' ? props.resource.backup : props.resource.snapshot;
            const isEnabled = config?.enabled ?? true;
            const onToggle = metric === 'backup' ? props.onToggleBackup : props.onToggleSnapshot;
            const titlePrefix = metric === 'backup' ? 'Backup' : 'Snapshot';

            return (
              <TableCell class={`${getPlatformTableCellClassForKind('badge')} align-middle`}>
                <Show
                  when={onToggle}
                  fallback={
                    <span class="text-sm text-slate-400" aria-hidden="true">
                      -
                    </span>
                  }
                >
                  <div class="flex items-center justify-center">
                    <StatusBadge
                      isEnabled={isEnabled}
                      onToggle={() => onToggle?.(props.resource.id)}
                      titleEnabled={`${titlePrefix} alerts enabled. Click to disable for this resource.`}
                      titleDisabled={`${titlePrefix} alerts disabled. Click to enable for this resource.`}
                      ariaLabel={`${titlePrefix} alerts for ${resourceLabel()}`}
                    />
                  </div>
                </Show>
              </TableCell>
            );
          }

          const openMetricEditor = (e?: MouseEvent) => {
            startEditing(metric, e);
          };

          return (
            <TableCell
              class={`${getPlatformTableCellClassForKind(getAlertResourceColumnKind(column))} align-middle`}
            >
              <Show
                when={alertResourceSupportsMetric(props.resource.type, metric)}
                fallback={
                  <span class="text-sm text-muted" aria-hidden="true">
                    -
                  </span>
                }
              >
                <Show
                  when={isEditing()}
                  fallback={
                    <div
                      role="button"
                      tabIndex={0}
                      onClick={openMetricEditor}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault();
                          openMetricEditor();
                        }
                      }}
                      class="cursor-pointer hover:bg-surface-hover rounded-sm px-1 py-0.5 transition-colors"
                      title={getAlertResourceTableEditMetricTitle()}
                      aria-label={`Edit ${column} threshold for ${resourceLabel()}: ${metricValueText(metric, displayValue(metric))}`}
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
                  <div class="flex w-full flex-col items-center justify-center gap-1.5">
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
                          if (typeof currentDisplayValue === 'number' && currentDisplayValue >= 0) {
                            return Math.round(currentDisplayValue);
                          }
                          return defaultSliderValue();
                        };

                        return (
                          <div class="w-36">
                            <ThresholdSlider
                              value={Math.max(sliderMin, Math.min(sliderMax, currentSliderValue()))}
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
                    <AlertResourceMetricEditor
                      metric={metric}
                      column={column}
                      scope="override"
                      layout="table"
                      value={getThresholds()?.[metric]}
                      inheritedDefault={props.resource.defaults?.[metric]}
                      resourceName={resourceLabel()}
                      onFocus={() =>
                        props.setActiveMetricInput({ resourceId: props.resource.id, metric })
                      }
                      onChange={(value) => updateEditingThreshold(metric, value)}
                      autoFocus={
                        isEditing() &&
                        props.activeMetricInput()?.resourceId === props.resource.id &&
                        props.activeMetricInput()?.metric === metric
                      }
                    />
                  </div>
                </Show>
              </Show>
            </TableCell>
          );
        }}
      </For>

      <Show when={props.showOfflineAlertsColumn}>
        <TableCell class={`${getPlatformTableCellClassForKind('badge')} align-middle`}>
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

              return renderOfflineStateButton(
                state,
                disabledGlobally,
                () => {
                  if (disabledGlobally) return;
                  const next = nextOfflineState(state);
                  props.onSetOfflineState?.(props.resource.id, next);
                },
                `Offline alerts for ${resourceLabel()}`,
              );
            }

            if (!props.onToggleNodeConnectivity) {
              return (
                <span class="text-sm text-slate-400" aria-hidden="true">
                  -
                </span>
              );
            }

            const globalOfflineDisabled = props.globalDisableOfflineFlag?.() ?? false;
            return renderToggleBadge({
              isEnabled: !globalOfflineDisabled && !props.resource.disableConnectivity,
              disabled: disabledGlobally,
              onToggle: () => {
                if (disabledGlobally) return;
                props.onToggleNodeConnectivity?.(props.resource.id);
              },
              titleEnabled: 'Offline alerts enabled. Click to disable for this resource.',
              titleDisabled: 'Offline alerts disabled. Click to enable for this resource.',
              titleWhenDisabled: 'Offline alerts controlled globally',
              ariaLabel: `Offline alerts for ${resourceLabel()}`,
            });
          })()}
        </TableCell>
      </Show>

      <TableCell class={getPlatformTableCellClassForKind('badge')}>
        <div class="flex items-center justify-center gap-1">
          <Show
            when={!isEditing()}
            fallback={
              <ActionIconButton
                onClick={cancelEditing}
                onMouseDown={(event) => event.preventDefault()}
                label="Cancel editing"
                tone="muted"
                size="xs"
              >
                <X class="w-4 h-4" aria-hidden="true" />
              </ActionIconButton>
            }
          >
            {(() => {
              const showEdit =
                props.resource.type !== 'dockerHost' && props.resource.editable !== false;
              const showRevert =
                props.resource.hasOverride ||
                ((props.resource.type === 'agent' || props.resource.type === 'dockerHost') &&
                  props.resource.disableConnectivity);
              const showIntent = typeof props.onConfigureResourceIntent === 'function';

              return (
                <Show
                  when={showEdit || showRevert || showIntent}
                  fallback={
                    <span class="text-xs text-muted" aria-hidden="true">
                      —
                    </span>
                  }
                >
                  <Show when={showEdit}>
                    <ActionIconButton
                      onClick={() => startEditing()}
                      label={`Edit thresholds for ${resourceLabel()}`}
                      title="Edit thresholds"
                      tone="accent"
                      size="xs"
                    >
                      <Pencil class="w-4 h-4" aria-hidden="true" />
                    </ActionIconButton>
                  </Show>
                  <Show when={showIntent}>
                    <ActionIconButton
                      onClick={() =>
                        props.onConfigureResourceIntent?.(
                          props.resource.id,
                          preferredIntentSignal(),
                        )
                      }
                      label={`Configure alert delay for ${resourceLabel()}`}
                      title="Configure individual alert delay"
                      tone="neutral"
                      size="xs"
                    >
                      <Timer class="w-4 h-4" aria-hidden="true" />
                    </ActionIconButton>
                  </Show>
                  <Show when={showRevert}>
                    <ActionIconButton
                      onClick={() => props.onRemoveOverride(props.resource.id)}
                      label={`Revert to defaults for ${resourceLabel()}`}
                      title={getAlertResourceTableRevertToDefaultsLabel()}
                      tone="neutral"
                      size="xs"
                    >
                      <RotateCcw class="w-4 h-4" aria-hidden="true" />
                    </ActionIconButton>
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
