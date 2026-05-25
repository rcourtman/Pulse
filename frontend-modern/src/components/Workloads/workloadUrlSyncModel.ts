import type { ViewMode } from '@/types/workloads';
import {
  buildWorkloadsRouteSearch,
  parseWorkloadsLinkSearch,
  WORKLOADS_QUERY_PARAMS,
} from '@/routing/resourceLinks';
import { areSearchParamsEquivalent } from '@/utils/searchParams';
import { isContainerWorkloadViewMode, normalizeWorkloadViewModeParam } from '@/utils/workloads';

export interface WorkloadsWorkloadUrlParams {
  type: string;
  platform: string;
  runtime: string;
  context: string;
  namespace: string;
  agent: string;
  resource: string;
}

export interface WorkloadsWorkloadRuntimeParamResolution {
  forceViewMode: ViewMode | null;
  runtime: string;
  shouldApply: boolean;
}

interface WorkloadsManagedWorkloadsNavigateTargetOptions {
  containerRuntime: string;
  currentPathname: string;
  currentSearch: string;
  selectedHostHint: string | null;
  selectedPlatform: string | null;
  selectedKubernetesContext: string | null;
  selectedKubernetesNamespace: string | null;
  selectedNode: string | null;
  viewMode: ViewMode;
}

export const parseWorkloadsWorkloadUrlParams = (search: string): WorkloadsWorkloadUrlParams =>
  parseWorkloadsLinkSearch(search);

const hasWorkloadsWorkloadKubernetesScope = (params: WorkloadsWorkloadUrlParams): boolean =>
  Boolean(params.context.trim()) || Boolean(params.namespace.trim());

export const resolveWorkloadsWorkloadTypeParam = (
  params: WorkloadsWorkloadUrlParams,
): ViewMode | null => {
  const nextMode = normalizeWorkloadViewModeParam(params.type);
  if (!nextMode) return null;
  if (hasWorkloadsWorkloadKubernetesScope(params) && nextMode !== 'pod') return null;
  return nextMode;
};

export const resolveWorkloadsWorkloadRuntimeParam = (
  params: WorkloadsWorkloadUrlParams,
): WorkloadsWorkloadRuntimeParamResolution => {
  const nextMode = resolveWorkloadsWorkloadTypeParam(params);
  const runtimeRelevant =
    !hasWorkloadsWorkloadKubernetesScope(params) &&
    ((nextMode ? isContainerWorkloadViewMode(nextMode) : false) || !params.type.trim());

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
    forceViewMode: 'container',
    runtime: params.runtime,
    shouldApply: true,
  };
};

export const resolveWorkloadsManagedWorkloadsNavigateTarget = ({
  containerRuntime,
  currentPathname,
  currentSearch,
  selectedHostHint,
  selectedPlatform,
  selectedKubernetesContext,
  selectedKubernetesNamespace,
  selectedNode,
  viewMode,
}: WorkloadsManagedWorkloadsNavigateTargetOptions): string | null => {
  const currentParams = new URLSearchParams(currentSearch);
  const nextParams = new URLSearchParams(currentSearch);
  const nextType = viewMode === 'all' ? '' : viewMode;
  const nextPlatform = selectedPlatform ?? '';
  const nextRuntime = isContainerWorkloadViewMode(viewMode) ? containerRuntime.trim() : '';
  const nextContext = viewMode === 'pod' ? (selectedKubernetesContext ?? '') : '';
  const nextNamespace = viewMode === 'pod' ? (selectedKubernetesNamespace ?? '') : '';
  const nextAgent = viewMode === 'pod' ? '' : (selectedNode ?? selectedHostHint ?? '');

  const managedSearch = buildWorkloadsRouteSearch({
    type: nextType || null,
    platform: nextPlatform || null,
    runtime: nextRuntime || null,
    context: nextContext || null,
    namespace: nextNamespace || null,
    agent: nextAgent || null,
  });
  nextParams.delete(WORKLOADS_QUERY_PARAMS.type);
  nextParams.delete(WORKLOADS_QUERY_PARAMS.platform);
  nextParams.delete(WORKLOADS_QUERY_PARAMS.runtime);
  nextParams.delete(WORKLOADS_QUERY_PARAMS.context);
  nextParams.delete(WORKLOADS_QUERY_PARAMS.namespace);
  nextParams.delete(WORKLOADS_QUERY_PARAMS.agent);
  new URLSearchParams(managedSearch).forEach((value, key) => {
    nextParams.set(key, value);
  });

  if (areSearchParamsEquivalent(currentParams, nextParams)) {
    return null;
  }

  const nextSearch = nextParams.toString();
  return nextSearch ? `${currentPathname}?${nextSearch}` : currentPathname;
};
