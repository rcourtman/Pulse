import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { buildAgentsPageFilterModel, buildAgentsPageModel } from '../agentsPageModel';

const resource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'agent-1',
    name: overrides.name ?? overrides.id ?? 'agent-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'agent-1',
    type: overrides.type ?? 'agent',
    platformId: overrides.platformId ?? 'agent-1',
    platformType: overrides.platformType ?? 'agent',
    sourceType: overrides.sourceType ?? 'agent',
    status: overrides.status ?? 'online',
    lastSeen: overrides.lastSeen ?? 1_700_000_000_000,
    ...overrides,
  }) as Resource;

describe('agentsPageModel', () => {
  it('strictly projects only canonical agent platform resources', () => {
    const model = buildAgentsPageModel([
      resource({ id: 'mac-mini', platformType: 'agent', type: 'agent' }),
      resource({ id: 'pve-node', platformType: 'proxmox-pve', type: 'agent' }),
      resource({ id: 'k8s-pod', platformType: 'agent', type: 'pod' }),
      resource({ id: 'docker-host', platformType: 'docker', type: 'docker-host' }),
    ]);

    expect(model.resources.map((item) => item.id)).toEqual(['mac-mini']);
  });

  it('filters agent resources by status and local identity search', () => {
    const resources = [
      resource({
        id: 'agent-mac',
        displayName: 'Studio Mac',
        status: 'online',
        identity: { hostname: 'studio-mac.local', ips: ['10.0.0.12'] },
      }),
      resource({
        id: 'agent-win',
        displayName: 'Windows Bench',
        status: 'offline',
        identity: { hostname: 'bench.local', ips: ['10.0.0.20'] },
      }),
    ];

    expect(buildAgentsPageFilterModel(resources, 'offline', '').filteredResources).toEqual([
      resources[1],
    ]);
    expect(buildAgentsPageFilterModel(resources, '', 'studio 10.0.0.12').filteredResources).toEqual(
      [resources[0]],
    );
  });
});
