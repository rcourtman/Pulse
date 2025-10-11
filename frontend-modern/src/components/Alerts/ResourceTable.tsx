import { For, Show, createSignal } from 'solid-js';
import { TogglePrimitive } from '@/components/shared/Toggle';
import { StatusBadge } from '@/components/shared/StatusBadge';
import type { Alert } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';

const COLUMN_TOOLTIP_LOOKUP: Record<string, string> = {
  'cpu %': 'Percent CPU utilization allowed before an alert fires.',
  'memory %': 'Percent memory usage threshold for triggering alerts.',
  'disk %': 'Percent disk usage threshold for this resource.',
  'disk r mb/s': 'Maximum sustained disk read throughput before alerting.',
  'disk w mb/s': 'Maximum sustained disk write throughput before alerting.',
  'net in mb/s': 'Inbound network throughput threshold for alerts.',
  'net out mb/s': 'Outbound network throughput threshold for alerts.',
  'usage %': 'Storage capacity usage percentage that triggers an alert.',
  'temp °c': 'CPU temperature limit for node alerts.',
  'temperature °c': 'CPU temperature limit for node alerts.',
  temperature: 'CPU temperature limit for node alerts.',
  'restart count': 'Maximum container restarts within the evaluation window.',
  'restart window': 'Time window used to evaluate the restart count threshold.',
  'restart window (s)': 'Time window used to evaluate the restart count threshold.',
  'memory warn %': 'Warning threshold for container memory usage.',
  'memory critical %': 'Critical threshold for container memory usage.',
  // PMG (Proxmox Mail Gateway) thresholds
  'queue warn': 'Early warning when total mail queue exceeds this message count.',
  'queue crit': 'Critical alert requiring urgent action when queue reaches this size.',
  'deferred warn': 'Early warning for messages stuck in deferred queue (waiting to retry delivery).',
  'deferred crit': 'Critical threshold for deferred messages indicating serious delivery problems.',
  'hold warn': 'Early warning when administratively held messages exceed this count.',
  'hold crit': 'Critical alert for held messages requiring immediate moderation attention.',
  'oldest warn (min)': 'Early warning when oldest queued message exceeds this age in minutes.',
  'oldest crit (min)': 'Critical alert when message queue age indicates delivery has stalled.',
  'spam warn': 'Early warning for spam messages accumulating in quarantine.',
  'spam crit': 'Critical spam quarantine level requiring urgent intervention.',
  'virus warn': 'Early warning for virus-positive messages in quarantine.',
  'virus crit': 'Critical virus quarantine threshold indicating potential outbreak.',
  'growth warn %': 'Early warning when quarantine growth rate exceeds this percentage.',
  'growth warn min': 'Minimum new messages required before growth percentage triggers warning.',
  'growth crit %': 'Critical quarantine growth rate requiring immediate investigation.',
  'growth crit min': 'Minimum new messages required before growth percentage triggers critical alert.',
};

const OFFLINE_ALERTS_TOOLTIP =
  'Toggle default behavior for powered-off or connectivity alerts for this resource type.';

export interface Resource {
  id: string;
  name: string;
  displayName?: string;
  rawName?: string;
  node?: string;
  instance?: string;
  host?: string;
  type?: string;
  resourceType?: string;
  thresholds?: Record<string, number | undefined>;
  defaults?: Record<string, number | undefined>;
  disabled?: boolean;
  disableConnectivity?: boolean;
  poweredOffSeverity?: 'warning' | 'critical';
  hasOverride?: boolean;
  status?: string;
  vmid?: number;
  cpu?: number;
  memory?: number;
  uptime?: number;
  clusterName?: string;
  isClusterMember?: boolean;
  delaySeconds?: number;
  [key: string]: unknown;
}

export interface GroupHeaderMeta {
  type?: 'node' | 'default';
  displayName?: string;
  rawName?: string;
  host?: string;
  status?: string;
  clusterName?: string;
  isClusterMember?: boolean;
}

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
  showDelayColumn?: boolean;
  globalDelaySeconds?: number;
  editingId: () => string | null;
  editingThresholds: () => Record<string, number | undefined>;
  setEditingThresholds: (value: Record<string, number | undefined>) => void;
  formatMetricValue: (metric: string, value: number | undefined) => string;
  hasActiveAlert: (resourceId: string, metric: string) => boolean;
  globalDefaults?: Record<string, number | undefined>;
  setGlobalDefaults?: (value: Record<string, number | undefined> | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>)) => void;
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
}

type OfflineState = 'off' | 'warning' | 'critical';

