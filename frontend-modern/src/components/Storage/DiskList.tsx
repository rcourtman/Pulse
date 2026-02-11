import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { formatBytes, formatPowerOnHours } from '@/utils/format';
import { formatTemperature } from '@/utils/temperature';
import type { Resource } from '@/types/resource';
import { getProxmoxData, getLinkedAgentId } from '@/utils/resourcePlatformData';
import { DiskDetail } from './DiskDetail';
import { DiskLiveMetric } from './DiskLiveMetric';

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

function extractDiskData(resource: Resource): PhysicalDiskData {
  const platformData = (resource.platformData as any) || {};
  const pd = platformData.physicalDisk || {};
  const proxmox = platformData.proxmox || {};
  const smart = pd.smart || {};

  return {
    node: proxmox.nodeName || resource.platformId || '',
    instance: proxmox.instance || '',
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
    smartAttributes: pd.smart
      ? {
        powerOnHours: smart.powerOnHours,
        powerCycles: smart.powerCycles,
        reallocatedSectors: smart.reallocatedSectors,
        pendingSectors: smart.pendingSectors,
        offlineUncorrectable: smart.offlineUncorrectable,
        udmaCrcErrors: smart.udmaCrcErrors,
        percentageUsed: smart.percentageUsed,
        availableSpare: smart.availableSpare,
        mediaErrors: smart.mediaErrors,
        unsafeShutdowns: smart.unsafeShutdowns,
      }
      : undefined,
  };
}

/** Returns true if any critical SMART counters are non-zero. */
function hasSmartWarning(disk: PhysicalDiskData): boolean {
  const attrs = disk.smartAttributes;
  if (!attrs) return false;
  if (attrs.reallocatedSectors && attrs.reallocatedSectors > 0) return true;
  if (attrs.pendingSectors && attrs.pendingSectors > 0) return true;
  if (attrs.mediaErrors && attrs.mediaErrors > 0) return true;
  return false;
}

interface DiskListProps {
  disks: Resource[];
  nodes: Resource[];
  selectedNode: string | null;
  searchTerm: string;
}

