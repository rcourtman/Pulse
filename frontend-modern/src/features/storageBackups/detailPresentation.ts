import { INFO_CARD_FRAME_CLASS } from '@/components/shared/InfoCardFrame';

// Classes of the compact disk detail the guest drawer embeds for a guest's
// physical disks (DiskDetail). The Storage page's pool and disk drawers are
// built from the shared drawer primitives and use none of these.

export const STORAGE_DETAIL_CARD_CLASS = 'rounded-md border border-border bg-surface p-3 shadow-xs';

export const STORAGE_DETAIL_SECTION_TITLE_CLASS = 'text-xs font-semibold text-base-content';

export const STORAGE_DETAIL_INLINE_LABEL_CLASS =
  'text-[10px] font-medium text-muted uppercase tracking-wide';

export const STORAGE_DETAIL_BADGE_CLASS =
  'text-[10px] font-normal text-slate-400 bg-surface-alt px-1.5 py-0.5 rounded-sm';

export const STORAGE_DETAIL_META_ROW_CLASS =
  'flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px]';

export const STORAGE_DETAIL_MONO_CHIP_CLASS =
  'text-muted font-mono bg-surface-alt px-1.5 py-0.5 rounded-sm border border-border';

export const STORAGE_DETAIL_HEADER_ROW_CLASS = 'flex items-center justify-between mb-2';
export const STORAGE_DETAIL_KEY_CLASS = 'text-muted';
export const STORAGE_DETAIL_VALUE_CLASS = 'text-base-content font-medium';

export const STORAGE_DETAIL_EMPTY_CLASS = `${INFO_CARD_FRAME_CLASS} text-center text-[11px] text-muted`;

export const STORAGE_DISK_DETAIL_ROOT_CLASS = 'space-y-3';
export const STORAGE_DISK_DETAIL_HEADER_CLASS =
  'flex flex-wrap items-end justify-between gap-3 border-b border-border-subtle pb-3';
export const STORAGE_DISK_DETAIL_MODEL_CLASS = 'font-semibold text-base-content text-sm';
export const STORAGE_DISK_DETAIL_SERIAL_CLASS = 'font-mono';
export const STORAGE_DISK_DETAIL_NODE_CLASS = 'text-muted';
export const STORAGE_DISK_DETAIL_ATTRIBUTE_GRID_CLASS =
  'flex flex-wrap gap-3 *:flex-1 *:basis-[calc(16.666%-0.5rem)] *:min-w-[120px]';
export const STORAGE_DISK_DETAIL_SECTION_CLASS = 'space-y-2';
export const STORAGE_DISK_DETAIL_LIVE_GRID_CLASS = 'grid grid-cols-1 sm:grid-cols-3 gap-3';
export const STORAGE_DISK_DETAIL_HISTORY_GRID_CLASS =
  'flex flex-wrap gap-3 *:flex-1 *:basis-[calc(33.333%-0.5rem)] *:min-w-[250px]';
export const STORAGE_DISK_DETAIL_SECTION_HEADING_CLASS = 'flex items-center gap-2';
