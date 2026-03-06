import { describe, expect, it } from 'vitest';

import { nodeFromResource } from '../resourceStateAdapters';
import type { Resource } from '@/types/resource';

const createNodeResource = (platformData: Record<string, unknown>): Resource =>
  ({
    id: 'node-1',
    type: 'agent',
    name: 'pve-node-1',
    displayName: 'PVE Node 1',
    platformId: 'pve-node-1',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    cpu: { current: 10 },
    memory: { current: 20, total: 1024, used: 256 },
    disk: { current: 30, total: 2048, used: 512 },
    platformData,
  }) as Resource;

describe('resourceStateAdapters nodeFromResource', () => {
  it('maps canonical linkedAgentId', () => {
    const node = nodeFromResource(
      createNodeResource({
        linkedAgentId: 'agent-canonical',
        proxmox: { nodeName: 'pve-node-1' },
      }),
    );

    expect(node?.linkedAgentId).toBe('agent-canonical');
  });

  it('falls back to the actionable agent identity when linkedAgentId is absent', () => {
    const node = nodeFromResource(
      createNodeResource({
        proxmox: { nodeName: 'pve-node-1' },
        agent: { agentId: 'agent-from-facet' },
      }),
    );

    expect(node?.linkedAgentId).toBe('agent-from-facet');
  });
});
