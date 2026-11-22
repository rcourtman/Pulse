import type { Resource } from '@/types/resource';
import { getAvailabilityProbePresentation } from '@/utils/availabilityProbePresentation';
import {
  getAgentFirstReportStatusIndicator,
  isPulseAgentPlatformResource,
} from '@/utils/agentResources';
import { getSimpleStatusIndicator } from '@/utils/status';
import { buildStandalonePath, STANDALONE_VMS_PATH } from '@/routing/resourceLinks';

export type StandaloneTabId = 'machines' | 'vms' | 'availability';

export type StandaloneVMStatusFilter = 'all' | 'running' | 'attention' | 'stopped';

export const normalizeStandaloneVMStatusFilter = (
  value: string | string[] | undefined,
): StandaloneVMStatusFilter => {
  const raw = (Array.isArray(value) ? value[0] : value)?.trim().toLowerCase();
  if (raw === 'running' || raw === 'attention' || raw === 'stopped') return raw;
  return 'all';
};

export interface StandaloneTabSpec {
  id: StandaloneTabId;
  label: string;
  path: string;
}

export interface StandalonePageModel {
  machines: Resource[];
  vms: Resource[];
  availabilityChecks: Resource[];
  resources: Resource[];
}

export interface StandalonePostureSummary {
  attention: number;
  critical: number;
  latestUpdateAt?: number;
  normal: number;
  total: number;
  unknown: number;
  warning: number;
}

const PROVIDER_OWNED_VM_SOURCES = new Set([
  'proxmox-pve',
  'vmware-vsphere',
  'truenas',
  'kubernetes',
]);

export const isStandaloneMachineResource = (resource: Resource): boolean =>
  isPulseAgentPlatformResource(resource);

export const isStandaloneVirtualMachineResource = (resource: Resource): boolean => {
  if (resource.type !== 'vm') return false;
  if (PROVIDER_OWNED_VM_SOURCES.has(resource.platformType)) return false;
  const sources = resource.sources ?? [];
  if (sources.some((source) => PROVIDER_OWNED_VM_SOURCES.has(source))) return false;
  return true;
};

export const isAgentlessAvailabilityResource = (resource: Resource): boolean =>
  resource.type === 'network-endpoint' || resource.platformType === 'availability';

export const getStandaloneTabSpecs = (
  hasVms: boolean,
  activeTab: StandaloneTabId,
): readonly StandaloneTabSpec[] => {
  const tabs: StandaloneTabSpec[] = [
    { id: 'machines', label: 'Machines', path: buildStandalonePath('machines') },
  ];
  if (hasVms || activeTab === 'vms') {
    tabs.push({ id: 'vms', label: 'VMs', path: STANDALONE_VMS_PATH });
  }
  tabs.push({
    id: 'availability',
    label: 'Availability checks',
    path: buildStandalonePath('availability'),
  });
  return tabs;
};

export const buildStandaloneMachineVmCounts = (
  machines: readonly Resource[],
  vms: readonly Resource[],
): Map<string, number> => {
  const counts = new Map<string, number>();
  if (machines.length === 0 || vms.length === 0) return counts;
  const machineIdSet = new Set(machines.map((machine) => machine.id));
  const machineIdByName = new Map<string, string>();
  for (const machine of machines) {
    for (const candidate of [machine.name, machine.displayName, machine.agent?.hostname]) {
      const normalized = candidate?.trim().toLowerCase();
      if (normalized && !machineIdByName.has(normalized)) {
        machineIdByName.set(normalized, machine.id);
      }
    }
  }
  for (const vm of vms) {
    const parentId = vm.parentId?.trim();
    let ownerId = parentId && machineIdSet.has(parentId) ? parentId : undefined;
    if (!ownerId) {
      const parentName = vm.parentName?.trim().toLowerCase();
      if (parentName) ownerId = machineIdByName.get(parentName);
    }
    if (!ownerId) continue;
    counts.set(ownerId, (counts.get(ownerId) ?? 0) + 1);
  }
  return counts;
};

