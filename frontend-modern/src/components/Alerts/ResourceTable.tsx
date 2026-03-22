import { For, Show, createEffect } from 'solid-js';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { TogglePrimitive } from '@/components/shared/Toggle';
import { StatusBadge } from '@/components/shared/StatusBadge';
import type { Alert } from '@/types/api';
import { Card } from '@/components/shared/Card';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/shared/Table';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { ThresholdSlider } from '@/components/Dashboard/ThresholdSlider';
import { HelpIcon } from '@/components/shared/HelpIcon';
import RotateCcw from 'lucide-solid/icons/rotate-ccw';
import {
  getAlertResourceTableAlertDelayLabel,
  getAlertResourceTableCustomBadgeLabel,
  getAlertResourceTableEditMetricTitle,
  getAlertResourceTableEditNotePlaceholder,
  getAlertResourceTableEmptyState,
  getAlertResourceTableMetricInputTitle,
  getAlertResourceTableMetricPlaceholder,
  getAlertResourceTableNoResultsState,
  getAlertResourceTableOfflineStateOrder,
  getAlertResourceTableOfflineStatePresentation,
  getAlertResourceTableOverrideNotePlaceholder,
  getAlertResourceTableResetFactoryDefaultsLabel,
  getAlertResourceTableRevertToDefaultsLabel,
} from '@/utils/alertResourceTablePresentation';
import {
  ALERT_BULK_EDIT_CLEAR_LABEL,
  getAlertBulkEditOpenLabel,
} from '@/utils/alertBulkEditPresentation';
import {
  ALERT_RESOURCE_TABLE_SLIDER_METRICS,
  alertResourceSupportsMetric,
  buildAlertResourceEditPayload,
  getAlertResourceColumnHeaderTooltip,
  getAlertResourceEnabledDefault,
  getAlertResourceLabel,
  getAlertResourceMetricBounds,
  getAlertResourceMetricDelayOverride,
  getAlertResourceMetricDisplayValue,
  getAlertResourceMetricStep,
  isAlertResourceMetricOverridden,
  normalizeAlertResourceMetricKey,
  type AlertResourceThresholdMap,
} from './alertResourceTableModel';
import { useAlertResourceTableState } from './useAlertResourceTableState';
import type { GroupHeaderMeta, Resource } from '@/features/alerts/thresholds/tableTypes';

export type { GroupHeaderMeta, Resource } from '@/features/alerts/thresholds/tableTypes';

const OFFLINE_ALERTS_TOOLTIP =
  'Toggle default behavior for powered-off or connectivity alerts for this resource type.';

interface ResourceTableProps {
  title: string;
  resources?: Resource[];
  groupedResources?: Record<string, Resource[]>;
  columns: string[];
  activeAlerts?: Record<string, Alert>;
  emptyMessage?: string;
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
  showOfflineAlertsColumn?: boolean; // Show separate column for offline/connectivity alerts
  globalOfflineSeverity?: 'warning' | 'critical';
  onSetGlobalOfflineState?: (state: OfflineState) => void;
  onSetOfflineState?: (resourceId: string, state: OfflineState) => void;
  onToggleBackup?: (resourceId: string, forceState?: boolean) => void;
  onToggleSnapshot?: (resourceId: string, forceState?: boolean) => void;
  showDelayColumn?: boolean;
  globalDelaySeconds?: number;
  editingId: () => string | null;
  editingThresholds: () => Record<string, number | undefined>;
  setEditingThresholds: (value: Record<string, number | undefined>) => void;
  formatMetricValue: (metric: string, value: number | undefined) => string;
  hasActiveAlert: (resourceId: string, metric: string) => boolean;
  globalDefaults?: Record<string, number | undefined>;
  setGlobalDefaults?: (
    value:
      | Record<string, number | undefined>
      | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
  ) => void;
  setHasUnsavedChanges?: (value: boolean) => void;
  globalDisableFlag?: () => boolean;
  onToggleGlobalDisable?: () => void;
  globalDisableOfflineFlag?: () => boolean;
  onToggleGlobalDisableOffline?: () => void;
  metricDelaySeconds?: Record<string, number>;
  onMetricDelayChange?: (metricKey: string, value: number | null) => void;
  groupHeaderMeta?: Record<string, GroupHeaderMeta>;
  factoryDefaults?: Record<string, number | undefined>;
  onResetDefaults?: () => void;
  editingNote: () => string;
  setEditingNote: (value: string) => void;
  onBulkEdit?: (resourceIds: string[]) => void;
}

type OfflineState = 'off' | 'warning' | 'critical';

