import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { toDiscoveryConfig } from '@/components/Infrastructure/ResourceDetailDrawer';

const baseResource = (): Resource => ({
  id: 'host-abcd',
  type: 'host',
  name: 'pve1',
  displayName: 'pve1',
  platformId: 'pve1',
  platformType: 'proxmox-pve',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: Date.now(),
  platformData: {
    sources: ['proxmox', 'agent'],
    proxmox: { nodeName: 'pve1' },
    agent: { agentId: 'host-1', hostname: 'pve1.local' },
  },
  identity: {
    hostname: 'stale-hostname',
  },
});

describe('toDiscoveryConfig', () => {
  it('prefers backend discoveryTarget over heuristic IDs', () => {
    const resource: Resource = {
      ...baseResource(),
      discoveryTarget: {
        resourceType: 'host',
        hostId: 'host-1',
        resourceId: 'host-1',
        hostname: 'pve1.local',
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'host',
      hostId: 'host-1',
      resourceId: 'host-1',
      hostname: 'pve1.local',
      metadataKind: 'host',
      metadataId: 'host-1',
      targetLabel: 'host',
    });
  });

  it('falls back to heuristic mapping when discoveryTarget is absent', () => {
    const config = toDiscoveryConfig(baseResource());
    expect(config).toEqual({
      resourceType: 'host',
      hostId: 'host-1',
      resourceId: 'host-1',
      hostname: 'stale-hostname',
      metadataKind: 'host',
      metadataId: 'host-1',
      targetLabel: 'host',
    });
  });
});
