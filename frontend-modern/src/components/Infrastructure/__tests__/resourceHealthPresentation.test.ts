import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { getResourceHealthIssuePresentation } from '../resourceHealthPresentation';

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'agent-tower',
    type: 'agent',
    name: 'Tower',
    displayName: 'Tower',
    platformId: 'tower',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'degraded',
    lastSeen: Date.now(),
    ...overrides,
  }) as Resource;

describe('resource health presentation', () => {
  it('surfaces host storage posture as the visible degraded reason', () => {
    const resource = makeResource({
      agent: {
        storagePostureSummary: 'Unraid array is running without parity protection',
        rebuildSummary: 'Unraid array is running check',
        unraid: {
          risk: {
            level: 'warning',
            reasons: [
              {
                code: 'unraid_no_parity',
                severity: 'warning',
                summary: 'Unraid array is running without parity protection',
              },
              {
                code: 'unraid_sync_active',
                severity: 'warning',
                summary: 'Unraid array is running check',
              },
            ],
          },
        },
      },
    });

    expect(getResourceHealthIssuePresentation(resource)).toMatchObject({
      primary: 'Unraid array is running without parity protection',
      compactLabel: 'No parity',
      details: ['Unraid array is running check'],
    });
  });

  it('does not add warning copy to healthy resources', () => {
    const resource = makeResource({
      status: 'online',
      agent: {
        storagePostureSummary: 'Unraid array is running without parity protection',
      },
    });

    expect(getResourceHealthIssuePresentation(resource)).toBeNull();
  });

  it('keeps every incident, not only the rollup, so the drawer lists them all', () => {
    const pool = makeResource({
      type: 'storage',
      status: 'warning',
      incidentSummary: 'Device /dev/sdc has SMART test failures.',
      incidents: [
        {
          code: 'truenas_volume_status',
          severity: 'warning',
          summary: 'Pool archive is DEGRADED: one member of mirror-0 is faulted.',
        },
        {
          code: 'truenas_smart',
          severity: 'warning',
          summary: 'Device /dev/sdc has SMART test failures.',
        },
      ],
      storage: { topology: 'pool', platform: 'truenas' } as Resource['storage'],
    });

    expect(getResourceHealthIssuePresentation(pool)).toMatchObject({
      primary: 'Device /dev/sdc has SMART test failures.',
      details: ['Pool archive is DEGRADED: one member of mirror-0 is faulted.'],
    });
  });

  it('says why a read-only TrueNAS dataset is amber', () => {
    const dataset = makeResource({
      type: 'storage',
      status: 'warning',
      tags: ['truenas', 'dataset', 'zfs', 'state:readonly'],
      storage: { topology: 'dataset', platform: 'truenas' } as Resource['storage'],
    });
    const share = makeResource({
      type: 'network-share',
      status: 'warning',
      tags: ['truenas', 'state:locked'],
    });

    expect(getResourceHealthIssuePresentation(dataset)?.primary).toBe('Dataset is read-only');
    // Only datasets read the dataset state tag; shares carry their own states.
    expect(getResourceHealthIssuePresentation(share)).toBeNull();
  });
});
