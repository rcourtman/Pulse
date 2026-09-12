import type { Node } from '@/types/api';
import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import {
  getWorkloadPlatformScopes,
  normalizeWorkloadViewModeParam,
  resolveWorkloadType,
  workloadMatchesViewMode,
} from '@/utils/workloads';
import { buildSourcePlatformOptions } from '@/utils/sourcePlatformOptions';
import type { WorkloadsFilterSelectOption } from './workloadsFilterModel';
import {
  getKubernetesContextKey,
  getWorkloadHostLabel,
  workloadHostScopeId,
  workloadNodeScopeId,
} from './workloadTopology';

export type WorkloadNodeOption = WorkloadsFilterSelectOption;

export const deserializeWorkloadViewMode = (raw: unknown): ViewMode => {
  if (typeof raw !== 'string') return 'all';
  return normalizeWorkloadViewModeParam(raw) ?? 'all';
};

export type WorkloadInventoryNode = Pick<Node, 'name' | 'instance'>;

export const buildWorkloadNodeOptions = (
  guests: WorkloadGuest[],
  nodes: readonly WorkloadInventoryNode[] = [],
): WorkloadNodeOption[] => {
  const labelsByScope = new Map<string, string>();
  const scopesByLabel = new Map<string, Set<string>>();

  // Inventory owns node identity even when a node currently has no guests.
  for (const node of nodes) {
    const name = node.name.trim();
    if (!name) continue;
    const scopes = scopesByLabel.get(name) ?? new Set<string>();
    scopes.add(workloadNodeScopeId({ node: name, instance: node.instance }));
    scopesByLabel.set(name, scopes);
  }

  for (const guest of guests) {
    const type = resolveWorkloadType(guest);
    if (type === 'pod') continue;
    const scope = workloadHostScopeId(guest);
    if (!scope || scope === '-') continue;
    const nodeName = (guest.node || '').trim();
    const label = getWorkloadHostLabel(guest);
    if (!label) continue;
    const disambiguationLabel = type === 'app-container' ? label : nodeName;
    if (!disambiguationLabel) continue;
    const scopes = scopesByLabel.get(disambiguationLabel) ?? new Set<string>();
    scopes.add(scope);
    scopesByLabel.set(disambiguationLabel, scopes);
  }

  for (const guest of guests) {
    const type = resolveWorkloadType(guest);
    if (type === 'pod') continue;
    const scope = workloadHostScopeId(guest);
    if (!scope || scope === '-' || labelsByScope.has(scope)) continue;
    if (type === 'app-container') {
      const hostLabel = getWorkloadHostLabel(guest);
      if (!hostLabel) continue;
      const hasDuplicateHostLabel = (scopesByLabel.get(hostLabel)?.size ?? 0) > 1;
      labelsByScope.set(
        scope,
        hasDuplicateHostLabel && scope !== hostLabel ? `${hostLabel} (${scope})` : hostLabel,
      );
      continue;
    }
    const nodeName = (guest.node || '').trim();
    const instance = (guest.instance || '').trim();
    if (!nodeName) continue;
    const hasDuplicateNodeName = (scopesByLabel.get(nodeName)?.size ?? 0) > 1;
    const label = hasDuplicateNodeName && instance ? `${nodeName} (${instance})` : nodeName;
    labelsByScope.set(scope, label);
  }

  for (const node of nodes) {
    const name = node.name.trim();
    if (!name) continue;
    const instance = (node.instance || '').trim();
    const scope = workloadNodeScopeId({ node: name, instance });
    if (labelsByScope.has(scope)) continue;
    const label =
      (scopesByLabel.get(name)?.size ?? 0) > 1 && instance ? `${name} (${instance})` : name;
    labelsByScope.set(scope, label);
  }

  return Array.from(labelsByScope.entries())
    .map(([value, label]) => ({ value, label }))
    .sort((a, b) => a.label.localeCompare(b.label));
};

export const buildWorkloadsKubernetesContextOptions = (guests: WorkloadGuest[]): string[] => {
  const contexts = new Set<string>();
  for (const guest of guests) {
    if (resolveWorkloadType(guest) !== 'pod') continue;
    const context = getKubernetesContextKey(guest);
    if (context) {
      contexts.add(context);
    }
  }
  return Array.from(contexts).sort((a, b) => a.localeCompare(b));
};

export const buildWorkloadsKubernetesNamespaceOptions = (
  guests: WorkloadGuest[],
  selectedContext: string | null,
): string[] => {
  const namespaces = new Set<string>();
  const contextFilter = (selectedContext || '').trim();
  for (const guest of guests) {
    if (resolveWorkloadType(guest) !== 'pod') continue;
    if (contextFilter && getKubernetesContextKey(guest) !== contextFilter) continue;
    const namespace = (guest.namespace || '').trim();
    if (namespace) namespaces.add(namespace);
  }
  return Array.from(namespaces).sort((a, b) => a.localeCompare(b));
};

export const buildWorkloadsVmwareClusterOptions = (guests: WorkloadGuest[]): string[] => {
  const set = new Set<string>();
  for (const guest of guests) {
    if (resolveWorkloadType(guest) !== 'vm') continue;
    const cluster = (guest.clusterName || '').trim();
    if (cluster) set.add(cluster);
  }
  return Array.from(set).sort((a, b) => a.localeCompare(b));
};

export const buildWorkloadsContainerRuntimeOptions = (guests: WorkloadGuest[]): string[] => {
  const runtimes = new Set<string>();
  for (const guest of guests) {
    if (resolveWorkloadType(guest) !== 'app-container') continue;
    const runtime = (guest.containerRuntime || '').trim();
    if (runtime) {
      runtimes.add(runtime);
    }
  }
  return Array.from(runtimes).sort((a, b) => a.localeCompare(b));
};

export const buildWorkloadsPlatformOptions = (
  guests: WorkloadGuest[],
  viewMode: ViewMode,
): WorkloadsFilterSelectOption[] =>
  buildSourcePlatformOptions(
    guests
      .filter((guest) => workloadMatchesViewMode(resolveWorkloadType(guest), viewMode))
      .flatMap((guest) => getWorkloadPlatformScopes(guest)),
  ).map((option) => ({
    value: option.key,
    label: option.label,
  }));
