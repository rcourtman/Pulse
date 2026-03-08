import { Component, Show, createMemo } from 'solid-js';
import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';
import { formatBytes, formatPercent } from '@/utils/format';
import { getMetricColorHex } from '@/utils/metricThresholds';
import type { NormalizedHealth, StorageRecord } from '@/features/storageBackups/models';
import type { Resource } from '@/types/resource';
import { EnhancedStorageBar } from './EnhancedStorageBar';
import { StoragePoolDetail } from './StoragePoolDetail';
import { ZFSHealthMap } from './ZFSHealthMap';
import { useStorageSparkline } from './useStorageSparklines';
import {
  getRecordActionSummary,
  getRecordHostLabel,
  getRecordImpactSummary,
  getRecordIssueLabel,
  getRecordIssueSummary,
  getRecordPlatformLabel,
  getRecordProtectionLabel,
  getRecordTopologyLabel,
  getRecordUsagePercent,
  getRecordZfsPool,
} from './useStorageModel';

interface StoragePoolRowProps {
  record: StorageRecord;
  expanded: boolean;
  groupExpanded: boolean;
  onToggleExpand: () => void;
  rowClass: string;
  rowStyle: Record<string, string>;
  physicalDisks: Resource[];
  alertDataAttrs: {
    'data-row-id': string;
    'data-alert-state': string;
    'data-alert-severity': string;
    'data-resource-highlighted': string;
  };
}

const HEALTH_BADGE: Record<NormalizedHealth, string> = {
  healthy: 'bg-green-100 text-green-700 dark:bg-green-950/60 dark:text-green-300',
  warning: 'bg-amber-100 text-amber-800 dark:bg-amber-950/60 dark:text-amber-300',
  critical: 'bg-red-100 text-red-700 dark:bg-red-950/60 dark:text-red-300',
  offline: 'bg-surface-hover text-muted',
  unknown: 'bg-surface-hover text-muted',
};

const platformBadgeClass = (platform: string): string => {
  switch (platform.trim().toLowerCase()) {
    case 'pve':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-950/60 dark:text-blue-300';
    case 'pbs':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300';
    case 'truenas':
      return 'bg-cyan-100 text-cyan-800 dark:bg-cyan-950/60 dark:text-cyan-300';
    case 'unraid':
      return 'bg-orange-100 text-orange-800 dark:bg-orange-950/60 dark:text-orange-300';
    default:
      return 'bg-surface-hover text-base-content';
  }
};

const protectionBadgeClass = (record: StorageRecord): string => {
  if (record.rebuildInProgress) {
    return 'bg-blue-100 text-blue-800 dark:bg-blue-950/60 dark:text-blue-300';
  }
  if (record.protectionReduced || record.incidentCategory === 'recoverability') {
    return 'bg-red-100 text-red-700 dark:bg-red-950/60 dark:text-red-300';
  }
  if (record.health === 'warning') {
    return 'bg-amber-100 text-amber-800 dark:bg-amber-950/60 dark:text-amber-300';
  }
  if (record.health === 'critical' || record.health === 'offline') {
    return 'bg-red-100 text-red-700 dark:bg-red-950/60 dark:text-red-300';
  }
  return 'bg-green-100 text-green-700 dark:bg-green-950/60 dark:text-green-300';
};

const issueBadgeClass = (record: StorageRecord): string => {
  const severity = (record.incidentSeverity || record.health || '').trim().toLowerCase();
  if (severity === 'critical' || severity === 'offline') {
    return 'bg-red-100 text-red-700 dark:bg-red-950/60 dark:text-red-300';
  }
  if (severity === 'warning') {
    return 'bg-amber-100 text-amber-800 dark:bg-amber-950/60 dark:text-amber-300';
  }
  return 'bg-green-100 text-green-700 dark:bg-green-950/60 dark:text-green-300';
};

