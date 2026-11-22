import type { Resource } from '@/types/resource';
import {
  formatPlatformTableTitleCaseValue,
  type PlatformTableSortValue,
} from '@/features/platformPage/sharedPlatformPage';
import { hasImpairedResourceSource } from '@/utils/resourceSourceHealth';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  normalizeStandaloneVMStatusFilter,
  type StandaloneVMStatusFilter,
} from './standalonePageModel';

export { normalizeStandaloneVMStatusFilter, type StandaloneVMStatusFilter };
export type StandaloneVMStatusBucket = Exclude<StandaloneVMStatusFilter, 'all'>;

export const STANDALONE_VM_SORT_KEYS = [
  'vm',
  'host',
  'state',
  'cpu',
  'memory',
  'diskio',
  'netio',
] as const;

export type StandaloneVMSortKey = (typeof STANDALONE_VM_SORT_KEYS)[number];

export const getStandaloneVMRuntimeState = (resource: Resource): string =>
  (
    asTrimmedString(resource.virtualMachine?.runtimeState) ??
    asTrimmedString(resource.status) ??
    ''
  ).toLowerCase();

const HEALTHY_SOURCE_STATUSES = new Set(['online', 'running', 'healthy', 'connected', 'ok']);

export const isStandaloneVMSourceImpaired = (resource: Resource): boolean => {
  if (hasImpairedResourceSource(resource, 'agent')) return true;
  const directStatus = (asTrimmedString(resource.sourceStatus?.agent?.status) ?? '').toLowerCase();
  return directStatus !== '' && !HEALTHY_SOURCE_STATUSES.has(directStatus);
};

export const mapStandaloneVMStatus = (resource: Resource): StandaloneVMStatusBucket => {
  const state = getStandaloneVMRuntimeState(resource);
  if (state === 'shutoff' || state === 'stopped' || state === 'halted' || state === 'offline') {
    return 'stopped';
  }
  if (
    isStandaloneVMSourceImpaired(resource) ||
    state === 'paused' ||
    state === 'suspended' ||
    state === 'shutting-down' ||
    state === 'crashed' ||
    state === 'degraded' ||
    state === 'warning' ||
    state === 'unknown' ||
    !state
  ) {
    return 'attention';
  }
  if (state === 'running' || state === 'blocked' || state === 'online') {
    return 'running';
  }
  return 'attention';
};

export const getStandaloneVMDisplayStatus = (resource: Resource): string => {
  const bucket = mapStandaloneVMStatus(resource);
  if (bucket === 'running') return 'online';
  if (bucket === 'stopped') return 'stopped';
  const state = getStandaloneVMRuntimeState(resource);
  if (state === 'crashed') return 'offline';
  return 'degraded';
};

export const getStandaloneVMStateLabel = (resource: Resource): string => {
  const state = getStandaloneVMRuntimeState(resource);
  switch (state) {
    case 'running':
    case 'online':
      return 'Running';
    case 'blocked':
      return 'Running';
    case 'shutoff':
      return 'Shutoff';
    case 'stopped':
    case 'offline':
      return 'Stopped';
    case 'halted':
      return 'Halted';
    case 'paused':
      return 'Paused';
    case 'suspended':
      return 'Suspended';
    case 'shutting-down':
      return 'Shutting down';
    case 'crashed':
      return 'Crashed';
    default:
      return state ? formatPlatformTableTitleCaseValue(state) : 'Unknown';
  }
};

export const getStandaloneVMHypervisorLabel = (resource: Resource): string => {
  const raw = (
    asTrimmedString(resource.virtualMachine?.hypervisor) ??
    asTrimmedString(resource.technology) ??
    ''
  ).toLowerCase();
  if (raw === 'libvirt') return 'Libvirt';
  if (raw === 'xcp-ng') return 'XCP-ng';
  return raw ? formatPlatformTableTitleCaseValue(raw) : '';
};

export const getStandaloneVMName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

export const resolveStandaloneVMHostResource = (
  vm: Resource,
  machinesById: ReadonlyMap<string, Resource>,
  machinesByName?: ReadonlyMap<string, Resource>,
): Resource | undefined => {
  const parentId = asTrimmedString(vm.parentId);
  if (parentId) {
    const byId = machinesById.get(parentId);
    if (byId) return byId;
  }
  const parentName = (asTrimmedString(vm.parentName) ?? '').toLowerCase();
  if (parentName && machinesByName) {
    return machinesByName.get(parentName);
  }
  return undefined;
};

