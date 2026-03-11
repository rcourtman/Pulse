import type { Resource } from '@/types/resource';
import { getSourcePlatformLabel, getSourcePlatformPresentation } from '@/utils/sourcePlatforms';
import { getPhysicalDiskNodeIdentity } from '@/components/Storage/diskResourceUtils';

export interface DiskHealthStatusPresentation {
  label: string;
  summary: string;
  tone: string;
}

export interface PhysicalDiskEmptyStatePresentation {
  title: string;
  nodeMessage: string | null;
  searchMessage: string | null;
  showRequirements: boolean;
  fallbackMessage: string;
  requirementsTitle: string;
  requirementsItems: string[];
  requirementsNote: string;
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

export const PHYSICAL_DISK_TABLE_SCROLL_CLASS = 'overflow-x-auto';
export const PHYSICAL_DISK_TABLE_CLASS = 'w-full text-xs';
export const PHYSICAL_DISK_TABLE_HEADER_ROW_CLASS = 'border-b border-border bg-surface-alt text-muted';
export const PHYSICAL_DISK_TABLE_BODY_CLASS = 'divide-y divide-border';
export const PHYSICAL_DISK_TABLE_ROW_CLASS = 'cursor-pointer transition-colors';
export const PHYSICAL_DISK_TABLE_ROW_SELECTED_CLASS = 'bg-blue-50 dark:bg-blue-900';
export const PHYSICAL_DISK_TABLE_ROW_HOVER_CLASS = 'hover:bg-surface-hover';
export const PHYSICAL_DISK_TABLE_ROW_STYLE = { height: '38px' } as const;
export const PHYSICAL_DISK_EXPAND_BUTTON_CLASS = 'rounded p-1 hover:bg-surface-hover transition-colors';
export const PHYSICAL_DISK_DETAIL_ROW_CELL_CLASS =
  'border-b border-border-subtle bg-surface-alt px-4 py-4 shadow-inner';
export const PHYSICAL_DISK_HEADER_DISK_CLASS =
  'px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider md:min-w-[220px]';
export const PHYSICAL_DISK_HEADER_SOURCE_CLASS =
  'px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[72px]';
export const PHYSICAL_DISK_HEADER_HOST_CLASS =
  'px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider md:min-w-[120px]';
export const PHYSICAL_DISK_HEADER_ROLE_CLASS =
  'hidden xl:table-cell px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider';
export const PHYSICAL_DISK_HEADER_PARENT_CLASS = PHYSICAL_DISK_HEADER_ROLE_CLASS;
export const PHYSICAL_DISK_HEADER_HEALTH_CLASS =
  'px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider md:min-w-[160px]';
export const PHYSICAL_DISK_HEADER_TEMP_CLASS =
  'hidden md:table-cell px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[72px]';
export const PHYSICAL_DISK_HEADER_SIZE_CLASS =
  'px-1.5 sm:px-2 py-0.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[96px]';
export const PHYSICAL_DISK_HEADER_EXPAND_CLASS = 'px-1.5 py-1 w-10';
export const PHYSICAL_DISK_CELL_DISK_CLASS = 'px-1.5 sm:px-2 py-1 align-middle text-xs md:min-w-[220px]';
export const PHYSICAL_DISK_CELL_SOURCE_CLASS = 'px-1.5 sm:px-2 py-1 align-middle text-xs w-[72px]';
export const PHYSICAL_DISK_CELL_HOST_CLASS = 'px-1.5 sm:px-2 py-1 align-middle text-xs md:min-w-[120px]';
export const PHYSICAL_DISK_CELL_ROLE_CLASS = 'hidden xl:table-cell px-1.5 sm:px-2 py-1 align-middle text-xs';
export const PHYSICAL_DISK_CELL_PARENT_CLASS = PHYSICAL_DISK_CELL_ROLE_CLASS;
export const PHYSICAL_DISK_CELL_HEALTH_CLASS = 'px-1.5 sm:px-2 py-1 align-middle text-xs md:min-w-[160px]';
export const PHYSICAL_DISK_CELL_TEMP_CLASS =
  'hidden md:table-cell px-1.5 sm:px-2 py-1 align-middle text-xs whitespace-nowrap w-[72px]';
export const PHYSICAL_DISK_CELL_SIZE_CLASS =
  'px-1.5 sm:px-2 py-1 align-middle text-xs whitespace-nowrap w-[96px]';
export const PHYSICAL_DISK_CELL_EXPAND_CLASS = 'px-1.5 py-1 align-middle text-right';
export const PHYSICAL_DISK_NAME_WRAP_CLASS = 'flex min-w-0 items-center whitespace-nowrap';
export const PHYSICAL_DISK_NAME_TEXT_CLASS = 'block min-w-0 truncate text-[12px] font-semibold text-base-content';
export const PHYSICAL_DISK_SOURCE_BADGE_CLASS =
  'inline-flex min-w-[3.25rem] justify-center px-1.5 py-px text-[9px] font-medium';
export const PHYSICAL_DISK_VALUE_TEXT_CLASS = 'block truncate text-[11px] text-base-content';
export const PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS = 'text-[11px] text-muted';
export const PHYSICAL_DISK_HEALTH_WRAP_CLASS = 'flex min-w-0 items-center gap-1.5 whitespace-nowrap';
export const PHYSICAL_DISK_HEALTH_LABEL_CLASS = 'shrink-0 text-[11px] font-semibold';
export const PHYSICAL_DISK_HEALTH_SUMMARY_CLASS = 'hidden xl:block truncate text-[11px] text-muted';
export const PHYSICAL_DISK_TEMPERATURE_CLASS = 'text-[11px] font-medium';
export const PHYSICAL_DISK_SIZE_VALUE_CLASS = 'text-[11px] text-base-content';
export const PHYSICAL_DISK_EXPAND_ICON_BASE_CLASS =
  'h-3.5 w-3.5 text-muted transition-transform duration-150';

const titleize = (value: string | undefined | null): string =>
  (value || '')
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

export function getPhysicalDiskPlatformLabel(resource: Resource, fallbackLabel: string): string {
  return fallbackLabel || 'Unknown';
}

export function getPhysicalDiskSourceBadgePresentation(resource: Resource): {
  label: string;
  className: string;
} {
  const presentation = getSourcePlatformPresentation(resource.platformType);
  return {
    label: presentation?.label || getPhysicalDiskPlatformLabel(resource, getSourcePlatformLabel(resource.platformType)),
    className: `${presentation?.tone || 'text-base-content'} ${PHYSICAL_DISK_SOURCE_BADGE_CLASS}`.trim(),
  };
}

export function getPhysicalDiskExpandIconClass(expanded: boolean): string {
  return `${PHYSICAL_DISK_EXPAND_ICON_BASE_CLASS} ${expanded ? 'rotate-90' : ''}`.trim();
}

export function getPhysicalDiskHostLabel(
  disk: PhysicalDiskPresentationData,
  resource: Resource,
): string {
  return (disk.node || resource.parentName || '').trim();
}

export function extractPhysicalDiskPresentationData(resource: Resource): PhysicalDiskPresentationData {
  const pd = resource.physicalDisk || ((resource.platformData as any)?.physicalDisk ?? {});
  const diskNode = getPhysicalDiskNodeIdentity(resource);
  const riskReasons = Array.isArray(pd.risk?.reasons)
    ? pd.risk.reasons
        .map((reason) => reason?.summary)
        .filter((summary): summary is string => typeof summary === 'string' && summary.length > 0)
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
  const term = searchTerm.toLowerCase();
  return [
    disk.model,
    disk.devPath,
    disk.serial,
    disk.node,
    getPhysicalDiskRoleLabel(disk),
    getPhysicalDiskParentLabel(disk),
    getPhysicalDiskPlatformLabel(resource, getSourcePlatformLabel(resource.platformType) || ''),
  ]
    .join(' ')
    .toLowerCase()
    .includes(term);
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

export function filterAndSortPhysicalDisks(
  disks: Resource[],
  options: {
    selectedNode: Resource | null;
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

  if (options.searchTerm) {
    visibleDisks = visibleDisks.filter((disk) =>
      matchesPhysicalDiskSearch(disk, options.getDiskData(disk), options.searchTerm),
    );
  }

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
    label: normalizedHealth === 'PASSED' || normalizedHealth === 'GOOD' ? 'Healthy' : 'Monitor',
    summary: 'No active disk-health issues.',
    tone: 'text-base-content',
  };
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
  if (disk.type?.trim()) return `${disk.type.toUpperCase()} Disk`;
  return '';
}

export function getPhysicalDiskParentLabel(disk: PhysicalDiskPresentationData): string {
  if (disk.storageGroup?.trim()) return disk.storageGroup.trim();
  return '';
}

export function getPhysicalDiskEmptyStatePresentation(options: {
  selectedNodeName: string | null;
  searchTerm: string;
  diskCount: number;
  hasPVENodes: boolean;
}): PhysicalDiskEmptyStatePresentation {
  return {
    title: 'No physical disks found',
    nodeMessage: options.selectedNodeName ? `for node ${options.selectedNodeName}` : null,
    searchMessage: options.searchTerm ? `matching "${options.searchTerm}"` : null,
    showRequirements: !options.searchTerm && options.diskCount === 0 && options.hasPVENodes,
    fallbackMessage:
      'No Proxmox nodes configured. Add a Proxmox VE cluster in Settings to monitor physical disks.',
    requirementsTitle: 'Physical disk monitoring requirements:',
    requirementsItems: [
      'Enable "Monitor physical disk health (SMART)" in Settings → Infrastructure (Proxmox node advanced settings)',
      'Enable SMART monitoring in Proxmox VE at Datacenter → Node → System → Advanced → "Monitor physical disk health"',
      'Wait 5 minutes for Proxmox to collect SMART data',
    ],
    requirementsNote: 'Note: Both Pulse and Proxmox must have SMART monitoring enabled.',
  };
}
