import type { JSX } from 'solid-js';
import type {
  PhysicalDiskCollectionStatus,
  PhysicalDiskFieldStatus,
  Resource,
  ResourceStorageRiskReason,
} from '@/types/resource';
import { getPlatformTableWeightedColumnWidthStyle } from '@/features/platformPage/sharedPlatformPage';
import {
  getSourcePlatformLabel,
  getSourcePlatformPresentation,
  resolvePlatformTypeFromSources,
} from '@/utils/sourcePlatforms';
import { getAllFilterOptionLabel } from '@/components/shared/filterOptionPresentation';
import { getPhysicalDiskNodeIdentity } from '@/components/Storage/diskResourceUtils';
import { getInfrastructureSettingsLocationLabel } from '@/utils/infrastructureSettingsPresentation';
import { normalizeStorageSourceKey, storageSourceMatchesFilter } from '@/utils/storageSources';
import { formatTemperature } from '@/utils/temperature';
import type { NormalizedHealth, StorageHealthFilter } from './models';
import { matchesStorageNodeTerms, parseStorageSearchQuery } from './storageSearchQuery';

export interface DiskHealthStatusPresentation {
  label: string;
  summary: string;
  tone: string;
}

export interface PhysicalDiskEmptyStatePresentation {
  title: string;
  nodeMessage: string | null;
  searchMessage: string | null;
  filterMessages: string[];
  showRequirements: boolean;
  fallbackMessage: string;
  requirementsTitle: string;
  requirementsItems: string[];
  requirementsNote: string;
}

export interface PhysicalDiskFilterOption {
  value: string;
  label: string;
}

export interface PhysicalDiskPresentationData {
  node: string;
  instance: string;
  devPath: string;
  model: string;
  vendor?: string;
  serial: string;
  wwn: string;
  size: number;
  health: string;
  riskLevel?: string;
  riskReasons: string[];
  /** The same reasons with their codes and severities, as the API sent them. */
  riskReasonDetails?: ResourceStorageRiskReason[];
  wearout: number;
  storageRole?: string;
  storageGroup?: string;
  storageState?: string;
  spunDown?: boolean;
  readCount?: number;
  writeCount?: number;
  errorCount?: number;
  type: string;
  controller?: string;
  target?: string;
  temperature: number;
  rpm: number;
  used: string;
  collection?: PhysicalDiskCollectionStatus;
  smartAttributes?: {
    powerOnHours?: number;
    powerCycles?: number;
    reallocatedSectors?: number;
    pendingSectors?: number;
    offlineUncorrectable?: number;
    udmaCrcErrors?: number;
    percentageUsed?: number;
    availableSpare?: number;
    mediaErrors?: number;
    unsafeShutdowns?: number;
  };
}

export const PHYSICAL_DISK_EMPTY_CARD_CLASS = 'text-center';
export const PHYSICAL_DISK_EMPTY_TITLE_CLASS = 'text-sm font-medium';
export const PHYSICAL_DISK_EMPTY_MESSAGE_CLASS = 'text-xs mt-1';
export const PHYSICAL_DISK_EMPTY_FALLBACK_CLASS =
  'mt-4 rounded-md border border-border bg-surface-alt p-4 text-left';
export const PHYSICAL_DISK_EMPTY_FALLBACK_TEXT_CLASS = 'text-sm text-muted';
export const PHYSICAL_DISK_EMPTY_REQUIREMENTS_CLASS =
  'mt-4 rounded-md border border-blue-200 bg-blue-50 p-4 text-left dark:border-blue-800 dark:bg-blue-900/25';
export const PHYSICAL_DISK_EMPTY_REQUIREMENTS_TITLE_CLASS =
  'mb-2 text-sm font-medium text-blue-900 dark:text-blue-100';
export const PHYSICAL_DISK_EMPTY_REQUIREMENTS_LIST_CLASS =
  'ml-4 list-decimal space-y-1.5 text-xs text-blue-800 dark:text-blue-200';
export const PHYSICAL_DISK_EMPTY_REQUIREMENTS_NOTE_CLASS =
  'mt-3 text-xs italic text-blue-700 dark:text-blue-300';

const PHYSICAL_DISK_TABLE_HEADER_CLASS =
  'overflow-hidden text-ellipsis whitespace-nowrap px-1 sm:px-1.5 lg:px-2 py-0.5 text-left text-[10px] lg:text-xs font-medium uppercase tracking-wider';

export const PHYSICAL_DISK_TABLE_CLASS = 'platform-table w-full table-fixed text-xs';
export const PHYSICAL_DISK_TABLE_HEADER_ROW_CLASS =
  'border-b border-border bg-surface-alt text-muted';
export const PHYSICAL_DISK_TABLE_BODY_CLASS = 'divide-y divide-border';
export const PHYSICAL_DISK_TABLE_ROW_CLASS = 'cursor-pointer transition-colors';
export const PHYSICAL_DISK_TABLE_ROW_SELECTED_CLASS = 'bg-blue-50 dark:bg-blue-900/25';
export const PHYSICAL_DISK_TABLE_ROW_HOVER_CLASS = 'hover:bg-surface-hover';
export const PHYSICAL_DISK_TABLE_ROW_STYLE = { height: '32px' } as const;
export const PHYSICAL_DISK_DETAIL_ROW_CELL_CLASS =
  'border-b border-border-subtle bg-surface-alt px-4 py-4 shadow-inner';
export type PhysicalDiskTableLayoutMode =
  'narrow' | 'compact' | 'basic' | 'operational' | 'expanded' | 'full';

export const getPhysicalDiskTableLayoutModeForContainer = (
  containerWidth: number,
): PhysicalDiskTableLayoutMode => {
  if (containerWidth >= 1_120) return 'full';
  if (containerWidth >= 900) return 'expanded';
  if (containerWidth >= 650) return 'operational';
  if (containerWidth >= 520) return 'basic';
  if (containerWidth > 0 && containerWidth < 360) return 'narrow';
  return 'compact';
};

