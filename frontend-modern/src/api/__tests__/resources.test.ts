import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { ResourceAPI } from '@/api/resources';
import type { DockerContainer, GuestNetworkInterface, HostNetworkInterface } from '@/types/api';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('ResourceAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('resolves an encoded resource reference with cancellable authenticated lookup', async () => {
    const signal = new AbortController().signal;
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({ id: 'stable-resource' });
    await expect(ResourceAPI.getResource(' cluster-a:node-1:101 ', { signal })).resolves.toEqual({
      id: 'stable-resource',
    });
    expect(apiFetchJSON).toHaveBeenCalledWith('/api/resources/cluster-a%3Anode-1%3A101', {
      cache: 'no-store',
      signal,
    });
  });

  it('fetches compact resource stats without hydrating list pages', async () => {
    const stats = {
      total: 1200,
      byType: { vm: 900, agent: 300 },
      byStatus: { running: 850, online: 300, stopped: 50 },
      bySource: { proxmox: 1200 },
      policyPosture: {
        totalResources: 1200,
        sensitivityCounts: { internal: 1200 },
        routingCounts: { 'cloud-summary': 1200 },
        redactionCounts: {},
      },
    };
    vi.mocked(apiFetchJSON).mockResolvedValueOnce(stats);

    await expect(ResourceAPI.getStats()).resolves.toEqual(stats);
    expect(apiFetchJSON).toHaveBeenCalledWith('/api/resources/stats', {
      cache: 'no-store',
    });
  });

  it('fetches the resource history bundle from the facet endpoint', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      resourceId: 'vm:42',
      capabilities: [{ name: 'restart', type: 'common' }],
      relationships: [
        {
          sourceId: 'vm:42',
          targetId: 'node-1',
          type: 'runs_on',
          confidence: 1,
          active: true,
          discoverer: 'proxmox_adapter',
          observedAt: '2026-03-18T12:00:00Z',
          lastSeenAt: '2026-03-18T12:00:00Z',
        },
      ],
      recentChanges: [{ id: 'change-1' }],
      counts: {
        recentChanges: 3,
        recentChangeKinds: {
          metric_anomaly: 2,
          restart: 1,
        },
        recentChangeSourceTypes: {
          platform_event: 1,
          pulse_diff: 2,
        },
        recentChangeSourceAdapters: {
          docker_adapter: 2,
          proxmox_adapter: 1,
        },
      },
    } as any);

    const result = await ResourceAPI.getFacetBundle('vm:42', {
      since: '2026-03-18T12:00:00Z',
      limit: 25,
      kind: 'restart',
      sourceType: 'platform_event',
      sourceAdapter: 'proxmox_adapter',
    });

    expect(apiFetchJSON).toHaveBeenCalledWith(
      '/api/resources/vm%3A42/facets?since=2026-03-18T12%3A00%3A00.000Z&limit=25&kind=restart&sourceType=platform_event&sourceAdapter=proxmox_adapter',
      {
        cache: 'no-store',
      },
    );
    expect(result).toEqual({
      resourceId: 'vm:42',
      capabilities: [{ name: 'restart', type: 'common' }],
      relationships: [
        {
          sourceId: 'vm:42',
          targetId: 'node-1',
          type: 'runs_on',
          confidence: 1,
          active: true,
          discoverer: 'proxmox_adapter',
          observedAt: '2026-03-18T12:00:00Z',
          lastSeenAt: '2026-03-18T12:00:00Z',
        },
      ],
      recentChanges: [{ id: 'change-1' }],
      counts: {
        recentChanges: 3,
        recentChangeKinds: {
          metric_anomaly: 2,
          restart: 1,
        },
        recentChangeSourceTypes: {
          platform_event: 1,
          pulse_diff: 2,
        },
        recentChangeSourceAdapters: {
          docker_adapter: 2,
          proxmox_adapter: 1,
        },
      },
    });
    expect(result.counts).toStrictEqual({
      recentChanges: 3,
      recentChangeKinds: {
        metric_anomaly: 2,
        restart: 1,
      },
      recentChangeSourceTypes: {
        platform_event: 1,
        pulse_diff: 2,
      },
      recentChangeSourceAdapters: {
        docker_adapter: 2,
        proxmox_adapter: 1,
      },
    });
  });

  it('omits invalid timeline query values instead of emitting broken URLs', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      resourceId: 'vm:42',
      recentChanges: [],
      count: 0,
    } as any);

    await ResourceAPI.getTimeline('vm:42', {
      since: 'not-a-date',
      limit: -1,
      kind: 'metric_anomaly',
      sourceType: 'pulse_diff',
      sourceAdapter: 'docker_adapter',
    });

    expect(apiFetchJSON).toHaveBeenCalledWith(
      '/api/resources/vm%3A42/timeline?kind=metric_anomaly&sourceType=pulse_diff&sourceAdapter=docker_adapter',
      {
        cache: 'no-store',
      },
    );
  });

  it('fetches the global resource timeline with canonical filters', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      resourceId: '',
      recentChanges: [{ id: 'activity-1' }],
      count: 1,
    } as any);

    const result = await ResourceAPI.getGlobalTimeline({
      limit: 100,
      kind: 'activity',
      sourceType: 'platform_event',
      sourceAdapter: 'vmware_adapter',
    });

    expect(apiFetchJSON).toHaveBeenCalledWith(
      '/api/resources/timeline?limit=100&kind=activity&sourceType=platform_event&sourceAdapter=vmware_adapter',
      {
        cache: 'no-store',
      },
    );
    expect(result).toEqual({
      resourceId: '',
      recentChanges: [{ id: 'activity-1' }],
      count: 1,
    });
  });

  it('preserves timeline filters when the time window is valid', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      resourceId: 'vm:42',
      recentChanges: [],
      count: 0,
    } as any);

    await ResourceAPI.getTimeline('vm:42', {
      since: '2026-03-18T12:00:00Z',
      limit: 10,
      kind: 'state_transition',
      sourceType: 'platform_event',
      sourceAdapter: 'proxmox_adapter',
    });

    expect(apiFetchJSON).toHaveBeenCalledWith(
      '/api/resources/vm%3A42/timeline?since=2026-03-18T12%3A00%3A00.000Z&limit=10&kind=state_transition&sourceType=platform_event&sourceAdapter=proxmox_adapter',
      {
        cache: 'no-store',
      },
    );
  });

  it('carries a Proxmox guest device placement through the resource read unchanged', async () => {
    // What the server sends for a VM with one tagged, firewalled device the
    // guest reports and one untagged device it reports nothing for.
    const tagged: HostNetworkInterface = {
      name: 'ens18',
      mac: 'BC:24:11:00:00:01',
      addresses: ['10.30.4.7'],
      bridge: 'vmbr0',
      vlan: 30,
      firewall: true,
    };
    const placedOnly: HostNetworkInterface = {
      name: 'net1',
      mac: 'BC:24:11:00:00:02',
      addresses: [],
      bridge: 'vmbr1',
    };
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      id: 'vm-100',
      type: 'vm',
      proxmox: { vmid: 100, networkInterfaces: [tagged, placedOnly] },
    });

    const resource = await ResourceAPI.getResource('vm-100');
    expect(resource.proxmox?.networkInterfaces).toEqual([tagged, placedOnly]);
    const [first, second] = resource.proxmox?.networkInterfaces ?? [];
    expect([first.bridge, first.vlan, first.firewall]).toEqual(['vmbr0', 30, true]);
    // Untagged and unfiltered are absent fields, not zero and false.
    expect('vlan' in second).toBe(false);
    expect('firewall' in second).toBe(false);

    // The legacy guest row shape lists the same three fields.
    const legacy: GuestNetworkInterface = {
      name: 'eth0',
      bridge: 'vmbr0',
      vlan: 20,
      firewall: false,
    };
    expect(legacy.vlan).toBe(20);
  });

  it('carries a container network mode through the resource read unchanged', async () => {
    const vpnId = 'ab'.repeat(32);
    // What the server sends for a container that uses another container's
    // network: the mode, and no networks or ports of its own.
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      id: 'app-container-1',
      type: 'app-container',
      docker: {
        hostname: 'tower',
        containerId: 'c-torrent',
        containerState: 'running',
        networkMode: `container:${vpnId}`,
      },
    });

    const resource = await ResourceAPI.getResource('app-container-1');
    expect(resource.docker?.networkMode).toBe(`container:${vpnId}`);
    expect(resource.docker?.networks).toBeUndefined();
    expect(resource.docker?.ports).toBeUndefined();

    // The legacy container row lists the same field.
    const legacy: Pick<DockerContainer, 'id' | 'networkMode'> = {
      id: 'c-torrent',
      networkMode: 'host',
    };
    expect(legacy.networkMode).toBe('host');
  });

  it('carries a host default gateway and DNS servers through the resource read unchanged', async () => {
    // What the server sends for a machine whose agent read both from the host.
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      id: 'agent-1',
      type: 'agent',
      agent: {
        hostname: 'tower',
        defaultGateways: [{ address: '192.168.0.1', interface: 'eth0' }],
        dnsServers: ['192.168.0.53', '1.1.1.1'],
      },
    });

    const resource = await ResourceAPI.getResource('agent-1');
    expect(resource.agent?.defaultGateways).toEqual([
      { address: '192.168.0.1', interface: 'eth0' },
    ]);
    expect(resource.agent?.dnsServers).toEqual(['192.168.0.53', '1.1.1.1']);

    // An agent that reported neither: both stay absent, which is unknown.
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      id: 'agent-2',
      type: 'agent',
      agent: { hostname: 'mac-mini' },
    });
    const older = await ResourceAPI.getResource('agent-2');
    expect(older.agent?.defaultGateways).toBeUndefined();
    expect(older.agent?.dnsServers).toBeUndefined();
  });
});
