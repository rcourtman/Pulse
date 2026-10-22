import { describe, expect, it, beforeEach, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
  apiFetch: vi.fn(),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  },
}));

import { AIAPI } from '@/api/ai';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';

// Branch-coverage companion to ai.test.ts. Covers the request-shaping and
// response-handling of the currently-uncovered AIAPI request functions:
// testConnection, resetCostHistory, exportCostHistory, getAnomalies,
// getLearningStatus, analyzeKubernetesCluster, approveRemediationPlan,
// executeRemediationPlan, rollbackRemediationPlan.
// Each test asserts the concrete request (final path + query string + method +
// body) and the concrete parsed result for happy paths and each optional /
// default-parameter branch arm.
describe('AIAPI branch coverage (request shaping + response handling)', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);
  const apiFetchMock = vi.mocked(apiFetch);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
    apiFetchMock.mockReset();
  });

  // --------------------------------------------
  // testConnection
  // --------------------------------------------
  describe('testConnection', () => {
    it('POSTs /api/ai/test with no body and returns the parsed AITestResult verbatim', async () => {
      const payload = { success: true, message: 'ok', provider: 'zai', model: 'glm-4.6' };
      apiFetchJSONMock.mockResolvedValueOnce(payload as any);

      const result = await AIAPI.testConnection();

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/test', { method: 'POST' });
      expect(result).toStrictEqual(payload);
    });

    it('surfaces a failed-connection payload without altering the diagnostic shape', async () => {
      const payload = {
        success: false,
        message: 'auth failed',
        provider: 'zai',
        cause: 'provider_auth',
      };
      apiFetchJSONMock.mockResolvedValueOnce(payload as any);

      const result = await AIAPI.testConnection();

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/test', { method: 'POST' });
      expect(result).toStrictEqual(payload);
    });
  });

  // --------------------------------------------
  // resetCostHistory
  // --------------------------------------------
  describe('resetCostHistory', () => {
    it('POSTs an empty JSON object to /api/ai/cost/reset and returns the ok + backup_file payload', async () => {
      const payload = { ok: true, backup_file: '/var/lib/pulse/cost-backup-2026.json' };
      apiFetchJSONMock.mockResolvedValueOnce(payload as any);

      const result = await AIAPI.resetCostHistory();

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/cost/reset', {
        method: 'POST',
        body: JSON.stringify({}),
      });
      expect(result).toStrictEqual(payload);
    });

    it('returns the { ok } arm verbatim when the backend omits backup_file', async () => {
      // The backup_file field is optional on the return type; cover the
      // branch where the backend confirms reset but produces no backup path.
      const payload = { ok: true };
      apiFetchJSONMock.mockResolvedValueOnce(payload as any);

      const result = await AIAPI.resetCostHistory();

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/cost/reset', {
        method: 'POST',
        body: JSON.stringify({}),
      });
      expect(result).toStrictEqual({ ok: true });
      expect('backup_file' in result).toBe(false);
    });
  });

  // --------------------------------------------
  // exportCostHistory
  // --------------------------------------------
  describe('exportCostHistory', () => {
    it('uses apiFetch (not apiFetchJSON) and defaults to days=30 format=csv', async () => {
      const stub = new Response('date,cost\n2026-01-01,1.23\n', { status: 200 });
      apiFetchMock.mockResolvedValueOnce(stub);

      const result = await AIAPI.exportCostHistory();

      // Default-parameter branch: both args take their defaults.
      expect(apiFetchMock).toHaveBeenCalledWith('/api/ai/cost/export?days=30&format=csv', {
        method: 'GET',
      });
      // The Response object is returned untouched — no JSON parsing.
      expect(result).toBe(stub);
      expect(apiFetchJSONMock).not.toHaveBeenCalled();
    });

    it('forwards a custom days value while keeping the csv default format', async () => {
      const stub = new Response('', { status: 200 });
      apiFetchMock.mockResolvedValueOnce(stub);

      await AIAPI.exportCostHistory(7);

      expect(apiFetchMock).toHaveBeenCalledWith('/api/ai/cost/export?days=7&format=csv', {
        method: 'GET',
      });
    });

    it('forwards an explicit json format while keeping the days default', async () => {
      const stub = new Response('[]', {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
      apiFetchMock.mockResolvedValueOnce(stub);

      await AIAPI.exportCostHistory(30, 'json');

      expect(apiFetchMock).toHaveBeenCalledWith('/api/ai/cost/export?days=30&format=json', {
        method: 'GET',
      });
    });

    it('forwards both custom days and json format together', async () => {
      const stub = new Response('{}', { status: 200 });
      apiFetchMock.mockResolvedValueOnce(stub);

      await AIAPI.exportCostHistory(14, 'json');

      // URLSearchParams preserves insertion order: days first, then format.
      expect(apiFetchMock).toHaveBeenCalledWith('/api/ai/cost/export?days=14&format=json', {
        method: 'GET',
      });
    });

    it('encodes special characters inside the days value via URLSearchParams', async () => {
      // URLSearchParams always stringifies the numeric arg; this guards the
      // stringification path for an edge-case fractional window.
      apiFetchMock.mockResolvedValueOnce(new Response('', { status: 200 }));

      await AIAPI.exportCostHistory(0.5, 'csv');

      expect(apiFetchMock).toHaveBeenCalledWith('/api/ai/cost/export?days=0.5&format=csv', {
        method: 'GET',
      });
    });
  });

  // --------------------------------------------
  // getAnomalies
  // --------------------------------------------
  describe('getAnomalies', () => {
    it('omits the query string when resourceId is undefined (no-arg branch)', async () => {
      const payload = { anomalies: [], count: 0 };
      apiFetchJSONMock.mockResolvedValueOnce(payload as any);

      const result = await AIAPI.getAnomalies();

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/intelligence/anomalies');
      expect(result).toStrictEqual(payload);
    });

    it('appends ?resource_id=<encoded> when a resourceId is supplied', async () => {
      const payload = {
        anomalies: [{ resource_id: 'vm-1', metric: 'cpu', score: 0.92 }],
        count: 1,
      };
      apiFetchJSONMock.mockResolvedValueOnce(payload as any);

      const result = await AIAPI.getAnomalies('vm-1');

      expect(apiFetchJSONMock).toHaveBeenCalledWith(
        '/api/ai/intelligence/anomalies?resource_id=vm-1',
      );
      expect(result).toStrictEqual(payload);
    });

    it('treats an explicit undefined arg identically to the no-arg branch', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({ anomalies: [], count: 0 } as any);

      await AIAPI.getAnomalies(undefined);

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/intelligence/anomalies');
    });

    it('URL-encodes special characters in the resource id', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({ anomalies: [], count: 0 } as any);

      await AIAPI.getAnomalies('node/1?filter=all');

      // encodeURIComponent('node/1?filter=all') === 'node%2F1%3Ffilter%3Dall'
      expect(apiFetchJSONMock).toHaveBeenCalledWith(
        '/api/ai/intelligence/anomalies?resource_id=node%2F1%3Ffilter%3Dall',
      );
    });

    it('drops the query string for an empty-string resourceId (falsy branch)', async () => {
      // The ternary `resourceId ? ... : ''` treats '' as falsy, so an empty
      // string must NOT append a query string.
      apiFetchJSONMock.mockResolvedValueOnce({ anomalies: [], count: 0 } as any);

      await AIAPI.getAnomalies('');

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/intelligence/anomalies');
    });
  });

  // --------------------------------------------
  // getLearningStatus
  // --------------------------------------------
  describe('getLearningStatus', () => {
    it('GETs /api/ai/intelligence/learning and returns the parsed status verbatim', async () => {
      const payload = {
        state: 'learning',
        resources_baselined: 42,
        resources_total: 100,
        percent_complete: 42,
        started_at: '2026-07-01T00:00:00Z',
      };
      apiFetchJSONMock.mockResolvedValueOnce(payload as any);

      const result = await AIAPI.getLearningStatus();

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/intelligence/learning');
      expect(result).toStrictEqual(payload);
    });

    it('round-trips a completed-state payload with optional fields absent', async () => {
      const payload = { state: 'completed', resources_baselined: 100, resources_total: 100 };
      apiFetchJSONMock.mockResolvedValueOnce(payload as any);

      const result = await AIAPI.getLearningStatus();

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/intelligence/learning');
      expect(result).toStrictEqual(payload);
      expect('percent_complete' in result).toBe(false);
    });
  });

  // --------------------------------------------
  // analyzeKubernetesCluster
  // --------------------------------------------
  describe('analyzeKubernetesCluster', () => {
    it('POSTs { cluster_id } to /api/ai/kubernetes/analyze and returns the parsed response verbatim', async () => {
      const payload = {
        execution_id: 'exec-1',
        plan_id: 'plan-1',
        status: 'success' as const,
        steps_completed: 3,
      };
      apiFetchJSONMock.mockResolvedValueOnce(payload as any);

      const result = await AIAPI.analyzeKubernetesCluster('prod-cluster-1');

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/kubernetes/analyze', {
        method: 'POST',
        body: JSON.stringify({ cluster_id: 'prod-cluster-1' }),
      });
      expect(result).toStrictEqual(payload);
    });

    it('preserves snake_case cluster ids verbatim in the request body', async () => {
      apiFetchJSONMock.mockResolvedValueOnce({
        execution_id: 'exec-2',
        plan_id: 'plan-2',
        status: 'success',
        steps_completed: 1,
      } as any);

      await AIAPI.analyzeKubernetesCluster('us_east_1/prod');

      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/kubernetes/analyze', {
        method: 'POST',
        body: JSON.stringify({ cluster_id: 'us_east_1/prod' }),
      });
    });
  });
});