export type PhysicalDiskTableColumnId =
  'disk' | 'device' | 'host' | 'role' | 'parent' | 'health' | 'life' | 'temp' | 'size';

const PHYSICAL_DISK_VISIBLE_COLUMNS: Record<
  PhysicalDiskTableLayoutMode,
  readonly PhysicalDiskTableColumnId[]
> = {
  // Endurance is available in the expanded disk detail. At the narrowest
  // phone width, keep identity, placement, immediate health, temperature,
  // and capacity readable on one line.
  narrow: ['disk', 'host', 'health', 'temp', 'size'],
  // Keep the phone view useful at a glance: identity, placement, health,
  // endurance, temperature, and capacity all fit without horizontal scroll.
  compact: ['disk', 'host', 'health', 'life', 'temp', 'size'],
  basic: ['disk', 'host', 'health', 'temp', 'size'],
  operational: ['disk', 'host', 'parent', 'health', 'life', 'temp', 'size'],
  expanded: ['disk', 'host', 'role', 'parent', 'health', 'life', 'temp', 'size'],
  full: ['disk', 'device', 'host', 'role', 'parent', 'health', 'life', 'temp', 'size'],
};

// Relative weights per layout, resolved through the canonical weighted-width
// helper. The phone layouts are sized from the measured values they show:
// a temperature ("100°C"), a capacity ("5.46 TB"), a compact health word and
// a short node name must each fit whole, and the disk model takes the rest.
const PHYSICAL_DISK_COLUMN_WEIGHTS: Record<
  PhysicalDiskTableLayoutMode,
  Partial<Record<PhysicalDiskTableColumnId, number>>
> = {
  narrow: { disk: 41, host: 12, health: 19, temp: 12, size: 16 },
  compact: { disk: 34, host: 11, health: 17, life: 11, temp: 12, size: 15 },
  basic: { disk: 33, host: 17, health: 22, temp: 12, size: 16 },
  operational: { disk: 25, host: 12, parent: 15, health: 17, life: 9, temp: 9, size: 13 },
  expanded: { disk: 22, host: 10, role: 9, parent: 14, health: 15, life: 8, temp: 8, size: 14 },
  full: {
    disk: 19,
    device: 9,
    host: 10,
    role: 8,
    parent: 12,
    health: 17,
    life: 6,
    temp: 7,
    size: 12,
  },
};

export const isPhysicalDiskColumnVisible = (
  layout: PhysicalDiskTableLayoutMode,
  columnId: PhysicalDiskTableColumnId,
): boolean => PHYSICAL_DISK_VISIBLE_COLUMNS[layout].includes(columnId);

export const getPhysicalDiskColumnWidthStyle = (
  layout: PhysicalDiskTableLayoutMode,
  columnId: PhysicalDiskTableColumnId,
): JSX.CSSProperties =>
  getPlatformTableWeightedColumnWidthStyle(
    columnId,
    PHYSICAL_DISK_COLUMN_WEIGHTS[layout],
    PHYSICAL_DISK_VISIBLE_COLUMNS[layout],
  );

// The shared table cell primitives pad every cell for desktop density. On the
// phone layouts that padding alone is a fifth of the row, so rendered cells
// and headers shed it the same way the Proxmox replication table does.
export const PHYSICAL_DISK_PHONE_CELL_PADDING_CLASS = 'px-1!';

export const getPhysicalDiskCellPaddingClass = (layout: PhysicalDiskTableLayoutMode): string =>
  layout === 'narrow' || layout === 'compact' ? PHYSICAL_DISK_PHONE_CELL_PADDING_CLASS : '';

// Health words that do not fit a phone-width health column fall back to a
// shorter form; the full label stays in the wider projections and the detail.
export const getPhysicalDiskHealthCompactLabel = (label: string): string => {
  if (label === 'Needs Attention') return 'Attention';
  if (label === 'Replace Now') return 'Replace';
  if (label === 'Running Hot') return 'Hot';
  return label;
};