export const resolveStandaloneVMHostName = (
  vm: Resource,
  machinesById: ReadonlyMap<string, Resource>,
  machinesByName?: ReadonlyMap<string, Resource>,
): string => {
  const host = resolveStandaloneVMHostResource(vm, machinesById, machinesByName);
  if (host) {
    return asTrimmedString(host.name) || asTrimmedString(host.displayName) || host.id;
  }
  return asTrimmedString(vm.parentName) || '—';
};

export const matchesStandaloneVMHostScope = (
  vm: Resource,
  selectedHostId: string | undefined,
  machinesById: ReadonlyMap<string, Resource>,
  machinesByName?: ReadonlyMap<string, Resource>,
): boolean => {
  const scope = asTrimmedString(selectedHostId);
  if (!scope) return true;
  if (asTrimmedString(vm.parentId) === scope) return true;
  const owner = resolveStandaloneVMHostResource(vm, machinesById, machinesByName);
  if (owner && owner.id === scope) return true;
  const scopeLower = scope.toLowerCase();
  const ownerName = resolveStandaloneVMHostName(vm, machinesById, machinesByName).toLowerCase();
  return ownerName !== '—' && ownerName === scopeLower;
};

export const getStandaloneVMTotalDiskIORate = (resource: Resource): number | null => {
  const read = resource.diskIO?.readRate;
  const write = resource.diskIO?.writeRate;
  const hasRead = typeof read === 'number' && Number.isFinite(read) && read >= 0;
  const hasWrite = typeof write === 'number' && Number.isFinite(write) && write >= 0;
  if (!hasRead && !hasWrite) return null;
  return (hasRead ? read : 0) + (hasWrite ? write : 0);
};

export const getStandaloneVMTotalNetIORate = (resource: Resource): number | null => {
  const rx = resource.network?.rxBytes;
  const tx = resource.network?.txBytes;
  const hasRx = typeof rx === 'number' && Number.isFinite(rx) && rx >= 0;
  const hasTx = typeof tx === 'number' && Number.isFinite(tx) && tx >= 0;
  if (!hasRx && !hasTx) return null;
  return (hasRx ? rx : 0) + (hasTx ? tx : 0);
};

export const filterStandaloneVMs = (
  vms: readonly Resource[],
  search: string,
  status: StandaloneVMStatusFilter,
  machinesById: ReadonlyMap<string, Resource>,
  machinesByName?: ReadonlyMap<string, Resource>,
  selectedHostId?: string,
): Resource[] => {
  const needle = search.trim().toLowerCase();
  return vms.filter((vm) => {
    if (!matchesStandaloneVMHostScope(vm, selectedHostId, machinesById, machinesByName)) {
      return false;
    }
    if (status !== 'all' && mapStandaloneVMStatus(vm) !== status) {
      return false;
    }
    if (!needle) return true;
    const haystack = [
      getStandaloneVMName(vm),
      vm.name,
      resolveStandaloneVMHostName(vm, machinesById, machinesByName),
      getStandaloneVMStateLabel(vm),
      getStandaloneVMRuntimeState(vm),
      getStandaloneVMHypervisorLabel(vm),
      vm.id,
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase();
    return haystack.includes(needle);
  });
};

export const getStandaloneVMSortValue = (
  resource: Resource,
  key: StandaloneVMSortKey,
  machinesById: ReadonlyMap<string, Resource>,
  machinesByName?: ReadonlyMap<string, Resource>,
): PlatformTableSortValue => {
  switch (key) {
    case 'vm':
      return getStandaloneVMName(resource);
    case 'host': {
      const host = resolveStandaloneVMHostName(resource, machinesById, machinesByName);
      return host === '—' ? null : host;
    }
    case 'state':
      return getStandaloneVMStateLabel(resource);
    case 'cpu': {
      const value = resource.cpu?.current;
      return typeof value === 'number' && Number.isFinite(value) ? value : null;
    }
    case 'memory': {
      const value = resource.memory?.current;
      return typeof value === 'number' && Number.isFinite(value) ? value : null;
    }
    case 'diskio':
      return getStandaloneVMTotalDiskIORate(resource);
    case 'netio':
      return getStandaloneVMTotalNetIORate(resource);
    default:
      key satisfies never;
      return null;
  }
};