export const DiskList: Component<DiskListProps> = (props) => {
  const [selectedDisk, setSelectedDisk] = createSignal<Resource | null>(null);

  // Check if there are any PVE nodes configured
  const hasPVENodes = createMemo(() => {
    return props.nodes.length > 0;
  });

  const diskDataById = createMemo(() => {
    const map = new Map<string, PhysicalDiskData>();
    for (const disk of props.disks || []) {
      map.set(disk.id, extractDiskData(disk));
    }
    return map;
  });

  const getDiskData = (disk: Resource): PhysicalDiskData => {
    return diskDataById().get(disk.id) ?? extractDiskData(disk);
  };

  // Filter disks based on selected node and search term
  const filteredDisks = createMemo(() => {
    let disks = props.disks || [];

    // Filter by node if selected using both instance and node name
    if (props.selectedNode) {
      const node = props.nodes.find((n) => n.id === props.selectedNode);
      if (node) {
        const instance = getProxmoxData(node)?.instance;
        disks = disks.filter(
          (d) => d.parentId === node.id || (getDiskData(d).instance === instance && getDiskData(d).node === node.name),
        );
      }
    }

    // Filter by search term
    if (props.searchTerm) {
      const term = props.searchTerm.toLowerCase();
      disks = disks.filter(
        (d) => {
          const data = getDiskData(d);
          return (
            data.model.toLowerCase().includes(term) ||
            data.devPath.toLowerCase().includes(term) ||
            data.serial.toLowerCase().includes(term) ||
            data.node.toLowerCase().includes(term)
          );
        },
      );
    }

    // Sort by node and devPath - create a copy to avoid mutating store
    return [...disks].sort((a, b) => {
      const aData = getDiskData(a);
      const bData = getDiskData(b);
      if (aData.node !== bData.node) return aData.node.localeCompare(bData.node);
      return aData.devPath.localeCompare(bData.devPath);
    });
  });

  // Get health status color and badge
  const getHealthStatus = (disk: PhysicalDiskData) => {
    const healthValue = (disk.health || '').trim();
    const normalizedHealth = healthValue.toUpperCase();
    const isHealthy =
      normalizedHealth === 'PASSED' ||
      normalizedHealth === 'OK' ||
      normalizedHealth === 'GOOD';

    if (isHealthy) {
      // Check wearout for SSDs
      if (disk.wearout > 0 && disk.wearout < 10) {
        return {
          color: 'text-yellow-700 dark:text-yellow-400',
          bgColor: 'bg-yellow-100 dark:bg-yellow-900/30',
          text: 'LOW LIFE',
        };
      }
      const label = normalizedHealth === 'PASSED' ? 'HEALTHY' : normalizedHealth;
      return {
        color: 'text-green-700 dark:text-green-400',
        bgColor: 'bg-green-100 dark:bg-green-900/30',
        text: label,
      };
    } else if (normalizedHealth === 'FAILED') {
      return {
        color: 'text-red-700 dark:text-red-400',
        bgColor: 'bg-red-100 dark:bg-red-900/30',
        text: 'FAILED',
      };
    }
    return {
      color: 'text-gray-700 dark:text-gray-400',
      bgColor: 'bg-gray-100 dark:bg-gray-700',
      text: 'UNKNOWN',
    };
  };

  // Get disk type badge color
  const getDiskTypeBadge = (type: string) => {
    switch (type.toLowerCase()) {
      case 'nvme':
        return 'bg-purple-100 dark:bg-purple-900/30 text-purple-800 dark:text-purple-300';
      case 'sata':
        return 'bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300';
      case 'sas':
        return 'bg-indigo-100 dark:bg-indigo-900/30 text-indigo-800 dark:text-indigo-300';
      default:
        return 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300';
    }
  };

  // Get selected node name for display
  const selectedNodeName = createMemo(() => {
    if (!props.selectedNode) return null;
    const node = props.nodes.find(n => n.id === props.selectedNode);
    return node?.name || null;
  });

  const handleRowClick = (disk: Resource) => {
    const current = selectedDisk();
    if (current && current.id === disk.id) {
      setSelectedDisk(null);
    } else {
      setSelectedDisk(disk);
    }
  };

  const getNodeHostId = (nodeName: string, instance: string) => {
    const node = props.nodes.find(n => n.name === nodeName && getProxmoxData(n)?.instance === instance);
    return node ? getLinkedAgentId(node) : undefined;
  };

  const getMetricResourceId = (disk: Resource) => {
    // Use the metrics target from the unified resource API
    if (disk.metricsTarget?.resourceId) {
      return disk.metricsTarget.resourceId;
    }
    // Fallback: try to construct from platform data
    const data = getDiskData(disk);
    const hostId = getNodeHostId(data.node, data.instance);
    if (!hostId) return null;
    // Strip /dev/ if present to match agent metric key
    const deviceName = data.devPath.replace('/dev/', '');
    return `${hostId}:${deviceName}`;
  };

  return (
    <div>
      <Show when={filteredDisks().length === 0}>
        <Card padding="lg" class="text-center">
          <div class="text-gray-500">
            <p class="text-sm font-medium">No physical disks found</p>
            {selectedNodeName() && <p class="text-xs mt-1">for node {selectedNodeName()}</p>}
            {props.searchTerm && <p class="text-xs mt-1">matching "{props.searchTerm}"</p>}
          </div>
          <Show when={!props.searchTerm && (props.disks || []).length === 0}>
            <Show
              when={hasPVENodes()}
              fallback={
                <div class="mt-4 p-4 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg text-left">
                  <p class="text-sm text-gray-600 dark:text-gray-400">
                    No Proxmox nodes configured. Add a Proxmox VE cluster in Settings to monitor physical disks.
                  </p>
                </div>
              }
            >
              <div class="mt-4 p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg text-left">
                <p class="text-sm font-medium text-blue-900 dark:text-blue-100 mb-2">
                  Physical disk monitoring requirements:
                </p>
                <ol class="text-xs text-blue-800 dark:text-blue-200 space-y-1.5 ml-4 list-decimal">
                  <li>Enable "Monitor physical disk health (SMART)" in Settings → Infrastructure (Proxmox node advanced settings)</li>
                  <li>Enable SMART monitoring in Proxmox VE at Datacenter → Node → System → Advanced → "Monitor physical disk health"</li>
                  <li>Wait 5 minutes for Proxmox to collect SMART data</li>
                </ol>
                <p class="text-xs text-blue-700 dark:text-blue-300 mt-3 italic">
                  Note: Both Pulse and Proxmox must have SMART monitoring enabled.
                </p>
              </div>
            </Show>
          </Show>
        </Card>
      </Show>

      <Show when={filteredDisks().length > 0}>
        <Card padding="none" tone="glass" class="overflow-hidden">
          <div class="overflow-x-auto" style={{ '-webkit-overflow-scrolling': 'touch' }}>
            <table class="w-full" style={{ "min-width": "800px" }}>
              <thead>
                <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
                  <th class="px-1 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[10%]">
                    Node
                  </th>
                  <th class="px-1 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[9%]">
                    Device
                  </th>
                  <th class="px-1 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[19%]">
                    Model
                  </th>
                  <th class="px-1 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[7%]">
                    Type
                  </th>
                  <th class="px-1 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[7%]">
                    FS
                  </th>
                  <th class="px-1 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[10%]">
                    Health
                  </th>
                  <th class="px-1 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[13%]">
                    SSD Life
                  </th>
                  <th class="px-1 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[8%]">
                    Power-On
                  </th>
                  <th class="px-1 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[7%]">
                    Temp
                  </th>
                  <th class="hidden md:table-cell px-1 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[8%]">
                    Read
                  </th>
                  <th class="hidden md:table-cell px-1 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[8%]">
                    Write
                  </th>
                  <th class="hidden md:table-cell px-1 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[6%]">
                    Busy
                  </th>
                  <th class="px-1 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[10%]">
                    Size
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                <For each={filteredDisks()}>
                  {(disk) => {
                    const data = getDiskData(disk);
                    const health = getHealthStatus(data);
                    const isSelected = () => selectedDisk()?.id === disk.id;
                    const warning = hasSmartWarning(data);

                    return (
                      <>
                        <tr
                          class={`cursor-pointer transition-colors ${isSelected() ? 'bg-blue-50 dark:bg-blue-900/20' : 'hover:bg-gray-50 dark:hover:bg-gray-700/30'}`}
                          onClick={() => handleRowClick(disk)}
                        >
                          <td class="px-1 py-1.5 text-xs">
                            <div class="flex items-center gap-1.5 min-w-0">
                              <div
                                class={`cursor-pointer transition-transform duration-200 ${isSelected() ? 'rotate-90' : ''}`}
                              >
                                <svg class="w-3.5 h-3.5 text-gray-500 hover:text-gray-700 dark:hover:text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                                </svg>
                              </div>
                              <span class="font-medium text-gray-900 dark:text-gray-100">
                                {data.node}
                              </span>
                            </div>
                          </td>
                          <td class="px-1 py-1.5 text-xs">
                            <span class="font-mono text-gray-600 dark:text-gray-400">
                              {data.devPath}
                            </span>
                          </td>
                          <td class="px-1 py-1.5 text-xs">
                            <span class="text-gray-700 dark:text-gray-300">
                              {data.model || 'Unknown'}
                            </span>
                          </td>
                          <td class="px-1 py-1.5 text-xs">
                            <span
                              class={`inline-block px-1.5 py-0.5 text-[10px] font-medium rounded ${getDiskTypeBadge(data.type)}`}
                            >
                              {data.type.toUpperCase()}
                            </span>
                          </td>
                          <td class="px-1 py-1.5 text-xs">
                            <Show
                              when={data.used && data.used !== 'unknown'}
                              fallback={<span class="text-gray-400">-</span>}
                            >
                              <span class="text-[10px] font-mono text-gray-600 dark:text-gray-400">
                                {data.used}
                              </span>
                            </Show>
                          </td>
                          <td class="px-1 py-1.5 text-xs">
                            <span
                              class={`inline-block px-1.5 py-0.5 text-[10px] font-medium rounded ${health.bgColor} ${health.color}`}
                            >
                              {health.text}
                            </span>
                            <Show when={warning}>
                              <span class="ml-1 text-yellow-500 dark:text-yellow-400" title="SMART warning: critical counters non-zero">
                                &#9888;
                              </span>
                            </Show>
                          </td>
                          <td class="px-1 py-1.5 text-xs">
                            <Show
                              when={data.wearout > 0}
                              fallback={<span class="text-gray-400">-</span>}
                            >
                              <div class="relative w-24 h-3.5 rounded overflow-hidden bg-gray-200 dark:bg-gray-600">
                                <div
                                  class={`absolute top-0 left-0 h-full ${data.wearout >= 50
                                    ? 'bg-green-500/60 dark:bg-green-500/50'
                                    : data.wearout >= 20
                                      ? 'bg-yellow-500/60 dark:bg-yellow-500/50'
                                      : data.wearout >= 10
                                        ? 'bg-orange-500/60 dark:bg-orange-500/50'
                                        : 'bg-red-500/60 dark:bg-red-500/50'
                                    }`}
                                  style={{ width: `${data.wearout}%` }}
                                />
                                <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-800 dark:text-gray-100 leading-none">
                                  <span class="whitespace-nowrap px-0.5">{data.wearout}%</span>
                                </span>
                              </div>
                            </Show>
                          </td>
                          <td class="px-1 py-1.5 text-xs">
                            <Show
                              when={data.smartAttributes?.powerOnHours != null}
                              fallback={<span class="text-gray-400">-</span>}
                            >
                              <span class="text-gray-700 dark:text-gray-300">
                                {formatPowerOnHours(data.smartAttributes!.powerOnHours!, true)}
                              </span>
                            </Show>
                          </td>
                          <td class="px-1 py-1.5 text-xs">
                            <Show
                              when={typeof data.temperature === 'number'}
                              fallback={<span class="font-medium text-gray-400">-</span>}
                            >
                              <span
                                class={`font-medium ${data.temperature > 70
                                  ? 'text-red-600 dark:text-red-400'
                                  : data.temperature > 60
                                    ? 'text-yellow-600 dark:text-yellow-400'
                                    : 'text-green-600 dark:text-green-400'
                                  }`}
                              >
                                {formatTemperature(data.temperature)}
                              </span>
                            </Show>
                          </td>
                          <td class="hidden md:table-cell px-1 py-1.5 align-middle">
                            <Show when={getMetricResourceId(disk)} fallback={<span class="text-gray-300">-</span>}>
                              {(resourceId) => (
                                <DiskLiveMetric resourceId={resourceId()} type="read" />
                              )}
                            </Show>
                          </td>
                          <td class="hidden md:table-cell px-1 py-1.5 align-middle">
                            <Show when={getMetricResourceId(disk)} fallback={<span class="text-gray-300">-</span>}>
                              {(resourceId) => (
                                <DiskLiveMetric resourceId={resourceId()} type="write" />
                              )}
                            </Show>
                          </td>
                          <td class="hidden md:table-cell px-1 py-1.5 align-middle">
                            <Show when={getMetricResourceId(disk)} fallback={<span class="text-gray-300">-</span>}>
                              {(resourceId) => (
                                <DiskLiveMetric resourceId={resourceId()} type="ioTime" />
                              )}
                            </Show>
                          </td>
                          <td class="px-1 py-1.5 text-xs whitespace-nowrap">
                            <span class="text-gray-700 dark:text-gray-300">
                              {formatBytes(data.size)}
                            </span>
                          </td>
                        </tr>
                        <Show when={isSelected()}>
                          <tr>
                            <td colSpan={13} class="bg-gray-50/50 dark:bg-gray-900/20 px-4 py-4 border-b border-gray-100 dark:border-gray-700 shadow-inner">
                              <DiskDetail disk={disk} nodes={props.nodes} />
                            </td>
                          </tr>
                        </Show>
                      </>
                    );
                  }}
                </For>
              </tbody>
            </table>
          </div>
        </Card>
      </Show>
    </div>
  );
};
