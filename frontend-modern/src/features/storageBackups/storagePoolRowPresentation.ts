import { getSourcePlatformPresentation } from '@/utils/sourcePlatforms';
import type { StorageRecord } from './models';
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
  compactImpact: string;
  compactIssue: string;
  compactIssueSummary: string;
}

export const STORAGE_POOL_ROW_CLASS = 'group cursor-pointer';
export const STORAGE_POOL_ROW_EXPANDED_CLASS = 'bg-surface-alt';
export const STORAGE_POOL_ROW_STYLE = { height: '38px' } as const;
export const STORAGE_POOL_ROW_NAME_CELL_CLASS = 'px-2 py-1 align-middle text-base-content';
export const STORAGE_POOL_ROW_NAME_TEXT_CLASS = 'block truncate text-[12px] font-semibold';
export const STORAGE_POOL_ROW_SOURCE_CELL_CLASS = 'px-2 py-1 align-middle text-[11px]';
export const STORAGE_POOL_ROW_SOURCE_BADGE_CLASS =
  'inline-flex min-w-[3.25rem] justify-center px-1.5 py-px text-[9px] font-medium';
export const STORAGE_POOL_ROW_TYPE_CELL_CLASS =
  'hidden xl:table-cell px-2 py-1 align-middle text-[11px] text-base-content';
export const STORAGE_POOL_ROW_HOST_CELL_CLASS = 'px-2 py-1 align-middle text-[11px] text-base-content';
export const STORAGE_POOL_ROW_TEXT_TRUNCATE_CLASS = 'block truncate';
export const STORAGE_POOL_ROW_PROTECTION_CELL_CLASS = 'px-2 py-1 align-middle text-[11px]';
export const STORAGE_POOL_ROW_PROTECTION_TEXT_CLASS = 'block truncate font-semibold';
export const STORAGE_POOL_ROW_USAGE_CELL_CLASS = 'px-2 py-1 align-middle md:min-w-[190px] xl:min-w-[220px]';
export const STORAGE_POOL_ROW_USAGE_WRAP_CLASS = 'flex items-center whitespace-nowrap text-[11px]';
export const STORAGE_POOL_ROW_USAGE_BAR_WRAP_CLASS = 'min-w-[120px] flex-1';
export const STORAGE_POOL_ROW_IMPACT_CELL_CLASS =
  'hidden lg:table-cell px-2 py-1 align-middle text-[11px] text-base-content';
export const STORAGE_POOL_ROW_ISSUE_CELL_CLASS = 'px-2 py-1 align-middle text-[11px]';
export const STORAGE_POOL_ROW_ISSUE_TEXT_CLASS = 'block truncate text-[11px] font-semibold';
export const STORAGE_POOL_ROW_EXPAND_CELL_CLASS = 'px-1.5 py-1 align-middle text-right';
export const STORAGE_POOL_ROW_EXPAND_BUTTON_CLASS = 'rounded p-1 hover:bg-surface-hover transition-colors';
export const STORAGE_POOL_ROW_PLACEHOLDER_CLASS = 'text-muted';
export const STORAGE_POOL_ROW_USAGE_FALLBACK_CLASS = 'text-[11px] text-muted';
export const STORAGE_POOL_ROW_EXPAND_ICON_BASE_CLASS =
  'h-3.5 w-3.5 text-muted transition-transform duration-150';

export const buildStoragePoolRowModel = (record: StorageRecord): StoragePoolRowModel => {
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
    compactImpact: getCompactStoragePoolImpactLabel(record),
    compactIssue: getCompactStoragePoolIssueLabel(record),
    compactIssueSummary: getCompactStoragePoolIssueSummary(record),
  };
};

export const getStoragePoolExpandIconClass = (expanded: boolean): string =>
  `${STORAGE_POOL_ROW_EXPAND_ICON_BASE_CLASS} ${expanded ? 'rotate-90' : ''}`.trim();

export const getStoragePoolImpactTextClass = (impact: string): string =>
  `${STORAGE_POOL_ROW_TEXT_TRUNCATE_CLASS} ${impact === '—' ? STORAGE_POOL_ROW_PLACEHOLDER_CLASS : ''}`.trim();
