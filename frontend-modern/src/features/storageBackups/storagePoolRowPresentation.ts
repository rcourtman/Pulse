import { getSourcePlatformPresentation } from '@/utils/sourcePlatforms';
import type { StorageRecord } from './models';
import type { StorageCapacityDeltaPresentation } from './storageCapacityDeltaPresentation';
import {
  getStorageRecordHostLabel,
  getStorageRecordPlatformLabel,
  getStorageRecordTopologyLabel,
  getStorageRecordZfsPool,
} from './recordPresentation';
import {
  getCompactStoragePoolImpactLabel,
  getCompactStoragePoolIssueLabel,
  getCompactStoragePoolIssueSummary,
  getCompactStoragePoolProtectionLabel,
  getCompactStoragePoolProtectionTitle,
} from './rowPresentation';

export interface StoragePoolRowModel {
  zfsPool: ReturnType<typeof getStorageRecordZfsPool>;
  totalBytes: number;
  usedBytes: number;
  freeBytes: number;
  platformLabel: string;
  platformToneClass: string;
  hostLabel: string;
  topologyLabel: string;
  compactProtection: string;
  compactProtectionTitle: string;
  capacityDeltaLabel: string;
  capacityDeltaTitle: string;
  capacityDeltaToneClass: string;
  compactImpact: string;
  compactIssue: string;
  compactIssueSummary: string;
}

export const STORAGE_POOL_ROW_CLASS = 'group cursor-pointer';
export const STORAGE_POOL_ROW_HEIGHT_CLASS = 'h-[38px]';
export const STORAGE_POOL_ROW_EXPANDED_CLASS = 'bg-surface-alt';
export const STORAGE_POOL_ROW_NAME_CELL_CLASS =
  'w-[34%] sm:w-[26%] md:w-[23%] xl:w-[18%] overflow-hidden px-1.5 sm:px-2 py-1 align-middle text-base-content';
export const STORAGE_POOL_ROW_NAME_TEXT_CLASS = 'block truncate text-[12px] font-semibold';
export const STORAGE_POOL_ROW_SOURCE_CELL_CLASS =
  'w-[13%] sm:w-[10%] md:w-[8%] overflow-hidden px-1 sm:px-2 py-1 align-middle text-[11px]';
export const STORAGE_POOL_ROW_SOURCE_BADGE_CLASS =
  'inline-flex max-w-full min-w-0 justify-center overflow-hidden text-ellipsis px-1 sm:px-1.5 py-px text-[9px] font-medium';
export const STORAGE_POOL_ROW_TYPE_CELL_CLASS =
  'hidden xl:table-cell xl:w-[8%] overflow-hidden px-2 py-1 align-middle text-[11px] text-base-content';
export const STORAGE_POOL_ROW_HOST_CELL_CLASS =
  'hidden sm:table-cell sm:w-[13%] md:w-[12%] xl:w-[10%] overflow-hidden px-1.5 sm:px-2 py-1 align-middle text-[11px] text-base-content';
export const STORAGE_POOL_ROW_TEXT_TRUNCATE_CLASS = 'block truncate';
export const STORAGE_POOL_ROW_PROTECTION_CELL_CLASS =
  'hidden sm:table-cell sm:w-[15%] md:w-[15%] xl:w-[13%] overflow-hidden px-1.5 sm:px-2 py-1 align-middle text-[11px]';
export const STORAGE_POOL_ROW_PROTECTION_TEXT_CLASS = 'block truncate font-semibold';
export const STORAGE_POOL_ROW_USAGE_CELL_CLASS =
  'w-[29%] sm:w-[16%] md:w-[25%] xl:w-[18%] overflow-hidden px-1.5 sm:px-2 py-1 align-middle';
export const STORAGE_POOL_ROW_USAGE_WRAP_CLASS = 'flex items-center whitespace-nowrap text-[11px]';
export const STORAGE_POOL_ROW_USAGE_BAR_WRAP_CLASS = 'min-w-0 flex-1';
export const STORAGE_POOL_ROW_GROWTH_CELL_CLASS =
  'hidden xl:table-cell xl:w-[11%] overflow-hidden px-2 py-1 align-middle text-[11px]';
export const STORAGE_POOL_ROW_GROWTH_TEXT_CLASS = 'block truncate font-mono font-semibold';
export const STORAGE_POOL_ROW_ISSUE_CELL_CLASS =
  'w-[24%] sm:w-[20%] md:w-[17%] xl:w-[14%] overflow-hidden px-1.5 sm:px-2 py-1 align-middle text-[11px]';
export const STORAGE_POOL_ROW_ISSUE_TEXT_CLASS = 'block truncate text-[11px] font-semibold';
export const STORAGE_POOL_ROW_PLACEHOLDER_CLASS = 'text-muted';
export const STORAGE_POOL_ROW_USAGE_FALLBACK_CLASS = 'text-[11px] text-muted';

export const buildStoragePoolRowModel = (
  record: StorageRecord,
  capacityDelta: StorageCapacityDeltaPresentation | null = null,
): StoragePoolRowModel => {
  const totalBytes = record.capacity.totalBytes || 0;
  const usedBytes = record.capacity.usedBytes || 0;
  const freeBytes =
    record.capacity.freeBytes ?? (totalBytes > 0 ? Math.max(totalBytes - usedBytes, 0) : 0);
  const platformPresentation = getSourcePlatformPresentation(record.source.platform);

  return {
    zfsPool: getStorageRecordZfsPool(record),
    totalBytes,
    usedBytes,
    freeBytes,
    platformLabel: platformPresentation?.label || getStorageRecordPlatformLabel(record),
    platformToneClass: platformPresentation?.tone || 'text-base-content',
    hostLabel: getStorageRecordHostLabel(record),
    topologyLabel: getStorageRecordTopologyLabel(record),
    compactProtection: getCompactStoragePoolProtectionLabel(record),
    compactProtectionTitle: getCompactStoragePoolProtectionTitle(record),
    capacityDeltaLabel: capacityDelta?.label ?? '—',
    capacityDeltaTitle: capacityDelta?.title ?? 'No used-capacity change history available.',
    capacityDeltaToneClass: capacityDelta?.toneClass ?? STORAGE_POOL_ROW_PLACEHOLDER_CLASS,
    compactImpact: getCompactStoragePoolImpactLabel(record),
    compactIssue: getCompactStoragePoolIssueLabel(record),
    compactIssueSummary: getCompactStoragePoolIssueSummary(record),
  };
};
