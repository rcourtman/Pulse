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
  it('projects agent-primary machine resources without admitting provider-owned host rows', () => {
    const model = buildAgentsPageModel([
      resource({ id: 'mac-mini', platformType: 'agent', type: 'agent', sources: ['agent'] }),
      resource({
        id: 'linux-docker-host',
        platformType: 'agent',
        platformScopes: ['agent', 'docker'],
        type: 'agent',
        sourceType: 'hybrid',
        sources: ['agent', 'docker'],
      }),
      resource({
        id: 'pve-node',
        platformType: 'proxmox-pve',
        type: 'agent',
        sourceType: 'hybrid',
        sources: ['proxmox', 'agent'],
      }),
      resource({
        id: 'esxi-host',
        platformType: 'vmware-vsphere',
        platformScopes: ['agent', 'vmware-vsphere'],
        type: 'agent',
        sourceType: 'api',
        sources: ['vmware'],
        agent: { agentId: 'vc-host-101', platform: 'vmware-vsphere' },
      }),
      resource({ id: 'k8s-pod', platformType: 'agent', type: 'pod', sources: ['agent'] }),
      resource({
        id: 'docker-host',
        platformType: 'docker',
        type: 'docker-host',
        sources: ['agent'],
      }),
    ]);

    expect(model.resources.map((item) => item.id)).toEqual(['mac-mini', 'linux-docker-host']);
  });

  it('keeps legacy source-less agent platform rows visible', () => {
    const model = buildAgentsPageModel([
      resource({ id: 'legacy-agent', platformType: 'agent', type: 'agent', sources: undefined }),
    ]);

    expect(model.resources.map((item) => item.id)).toEqual(['legacy-agent']);
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
