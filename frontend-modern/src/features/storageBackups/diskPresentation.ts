import type { Resource } from '@/types/resource';
import {
  getSourcePlatformLabel,
  getSourcePlatformPresentation,
  resolvePlatformTypeFromSources,
} from '@/utils/sourcePlatforms';
import { getAllFilterOptionLabel } from '@/components/shared/filterOptionPresentation';
import { getPhysicalDiskNodeIdentity } from '@/components/Storage/diskResourceUtils';
import { getInfrastructureSettingsLocationLabel } from '@/utils/infrastructureSettingsPresentation';
import { normalizeStorageSourceKey } from '@/utils/storageSources';
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
  serial: string;
  wwn: string;
  size: number;
  health: string;
  riskLevel?: string;
  riskReasons: string[];
  wearout: number;
  storageRole?: string;
  storageGroup?: string;
  type: string;
  temperature: number;
  rpm: number;
  used: string;
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
  'mt-4 rounded-md border border-blue-200 bg-blue-50 p-4 text-left dark:border-blue-800 dark:bg-blue-900';
export const PHYSICAL_DISK_EMPTY_REQUIREMENTS_TITLE_CLASS =
  'mb-2 text-sm font-medium text-blue-900 dark:text-blue-100';
export const PHYSICAL_DISK_EMPTY_REQUIREMENTS_LIST_CLASS =
  'ml-4 list-decimal space-y-1.5 text-xs text-blue-800 dark:text-blue-200';
export const PHYSICAL_DISK_EMPTY_REQUIREMENTS_NOTE_CLASS =
  'mt-3 text-xs italic text-blue-700 dark:text-blue-300';

const PHYSICAL_DISK_TABLE_HEADER_CLASS =
  'overflow-hidden text-ellipsis whitespace-nowrap px-1 sm:px-1.5 lg:px-2 py-0.5 text-left text-[10px] sm:text-[11px] lg:text-xs font-medium uppercase tracking-wider';

export const PHYSICAL_DISK_TABLE_CLASS = 'w-full table-fixed text-xs';
export const PHYSICAL_DISK_TABLE_HEADER_ROW_CLASS =
  'border-b border-border bg-surface-alt text-muted';
export const PHYSICAL_DISK_TABLE_BODY_CLASS = 'divide-y divide-border';
export const PHYSICAL_DISK_TABLE_ROW_CLASS = 'cursor-pointer transition-colors';
export const PHYSICAL_DISK_TABLE_ROW_SELECTED_CLASS = 'bg-blue-50 dark:bg-blue-900';
export const PHYSICAL_DISK_TABLE_ROW_HOVER_CLASS = 'hover:bg-surface-hover';
export const PHYSICAL_DISK_TABLE_ROW_STYLE = { height: '38px' } as const;
export const PHYSICAL_DISK_DETAIL_ROW_CELL_CLASS =
  'border-b border-border-subtle bg-surface-alt px-4 py-4 shadow-inner';
export const PHYSICAL_DISK_COL_DISK_CLASS = 'w-[42%] sm:w-[28%] md:w-[28%] xl:w-[22%]';
export const PHYSICAL_DISK_COL_SOURCE_CLASS = 'w-[15%] sm:w-[11%] md:w-[9%] xl:w-[8%]';
export const PHYSICAL_DISK_COL_HOST_CLASS =
  'hidden sm:table-column sm:w-[18%] md:w-[14%] xl:w-[12%]';
export const PHYSICAL_DISK_COL_ROLE_CLASS = 'hidden xl:table-column xl:w-[10%]';
export const PHYSICAL_DISK_COL_PARENT_CLASS = 'hidden xl:table-column xl:w-[13%]';
export const PHYSICAL_DISK_COL_HEALTH_CLASS = 'w-[25%] sm:w-[24%] md:w-[20%] xl:w-[16%]';
export const PHYSICAL_DISK_COL_TEMP_CLASS = 'hidden md:table-column md:w-[9%] xl:w-[7%]';
export const PHYSICAL_DISK_COL_SIZE_CLASS = 'w-[18%] sm:w-[19%] md:w-[20%] xl:w-[12%]';
const PHYSICAL_DISK_CELL_HOST_RESPONSIVE_CLASS =
  'hidden sm:table-cell sm:w-[18%] md:w-[14%] xl:w-[12%]';
