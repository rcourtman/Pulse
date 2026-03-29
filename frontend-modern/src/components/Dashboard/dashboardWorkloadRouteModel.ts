import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import { normalizeWorkloadViewModeParam, resolveWorkloadType } from '@/utils/workloads';
import { buildSourcePlatformOptions } from '@/utils/sourcePlatformOptions';
import type { DashboardFilterSelectOption } from './dashboardFilterModel';
import { getKubernetesContextKey, workloadNodeScopeId } from './workloadTopology';

export type DashboardWorkloadNodeOption = DashboardFilterSelectOption;

export const deserializeDashboardWorkloadViewMode = (raw: unknown): ViewMode => {
  if (typeof raw !== 'string') return 'all';
  return normalizeWorkloadViewModeParam(raw) ?? 'all';
};

export const buildDashboardWorkloadNodeOptions = (
  guests: WorkloadGuest[],
): DashboardWorkloadNodeOption[] => {
  const labelsByScope = new Map<string, string>();
  const nodeNameCounts = new Map<string, number>();

  for (const guest of guests) {
    const type = resolveWorkloadType(guest);
    if (type === 'pod') continue;
    const scope = workloadNodeScopeId(guest);
    if (!scope || scope === '-') continue;
    const nodeName = (guest.node || '').trim();
    if (!nodeName) continue;
    nodeNameCounts.set(nodeName, (nodeNameCounts.get(nodeName) || 0) + 1);
  }

  for (const guest of guests) {
    const type = resolveWorkloadType(guest);
    if (type === 'pod') continue;
    const scope = workloadNodeScopeId(guest);
    if (!scope || scope === '-' || labelsByScope.has(scope)) continue;
    const nodeName = (guest.node || '').trim();
    const instance = (guest.instance || '').trim();
    if (!nodeName) continue;
    const hasDuplicateNodeName = (nodeNameCounts.get(nodeName) || 0) > 1;
    const label = hasDuplicateNodeName && instance ? `${nodeName} (${instance})` : nodeName;
    labelsByScope.set(scope, label);
  }

  return Array.from(labelsByScope.entries())
    .map(([value, label]) => ({ value, label }))
    .sort((a, b) => a.label.localeCompare(b.label));
};

export const buildDashboardKubernetesContextOptions = (guests: WorkloadGuest[]): string[] => {
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

export const buildDashboardKubernetesNamespaceOptions = (
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

export const buildDashboardContainerRuntimeOptions = (guests: WorkloadGuest[]): string[] => {
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

export const buildDashboardPlatformOptions = (
  guests: WorkloadGuest[],
  viewMode: ViewMode,
): DashboardFilterSelectOption[] =>
  buildSourcePlatformOptions(
    guests
      .filter((guest) => viewMode === 'all' || resolveWorkloadType(guest) === viewMode)
      .map((guest) => guest.platformType || ''),
  ).map((option) => ({
    value: option.key,
    label: option.label,
  }));
