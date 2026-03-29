import type { ViewMode } from '@/types/workloads';
import {
  buildWorkloadsPath,
  parseWorkloadsLinkSearch,
  WORKLOADS_PATH,
  WORKLOADS_QUERY_PARAMS,
} from '@/routing/resourceLinks';
import { areSearchParamsEquivalent } from '@/utils/searchParams';
import { normalizeWorkloadViewModeParam } from '@/utils/workloads';

export interface DashboardWorkloadUrlParams {
  type: string;
  platform: string;
  runtime: string;
  context: string;
  namespace: string;
  agent: string;
  resource: string;
}

export interface DashboardWorkloadRuntimeParamResolution {
  forceViewMode: ViewMode | null;
  runtime: string;
  shouldApply: boolean;
}

interface DashboardManagedWorkloadsNavigateTargetOptions {
  containerRuntime: string;
  currentSearch: string;
  selectedHostHint: string | null;
  selectedPlatform: string | null;
  selectedKubernetesContext: string | null;
  selectedKubernetesNamespace: string | null;
  selectedNode: string | null;
  viewMode: ViewMode;
}

export const parseDashboardWorkloadUrlParams = (search: string): DashboardWorkloadUrlParams =>
  parseWorkloadsLinkSearch(search);

const hasDashboardWorkloadKubernetesScope = (params: DashboardWorkloadUrlParams): boolean =>
  Boolean(params.context.trim()) || Boolean(params.namespace.trim());

export const resolveDashboardWorkloadTypeParam = (
  params: DashboardWorkloadUrlParams,
): ViewMode | null => {
  const nextMode = normalizeWorkloadViewModeParam(params.type);
  if (!nextMode) return null;
  if (hasDashboardWorkloadKubernetesScope(params) && nextMode !== 'pod') return null;
  return nextMode;
};

export const resolveDashboardWorkloadRuntimeParam = (
  params: DashboardWorkloadUrlParams,
): DashboardWorkloadRuntimeParamResolution => {
  const nextMode = resolveDashboardWorkloadTypeParam(params);
  const runtimeRelevant =
    !hasDashboardWorkloadKubernetesScope(params) &&
    (nextMode === 'app-container' || !params.type.trim());

  if (!runtimeRelevant) {
    return {
      forceViewMode: null,
      runtime: params.runtime,
      shouldApply: false,
    };
  }

  if (!params.runtime.trim()) {
    return {
      forceViewMode: null,
      runtime: '',
      shouldApply: true,
    };
  }

  return {
    forceViewMode: 'app-container',
    runtime: params.runtime,
    shouldApply: true,
  };
};

export const resolveDashboardManagedWorkloadsNavigateTarget = ({
  containerRuntime,
  currentSearch,
  selectedHostHint,
  selectedPlatform,
  selectedKubernetesContext,
  selectedKubernetesNamespace,
  selectedNode,
  viewMode,
}: DashboardManagedWorkloadsNavigateTargetOptions): string | null => {
  const currentParams = new URLSearchParams(currentSearch);
  const nextParams = new URLSearchParams(currentSearch);
  const nextType = viewMode === 'all' ? '' : viewMode;
  const nextPlatform = selectedPlatform ?? '';
  const nextRuntime = viewMode === 'app-container' ? containerRuntime.trim() : '';
  const nextContext = viewMode === 'pod' ? (selectedKubernetesContext ?? '') : '';
  const nextNamespace = viewMode === 'pod' ? (selectedKubernetesNamespace ?? '') : '';
  const nextAgent = viewMode === 'pod' ? '' : (selectedNode ?? selectedHostHint ?? '');

  const managedPath = buildWorkloadsPath({
    type: nextType || null,
    platform: nextPlatform || null,
    runtime: nextRuntime || null,
    context: nextContext || null,
    namespace: nextNamespace || null,
    agent: nextAgent || null,
  });
  const managedUrl = new URL(managedPath, 'http://pulse.local');
  nextParams.delete(WORKLOADS_QUERY_PARAMS.type);
  nextParams.delete(WORKLOADS_QUERY_PARAMS.platform);
  nextParams.delete(WORKLOADS_QUERY_PARAMS.runtime);
  nextParams.delete(WORKLOADS_QUERY_PARAMS.context);
  nextParams.delete(WORKLOADS_QUERY_PARAMS.namespace);
  nextParams.delete(WORKLOADS_QUERY_PARAMS.agent);
  managedUrl.searchParams.forEach((value, key) => {
    nextParams.set(key, value);
  });

  if (areSearchParamsEquivalent(currentParams, nextParams)) {
    return null;
  }

  const nextSearch = nextParams.toString();
  return nextSearch ? `${WORKLOADS_PATH}?${nextSearch}` : WORKLOADS_PATH;
};