export const StoragePoolRow: Component<StoragePoolRowProps> = (props) => {
  const zfsPool = createMemo(() => getRecordZfsPool(props.record));
  const totalBytes = createMemo(() => props.record.capacity.totalBytes || 0);
  const usedBytes = createMemo(() => props.record.capacity.usedBytes || 0);
  const freeBytes = createMemo(
    () =>
      props.record.capacity.freeBytes ??
      (totalBytes() > 0 ? Math.max(totalBytes() - usedBytes(), 0) : 0),
  );
  const usagePercent = createMemo(() => getRecordUsagePercent(props.record));
  const platformLabel = createMemo(() => getRecordPlatformLabel(props.record));
  const hostLabel = createMemo(() => getRecordHostLabel(props.record));
  const topologyLabel = createMemo(() => getRecordTopologyLabel(props.record));
  const protectionLabel = createMemo(() => getRecordProtectionLabel(props.record));
  const issueLabel = createMemo(() => getRecordIssueLabel(props.record));
  const issueSummary = createMemo(() => getRecordIssueSummary(props.record));
  const impactSummary = createMemo(() => getRecordImpactSummary(props.record));
  const actionSummary = createMemo(() => getRecordActionSummary(props.record));

  const sparklineResourceId = createMemo(() => props.record.refs?.resourceId || props.record.id);
  const { data: sparklineData } = useStorageSparkline(
    sparklineResourceId,
    () => props.groupExpanded,
  );

  const sparklineColor = createMemo(() => {
    const data = sparklineData();
    const latestValue = data.length > 0 ? data[data.length - 1].value : usagePercent();
    return getMetricColorHex(latestValue, 'disk');
  });

  return (
    <>
      <tr
        class={`group cursor-pointer ${props.rowClass} ${props.expanded ? 'bg-surface-alt' : ''}`}
        style={{ ...props.rowStyle, 'height': '38px' }}
        onClick={props.onToggleExpand}
        {...props.alertDataAttrs}
      >
        <td class="px-2 py-1 align-middle text-base-content">
          <div class="flex min-w-0 items-center gap-1.5 whitespace-nowrap">
            <span class="truncate text-[12px] font-semibold" title={props.record.name}>
              {props.record.name}
            </span>
            <span
              class={`shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold ${platformBadgeClass(
                platformLabel(),
              )}`}
            >
              {platformLabel()}
            </span>
            <span class="shrink-0 rounded bg-surface-hover px-1.5 py-0.5 text-[10px] font-medium text-base-content">
              {topologyLabel()}
            </span>
            <Show when={zfsPool() && zfsPool()!.devices.length > 0}>
              <span class="shrink-0">
                <ZFSHealthMap pool={zfsPool()!} />
              </span>
            </Show>
            <Show when={zfsPool() && zfsPool()!.state !== 'ONLINE'}>
              <span
                class={`shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold ${
                  zfsPool()?.state === 'DEGRADED'
                    ? 'bg-amber-100 text-amber-800 dark:bg-amber-950/60 dark:text-amber-300'
                    : 'bg-red-100 text-red-700 dark:bg-red-950/60 dark:text-red-300'
                }`}
              >
                {zfsPool()?.state}
              </span>
            </Show>
            <Show
              when={
                zfsPool() &&
                (zfsPool()!.readErrors > 0 ||
                  zfsPool()!.writeErrors > 0 ||
                  zfsPool()!.checksumErrors > 0)
              }
            >
              <span class="shrink-0 rounded bg-red-100 px-1.5 py-0.5 text-[10px] font-semibold text-red-700 dark:bg-red-950/60 dark:text-red-300">
                ERRORS
              </span>
            </Show>
            <span
              class={`shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium ${HEALTH_BADGE[props.record.health]}`}
            >
              {props.record.statusLabel || props.record.health}
            </span>
          </div>
        </td>

        <td class="px-2 py-1 align-middle text-[11px] text-base-content">
          <span class="block truncate" title={`${hostLabel()} · ${platformLabel()}`}>
            {hostLabel()} · {platformLabel()}
          </span>
        </td>

        <td class="px-2 py-1 align-middle text-[11px]">
          <span
            class={`inline-flex max-w-full rounded px-1.5 py-0.5 font-semibold ${protectionBadgeClass(
              props.record,
            )}`}
            title={protectionLabel()}
          >
            <span class="truncate">{protectionLabel()}</span>
          </span>
        </td>

        <td class="px-2 py-1 align-middle md:min-w-[220px]">
          <Show when={totalBytes() > 0} fallback={<span class="text-[11px] text-muted">n/a</span>}>
            <div class="flex items-center gap-2 whitespace-nowrap text-[11px]">
              <span class="shrink-0 font-medium text-base-content">{formatPercent(usagePercent())}</span>
              <div class="min-w-[120px] flex-1">
                <EnhancedStorageBar
                  used={usedBytes()}
                  total={Math.max(totalBytes(), 0)}
                  free={Math.max(freeBytes(), 0)}
                  zfsPool={zfsPool() || undefined}
                />
              </div>
              <span class="truncate text-muted" title={`${formatBytes(usedBytes())} / ${formatBytes(totalBytes())}`}>
                {formatBytes(usedBytes())} / {formatBytes(totalBytes())}
              </span>
              <Show when={sparklineData().length > 0}>
                <div class="hidden xl:block shrink-0" style={{ width: '70px', height: '16px' }}>
                  <InteractiveSparkline
                    series={[{ data: sparklineData(), color: sparklineColor() }]}
                    yMode="percent"
                    size="sm"
                  />
                </div>
              </Show>
            </div>
          </Show>
        </td>

        <td class="hidden lg:table-cell px-2 py-1 align-middle text-[11px] text-base-content">
          <span class="block truncate" title={impactSummary()}>
            {impactSummary()}
          </span>
        </td>

        <td class="px-2 py-1 align-middle text-[11px]">
          <div class="flex min-w-0 items-center gap-1.5 whitespace-nowrap">
            <span
              class={`shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold ${issueBadgeClass(
                props.record,
              )}`}
            >
              {issueLabel()}
            </span>
            <span class="truncate text-muted" title={issueSummary() || 'No active issues'}>
              {issueSummary() || 'No active issues'}
            </span>
          </div>
        </td>

        <td class="hidden xl:table-cell px-2 py-1 align-middle text-[11px] text-base-content">
          <span class="block truncate" title={actionSummary()}>
            {actionSummary()}
          </span>
        </td>

        <td class="px-1.5 py-1 align-middle text-right">
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              props.onToggleExpand();
            }}
            class="rounded p-1 hover:bg-surface-hover transition-colors"
            aria-label={`Toggle details for ${props.record.name}`}
          >
            <svg
              class={`h-3.5 w-3.5 text-muted transition-transform duration-150 ${
                props.expanded ? 'rotate-90' : ''
              }`}
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M9 18l6-6-6-6" />
            </svg>
          </button>
        </td>
      </tr>
      <Show when={props.expanded}>
        <StoragePoolDetail record={props.record} physicalDisks={props.physicalDisks} />
      </Show>
    </>
  );
};