const AGENT_REPORT_STALE_AFTER_MS = 5 * 60 * 1000;

export const getStandaloneResourceStatusIndicator = (resource: Resource, nowMs = Date.now()) => {
  const waiting = getAgentFirstReportStatusIndicator(resource);
  if (waiting) return waiting;
  const indicator = getSimpleStatusIndicator(resource.status);
  if (indicator.variant === 'danger') return indicator;
  const availability = getAvailabilityProbePresentation(resource, new Date(nowMs));
  if (
    availability?.freshnessLabel === 'stale' ||
    availability?.freshnessLabel === 'freshness unknown' ||
    resource.availability?.correlationState === 'ambiguous' ||
    resource.availability?.correlationState === 'unresolved'
  ) {
    return {
      variant: 'warning' as const,
      label:
        availability?.freshnessLabel === 'stale'
          ? 'Stale'
          : availability?.freshnessLabel === 'freshness unknown'
            ? 'Freshness unknown'
            : 'Identity unresolved',
    };
  }
  if (
    resource.type === 'agent' &&
    (resource.agent?.stale === true ||
      (Number.isFinite(resource.lastSeen) &&
        resource.lastSeen > 0 &&
        nowMs - resource.lastSeen > AGENT_REPORT_STALE_AFTER_MS))
  ) {
    return { variant: 'warning' as const, label: 'Stale' };
  }
  return indicator;
};

export function buildStandalonePostureSummary(
  resources: readonly Resource[],
  nowMs = Date.now(),
): StandalonePostureSummary {
  const summary: StandalonePostureSummary = {
    attention: 0,
    critical: 0,
    normal: 0,
    total: resources.length,
    unknown: 0,
    warning: 0,
  };

  for (const resource of resources) {
    const variant = getStandaloneResourceStatusIndicator(resource, nowMs).variant;
    if (variant === 'success') {
      summary.normal += 1;
    } else if (variant === 'danger') {
      summary.critical += 1;
      summary.attention += 1;
    } else if (variant === 'warning') {
      summary.warning += 1;
      summary.attention += 1;
    } else {
      summary.unknown += 1;
    }

    if (
      Number.isFinite(resource.lastSeen) &&
      resource.lastSeen > 0 &&
      (!summary.latestUpdateAt || resource.lastSeen > summary.latestUpdateAt)
    ) {
      summary.latestUpdateAt = resource.lastSeen;
    }
  }

  return summary;
}

const ATTENTION_SORT_PRIORITY = {
  danger: 0,
  warning: 1,
  muted: 2,
  info: 2,
  success: 3,
} as const;

export function sortStandaloneResourcesByAttention(
  resources: readonly Resource[],
  nowMs = Date.now(),
): Resource[] {
  // Clock ticks can re-sort retained resources. Classify each once for this
  // sort, not twice per comparison, without caching across source updates.
  const priority = new Map<Resource, number>();
  for (const resource of resources) {
    priority.set(
      resource,
      ATTENTION_SORT_PRIORITY[getStandaloneResourceStatusIndicator(resource, nowMs).variant],
    );
  }
  return [...resources].sort((left, right) => {
    const leftPriority = priority.get(left)!;
    const rightPriority = priority.get(right)!;
    if (leftPriority !== rightPriority) return leftPriority - rightPriority;
    return left.displayName.localeCompare(right.displayName, undefined, { numeric: true });
  });
}

export function buildStandalonePageModel(resources: readonly Resource[]): StandalonePageModel {
  const machines = resources.filter(isStandaloneMachineResource);
  const vms = resources.filter(isStandaloneVirtualMachineResource);
  const availabilityChecks = resources.filter(isAgentlessAvailabilityResource);
  return {
    machines,
    vms,
    availabilityChecks,
    resources: machines,
  };
}
