import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';

import { storageOverrideIdCandidates } from '../alertOverridesModel';

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
