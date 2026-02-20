import { For, Show, createSignal, createEffect } from 'solid-js';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { TogglePrimitive } from '@/components/shared/Toggle';
import { StatusBadge } from '@/components/shared/StatusBadge';
import type { Alert } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { ThresholdSlider } from '@/components/Dashboard/ThresholdSlider';
import { HelpIcon } from '@/components/shared/HelpIcon';
import { logger } from '@/utils/logger';

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
  'disk temp °c': 'Individual disk temperature threshold for host agents.',
  'restart count': 'Maximum container restarts within the evaluation window.',
  'restart window': 'Time window used to evaluate the restart count threshold.',
  'restart window (s)': 'Time window used to evaluate the restart count threshold.',
  'memory warn %': 'Warning threshold for container memory usage.',
  'memory critical %': 'Critical threshold for container memory usage.',
  // PMG (Proxmox Mail Gateway) thresholds
  'queue warn': 'Early warning when total mail queue exceeds this message count.',
  'queue crit': 'Critical alert requiring urgent action when queue reaches this size.',
  'deferred warn':
    'Early warning for messages stuck in deferred queue (waiting to retry delivery).',
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
  'growth crit min':
    'Minimum new messages required before growth percentage triggers critical alert.',
  'warning size (gib)': 'Total snapshot size in GiB that raises a warning.',
  'critical size (gib)': 'Total snapshot size in GiB that raises a critical alert.',
};

const OFFLINE_ALERTS_TOOLTIP =
  'Toggle default behavior for powered-off or connectivity alerts for this resource type.';

const SLIDER_METRICS = new Set(['cpu', 'memory', 'disk', 'temperature', 'diskTemperature']);

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
  subtitle?: string;
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
  editScope?: 'snapshot' | 'backup';
  isEnabled?: boolean;
  toggleEnabled?: () => void;
  toggleTitleEnabled?: string;
  toggleTitleDisabled?: string;
  editable?: boolean;
  note?: string;
  backup?: any;
  snapshot?: any;
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
}

type OfflineState = 'off' | 'warning' | 'critical';

