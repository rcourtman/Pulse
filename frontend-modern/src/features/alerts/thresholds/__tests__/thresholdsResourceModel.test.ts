import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';

import {
  agentDiskResourceId,
  hostOverrideIdCandidates,
  normalizeStorageStatus,
  uniqueIds,
} from '../thresholdsResourceModel';

describe('thresholdsResourceModel', () => {
  it('deduplicates candidate identifiers while preserving order', () => {
    expect(uniqueIds(' agent-1 ', undefined, 'agent-1', 'agent-2', '')).toEqual([
      'agent-1',
      'agent-2',
    ]);
  });

  it('builds host override candidates from the canonical resource identifiers', () => {
    const resource = {
      id: 'agent-runtime',
      type: 'agent',
      discoveryTarget: {
        resourceType: 'agent',
        resourceId: 'agent-discovery',
        agentId: 'agent-discovery',
      },
      agent: {
        agentId: 'agent-runtime',
      },
      platformData: {
        agent: {
          agentId: 'agent-platform',
        },
        agentId: 'agent-platform',
      },
    } as unknown as Resource;

    expect(hostOverrideIdCandidates(resource)).toEqual([
      'agent-discovery',
      'agent-runtime',
      'agent-platform',
    ]);
  });

  it('sanitizes agent disk ids with the backend-compatible label rules', () => {
    expect(agentDiskResourceId('agent-1', '/var/lib/docker', '')).toBe(
      'agent:agent-1/disk:var-lib-docker',
    );
    expect(agentDiskResourceId('agent-1', '', '/dev/sda1')).toBe('agent:agent-1/disk:dev-sda1');
  });

  it('normalizes storage status to the table availability contract', () => {
    expect(normalizeStorageStatus('online')).toBe('available');
    expect(normalizeStorageStatus('running')).toBe('available');
    expect(normalizeStorageStatus('offline')).toBe('offline');
    expect(normalizeStorageStatus(undefined)).toBe('offline');
  });
});