export function ResourceTable(props: ResourceTableProps) {
  const { isMobile } = useBreakpoint();
  const {
    activeMetricInput,
    setActiveMetricInput,
    showDelayRow,
    setShowDelayRow,
    selectedIds,
    hasRows,
    hasCustomGlobalDefaults,
    toggleSelection,
    toggleAll,
    allSelected,
    someSelected,
    clearSelectedIds,
  } = useAlertResourceTableState({
    resources: props.resources,
    groupedResources: props.groupedResources,
    globalDefaults: props.globalDefaults,
    factoryDefaults: props.factoryDefaults,
  });

  const totalColumnCount = () =>
    props.columns.length + 3 + (props.showOfflineAlertsColumn ? 1 : 0) + (props.onBulkEdit ? 1 : 0);

  const renderGroupHeader = (groupKey: string, meta?: GroupHeaderMeta) => {
    const groupLabel = meta?.displayName || meta?.rawName || groupKey;

    if (!meta || meta.type !== 'agent') {
      return <span class="text-xs font-medium text-muted">{groupLabel}</span>;
    }

    return (
      <div class="flex flex-wrap items-center gap-3">
        <Show
          when={meta.host}
          fallback={<span class="text-sm font-medium text-base-content">{groupLabel}</span>}
        >
          {(host) => (
            <a
              href={host() as string}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()}
              class="text-sm font-medium text-base-content transition-colors duration-150 hover:text-sky-600 dark:hover:text-sky-400"
              title={`Open ${groupLabel} web interface`}
            >
              {groupLabel}
            </a>
          )}
        </Show>
        <Show when={meta.clusterName}>
          <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">
            {meta.clusterName}
          </span>
        </Show>
      </div>
    );
  };
  const getThresholds = (resource: Resource, isEditing: boolean): AlertResourceThresholdMap =>
    isEditing ? props.editingThresholds() : (resource.thresholds ?? {});
  const getResourceLabel = (resource: Resource) => getAlertResourceLabel(resource);
  const getDisplayValue = (resource: Resource, metric: string, isEditing: boolean) =>
    getAlertResourceMetricDisplayValue(resource, metric, props.editingThresholds(), isEditing);
  const startEditing = (resource: Resource, metric?: string, event?: MouseEvent) => {
    event?.stopPropagation();
    if (resource.editable === false) {
      return;
    }
    if (metric) {
      setActiveMetricInput({ resourceId: resource.id, metric });
    }
    const payload = buildAlertResourceEditPayload(resource);
    props.onEdit(resource.id, payload.thresholds, payload.defaults, payload.note);
  };
  const cancelEditing = () => {
    props.onCancelEdit();
    setActiveMetricInput(null);
  };
  const saveEditing = (resourceId: string) => {
    props.onSaveEdit(resourceId);
    setActiveMetricInput(null);
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
          <div
            class="w-1.5 h-1.5 rounded-full bg-red-500 animate-pulse"
            title="Active alert"
          />{' '}
        </Show>{' '}
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

  const renderMobileView = () => (
    <div class="space-y-4">
      {/* Global Defaults - Mobile Card */}
      <Show when={props.globalDefaults && props.setGlobalDefaults && props.setHasUnsavedChanges}>
        <Card
          padding="sm"
          class="border border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900"
        >
          <div class="flex justify-between items-center mb-3">
            <div class="flex items-center gap-2">
              <span class="font-semibold text-sm">Global Defaults</span>
              <Show when={hasCustomGlobalDefaults()}>
                <span class="text-[10px] px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                  {getAlertResourceTableCustomBadgeLabel()}
                </span>
              </Show>
            </div>
            <Show when={props.onToggleGlobalDisable}>
              <TogglePrimitive
                size="sm"
                checked={!props.globalDisableFlag?.()}
                onToggle={() => {
                  props.onToggleGlobalDisable?.();
                  props.setHasUnsavedChanges?.(true);
                }}
              />
            </Show>
          </div>

          <div class="grid grid-cols-2 gap-2">
            <For
              each={props.columns.filter((c) => {
                const m = normalizeAlertResourceMetricKey(c);
                return m !== 'backup' && m !== 'snapshot';
              })}
            >
              {(column) => {
                const metric = normalizeAlertResourceMetricKey(column);
                const bounds = getAlertResourceMetricBounds(metric);
                const val = () => props.globalDefaults?.[metric] ?? 0;
                const isOff = () => val() === -1;
                return (
                  <div class="p-2 bg-surface rounded border border-border-subtle flex flex-col gap-1">
                    <span class="text-[10px] uppercase text-slate-500 font-medium">{column}</span>
                    <div class="relative">
                      <input
                        type="number"
                        min={bounds.min}
                        max={bounds.max}
                        step={getAlertResourceMetricStep(metric)}
                        value={isOff() ? '' : val()}
                        placeholder={getAlertResourceTableMetricPlaceholder(isOff())}
                        disabled={isOff()}
                        class={`w-full text-sm p-1 rounded border text-center ${isOff() ? 'bg-surface-hover' : ' border-border'}`}
                        onInput={(e) => {
                          const value = parseFloat(e.currentTarget.value);
                          if (props.setGlobalDefaults) {
                            props.setGlobalDefaults((prev) => ({
                              ...prev,
                              [metric]: Number.isNaN(value) ? 0 : value,
                            }));
                          }
                          if (props.setHasUnsavedChanges) props.setHasUnsavedChanges(true);
                        }}
                      />
                      <Show when={isOff()}>
                        <button
                          type="button"
                          class="absolute inset-0 w-full"
                          onClick={() => {
                            if (!props.setGlobalDefaults) return;
                            props.setGlobalDefaults((prev) => ({
                              ...prev,
                              [metric]: getAlertResourceEnabledDefault(metric),
                            }));
                            props.setHasUnsavedChanges?.(true);
                          }}
                          aria-label={`Enable ${column} default`}
                        ></button>
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
          props.groupedResources
            ? Object.entries(props.groupedResources).sort(([a], [b]) => a.localeCompare(b))
            : [['default', props.resources || []] as [string, Resource[]]]
        }
      >
        {([groupName, resources]: [string, Resource[]]) => (
          <div class="space-y-2">
            <Show when={props.groupedResources}>
              <div class="px-1 font-medium text-xs text-slate-500 uppercase mt-4 mb-1">
                {renderGroupHeader(groupName, props.groupHeaderMeta?.[groupName])}
              </div>
            </Show>
            <For each={resources as Resource[]}>
              {(resource) => {
                const isEditing = () => props.editingId() === resource.id;
                const thresholds = () => getThresholds(resource, isEditing());
                const displayValue = (metric: string) =>
                  getDisplayValue(resource, metric, isEditing());
                const isOverridden = (metric: string) =>
                  isAlertResourceMetricOverridden(resource, metric);

                return (
                  <Card
                    padding="sm"
                    class={`flex flex-col gap-3 transition-opacity ${resource.disabled || props.globalDisableFlag?.() ? 'opacity-60' : ''}`}
                  >
                    {/* Header */}
                    <div class="flex items-center justify-between">
                      <div class="flex items-center gap-3 min-w-0">
                        {/* Toggle */}
                        <Show when={props.onToggleDisabled}>
                          <div class="shrink-0 scale-90 origin-left">
                            <TogglePrimitive
                              size="sm"
                              checked={
                                !(props.globalDisableFlag?.() ?? false) && !resource.disabled
                              }
                              disabled={props.globalDisableFlag?.() ?? false}
                              onToggle={() =>
                                !(props.globalDisableFlag?.() ?? false) &&
                                props.onToggleDisabled?.(resource.id)
                              }
                            />
                          </div>
                        </Show>

                        {/* Name */}
                        <div class="min-w-0 truncate">
                          <div class="font-medium text-sm truncate">
                            {getResourceLabel(resource)}
                          </div>
                          <Show when={resource.subtitle}>
                            <div class="text-xs text-slate-500 truncate">{resource.subtitle}</div>
                          </Show>
                        </div>
                      </div>

                      {/* Actions */}
                      <div class="flex gap-1 shrink-0">
                        <Show when={!isEditing() && resource.type !== 'dockerHost'}>
                          <button
                            type="button"
                            onClick={() => startEditing(resource)}
                            class="p-1.5 bg-blue-50 dark:bg-blue-900 text-blue-600 rounded"
                            aria-label={`Edit thresholds for ${getResourceLabel(resource)}`}
                          >
                            <svg
                              class="w-4 h-4"
                              fill="none"
                              stroke="currentColor"
                              viewBox="0 0 24 24"
                            >
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
                            <svg
                              class="w-4 h-4"
                              fill="none"
                              stroke="currentColor"
                              viewBox="0 0 24 24"
                            >
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
                            aria-label={`Save threshold edits for ${getResourceLabel(resource)}`}
                          >
                            <svg
                              class="w-4 h-4"
                              fill="none"
                              stroke="currentColor"
                              viewBox="0 0 24 24"
                            >
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
                            onClick={() => props.onRemoveOverride(resource.id)}
                            class="p-1.5 bg-surface-alt hover:text-muted rounded transition-colors"
                            aria-label={`Revert to defaults for ${getResourceLabel(resource)}`}
                            title={getAlertResourceTableRevertToDefaultsLabel()}
                          >
                            <RotateCcw class="w-4 h-4" />
                          </button>
                        </Show>
                      </div>
                    </div>

                    {/* Note editor in mobile */}
                    <Show when={isEditing()}>
                      <textarea
                        class="w-full text-xs p-2 rounded border border-border bg-surface-alt"
                        rows={2}
                        placeholder={getAlertResourceTableEditNotePlaceholder()}
                        value={props.editingNote()}
                        onInput={(e) => props.setEditingNote(e.currentTarget.value)}
                      />
                    </Show>

                    {/* Metrics Grid */}
                    <div class="grid grid-cols-2 gap-2 text-sm border-t pt-2">
                      <For each={props.columns}>
                        {(column) => {
                          const metric = normalizeAlertResourceMetricKey(column);
                          // Check support
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
                                    aria-label={`Edit ${column} threshold for ${getResourceLabel(resource)}`}
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
                                    const val = parseFloat(e.currentTarget.value);
                                    props.setEditingThresholds({
                                      ...props.editingThresholds(),
                                      [metric]: Number.isNaN(val) ? undefined : val,
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
      <Show when={hasRows() === false}>
        <div class="text-center p-8 text-slate-500 text-sm italic bg-surface-alt rounded-md">
          {getAlertResourceTableEmptyState(props.emptyMessage)}
        </div>
      </Show>
    </div>
  );

  return (
    <>
      <Show when={!isMobile()} fallback={renderMobileView()}>
        <Card
          padding="none"
          class="overflow-hidden border border-border"
          border={false}
          tone="card"
        >
          <div class="px-4 py-3 border-b border-border">
            <SectionHeader title={props.title} size="sm" />
          </div>
          <Table class="w-full whitespace-normal">
            <TableHeader>
              <TableRow class="text-muted">
                <Show when={props.onBulkEdit}>
                  <TableHead class="text-center w-10 px-2 border-r border-border">
                    <input
                      type="checkbox"
                      checked={allSelected()}
                      ref={(el) => {
                        createEffect(() => {
                          el.indeterminate = someSelected();
                        });
                      }}
                      onChange={(e) => toggleAll(e.currentTarget.checked)}
                      class="rounded text-sky-600 focus:ring-sky-500 transition-shadow cursor-pointer"
                      aria-label="Select all resources"
                    />
                  </TableHead>
                </Show>
                <TableHead class="text-center w-16">Alerts</TableHead>
                <TableHead class="text-left w-1/4">Resource</TableHead>
                <For each={props.columns}>
                  {(column) => (
                    <TableHead
                      class="text-center whitespace-normal break-words"
                      title={getAlertResourceColumnHeaderTooltip(column)}
                    >
                      {column}
                    </TableHead>
                  )}
                </For>
                <Show when={props.showOfflineAlertsColumn}>
                  <TableHead class="text-center" title={OFFLINE_ALERTS_TOOLTIP}>
                    Offline Alerts
                  </TableHead>
                </Show>
                <TableHead class="text-center">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody class="divide-y divide-border">
              {/* Global Defaults Row */}
              <Show
                when={props.globalDefaults && props.setGlobalDefaults && props.setHasUnsavedChanges}
              >
                <TableRow
                  class={`bg-surface-alt ${props.globalDisableFlag?.() ? 'opacity-40' : ''}`}
                >
                  <Show when={props.onBulkEdit}>
                    <TableCell class="p-1 px-2 border-r border-border" />
                  </Show>
                  <TableCell class="p-1 px-2 text-center align-middle">
                    <Show
                      when={props.onToggleGlobalDisable}
                      fallback={<span class="text-sm text-slate-400">-</span>}
                    >
                      <div class="flex items-center justify-center">
                        <TogglePrimitive
                          size="sm"
                          checked={!props.globalDisableFlag?.()}
                          onToggle={() => {
                            props.onToggleGlobalDisable?.();
                            props.setHasUnsavedChanges?.(true);
                          }}
                          class="my-[1px]"
                          title="Global alerts toggle - disable all alerts for this resource type"
                          ariaLabel="Global alerts toggle"
                        />
                      </div>
                    </Show>
                  </TableCell>
                  <TableCell class="p-1 px-2">
                    <div class="flex items-center gap-2">
                      <span class="text-sm font-semibold text-base-content">Global Defaults</span>
                      <Show when={hasCustomGlobalDefaults()}>
                        <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                          {getAlertResourceTableCustomBadgeLabel()}
                        </span>
                      </Show>
                    </div>
                  </TableCell>
                  <For each={props.columns}>
                    {(column) => {
                      const metric = normalizeAlertResourceMetricKey(column);
                      const bounds = getAlertResourceMetricBounds(metric);
                      const val = () => props.globalDefaults?.[metric] ?? 0;
                      const isOff = () => val() === -1;

                      return (
                        <TableCell class="p-1 px-2 text-center align-middle">
                          <div class="relative flex justify-center w-full">
                            <input
                              type="number"
                              min={bounds.min}
                              max={bounds.max}
                              step={getAlertResourceMetricStep(metric)}
                              value={isOff() ? '' : val()}
                              placeholder={getAlertResourceTableMetricPlaceholder(isOff())}
                              disabled={isOff()}
                              onInput={(e) => {
                                const value = parseFloat(e.currentTarget.value);
                                if (props.setGlobalDefaults) {
                                  props.setGlobalDefaults((prev) => ({
                                    ...prev,
                                    [metric]: Number.isNaN(value) ? 0 : value,
                                  }));
                                }
                                if (props.setHasUnsavedChanges) {
                                  props.setHasUnsavedChanges(true);
                                }
                              }}
                              class={`w-16 px-2 py-0.5 text-sm text-center border rounded ${
                                isOff()
                                  ? 'border-border bg-surface-alt text-muted italic placeholder: dark:placeholder: placeholder:opacity-60 pointer-events-none'
                                  : 'border-border text-base-content focus:border-blue-500 focus:ring-1 focus:ring-blue-500'
                              }`}
                              title={getAlertResourceTableMetricInputTitle(isOff())}
                            />
                            <Show when={isOff()}>
                              <button
                                type="button"
                                class="absolute inset-0 w-full rounded cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
                                onClick={() => {
                                  if (!props.setGlobalDefaults) return;
                                  props.setGlobalDefaults((prev) => ({
                                    ...prev,
                                    [metric]: getAlertResourceEnabledDefault(metric),
                                  }));
                                  props.setHasUnsavedChanges?.(true);
                                }}
                                title={getAlertResourceTableMetricInputTitle(true)}
                              >
                                <span class="sr-only">Enable {column} default</span>
                              </button>
                            </Show>
                          </div>
                        </TableCell>
                      );
                    }}
                  </For>
                  <Show when={props.showOfflineAlertsColumn}>
                    <TableCell class="p-1 px-2 text-center align-middle">
                      <Show
                        when={props.onSetGlobalOfflineState}
                        fallback={
                          <Show
                            when={props.onToggleGlobalDisableOffline}
                            fallback={<span class="text-sm text-slate-400">-</span>}
                          >
                            {(() => {
                              const defaultDisabled = props.globalDisableOfflineFlag?.() ?? false;
                              return renderToggleBadge({
                                isEnabled: !defaultDisabled,
                                size: 'md',
                                onToggle: () => {
                                  props.onToggleGlobalDisableOffline?.();
                                  props.setHasUnsavedChanges?.(true);
                                },
                                labelEnabled: 'On',
                                labelDisabled: 'Off',
                                titleEnabled:
                                  'Offline alerts currently enabled by default. Click to disable.',
                                titleDisabled:
                                  'Offline alerts currently disabled by default. Click to enable.',
                              });
                            })()}
                          </Show>
                        }
                      >
                        {(() => {
                          const disabledGlobally = props.globalDisableFlag?.() ?? false;
                          const defaultDisabled = props.globalDisableOfflineFlag?.() ?? false;
                          const defaultSeverity = props.globalOfflineSeverity ?? 'warning';
                          const state: OfflineState = defaultDisabled
                            ? 'off'
                            : defaultSeverity === 'critical'
                              ? 'critical'
                              : 'warning';

                          return renderOfflineStateButton(state, disabledGlobally, () => {
                            if (disabledGlobally) return;
                            const next = nextOfflineState(state);
                            props.onSetGlobalOfflineState?.(next);
                          });
                        })()}
                      </Show>
                    </TableCell>
                  </Show>
                  <TableCell class="p-1 px-2 text-center align-middle">
                    <div class="flex items-center justify-center gap-1">
                      <Show
                        when={
                          props.showDelayColumn && typeof props.onMetricDelayChange === 'function'
                        }
                      >
                        <button
                          type="button"
                          onClick={() => setShowDelayRow(!showDelayRow())}
                          class="p-1 hover:text-muted transition-colors"
                          title={
                            showDelayRow()
                              ? 'Hide alert delay settings'
                              : 'Show alert delay settings'
                          }
                          aria-label={
                            showDelayRow()
                              ? 'Hide alert delay settings'
                              : 'Show alert delay settings'
                          }
                        >
                          <svg
                            class={`w-4 h-4 transition-transform ${showDelayRow() ? 'rotate-180' : ''}`}
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M19 9l-7 7-7-7"
                            />
                          </svg>
                        </button>
                      </Show>
                      <Show when={hasCustomGlobalDefaults() && props.onResetDefaults}>
                        <button
                          type="button"
                          onClick={() => props.onResetDefaults?.()}
                          class="p-1 text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 transition-colors"
                          title={getAlertResourceTableResetFactoryDefaultsLabel()}
                          aria-label={getAlertResourceTableResetFactoryDefaultsLabel()}
                        >
                          <svg
                            class="w-4 h-4"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                            />
                          </svg>
                        </button>
                      </Show>
                      <Show when={!props.showDelayColumn && !hasCustomGlobalDefaults()}>
                        <span class="text-sm text-slate-400">-</span>
                      </Show>
                    </div>
                  </TableCell>
                </TableRow>
              </Show>
              <Show
                when={
                  showDelayRow() &&
                  props.showDelayColumn &&
                  typeof props.onMetricDelayChange === 'function'
                }
              >
                <TableRow
                  class={`bg-surface-alt ${props.globalDisableFlag?.() ? 'opacity-40' : ''}`}
                >
                  <Show when={props.onBulkEdit}>
                    <TableCell class="p-1 px-2 border-r border-border" />
                  </Show>
                  <TableCell class="p-1 px-2 text-center align-middle">
                    <span class="text-sm">-</span>
                  </TableCell>
                  <TableCell class="p-1 px-2 align-middle">
                    <span class="text-xs font-semibold uppercase tracking-wide text-muted inline-flex items-center gap-1">
                      {getAlertResourceTableAlertDelayLabel()}
                      <HelpIcon contentId="alerts.thresholds.delay" size="xs" />
                    </span>
                  </TableCell>
                  <For each={props.columns}>
                    {(column) => {
                      const metric = normalizeAlertResourceMetricKey(column);
                      const typeDefaultDelay = props.globalDelaySeconds ?? 5;
                      const overrideDelay = getAlertResourceMetricDelayOverride(
                        props.metricDelaySeconds,
                        metric,
                      );

                      return (
                        <TableCell class="p-1 px-2 text-center align-middle">
                          <div class="relative flex justify-center w-full">
                            <input
                              type="number"
                              min="0"
                              value={(() => {
                                return overrideDelay !== undefined ? overrideDelay : '';
                              })()}
                              placeholder={String(typeDefaultDelay)}
                              class="w-16 rounded border border-border bg-surface px-2 py-0.5 text-sm text-center text-base-content focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                              onInput={(e) => {
                                const raw = e.currentTarget.value;
                                if (raw === '') {
                                  props.onMetricDelayChange?.(metric, null);
                                  props.setHasUnsavedChanges?.(true);
                                } else {
                                  const parsed = parseInt(raw, 10);
                                  if (Number.isNaN(parsed)) {
                                    return;
                                  }
                                  const sanitized = Math.max(0, parsed);
                                  // If the value matches the default, clear the override
                                  if (sanitized === typeDefaultDelay) {
                                    props.onMetricDelayChange?.(metric, null);
                                  } else {
                                    props.onMetricDelayChange?.(metric, sanitized);
                                  }
                                  props.setHasUnsavedChanges?.(true);
                                }
                              }}
                            />
                          </div>
                        </TableCell>
                      );
                    }}
                  </For>
                  <Show when={props.showOfflineAlertsColumn}>
                    <TableCell class="p-1 px-2 text-center align-middle">
                      <span class="text-sm">-</span>
                    </TableCell>
                  </Show>
                  <TableCell class="p-1 px-2 text-center align-middle">
                    <span class="text-sm">-</span>
                  </TableCell>
                </TableRow>
              </Show>
              <Show when={props.groupedResources}>
                <For
                  each={Object.entries(props.groupedResources || {}).sort(([a], [b]) =>
                    a.localeCompare(b),
                  )}
                >
                  {([nodeName, resources]) => {
                    const headerMeta = props.groupHeaderMeta?.[nodeName];

                    return (
                      <>
                        {/* Node group header */}
                        <TableRow class="bg-surface-alt">
                          <TableCell
                            colspan={totalColumnCount()}
                            class="p-1 px-2 text-xs font-medium text-muted"
                          >
                            {renderGroupHeader(nodeName, headerMeta)}
                          </TableCell>
                        </TableRow>
                        {/* Resources in this group */}
                        <For each={resources}>
                          {(resource) => {
                            const isEditing = () => props.editingId() === resource.id;
                            const thresholds = () => getThresholds(resource, isEditing());
                            const displayValue = (metric: string) =>
                              getDisplayValue(resource, metric, isEditing());
                            const isOverridden = (metric: string) =>
                              isAlertResourceMetricOverridden(resource, metric);

                            return (
                              <TableRow
                                class={`transition-colors ${resource.disabled || props.globalDisableFlag?.() ? 'opacity-40' : ''} ${resource.hasOverride ? 'bg-sky-50/50 hover:bg-sky-50/80 dark:bg-sky-900/20 dark:hover:bg-sky-900/30 border-l-[3px] border-l-sky-400 dark:border-l-sky-500' : 'hover:bg-surface-hover'}`}
                              >
                                {/* Bulk Edit Checkbox column */}
                                <Show when={props.onBulkEdit}>
                                  <TableCell class="p-1 px-2 text-center align-middle border-r border-border">
                                    <input
                                      type="checkbox"
                                      checked={selectedIds().has(resource.id)}
                                      onChange={(e) =>
                                        toggleSelection(resource.id, e.currentTarget.checked)
                                      }
                                      class="rounded border-border text-sky-600 focus:ring-sky-500 transition-shadow cursor-pointer"
                                      aria-label={`Select ${getResourceLabel(resource)}`}
                                    />
                                  </TableCell>
                                </Show>

                                {/* Alert toggle column */}
                                <TableCell class="p-1 px-2 text-center align-middle">
                                  <Show when={props.onToggleDisabled}>
                                    {(() => {
                                      const globallyDisabled = props.globalDisableFlag?.() ?? false;
                                      const isChecked = !globallyDisabled && !resource.disabled;
                                      return (
                                        <div class="flex items-center justify-center">
                                          <TogglePrimitive
                                            size="sm"
                                            checked={isChecked}
                                            disabled={globallyDisabled}
                                            onToggle={() =>
                                              !globallyDisabled &&
                                              props.onToggleDisabled?.(resource.id)
                                            }
                                            class="my-[1px]"
                                            title={
                                              globallyDisabled
                                                ? 'Alerts disabled globally'
                                                : resource.disabled
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
                                      when={resource.type === 'agent'}
                                      fallback={
                                        <span
                                          class={`text-sm font-medium truncate flex-nowrap ${resource.disabled ? 'text-slate-500 ' : 'text-base-content'}`}
                                        >
                                          {getResourceLabel(resource)}
                                        </span>
                                      }
                                    >
                                      <div
                                        class="flex items-center gap-3 min-w-0"
                                        title={resource.status || undefined}
                                      >
                                        <Show
                                          when={resource.host}
                                          fallback={
                                            <span
                                              class={`text-sm font-medium truncate flex-nowrap ${resource.disabled ? 'text-slate-500 ' : 'text-base-content'}`}
                                            >
                                              {getResourceLabel(resource)}
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
                                                resource.disabled
                                                  ? 'text-slate-500 '
                                                  : 'text-base-content hover:text-sky-600 dark:hover:text-sky-400'
                                              }`}
                                              title={`Open ${getResourceLabel(resource)} web interface`}
                                            >
                                              {getResourceLabel(resource)}
                                            </a>
                                          )}
                                        </Show>
                                        <Show when={resource.clusterName}>
                                          <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">
                                            {resource.clusterName}
                                          </span>
                                        </Show>
                                        <Show when={resource.type === 'storage' && resource.node}>
                                          <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300">
                                            {resource.node}
                                          </span>
                                        </Show>
                                      </div>
                                    </Show>
                                    <Show when={resource.subtitle}>
                                      <span class="text-xs text-muted">
                                        {resource.subtitle as string}
                                      </span>
                                    </Show>
                                    <Show
                                      when={resource.hasOverride || resource.disableConnectivity}
                                    >
                                      <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                                        {getAlertResourceTableCustomBadgeLabel()}
                                      </span>
                                    </Show>
                                    <Show when={isEditing()}>
                                      <div class="mt-2 w-full">
                                        <label class="sr-only" for={`note-${resource.id}`}>
                                          Override note
                                        </label>
                                        <textarea
                                          id={`note-${resource.id}`}
                                          class="w-full rounded border border-border bg-surface px-2 py-1 text-xs text-base-content focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                                          rows={2}
                                          placeholder={getAlertResourceTableOverrideNotePlaceholder()}
                                          value={props.editingNote()}
                                          onInput={(e) =>
                                            props.setEditingNote(e.currentTarget.value)
                                          }
                                        />
                                      </div>
                                    </Show>
                                    <Show when={!isEditing() && resource.note}>
                                      <p class="mt-2 text-xs italic text-muted break-words">
                                        {resource.note as string}
                                      </p>
                                    </Show>
                                  </div>
                                </TableCell>
                                {/* Metric columns - dynamically rendered based on resource type */}
                                <For each={props.columns}>
                                  {(column) => {
                                    const metric = normalizeAlertResourceMetricKey(column);
                                    const showMetric = () =>
                                      alertResourceSupportsMetric(resource.type, metric);
                                    const bounds = getAlertResourceMetricBounds(metric);
                                    const isDisabled = () => thresholds()?.[metric] === -1;
                                    const isSpecialToggle =
                                      metric === 'backup' || metric === 'snapshot';

                                    if (isSpecialToggle) {
                                      const config =
                                        metric === 'backup' ? resource.backup : resource.snapshot;
                                      const isEnabled = config?.enabled ?? true;
                                      const onToggle =
                                        metric === 'backup'
                                          ? props.onToggleBackup
                                          : props.onToggleSnapshot;
                                      const titlePrefix =
                                        metric === 'backup' ? 'Backup' : 'Snapshot';

                                      return (
                                        <TableCell class="p-1 px-2 text-center align-middle">
                                          <Show
                                            when={onToggle}
                                            fallback={<span class="text-sm text-slate-400">-</span>}
                                          >
                                            <div class="flex items-center justify-center">
                                              <StatusBadge
                                                isEnabled={isEnabled}
                                                onToggle={() => onToggle?.(resource.id)}
                                                titleEnabled={`${titlePrefix} alerts enabled. Click to disable for this resource.`}
                                                titleDisabled={`${titlePrefix} alerts disabled. Click to enable for this resource.`}
                                              />
                                            </div>
                                          </Show>
                                        </TableCell>
                                      );
                                    }

                                    const openMetricEditor = (e: MouseEvent) => {
                                      startEditing(resource, metric, e);
                                    };

                                    return (
                                      <TableCell class="p-1 px-2 text-center align-middle">
                                        <Show
                                          when={showMetric()}
                                          fallback={<span class="text-sm text-muted">-</span>}
                                        >
                                          <Show
                                            when={isEditing()}
                                            fallback={
                                              <div
                                                onClick={(event) => {
                                                  openMetricEditor(event);
                                                }}
                                                class="cursor-pointer hover:bg-surface-hover rounded px-1 py-0.5 transition-colors"
                                                title={getAlertResourceTableEditMetricTitle()}
                                              >
                                                <MetricValueWithHeat
                                                  resourceId={resource.id}
                                                  metric={metric}
                                                  value={displayValue(metric)}
                                                  isOverridden={isOverridden(metric)}
                                                />
                                              </div>
                                            }
                                          >
                                            <div class="flex w-full items-center gap-3">
                                              <Show when={ALERT_RESOURCE_TABLE_SLIDER_METRICS.has(metric)}>
                                                {(() => {
                                                  const isTemperatureMetric =
                                                    metric === 'temperature' ||
                                                    metric === 'diskTemperature';
                                                  const sliderMin = Math.max(0, bounds.min);
                                                  const sliderMax = isTemperatureMetric
                                                    ? Math.max(
                                                        sliderMin,
                                                        bounds.max > 0 ? bounds.max : 120,
                                                      )
                                                    : bounds.max;
                                                  const defaultSliderValue = () => {
                                                    if (metric === 'disk') return 90;
                                                    if (metric === 'memory') return 85;
                                                    if (metric === 'temperature') return 80;
                                                    if (metric === 'diskTemperature') return 55;
                                                    return 80;
                                                  };
                                                  const currentSliderValue = () => {
                                                    const editingVal =
                                                      props.editingThresholds()?.[metric];
                                                    if (
                                                      typeof editingVal === 'number' &&
                                                      editingVal >= 0
                                                    ) {
                                                      return Math.round(editingVal);
                                                    }
                                                    const displayVal = displayValue(metric);
                                                    if (
                                                      typeof displayVal === 'number' &&
                                                      displayVal >= 0
                                                    ) {
                                                      return Math.round(displayVal);
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
                                                  value={thresholds()?.[metric] ?? ''}
                                                  placeholder={getAlertResourceTableMetricPlaceholder(
                                                    isDisabled(),
                                                  )}
                                                  title={getAlertResourceTableMetricInputTitle(
                                                    isDisabled(),
                                                  )}
                                                  ref={(el) => {
                                                    if (
                                                      isEditing() &&
                                                      activeMetricInput()?.resourceId ===
                                                        resource.id &&
                                                      activeMetricInput()?.metric === metric
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
                                                    if (props.editingId() === resource.id) {
                                                      saveEditing(resource.id);
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

                                {/* Offline Alerts column - Connectivity/powered-off alerts */}
                                <Show when={props.showOfflineAlertsColumn}>
                                  <TableCell class="p-1 px-2 text-center align-middle">
                                    {(() => {
                                      const disabledGlobally = props.globalDisableFlag?.() ?? false;
                                      const supportsTriState =
                                        typeof props.onSetOfflineState === 'function' &&
                                        (resource.type === 'guest' ||
                                          resource.type === 'dockerContainer');

                                      if (supportsTriState) {
                                        const defaultDisabled =
                                          props.globalDisableOfflineFlag?.() ?? false;
                                        const defaultSeverity =
                                          props.globalOfflineSeverity ?? 'warning';

                                        let state: OfflineState;
                                        if (resource.disableConnectivity) {
                                          state = 'off';
                                        } else if (resource.poweredOffSeverity) {
                                          state = resource.poweredOffSeverity;
                                        } else if (defaultDisabled) {
                                          state = 'off';
                                        } else {
                                          state =
                                            defaultSeverity === 'critical' ? 'critical' : 'warning';
                                        }

                                        return renderOfflineStateButton(
                                          state,
                                          disabledGlobally,
                                          () => {
                                            if (disabledGlobally) return;
                                            const next = nextOfflineState(state);
                                            props.onSetOfflineState?.(resource.id, next);
                                          },
                                        );
                                      }

                                      if (!props.onToggleNodeConnectivity) {
                                        return <span class="text-sm text-slate-400">-</span>;
                                      }

                                      const globalOfflineDisabled =
                                        props.globalDisableOfflineFlag?.() ?? false;
                                      return renderToggleBadge({
                                        isEnabled:
                                          !globalOfflineDisabled && !resource.disableConnectivity,
                                        disabled: disabledGlobally,
                                        onToggle: () => {
                                          if (disabledGlobally) return;
                                          props.onToggleNodeConnectivity?.(resource.id);
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

                                {/* Actions column */}
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
                                          <svg
                                            class="w-4 h-4"
                                            fill="none"
                                            stroke="currentColor"
                                            viewBox="0 0 24 24"
                                          >
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
                                      <Show when={resource.type !== 'dockerHost'}>
                                        <button
                                          type="button"
                                          onClick={() => startEditing(resource)}
                                          class="p-1 text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                                          title="Edit thresholds"
                                          aria-label={`Edit thresholds for ${getResourceLabel(resource)}`}
                                        >
                                          <svg
                                            class="w-4 h-4"
                                            fill="none"
                                            stroke="currentColor"
                                            viewBox="0 0 24 24"
                                          >
                                            <path
                                              stroke-linecap="round"
                                              stroke-linejoin="round"
                                              stroke-width="2"
                                              d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                                            />
                                          </svg>
                                        </button>
                                      </Show>
                                      <Show
                                        when={
                                          resource.hasOverride ||
                                          ((resource.type === 'agent' ||
                                            resource.type === 'dockerHost') &&
                                            resource.disableConnectivity)
                                        }
                                      >
                                        <button
                                          type="button"
                                          onClick={() => props.onRemoveOverride(resource.id)}
                                          class="p-1 hover:text-muted transition-colors"
                                          title={getAlertResourceTableRevertToDefaultsLabel()}
                                          aria-label={`Revert to defaults for ${getResourceLabel(resource)}`}
                                        >
                                          <RotateCcw class="w-4 h-4" />
                                        </button>
                                      </Show>
                                    </Show>
                                  </div>
                                </TableCell>
                              </TableRow>
                            );
                          }}
                        </For>
                      </>
                    );
                  }}
                </For>
              </Show>
              <Show when={!props.groupedResources && props.resources}>
                <Show
                  when={props.resources && props.resources.length === 0}
                  fallback={
                    <For each={props.resources}>
                      {(resource) => {
                        const isEditing = () => props.editingId() === resource.id;
                        const thresholds = () => getThresholds(resource, isEditing());
                        const displayValue = (metric: string) =>
                          getDisplayValue(resource, metric, isEditing());
                        const isOverridden = (metric: string) =>
                          isAlertResourceMetricOverridden(resource, metric);

                        return (
                          <TableRow
                            class={`transition-colors ${resource.disabled || props.globalDisableFlag?.() ? 'opacity-40' : ''} ${resource.hasOverride ? 'bg-sky-50/50 hover:bg-sky-50/80 dark:bg-sky-900/20 dark:hover:bg-sky-900/30 border-l-[3px] border-l-sky-400 dark:border-l-sky-500' : 'hover:bg-surface-hover'}`}
                          >
                            {/* Bulk Edit Checkbox column */}
                            <Show when={props.onBulkEdit}>
                              <TableCell class="p-1 px-2 text-center align-middle border-r border-border">
                                <input
                                  type="checkbox"
                                  checked={selectedIds().has(resource.id)}
                                  onChange={(e) =>
                                    toggleSelection(resource.id, e.currentTarget.checked)
                                  }
                                  class="rounded border-border text-sky-600 focus:ring-sky-500 transition-shadow cursor-pointer"
                                  aria-label={`Select ${getResourceLabel(resource)}`}
                                />
                              </TableCell>
                            </Show>

                            {/* Alert toggle column */}
                            <TableCell class="p-1 px-2 text-center align-middle">
                              <Show when={props.onToggleDisabled}>
                                {(() => {
                                  const globallyDisabled = props.globalDisableFlag?.() ?? false;
                                  const isChecked = !globallyDisabled && !resource.disabled;
                                  return (
                                    <div class="flex items-center justify-center">
                                      <TogglePrimitive
                                        size="sm"
                                        checked={isChecked}
                                        disabled={globallyDisabled}
                                        onToggle={() =>
                                          !globallyDisabled && props.onToggleDisabled?.(resource.id)
                                        }
                                        class="my-[1px]"
                                        title={
                                          globallyDisabled
                                            ? 'Alerts disabled globally'
                                            : resource.disabled
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
                              <Show
                                when={resource.type === 'agent'}
                                fallback={
                                  <div class="flex items-center gap-2 min-w-0">
                                    <span
                                      class={`text-sm font-medium truncate flex-nowrap ${resource.disabled ? 'text-slate-500 ' : 'text-base-content'}`}
                                    >
                                      {getResourceLabel(resource)}
                                    </span>
                                    <Show
                                      when={resource.hasOverride || resource.disableConnectivity}
                                    >
                                      <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                                        {getAlertResourceTableCustomBadgeLabel()}
                                      </span>
                                    </Show>
                                  </div>
                                }
                              >
                                <div
                                  class="flex items-center gap-3 min-w-0"
                                  title={resource.status || undefined}
                                >
                                  <Show
                                    when={resource.host}
                                    fallback={
                                      <span
                                        class={`text-sm font-medium truncate flex-nowrap ${resource.disabled ? 'text-slate-500 ' : 'text-base-content'}`}
                                      >
                                        {getResourceLabel(resource)}
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
                                          resource.disabled
                                            ? 'text-slate-500 '
                                            : 'text-base-content hover:text-sky-600 dark:hover:text-sky-400'
                                        }`}
                                        title={`Open ${getResourceLabel(resource)} web interface`}
                                      >
                                        {getResourceLabel(resource)}
                                      </a>
                                    )}
                                  </Show>
                                  <Show when={resource.clusterName}>
                                    <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">
                                      {resource.clusterName}
                                    </span>
                                  </Show>
                                  <Show when={resource.hasOverride || resource.disableConnectivity}>
                                    <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                                      {getAlertResourceTableCustomBadgeLabel()}
                                    </span>
                                  </Show>
                                </div>
                              </Show>
                            </TableCell>
                            {/* Metric columns - dynamically rendered based on resource type */}
                            <For each={props.columns}>
                              {(column) => {
                                const metric = normalizeAlertResourceMetricKey(column);

                                // Check if this metric applies to this resource type
                                const showMetric = () => {
                                  if (
                                    resource.type === 'agent' &&
                                    ['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(
                                      metric,
                                    )
                                  ) {
                                    return false;
                                  }
                                  if (resource.type === 'pbs') {
                                    // PBS only has CPU and Memory metrics
                                    return ['cpu', 'memory'].includes(metric);
                                  }
                                  if (resource.type === 'storage') {
                                    return metric === 'usage';
                                  }
                                  return true;
                                };

                                const isDisabled = () => thresholds()?.[metric] === -1;

                                const openMetricEditor = (e: MouseEvent) => {
                                  startEditing(resource, metric, e);
                                };

                                return (
                                  <TableCell class="p-1 px-2 text-center align-middle">
                                    <Show
                                      when={showMetric()}
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
                                              resourceId={resource.id}
                                              metric={metric}
                                              value={displayValue(metric)}
                                              isOverridden={isOverridden(metric)}
                                            />
                                          </div>
                                        }
                                      >
                                        <div class="flex items-center justify-center">
                                          <input
                                            type="number"
                                            min="-1"
                                            max={
                                              metric.includes('disk') ||
                                              metric.includes('memory') ||
                                              metric.includes('cpu') ||
                                              metric === 'usage'
                                                ? 100
                                                : 10000
                                            }
                                            step={getAlertResourceMetricStep(metric)}
                                            value={thresholds()?.[metric] ?? ''}
                                            placeholder={getAlertResourceTableMetricPlaceholder(
                                              isDisabled(),
                                            )}
                                            title={getAlertResourceTableMetricInputTitle(
                                              isDisabled(),
                                            )}
                                            ref={(el) => {
                                              if (
                                                isEditing() &&
                                                activeMetricInput()?.resourceId === resource.id &&
                                                activeMetricInput()?.metric === metric
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
                                              if (props.editingId() === resource.id) {
                                                saveEditing(resource.id);
                                              }
                                            }}
                                            class={`w-16 px-2 py-0.5 text-sm text-center border rounded ${
                                              isDisabled()
                                                ? 'bg-surface-alt text-slate-400 border-border'
                                                : ' text-base-content border-border'
                                            }`}
                                          />
                                        </div>
                                      </Show>
                                    </Show>
                                  </TableCell>
                                );
                              }}
                            </For>

                            {/* Offline Alerts column - Connectivity/powered-off alerts */}
                            <Show when={props.showOfflineAlertsColumn}>
                              <TableCell class="p-1 px-2 text-center align-middle">
                                <Show when={props.onToggleNodeConnectivity}>
                                  {(() => {
                                    const defaultOfflineDisabled =
                                      props.globalDisableOfflineFlag?.() ?? false;
                                    const isEnabled = !(
                                      resource.disableConnectivity || defaultOfflineDisabled
                                    );
                                    const disabledGlobally = props.globalDisableFlag?.() ?? false;
                                    return (
                                      <StatusBadge
                                        isEnabled={isEnabled}
                                        disabled={disabledGlobally}
                                        onToggle={() =>
                                          props.onToggleNodeConnectivity?.(resource.id)
                                        }
                                        titleEnabled="Offline alerts enabled. Click to disable for this resource."
                                        titleDisabled="Offline alerts disabled. Click to enable for this resource."
                                        titleWhenDisabled="Offline alerts controlled globally"
                                      />
                                    );
                                  })()}
                                </Show>
                              </TableCell>
                            </Show>

                            {/* Actions column */}
                            <TableCell class="p-1 px-2">
                              <div class="flex items-center justify-center gap-1">
                                <Show when={typeof resource.toggleEnabled === 'function'}>
                                  <StatusBadge
                                    size="sm"
                                    isEnabled={resource.isEnabled ?? true}
                                    onToggle={resource.toggleEnabled}
                                    titleEnabled={resource.toggleTitleEnabled}
                                    titleDisabled={resource.toggleTitleDisabled}
                                  />
                                </Show>
                                <Show
                                  when={
                                    resource.editable !== false &&
                                    typeof props.onEdit === 'function'
                                  }
                                  fallback={<span class="text-xs text-muted">—</span>}
                                >
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
                                        <svg
                                          class="w-4 h-4"
                                          fill="none"
                                          stroke="currentColor"
                                          viewBox="0 0 24 24"
                                        >
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
                                    <div class="flex items-center gap-1">
                                      <button
                                        type="button"
                                        onClick={() => startEditing(resource)}
                                        class="p-1 text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                                        title="Edit thresholds"
                                        aria-label={`Edit thresholds for ${getResourceLabel(resource)}`}
                                      >
                                        <svg
                                          class="w-4 h-4"
                                          fill="none"
                                          stroke="currentColor"
                                          viewBox="0 0 24 24"
                                        >
                                          <path
                                            stroke-linecap="round"
                                            stroke-linejoin="round"
                                            stroke-width="2"
                                            d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                                          />
                                        </svg>
                                      </button>
                                      <Show
                                        when={
                                          resource.hasOverride ||
                                          (resource.type === 'agent' &&
                                            resource.disableConnectivity)
                                        }
                                      >
                                        <button
                                          type="button"
                                          onClick={() => props.onRemoveOverride(resource.id)}
                                          class="p-1 hover:text-base-content transition-colors"
                                          title={getAlertResourceTableRevertToDefaultsLabel()}
                                          aria-label={`Revert to defaults for ${getResourceLabel(resource)}`}
                                        >
                                          <RotateCcw class="w-4 h-4" />
                                        </button>
                                      </Show>
                                    </div>
                                  </Show>
                                </Show>
                              </div>
                            </TableCell>
                          </TableRow>
                        );
                      }}
                    </For>
                  }
                >
                  <TableRow>
                    <TableCell
                      colspan={totalColumnCount()}
                      class="px-4 py-8 text-center text-sm text-muted"
                    >
                      {getAlertResourceTableNoResultsState(props.title)}
                    </TableCell>
                  </TableRow>
                </Show>
              </Show>
              <Show when={hasRows() === false}>
                <TableRow>
                  <TableCell
                    colspan={totalColumnCount()}
                    class="px-4 py-6 text-sm text-center text-muted"
                  >
                    {getAlertResourceTableEmptyState(props.emptyMessage)}
                  </TableCell>
                </TableRow>
              </Show>
            </TableBody>
          </Table>
        </Card>
      </Show>

      <Show when={selectedIds().size > 0 && props.onBulkEdit}>
        <div class="fixed bottom-8 left-1/2 -translate-x-1/2 bg-base border border-border shadow-2xl rounded-full px-5 py-3 flex items-center gap-6 z-[100] animate-in slide-in-from-bottom-5">
          <span class="text-sm font-medium text-white">
            {selectedIds().size} <span class="text-slate-400">selected</span>
          </span>
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="bg-blue-600 hover:bg-blue-500 text-white rounded-full px-5 py-1.5 text-sm font-medium transition-colors shadow-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
              onClick={() => {
                if (props.onBulkEdit) {
                  props.onBulkEdit(Array.from(selectedIds()));
                  clearSelectedIds();
                }
              }}
            >
              {getAlertBulkEditOpenLabel()}
            </button>
            <button
              type="button"
              class="text-slate-400 hover:text-white bg-surface hover:bg-slate-700 rounded-full p-1.5 transition-colors focus:outline-none"
              onClick={clearSelectedIds}
              aria-label={ALERT_BULK_EDIT_CLEAR_LABEL}
              title={ALERT_BULK_EDIT_CLEAR_LABEL}
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
          </div>
        </div>
      </Show>
    </>
  );
}
