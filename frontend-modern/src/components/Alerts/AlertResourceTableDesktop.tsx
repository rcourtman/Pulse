import { For, Show, createEffect } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { TogglePrimitive } from '@/components/shared/Toggle';
import { StatusBadge } from '@/components/shared/StatusBadge';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/shared/Table';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { HelpIcon } from '@/components/shared/HelpIcon';
import { AlertResourceTableRow } from './AlertResourceTableRow';
import { AlertResourceGroupHeader } from './AlertResourceGroupHeader';
import {
  getAlertResourceTableAlertDelayLabel,
  getAlertResourceTableCustomBadgeLabel,
  getAlertResourceTableEmptyState,
  getAlertResourceTableMetricInputTitle,
  getAlertResourceTableMetricPlaceholder,
  getAlertResourceTableOfflineStateOrder,
  getAlertResourceTableOfflineStatePresentation,
  getAlertResourceTableResetFactoryDefaultsLabel,
  getAlertResourceTableNoResultsState,
} from '@/utils/alertResourceTablePresentation';
import {
  getAlertResourceColumnHeaderTooltip,
  getAlertResourceEnabledDefault,
  getAlertResourceMetricBounds,
  getAlertResourceMetricDelayOverride,
  getAlertResourceMetricStep,
  normalizeAlertResourceMetricKey,
} from './alertResourceTableModel';
import type { OfflineState, ResourceTableProps } from './ResourceTable';

const OFFLINE_ALERTS_TOOLTIP =
  'Toggle default behavior for powered-off or connectivity alerts for this resource type.';

interface AlertResourceTableDesktopProps {
  table: ResourceTableProps;
  hasRows: () => boolean;
  hasCustomGlobalDefaults: () => boolean;
  activeMetricInput: () => { resourceId: string; metric: string } | null;
  setActiveMetricInput: (value: { resourceId: string; metric: string } | null) => void;
  showDelayRow: () => boolean;
  setShowDelayRow: (value: boolean) => void;
  selectedIds: () => Set<string>;
  toggleSelection: (id: string, checked: boolean) => void;
  toggleAll: (checked: boolean) => void;
  allSelected: () => boolean;
  someSelected: () => boolean;
}

