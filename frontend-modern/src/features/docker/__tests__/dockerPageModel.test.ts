import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { DOCKER_TAB_SPECS, buildDockerPageModel } from '../dockerPageModel';

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

describe('dockerPageModel', () => {
  it('declares the Docker section set, omitting Swarm services until the canonical projection exists', () => {
    expect(DOCKER_TAB_SPECS.map((tab) => tab.id)).toEqual(['overview', 'containers']);
  });

  it('buckets Docker hosts and containers from canonical resources', () => {
    const model = buildDockerPageModel([
      makeResource({ id: 'docker-host-1', type: 'agent' }),
      makeResource({
        id: 'ctr-1',
        type: 'app-container',
        platformType: 'docker',
      }),
      makeResource({
        id: 'pve-node-1',
        type: 'agent',
        platformType: 'proxmox-pve',
      }),
    ]);

    expect(model.hosts.map((r) => r.id)).toEqual(['docker-host-1']);
    expect(model.containers.map((r) => r.id)).toEqual(['ctr-1']);
    expect(model.resources.map((r) => r.id).sort()).toEqual(
      ['ctr-1', 'docker-host-1'].sort(),
    );
  });

  it('excludes non-Docker hosts that share the agent type', () => {
    const model = buildDockerPageModel([
      makeResource({
        id: 'truenas-host',
        type: 'agent',
        platformType: 'truenas',
      }),
    ]);
    expect(model.hosts).toEqual([]);
    expect(model.resources).toEqual([]);
  });
});
