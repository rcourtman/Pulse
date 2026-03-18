import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import {
  asMetricsHistoryResourceType,
  canonicalizeMetricsHistoryTargetType,
  ChartsAPI,
  mapUnifiedTypeToHistoryResourceType,
  toMetricsHistoryAPIResourceType,
} from '@/api/charts';
import { apiFetchJSON } from '@/utils/apiClient';

describe('ChartsAPI', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('calls /api/charts with default range', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);

    await ChartsAPI.getCharts();

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts?range=1h', { signal: undefined });
  });

  it('forwards an abort signal when provided', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);
    const controller = new AbortController();

    await ChartsAPI.getCharts('24h', controller.signal);

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts?range=24h', {
      signal: controller.signal,
    });
  });

  it('adds node query for host-scoped workloads summary', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);

    await ChartsAPI.getWorkloadsSummaryCharts('1h', undefined, { nodeId: 'cluster-a-node-1' });

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/charts/workloads-summary?range=1h&node=cluster-a-node-1',
      { signal: undefined },
    );
  });

  it('calls workload-only charts endpoint with node and maxPoints', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);

    await ChartsAPI.getWorkloadCharts('1h', undefined, {
      nodeId: 'cluster-a-node-1',
      maxPoints: 180.2,
    });

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/charts/workloads?range=1h&node=cluster-a-node-1&maxPoints=180',
      { signal: undefined },
    );
  });

  it('builds metrics history query params including maxPoints', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);

    await ChartsAPI.getMetricsHistory({
      resourceType: 'agent',
      resourceId: 'agent-1',
      metric: 'cpu',
      range: '7d',
      maxPoints: 321.4,
    });

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/metrics-store/history?resourceType=agent&resourceId=agent-1&metric=cpu&range=7d&maxPoints=321',
    );
  });

  it('passes through agent metrics history requests', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);

    await ChartsAPI.getMetricsHistory({
      resourceType: 'agent',
      resourceId: 'agent-1',
      metric: 'cpu',
      range: '24h',
    });

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/metrics-store/history?resourceType=agent&resourceId=agent-1&metric=cpu&range=24h',
    );
  });

  it('maps canonical kubernetes resource types to the backend k8s history token', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);

    await ChartsAPI.getMetricsHistory({
      resourceType: 'pod',
      resourceId: 'k8s:cluster-a:pod:api-1',
      metric: 'cpu',
      range: '24h',
    });

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/metrics-store/history?resourceType=k8s&resourceId=k8s%3Acluster-a%3Apod%3Aapi-1&metric=cpu&range=24h',
    );
    expect(toMetricsHistoryAPIResourceType('k8s-cluster')).toBe('k8s');
    expect(toMetricsHistoryAPIResourceType('k8s-node')).toBe('k8s');
    expect(toMetricsHistoryAPIResourceType('pod')).toBe('k8s');
  });

  it('exposes shared metrics history resource type helpers', () => {
    expect(asMetricsHistoryResourceType('agent')).toBe('agent');
    expect(asMetricsHistoryResourceType('disk')).toBe('disk');
    expect(asMetricsHistoryResourceType('host')).toBeNull();

    expect(mapUnifiedTypeToHistoryResourceType('truenas')).toBe('agent');
    expect(mapUnifiedTypeToHistoryResourceType('pod')).toBe('pod');
    expect(mapUnifiedTypeToHistoryResourceType('container')).toBeNull();

    expect(canonicalizeMetricsHistoryTargetType('node', 'agent')).toBe('agent');
    expect(canonicalizeMetricsHistoryTargetType('k8s', 'k8s-node')).toBe('k8s-node');
    expect(canonicalizeMetricsHistoryTargetType('k8s', 'pod')).toBe('pod');
    expect(canonicalizeMetricsHistoryTargetType('k8s', 'agent')).toBeNull();
    expect(canonicalizeMetricsHistoryTargetType('agent', 'agent')).toBe('agent');
  });
});
