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
});
