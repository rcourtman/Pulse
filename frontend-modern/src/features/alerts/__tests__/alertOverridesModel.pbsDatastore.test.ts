import { describe, expect, it } from 'vitest';

import type { PBSInstance } from '@/types/api';
import type { RawOverrideConfig } from '@/types/alerts';
import type { Resource } from '@/types/resource';

import { buildProjectedOverrides, storageOverrideIdCandidates } from '../alertOverridesModel';

const pbsDatastore = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'storage-hash',
    name: 'main',
    type: 'storage',
    storage: { platform: 'pbs', type: 'pbs-datastore' },
    metricsTarget: { resourceType: 'storage', resourceId: 'pbs-pbs-docker/main' },
    ...overrides,
  }) as Resource;

describe('storageOverrideIdCandidates for PBS datastores (#1591)', () => {
  it('includes the legacy dash-format key after the canonical candidates', () => {
    expect(storageOverrideIdCandidates(pbsDatastore())).toEqual([
      'pbs-pbs-docker/main',
      'storage-hash',
      'pbs-pbs-docker-main',
    ]);
  });

  it('does not fabricate a legacy key for non-PBS storage', () => {
    expect(
      storageOverrideIdCandidates(
        pbsDatastore({ storage: { platform: 'truenas' } as Resource['storage'] }),
      ),
    ).toEqual(['pbs-pbs-docker/main', 'storage-hash']);
  });

  it('does not fabricate a legacy key without a canonical metrics target', () => {
    expect(storageOverrideIdCandidates(pbsDatastore({ metricsTarget: undefined }))).toEqual([
      'storage-hash',
    ]);
  });
});

describe('buildProjectedOverrides for PBS datastores (#1591)', () => {
  const project = (rawConfig: Record<string, RawOverrideConfig>) =>
    buildProjectedOverrides({
      rawConfig,
      nodeResources: [],
      vmResources: [],
      containerResources: [],
      storageResources: [pbsDatastore()],
      agentResourceList: [],
      containerRuntimeResources: [],
      getChildren: () => [],
      pbsInstanceById: new Map<string, PBSInstance>(),
    });

  it('projects a datastore override saved under the canonical pbs-prefixed key', () => {
    const overrides = project({
      'pbs-pbs-docker/main': { disabled: true } as RawOverrideConfig,
    });
    expect(overrides).toHaveLength(1);
    expect(overrides[0]).toMatchObject({
      id: 'pbs-pbs-docker/main',
      type: 'storage',
      disabled: true,
    });
  });

  it('projects a datastore override saved under the legacy dash key', () => {
    const overrides = project({
      'pbs-pbs-docker-main': { disabled: true } as RawOverrideConfig,
    });
    expect(overrides).toHaveLength(1);
    expect(overrides[0]).toMatchObject({
      id: 'pbs-pbs-docker/main',
      type: 'storage',
      disabled: true,
    });
  });

  it('still drops pbs-prefixed keys that match neither an instance nor storage', () => {
    expect(project({ 'pbs-gone-instance': { disabled: true } as RawOverrideConfig })).toHaveLength(
      0,
    );
  });
});
