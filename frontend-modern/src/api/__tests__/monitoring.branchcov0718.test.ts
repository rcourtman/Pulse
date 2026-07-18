import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MonitoringAPI } from '../monitoring';
import { apiFetch } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetch: vi.fn(),
  apiFetchJSON: vi.fn(),
}));

describe('MonitoringAPI branch coverage (optional query params / empty-input guards / null-shape arms)', () => {
  const fetchMock = vi.mocked(apiFetch);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('deleteDockerRuntime — query-string assembly branches', () => {
    it('joins both hide and force flags in insertion order when both are present', async () => {
      // Exercises the combined branch where BOTH `if (options.hide)` and
      // `if (options.force)` arms fire. URLSearchParams iterates in insertion
      // order, so the on-the-wire query is `hide=true&force=true` (not the
      // reverse) and the `?` prefix is appended because `query` is non-empty.
      fetchMock.mockResolvedValueOnce({ ok: true, status: 204 } as unknown as Response);

      const result = await MonitoringAPI.deleteDockerRuntime('agent-1', {
        hide: true,
        force: true,
      });

      expect(fetchMock).toHaveBeenCalledWith(
        '/api/agents/docker/runtimes/agent-1?hide=true&force=true',
        expect.objectContaining({ method: 'DELETE' }),
      );
      // 204 is in the allowed-status set ([204, 404]) -> empty default payload.
      expect(result).toEqual({});
    });

    it('applies encodeURIComponent to agentIds containing path/space characters', async () => {
      // Exercises the encodeURIComponent branch on the path segment: a raw `/`
      // would otherwise be interpreted as a path separator and a raw space
      // would break the URL, so both must be percent-encoded.
      fetchMock.mockResolvedValueOnce({ ok: true, status: 204 } as unknown as Response);

      await MonitoringAPI.deleteDockerRuntime('agent/with space');

      expect(fetchMock).toHaveBeenCalledWith(
        '/api/agents/docker/runtimes/agent%2Fwith%20space',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });

    it('omits the query string entirely when options is the explicit empty object', async () => {
      // Exercises the falsy-query arm of `query ? \`?${query}\` : ''` — passing
      // an explicit `{}` produces an empty URLSearchParams and the URL must
      // have no trailing `?`. (Distinct from the default-undefined call already
      // covered in monitoring.test.ts.)
      fetchMock.mockResolvedValueOnce({ ok: true, status: 204 } as unknown as Response);

      await MonitoringAPI.deleteDockerRuntime('agent-1', {});

      expect(fetchMock).toHaveBeenCalledWith(
        '/api/agents/docker/runtimes/agent-1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });
  });

  describe('lookupAgent — query-param / null-shape branches', () => {
    it('builds a hostname-only query string when only hostname is supplied', async () => {
      // Exercises the `if (params.hostname)` arm in isolation (id arm untaken)
      // and confirms the resulting URL carries `?hostname=...` with no `id=`.
      fetchMock.mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            success: true,
            agent: {
              id: 'agent-h',
              hostname: 'host.local',
              status: 'online',
              connected: true,
              lastSeen: '2026-02-03T04:05:06Z',
            },
          }),
          { status: 200 },
        ),
      );

      const result = await MonitoringAPI.lookupAgent({ hostname: 'host.local' });

      expect(fetchMock).toHaveBeenCalledWith('/api/agents/agent/lookup?hostname=host.local');
      expect(result?.agent?.id).toBe('agent-h');
      // coerceTimestampMillis parses the ISO string into epoch millis.
      expect(result?.agent?.lastSeen).toBe(Date.parse('2026-02-03T04:05:06Z'));
    });

    it('joins id and hostname in insertion order when both are supplied', async () => {
      // Exercises BOTH `if (params.id)` and `if (params.hostname)` arms together.
      // Insertion order is id-first, so the query is `id=...&hostname=...`.
      fetchMock.mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            success: true,
            agent: {
              id: 'agent-1',
              hostname: 'host.local',
              status: 'online',
              connected: true,
              lastSeen: '2026-02-03T04:05:06Z',
            },
          }),
          { status: 200 },
        ),
      );

      await MonitoringAPI.lookupAgent({ id: 'agent-1', hostname: 'host.local' });

      expect(fetchMock).toHaveBeenCalledWith(
        '/api/agents/agent/lookup?id=agent-1&hostname=host.local',
      );
    });

    it('rejects with the canonical message and skips the transport when no identifier is supplied', async () => {
      // Exercises the `if (!search.toString())` guard arm — an empty params
      // object yields an empty URLSearchParams, so the function must throw
      // BEFORE issuing any request.
      await expect(MonitoringAPI.lookupAgent({})).rejects.toThrow(
        'Provide an agent identifier or hostname to look up.',
      );
      expect(fetchMock).not.toHaveBeenCalled();
    });

    it('returns null when the parsed payload omits the agent identity object', async () => {
      // Exercises the `if (!identity) return null` arm. The response parses to
      // a truthy object (so the earlier `if (!data)` arm is NOT taken), but the
      // nested `agent` field is absent — the function must still resolve null.
      fetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ success: true }), { status: 200 }),
      );

      const result = await MonitoringAPI.lookupAgent({ id: 'agent-x' });

      expect(fetchMock).toHaveBeenCalledWith('/api/agents/agent/lookup?id=agent-x');
      expect(result).toBeNull();
    });
  });

  describe('agent-management — empty-input guard arms', () => {
    it('deleteAgent throws the canonical message and skips the transport for an empty agentId', async () => {
      // Exercises the `if (!agentId)` guard arm in deleteAgent — no DELETE may
      // be issued against a missing identifier.
      await expect(MonitoringAPI.deleteAgent('')).rejects.toThrow(
        'Agent ID is required to remove an agent.',
      );
      expect(fetchMock).not.toHaveBeenCalled();
    });

    it('allowHostAgentReenroll throws the canonical message and skips the transport for an empty agentId', async () => {
      // Exercises the `if (!agentId)` guard arm in allowHostAgentReenroll.
      await expect(MonitoringAPI.allowHostAgentReenroll('')).rejects.toThrow(
        'Agent ID is required to allow reconnect.',
      );
      expect(fetchMock).not.toHaveBeenCalled();
    });

    it('updateAgentConfig throws the canonical message and skips the transport for an empty agentId', async () => {
      // Exercises the `if (!agentId)` guard arm in updateAgentConfig — the
      // PATCH body is never built or sent.
      await expect(
        MonitoringAPI.updateAgentConfig('', { commandsEnabled: true }),
      ).rejects.toThrow('Agent ID is required to update agent config.');
      expect(fetchMock).not.toHaveBeenCalled();
    });

    it('unlinkAgent throws the canonical message and skips the transport for an empty agentId', async () => {
      // Exercises the `if (!agentId)` guard arm in unlinkAgent — the POST
      // unlink body is never built or sent.
      await expect(MonitoringAPI.unlinkAgent('')).rejects.toThrow(
        'Agent ID is required to unlink an agent.',
      );
      expect(fetchMock).not.toHaveBeenCalled();
    });
  });

  describe('deleteAgent — body-read error swallow arm', () => {
    it('resolves successfully when consuming the response body throws after a 2xx delete', async () => {
      // Exercises the `try { await response.text() } catch (_err)` swallow arm.
      // The deletion already succeeded (ok:true), so a body-read failure must
      // NOT propagate — the connection-pool reuse comment in source documents
      // this contract; this test pins it.
      fetchMock.mockResolvedValueOnce({
        ok: true,
        status: 204,
        text: () => Promise.reject(new Error('body stream aborted')),
      } as unknown as Response);

      await expect(MonitoringAPI.deleteAgent('agent-1')).resolves.toBeUndefined();
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/agents/agent/agent-1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });
  });
});
