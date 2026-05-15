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
  it('declares the TrueNAS section set, omitting Hosts until the canonical projection exists', () => {
    expect(TRUENAS_TAB_SPECS.map((tab) => tab.id)).toEqual(['storage', 'apps']);
  });

  it('buckets storage, apps, and disks while ignoring non-TrueNAS resources', () => {
    const model = buildTrueNASPageModel([
      makeResource({ id: 'truenas-app', type: 'app-container' }),
      makeResource({ id: 'truenas-pool', type: 'pool' }),
      makeResource({ id: 'truenas-disk', type: 'physical_disk' }),
      makeResource({ id: 'docker-host', type: 'agent', platformType: 'docker' }),
      makeResource({ id: 'pve-node', type: 'agent', platformType: 'proxmox-pve' }),
    ]);

    expect(model.apps.map((r) => r.id)).toEqual(['truenas-app']);
    expect(model.resources.map((r) => r.id).sort()).toEqual(
      ['truenas-app', 'truenas-disk', 'truenas-pool'].sort(),
    );
  });
});