export function ResourceTable(props: ResourceTableProps) {
  const flattenResources = (): Resource[] => {
    if (props.groupedResources) {
      return Object.values(props.groupedResources).flat();
    }
    return props.resources ?? [];
  };

  const hasRows = () => flattenResources().length > 0;

  const [activeMetricInput, setActiveMetricInput] = createSignal<{ resourceId: string; metric: string } | null>(null);
  const [showDelayRow, setShowDelayRow] = createSignal(false);

  // Check if global defaults have been customized from factory defaults
  const hasCustomGlobalDefaults = () => {
    if (!props.globalDefaults || !props.factoryDefaults) return false;
    return Object.keys(props.factoryDefaults).some(key => {
      const current = props.globalDefaults?.[key];
      const factory = props.factoryDefaults?.[key];
      return current !== undefined && current !== factory;
    });
  };

  const normalizeMetricKey = (column: string): string => {
    const key = column.trim().toLowerCase();
    const mapped = (
      new Map<string, string>([
        ['cpu %', 'cpu'],
        ['memory %', 'memory'],
        ['disk %', 'disk'],
        ['disk r mb/s', 'diskRead'],
        ['disk w mb/s', 'diskWrite'],
        ['net in mb/s', 'networkIn'],
        ['net out mb/s', 'networkOut'],
        ['usage %', 'usage'],
        ['temp °c', 'temperature'],
        ['temperature °c', 'temperature'],
        ['temperature', 'temperature'],
        ['restart count', 'restartCount'],
        ['restart window', 'restartWindow'],
        ['restart window (s)', 'restartWindow'],
        ['memory warn %', 'memoryWarnPct'],
        ['memory critical %', 'memoryCriticalPct'],
      ])
    ).get(key);
    if (mapped) {
      return mapped;
    }

    return key
      .replace(' %', '')
      .replace(' °c', '')
      .replace(' mb/s', '')
      .replace('disk r', 'diskRead')
      .replace('disk w', 'diskWrite')
      .replace('net in', 'networkIn')
      .replace('net out', 'networkOut');
  };

  const metricBounds = (metric: string): { min: number; max: number } => {
    if (metric === 'temperature') {
      return { min: -1, max: 150 };
    }
    if (['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(metric)) {
      return { min: -1, max: 10000 };
    }
    if (['cpu', 'memory', 'disk', 'usage', 'memoryWarnPct', 'memoryCriticalPct'].includes(metric)) {
      return { min: -1, max: 100 };
    }
    if (metric === 'restartCount') {
      return { min: -1, max: 50 };
    }
    if (metric === 'restartWindow') {
      return { min: -1, max: 86400 };
    }
    return { min: -1, max: 10000 };
  };

  const getEnabledDefaultValue = (metric: string): number => {
    if (['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(metric)) {
      return 100;
    }
    if (metric === 'temperature') {
      return 80;
    }
    if (metric === 'restartCount') {
      return 3;
    }
    if (metric === 'restartWindow') {
      return 300;
    }
    if (metric === 'memoryWarnPct') {
      return 90;
    }
    if (metric === 'memoryCriticalPct') {
      return 95;
    }
    return 80;
  };

  const metricDelayOverride = (metric: string): number | undefined => {
    const normalized = metric.trim().toLowerCase();
    const value = props.metricDelaySeconds?.[normalized] ?? props.metricDelaySeconds?.[metric];
    if (typeof value !== 'number' || !Number.isFinite(value)) {
      return undefined;
    }
    return value;
  };

  const totalColumnCount = () =>
    props.columns.length +
    3 +
    (props.showOfflineAlertsColumn ? 1 : 0);

  const getColumnHeaderTooltip = (column: string): string | undefined => {
    const normalized = column.trim().toLowerCase();
    return COLUMN_TOOLTIP_LOOKUP[column] ?? COLUMN_TOOLTIP_LOOKUP[normalized];
  };

  const resourceSupportsMetric = (resourceType: string | undefined, metric: string): boolean => {
    if (!resourceType) return true;
    if (
      resourceType === 'node' &&
      ['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(metric)
    ) {
      return false;
    }
    if (resourceType === 'pbs') {
      return ['cpu', 'memory'].includes(metric);
    }
    if (resourceType === 'storage') {
      return metric === 'usage';
    }
    if (resourceType === 'dockerContainer') {
      return [
        'cpu',
        'memory',
        'restartCount',
        'restartWindow',
        'memoryWarnPct',
        'memoryCriticalPct',
      ].includes(metric);
    }
    return true;
  };

  const renderGroupHeader = (groupKey: string, meta?: GroupHeaderMeta) => {
    if (!meta || meta.type !== 'node') {
      return <span class="text-xs font-medium text-gray-600 dark:text-gray-400">{groupKey}</span>;
    }

    return (
      <div class="flex flex-wrap items-center gap-3">
        <Show when={meta.host} fallback={
          <span class="text-sm font-medium text-gray-900 dark:text-gray-100">{meta.displayName || groupKey}</span>
        }>
          {(host) => (
            <a
              href={host() as string}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()}
              class="text-sm font-medium text-gray-900 dark:text-gray-100 transition-colors duration-150 hover:text-sky-600 dark:hover:text-sky-400"
              title={`Open ${meta.displayName || groupKey} web interface`}
            >
              {meta.displayName || groupKey}
            </a>
          )}
        </Show>
        <Show when={meta.clusterName}>
          <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
              {meta.clusterName}
            </span>
          </Show>
      </div>
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
      : props.formatMetricValue(metricProps.metric, metricProps.value);

    return (
      <div
        class={`flex items-center justify-center gap-1 ${isDisabledMetric ? 'opacity-60' : ''}`.trim()}
        title={isDisabledMetric ? 'Disabled (no alerts for this metric)' : ''}
      >
        <span
          class={`text-sm ${
            isDisabledMetric
              ? 'text-gray-400 dark:text-gray-500 italic'
              : metricProps.isOverridden
                ? 'text-gray-900 dark:text-gray-100 font-bold'
                : 'text-gray-900 dark:text-gray-100'
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
  }) => {
    return <StatusBadge {...config} />;
  };

  const offlineStateOrder: OfflineState[] = ['off', 'warning', 'critical'];

  const offlineStateConfig: Record<OfflineState, { label: string; className: string; title: string }> = {
    off: {
      label: 'Off',
      className:
        'bg-gray-200 text-gray-600 hover:bg-gray-300 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600',
      title: 'Offline alerts disabled for this resource.',
    },
    warning: {
      label: 'Warn',
      className:
        'bg-blue-50 text-blue-700 hover:bg-blue-100 dark:bg-blue-500/20 dark:text-blue-200 dark:hover:bg-blue-500/30',
      title: 'Offline alerts will raise warning-level notifications.',
    },
    critical: {
      label: 'Crit',
      className:
        'bg-red-50 text-red-700 hover:bg-red-100 dark:bg-red-500/20 dark:text-red-200 dark:hover:bg-red-500/30',
      title: 'Offline alerts will raise critical-level notifications.',
    },
  };

  const nextOfflineState = (state: OfflineState): OfflineState => {
    const idx = offlineStateOrder.indexOf(state);
    return offlineStateOrder[(idx + 1) % offlineStateOrder.length];
  };

  const renderOfflineStateButton = (state: OfflineState, disabled: boolean, onToggle: () => void) => {
    const config = offlineStateConfig[state];
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
    <Card
      padding="none"
      class="overflow-hidden border border-gray-200 dark:border-gray-700"
      border={false}
    >
      <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <SectionHeader title={props.title} size="sm" />
      </div>
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700">
              <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-16">
                Alerts
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Resource
              </th>
              <For each={props.columns}>
                {(column) => (
                  <th
                    class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
                    title={getColumnHeaderTooltip(column)}
                  >
                    {column}
                  </th>
                )}
              </For>
              <Show when={props.showOfflineAlertsColumn}>
                <th
                  class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
                  title={OFFLINE_ALERTS_TOOLTIP}
                >
                  Offline Alerts
                </th>
              </Show>
              <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
            {/* Global Defaults Row */}
            <Show when={props.globalDefaults && props.setGlobalDefaults && props.setHasUnsavedChanges}>
              <tr class={`bg-gray-50 dark:bg-gray-800/50 border-b border-gray-300 dark:border-gray-600 ${props.globalDisableFlag?.() ? 'opacity-40' : ''}`}>
                <td class="p-1 px-2 text-center align-middle">
                  <Show when={props.onToggleGlobalDisable} fallback={<span class="text-sm text-gray-400">-</span>}>
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
                </td>
                <td class="p-1 px-2">
                  <div class="flex items-center gap-2">
                    <span class="text-sm font-semibold text-gray-700 dark:text-gray-300">
                      Global Defaults
                    </span>
                    <Show when={hasCustomGlobalDefaults()}>
                      <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded">
                        Custom
                      </span>
                    </Show>
                  </div>
                </td>
                <For each={props.columns}>
                  {(column) => {
                    const metric = normalizeMetricKey(column);
                    const bounds = metricBounds(metric);
                    const val = () => props.globalDefaults?.[metric] ?? 0;
                    const isOff = () => val() === -1;

                    return (
                      <td class="p-1 px-2 text-center align-middle">
                        <div class="relative flex justify-center w-full">
                          <input
                            type="number"
                            min={bounds.min}
                            max={bounds.max}
                            value={isOff() ? '' : val()}
                            placeholder={isOff() ? 'Off' : ''}
                            disabled={isOff()}
                            onInput={(e) => {
                              const value = parseInt(e.currentTarget.value, 10);
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
                                ? 'border-gray-300 dark:border-gray-600 bg-gray-100 dark:bg-gray-800 text-gray-400 dark:text-gray-500 italic placeholder:text-gray-400 dark:placeholder:text-gray-500 placeholder:opacity-60 pointer-events-none'
                                : 'border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:border-blue-500 focus:ring-1 focus:ring-blue-500'
                            }`}
                            title={isOff() ? 'Click to enable this metric' : 'Set to -1 to disable alerts for this metric'}
                          />
                          <Show when={isOff()}>
                            <button
                              type="button"
                              class="absolute inset-0 w-full rounded cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
                              onClick={() => {
                                if (!props.setGlobalDefaults) return;
                                props.setGlobalDefaults((prev) => ({
                                  ...prev,
                                  [metric]: getEnabledDefaultValue(metric),
                                }));
                                props.setHasUnsavedChanges?.(true);
                              }}
                              title="Click to enable this metric"
                            >
                              <span class="sr-only">Enable {column} default</span>
                            </button>
                          </Show>
                        </div>
                      </td>
                    );
                  }}
                </For>
                <Show when={props.showOfflineAlertsColumn}>
                  <td class="p-1 px-2 text-center align-middle">
                    <Show when={props.onSetGlobalOfflineState} fallback={
                      <Show when={props.onToggleGlobalDisableOffline} fallback={<span class="text-sm text-gray-400">-</span>}>
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
                            titleEnabled: 'Offline alerts currently enabled by default. Click to disable.',
                            titleDisabled: 'Offline alerts currently disabled by default. Click to enable.',
                          });
                        })()}
                      </Show>
                    }>
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
                  </td>
                </Show>
                <td class="p-1 px-2 text-center align-middle">
                  <div class="flex items-center justify-center gap-1">
                    <Show when={props.showDelayColumn && typeof props.onMetricDelayChange === 'function'}>
                      <button
                        type="button"
                        onClick={() => setShowDelayRow(!showDelayRow())}
                        class="p-1 text-gray-600 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300 transition-colors"
                        title={showDelayRow() ? 'Hide alert delay settings' : 'Show alert delay settings'}
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
                        title="Reset to factory defaults"
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
                      <span class="text-sm text-gray-400">-</span>
                    </Show>
                  </div>
                </td>
              </tr>
            </Show>
            <Show when={showDelayRow() && props.showDelayColumn && typeof props.onMetricDelayChange === 'function'}>
              <tr class={`bg-gray-50 dark:bg-gray-800/50 border-b border-gray-300 dark:border-gray-600 ${props.globalDisableFlag?.() ? 'opacity-40' : ''}`}>
                <td class="p-1 px-2 text-center align-middle">
                  <span class="text-sm text-gray-400">-</span>
                </td>
                <td class="p-1 px-2 align-middle">
                  <span class="text-xs font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-300">
                    Alert Delay (s)
                  </span>
                </td>
                <For each={props.columns}>
                  {(column) => {
                    const metric = normalizeMetricKey(column);
                    const typeDefaultDelay = props.globalDelaySeconds ?? 5;
                    const overrideDelay = metricDelayOverride(metric);

                    return (
                      <td class="p-1 px-2 text-center align-middle">
                        <div class="relative flex justify-center w-full">
                          <input
                            type="number"
                            min="0"
                            value={(() => {
                              return overrideDelay !== undefined ? overrideDelay : '';
                            })()}
                            placeholder={String(typeDefaultDelay)}
                            class="w-16 rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-2 py-0.5 text-sm text-center text-gray-900 dark:text-gray-100 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
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
                      </td>
                    );
                  }}
                </For>
                <Show when={props.showOfflineAlertsColumn}>
                  <td class="p-1 px-2 text-center align-middle">
                    <span class="text-sm text-gray-400">-</span>
                  </td>
                </Show>
                <td class="p-1 px-2 text-center align-middle">
                  <span class="text-sm text-gray-400">-</span>
                </td>
              </tr>
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
                      <tr class="bg-gray-50 dark:bg-gray-700/50">
                        <td
                          colspan={totalColumnCount()}
                          class="p-1 px-2 text-xs font-medium text-gray-600 dark:text-gray-400"
                        >
                          {renderGroupHeader(nodeName, headerMeta)}
                        </td>
                      </tr>
                      {/* Resources in this group */}
                      <For each={resources}>
                        {(resource) => {
                        const isEditing = () => props.editingId() === resource.id;
                        const thresholds = (): Record<string, number | undefined> => {
                          if (isEditing()) {
                            return props.editingThresholds();
                          }
                          return resource.thresholds ?? {};
                        };
                        const displayValue = (metric: string): number => {
                          const parseNumeric = (value: unknown): number | undefined => {
                            if (value === undefined || value === null) return undefined;
                            if (typeof value === 'number') return value;
                            const parsed = Number(value);
                            return Number.isFinite(parsed) ? parsed : undefined;
                          };

                          const extract = (source: Record<string, unknown> | undefined) =>
                            parseNumeric(source?.[metric]);

                          const defaults = resource.defaults as Record<string, unknown> | undefined;

                          if (isEditing()) {
                            const edited = extract(thresholds() as Record<string, unknown>);
                            if (edited !== undefined) {
                              return edited;
                            }
                            const fallback = extract(defaults);
                            return fallback !== undefined ? fallback : 0;
                          }

                          const liveValue = extract(resource.thresholds as Record<string, unknown> | undefined);
                          if (liveValue !== undefined) {
                            return liveValue;
                          }

                          const fallback = extract(defaults);
                          return fallback !== undefined ? fallback : 0;
                        };
                        const isOverridden = (metric: string) => {
                          return (
                            resource.thresholds?.[metric] !== undefined &&
                            resource.thresholds?.[metric] !== null
                          );
                        };

                        return (
                          <tr
                            class={`hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-colors ${resource.disabled || props.globalDisableFlag?.() ? 'opacity-40' : ''}`}
                          >
                            {/* Alert toggle column */}
                            <td class="p-1 px-2 text-center align-middle">
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
                                        onToggle={() => !globallyDisabled && props.onToggleDisabled?.(resource.id)}
                                        class="my-[1px]"
                                        title={
                                          globallyDisabled
                                            ? 'Alerts disabled globally'
                                            : resource.disabled
                                              ? 'Click to enable alerts'
                                              : 'Click to disable alerts'
                                        }
                                        ariaLabel={isChecked ? 'Alerts enabled for this resource' : 'Alerts disabled for this resource'}
                                      />
                                    </div>
                                  );
                                })()}
                              </Show>
                            </td>
                          <td class="p-1 px-2">
                            <Show when={resource.type === 'node'} fallback={
                              <div class="flex items-center gap-2">
                                <span
                                  class={`text-sm font-medium ${resource.disabled ? 'text-gray-500 dark:text-gray-500' : 'text-gray-900 dark:text-gray-100'}`}
                                >
                                  {resource.name}
                                </span>
                                <Show when={resource.hasOverride || resource.disableConnectivity}>
                                  <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded">
                                    Custom
                                  </span>
                                </Show>
                              </div>
                            }>
                              <div class="flex flex-wrap items-center gap-3" title={resource.status || undefined}>
                                <Show when={resource.host} fallback={
                                  <span
                                    class={`text-sm font-medium ${resource.disabled ? 'text-gray-500 dark:text-gray-500' : 'text-gray-900 dark:text-gray-100'}`}
                                  >
                                    {resource.type === 'node'
                                      ? resource.name
                                      : resource.displayName || resource.name}
                                  </span>
                                }>
                                  {(host) => (
                                    <a
                                      href={host() as string}
                                      target="_blank"
                                      rel="noopener noreferrer"
                                      onClick={(e) => e.stopPropagation()}
                                      class={`text-sm font-medium transition-colors duration-150 ${
                                        resource.disabled
                                          ? 'text-gray-500 dark:text-gray-500'
                                          : 'text-gray-900 dark:text-gray-100 hover:text-sky-600 dark:hover:text-sky-400'
                                      }`}
                                      title={`Open ${resource.displayName || resource.name} web interface`}
                                    >
                                      {resource.type === 'node'
                                        ? resource.name
                                        : resource.displayName || resource.name}
                                    </a>
                                  )}
                                </Show>
                                <Show when={resource.clusterName}>
                                  <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
                                    {resource.clusterName}
                                  </span>
                                </Show>
                                <Show when={resource.hasOverride || resource.disableConnectivity}>
                                  <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded">
                                    Custom
                                  </span>
                                </Show>
                              </div>
                            </Show>
                          </td>
                          {/* Metric columns - dynamically rendered based on resource type */}
                            <For each={props.columns}>
                              {(column) => {
                                const metric = normalizeMetricKey(column);
                                const showMetric = () => resourceSupportsMetric(resource.type, metric);
                                const bounds = metricBounds(metric);
                                const isDisabled = () => thresholds()?.[metric] === -1;

                                const openMetricEditor = (e: MouseEvent) => {
                                  e.stopPropagation();
                                  setActiveMetricInput({ resourceId: resource.id, metric });
                                  props.onEdit(
                                    resource.id,
                                    resource.thresholds ? { ...resource.thresholds } : {},
                                    resource.defaults ? { ...resource.defaults } : {},
                                  );
                                };

                                return (
                  <td class="p-1 px-2 text-center align-middle">
                                    <Show
                                      when={showMetric()}
                                      fallback={
                                        <span class="text-sm text-gray-400 dark:text-gray-500">
                                          -
                                        </span>
                                      }
                                    >
                                        <Show
                                          when={isEditing()}
                                          fallback={
                                            <div
                                              onClick={(event) => {
                                                openMetricEditor(event);
                                              }}
                                              class="cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-800 rounded px-1 py-0.5 transition-colors"
                                              title="Click to edit this metric"
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
                                              min={bounds.min}
                                              max={bounds.max}
                                              value={thresholds()?.[metric] ?? ''}
                                              placeholder={isDisabled() ? 'Off' : ''}
                                              title="Set to -1 to disable alerts for this metric"
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
                                                  props.setEditingThresholds({
                                                    ...props.editingThresholds(),
                                                    [metric]: undefined,
                                                  });
                                                  return;
                                                }
                                                const val = parseInt(raw, 10);
                                                if (!Number.isNaN(val)) {
                                                  props.setEditingThresholds({
                                                    ...props.editingThresholds(),
                                                    [metric]: val,
                                                  });
                                                }
                                              }}
                                              onBlur={() => {
                                                if (props.editingId() === resource.id) {
                                                  props.onSaveEdit(resource.id);
                                                }
                                                setActiveMetricInput(null);
                                              }}
                                              class={`w-16 px-2 py-0.5 text-sm text-center border rounded ${
                                                isDisabled()
                                                  ? 'bg-gray-100 dark:bg-gray-800 text-gray-400 dark:text-gray-600 border-gray-300 dark:border-gray-600'
                                                  : 'bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 border-gray-300 dark:border-gray-600'
                                              }`}
                                            />
                                          </div>
                                        </Show>
                                     </Show>
                                  </td>
                                );
                              }}
                            </For>

                            {/* Offline Alerts column - Connectivity/powered-off alerts */}
                            <Show when={props.showOfflineAlertsColumn}>
                              <td class="p-1 px-2 text-center align-middle">
                                {(() => {
                                  const disabledGlobally = props.globalDisableFlag?.() ?? false;
                                  const supportsTriState =
                                    typeof props.onSetOfflineState === 'function' &&
                                    (resource.type === 'guest' || resource.type === 'dockerContainer');

                                  if (supportsTriState) {
                                    const defaultDisabled = props.globalDisableOfflineFlag?.() ?? false;
                                    const defaultSeverity = props.globalOfflineSeverity ?? 'warning';

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

                                    return renderOfflineStateButton(state, disabledGlobally, () => {
                                      if (disabledGlobally) return;
                                      const next = nextOfflineState(state);
                                      props.onSetOfflineState?.(resource.id, next);
                                    });
                                  }

                                  if (!props.onToggleNodeConnectivity) {
                                    return <span class="text-sm text-gray-400">-</span>;
                                  }

                                  const globalOfflineDisabled = props.globalDisableOfflineFlag?.() ?? false;
                                  return renderToggleBadge({
                                    isEnabled: !globalOfflineDisabled && !resource.disableConnectivity,
                                    disabled: disabledGlobally,
                                    onToggle: () => {
                                      if (disabledGlobally) return;
                                      props.onToggleNodeConnectivity?.(resource.id);
                                    },
                                    titleEnabled: 'Offline alerts enabled. Click to disable for this resource.',
                                    titleDisabled: 'Offline alerts disabled. Click to enable for this resource.',
                                    titleWhenDisabled: 'Offline alerts controlled globally',
                                  });
                                })()}
                              </td>
                            </Show>

                            {/* Actions column */}
                            <td class="p-1 px-2">
                              <div class="flex items-center justify-center gap-1">
                                <Show
                                  when={!isEditing()}
                                  fallback={
                                    <button
                                      type="button"
                                      onClick={() => {
                                        props.onCancelEdit();
                                        setActiveMetricInput(null);
                                      }}
                                      class="p-1 text-gray-600 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
                                      title="Cancel editing"
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
                                      onClick={() =>
                                        props.onEdit(
                                          resource.id,
                                          resource.thresholds ? { ...resource.thresholds } : {},
                                          resource.defaults ? { ...resource.defaults } : {},
                                        )
                                      }
                                      class="p-1 text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                                      title="Edit thresholds"
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
                                      ((resource.type === 'node' || resource.type === 'dockerHost') &&
                                        resource.disableConnectivity)
                                    }
                                  >
                                    <button
                                      type="button"
                                      onClick={() => props.onRemoveOverride(resource.id)}
                                      class="p-1 text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
                                      title="Remove override"
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
                                </Show>
                              </div>
                            </td>
                          </tr>
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
                      const thresholds = (): Record<string, number | undefined> => {
                        if (isEditing()) {
                          return props.editingThresholds();
                        }
                        return resource.thresholds ?? {};
                      };
                      const displayValue = (metric: string): number => {
                        const parseNumeric = (value: unknown): number | undefined => {
                          if (value === undefined || value === null) return undefined;
                          if (typeof value === 'number') return value;
                          const parsed = Number(value);
                          return Number.isFinite(parsed) ? parsed : undefined;
                        };

                        const extract = (source: Record<string, unknown> | undefined) =>
                          parseNumeric(source?.[metric]);

                        const defaults = resource.defaults as Record<string, unknown> | undefined;

                        if (isEditing()) {
                          const edited = extract(thresholds() as Record<string, unknown>);
                          if (edited !== undefined) {
                            return edited;
                          }
                          const fallback = extract(defaults);
                          return fallback !== undefined ? fallback : 0;
                        }

                        const liveValue = extract(resource.thresholds as Record<string, unknown> | undefined);
                        if (liveValue !== undefined) {
                          return liveValue;
                        }

                        const fallback = extract(defaults);
                        return fallback !== undefined ? fallback : 0;
                      };
                      const isOverridden = (metric: string) => {
                        return (
                          resource.thresholds?.[metric] !== undefined &&
                          resource.thresholds?.[metric] !== null
                        );
                      };

                      return (
                        <tr
                          class={`hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-colors ${resource.disabled || props.globalDisableFlag?.() ? 'opacity-40' : ''}`}
                        >
                          {/* Alert toggle column */}
                          <td class="p-1 px-2 text-center align-middle">
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
                                        onToggle={() => !globallyDisabled && props.onToggleDisabled?.(resource.id)}
                                        class="my-[1px]"
                                        title={
                                          globallyDisabled
                                            ? 'Alerts disabled globally'
                                            : resource.disabled
                                              ? 'Click to enable alerts'
                                              : 'Click to disable alerts'
                                        }
                                        ariaLabel={isChecked ? 'Alerts enabled for this resource' : 'Alerts disabled for this resource'}
                                      />
                                    </div>
                                  );
                                })()}
                              </Show>
                          </td>
                          <td class="p-1 px-2">
                            <Show when={resource.type === 'node'} fallback={
                              <div class="flex items-center gap-2">
                                <span
                                  class={`text-sm font-medium ${resource.disabled ? 'text-gray-500 dark:text-gray-500' : 'text-gray-900 dark:text-gray-100'}`}
                                >
                                  {resource.name}
                                </span>
                                <Show when={resource.hasOverride || resource.disableConnectivity}>
                                  <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded">
                                    Custom
                                  </span>
                                </Show>
                              </div>
                            }>
                              <div class="flex flex-wrap items-center gap-3" title={resource.status || undefined}>
                                <Show when={resource.host} fallback={
                                  <span
                                    class={`text-sm font-medium ${resource.disabled ? 'text-gray-500 dark:text-gray-500' : 'text-gray-900 dark:text-gray-100'}`}
                                  >
                                    {resource.type === 'node'
                                      ? resource.name
                                      : resource.displayName || resource.name}
                                  </span>
                                }>
                                  {(host) => (
                                    <a
                                      href={host() as string}
                                      target="_blank"
                                      rel="noopener noreferrer"
                                      onClick={(e) => e.stopPropagation()}
                                      class={`text-sm font-medium transition-colors duration-150 ${
                                        resource.disabled
                                          ? 'text-gray-500 dark:text-gray-500'
                                          : 'text-gray-900 dark:text-gray-100 hover:text-sky-600 dark:hover:text-sky-400'
                                      }`}
                                      title={`Open ${resource.displayName || resource.name} web interface`}
                                    >
                                      {resource.type === 'node'
                                        ? resource.name
                                        : resource.displayName || resource.name}
                                    </a>
                                  )}
                                </Show>
                                <Show when={resource.clusterName}>
                                  <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
                                    {resource.clusterName}
                                  </span>
                                </Show>
                                <Show when={resource.hasOverride || resource.disableConnectivity}>
                                  <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded">
                                    Custom
                                  </span>
                                </Show>
                              </div>
                            </Show>
                          </td>
                          {/* Metric columns - dynamically rendered based on resource type */}
                          <For each={props.columns}>
                            {(column) => {
                              const metric = column
                                .toLowerCase()
                                .replace(' %', '')
                                .replace(' mb/s', '')
                                .replace('disk r', 'diskRead')
                                .replace('disk w', 'diskWrite')
                                .replace('net in', 'networkIn')
                                .replace('net out', 'networkOut');

                              // Check if this metric applies to this resource type
                              const showMetric = () => {
                                if (
                                  resource.type === 'node' &&
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
                                e.stopPropagation();
                                setActiveMetricInput({ resourceId: resource.id, metric });
                                props.onEdit(
                                  resource.id,
                                  resource.thresholds ? { ...resource.thresholds } : {},
                                  resource.defaults ? { ...resource.defaults } : {},
                                );
                              };

                              return (
                                <td class="p-1 px-2 text-center align-middle">
                                  <Show
                                    when={showMetric()}
                                    fallback={
                                      <span class="text-sm text-gray-400 dark:text-gray-500">
                                        -
                                      </span>
                                    }
                                  >
                                    <Show
                                      when={isEditing()}
                                      fallback={
                                        <div
                                          onClick={openMetricEditor}
                                          class="cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-800 rounded px-1 py-0.5 transition-colors"
                                          title="Click to edit this metric"
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
                                          value={thresholds()?.[metric] ?? ''}
                                          placeholder={isDisabled() ? 'Off' : ''}
                                          title="Set to -1 to disable alerts for this metric"
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
                                              props.setEditingThresholds({
                                                ...props.editingThresholds(),
                                                [metric]: undefined,
                                              });
                                              return;
                                            }
                                            const val = parseInt(raw, 10);
                                            if (!Number.isNaN(val)) {
                                              props.setEditingThresholds({
                                                ...props.editingThresholds(),
                                                [metric]: val,
                                              });
                                            }
                                          }}
                                          onBlur={() => {
                                            if (props.editingId() === resource.id) {
                                              props.onSaveEdit(resource.id);
                                            }
                                            setActiveMetricInput(null);
                                          }}
                                          class={`w-16 px-2 py-0.5 text-sm text-center border rounded ${
                                            isDisabled()
                                              ? 'bg-gray-100 dark:bg-gray-800 text-gray-400 dark:text-gray-600 border-gray-300 dark:border-gray-600'
                                              : 'bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 border-gray-300 dark:border-gray-600'
                                          }`}
                                        />
                                      </div>
                                    </Show>
                                  </Show>
                                </td>
                              );
                            }}
                          </For>

                          {/* Offline Alerts column - Connectivity/powered-off alerts */}
                          <Show when={props.showOfflineAlertsColumn}>
                            <td class="p-1 px-2 text-center align-middle">
                              <Show when={props.onToggleNodeConnectivity}>
                                {(() => {
                                  const defaultOfflineDisabled = props.globalDisableOfflineFlag?.() ?? false;
                                  const isEnabled = !(resource.disableConnectivity || defaultOfflineDisabled);
                                  const disabledGlobally = props.globalDisableFlag?.() ?? false;
                                  return (
                                    <StatusBadge
                                      isEnabled={isEnabled}
                                      disabled={disabledGlobally}
                                      onToggle={() => props.onToggleNodeConnectivity?.(resource.id)}
                                      titleEnabled="Offline alerts enabled. Click to disable for this resource."
                                      titleDisabled="Offline alerts disabled. Click to enable for this resource."
                                      titleWhenDisabled="Offline alerts controlled globally"
                                    />
                                  );
                                })()}
                            </Show>
                          </td>
                        </Show>

                        {/* Actions column */}
                        <td class="p-1 px-2">
                          <div class="flex items-center justify-center gap-1">
                            <Show
                                when={!isEditing()}
                                fallback={
                                  <button
                                    type="button"
                                    onClick={() => {
                                      props.onCancelEdit();
                                      setActiveMetricInput(null);
                                    }}
                                    class="p-1 text-gray-600 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
                                    title="Cancel editing"
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
                                <button
                                  type="button"
                                  onClick={() =>
                                    props.onEdit(
                                      resource.id,
                                      resource.thresholds ? { ...resource.thresholds } : {},
                                      resource.defaults ? { ...resource.defaults } : {},
                                    )
                                  }
                                  class="p-1 text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                                  title="Edit thresholds"
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
                                    (resource.type === 'node' && resource.disableConnectivity)
                                  }
                                >
                                  <button
                                    type="button"
                                    onClick={() => props.onRemoveOverride(resource.id)}
                                    class="p-1 text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
                                    title="Remove override"
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
                              </Show>
                            </div>
                          </td>
                        </tr>
                      );
                    }}
                  </For>
                }
              >
                <tr>
                  <td
                    colspan={totalColumnCount()}
                    class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400"
                  >
                    No {props.title.toLowerCase()} found
                  </td>
                </tr>
              </Show>
            </Show>
            <Show when={!hasRows()}>
              <tr>
                <td
                  colspan={totalColumnCount()}
                  class="px-4 py-6 text-sm text-center text-gray-500 dark:text-gray-400"
                >
                  {props.emptyMessage || 'No resources available.'}
                </td>
              </tr>
            </Show>
          </tbody>
        </table>
      </div>
    </Card>
  );
}
