import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetch: vi.fn(),
}));

import { getDiscovery, getDiscoveryInfo, listDiscoveriesByType } from '@/api/discovery';
import { apiFetch } from '@/utils/apiClient';

describe('discovery api', () => {
  const apiFetchMock = vi.mocked(apiFetch);

  beforeEach(() => {
    apiFetchMock.mockReset();
  });

  it('returns null for missing host discovery without calling host detail endpoint', async () => {
    apiFetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          discoveries: [],
          total: 0,
        }),
        { status: 200 },
      ),
    );

    const result = await getDiscovery('host', 'host-1', 'host-1');

    expect(result).toBeNull();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(apiFetchMock).toHaveBeenCalledWith('/api/discovery/host/host-1');
  });

  it('resolves host discovery through host list before fetching details', async () => {
    apiFetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          discoveries: [
            {
              id: 'host:host-1:host-1',
              resource_type: 'host',
              resource_id: 'host-1',
              host_id: 'host-1',
              hostname: 'host-1.local',
              service_type: 'linux',
              service_name: 'Host',
              service_version: '',
              category: 'unknown',
              confidence: 0.9,
              has_user_notes: false,
              updated_at: '2026-02-06T00:00:00Z',
            },
          ],
          total: 1,
        }),
        { status: 200 },
      ),
    );
    apiFetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 'host:host-1:host-1',
          resource_type: 'host',
          resource_id: 'host-1',
          host_id: 'host-1',
          hostname: 'host-1.local',
          service_type: 'linux',
          service_name: 'Host',
          service_version: '',
          category: 'unknown',
          cli_access: '',
          facts: [],
          config_paths: [],
          data_paths: [],
          log_paths: [],
          ports: [],
          user_notes: '',
          user_secrets: {},
          confidence: 0.9,
          ai_reasoning: '',
          discovered_at: '2026-02-06T00:00:00Z',
          updated_at: '2026-02-06T00:00:00Z',
          scan_duration: 1,
        }),
        { status: 200 },
      ),
    );

    const result = await getDiscovery('host', 'host-1', 'host-1');

    expect(result?.id).toBe('host:host-1:host-1');
    expect(apiFetchMock).toHaveBeenCalledTimes(2);
    expect(apiFetchMock).toHaveBeenNthCalledWith(1, '/api/discovery/host/host-1');
    expect(apiFetchMock).toHaveBeenNthCalledWith(2, '/api/discovery/host/host-1/host-1');
  });

  it('keeps non-host discovery lookups on the typed resource endpoint', async () => {
    apiFetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 'vm:node-1:100',
          resource_type: 'vm',
          resource_id: '100',
          host_id: 'node-1',
          hostname: 'vm-100',
          service_type: 'linux',
          service_name: 'VM',
          service_version: '',
          category: 'unknown',
          cli_access: '',
          facts: [],
          config_paths: [],
          data_paths: [],
          log_paths: [],
          ports: [],
          user_notes: '',
          user_secrets: {},
          confidence: 0.7,
          ai_reasoning: '',
          discovered_at: '2026-02-06T00:00:00Z',
          updated_at: '2026-02-06T00:00:00Z',
          scan_duration: 1,
        }),
        { status: 200 },
      ),
    );

    const result = await getDiscovery('vm', 'node-1', '100');

    expect(result?.id).toBe('vm:node-1:100');
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
    expect(apiFetchMock).toHaveBeenCalledWith('/api/discovery/vm/node-1/100');
  });

  it('encodes dynamic resource type segments', async () => {
    apiFetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          discoveries: [],
          total: 0,
        }),
        { status: 200 },
      ),
    );

    await listDiscoveriesByType('vm/root' as any);
    expect(apiFetchMock).toHaveBeenCalledWith('/api/discovery/type/vm%2Froot');

    apiFetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          commands: [],
          ai_provider: '',
          notes: '',
        }),
        { status: 200 },
      ),
    );
    await getDiscoveryInfo('host/root' as any);
    expect(apiFetchMock).toHaveBeenCalledWith('/api/discovery/info/host%2Froot');
  });
});
