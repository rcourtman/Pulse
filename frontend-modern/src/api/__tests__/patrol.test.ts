import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { getPatrolRunHistory, getPatrolRunHistoryWithToolCalls } from '@/api/patrol';
import { apiFetchJSON } from '@/utils/apiClient';

describe('patrol api', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
    apiFetchJSONMock.mockResolvedValue([] as any);
  });

  it('normalizes invalid limits for patrol history queries', async () => {
    await getPatrolRunHistory(Number.POSITIVE_INFINITY);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/runs?limit=30');

    await getPatrolRunHistory(0);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/runs?limit=1');
  });

  it('builds tool-call history query with normalized limit', async () => {
    await getPatrolRunHistoryWithToolCalls(25.9);
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/ai/patrol/runs?include=tool_calls&limit=25',
    );
  });

  it('normalizes patrol run alert identifiers', async () => {
    apiFetchJSONMock.mockResolvedValueOnce([
      {
        id: 'run-1',
        started_at: '2026-03-01T00:00:00Z',
        completed_at: '2026-03-01T00:01:00Z',
        duration_ms: 60000,
        type: 'patrol',
        resources_checked: 1,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        pbs_checked: 0,
        pmg_checked: 0,
        kubernetes_checked: 0,
        new_findings: 0,
        existing_findings: 0,
        rejected_findings: 0,
        resolved_findings: 0,
        findings_summary: 'ok',
        finding_ids: [],
        error_count: 0,
        status: 'healthy',
        tool_call_count: 0,
        alert_identifier: 'canonical-alert-1',
      },
    ] as any);

    const result = await getPatrolRunHistory();

    expect(result[0]).toMatchObject({
      alertIdentifier: 'canonical-alert-1',
    });
  });

  it('normalizes malformed patrol history payloads to empty arrays', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ runs: [] } as any);

    await expect(getPatrolRunHistory()).resolves.toEqual([]);
  });

  it('normalizes malformed tool-call patrol history payloads to empty arrays', async () => {
    apiFetchJSONMock.mockResolvedValueOnce(null as any);

    await expect(getPatrolRunHistoryWithToolCalls()).resolves.toEqual([]);
  });
});
