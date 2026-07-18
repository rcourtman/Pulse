import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetch: vi.fn(),
}));

import {
  deleteDiscovery,
  getConnectedAgents,
  getDiscoveryProgress,
  getDiscoveryStatus,
  listDiscoveries,
  listDiscoveriesByAgent,
  updateDiscoveryNotes,
  getDiscovery,
} from '@/api/discovery';
import { apiFetch } from '@/utils/apiClient';
import type {
  DiscoveryListResponse,
  DiscoveryProgress,
  DiscoveryStatus,
  DiscoverySummary,
  ResourceDiscovery,
  UpdateNotesRequest,
} from '@/types/discovery';

const okJson = (body: unknown, status = 200): Response =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });

const progressFixture = (): DiscoveryProgress => ({
  resource_id: '100',
  status: 'running',
  current_step: 'collecting facts',
  current_command: 'uname -a',
  total_steps: 5,
  completed_steps: 2,
  elapsed_ms: 1234,
  percent_complete: 40,
  started_at: '2026-07-18T00:00:00Z',
  updated_at: '2026-07-18T00:00:01Z',
});

const statusFixture = (): DiscoveryStatus => ({
  running: true,
  last_run: '2026-07-18T00:00:00Z',
  interval: '0 */6 * * *',
  cache_size: 12,
  ai_analyzer_set: true,
  scanner_set: true,
  store_set: true,
  max_discovery_age: '720h',
  fingerprint_count: 7,
  last_fingerprint_scan: '2026-07-18T00:00:00Z',
  changed_count: 1,
  stale_count: 0,
});

const discoveryDetailFixture = (): ResourceDiscovery => ({
  id: 'vm:node-1:100',
  resource_type: 'vm',
  resource_id: '100',
  target_id: 'node-1',
  hostname: 'vm-100',
  service_type: 'linux',
  service_name: 'VM',
  service_version: '1.0',
  category: 'unknown',
  cli_access: '',
  facts: [],
  config_paths: [],
  data_paths: [],
  log_paths: [],
  ports: [],
  user_notes: 'updated notes',
  user_secrets: { token: 'abc' },
  confidence: 0.7,
  ai_reasoning: '',
  discovered_at: '2026-07-18T00:00:00Z',
  updated_at: '2026-07-18T00:00:01Z',
  scan_duration: 1,
});