const PHYSICAL_DISK_CELL_ROLE_RESPONSIVE_CLASS = 'hidden xl:table-cell xl:w-[10%]';
const PHYSICAL_DISK_CELL_PARENT_RESPONSIVE_CLASS = 'hidden xl:table-cell xl:w-[13%]';
const PHYSICAL_DISK_CELL_TEMP_RESPONSIVE_CLASS = 'hidden md:table-cell md:w-[9%] xl:w-[7%]';
export const PHYSICAL_DISK_HEADER_DISK_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_COL_DISK_CLASS}`;
export const PHYSICAL_DISK_HEADER_SOURCE_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_COL_SOURCE_CLASS}`;
export const PHYSICAL_DISK_HEADER_HOST_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_COL_HOST_CLASS}`;
export const PHYSICAL_DISK_HEADER_ROLE_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_COL_ROLE_CLASS}`;
export const PHYSICAL_DISK_HEADER_PARENT_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_COL_PARENT_CLASS}`;
export const PHYSICAL_DISK_HEADER_HEALTH_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_COL_HEALTH_CLASS}`;
export const PHYSICAL_DISK_HEADER_TEMP_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_COL_TEMP_CLASS}`;
export const PHYSICAL_DISK_HEADER_SIZE_CLASS = `${PHYSICAL_DISK_TABLE_HEADER_CLASS} ${PHYSICAL_DISK_COL_SIZE_CLASS}`;
export const PHYSICAL_DISK_CELL_DISK_CLASS = `${PHYSICAL_DISK_COL_DISK_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs`;
export const PHYSICAL_DISK_CELL_SOURCE_CLASS = `${PHYSICAL_DISK_COL_SOURCE_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs`;
export const PHYSICAL_DISK_CELL_HOST_CLASS = `${PHYSICAL_DISK_CELL_HOST_RESPONSIVE_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs`;
export const PHYSICAL_DISK_CELL_ROLE_CLASS = `${PHYSICAL_DISK_CELL_ROLE_RESPONSIVE_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs`;
export const PHYSICAL_DISK_CELL_PARENT_CLASS = `${PHYSICAL_DISK_CELL_PARENT_RESPONSIVE_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs`;
export const PHYSICAL_DISK_CELL_HEALTH_CLASS = `${PHYSICAL_DISK_COL_HEALTH_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs`;
export const PHYSICAL_DISK_CELL_TEMP_CLASS = `${PHYSICAL_DISK_CELL_TEMP_RESPONSIVE_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs whitespace-nowrap`;
export const PHYSICAL_DISK_CELL_SIZE_CLASS = `${PHYSICAL_DISK_COL_SIZE_CLASS} overflow-hidden px-1 sm:px-1.5 lg:px-2 py-1 align-middle text-xs whitespace-nowrap`;
export const PHYSICAL_DISK_NAME_WRAP_CLASS = 'flex min-w-0 items-center gap-1.5 whitespace-nowrap';
export const PHYSICAL_DISK_NAME_TEXT_CLASS =
  'block min-w-0 truncate text-[12px] font-semibold text-base-content';
export const PHYSICAL_DISK_SOURCE_BADGE_CLASS =
  'inline-flex max-w-full min-w-0 justify-center overflow-hidden text-ellipsis px-1 sm:px-1.5 py-px text-[9px] font-medium';
export const PHYSICAL_DISK_VALUE_TEXT_CLASS = 'block truncate text-[11px] text-base-content';
export const PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS = 'text-[11px] text-muted';
export const PHYSICAL_DISK_HEALTH_WRAP_CLASS =
  'flex min-w-0 items-center gap-1.5 whitespace-nowrap';
export const PHYSICAL_DISK_HEALTH_LABEL_CLASS = 'min-w-0 truncate text-[11px] font-semibold';
export const PHYSICAL_DISK_HEALTH_SUMMARY_CLASS = 'hidden xl:block truncate text-[11px] text-muted';
export const PHYSICAL_DISK_TEMPERATURE_CLASS = 'text-[11px] font-medium';
export const PHYSICAL_DISK_SIZE_VALUE_CLASS = 'text-[11px] text-base-content';

const titleize = (value: string | undefined | null): string =>
  (value || '')
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

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
  const platformSources = readStringArray(
    (resource.platformData as { sources?: unknown } | undefined)?.sources,
  );
  const sourceStatus = (resource.platformData as { sourceStatus?: unknown } | undefined)
    ?.sourceStatus;
  const sourceStatusSources =
    sourceStatus && typeof sourceStatus === 'object' ? Object.keys(sourceStatus) : [];

  return [...platformSources, ...directSources, ...sourceStatusSources];
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
  const riskReasons = Array.isArray(pd.risk?.reasons)
    ? pd.risk.reasons
        .map((reason: { summary?: unknown }) => reason?.summary)
        .filter(
          (summary: unknown): summary is string =>
            typeof summary === 'string' && summary.length > 0,
        )
    : [];

  return {
    node: diskNode.node,
    instance: diskNode.instance,
    devPath: pd.devPath || '',
    model: pd.model || resource.name || '',
    serial: pd.serial || '',
    wwn: pd.wwn || '',
    type: pd.diskType || '',
    size: pd.sizeBytes || 0,
    health: pd.health || 'UNKNOWN',
    wearout: pd.wearout ?? -1,
    temperature: pd.temperature ?? 0,
    rpm: pd.rpm ?? 0,
    used: pd.used || '',
    storageRole: pd.storageRole,
    storageGroup: pd.storageGroup,
    riskLevel: pd.risk?.level,
    riskReasons,
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

export function buildPhysicalDiskPresentationDataMap(
  disks: Resource[],
): Map<string, PhysicalDiskPresentationData> {
  const map = new Map<string, PhysicalDiskPresentationData>();
  for (const disk of disks || []) {
    map.set(disk.id, extractPhysicalDiskPresentationData(disk));
  }
  return map;
}

const getPhysicalDiskPriority = (disk: PhysicalDiskPresentationData): number =>
  (disk.riskLevel === 'critical' ? 300 : disk.riskLevel === 'warning' ? 200 : 0) +
  (hasPhysicalDiskSmartWarning(disk) ? 50 : 0);

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
    disk.devPath,
    disk.serial,
    disk.node,
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
  if (selectedSource !== 'all' && getPhysicalDiskSourceKey(resource) !== selectedSource) {
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

export function getPhysicalDiskHealthStatus(
  disk: PhysicalDiskPresentationData,
): DiskHealthStatusPresentation {
  const normalizedHealth = (disk.health || '').trim().toUpperCase();
  const criticalRisk = (disk.riskLevel || '').trim().toLowerCase() === 'critical';
  const warningRisk = (disk.riskLevel || '').trim().toLowerCase() === 'warning';
  const smartWarning = hasPhysicalDiskSmartWarning(disk);
  const lowLife = disk.wearout > 0 && disk.wearout < 10;

  if (normalizedHealth === 'FAILED' || criticalRisk) {
    return {
      label: 'Replace Now',
      summary: disk.riskReasons[0] || 'Disk health has degraded to a critical state.',
      tone: 'text-red-700 dark:text-red-300',
    };
  }

  if (warningRisk || smartWarning || lowLife) {
    return {
      label: 'Needs Attention',
      summary:
        disk.riskReasons[0] ||
        (lowLife ? 'SSD life is running low.' : 'SMART counters indicate elevated risk.'),
      tone: 'text-amber-700 dark:text-amber-300',
    };
  }

  return {
    label: normalizedHealth === 'PASSED' || normalizedHealth === 'GOOD' ? 'Healthy' : 'Unknown',
    summary:
      normalizedHealth === 'PASSED' || normalizedHealth === 'GOOD'
        ? 'No active disk-health issues.'
        : 'Health state is not reported.',
    tone: 'text-base-content',
  };
}

export function getPhysicalDiskNormalizedHealth(
  resource: Resource,
  disk: PhysicalDiskPresentationData,
): NormalizedHealth {
  if (resource.status === 'offline') return 'offline';
  const status = getPhysicalDiskHealthStatus(disk).label;
  if (status === 'Replace Now') return 'critical';
  if (status === 'Needs Attention') return 'warning';
  if (status === 'Healthy') return 'healthy';
  return 'unknown';
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
  return '';
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
