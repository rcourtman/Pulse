import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getResourceSourceHealth,
  getResourceSourceStatus,
  hasImpairedResourceSource,
} from '@/utils/resourceSourceHealth';

const makeResource = (sourceStatus: Record<string, unknown> | undefined): Resource =>
  ({
    id: 'resource-1',
    type: 'agent',
    name: 'resource-1',
    displayName: 'resource-1',
    platformId: 'lab',
    platformType: 'vmware-vsphere',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    platformData: sourceStatus ? { sourceStatus } : {},
  }) as Resource;

describe('resourceSourceHealth', () => {
  it('matches source-status keys through canonical platform aliases', () => {
    const resource = makeResource({
      vmware: { status: 'stale', lastSeen: '2026-05-20T00:00:00Z' },
    });

    expect(getResourceSourceStatus(resource, 'vmware-vsphere')).toMatchObject({
      status: 'stale',
    });
    expect(getResourceSourceHealth(resource, 'vmware-vsphere')).toBe('impaired');
    expect(hasImpairedResourceSource(resource, 'vmware-vsphere')).toBe(true);
  });

  it('treats explicit connected source statuses as healthy and missing source statuses as unknown', () => {
    expect(
      getResourceSourceHealth(makeResource({ truenas: { status: 'online' } }), 'truenas'),
    ).toBe('connected');
    expect(getResourceSourceHealth(makeResource(undefined), 'truenas')).toBe('unknown');
  });

  it('treats explicit non-connected source statuses as impaired', () => {
    for (const status of ['offline', 'error', 'degraded', 'unknown', 'unauthorized']) {
      expect(getResourceSourceHealth(makeResource({ truenas: { status } }), 'truenas')).toBe(
        'impaired',
      );
    }
  });
});