describe('discovery api branch coverage', () => {
  const apiFetchMock = vi.mocked(apiFetch);

  beforeEach(() => {
    apiFetchMock.mockReset();
  });

  describe('buildTypedDiscoverySubresourcePath / getDiscoveryProgress', () => {
    it('GETs the typed progress subresource and parses the payload', async () => {
      apiFetchMock.mockResolvedValueOnce(okJson(progressFixture()));

      const result = await getDiscoveryProgress('vm', 'node-1', '100');

      expect(apiFetchMock).toHaveBeenCalledTimes(1);
      expect(apiFetchMock).toHaveBeenCalledWith(
        '/api/discovery/vm/node-1/100/progress',
      );
      expect(result).toMatchObject({
        resource_id: '100',
        status: 'running',
        percent_complete: 40,
        completed_steps: 2,
        total_steps: 5,
      });
    });

    it('maps pod -> k8s when building the progress subresource path', async () => {
      apiFetchMock.mockResolvedValueOnce(okJson(progressFixture()));

      await getDiscoveryProgress('pod', 'cluster-a', 'default/api');

      expect(apiFetchMock).toHaveBeenCalledWith(
        '/api/discovery/k8s/cluster-a/default%2Fapi/progress',
      );
    });

    it('encodes slashes in the target and resource id segments', async () => {
      apiFetchMock.mockResolvedValueOnce(okJson(progressFixture()));

      await getDiscoveryProgress(
        'vm/root' as never,
        'node/1' as never,
        'id/with/slash' as never,
      );

      expect(apiFetchMock).toHaveBeenCalledWith(
        '/api/discovery/vm%2Froot/node%2F1/id%2Fwith%2Fslash/progress',
      );
    });

    it('throws a request error on non-ok progress response', async () => {
      apiFetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'scanner offline' }), {
          status: 503,
        }),
      );

      await expect(getDiscoveryProgress('vm', 'node-1', '100')).rejects.toThrow(
        'scanner offline',
      );
    });

    it('throws a parse error when progress body is empty', async () => {
      apiFetchMock.mockResolvedValueOnce(new Response('', { status: 200 }));

      await expect(getDiscoveryProgress('vm', 'node-1', '100')).rejects.toThrow(
        'Failed to parse discovery progress',
      );
    });
  });

  describe('listDiscoveries', () => {
    it('GETs the discovery root and returns the parsed list payload', async () => {
      const payload: DiscoveryListResponse = {
        discoveries: [
          {
            id: 'vm:node-1:100',
            resource_type: 'vm',
            resource_id: '100',
            target_id: 'node-1',
            hostname: 'vm-100',
            service_type: 'linux',
            service_name: 'VM',
            service_version: '',
            category: 'unknown',
            confidence: 0.7,
            has_user_notes: false,
            updated_at: '2026-07-18T00:00:00Z',
          },
        ],
        total: 1,
      };
      apiFetchMock.mockResolvedValueOnce(okJson(payload));

      const result = await listDiscoveries();

      expect(apiFetchMock).toHaveBeenCalledTimes(1);
      expect(apiFetchMock).toHaveBeenCalledWith('/api/discovery');
      expect(result.total).toBe(1);
      expect(result.discoveries).toHaveLength(1);
      expect(result.discoveries[0].id).toBe('vm:node-1:100');
    });

    it('returns the empty list payload unchanged', async () => {
      apiFetchMock.mockResolvedValueOnce(
        okJson({ discoveries: [], total: 0 } satisfies DiscoveryListResponse),
      );

      const result = await listDiscoveries();

      expect(result.total).toBe(0);
      expect(result.discoveries).toEqual([]);
    });

    it('surfaces backend error messages on non-ok list responses', async () => {
      apiFetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ message: 'discovery store offline' }), {
          status: 500,
        }),
      );

      await expect(listDiscoveries()).rejects.toThrow('discovery store offline');
    });

    it('throws a parse error on empty body', async () => {
      apiFetchMock.mockResolvedValueOnce(new Response('', { status: 200 }));

      await expect(listDiscoveries()).rejects.toThrow('Failed to parse discoveries');
    });
  });

  describe('listDiscoveriesByAgent', () => {
    it('GETs the agent collection route with an encoded agent id', async () => {
      apiFetchMock.mockResolvedValueOnce(
        okJson({ discoveries: [], total: 0 } satisfies DiscoveryListResponse),
      );

      await listDiscoveriesByAgent('host-1');

      expect(apiFetchMock).toHaveBeenCalledTimes(1);
      expect(apiFetchMock).toHaveBeenCalledWith('/api/discovery/agent/host-1');
    });

    it('encodes special characters in the agent id segment', async () => {
      apiFetchMock.mockResolvedValueOnce(
        okJson({ discoveries: [], total: 0 } satisfies DiscoveryListResponse),
      );

      await listDiscoveriesByAgent('agent/with slash');

      expect(apiFetchMock).toHaveBeenCalledWith(
        '/api/discovery/agent/agent%2Fwith%20slash',
      );
    });

    it('returns the parsed agent discovery list', async () => {
      apiFetchMock.mockResolvedValueOnce(
        okJson({
          discoveries: [
            {
              id: 'agent:host-1:host-1',
              resource_type: 'agent',
              resource_id: 'host-1',
              target_id: 'host-1',
              hostname: 'host-1.local',
              service_type: 'linux',
              service_name: 'Agent',
              service_version: '',
              category: 'unknown',
              confidence: 0.9,
              has_user_notes: false,
              updated_at: '2026-07-18T00:00:00Z',
            },
          ],
          total: 1,
        } satisfies DiscoveryListResponse),
      );

      const result = await listDiscoveriesByAgent('host-1');

      expect(result.total).toBe(1);
      expect(result.discoveries[0].hostname).toBe('host-1.local');
    });

    it('throws backend error message on non-ok agent list response', async () => {
      apiFetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'agent not connected' }), {
          status: 404,
        }),
      );

      await expect(listDiscoveriesByAgent('ghost')).rejects.toThrow(
        'agent not connected',
      );
    });
  });

  describe('updateDiscoveryNotes', () => {
    it('PUTs notes + secrets body to the typed notes subresource', async () => {
      apiFetchMock.mockResolvedValueOnce(okJson(discoveryDetailFixture()));

      const notes: UpdateNotesRequest = {
        user_notes: 'updated notes',
        user_secrets: { token: 'abc' },
      };

      const result = await updateDiscoveryNotes('vm', 'node-1', '100', notes);

      expect(apiFetchMock).toHaveBeenCalledTimes(1);
      expect(apiFetchMock).toHaveBeenCalledWith('/api/discovery/vm/node-1/100/notes', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(notes),
      });
      expect(result.id).toBe('vm:node-1:100');
      expect(result.user_notes).toBe('updated notes');
    });

    it('PUTs notes-only body when user_secrets is omitted (optional param absent)', async () => {
      apiFetchMock.mockResolvedValueOnce(okJson(discoveryDetailFixture()));

      const notes: UpdateNotesRequest = { user_notes: 'just text' };

      await updateDiscoveryNotes('agent', 'host-1', 'host-1', notes);

      expect(apiFetchMock).toHaveBeenCalledWith(
        '/api/discovery/agent/host-1/host-1/notes',
        {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ user_notes: 'just text' }),
        },
      );
    });

    it('maps pod -> k8s for the notes subresource', async () => {
      apiFetchMock.mockResolvedValueOnce(okJson(discoveryDetailFixture()));

      await updateDiscoveryNotes('pod', 'cluster-a', 'default/api', {
        user_notes: 'k8s notes',
      });

      expect(apiFetchMock).toHaveBeenCalledWith(
        '/api/discovery/k8s/cluster-a/default%2Fapi/notes',
        expect.objectContaining({ method: 'PUT' }),
      );
    });

    it('throws on non-ok notes update response', async () => {
      apiFetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'note too long' }), { status: 413 }),
      );

      await expect(
        updateDiscoveryNotes('vm', 'node-1', '100', { user_notes: 'x'.repeat(10_000) }),
      ).rejects.toThrow('note too long');
    });
  });

  describe('deleteDiscovery', () => {
    it('sends DELETE without a body and resolves void on ok', async () => {
      apiFetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

      await expect(deleteDiscovery('vm', 'node-1', '100')).resolves.toBeUndefined();

      expect(apiFetchMock).toHaveBeenCalledTimes(1);
      expect(apiFetchMock).toHaveBeenCalledWith('/api/discovery/vm/node-1/100', {
        method: 'DELETE',
      });
    });

    it('encodes a pod/k8s delete route', async () => {
      apiFetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

      await deleteDiscovery('pod', 'cluster-a', 'default/api');

      expect(apiFetchMock).toHaveBeenCalledWith(
        '/api/discovery/k8s/cluster-a/default%2Fapi',
        { method: 'DELETE' },
      );
    });

    it('throws the backend error message on non-ok delete response', async () => {
      apiFetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'discovery in use' }), { status: 409 }),
      );

      await expect(deleteDiscovery('vm', 'node-1', '100')).rejects.toThrow(
        'discovery in use',
      );
    });

    it('falls back to the default message when the error body is empty', async () => {
      apiFetchMock.mockResolvedValueOnce(new Response('', { status: 500 }));

      await expect(deleteDiscovery('vm', 'node-1', '100')).rejects.toThrow(
        'Failed to delete discovery',
      );
    });
  });

  describe('getDiscoveryStatus', () => {
    it('GETs /api/discovery/status and parses the status payload', async () => {
      apiFetchMock.mockResolvedValueOnce(okJson(statusFixture()));

      const result = await getDiscoveryStatus();

      expect(apiFetchMock).toHaveBeenCalledTimes(1);
      expect(apiFetchMock).toHaveBeenCalledWith('/api/discovery/status');
      expect(result.running).toBe(true);
      expect(result.fingerprint_count).toBe(7);
      expect(result.changed_count).toBe(1);
    });

    it('parses a status payload that omits the optional fingerprint fields', async () => {
      apiFetchMock.mockResolvedValueOnce(
        okJson({
          running: false,
          last_run: '',
          interval: '',
          cache_size: 0,
          ai_analyzer_set: false,
          scanner_set: false,
          store_set: false,
        } satisfies DiscoveryStatus),
      );

      const result = await getDiscoveryStatus();

      expect(result.running).toBe(false);
      expect(result.fingerprint_count).toBeUndefined();
      expect(result.changed_count).toBeUndefined();
    });

    it('throws backend error on non-ok status response', async () => {
      apiFetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ message: 'status unavailable' }), {
          status: 503,
        }),
      );

      await expect(getDiscoveryStatus()).rejects.toThrow('status unavailable');
    });
  });

  describe('getConnectedAgents', () => {
    it('GETs /api/ai/agents and parses the connected-agent list', async () => {
      apiFetchMock.mockResolvedValueOnce(
        okJson({
          count: 2,
          agents: [
            {
              agent_id: 'host-1',
              hostname: 'host-1.local',
              version: '1.2.3',
              platform: 'linux',
              connected_at: '2026-07-18T00:00:00Z',
            },
            {
              agent_id: 'host-2',
              hostname: 'host-2.local',
              version: '1.2.3',
              platform: 'darwin',
              connected_at: '2026-07-18T00:00:01Z',
            },
          ],
        }),
      );

      const result = await getConnectedAgents();

      expect(apiFetchMock).toHaveBeenCalledTimes(1);
      expect(apiFetchMock).toHaveBeenCalledWith('/api/ai/agents');
      expect(result.count).toBe(2);
      expect(result.agents).toHaveLength(2);
      expect(result.agents[0].agent_id).toBe('host-1');
    });

    it('returns an empty agent list unchanged', async () => {
      apiFetchMock.mockResolvedValueOnce(okJson({ count: 0, agents: [] }));

      const result = await getConnectedAgents();

      expect(result.count).toBe(0);
      expect(result.agents).toEqual([]);
    });

    it('throws backend error on non-ok connected-agents response', async () => {
      apiFetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'websocket server down' }), {
          status: 502,
        }),
      );

      await expect(getConnectedAgents()).rejects.toThrow('websocket server down');
    });
  });

  describe('getDiscovery agent-resolution branches', () => {
    it('matches a summary where resource_id equals the requested target_id (not resourceId)', async () => {
      const summary: DiscoverySummary = {
        id: 'agent:host-1:agent-X',
        resource_type: 'agent',
        resource_id: 'agent-X',
        target_id: 'host-1',
        hostname: 'host-1.local',
        service_type: 'linux',
        service_name: 'Agent',
        service_version: '',
        category: 'unknown',
        confidence: 0.9,
        has_user_notes: false,
        updated_at: '2026-07-18T00:00:00Z',
      };
      apiFetchMock.mockResolvedValueOnce(
        okJson({ discoveries: [summary], total: 1 } satisfies DiscoveryListResponse),
      );
      apiFetchMock.mockResolvedValueOnce(
        okJson({
          ...discoveryDetailFixture(),
          id: 'agent:host-1:agent-X',
          resource_type: 'agent',
          target_id: 'host-1',
        }),
      );

      const result = await getDiscovery('agent', 'host-1', 'does-not-match');

      expect(result?.id).toBe('agent:host-1:agent-X');
      // resolveDiscoveryAgentId picks target_id ('host-1') because agent_id is absent
      expect(apiFetchMock).toHaveBeenNthCalledWith(
        2,
        '/api/discovery/agent/host-1/agent-X',
      );
    });

    it('matches a summary through resolveDiscoveryAgentId when target_id equals the request', async () => {
      const summary: DiscoverySummary = {
        id: 'agent:legacy:agent-7',
        resource_type: 'agent',
        resource_id: 'agent-7',
        agent_id: 'agent-7-canonical',
        target_id: 'agent-7-canonical',
        hostname: 'agent-7.local',
        service_type: 'linux',
        service_name: 'Agent',
        service_version: '',
        category: 'unknown',
        confidence: 0.9,
        has_user_notes: false,
        updated_at: '2026-07-18T00:00:00Z',
      };
      apiFetchMock.mockResolvedValueOnce(
        okJson({ discoveries: [summary], total: 1 } satisfies DiscoveryListResponse),
      );
      apiFetchMock.mockResolvedValueOnce(
        okJson({
          ...discoveryDetailFixture(),
          id: 'agent:agent-7-canonical:agent-7',
          resource_type: 'agent',
          target_id: 'agent-7-canonical',
        }),
      );

      const result = await getDiscovery('agent', 'agent-7-canonical', 'nope');

      expect(result?.id).toBe('agent:agent-7-canonical:agent-7');
      // First find() matches because resolveDiscoveryAgentId(d) === targetId
      expect(apiFetchMock).toHaveBeenNthCalledWith(
        2,
        '/api/discovery/agent/agent-7-canonical/agent-7',
      );
    });

    it('falls back to the first agent discovery when nothing matches the predicate', async () => {
      const summary: DiscoverySummary = {
        id: 'agent:other:any-id',
        resource_type: 'agent',
        resource_id: 'unrelated-id',
        target_id: 'unrelated-target',
        agent_id: 'resolved-canonical',
        hostname: 'agent-other.local',
        service_type: 'linux',
        service_name: 'Agent',
        service_version: '',
        category: 'unknown',
        confidence: 0.9,
        has_user_notes: false,
        updated_at: '2026-07-18T00:00:00Z',
      };
      apiFetchMock.mockResolvedValueOnce(
        okJson({ discoveries: [summary], total: 1 } satisfies DiscoveryListResponse),
      );
      apiFetchMock.mockResolvedValueOnce(
        okJson({
          ...discoveryDetailFixture(),
          id: 'agent:resolved-canonical:unrelated-id',
          resource_type: 'agent',
          target_id: 'resolved-canonical',
        }),
      );

      const result = await getDiscovery('agent', 'host-1', 'host-1');

      expect(result?.id).toBe('agent:resolved-canonical:unrelated-id');
      // Fallback find picks the first agent discovery; canonical agent_id drives the detail path
      expect(apiFetchMock).toHaveBeenNthCalledWith(
        2,
        '/api/discovery/agent/resolved-canonical/unrelated-id',
      );
    });

    it('returns null when the agent list has no agent-typed summaries', async () => {
      const nonAgentSummary: DiscoverySummary = {
        id: 'vm:node-1:100',
        resource_type: 'vm',
        resource_id: '100',
        target_id: 'node-1',
        hostname: 'vm-100',
        service_type: 'linux',
        service_name: 'VM',
        service_version: '',
        category: 'unknown',
        confidence: 0.7,
        has_user_notes: false,
        updated_at: '2026-07-18T00:00:00Z',
      };
      apiFetchMock.mockResolvedValueOnce(
        okJson({
          discoveries: [nonAgentSummary],
          total: 1,
        } satisfies DiscoveryListResponse),
      );

      const result = await getDiscovery('agent', 'host-1', 'host-1');

      expect(result).toBeNull();
      expect(apiFetchMock).toHaveBeenCalledTimes(1);
      expect(apiFetchMock).toHaveBeenNthCalledWith(1, '/api/discovery/agent/host-1');
    });

    it('returns null when the resolved agent discovery has no usable agent id', async () => {
      // All three candidate id fields are empty strings, so resolveDiscoveryAgentId returns ''
      const summary: DiscoverySummary = {
        id: 'agent::',
        resource_type: 'agent',
        resource_id: '',
        target_id: '',
        agent_id: '',
        hostname: 'empty.local',
        service_type: 'linux',
        service_name: 'Agent',
        service_version: '',
        category: 'unknown',
        confidence: 0.9,
        has_user_notes: false,
        updated_at: '2026-07-18T00:00:00Z',
      };
      apiFetchMock.mockResolvedValueOnce(
        okJson({ discoveries: [summary], total: 1 } satisfies DiscoveryListResponse),
      );

      const result = await getDiscovery('agent', 'host-1', 'host-1');

      expect(result).toBeNull();
      expect(apiFetchMock).toHaveBeenCalledTimes(1);
    });

    it('uses resource_id as the agent id when agent_id and target_id are absent', async () => {
      const summary: DiscoverySummary = {
        id: 'agent:rid-only:rid-9',
        resource_type: 'agent',
        resource_id: 'rid-9',
        hostname: 'rid-9.local',
        service_type: 'linux',
        service_name: 'Agent',
        service_version: '',
        category: 'unknown',
        confidence: 0.9,
        has_user_notes: false,
        updated_at: '2026-07-18T00:00:00Z',
      };
      apiFetchMock.mockResolvedValueOnce(
        okJson({ discoveries: [summary], total: 1 } satisfies DiscoveryListResponse),
      );
      apiFetchMock.mockResolvedValueOnce(
        okJson({
          ...discoveryDetailFixture(),
          id: 'agent:rid-9:rid-9',
          resource_type: 'agent',
          target_id: 'rid-9',
        }),
      );

      const result = await getDiscovery('agent', 'rid-9', 'rid-9');

      expect(result?.id).toBe('agent:rid-9:rid-9');
      expect(apiFetchMock).toHaveBeenNthCalledWith(
        2,
        '/api/discovery/agent/rid-9/rid-9',
      );
    });
  });
});
