import type { ZFSPool } from '@/types/api';
import { formatBytes, formatPercent } from '@/utils/format';

export type StorageBarTooltipRow = {
  label: string;
  value: string;
  bordered?: boolean;
};

export type StorageBarZfsSummary = {
  hasErrors: boolean;
  isScrubbing: boolean;
  isResilvering: boolean;
  state: string;
  scan: string;
  errorSummary: string;
};

export const STORAGE_BAR_ROOT_CLASS = 'metric-text w-full h-5 flex items-center min-w-0';
export const STORAGE_BAR_PROGRESS_CLASS = 'h-full';
export const STORAGE_BAR_PULSE_OVERLAY_CLASS = 'absolute inset-0 w-full h-full animate-pulse';
export const STORAGE_BAR_LABEL_WRAP_CLASS =
  'absolute inset-0 flex items-center justify-center text-[10px] font-medium text-base-content leading-none pointer-events-none min-w-0 overflow-hidden';
export const STORAGE_BAR_LABEL_TEXT_CLASS =
  'max-w-full min-w-0 whitespace-nowrap overflow-hidden text-ellipsis px-0.5 text-center';
export const STORAGE_BAR_TOOLTIP_WRAP_CLASS = 'min-w-[160px]';
export const STORAGE_BAR_TOOLTIP_TITLE_CLASS = 'font-medium mb-1 text-slate-300 border-b border-border pb-1';
export const STORAGE_BAR_TOOLTIP_LABEL_CLASS = 'text-slate-400';
export const STORAGE_BAR_TOOLTIP_VALUE_CLASS = 'text-base-content';
export const STORAGE_BAR_ZFS_SECTION_CLASS = 'mt-1 pt-1 border-t border-slate-600';
export const STORAGE_BAR_ZFS_HEADING_CLASS = 'font-medium mb-0.5 text-blue-300';
export const STORAGE_BAR_ZFS_STATE_ROW_CLASS = 'flex justify-between gap-3 py-0.5';
export const STORAGE_BAR_ZFS_STATE_LABEL_CLASS = 'text-slate-400';

export const getStorageBarUsagePercent = (used: number, total: number): number => {
  if (total <= 0) return 0;
  return (used / total) * 100;
};

export const getStorageBarLabel = (used: number, total: number): string =>
  `${formatPercent(getStorageBarUsagePercent(used, total))} (${formatBytes(used)}/${formatBytes(total)})`;

export const getStorageBarTooltipTitle = (): string => 'Storage Details';

export const getStorageBarTooltipRowClass = (bordered = false): string =>
  `flex justify-between gap-3 py-0.5 ${bordered ? 'border-t border-border mt-0.5 pt-0.5' : ''}`;

export const getStorageBarZfsHeadingLabel = (): string => 'ZFS Status';

export const getStorageBarTooltipRows = (
  used: number,
  free: number,
  total: number,
): StorageBarTooltipRow[] => [
  { label: 'Used', value: formatBytes(used) },
  { label: 'Free', value: formatBytes(free) },
  { label: 'Total', value: formatBytes(total), bordered: true },
];

export const getStorageBarZfsSummary = (zfsPool?: ZFSPool): StorageBarZfsSummary | null => {
  if (!zfsPool) return null;

  const scan = zfsPool.scan || '';
  const hasErrors =
    zfsPool.readErrors > 0 || zfsPool.writeErrors > 0 || zfsPool.checksumErrors > 0;

  return {
    hasErrors,
    isScrubbing: scan.toLowerCase().includes('scrub'),
    isResilvering: scan.toLowerCase().includes('resilver'),
    state: zfsPool.state,
    scan: scan && scan !== 'none' ? scan : '',
    errorSummary: hasErrors
      ? `Errors: R:${zfsPool.readErrors} W:${zfsPool.writeErrors} C:${zfsPool.checksumErrors}`
      : '',
  };
};
