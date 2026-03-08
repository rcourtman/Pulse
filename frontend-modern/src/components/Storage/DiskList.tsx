import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import { Card } from '@/components/shared/Card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { formatBytes, formatPowerOnHours } from '@/utils/format';
import { formatTemperature } from '@/utils/temperature';
import type { Resource } from '@/types/resource';
import { getProxmoxData } from '@/utils/resourcePlatformData';
import { DiskDetail } from './DiskDetail';
import { getPhysicalDiskNodeIdentity, matchesPhysicalDiskNode } from './diskResourceUtils';

interface PhysicalDiskData {
  node: string;
  instance: string;
  devPath: string;
  model: string;
  serial: string;
  wwn: string;
  type: string;
  size: number;
  health: string;
  wearout: number;
  temperature: number;
  rpm: number;
  used: string;
  storageRole?: string;
  storageGroup?: string;
  riskLevel?: string;
  riskReasons: string[];
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

interface DiskListProps {
  disks: Resource[];
  nodes: Resource[];
  selectedNode: string | null;
  searchTerm: string;
}

const titleize = (value: string | undefined | null): string =>
  (value || '')
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

const platformLabel = (resource: Resource): string => {
  switch ((resource.platformType || '').trim().toLowerCase()) {
    case 'proxmox-pve':
      return 'PVE';
    case 'proxmox-pbs':
      return 'PBS';
    case 'truenas':
      return 'TrueNAS';
    case 'agent':
      return 'Agent';
    default:
      return titleize(resource.platformType) || 'Unknown';
  }
};

const platformTextClass = (resource: Resource): string => {
  switch ((resource.platformType || '').trim().toLowerCase()) {
    case 'proxmox-pve':
      return 'text-blue-700 dark:text-blue-300';
    case 'proxmox-pbs':
      return 'text-emerald-700 dark:text-emerald-300';
    case 'truenas':
      return 'text-cyan-700 dark:text-cyan-300';
    case 'agent':
      return 'text-violet-700 dark:text-violet-300';
    default:
      return 'text-base-content';
  }
};

function extractDiskData(resource: Resource): PhysicalDiskData {
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

function hasSmartWarning(disk: PhysicalDiskData): boolean {
  const attrs = disk.smartAttributes;
  if (!attrs) return false;
  return Boolean(
    (attrs.reallocatedSectors && attrs.reallocatedSectors > 0) ||
      (attrs.pendingSectors && attrs.pendingSectors > 0) ||
      (attrs.mediaErrors && attrs.mediaErrors > 0),
  );
}

const getDiskHealthStatus = (disk: PhysicalDiskData) => {
  const normalizedHealth = (disk.health || '').trim().toUpperCase();
  const criticalRisk = (disk.riskLevel || '').trim().toLowerCase() === 'critical';
  const warningRisk = (disk.riskLevel || '').trim().toLowerCase() === 'warning';
  const smartWarning = hasSmartWarning(disk);
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
};

const getDiskRoleLabel = (disk: PhysicalDiskData): string => {
  if (disk.storageRole?.trim()) return titleize(disk.storageRole);
  if (disk.type?.trim()) return `${disk.type.toUpperCase()} Disk`;
  return 'Disk';
};

const getDiskParentLabel = (disk: PhysicalDiskData): string => {
  if (disk.storageGroup?.trim()) return disk.storageGroup.trim();
  return 'Standalone Device';
};

const getWearSummary = (disk: PhysicalDiskData): string => {
  if (disk.wearout > 0) return `${disk.wearout}% life left`;
  if (disk.smartAttributes?.percentageUsed != null) {
    return `${disk.smartAttributes.percentageUsed}% used`;
  }
  if (disk.smartAttributes?.powerOnHours != null) {
    return formatPowerOnHours(disk.smartAttributes.powerOnHours, true);
  }
  return 'No wear data';
};

const getTemperatureTone = (temperature: number): string => {
  if (temperature >= 70) return 'text-red-600 dark:text-red-400';
  if (temperature >= 60) return 'text-amber-600 dark:text-amber-400';
  return 'text-green-600 dark:text-green-400';
};

export const DiskList: Component<DiskListProps> = (props) => {
  const [selectedDisk, setSelectedDisk] = createSignal<Resource | null>(null);

  const hasPVENodes = createMemo(() => props.nodes.length > 0);

  const diskDataById = createMemo(() => {
    const map = new Map<string, PhysicalDiskData>();
    for (const disk of props.disks || []) {
      map.set(disk.id, extractDiskData(disk));
    }
    return map;
  });

  const getDiskData = (disk: Resource): PhysicalDiskData =>
    diskDataById().get(disk.id) ?? extractDiskData(disk);

  const filteredDisks = createMemo(() => {
    let disks = props.disks || [];

    if (props.selectedNode) {
      const node = props.nodes.find((n) => n.id === props.selectedNode);
      if (node) {
        disks = disks.filter((d) =>
          matchesPhysicalDiskNode(d, {
            id: node.id,
            name: node.name,
            instance: getProxmoxData(node)?.instance,
          }),
        );
      }
    }

    if (props.searchTerm) {
      const term = props.searchTerm.toLowerCase();
      disks = disks.filter((disk) => {
        const data = getDiskData(disk);
        return [
          data.model,
          data.devPath,
          data.serial,
          data.node,
          getDiskRoleLabel(data),
          getDiskParentLabel(data),
          platformLabel(disk),
        ]
          .join(' ')
          .toLowerCase()
          .includes(term);
      });
    }

    return [...disks].sort((a, b) => {
      const aData = getDiskData(a);
      const bData = getDiskData(b);
      const aPriority =
        (aData.riskLevel === 'critical' ? 300 : aData.riskLevel === 'warning' ? 200 : 0) +
        (hasSmartWarning(aData) ? 50 : 0);
      const bPriority =
        (bData.riskLevel === 'critical' ? 300 : bData.riskLevel === 'warning' ? 200 : 0) +
        (hasSmartWarning(bData) ? 50 : 0);
      if (aPriority !== bPriority) return bPriority - aPriority;
      if (aData.node !== bData.node) return aData.node.localeCompare(bData.node);
      return (aData.devPath || a.name).localeCompare(bData.devPath || b.name);
    });
  });

  const selectedNodeName = createMemo(() => {
    if (!props.selectedNode) return null;
    return props.nodes.find((n) => n.id === props.selectedNode)?.name || null;
  });

  const handleRowClick = (disk: Resource) => {
    setSelectedDisk((current) => (current?.id === disk.id ? null : disk));
  };

  return (
    <div>
      <Show when={filteredDisks().length === 0}>
        <Card padding="lg" class="text-center">
          <div class="">
            <p class="text-sm font-medium">No physical disks found</p>
            {selectedNodeName() && <p class="text-xs mt-1">for node {selectedNodeName()}</p>}
            {props.searchTerm && <p class="text-xs mt-1">matching "{props.searchTerm}"</p>}
          </div>
          <Show when={!props.searchTerm && (props.disks || []).length === 0}>
            <Show
              when={hasPVENodes()}
              fallback={
                <div class="mt-4 rounded-md border border-border bg-surface-alt p-4 text-left">
                  <p class="text-sm text-muted">
                    No Proxmox nodes configured. Add a Proxmox VE cluster in Settings to monitor
                    physical disks.
                  </p>
                </div>
              }
            >
              <div class="mt-4 rounded-md border border-blue-200 bg-blue-50 p-4 text-left dark:border-blue-800 dark:bg-blue-900">
                <p class="mb-2 text-sm font-medium text-blue-900 dark:text-blue-100">
                  Physical disk monitoring requirements:
                </p>
                <ol class="ml-4 list-decimal space-y-1.5 text-xs text-blue-800 dark:text-blue-200">
                  <li>
                    Enable "Monitor physical disk health (SMART)" in Settings → Infrastructure
                    (Proxmox node advanced settings)
                  </li>
                  <li>
                    Enable SMART monitoring in Proxmox VE at Datacenter → Node → System → Advanced →
                    "Monitor physical disk health"
                  </li>
                  <li>Wait 5 minutes for Proxmox to collect SMART data</li>
                </ol>
                <p class="mt-3 text-xs italic text-blue-700 dark:text-blue-300">
                  Note: Both Pulse and Proxmox must have SMART monitoring enabled.
                </p>
              </div>
            </Show>
          </Show>
        </Card>
      </Show>

      <Show when={filteredDisks().length > 0}>
        <Card padding="none" tone="card" class="overflow-hidden">
          <div class="overflow-x-auto" style={{ '-webkit-overflow-scrolling': 'touch' }}>
            <Table class="w-full">
              <TableHeader>
                <TableRow class="border-b border-border bg-surface-alt text-muted">
                  <TableHead class="px-2 py-1 text-left text-[11px] font-medium uppercase tracking-wider">
                    Disk
                  </TableHead>
                  <TableHead class="px-2 py-1 text-left text-[11px] font-medium uppercase tracking-wider">
                    Source
                  </TableHead>
                  <TableHead class="px-2 py-1 text-left text-[11px] font-medium uppercase tracking-wider">
                    Host
                  </TableHead>
                  <TableHead class="hidden xl:table-cell px-2 py-1 text-left text-[11px] font-medium uppercase tracking-wider">
                    Role
                  </TableHead>
                  <TableHead class="hidden xl:table-cell px-2 py-1 text-left text-[11px] font-medium uppercase tracking-wider">
                    Belongs To
                  </TableHead>
                  <TableHead class="px-2 py-1 text-left text-[11px] font-medium uppercase tracking-wider">
                    Health
                  </TableHead>
                  <TableHead class="hidden md:table-cell px-2 py-1 text-left text-[11px] font-medium uppercase tracking-wider">
                    Wear / Temp
                  </TableHead>
                  <TableHead class="px-2 py-1 text-left text-[11px] font-medium uppercase tracking-wider">
                    Size
                  </TableHead>
                  <TableHead class="px-1.5 py-1 w-10" />
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border">
                <For each={filteredDisks()}>
                  {(disk) => {
                    const data = getDiskData(disk);
                    const status = getDiskHealthStatus(data);
                    const isSelected = () => selectedDisk()?.id === disk.id;

                    return (
                      <>
                        <TableRow
                          class={`cursor-pointer transition-colors ${
                            isSelected() ? 'bg-blue-50 dark:bg-blue-900' : 'hover:bg-surface-hover'
                          }`}
                          onClick={() => handleRowClick(disk)}
                        >
                          <TableCell class="px-2 py-1 align-middle text-xs">
                            <div class="flex min-w-0 items-center gap-2 whitespace-nowrap">
                              <span class="truncate text-[12px] font-semibold text-base-content">
                                {data.model || 'Unknown Disk'}
                              </span>
                              <span
                                class="hidden lg:inline shrink-0 font-mono text-[10px] text-muted"
                                title={data.devPath || disk.name}
                              >
                                {data.devPath || disk.name}
                              </span>
                              <Show when={data.serial}>
                                <span
                                  class="hidden xl:block truncate text-[11px] text-muted"
                                  title={data.serial}
                                >
                                  S/N {data.serial}
                                </span>
                              </Show>
                            </div>
                          </TableCell>

                          <TableCell class="px-2 py-1 align-middle text-xs">
                            <span
                              class={`inline-block text-[11px] font-semibold tracking-wide ${platformTextClass(disk)}`}
                            >
                              {platformLabel(disk)}
                            </span>
                          </TableCell>

                          <TableCell class="px-2 py-1 align-middle text-xs">
                            <span
                              class="block truncate text-[11px] text-base-content"
                              title={data.node || disk.parentName || 'Unknown Host'}
                            >
                              {data.node || disk.parentName || 'Unknown Host'}
                            </span>
                          </TableCell>

                          <TableCell class="hidden xl:table-cell px-2 py-1 align-middle text-xs">
                            <span
                              class="block truncate text-[11px] text-base-content"
                              title={getDiskRoleLabel(data)}
                            >
                              {getDiskRoleLabel(data)}
                            </span>
                          </TableCell>

                          <TableCell class="hidden xl:table-cell px-2 py-1 align-middle text-xs">
                            <span
                              class="block truncate text-[11px] text-base-content"
                              title={getDiskParentLabel(data)}
                            >
                              {getDiskParentLabel(data)}
                            </span>
                          </TableCell>

                          <TableCell class="px-2 py-1 align-middle text-xs">
                            <div class="flex min-w-0 items-center gap-1.5 whitespace-nowrap">
                              <span
                                class={`shrink-0 text-[11px] font-semibold ${status.tone}`}
                              >
                                {status.label}
                              </span>
                              <span
                                class="hidden xl:block truncate text-[11px] text-muted"
                                title={status.summary}
                              >
                                {status.summary}
                              </span>
                            </div>
                          </TableCell>

                          <TableCell class="hidden md:table-cell px-2 py-1 align-middle text-xs">
                            <div class="flex min-w-0 items-center gap-2 whitespace-nowrap">
                              <span
                                class="hidden xl:block truncate text-[11px] text-base-content"
                                title={getWearSummary(data)}
                              >
                                {getWearSummary(data)}
                              </span>
                              <span class={`shrink-0 text-[11px] font-medium ${getTemperatureTone(data.temperature)}`}>
                                {data.temperature > 0 ? formatTemperature(data.temperature) : '-'}
                              </span>
                            </div>
                          </TableCell>

                          <TableCell class="px-2 py-1 align-middle text-xs whitespace-nowrap">
                            <span class="text-[11px] text-base-content">{formatBytes(data.size)}</span>
                          </TableCell>

                          <TableCell class="px-1.5 py-1 align-middle text-right">
                            <button
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                handleRowClick(disk);
                              }}
                              class="rounded p-1 hover:bg-surface-hover transition-colors"
                              aria-label={`Toggle details for ${data.model || 'disk'}`}
                            >
                              <svg
                                class={`h-3.5 w-3.5 text-muted transition-transform duration-150 ${
                                  isSelected() ? 'rotate-90' : ''
                                }`}
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                              >
                                <path
                                  stroke-linecap="round"
                                  stroke-linejoin="round"
                                  stroke-width="2"
                                  d="M9 5l7 7-7 7"
                                />
                              </svg>
                            </button>
                          </TableCell>
                        </TableRow>
                        <Show when={isSelected()}>
                          <TableRow>
                            <TableCell
                              colSpan={9}
                              class="border-b border-border-subtle bg-surface-alt px-4 py-4 shadow-inner"
                            >
                              <DiskDetail disk={disk} nodes={props.nodes} />
                            </TableCell>
                          </TableRow>
                        </Show>
                      </>
                    );
                  }}
                </For>
              </TableBody>
            </Table>
          </div>
        </Card>
      </Show>
    </div>
  );
};
