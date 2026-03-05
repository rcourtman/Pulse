import { describe, expect, it, vi, beforeEach } from 'vitest';
import { MonitoringAPI } from '../monitoring';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetch: vi.fn(),
  apiFetchJSON: vi.fn(),
}));

describe('MonitoringAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getState', () => {
    it('fetches state', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({});

      await MonitoringAPI.getState();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/state');
    });
  });

  describe('getPerformance', () => {
    it('fetches performance', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({});

      await MonitoringAPI.getPerformance();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/performance');
    });
  });

  describe('getStats', () => {
    it('fetches stats', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({});

      await MonitoringAPI.getStats();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/stats');
    });
  });

  describe('exportDiagnostics', () => {
    it('exports diagnostics as blob', async () => {
      const mockBlob = new Blob(['diagnostics data']);
      const mockResponse = {
        ok: true,
        blob: () => Promise.resolve(mockBlob),
      } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);

      const result = await MonitoringAPI.exportDiagnostics();

      expect(apiFetch).toHaveBeenCalledWith('/api/diagnostics/export');
      expect(result).toBe(mockBlob);
    });
  });

  describe('deleteDockerHost', () => {
    it('deletes docker host', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true, status: 204 } as unknown as Response);

      await MonitoringAPI.deleteDockerHost('host-1');

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/agents/docker/runtimes/host-1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });

    it('handles hide option', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true, status: 204 } as unknown as Response);

      await MonitoringAPI.deleteDockerHost('host-1', { hide: true });

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/agents/docker/runtimes/host-1?hide=true',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });

    it('handles force option', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true, status: 204 } as unknown as Response);

      await MonitoringAPI.deleteDockerHost('host-1', { force: true });

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/agents/docker/runtimes/host-1?force=true',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });

    it('returns empty object on 404', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: false, status: 404 } as unknown as Response);

      const result = await MonitoringAPI.deleteDockerHost('host-1');

      expect(result).toEqual({});
    });
  });

  describe('unhideDockerHost', () => {
    it('unhides docker host', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true } as unknown as Response);

      await MonitoringAPI.unhideDockerHost('host-1');

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/agents/docker/runtimes/host-1/unhide',
        expect.objectContaining({ method: 'PUT' }),
      );
    });
  });

  describe('markDockerHostPendingUninstall', () => {
    it('marks host for uninstall', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true } as unknown as Response);

      await MonitoringAPI.markDockerHostPendingUninstall('host-1');

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/agents/docker/runtimes/host-1/pending-uninstall',
        expect.objectContaining({ method: 'PUT' }),
      );
    });
  });

  describe('setDockerHostDisplayName', () => {
    it('sets display name', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true } as unknown as Response);

      await MonitoringAPI.setDockerHostDisplayName('host-1', 'New Name');

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/agents/docker/runtimes/host-1/display-name',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ displayName: 'New Name' }),
        }),
      );
    });
  });

  describe('agent management', () => {
    it('deletes agent via unified backend route', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve(''),
      } as unknown as Response);

      await MonitoringAPI.deleteAgent('agent-1');

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/agents/agent/agent-1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });

    it('updates agent config via unified backend route', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true } as unknown as Response);

      await MonitoringAPI.updateAgentConfig('agent-1', { commandsEnabled: true });

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/agents/agent/agent-1/config',
        expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify({ commandsEnabled: true }),
        }),
      );
    });

    it('looks up agent and normalizes timestamp', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            success: true,
            agent: {
              id: 'agent-1',
              hostname: 'agent-1.local',
              status: 'online',
              connected: true,
              lastSeen: '2026-01-01T00:00:00Z',
            },
          }),
          { status: 200 },
        ),
      );

      const result = await MonitoringAPI.lookupAgent({ id: 'agent-1' });

      expect(apiFetch).toHaveBeenCalledWith('/api/agents/agent/lookup?id=agent-1');
      expect(result?.agent?.id).toBe('agent-1');
      expect(typeof result?.agent?.lastSeen).toBe('number');
    });

    it('normalizes agent-first lookup payload', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            success: true,
            agent: {
              id: 'agent-2',
              hostname: 'agent-2.local',
              status: 'online',
              connected: true,
              lastSeen: '2026-01-01T00:00:00Z',
            },
          }),
          { status: 200 },
        ),
      );

      const result = await MonitoringAPI.lookupAgent({ id: 'agent-2' });

      expect(result?.agent?.id).toBe('agent-2');
      expect(typeof result?.agent?.lastSeen).toBe('number');
    });

    it('unlinks agent using canonical agentId', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true } as unknown as Response);

      await MonitoringAPI.unlinkAgent('agent-1');

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/agents/agent/unlink',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            agentId: 'agent-1',
          }),
        }),
      );
    });
  });

  describe('deleteKubernetesCluster', () => {
    it('deletes kubernetes cluster', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true, status: 204 } as unknown as Response);

      await MonitoringAPI.deleteKubernetesCluster('cluster-1');

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/agents/kubernetes/clusters/cluster-1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });
  });

  describe('unhideKubernetesCluster', () => {
    it('unhides kubernetes cluster', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true } as unknown as Response);

      await MonitoringAPI.unhideKubernetesCluster('cluster-1');

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/agents/kubernetes/clusters/cluster-1/unhide',
        expect.objectContaining({ method: 'PUT' }),
      );
    });
  });
});
