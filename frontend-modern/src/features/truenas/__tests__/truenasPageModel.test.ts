import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { TRUENAS_TAB_SPECS, buildTrueNASPageModel } from '../truenasPageModel';

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'truenas',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

describe('truenasPageModel', () => {
  it('declares the TrueNAS section set as Overview + Storage', () => {
    expect(TRUENAS_TAB_SPECS.map((tab) => tab.id)).toEqual(['overview', 'storage']);
  });

  it('buckets systems and apps while keeping storage inventory in scope for shared surfaces', () => {
    const model = buildTrueNASPageModel([
      makeResource({ id: 'truenas-system', type: 'agent' }),
      makeResource({ id: 'truenas-app', type: 'app-container' }),
      makeResource({ id: 'truenas-pool', type: 'pool' }),
      makeResource({ id: 'truenas-disk', type: 'physical_disk' }),
      makeResource({ id: 'docker-host', type: 'agent', platformType: 'docker' }),
      makeResource({ id: 'pve-node', type: 'agent', platformType: 'proxmox-pve' }),
    ]);

    expect(model.systems.map((r) => r.id)).toEqual(['truenas-system']);
    expect(model.apps.map((r) => r.id)).toEqual(['truenas-app']);
    expect(model.resources.map((r) => r.id).sort()).toEqual(
      ['truenas-app', 'truenas-disk', 'truenas-pool', 'truenas-system'].sort(),
    );
  });
});