export function AlertResourceTableDesktop(props: AlertResourceTableDesktopProps) {
  const totalColumnCount = () =>
    props.table.columns.length +
    3 +
    (props.table.showOfflineAlertsColumn ? 1 : 0) +
    (props.table.onBulkEdit ? 1 : 0);

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

  return (
    <Card padding="none" class="overflow-hidden border border-border" border={false} tone="card">
      <div class="px-4 py-3 border-b border-border">
        <SectionHeader title={props.table.title} size="sm" />
      </div>
      <Table class="w-full whitespace-normal">
        <TableHeader>
          <TableRow class="text-muted">
            <Show when={props.table.onBulkEdit}>
              <TableHead class="text-center w-10 px-2 border-r border-border">
                <input
                  type="checkbox"
                  checked={props.allSelected()}
                  ref={(el) => {
                    createEffect(() => {
                      el.indeterminate = props.someSelected();
                    });
                  }}
                  onChange={(e) => props.toggleAll(e.currentTarget.checked)}
                  class="rounded text-sky-600 focus:ring-sky-500 transition-shadow cursor-pointer"
                  aria-label="Select all resources"
                />
              </TableHead>
            </Show>
            <TableHead class="text-center w-16">Alerts</TableHead>
            <TableHead class="text-left w-1/4">Resource</TableHead>
            <For each={props.table.columns}>
              {(column) => (
                <TableHead
                  class="text-center whitespace-normal break-words"
                  title={getAlertResourceColumnHeaderTooltip(column)}
                >
                  {column}
                </TableHead>
              )}
            </For>
            <Show when={props.table.showOfflineAlertsColumn}>
              <TableHead class="text-center" title={OFFLINE_ALERTS_TOOLTIP}>
                Offline Alerts
              </TableHead>
            </Show>
            <TableHead class="text-center">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <Show
            when={
              props.table.globalDefaults &&
              props.table.setGlobalDefaults &&
              props.table.setHasUnsavedChanges
            }
          >
            <TableRow
              class={`bg-surface-alt ${props.table.globalDisableFlag?.() ? 'opacity-40' : ''}`}
            >
              <Show when={props.table.onBulkEdit}>
                <TableCell class="p-1 px-2 border-r border-border" />
              </Show>
              <TableCell class="p-1 px-2 text-center align-middle">
                <Show
                  when={props.table.onToggleGlobalDisable}
                  fallback={<span class="text-sm text-slate-400">-</span>}
                >
                  <div class="flex items-center justify-center">
                    <TogglePrimitive
                      size="sm"
                      checked={!props.table.globalDisableFlag?.()}
                      onToggle={() => {
                        props.table.onToggleGlobalDisable?.();
                        props.table.setHasUnsavedChanges?.(true);
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
                  <Show when={props.hasCustomGlobalDefaults()}>
                    <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                      {getAlertResourceTableCustomBadgeLabel()}
                    </span>
                  </Show>
                </div>
              </TableCell>
              <For each={props.table.columns}>
                {(column) => {
                  const metric = normalizeAlertResourceMetricKey(column);
                  const bounds = getAlertResourceMetricBounds(metric);
                  const value = () => props.table.globalDefaults?.[metric] ?? 0;
                  const isOff = () => value() === -1;

                  return (
                    <TableCell class="p-1 px-2 text-center align-middle">
                      <div class="relative flex justify-center w-full">
                        <input
                          type="number"
                          min={bounds.min}
                          max={bounds.max}
                          step={getAlertResourceMetricStep(metric)}
                          value={isOff() ? '' : value()}
                          placeholder={getAlertResourceTableMetricPlaceholder(isOff())}
                          disabled={isOff()}
                          onInput={(e) => {
                            const nextValue = parseFloat(e.currentTarget.value);
                            props.table.setGlobalDefaults?.((prev) => ({
                              ...prev,
                              [metric]: Number.isNaN(nextValue) ? 0 : nextValue,
                            }));
                            props.table.setHasUnsavedChanges?.(true);
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
                              props.table.setGlobalDefaults?.((prev) => ({
                                ...prev,
                                [metric]: getAlertResourceEnabledDefault(metric),
                              }));
                              props.table.setHasUnsavedChanges?.(true);
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
              <Show when={props.table.showOfflineAlertsColumn}>
                <TableCell class="p-1 px-2 text-center align-middle">
                  <Show
                    when={props.table.onSetGlobalOfflineState}
                    fallback={
                      <Show
                        when={props.table.onToggleGlobalDisableOffline}
                        fallback={<span class="text-sm text-slate-400">-</span>}
                      >
                        {(() => {
                          const defaultDisabled = props.table.globalDisableOfflineFlag?.() ?? false;

                          return renderToggleBadge({
                            isEnabled: !defaultDisabled,
                            size: 'md',
                            onToggle: () => {
                              props.table.onToggleGlobalDisableOffline?.();
                              props.table.setHasUnsavedChanges?.(true);
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
                      const disabledGlobally = props.table.globalDisableFlag?.() ?? false;
                      const defaultDisabled =
                        props.table.globalDisableOfflineFlag?.() ?? false;
                      const defaultSeverity = props.table.globalOfflineSeverity ?? 'warning';
                      const state: OfflineState = defaultDisabled
                        ? 'off'
                        : defaultSeverity === 'critical'
                          ? 'critical'
                          : 'warning';

                      return renderOfflineStateButton(state, disabledGlobally, () => {
                        if (disabledGlobally) return;
                        const next = nextOfflineState(state);
                        props.table.onSetGlobalOfflineState?.(next);
                      });
                    })()}
                  </Show>
                </TableCell>
              </Show>
              <TableCell class="p-1 px-2 text-center align-middle">
                <div class="flex items-center justify-center gap-1">
                  <Show
                    when={
                      props.table.showDelayColumn &&
                      typeof props.table.onMetricDelayChange === 'function'
                    }
                  >
                    <button
                      type="button"
                      onClick={() => props.setShowDelayRow(!props.showDelayRow())}
                      class="p-1 hover:text-muted transition-colors"
                      title={
                        props.showDelayRow()
                          ? 'Hide alert delay settings'
                          : 'Show alert delay settings'
                      }
                      aria-label={
                        props.showDelayRow()
                          ? 'Hide alert delay settings'
                          : 'Show alert delay settings'
                      }
                    >
                      <svg
                        class={`w-4 h-4 transition-transform ${props.showDelayRow() ? 'rotate-180' : ''}`}
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
                  <Show when={props.hasCustomGlobalDefaults() && props.table.onResetDefaults}>
                    <button
                      type="button"
                      onClick={() => props.table.onResetDefaults?.()}
                      class="p-1 text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 transition-colors"
                      title={getAlertResourceTableResetFactoryDefaultsLabel()}
                      aria-label={getAlertResourceTableResetFactoryDefaultsLabel()}
                    >
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                        />
                      </svg>
                    </button>
                  </Show>
                  <Show when={!props.table.showDelayColumn && !props.hasCustomGlobalDefaults()}>
                    <span class="text-sm text-slate-400">-</span>
                  </Show>
                </div>
              </TableCell>
            </TableRow>
          </Show>
          <Show
            when={
              props.showDelayRow() &&
              props.table.showDelayColumn &&
              typeof props.table.onMetricDelayChange === 'function'
            }
          >
            <TableRow
              class={`bg-surface-alt ${props.table.globalDisableFlag?.() ? 'opacity-40' : ''}`}
            >
              <Show when={props.table.onBulkEdit}>
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
              <For each={props.table.columns}>
                {(column) => {
                  const metric = normalizeAlertResourceMetricKey(column);
                  const typeDefaultDelay = props.table.globalDelaySeconds ?? 5;
                  const overrideDelay = getAlertResourceMetricDelayOverride(
                    props.table.metricDelaySeconds,
                    metric,
                  );

                  return (
                    <TableCell class="p-1 px-2 text-center align-middle">
                      <div class="relative flex justify-center w-full">
                        <input
                          type="number"
                          min="0"
                          value={overrideDelay !== undefined ? overrideDelay : ''}
                          placeholder={String(typeDefaultDelay)}
                          class="w-16 rounded border border-border bg-surface px-2 py-0.5 text-sm text-center text-base-content focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                          onInput={(e) => {
                            const raw = e.currentTarget.value;
                            if (raw === '') {
                              props.table.onMetricDelayChange?.(metric, null);
                              props.table.setHasUnsavedChanges?.(true);
                              return;
                            }

                            const parsed = parseInt(raw, 10);
                            if (Number.isNaN(parsed)) {
                              return;
                            }

                            const sanitized = Math.max(0, parsed);
                            props.table.onMetricDelayChange?.(
                              metric,
                              sanitized === typeDefaultDelay ? null : sanitized,
                            );
                            props.table.setHasUnsavedChanges?.(true);
                          }}
                        />
                      </div>
                    </TableCell>
                  );
                }}
              </For>
              <Show when={props.table.showOfflineAlertsColumn}>
                <TableCell class="p-1 px-2 text-center align-middle">
                  <span class="text-sm">-</span>
                </TableCell>
              </Show>
              <TableCell class="p-1 px-2 text-center align-middle">
                <span class="text-sm">-</span>
              </TableCell>
            </TableRow>
          </Show>
          <Show when={props.table.groupedResources}>
            <For
              each={Object.entries(props.table.groupedResources || {}).sort(([a], [b]) =>
                a.localeCompare(b),
              )}
            >
              {([nodeName, resources]) => (
                <>
                  <TableRow class="bg-surface-alt">
                    <TableCell
                      colspan={totalColumnCount()}
                      class="p-1 px-2 text-xs font-medium text-muted"
                    >
                      <AlertResourceGroupHeader
                        groupKey={nodeName}
                        meta={props.table.groupHeaderMeta?.[nodeName]}
                      />
                    </TableCell>
                  </TableRow>
                  <For each={resources}>
                    {(resource) => (
                      <AlertResourceTableRow
                        resource={resource}
                        columns={props.table.columns}
                        editingId={props.table.editingId}
                        editingThresholds={props.table.editingThresholds}
                        setEditingThresholds={props.table.setEditingThresholds}
                        formatMetricValue={props.table.formatMetricValue}
                        hasActiveAlert={props.table.hasActiveAlert}
                        onEdit={props.table.onEdit}
                        onSaveEdit={props.table.onSaveEdit}
                        onCancelEdit={props.table.onCancelEdit}
                        onRemoveOverride={props.table.onRemoveOverride}
                        onToggleDisabled={props.table.onToggleDisabled}
                        onToggleNodeConnectivity={props.table.onToggleNodeConnectivity}
                        showOfflineAlertsColumn={props.table.showOfflineAlertsColumn}
                        globalOfflineSeverity={props.table.globalOfflineSeverity}
                        onSetOfflineState={props.table.onSetOfflineState}
                        onToggleBackup={props.table.onToggleBackup}
                        onToggleSnapshot={props.table.onToggleSnapshot}
                        globalDisableFlag={props.table.globalDisableFlag}
                        globalDisableOfflineFlag={props.table.globalDisableOfflineFlag}
                        editingNote={props.table.editingNote}
                        setEditingNote={props.table.setEditingNote}
                        activeMetricInput={props.activeMetricInput}
                        setActiveMetricInput={props.setActiveMetricInput}
                        showBulkSelection={Boolean(props.table.onBulkEdit)}
                        selected={props.selectedIds().has(resource.id)}
                        onToggleSelection={(checked) =>
                          props.toggleSelection(resource.id, checked)
                        }
                      />
                    )}
                  </For>
                </>
              )}
            </For>
          </Show>
          <Show when={!props.table.groupedResources && props.table.resources}>
            <Show
              when={props.table.resources && props.table.resources.length === 0}
              fallback={
                <For each={props.table.resources}>
                  {(resource) => (
                    <AlertResourceTableRow
                      resource={resource}
                      columns={props.table.columns}
                      editingId={props.table.editingId}
                      editingThresholds={props.table.editingThresholds}
                      setEditingThresholds={props.table.setEditingThresholds}
                      formatMetricValue={props.table.formatMetricValue}
                      hasActiveAlert={props.table.hasActiveAlert}
                      onEdit={props.table.onEdit}
                      onSaveEdit={props.table.onSaveEdit}
                      onCancelEdit={props.table.onCancelEdit}
                      onRemoveOverride={props.table.onRemoveOverride}
                      onToggleDisabled={props.table.onToggleDisabled}
                      onToggleNodeConnectivity={props.table.onToggleNodeConnectivity}
                      showOfflineAlertsColumn={props.table.showOfflineAlertsColumn}
                      globalOfflineSeverity={props.table.globalOfflineSeverity}
                      onSetOfflineState={props.table.onSetOfflineState}
                      onToggleBackup={props.table.onToggleBackup}
                      onToggleSnapshot={props.table.onToggleSnapshot}
                      globalDisableFlag={props.table.globalDisableFlag}
                      globalDisableOfflineFlag={props.table.globalDisableOfflineFlag}
                      editingNote={props.table.editingNote}
                      setEditingNote={props.table.setEditingNote}
                      activeMetricInput={props.activeMetricInput}
                      setActiveMetricInput={props.setActiveMetricInput}
                      showBulkSelection={Boolean(props.table.onBulkEdit)}
                      selected={props.selectedIds().has(resource.id)}
                      onToggleSelection={(checked) =>
                        props.toggleSelection(resource.id, checked)
                      }
                    />
                  )}
                </For>
              }
            >
              <TableRow>
                <TableCell
                  colspan={totalColumnCount()}
                  class="px-4 py-8 text-center text-sm text-muted"
                >
                  {getAlertResourceTableNoResultsState(props.table.title)}
                </TableCell>
              </TableRow>
            </Show>
          </Show>
          <Show when={props.hasRows() === false}>
            <TableRow>
              <TableCell colspan={totalColumnCount()} class="px-4 py-6 text-sm text-center text-muted">
                {getAlertResourceTableEmptyState(props.table.emptyMessage)}
              </TableCell>
            </TableRow>
          </Show>
        </TableBody>
      </Table>
    </Card>
  );
}
