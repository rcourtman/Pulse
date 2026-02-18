import { Component, createMemo, Show } from 'solid-js';
import { Sparkline } from '@/components/shared/Sparkline';
import { EnhancedStorageBar } from './EnhancedStorageBar';
import { StoragePoolDetail } from './StoragePoolDetail';
import { ZFSHealthMap } from './ZFSHealthMap';
import { useStorageSparkline } from './useStorageSparklines';
import { getRecordNodeLabel, getRecordType, getRecordZfsPool } from './useStorageModel';
import type { NormalizedHealth, StorageRecord } from '@/features/storageBackups/models';
import type { Resource } from '@/types/resource';
import type { StorageGroupKey } from './useStorageModel';

interface StoragePoolRowProps {
  record: StorageRecord;
  groupBy: StorageGroupKey;
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

const HEALTH_BADGE: Record<NormalizedHealth, { bg: string; text: string }> = {
  healthy: { bg: 'bg-green-100 dark:bg-green-900/40', text: 'text-green-700 dark:text-green-300' },
  warning: { bg: 'bg-yellow-100 dark:bg-yellow-900/40', text: 'text-yellow-700 dark:text-yellow-300' },
  critical: { bg: 'bg-red-100 dark:bg-red-900/40', text: 'text-red-700 dark:text-red-300' },
  offline: { bg: 'bg-gray-100 dark:bg-gray-700/40', text: 'text-gray-600 dark:text-gray-300' },
  unknown: { bg: 'bg-gray-100 dark:bg-gray-700/40', text: 'text-gray-500 dark:text-gray-400' },
};

const TYPE_BADGE: Record<string, string> = {
  zfspool: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
  dir: 'bg-gray-100 text-gray-700 dark:bg-gray-700/40 dark:text-gray-300',
  lvm: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
  lvmthin: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
  nfs: 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300',
  cifs: 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300',
  cephfs: 'bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-300',
  rbd: 'bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-300',
  btrfs: 'bg-teal-100 text-teal-700 dark:bg-teal-900/40 dark:text-teal-300',
  iscsi: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/40 dark:text-cyan-300',
  pbs: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
};

const getTypeBadgeClass = (type: string): string => {
  const key = type.toLowerCase().replace(/[-_\s]/g, '');
  return TYPE_BADGE[key] || 'bg-gray-100 text-gray-600 dark:bg-gray-700/40 dark:text-gray-400';
};

export const StoragePoolRow: Component<StoragePoolRowProps> = (props) => {
  const zfsPool = createMemo(() => getRecordZfsPool(props.record));
  const type = createMemo(() => getRecordType(props.record));
  const totalBytes = createMemo(() => props.record.capacity.totalBytes || 0);
  const usedBytes = createMemo(() => props.record.capacity.usedBytes || 0);
  const freeBytes = createMemo(() =>
    props.record.capacity.freeBytes ?? (totalBytes() > 0 ? Math.max(totalBytes() - usedBytes(), 0) : 0),
  );
  const healthBadge = createMemo(() => HEALTH_BADGE[props.record.health]);

  const sparklineResourceId = createMemo(() => props.record.refs?.resourceId || props.record.id);
  const { data: sparklineData } = useStorageSparkline(
    sparklineResourceId,
    () => props.groupExpanded,
  );

  return (
    <>
      <tr
        class={props.rowClass}
        style={props.rowStyle}
        {...props.alertDataAttrs}
      >
        {/* Name + badges */}
        <td class="px-1.5 sm:px-2 py-1 text-gray-900 dark:text-gray-100">
          <div class="flex items-center gap-1.5 min-w-0">
            <span class="truncate max-w-[220px] text-[11px]" title={props.record.name}>
              {props.record.name}
            </span>
            <Show when={zfsPool() && zfsPool()!.devices.length > 0}>
              <span class="mx-0.5">
                <ZFSHealthMap pool={zfsPool()!} />
              </span>
            </Show>
            <Show when={zfsPool() && zfsPool()!.state !== 'ONLINE'}>
              <span
                class={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
                  zfsPool()?.state === 'DEGRADED'
                    ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300'
                    : 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'
                }`}
              >
                {zfsPool()?.state}
              </span>
            </Show>
            <Show
              when={
                zfsPool() &&
                ((zfsPool()!.readErrors > 0) ||
                  (zfsPool()!.writeErrors > 0) ||
                  (zfsPool()!.checksumErrors > 0))
              }
            >
              <span class="px-1.5 py-0.5 rounded text-[10px] font-medium bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300">
                ERRORS
              </span>
            </Show>
          </div>
        </td>

        {/* Node (only when not grouped by node) */}
        <Show when={props.groupBy !== 'node'}>
          <td class="px-1.5 sm:px-2 py-1 text-xs text-gray-600 dark:text-gray-400">
            {getRecordNodeLabel(props.record)}
          </td>
        </Show>

        {/* Type badge */}
        <td class="px-1.5 sm:px-2 py-1">
          <span class={`inline-block px-1.5 py-0.5 rounded text-[10px] font-medium ${getTypeBadgeClass(type())}`}>
            {type()}
          </span>
        </td>

        {/* Capacity bar */}
        <td class="px-1.5 sm:px-2 py-1 min-w-[180px]">
          <Show
            when={totalBytes() > 0}
            fallback={<span class="text-xs text-gray-400">n/a</span>}
          >
            <EnhancedStorageBar
              used={usedBytes()}
              total={Math.max(totalBytes(), 0)}
              free={Math.max(freeBytes(), 0)}
              zfsPool={zfsPool() || undefined}
            />
          </Show>
        </td>

        {/* Sparkline: 7-day usage trend */}
        <td class="px-1.5 sm:px-2 py-1 w-[120px] hidden md:table-cell">
          <Show when={sparklineData().length > 0} fallback={<div class="h-4 w-full" />}>
            <Sparkline data={sparklineData()} metric="disk" width={100} height={20} />
          </Show>
        </td>

        {/* Health */}
        <td class="px-1.5 sm:px-2 py-1">
          <span class={`inline-block px-1.5 py-0.5 rounded text-[10px] font-medium ${healthBadge().bg} ${healthBadge().text}`}>
            {props.record.health}
          </span>
        </td>

        {/* Expand chevron */}
        <td class="px-1.5 sm:px-2 py-1 text-right">
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              props.onToggleExpand();
            }}
            class="p-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
            aria-label={`Toggle details for ${props.record.name}`}
          >
            <svg
              class={`w-3.5 h-3.5 text-gray-400 dark:text-gray-500 transition-transform duration-150 ${
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
