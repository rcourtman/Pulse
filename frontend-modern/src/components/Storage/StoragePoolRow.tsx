import { Component, Show, createMemo } from 'solid-js';
import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';
import { formatBytes, formatPercent } from '@/utils/format';
import { getMetricColorHex } from '@/utils/metricThresholds';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { Resource } from '@/types/resource';
import { EnhancedStorageBar } from './EnhancedStorageBar';
import { StoragePoolDetail } from './StoragePoolDetail';
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
  return 'bg-surface-hover text-base-content';
};

const issueBadgeClass = (record: StorageRecord): string => {
  const severity = (record.incidentSeverity || record.health || '').trim().toLowerCase();
  if (severity === 'critical' || severity === 'offline') {
    return 'bg-red-100 text-red-700 dark:bg-red-950/60 dark:text-red-300';
  }
  if (severity === 'warning') {
    return 'bg-amber-100 text-amber-800 dark:bg-amber-950/60 dark:text-amber-300';
  }
  return 'bg-surface-hover text-base-content';
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
  const normalizedStatus = createMemo(() => (props.record.statusLabel || '').trim().toLowerCase());

  const compactProtection = createMemo(() => {
    if (props.record.rebuildInProgress || props.record.protectionReduced) {
      return protectionLabel();
    }
    const label = protectionLabel().trim();
    if (label && label.toLowerCase() !== 'healthy') {
      return label;
    }
    return '—';
  });

  const compactImpact = createMemo(() => {
    if (
      (props.record.consumerCount || 0) > 0 ||
      (props.record.protectedWorkloadCount || 0) > 0 ||
      (props.record.affectedDatastoreCount || 0) > 0
    ) {
      return impactSummary();
    }
    return '—';
  });

  const compactIssue = createMemo(() => {
    const label = issueLabel().trim();
    if (label && label.toLowerCase() !== 'healthy') {
      return label;
    }
    if (zfsPool() && zfsPool()!.state !== 'ONLINE') {
      return zfsPool()!.state;
    }
    if (
      normalizedStatus() &&
      !['online', 'available', 'running', 'healthy'].includes(normalizedStatus())
    ) {
      return props.record.statusLabel || 'Issue';
    }
    return '—';
  });

  const compactIssueSummary = createMemo(() => {
    if (compactIssue() === '—') return '';
    const summary = issueSummary().trim();
    if (summary && summary.toLowerCase() !== 'healthy') {
      return summary;
    }
    const pool = zfsPool();
    if (!pool) return '';
    const errorParts: string[] = [];
    if ((pool.readErrors || 0) > 0) errorParts.push(`${pool.readErrors} read`);
    if ((pool.writeErrors || 0) > 0) errorParts.push(`${pool.writeErrors} write`);
    if ((pool.checksumErrors || 0) > 0) errorParts.push(`${pool.checksumErrors} checksum`);
    return errorParts.length > 0 ? `${errorParts.join(', ')} errors` : '';
  });

  const compactAction = createMemo(() => {
    if (compactIssue() === '—') return '—';
    const action = actionSummary().trim();
    if (action && action.toLowerCase() !== 'monitor') {
      return action;
    }
    return 'Investigate';
  });

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
        style={{ ...props.rowStyle, height: '38px' }}
        onClick={props.onToggleExpand}
        {...props.alertDataAttrs}
      >
        <td class="px-2 py-1 align-middle text-base-content">
          <span class="block truncate text-[12px] font-semibold" title={props.record.name}>
            {props.record.name}
          </span>
        </td>

        <td class="px-2 py-1 align-middle text-[11px]">
          <span
            class={`inline-flex rounded px-1.5 py-0.5 font-semibold ${platformBadgeClass(
              platformLabel(),
            )}`}
          >
            {platformLabel()}
          </span>
        </td>

        <td class="px-2 py-1 align-middle text-[11px] text-base-content">
          <span class="block truncate" title={topologyLabel()}>
            {topologyLabel()}
          </span>
        </td>

        <td class="px-2 py-1 align-middle text-[11px] text-base-content">
          <span class="block truncate" title={hostLabel()}>
            {hostLabel()}
          </span>
        </td>

        <td class="px-2 py-1 align-middle text-[11px]">
          <Show when={compactProtection() !== '—'} fallback={<span class="text-muted">—</span>}>
            <span
              class={`inline-flex max-w-full rounded px-1.5 py-0.5 font-semibold ${protectionBadgeClass(
                props.record,
              )}`}
              title={compactProtection()}
            >
              <span class="truncate">{compactProtection()}</span>
            </span>
          </Show>
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
              <span
                class="truncate text-muted"
                title={`${formatBytes(usedBytes())} / ${formatBytes(totalBytes())}`}
              >
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
          <span class={`block truncate ${compactImpact() === '—' ? 'text-muted' : ''}`} title={compactImpact()}>
            {compactImpact()}
          </span>
        </td>

        <td class="px-2 py-1 align-middle text-[11px]">
          <Show when={compactIssue() !== '—'} fallback={<span class="text-muted">—</span>}>
            <div class="flex min-w-0 items-center gap-1.5 whitespace-nowrap">
              <span
                class={`shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold ${issueBadgeClass(
                  props.record,
                )}`}
              >
                {compactIssue()}
              </span>
              <Show when={compactIssueSummary()}>
                <span class="truncate text-muted" title={compactIssueSummary()}>
                  {compactIssueSummary()}
                </span>
              </Show>
            </div>
          </Show>
        </td>

        <td class="hidden xl:table-cell px-2 py-1 align-middle text-[11px] text-base-content">
          <span class={`block truncate ${compactAction() === '—' ? 'text-muted' : ''}`} title={compactAction()}>
            {compactAction()}
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