export function ResourceTable(props: ResourceTableProps) {
  const { isMobile } = useBreakpoint();

  const flattenResources = (): Resource[] => {
    if (props.groupedResources) {
      return Object.values(props.groupedResources).flat();
    }
    return props.resources ?? [];
  };

  const hasRows = () => {
    if (flattenResources().length > 0) {
      return true;
    }
    if (props.groupedResources && Object.keys(props.groupedResources).length > 0) {
      return true;
    }
    return Boolean(props.globalDefaults);
  };

  const [activeMetricInput, setActiveMetricInput] = createSignal<{
    resourceId: string;
    metric: string;
  } | null>(null);
  const [showDelayRow, setShowDelayRow] = createSignal(false);

  // Track changes to global defaults and factory defaults for debugging
  createEffect(() => {
    logger.debug('[ResourceTable] props changed', {
      title: props.title,
      globalDefaults: props.globalDefaults,
      factoryDefaults: props.factoryDefaults,
      onResetDefaults: !!props.onResetDefaults,
    });
  });

  // Check if global defaults have been customized from factory defaults
  const hasCustomGlobalDefaults = () => {
    logger.debug('[ResourceTable] hasCustomGlobalDefaults check', {
      globalDefaults: props.globalDefaults,
      factoryDefaults: props.factoryDefaults,
      title: props.title,
    });
    if (!props.globalDefaults || !props.factoryDefaults) {
      logger.debug('[ResourceTable] Missing props, returning false');
      return false;
    }
    const result = Object.keys(props.factoryDefaults).some((key) => {
      const current = props.globalDefaults?.[key];
      const factory = props.factoryDefaults?.[key];
      const differs = current !== undefined && current !== factory;
      if (differs) {
        logger.debug('[ResourceTable] Difference found', {
          key,
          current,
          factory,
        });
      }
      return differs;
    });
    logger.debug('[ResourceTable] hasCustomGlobalDefaults result', { result });
    return result;
  };

  const normalizeMetricKey = (column: string): string => {
    const key = column.trim().toLowerCase();
    const mapped = new Map<string, string>([
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
      ['warning size (gib)', 'warningSizeGiB'],
      ['critical size (gib)', 'criticalSizeGiB'],
      ['disk temp °c', 'diskTemperature'],
      ['backup', 'backup'],
      ['snapshot', 'snapshot'],
    ]).get(key);
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
    if (metric === 'temperature' || metric === 'diskTemperature') {
      return { min: -1, max: 150 };
    }
    if (['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(metric)) {
      return { min: -1, max: 10000 };
    }
    if (['cpu', 'memory', 'disk', 'usage', 'memoryWarnPct', 'memoryCriticalPct'].includes(metric)) {
      return { min: -1, max: 100 };
    }
    if (['warningSizeGiB', 'criticalSizeGiB'].includes(metric)) {
      return { min: -1, max: 100000 };
    }
    if (metric === 'restartCount') {
      return { min: -1, max: 50 };
    }
    if (metric === 'restartWindow') {
      return { min: -1, max: 86400 };
    }
    return { min: -1, max: 10000 };
  };

  const metricStep = (metric: string): string | number => {
    if (['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(metric)) {
      return 'any';
    }
    if (['warningSizeGiB', 'criticalSizeGiB'].includes(metric)) {
      return 'any';
    }
    return 1;
  };

  const getEnabledDefaultValue = (metric: string): number => {
    if (['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(metric)) {
      return 100;
    }
    if (metric === 'temperature') {
      return 80;
    }
    if (metric === 'diskTemperature') {
      return 55;
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

  const totalColumnCount = () => props.columns.length + 3 + (props.showOfflineAlertsColumn ? 1 : 0);

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
      return <span class="text-xs font-medium text-slate-600 dark:text-slate-400">{groupKey}</span>;
    }

    return (
      <div class="flex flex-wrap items-center gap-3">
        <Show
          when={meta.host}
          fallback={
            <span class="text-sm font-medium text-slate-900 dark:text-slate-100">
              {meta.displayName || groupKey}
            </span>
          }
        >
          {(host) => (
            <a
              href={host() as string}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()}
              class="text-sm font-medium text-slate-900 dark:text-slate-100 transition-colors duration-150 hover:text-sky-600 dark:hover:text-sky-400"
              title={`Open ${meta.displayName || groupKey} web interface`}
            >
              {meta.displayName || groupKey}
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
          class={`text-sm ${isDisabledMetric
            ? 'text-slate-400 dark:text-slate-500 italic'
            : metricProps.isOverridden
              ? 'text-slate-900 dark:text-slate-100 font-bold'
              : 'text-slate-900 dark:text-slate-100'
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

  const offlineStateOrder: OfflineState[] = ['off', 'warning', 'critical'];

  const offlineStateConfig: Record<
    OfflineState,
    { label: string; className: string; title: string }
  > = {
    off: {
      label: 'Off',
      className:
        'bg-slate-200 text-slate-600 hover:bg-slate-300 dark:bg-slate-700 dark:text-slate-300 dark:hover:bg-slate-600',
      title: 'Offline alerts disabled for this resource.',
    },
    warning: {
      label: 'Warn',
      className:
        'bg-blue-50 text-blue-700 hover:bg-blue-100 dark:bg-blue-900 dark:text-blue-200 dark:hover:bg-blue-800',
      title: 'Offline alerts will raise warning-level notifications.',
    },
    critical: {
      label: 'Crit',
      className:
        'bg-red-50 text-red-700 hover:bg-red-100 dark:bg-red-900 dark:text-red-200 dark:hover:bg-red-800',
      title: 'Offline alerts will raise critical-level notifications.',
    },
  };

  const nextOfflineState = (state: OfflineState): OfflineState => {
    const idx = offlineStateOrder.indexOf(state);
    return offlineStateOrder[(idx + 1) % offlineStateOrder.length];
  };

  const renderOfflineStateButton = (
    state: OfflineState,
    disabled: boolean,
    onToggle: () => void,
  ) => {
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

  const renderMobileView = () => (
    <div class="space-y-4">
      {/* Global Defaults - Mobile Card */}
      <Show when={props.globalDefaults && props.setGlobalDefaults && props.setHasUnsavedChanges}>
        <Card padding="sm" class="border border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900">
          <div class="flex justify-between items-center mb-3">
            <div class="flex items-center gap-2">
              <span class="font-semibold text-sm">Global Defaults</span>
              <Show when={hasCustomGlobalDefaults()}>
                <span class="text-[10px] px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Custom</span>
              </Show>
            </div>
            <Show when={props.onToggleGlobalDisable}>
              <TogglePrimitive size="sm" checked={!props.globalDisableFlag?.()} onToggle={() => { props.onToggleGlobalDisable?.(); props.setHasUnsavedChanges?.(true); }} />
            </Show>
          </div>

          <div class="grid grid-cols-2 gap-2">
            <For each={props.columns.filter((c) => { const m = normalizeMetricKey(c); return m !== 'backup' && m !== 'snapshot'; })}>
              {(column) => {
                const metric = normalizeMetricKey(column);
                const bounds = metricBounds(metric);
                const val = () => props.globalDefaults?.[metric] ?? 0;
                const isOff = () => val() === -1;
                return (
                  <div class="p-2 bg-white dark:bg-slate-800 rounded border border-slate-100 dark:border-slate-700 flex flex-col gap-1">
                    <span class="text-[10px] uppercase text-slate-500 font-medium">{column}</span>
                    <div class="relative">
                      <input type="number" min={bounds.min} max={bounds.max} step={metricStep(metric)}
                        value={isOff() ? '' : val()}
                        placeholder={isOff() ? 'Off' : ''}
                        disabled={isOff()}
                        class={`w-full text-sm p-1 rounded border text-center ${isOff() ? 'bg-slate-100 dark:bg-slate-700' : 'bg-white dark:bg-slate-600 border-slate-200 dark:border-slate-500'}`}
                        onInput={(e) => {
                          const value = parseFloat(e.currentTarget.value);
                          if (props.setGlobalDefaults) {
                            props.setGlobalDefaults((prev) => ({ ...prev, [metric]: Number.isNaN(value) ? 0 : value }));
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
                            props.setGlobalDefaults((prev) => ({ ...prev, [metric]: getEnabledDefaultValue(metric) }));
                            props.setHasUnsavedChanges?.(true);
                          }}
                          aria-label={`Enable ${column} default`}
                        ></button>
                      </Show>
                    </div>
                  </div>
                )
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
              <div class="px-1 font-medium text-xs text-slate-500 uppercase mt-4 mb-1">{renderGroupHeader(groupName, props.groupHeaderMeta?.[groupName])}</div>
            </Show>
            <For each={resources as Resource[]}>
              {(resource) => {
                const isEditing = () => props.editingId() === resource.id;
                const thresholds = () => isEditing() ? props.editingThresholds() : (resource.thresholds ?? {});

                // Helper logic duplicated/adapted for scope access
                const displayValue = (metric: string): number => {
                  const parseNumeric = (value: unknown): number | undefined => {
                    if (value === undefined || value === null) return undefined;
                    if (typeof value === 'number') return value;
                    const parsed = Number(value);
                    return Number.isFinite(parsed) ? parsed : undefined;
                  };
                  const extract = (source: Record<string, unknown> | undefined) => parseNumeric(source?.[metric]);
                  const defaults = resource.defaults as Record<string, unknown> | undefined;

                  if (isEditing()) {
                    const edited = extract(thresholds() as Record<string, unknown>);
                    if (edited !== undefined) return edited;
                    const fallback = extract(defaults);
                    return fallback !== undefined ? fallback : 0;
                  }

                  const liveValue = extract(resource.thresholds as Record<string, unknown> | undefined);
                  if (liveValue !== undefined) return liveValue;
                  const fallback = extract(defaults);
                  return fallback !== undefined ? fallback : 0;
                };

                const isOverridden = (metric: string) => resource.thresholds?.[metric] !== undefined && resource.thresholds?.[metric] !== null;

                return (
                  <Card padding="sm" class={`flex flex-col gap-3 transition-opacity ${resource.disabled || props.globalDisableFlag?.() ? 'opacity-60' : ''}`}>
                    {/* Header */}
                    <div class="flex items-center justify-between">
                      <div class="flex items-center gap-3 min-w-0">
                        {/* Toggle */}
                        <Show when={props.onToggleDisabled}>
                          <div class="shrink-0 scale-90 origin-left">
                            <TogglePrimitive
                              size="sm"
                              checked={!(props.globalDisableFlag?.() ?? false) && !resource.disabled}
                              disabled={props.globalDisableFlag?.() ?? false}
                              onToggle={() => !(props.globalDisableFlag?.() ?? false) && props.onToggleDisabled?.(resource.id)}
                            />
                          </div>
                        </Show>

                        {/* Name */}
                        <div class="min-w-0 truncate">
                          <div class="font-medium text-sm truncate">{resource.displayName || resource.name}</div>
                          <Show when={resource.subtitle}><div class="text-xs text-slate-500 truncate">{resource.subtitle}</div></Show>
                        </div>
                      </div>

                      {/* Actions */}
                      <div class="flex gap-1 shrink-0">
                        <Show when={!isEditing() && resource.type !== 'dockerHost'}>
                          <button
                            type="button"
                            onClick={() => props.onEdit(resource.id, resource.thresholds ? { ...resource.thresholds } : {}, resource.defaults ? { ...resource.defaults } : {}, resource.note)}
                            class="p-1.5 bg-blue-50 dark:bg-blue-900 text-blue-600 rounded"
                            aria-label={`Edit thresholds for ${resource.displayName || resource.name}`}
                          >
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" /></svg>
                          </button>
                        </Show>
                        <Show when={isEditing()}>
                          <button
                            type="button"
                            onClick={() => { props.onCancelEdit(); setActiveMetricInput(null); }}
                            class="p-1.5 bg-slate-100 dark:bg-slate-700 text-slate-600 rounded"
                            aria-label="Cancel threshold edits"
                          >
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
                          </button>
                          <button
                            type="button"
                            onClick={() => { props.onSaveEdit(resource.id); setActiveMetricInput(null); }}
                            class="p-1.5 bg-green-50 dark:bg-green-900 text-green-600 rounded"
                            aria-label={`Save threshold edits for ${resource.displayName || resource.name}`}
                          >
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" /></svg>
                          </button>
                        </Show>
                        <Show when={resource.hasOverride || (resource.type === 'node' && resource.disableConnectivity)}>
                          <button
                            type="button"
                            onClick={() => props.onRemoveOverride(resource.id)}
                            class="p-1.5 bg-red-50 dark:bg-red-900 text-red-600 rounded"
                            aria-label={`Remove override for ${resource.displayName || resource.name}`}
                          >
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                          </button>
                        </Show>
                      </div>
                    </div>

                    {/* Note editor in mobile */}
                    <Show when={isEditing()}>
                      <textarea
                        class="w-full text-xs p-2 rounded border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800"
                        rows={2}
                        placeholder="Add a note..."
                        value={props.editingNote()}
                        onInput={(e) => props.setEditingNote(e.currentTarget.value)}
                      />
                    </Show>

                    {/* Metrics Grid */}
                    <div class="grid grid-cols-2 gap-2 text-sm border-t pt-2 dark:border-slate-800">
                      <For each={props.columns}>
                        {(column) => {
                          const metric = normalizeMetricKey(column);
                          // Check support
                          if (!resourceSupportsMetric(resource.type, metric)) return null;

                          const isDisabled = () => thresholds()?.[metric] === -1;
                          const bounds = metricBounds(metric);

                          return (
                            <div class="flex justify-between items-center p-1.5 bg-slate-50 dark:bg-slate-800 rounded">
                              <span class="text-[10px] text-slate-500 uppercase font-bold tracking-wider">{column.replace(/mb\/s|%|°c/gi, '').trim()}</span>

                              <Show when={isEditing()} fallback={
                                <button
                                  type="button"
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    setActiveMetricInput({ resourceId: resource.id, metric });
                                    props.onEdit(
                                      resource.id,
                                      resource.thresholds ? { ...resource.thresholds } : {},
                                      resource.defaults ? { ...resource.defaults } : {},
                                      resource.note,
                                    );
                                  }}
                                  class="font-mono text-xs font-medium cursor-pointer rounded px-1 -mx-1 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
                                  aria-label={`Edit ${column} threshold for ${resource.displayName || resource.name}`}
                                >
                                  <MetricValueWithHeat resourceId={resource.id} metric={metric} value={displayValue(metric)} isOverridden={isOverridden(metric)} />
                                </button>
                              }>
                                <input type="number" min={bounds.min} max={bounds.max}
                                  value={thresholds()?.[metric] ?? ''}
                                  placeholder={isDisabled() ? 'Off' : ''}
                                  class="w-16 text-right text-xs p-1 rounded border border-slate-200 dark:border-slate-600 bg-white dark:bg-slate-900"
                                  onInput={(e) => {
                                    const val = parseFloat(e.currentTarget.value);
                                    props.setEditingThresholds({ ...props.editingThresholds(), [metric]: Number.isNaN(val) ? undefined : val });
                                  }}
                                />
                              </Show>
                            </div>
                          )
                        }}
                      </For>
                    </div>
                  </Card>
                )
              }}
            </For>
          </div>
        )}
      </For>
      <Show when={!hasRows()}>
        <div class="text-center p-8 text-slate-500 text-sm italic bg-slate-50 dark:bg-slate-800 rounded-md">
          {props.emptyMessage || 'No resources available.'}
        </div>
      </Show>
    </div>
  );

  return (
    <Show when={!isMobile()} fallback={renderMobileView()}>
      <Card
        padding="none"
        class="overflow-hidden border border-slate-200 dark:border-slate-700"
        border={false}
        tone="card"
      >
        <div class="px-4 py-3 border-b border-slate-200 dark:border-slate-700">
          <SectionHeader title={props.title} size="sm" />
        </div>
        <div class="overflow-x-auto" style={{ '-webkit-overflow-scrolling': 'touch' }}>
          <table class="w-full">
            <thead>
              <tr class="bg-slate-50 dark:bg-slate-900 border-b border-slate-200 dark:border-slate-700">
                <th class="px-3 py-2 text-center text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider w-16">
                  Alerts
                </th>
                <th class="px-3 py-2 text-left text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                  Resource
                </th>
                <For each={props.columns}>
                  {(column) => (
                    <th
                      class="px-3 py-2 text-center text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider"
                      title={getColumnHeaderTooltip(column)}
                    >
                      {column}
                    </th>
                  )}
                </For>
                <Show when={props.showOfflineAlertsColumn}>
                  <th
                    class="px-3 py-2 text-center text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider"
                    title={OFFLINE_ALERTS_TOOLTIP}
                  >
                    Offline Alerts
                  </th>
                </Show>
                <th class="px-3 py-2 text-center text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60">
              {/* Global Defaults Row */}
              <Show
                when={props.globalDefaults && props.setGlobalDefaults && props.setHasUnsavedChanges}
              >
                <tr
                  class={`bg-slate-50 dark:bg-slate-800 ${props.globalDisableFlag?.() ? 'opacity-40' : ''}`}
                >
                  <td class="p-1 px-2 text-center align-middle">
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
                  </td>
                  <td class="p-1 px-2">
                    <div class="flex items-center gap-2">
                      <span class="text-sm font-semibold text-slate-700 dark:text-slate-300">
                        Global Defaults
                      </span>
                      <Show when={hasCustomGlobalDefaults()}>
                        <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
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
                              step={metricStep(metric)}
                              value={isOff() ? '' : val()}
                              placeholder={isOff() ? 'Off' : ''}
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
                              class={`w-16 px-2 py-0.5 text-sm text-center border rounded ${isOff()
                                ? 'border-slate-300 dark:border-slate-600 bg-slate-100 dark:bg-slate-800 text-slate-400 dark:text-slate-500 italic placeholder:text-slate-400 dark:placeholder:text-slate-500 placeholder:opacity-60 pointer-events-none'
                                : 'border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-700 text-slate-900 dark:text-slate-100 focus:border-blue-500 focus:ring-1 focus:ring-blue-500'
                                }`}
                              title={
                                isOff()
                                  ? 'Click to enable this metric'
                                  : 'Set to -1 to disable alerts for this metric'
                              }
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
                    </td>
                  </Show>
                  <td class="p-1 px-2 text-center align-middle">
                    <div class="flex items-center justify-center gap-1">
                      <Show
                        when={
                          props.showDelayColumn && typeof props.onMetricDelayChange === 'function'
                        }
                      >
                        <button
                          type="button"
                          onClick={() => setShowDelayRow(!showDelayRow())}
                          class="p-1 text-slate-600 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-300 transition-colors"
                          title={
                            showDelayRow() ? 'Hide alert delay settings' : 'Show alert delay settings'
                          }
                          aria-label={
                            showDelayRow() ? 'Hide alert delay settings' : 'Show alert delay settings'
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
                          title="Reset to factory defaults"
                          aria-label="Reset to factory defaults"
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
                      <Show when={!props.showDelayColumn && !hasCustomGlobalDefaults()}>
                        <span class="text-sm text-slate-400">-</span>
                      </Show>
                    </div>
                  </td>
                </tr>
              </Show>
              <Show
                when={
                  showDelayRow() &&
                  props.showDelayColumn &&
                  typeof props.onMetricDelayChange === 'function'
                }
              >
                <tr
                  class={`bg-slate-50 dark:bg-slate-800 ${props.globalDisableFlag?.() ? 'opacity-40' : ''}`}
                >
                  <td class="p-1 px-2 text-center align-middle">
                    <span class="text-sm text-slate-400">-</span>
                  </td>
                  <td class="p-1 px-2 align-middle">
                    <span class="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-300 inline-flex items-center gap-1">
                      Alert Delay (s)
                      <HelpIcon contentId="alerts.thresholds.delay" size="xs" />
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
                              class="w-16 rounded border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-700 px-2 py-0.5 text-sm text-center text-slate-900 dark:text-slate-100 focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
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
                      <span class="text-sm text-slate-400">-</span>
                    </td>
                  </Show>
                  <td class="p-1 px-2 text-center align-middle">
                    <span class="text-sm text-slate-400">-</span>
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
                        <tr class="bg-slate-50 dark:bg-slate-800">
                          <td
                            colspan={totalColumnCount()}
                            class="p-1 px-2 text-xs font-medium text-slate-600 dark:text-slate-400"
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

                              const defaults = resource.defaults as
                                | Record<string, unknown>
                                | undefined;

                              if (isEditing()) {
                                const edited = extract(thresholds() as Record<string, unknown>);
                                if (edited !== undefined) {
                                  return edited;
                                }
                                const fallback = extract(defaults);
                                return fallback !== undefined ? fallback : 0;
                              }

                              const liveValue = extract(
                                resource.thresholds as Record<string, unknown> | undefined,
                              );
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
                                class={`hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors ${resource.disabled || props.globalDisableFlag?.() ? 'opacity-40' : ''}`}
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
                                </td>
                                <td class="p-1 px-2">
                                  <div class="flex items-center gap-2 min-w-0">
                                    <Show
                                      when={resource.type === 'node'}
                                      fallback={
                                        <span
                                          class={`text-sm font-medium truncate flex-nowrap ${resource.disabled ? 'text-slate-500 dark:text-slate-500' : 'text-slate-900 dark:text-slate-100'}`}
                                        >
                                          {resource.name}
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
                                              class={`text-sm font-medium truncate flex-nowrap ${resource.disabled ? 'text-slate-500 dark:text-slate-500' : 'text-slate-900 dark:text-slate-100'}`}
                                            >
                                              {resource.type === 'node'
                                                ? resource.name
                                                : resource.displayName || resource.name}
                                            </span>
                                          }
                                        >
                                          {(host) => (
                                            <a
                                              href={host() as string}
                                              target="_blank"
                                              rel="noopener noreferrer"
                                              onClick={(e) => e.stopPropagation()}
                                              class={`text-sm font-medium truncate flex-nowrap transition-colors duration-150 ${resource.disabled
                                                ? 'text-slate-500 dark:text-slate-500'
                                                : 'text-slate-900 dark:text-slate-100 hover:text-sky-600 dark:hover:text-sky-400'
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
                                      <span class="text-xs text-slate-500 dark:text-slate-400">
                                        {resource.subtitle as string}
                                      </span>
                                    </Show>
                                    <Show when={resource.hasOverride || resource.disableConnectivity}>
                                      <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                                        Custom
                                      </span>
                                    </Show>
                                    <Show when={isEditing()}>
                                      <div class="mt-2 w-full">
                                        <label class="sr-only" for={`note-${resource.id}`}>
                                          Override note
                                        </label>
                                        <textarea
                                          id={`note-${resource.id}`}
                                          class="w-full rounded border border-slate-300 bg-white px-2 py-1 text-xs text-slate-700 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-200"
                                          rows={2}
                                          placeholder="Add a note about this override (optional)"
                                          value={props.editingNote()}
                                          onInput={(e) => props.setEditingNote(e.currentTarget.value)}
                                        />
                                      </div>
                                    </Show>
                                    <Show when={!isEditing() && resource.note}>
                                      <p class="mt-2 text-xs italic text-slate-500 dark:text-slate-400 break-words">
                                        {resource.note as string}
                                      </p>
                                    </Show>
                                  </div>
                                </td>
                                {/* Metric columns - dynamically rendered based on resource type */}
                                <For each={props.columns}>
                                  {(column) => {
                                    const metric = normalizeMetricKey(column);
                                    const showMetric = () =>
                                      resourceSupportsMetric(resource.type, metric);
                                    const bounds = metricBounds(metric);
                                    const isDisabled = () => thresholds()?.[metric] === -1;
                                    const isSpecialToggle = metric === 'backup' || metric === 'snapshot';

                                    if (isSpecialToggle) {
                                      const config = metric === 'backup' ? resource.backup : resource.snapshot;
                                      const isEnabled = config?.enabled ?? true;
                                      const onToggle = metric === 'backup' ? props.onToggleBackup : props.onToggleSnapshot;
                                      const titlePrefix = metric === 'backup' ? 'Backup' : 'Snapshot';

                                      return (
                                        <td class="p-1 px-2 text-center align-middle">
                                          <Show when={onToggle} fallback={<span class="text-sm text-slate-400">-</span>}>
                                            <div class="flex items-center justify-center">
                                              <StatusBadge
                                                isEnabled={isEnabled}
                                                onToggle={() => onToggle?.(resource.id)}
                                                titleEnabled={`${titlePrefix} alerts enabled. Click to disable for this resource.`}
                                                titleDisabled={`${titlePrefix} alerts disabled. Click to enable for this resource.`}
                                              />
                                            </div>
                                          </Show>
                                        </td>
                                      );
                                    }

                                    const openMetricEditor = (e: MouseEvent) => {
                                      e.stopPropagation();
                                      setActiveMetricInput({ resourceId: resource.id, metric });
                                      props.onEdit(
                                        resource.id,
                                        resource.thresholds ? { ...resource.thresholds } : {},
                                        resource.defaults ? { ...resource.defaults } : {},
                                        typeof resource.note === 'string' ? resource.note : undefined,
                                      );
                                    };

                                    return (
                                      <td class="p-1 px-2 text-center align-middle">
                                        <Show
                                          when={showMetric()}
                                          fallback={
                                            <span class="text-sm text-slate-400 dark:text-slate-500">
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
                                                class="cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 rounded px-1 py-0.5 transition-colors"
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
                                            <div class="flex w-full items-center gap-3">
                                              <Show when={SLIDER_METRICS.has(metric)}>
                                                {(() => {
                                                  const isTemperatureMetric = metric === 'temperature' || metric === 'diskTemperature';
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
                                                  step={metricStep(metric)}
                                                  value={thresholds()?.[metric] ?? ''}
                                                  placeholder={isDisabled() ? 'Off' : ''}
                                                  title="Set to -1 to disable alerts for this metric"
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
                                                      props.setEditingThresholds({
                                                        ...props.editingThresholds(),
                                                        [metric]: undefined,
                                                      });
                                                      return;
                                                    }
                                                    const val = parseFloat(raw);
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
                                                  class={`w-16 px-2 py-0.5 text-sm text-center border rounded ${isDisabled()
                                                    ? 'bg-slate-100 dark:bg-slate-800 text-slate-400 dark:text-slate-600 border-slate-300 dark:border-slate-600'
                                                    : 'bg-white dark:bg-slate-700 text-slate-900 dark:text-slate-100 border-slate-300 dark:border-slate-600'
                                                    }`}
                                                />
                                              </div>
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
                                          class="p-1 text-slate-600 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-300"
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
                                          onClick={() =>
                                            props.onEdit(
                                              resource.id,
                                              resource.thresholds ? { ...resource.thresholds } : {},
                                              resource.defaults ? { ...resource.defaults } : {},
                                              typeof resource.note === 'string' ? resource.note : undefined,
                                            )
                                          }
                                          class="p-1 text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                                          title="Edit thresholds"
                                          aria-label={`Edit thresholds for ${resource.displayName || resource.name}`}
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
                                          ((resource.type === 'node' ||
                                            resource.type === 'dockerHost') &&
                                            resource.disableConnectivity)
                                        }
                                      >
                                        <button
                                          type="button"
                                          onClick={() => props.onRemoveOverride(resource.id)}
                                          class="p-1 text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
                                          title="Remove override"
                                          aria-label={`Remove override for ${resource.displayName || resource.name}`}
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

                          const liveValue = extract(
                            resource.thresholds as Record<string, unknown> | undefined,
                          );
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
                            class={`hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors ${resource.disabled || props.globalDisableFlag?.() ? 'opacity-40' : ''}`}
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
                            </td>
                            <td class="p-1 px-2">
                              <Show
                                when={resource.type === 'node'}
                                fallback={
                                  <div class="flex items-center gap-2 min-w-0">
                                    <span
                                      class={`text-sm font-medium truncate flex-nowrap ${resource.disabled ? 'text-slate-500 dark:text-slate-500' : 'text-slate-900 dark:text-slate-100'}`}
                                    >
                                      {resource.name}
                                    </span>
                                    <Show when={resource.hasOverride || resource.disableConnectivity}>
                                      <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                                        Custom
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
                                        class={`text-sm font-medium truncate flex-nowrap ${resource.disabled ? 'text-slate-500 dark:text-slate-500' : 'text-slate-900 dark:text-slate-100'}`}
                                      >
                                        {resource.type === 'node'
                                          ? resource.name
                                          : resource.displayName || resource.name}
                                      </span>
                                    }
                                  >
                                    {(host) => (
                                      <a
                                        href={host() as string}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        onClick={(e) => e.stopPropagation()}
                                        class={`text-sm font-medium truncate flex-nowrap transition-colors duration-150 ${resource.disabled
                                          ? 'text-slate-500 dark:text-slate-500'
                                          : 'text-slate-900 dark:text-slate-100 hover:text-sky-600 dark:hover:text-sky-400'
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
                                    <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">
                                      {resource.clusterName}
                                    </span>
                                  </Show>
                                  <Show when={resource.hasOverride || resource.disableConnectivity}>
                                    <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
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
                                  if (resource.editable === false) {
                                    return;
                                  }
                                  setActiveMetricInput({ resourceId: resource.id, metric });
                                  props.onEdit(
                                    resource.id,
                                    resource.thresholds ? { ...resource.thresholds } : {},
                                    resource.defaults ? { ...resource.defaults } : {},
                                    typeof resource.note === 'string' ? resource.note : undefined,
                                  );
                                };

                                return (
                                  <td class="p-1 px-2 text-center align-middle">
                                    <Show
                                      when={showMetric()}
                                      fallback={
                                        <span class="text-sm text-slate-400 dark:text-slate-500">
                                          -
                                        </span>
                                      }
                                    >
                                      <Show
                                        when={isEditing()}
                                        fallback={
                                          <div
                                            onClick={openMetricEditor}
                                            class="cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 rounded px-1 py-0.5 transition-colors"
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
                                            step={metricStep(metric)}
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
                                              const val = parseFloat(raw);
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
                                            class={`w-16 px-2 py-0.5 text-sm text-center border rounded ${isDisabled()
                                              ? 'bg-slate-100 dark:bg-slate-800 text-slate-400 dark:text-slate-600 border-slate-300 dark:border-slate-600'
                                              : 'bg-white dark:bg-slate-700 text-slate-900 dark:text-slate-100 border-slate-300 dark:border-slate-600'
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
                                    resource.editable !== false && typeof props.onEdit === 'function'
                                  }
                                  fallback={
                                    <span class="text-xs text-slate-400 dark:text-slate-600">—</span>
                                  }
                                >
                                  <Show
                                    when={!isEditing()}
                                    fallback={
                                      <button
                                        type="button"
                                        onClick={() => {
                                          props.onCancelEdit();
                                          setActiveMetricInput(null);
                                        }}
                                        class="p-1 text-slate-600 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-300"
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
                                        onClick={() =>
                                          props.onEdit(
                                            resource.id,
                                            resource.thresholds ? { ...resource.thresholds } : {},
                                            resource.defaults ? { ...resource.defaults } : {},
                                            typeof resource.note === 'string' ? resource.note : undefined,
                                          )
                                        }
                                        class="p-1 text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                                        title="Edit thresholds"
                                        aria-label={`Edit thresholds for ${resource.displayName || resource.name}`}
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
                                          aria-label={`Remove override for ${resource.displayName || resource.name}`}
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
                                    </div>
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
                      class="px-4 py-8 text-center text-sm text-slate-500 dark:text-slate-400"
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
                    class="px-4 py-6 text-sm text-center text-slate-500 dark:text-slate-400"
                  >
                    {props.emptyMessage || 'No resources available.'}
                  </td>
                </tr>
              </Show>
            </tbody>
          </table>
        </div>
      </Card>
    </Show>
  );
}
