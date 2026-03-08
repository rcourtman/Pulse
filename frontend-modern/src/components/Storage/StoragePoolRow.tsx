import { Component, Show, createMemo } from 'solid-js';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { Resource } from '@/types/resource';
import { EnhancedStorageBar } from './EnhancedStorageBar';
import { StoragePoolDetail } from './StoragePoolDetail';
import {
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

const platformTextClass = (platform: string): string => {
  switch (platform.trim().toLowerCase()) {
    case 'pve':
      return 'text-blue-700 dark:text-blue-300';
    case 'pbs':
      return 'text-emerald-700 dark:text-emerald-300';
    case 'truenas':
      return 'text-cyan-700 dark:text-cyan-300';
    case 'unraid':
      return 'text-orange-700 dark:text-orange-300';
    default:
      return 'text-base-content';
  }
};

const protectionTextClass = (record: StorageRecord): string => {
  if (record.rebuildInProgress) {
    return 'text-blue-700 dark:text-blue-300';
  }
  if (record.protectionReduced || record.incidentCategory === 'recoverability') {
    return 'text-red-700 dark:text-red-300';
  }
  return 'text-base-content';
};

const issueTextClass = (record: StorageRecord): string => {
  const severity = (record.incidentSeverity || record.health || '').trim().toLowerCase();
  if (severity === 'critical' || severity === 'offline') {
    return 'text-red-700 dark:text-red-300';
  }
  if (severity === 'warning') {
    return 'text-amber-700 dark:text-amber-300';
  }
  return 'text-base-content';
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
            class={`inline-block font-semibold tracking-wide ${platformTextClass(platformLabel())}`}
          >
            {platformLabel()}
          </span>
        </td>

        <td class="hidden xl:table-cell px-2 py-1 align-middle text-[11px] text-base-content">
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
              class={`block truncate font-semibold ${protectionTextClass(props.record)}`}
              title={compactProtection()}
            >
              {compactProtection()}
            </span>
          </Show>
        </td>

        <td class="px-2 py-1 align-middle md:min-w-[190px] xl:min-w-[220px]">
          <Show when={totalBytes() > 0} fallback={<span class="text-[11px] text-muted">n/a</span>}>
            <div class="flex items-center whitespace-nowrap text-[11px]">
              <div class="min-w-[120px] flex-1">
                <EnhancedStorageBar
                  used={usedBytes()}
                  total={Math.max(totalBytes(), 0)}
                  free={Math.max(freeBytes(), 0)}
                  zfsPool={zfsPool() || undefined}
                />
              </div>
            </div>
          </Show>
        </td>

        <td class="hidden lg:table-cell px-2 py-1 align-middle text-[11px] text-base-content">
          <span
            class={`block truncate ${compactImpact() === '—' ? 'text-muted' : ''}`}
            title={compactImpact()}
          >
            {compactImpact()}
          </span>
        </td>

        <td class="px-2 py-1 align-middle text-[11px]">
          <Show when={compactIssue() !== '—'} fallback={<span class="text-muted">—</span>}>
            <div class="flex min-w-0 items-center gap-1.5 whitespace-nowrap">
              <span class={`shrink-0 text-[11px] font-semibold ${issueTextClass(props.record)}`}>
                {compactIssue()}
              </span>
              <Show when={compactIssueSummary()}>
                <span class="hidden xl:block truncate text-muted" title={compactIssueSummary()}>
                  {compactIssueSummary()}
                </span>
              </Show>
            </div>
          </Show>
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
