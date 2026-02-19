import { describe, expect, it } from 'vitest';

import { buildMentionResources } from '@/components/AI/Chat/mentionResources';
import type { State } from '@/types/api';

describe('mentionResources', () => {
  it('deduplicates host mentions when docker host and host agent represent the same machine', () => {
    const state = {
      nodes: [],
      vms: [],
      containers: [],
      dockerHosts: [
        {
          id: 'host-1',
          agentId: 'agent-1',
          hostname: 'pve01.local',
          displayName: 'pve01',
          cpus: 4,
          totalMemoryBytes: 16,
          uptimeSeconds: 100,
          status: 'online',
          lastSeen: Date.now(),
          intervalSeconds: 15,
          containers: [],
        },
      ],
      hosts: [
        {
          id: 'host-1',
          hostname: 'pve01.local',
          displayName: 'pve01',
          status: 'offline',
          lastSeen: Date.now(),
          memory: { total: 16, used: 8, free: 8, usage: 50 },
        },
      ],
    } as unknown as State;

    const resources = buildMentionResources(state);
    const hosts = resources.filter((resource) => resource.type === 'host');
    expect(hosts).toHaveLength(1);
    expect(hosts[0].status).toBe('online');
  });

  it('deduplicates cluster node mentions from multiple instances and keeps the healthiest status', () => {
    const state = {
      nodes: [
        {
          instance: 'cluster-entry-a',
          name: 'pve01',
          status: 'online',
          clusterName: 'cluster-a',
        },
        {
          instance: 'cluster-entry-b',
          name: 'pve01',
          status: 'offline',
          clusterName: 'cluster-a',
        },
      ],
      vms: [],
      containers: [],
      dockerHosts: [],
      hosts: [],
    } as unknown as State;

    const resources = buildMentionResources(state);
    const nodes = resources.filter((resource) => resource.type === 'node');
    expect(nodes).toHaveLength(1);
    expect(nodes[0].name).toBe('pve01');
    expect(nodes[0].status).toBe('online');
  });

  it('deduplicates node and host via linkedHostAgentId', () => {
    const state = {
      nodes: [
        {
          id: 'node/pve01',
          instance: 'inst-1',
          name: 'pve01',
          status: 'online',
          linkedHostAgentId: 'abc-123',
        },
      ],
      vms: [],
      containers: [],
      dockerHosts: [],
      hosts: [
        {
          id: 'abc-123',
          hostname: 'pve01.local',
          displayName: 'pve01',
          status: 'online',
          linkedNodeId: 'node/pve01',
          lastSeen: Date.now(),
          memory: { total: 16, used: 8, free: 8, usage: 50 },
        },
      ],
    } as unknown as State;

    const resources = buildMentionResources(state);
    const nodeAndHostEntries = resources.filter(
      (r) => r.type === 'node' || r.type === 'host',
    );
    expect(nodeAndHostEntries).toHaveLength(1);
    expect(nodeAndHostEntries[0].type).toBe('node');
    expect(nodeAndHostEntries[0].name).toBe('pve01');
  });

  it('deduplicates node and host via linkedNodeId', () => {
    const state = {
      nodes: [
        {
          id: 'node/pve02',
          instance: 'inst-2',
          name: 'pve02',
          status: 'online',
        },
      ],
      vms: [],
      containers: [],
      dockerHosts: [],
      hosts: [
        {
          id: 'def-456',
          hostname: 'pve02.local',
          displayName: 'pve02',
          status: 'online',
          linkedNodeId: 'node/pve02',
          lastSeen: Date.now(),
          memory: { total: 16, used: 8, free: 8, usage: 50 },
        },
      ],
    } as unknown as State;

    const resources = buildMentionResources(state);
    const nodeAndHostEntries = resources.filter(
      (r) => r.type === 'node' || r.type === 'host',
    );
    expect(nodeAndHostEntries).toHaveLength(1);
    expect(nodeAndHostEntries[0].type).toBe('node');
  });

  it('does not merge unrelated nodes and hosts', () => {
    const state = {
      nodes: [
        {
          id: 'node/pve01',
          instance: 'inst-1',
          name: 'pve01',
          status: 'online',
        },
      ],
      vms: [],
      containers: [],
      dockerHosts: [],
      hosts: [
        {
          id: 'xyz-999',
          hostname: 'standalone.local',
          displayName: 'standalone',
          status: 'online',
          lastSeen: Date.now(),
          memory: { total: 16, used: 8, free: 8, usage: 50 },
        },
      ],
    } as unknown as State;

    const resources = buildMentionResources(state);
    const nodeAndHostEntries = resources.filter(
      (r) => r.type === 'node' || r.type === 'host',
    );
    expect(nodeAndHostEntries).toHaveLength(2);
    expect(nodeAndHostEntries.find((r) => r.type === 'node')).toBeDefined();
    expect(nodeAndHostEntries.find((r) => r.type === 'host')).toBeDefined();
  });
});