export const PHYSICAL_DISK_COL_DISK_CLASS = '';
export const PHYSICAL_DISK_COL_DEVICE_CLASS = '';
export const PHYSICAL_DISK_COL_HOST_CLASS = '';
export const PHYSICAL_DISK_COL_ROLE_CLASS = '';
export const PHYSICAL_DISK_COL_PARENT_CLASS = '';
export const PHYSICAL_DISK_COL_HEALTH_CLASS = '';
export const PHYSICAL_DISK_COL_LIFE_CLASS = '';
export const PHYSICAL_DISK_COL_TEMP_CLASS = '';
export const PHYSICAL_DISK_COL_SIZE_CLASS = '';
const PHYSICAL_DISK_CELL_DEVICE_RESPONSIVE_CLASS = '';
const PHYSICAL_DISK_CELL_HOST_RESPONSIVE_CLASS = '';
const PHYSICAL_DISK_CELL_ROLE_RESPONSIVE_CLASS = '';
const PHYSICAL_DISK_CELL_PARENT_RESPONSIVE_CLASS = '';
const PHYSICAL_DISK_CELL_LIFE_RESPONSIVE_CLASS = '';
const PHYSICAL_DISK_CELL_TEMP_RESPONSIVE_CLASS = '';
export const PHYSICAL_DISK_HEADER_DISK_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_COL_DISK_CLASS}`;
export const PHYSICAL_DISK_HEADER_DEVICE_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_CELL_DEVICE_RESPONSIVE_CLASS}`;
// Headers for responsively-hidden columns must use `table-cell` (not
// `table-column`, which is for `<col>` and does not render `<th>`
// content), otherwise the cells render but the column header is blank.
export const PHYSICAL_DISK_HEADER_HOST_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_CELL_HOST_RESPONSIVE_CLASS}`;
export const PHYSICAL_DISK_HEADER_ROLE_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_CELL_ROLE_RESPONSIVE_CLASS}`;
export const PHYSICAL_DISK_HEADER_PARENT_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_CELL_PARENT_RESPONSIVE_CLASS}`;
export const PHYSICAL_DISK_HEADER_HEALTH_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_COL_HEALTH_CLASS}`;
export const PHYSICAL_DISK_HEADER_LIFE_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_CELL_LIFE_RESPONSIVE_CLASS}`;
export const PHYSICAL_DISK_HEADER_TEMP_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_CELL_TEMP_RESPONSIVE_CLASS}`;
export const PHYSICAL_DISK_HEADER_SIZE_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_COL_SIZE_CLASS}`;
export const PHYSICAL_DISK_CELL_DISK_CLASS = `${PHYSICAL_DISK_COL_DISK_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs`;
export const PHYSICAL_DISK_CELL_DEVICE_CLASS = `${PHYSICAL_DISK_CELL_DEVICE_RESPONSIVE_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs`;
export const PHYSICAL_DISK_CELL_HOST_CLASS = `${PHYSICAL_DISK_CELL_HOST_RESPONSIVE_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs`;
export const PHYSICAL_DISK_CELL_ROLE_CLASS = `${PHYSICAL_DISK_CELL_ROLE_RESPONSIVE_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs`;
export const PHYSICAL_DISK_CELL_PARENT_CLASS = `${PHYSICAL_DISK_CELL_PARENT_RESPONSIVE_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs`;
export const PHYSICAL_DISK_CELL_HEALTH_CLASS = `${PHYSICAL_DISK_COL_HEALTH_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs`;
export const PHYSICAL_DISK_CELL_LIFE_CLASS = `${PHYSICAL_DISK_CELL_LIFE_RESPONSIVE_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs whitespace-nowrap`;
export const PHYSICAL_DISK_CELL_TEMP_CLASS = `${PHYSICAL_DISK_CELL_TEMP_RESPONSIVE_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs whitespace-nowrap`;
export const PHYSICAL_DISK_CELL_SIZE_CLASS = `${PHYSICAL_DISK_COL_SIZE_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs whitespace-nowrap`;
export const PHYSICAL_DISK_NAME_WRAP_CLASS = 'flex min-w-0 items-center gap-1.5 whitespace-nowrap';
export const PHYSICAL_DISK_NAME_TEXT_CLASS =
  'block min-w-0 truncate text-[12px] font-semibold text-base-content';
export const PHYSICAL_DISK_SOURCE_BADGE_CLASS =
  'inline-flex max-w-full min-w-0 justify-center overflow-hidden text-ellipsis px-1 sm:px-1.5 py-px text-[9px] font-medium';
export const PHYSICAL_DISK_VALUE_TEXT_CLASS = 'block truncate text-[11px] text-base-content';
export const PHYSICAL_DISK_DEVICE_TEXT_CLASS =
  'block truncate font-mono text-[11px] text-base-content';
export const PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS = 'text-[11px] text-muted';
export const PHYSICAL_DISK_LIFE_CLASS = 'text-[11px] font-medium';
export const PHYSICAL_DISK_HEALTH_WRAP_CLASS =
  'flex min-w-0 items-center gap-1.5 whitespace-nowrap';
// The health word is the row's verdict and always renders whole. Its reason
// lives in the cell title and in the disk drawer, not in a truncating sibling
// that would compete with it for the track.
export const PHYSICAL_DISK_HEALTH_LABEL_CLASS = 'shrink-0 text-[11px] font-semibold';
export const PHYSICAL_DISK_TEMPERATURE_CLASS = 'text-[11px] font-medium';
// A retained reading takes no threshold colour, so it cannot read as a disk
// running hot or cool right now. The dotted underline marks the value as one
// its title explains.
export const PHYSICAL_DISK_TEMPERATURE_LAST_KNOWN_CLASS =
  'text-muted underline decoration-dotted underline-offset-2 cursor-help';
export const PHYSICAL_DISK_SIZE_VALUE_CLASS = 'text-[11px] text-base-content';

const titleize = (value: string | undefined | null): string =>
  (value || '')
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

const UNRAID_PHYSICAL_DISK_ROLES = new Set(['data', 'parity', 'cache']);
const UNRAID_PHYSICAL_DISK_FAULT_STATES = new Set([
  'disabled',
  'invalid',
  'missing',
  'wrong',
  'error',
  'failed',
  'faulted',
]);
const PHYSICAL_DISK_BAD_HEALTH_STATES = new Set([
  'BAD',
  'CRITICAL',
  'ERROR',
  'FAILED',
  'FAIL',
  'FAULTED',
  'UNHEALTHY',
]);

// PVE reports SCSI/SAS drives as OK (ATA drives say PASSED); older server
// builds pass that raw value through, so accept it here as well (#1595).
const PHYSICAL_DISK_HEALTHY_STATES = new Set(['PASSED', 'GOOD', 'OK']);

const normalizePhysicalDiskState = (value: string | undefined | null): string =>
  (value || '').trim().toLowerCase();

const normalizePhysicalDiskHealth = (value: string | undefined | null): string =>
  (value || '').trim().toUpperCase();

export function isUnraidPhysicalDisk(disk: PhysicalDiskPresentationData): boolean {
  const storageGroup = normalizePhysicalDiskState(disk.storageGroup);
  const storageRole = normalizePhysicalDiskState(disk.storageRole);
  return (
    storageGroup === 'unraid-array' ||
    (Boolean(disk.storageState?.trim()) && UNRAID_PHYSICAL_DISK_ROLES.has(storageRole))
  );
}

export function hasUnraidPhysicalDiskFaultSignal(disk: PhysicalDiskPresentationData): boolean {
  if (!isUnraidPhysicalDisk(disk)) return false;
  if ((disk.errorCount || 0) > 0) return true;
  if (UNRAID_PHYSICAL_DISK_FAULT_STATES.has(normalizePhysicalDiskState(disk.storageState))) {
    return true;
  }
  return PHYSICAL_DISK_BAD_HEALTH_STATES.has(normalizePhysicalDiskHealth(disk.health));
}

const slugifyPhysicalDiskFacetValue = (value: string): string =>
  value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

export const DEFAULT_PHYSICAL_DISK_FACET_FILTER = 'all';
export const PHYSICAL_DISK_ALL_ROLES_FILTER_LABEL = getAllFilterOptionLabel('roles');
export const PHYSICAL_DISK_ALL_GROUPS_FILTER_LABEL = getAllFilterOptionLabel('groups');

export const normalizePhysicalDiskFacetFilter = (value: string | null | undefined): string => {
  const normalized = slugifyPhysicalDiskFacetValue(value || '');
  return normalized && normalized !== DEFAULT_PHYSICAL_DISK_FACET_FILTER
    ? normalized
    : DEFAULT_PHYSICAL_DISK_FACET_FILTER;
};

export function getPhysicalDiskPlatformLabel(_resource: Resource, fallbackLabel: string): string {
  return fallbackLabel || 'Unknown';
}

const readStringArray = (value: unknown): string[] =>
  Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
    : [];

const readPhysicalDiskSourceCandidates = (resource: Resource): string[] => {
  const directSources = readStringArray((resource as { sources?: unknown }).sources);
  const directPlatformScopes = readStringArray(
    (resource as { platformScopes?: unknown }).platformScopes,
  );
  const platformSources = readStringArray(
    (resource.platformData as { sources?: unknown } | undefined)?.sources,
  );
  const platformScopes = readStringArray(
    (resource.platformData as { platformScopes?: unknown } | undefined)?.platformScopes,
  );
  const sourceStatus = (resource.platformData as { sourceStatus?: unknown } | undefined)
    ?.sourceStatus;
  const sourceStatusSources =
    sourceStatus && typeof sourceStatus === 'object' ? Object.keys(sourceStatus) : [];

  return [
    ...directPlatformScopes,
    ...platformScopes,
    ...platformSources,
    ...directSources,
    ...sourceStatusSources,
  ];
};

export function getPhysicalDiskSourceKey(resource: Resource): string {
  const resolvedFromSources = resolvePlatformTypeFromSources(
    readPhysicalDiskSourceCandidates(resource),
  );
  return normalizeStorageSourceKey(resolvedFromSources || resource.platformType);
}

export function getPhysicalDiskSourceBadgePresentation(resource: Resource): {
  label: string;
  className: string;
} {
  const sourceKey = getPhysicalDiskSourceKey(resource);
  const presentation = getSourcePlatformPresentation(sourceKey);
  return {
    label:
      presentation?.label ||
      getPhysicalDiskPlatformLabel(resource, getSourcePlatformLabel(sourceKey)),
    className:
      `${presentation?.tone || 'text-base-content'} ${PHYSICAL_DISK_SOURCE_BADGE_CLASS}`.trim(),
  };
}

export function getPhysicalDiskHostLabel(
  disk: PhysicalDiskPresentationData,
  resource: Resource,
): string {
  return (disk.node || resource.parentName || '').trim();
}

export function extractPhysicalDiskPresentationData(
  resource: Resource,
): PhysicalDiskPresentationData {
  const pd = resource.physicalDisk || ((resource.platformData as any)?.physicalDisk ?? {});
  const diskNode = getPhysicalDiskNodeIdentity(resource);
  // A reason's code and severity decide the verdict even when it carries no
  // display text, so only the summary list drops empty summaries.
  const riskReasonDetails: ResourceStorageRiskReason[] = Array.isArray(pd.risk?.reasons)
    ? pd.risk.reasons
        .filter((reason: unknown) => reason !== null && typeof reason === 'object')
        .map((reason: { code?: unknown; severity?: unknown; summary?: unknown }) => ({
          code: typeof reason.code === 'string' ? reason.code : '',
          severity: typeof reason.severity === 'string' ? reason.severity : '',
          summary: typeof reason.summary === 'string' ? reason.summary : '',
        }))
        .filter((reason: ResourceStorageRiskReason) => reason.code || reason.summary)
    : [];
  const riskReasons = riskReasonDetails
    .map((reason) => reason.summary)
    .filter((summary) => summary.length > 0);

  return {
    node: diskNode.node,
    instance: diskNode.instance,
    devPath: pd.devPath || '',
    model: pd.model || resource.name || '',
    vendor: pd.vendor || '',
    serial: pd.serial || '',
    wwn: pd.wwn || '',
    type: pd.diskType || '',
    controller: pd.controller,
    target: pd.target,
    size: pd.sizeBytes || 0,
    health: pd.health || 'UNKNOWN',
    wearout: pd.wearout ?? -1,
    temperature: pd.temperature ?? 0,
    rpm: pd.rpm ?? 0,
    used: pd.used || '',
    storageRole: pd.storageRole,
    storageGroup: pd.storageGroup,
    storageState: pd.storageState,
    spunDown: pd.spunDown,
    readCount: pd.readCount,
    writeCount: pd.writeCount,
    errorCount: pd.errorCount,
    collection: pd.collection,
    riskLevel: pd.risk?.level,
    riskReasons,
    riskReasonDetails,
    smartAttributes: pd.smart
      ? {
          powerOnHours: pd.smart.powerOnHours,
          powerCycles: pd.smart.powerCycles,
          reallocatedSectors: pd.smart.reallocatedSectors,
          pendingSectors: pd.smart.pendingSectors,
          offlineUncorrectable: pd.smart.offlineUncorrectable,
          udmaCrcErrors: pd.smart.udmaCrcErrors,
          percentageUsed: pd.smart.percentageUsed,
          availableSpare: pd.smart.availableSpare,
          mediaErrors: pd.smart.mediaErrors,
          unsafeShutdowns: pd.smart.unsafeShutdowns,
        }
      : undefined,
  };
}

export function getPhysicalDiskFieldStatusMessage(
  label: string,
  status: PhysicalDiskFieldStatus | null | undefined,
): string {
  if (!status || status.state === 'available') return '';
  const reason = status.reason?.trim();
  switch (status.state) {
    case 'unsupported':
      return `${label} is unsupported${reason ? `: ${reason}` : '.'}`;
    case 'unavailable':
      return `${label} is temporarily unavailable${reason ? `: ${reason}` : '.'}`;
    case 'missing':
      return `${label} is unexpectedly missing${reason ? `: ${reason}` : '.'}`;
    default:
      return '';
  }
}

// Normalization may keep the last known temperature when the current
// observation is not available, such as a disk in standby or a host agent that
// stopped reporting. The value is a current reading only when its collection
// state is available, or when the source predates collection state.
export function isPhysicalDiskTemperatureCurrent(
  collection: PhysicalDiskCollectionStatus | null | undefined,
): boolean {
  const state = collection?.temperature?.state;
  return !state || state === 'available';
}

export function getPhysicalDiskLastKnownTemperatureTitle(
  status: PhysicalDiskFieldStatus | null | undefined,
): string {
  const reason = status?.reason?.trim();
  return reason ? `Last known reading, not current: ${reason}` : 'Last known reading, not current';
}

export interface PhysicalDiskTemperaturePresentation {
  label: string;
  current: boolean;
  /** Set only for a last-known reading: why the value is not current. */
  title?: string;
}

export function getPhysicalDiskTemperaturePresentation(
  disk: Pick<PhysicalDiskPresentationData, 'temperature' | 'collection'>,
): PhysicalDiskTemperaturePresentation | null {
  if (!Number.isFinite(disk.temperature) || disk.temperature <= 0) return null;
  const current = isPhysicalDiskTemperatureCurrent(disk.collection);
  return {
    label: formatTemperature(disk.temperature),
    current,
    title: current
      ? undefined
      : getPhysicalDiskLastKnownTemperatureTitle(disk.collection?.temperature),
  };
}

export function getPhysicalDiskCollectionMessages(disk: PhysicalDiskPresentationData): string[] {
  const collection = disk.collection;
  if (!collection) return [];
  return [
    getPhysicalDiskFieldStatusMessage('Serial number', collection.serial),
    getPhysicalDiskFieldStatusMessage('Temperature', collection.temperature),
    getPhysicalDiskFieldStatusMessage('Disk I/O', collection.io),
    getPhysicalDiskFieldStatusMessage('Controller association', collection.controller),
    getPhysicalDiskFieldStatusMessage('Pool membership', collection.pool),
  ].filter((message): message is string => message.length > 0);
}

export function buildPhysicalDiskPresentationDataMap(
  disks: Resource[],
): Map<string, PhysicalDiskPresentationData> {
  const map = new Map<string, PhysicalDiskPresentationData>();
  for (const disk of disks || []) {
    map.set(disk.id, extractPhysicalDiskPresentationData(disk));
  }
  return map;
}

// Disks sort in the order their verdicts ask for action: replacement first,
// then a disk running critically hot, then other warnings, then a warm disk.
const getPhysicalDiskVerdictPriority = (disk: PhysicalDiskPresentationData): number => {
  const verdict = getPhysicalDiskHealthVerdict(disk);
  switch (verdict.kind) {
    case 'replace':
      return 400;
    case 'hot':
      return verdict.critical ? 300 : 100;
    case 'attention':
      return 200;
    default:
      return 0;
  }
};

const getPhysicalDiskPriority = (disk: PhysicalDiskPresentationData): number =>
  getPhysicalDiskVerdictPriority(disk) + (hasPhysicalDiskSmartWarning(disk) ? 50 : 0);

export function matchesPhysicalDiskSearch(
  resource: Resource,
  disk: PhysicalDiskPresentationData,
  searchTerm: string,
): boolean {
  const parsed = parseStorageSearchQuery(searchTerm);
  const nodeHints = [
    disk.node,
    resource.parentName,
    resource.identity?.hostname,
    resource.canonicalIdentity?.hostname,
  ].filter((value): value is string => typeof value === 'string' && value.trim().length > 0);
  if (!matchesStorageNodeTerms(nodeHints, parsed.nodeTerms)) {
    return false;
  }
  if (parsed.freeTerms.length === 0) return true;
  const haystack = [
    disk.model,
    disk.vendor,
    disk.devPath,
    disk.serial,
    disk.wwn,
    disk.node,
    disk.instance,
    disk.type,
    disk.controller,
    disk.target,
    getPhysicalDiskRoleLabel(disk),
    getPhysicalDiskParentLabel(disk),
    getPhysicalDiskPlatformLabel(
      resource,
      getSourcePlatformLabel(getPhysicalDiskSourceKey(resource)),
    ),
  ]
    .join(' ')
    .toLowerCase();
  return parsed.freeTerms.every((term) => haystack.includes(term));
}

export function comparePhysicalDiskPresentation(
  aResource: Resource,
  aDisk: PhysicalDiskPresentationData,
  bResource: Resource,
  bDisk: PhysicalDiskPresentationData,
): number {
  const aPriority = getPhysicalDiskPriority(aDisk);
  const bPriority = getPhysicalDiskPriority(bDisk);
  if (aPriority !== bPriority) return bPriority - aPriority;
  if (aDisk.node !== bDisk.node) return aDisk.node.localeCompare(bDisk.node);
  return (aDisk.devPath || aResource.name).localeCompare(bDisk.devPath || bResource.name);
}

export function matchesPhysicalDiskFilterState(
  resource: Resource,
  disk: PhysicalDiskPresentationData,
  options: {
    sourceFilter?: string;
    healthFilter?: StorageHealthFilter;
    roleFilter?: string;
    groupFilter?: string;
    searchTerm?: string;
  },
): boolean {
  const selectedSource = normalizeStorageSourceKey(options.sourceFilter || 'all');
  if (!storageSourceMatchesFilter(getPhysicalDiskSourceKey(resource), selectedSource)) {
    return false;
  }

  const healthFilter = options.healthFilter || 'all';
  if (
    healthFilter !== 'all' &&
    !matchesPhysicalDiskHealthFilter(getPhysicalDiskNormalizedHealth(resource, disk), healthFilter)
  ) {
    return false;
  }

  const selectedRole = normalizePhysicalDiskFacetFilter(options.roleFilter);
  if (
    selectedRole !== DEFAULT_PHYSICAL_DISK_FACET_FILTER &&
    getPhysicalDiskRoleFilterValue(disk) !== selectedRole
  ) {
    return false;
  }

  const selectedGroup = normalizePhysicalDiskFacetFilter(options.groupFilter);
  if (
    selectedGroup !== DEFAULT_PHYSICAL_DISK_FACET_FILTER &&
    getPhysicalDiskGroupFilterValue(disk) !== selectedGroup
  ) {
    return false;
  }

  return matchesPhysicalDiskSearch(resource, disk, options.searchTerm || '');
}

export function filterAndSortPhysicalDisks(
  disks: Resource[],
  options: {
    selectedNode: Resource | null;
    sourceFilter?: string;
    healthFilter?: StorageHealthFilter;
    roleFilter?: string;
    groupFilter?: string;
    searchTerm: string;
    getDiskData: (disk: Resource) => PhysicalDiskPresentationData;
    matchesNode: (disk: Resource, node: { id: string; name: string; instance?: string }) => boolean;
  },
): Resource[] {
  let visibleDisks = disks || [];

  if (options.selectedNode) {
    visibleDisks = visibleDisks.filter((disk) =>
      options.matchesNode(disk, {
        id: options.selectedNode!.id,
        name: options.selectedNode!.name,
        instance: (options.selectedNode!.platformData as any)?.proxmox?.instance,
      }),
    );
  }

  visibleDisks = visibleDisks.filter((disk) =>
    matchesPhysicalDiskFilterState(disk, options.getDiskData(disk), {
      sourceFilter: options.sourceFilter,
      healthFilter: options.healthFilter,
      roleFilter: options.roleFilter,
      groupFilter: options.groupFilter,
      searchTerm: options.searchTerm,
    }),
  );

  return [...visibleDisks].sort((a, b) => {
    const aData = options.getDiskData(a);
    const bData = options.getDiskData(b);
    return comparePhysicalDiskPresentation(a, aData, b, bData);
  });
}

export function hasPhysicalDiskSmartWarning(disk: PhysicalDiskPresentationData): boolean {
  const attrs = disk.smartAttributes;
  if (!attrs) return false;
  return Boolean(
    (attrs.reallocatedSectors && attrs.reallocatedSectors > 0) ||
    (attrs.pendingSectors && attrs.pendingSectors > 0) ||
    (attrs.mediaErrors && attrs.mediaErrors > 0),
  );
}

// Heat is the one disk risk whose fix is cooling, not a replacement disk.
// Alerts raise it as a temperature metric rather than as disk health, so the
// verdict names it apart from the evidence that does mean replacing the disk.
const PHYSICAL_DISK_TEMPERATURE_RISK_CODE = 'temperature_high';

const PHYSICAL_DISK_RISK_SEVERITY_RANK: Record<string, number> = {
  monitor: 1,
  warning: 2,
  critical: 3,
};

const getPhysicalDiskRiskSeverityRank = (severity: string | undefined): number =>
  PHYSICAL_DISK_RISK_SEVERITY_RANK[(severity || '').trim().toLowerCase()] ?? 0;

interface PhysicalDiskRiskEvidence {
  rank: number;
  summary?: string;
}

// Splits the disk's risk into heat and everything else, keeping the most
// severe reason of each. A risk level that none of its reasons explains, or
// one sent without reason codes, stays with the disk-health evidence.
function splitPhysicalDiskRisk(disk: PhysicalDiskPresentationData): {
  health: PhysicalDiskRiskEvidence;
  heat: PhysicalDiskRiskEvidence;
} {
  const health: PhysicalDiskRiskEvidence = { rank: 0 };
  const heat: PhysicalDiskRiskEvidence = { rank: 0 };
  const details =
    disk.riskReasonDetails ??
    disk.riskReasons.map((summary) => ({ code: '', severity: disk.riskLevel || '', summary }));
  for (const reason of details) {
    const evidence = reason.code === PHYSICAL_DISK_TEMPERATURE_RISK_CODE ? heat : health;
    const rank = getPhysicalDiskRiskSeverityRank(reason.severity);
    if (rank > evidence.rank) {
      evidence.rank = rank;
      evidence.summary = reason.summary || undefined;
    } else if (rank === evidence.rank && !evidence.summary) {
      evidence.summary = reason.summary || undefined;
    }
  }
  const levelRank = getPhysicalDiskRiskSeverityRank(disk.riskLevel);
  if (levelRank > Math.max(health.rank, heat.rank)) {
    // A reason with a known, lower severity does not explain this level, so
    // its text must not stand in as the verdict's reason.
    if (health.rank > 0) health.summary = undefined;
    health.rank = levelRank;
  }
  return { health, heat };
}

type PhysicalDiskHealthVerdict =
  | { kind: 'replace'; summary: string }
  | { kind: 'hot'; critical: boolean; summary: string }
  | { kind: 'attention'; summary: string }
  | { kind: 'clear' };

function getPhysicalDiskHealthVerdict(
  disk: PhysicalDiskPresentationData,
): PhysicalDiskHealthVerdict {
  const { health, heat } = splitPhysicalDiskRisk(disk);
  const critical = PHYSICAL_DISK_RISK_SEVERITY_RANK.critical;
  const warning = PHYSICAL_DISK_RISK_SEVERITY_RANK.warning;
  const lowLife = isPhysicalDiskWearoutReported(disk) && disk.wearout < 10;

  if (normalizePhysicalDiskHealth(disk.health) === 'FAILED' || health.rank >= critical) {
    return {
      kind: 'replace',
      summary: health.summary || 'Disk health has degraded to a critical state.',
    };
  }
  if (heat.rank >= critical) {
    return { kind: 'hot', critical: true, summary: heat.summary || 'Disk temperature is high.' };
  }
  if (health.rank >= warning || hasPhysicalDiskSmartWarning(disk) || lowLife) {
    return {
      kind: 'attention',
      summary:
        health.summary ||
        (lowLife ? 'SSD life is running low.' : 'SMART counters indicate elevated risk.'),
    };
  }
  if (heat.rank >= warning) {
    return { kind: 'hot', critical: false, summary: heat.summary || 'Disk temperature is high.' };
  }
  return { kind: 'clear' };
}

const PHYSICAL_DISK_CRITICAL_TONE = 'text-red-700 dark:text-red-300';
const PHYSICAL_DISK_WARNING_TONE = 'text-amber-700 dark:text-amber-300';

export function getPhysicalDiskHealthStatus(
  disk: PhysicalDiskPresentationData,
): DiskHealthStatusPresentation {
  const verdict = getPhysicalDiskHealthVerdict(disk);

  if (verdict.kind === 'replace') {
    return { label: 'Replace Now', summary: verdict.summary, tone: PHYSICAL_DISK_CRITICAL_TONE };
  }

  if (verdict.kind === 'hot') {
    return {
      label: 'Running Hot',
      summary: verdict.summary,
      tone: verdict.critical ? PHYSICAL_DISK_CRITICAL_TONE : PHYSICAL_DISK_WARNING_TONE,
    };
  }

  if (verdict.kind === 'attention') {
    return { label: 'Needs Attention', summary: verdict.summary, tone: PHYSICAL_DISK_WARNING_TONE };
  }

  if (isUnraidPhysicalDisk(disk) && !hasUnraidPhysicalDiskFaultSignal(disk)) {
    return {
      label: normalizePhysicalDiskState(disk.storageState) === 'online' ? 'Online' : 'Unknown',
      summary: 'No active disk-health issues.',
      tone: 'text-base-content',
    };
  }

  const isHealthy = PHYSICAL_DISK_HEALTHY_STATES.has(normalizePhysicalDiskHealth(disk.health));
  return {
    label: isHealthy ? 'Healthy' : 'Unknown',
    summary: isHealthy ? 'No active disk-health issues.' : 'Health state is not reported.',
    tone: isHealthy ? 'text-emerald-700 dark:text-emerald-300' : 'text-base-content',
  };
}

export function getPhysicalDiskNormalizedHealth(
  resource: Resource,
  disk: PhysicalDiskPresentationData,
): NormalizedHealth {
  const verdict = getPhysicalDiskHealthVerdict(disk);
  const clearLabel = verdict.kind === 'clear' ? getPhysicalDiskHealthStatus(disk).label : null;
  if (clearLabel === 'Online') return 'healthy';
  if (resource.status === 'offline') return 'offline';
  if (verdict.kind === 'replace') return 'critical';
  // A disk running hot sits in the health filter at its temperature tier.
  if (verdict.kind === 'hot') return verdict.critical ? 'critical' : 'warning';
  if (verdict.kind === 'attention') return 'warning';
  return clearLabel === 'Healthy' ? 'healthy' : 'unknown';
}

export function matchesPhysicalDiskHealthFilter(
  health: NormalizedHealth,
  filter: StorageHealthFilter,
): boolean {
  if (filter === 'all') return true;
  if (filter === 'attention') {
    return health === 'warning' || health === 'critical' || health === 'offline';
  }
  return health === filter;
}

export function getPhysicalDiskHealthSummary(status: DiskHealthStatusPresentation): string {
  const summary = status.summary?.trim() || '';
  if (!summary || summary === 'No active disk-health issues.') {
    return '';
  }
  return summary;
}

export function getPhysicalDiskRoleLabel(disk: PhysicalDiskPresentationData): string {
  if (disk.storageRole?.trim()) return titleize(disk.storageRole);
  const normalizedType = disk.type?.trim().toLowerCase();
  if (normalizedType === 'nvme') return 'NVMe disk';
  if (normalizedType === 'sata') return 'SATA disk';
  if (normalizedType === 'sas') return 'SAS disk';
  if (normalizedType === 'ssd') return 'SSD';
  if (normalizedType === 'hdd') return 'HDD';
  if (normalizedType) return `${titleize(normalizedType)} disk`;
  return '';
}

export function getPhysicalDiskParentLabel(disk: PhysicalDiskPresentationData): string {
  if (disk.storageGroup?.trim()) return disk.storageGroup.trim();
  // Proxmox reports a usage string ("ZFS", "ext4", "BIOS boot", "LVM", ...)
  // for disks it cannot map to a named pool; better than an empty cell.
  const used = disk.used?.trim() || '';
  if (used && used.toLowerCase() !== 'unknown') return used;
  return '';
}

// Mirrors storagehealth.WearoutReported on the backend. Wearout is "% life
// remaining" (100 = new). -1 is the unreported sentinel, and 0 is a real
// reading only from a device that reports endurance at all, so a 0 from a
// rotational disk is absent evidence rather than a spent disk.
export function isPhysicalDiskWearoutReported(disk: PhysicalDiskPresentationData): boolean {
  if (typeof disk.wearout !== 'number') return false;
  if (disk.wearout > 0) return true;
  const normalizedType = (disk.type || '').trim().toLowerCase();
  return disk.wearout === 0 && (normalizedType === 'nvme' || normalizedType === 'ssd');
}

export function getPhysicalDiskLifeLabel(disk: PhysicalDiskPresentationData): string {
  if (typeof disk.wearout !== 'number' || disk.wearout < 0) return '';
  return `${Math.min(disk.wearout, 100)}%`;
}

export function getPhysicalDiskLifeTextClass(disk: PhysicalDiskPresentationData): string {
  if (!isPhysicalDiskWearoutReported(disk)) {
    return PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS;
  }
  if (disk.wearout < 20) return 'text-red-600 dark:text-red-400';
  if (disk.wearout < 50) return 'text-amber-600 dark:text-amber-400';
  return 'text-green-600 dark:text-green-400';
}

export function getPhysicalDiskRoleFilterValue(disk: PhysicalDiskPresentationData): string {
  return normalizePhysicalDiskFacetFilter(getPhysicalDiskRoleLabel(disk));
}

export function getPhysicalDiskGroupFilterValue(disk: PhysicalDiskPresentationData): string {
  return normalizePhysicalDiskFacetFilter(getPhysicalDiskParentLabel(disk));
}

const buildPhysicalDiskFacetOptions = (
  disks: Resource[],
  allLabel: string,
  getLabel: (disk: PhysicalDiskPresentationData) => string,
): PhysicalDiskFilterOption[] => {
  const byValue = new Map<string, string>();
  for (const resource of disks || []) {
    const label = getLabel(extractPhysicalDiskPresentationData(resource)).trim();
    if (!label) continue;
    byValue.set(normalizePhysicalDiskFacetFilter(label), label);
  }

  return [
    { value: DEFAULT_PHYSICAL_DISK_FACET_FILTER, label: allLabel },
    ...Array.from(byValue.entries())
      .sort(([, labelA], [, labelB]) => labelA.localeCompare(labelB))
      .map(([value, label]) => ({ value, label })),
  ];
};

export const buildPhysicalDiskRoleFilterOptions = (disks: Resource[]): PhysicalDiskFilterOption[] =>
  buildPhysicalDiskFacetOptions(
    disks,
    PHYSICAL_DISK_ALL_ROLES_FILTER_LABEL,
    getPhysicalDiskRoleLabel,
  );

export const buildPhysicalDiskGroupFilterOptions = (
  disks: Resource[],
): PhysicalDiskFilterOption[] =>
  buildPhysicalDiskFacetOptions(
    disks,
    PHYSICAL_DISK_ALL_GROUPS_FILTER_LABEL,
    getPhysicalDiskParentLabel,
  );

const getPhysicalDiskHealthFilterEmptyTitle = (filter: StorageHealthFilter): string | null => {
  switch (filter) {
    case 'attention':
      return 'No disks need attention';
    case 'healthy':
      return 'No healthy disks found';
    case 'warning':
      return 'No warning disks found';
    case 'critical':
      return 'No critical disks found';
    case 'offline':
      return 'No offline disks found';
    case 'unknown':
      return 'No disks with unknown health';
    default:
      return null;
  }
};

export function getPhysicalDiskEmptyStatePresentation(options: {
  selectedNodeName: string | null;
  searchTerm: string;
  diskCount: number;
  hasPVENodes: boolean;
  healthFilter?: StorageHealthFilter;
  sourceFilterLabel?: string | null;
  roleFilterLabel?: string | null;
  groupFilterLabel?: string | null;
}): PhysicalDiskEmptyStatePresentation {
  const healthFilter = options.healthFilter || 'all';
  const hasScopedFilter = Boolean(
    options.searchTerm ||
    options.sourceFilterLabel ||
    options.roleFilterLabel ||
    options.groupFilterLabel ||
    healthFilter !== 'all',
  );
  const filterMessages = [
    options.sourceFilterLabel ? `from ${options.sourceFilterLabel}` : null,
    options.roleFilterLabel ? `with role ${options.roleFilterLabel}` : null,
    options.groupFilterLabel ? `in ${options.groupFilterLabel}` : null,
  ].filter((message): message is string => Boolean(message));

  return {
    title:
      options.diskCount === 0
        ? 'No physical disks found'
        : getPhysicalDiskHealthFilterEmptyTitle(healthFilter) || 'No disks match these filters',
    nodeMessage: options.selectedNodeName ? `for node ${options.selectedNodeName}` : null,
    searchMessage: options.searchTerm ? `matching "${options.searchTerm}"` : null,
    filterMessages,
    showRequirements: !hasScopedFilter && options.diskCount === 0 && options.hasPVENodes,
    fallbackMessage: `No Proxmox nodes configured. Add Proxmox VE in ${getInfrastructureSettingsLocationLabel()} to monitor physical disks.`,
    requirementsTitle: 'Physical disk monitoring requirements:',
    requirementsItems: [
      `Enable "Monitor physical disk health (SMART)" in ${getInfrastructureSettingsLocationLabel()} for the Proxmox node`,
      'Enable SMART monitoring in Proxmox VE at Datacenter → Node → System → Advanced → "Monitor physical disk health"',
      'Wait 5 minutes for Proxmox to collect SMART data',
    ],
    requirementsNote: 'Note: Both Pulse and Proxmox must have SMART monitoring enabled.',
  };
}
