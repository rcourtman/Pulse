import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getPlatformAgentRecord,
  getPlatformDataRecord,
  getPreferredResourceKubernetesContext,
  hasDockerFacetEvidence,
} from '@/utils/agentResources';

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'resource-1',
    type: 'agent',
    name: 'resource-1',
    displayName: 'resource-1',
    platformId: 'resource-1',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: Date.now(),
    ...overrides,
  }) as Resource;

describe('getPlatformDataRecord', () => {
  it('returns the platformData record by reference when present', () => {
    const platformData = { agent: { agentId: 'agent-1' } };
    const resource = makeResource({ platformData });
    expect(getPlatformDataRecord(resource)).toBe(platformData);
  });

  it('returns the record for an empty platformData object', () => {
    const resource = makeResource({ platformData: {} });
    expect(getPlatformDataRecord(resource)).toEqual({});
  });

  it('returns undefined when platformData is absent', () => {
    expect(getPlatformDataRecord(makeResource())).toBeUndefined();
  });

  it('returns undefined when platformData is null (falsy guard)', () => {
    expect(
      getPlatformDataRecord(
        makeResource({ platformData: null as unknown as Record<string, unknown> }),
      ),
    ).toBeUndefined();
  });
});

describe('getPlatformAgentRecord', () => {
  it('returns the nested agent facet by reference when present', () => {
    const agent = { agentId: 'agent-1', hostname: 'tower' };
    const resource = makeResource({ platformData: { agent } });
    expect(getPlatformAgentRecord(resource)).toBe(agent);
  });

  it('returns an empty record when the agent facet is present but empty', () => {
    const resource = makeResource({ platformData: { agent: {} } });
    expect(getPlatformAgentRecord(resource)).toEqual({});
  });

  it('returns undefined when platformData is absent', () => {
    expect(getPlatformAgentRecord(makeResource())).toBeUndefined();
  });

  it('returns undefined when platformData has no agent key', () => {
    expect(
      getPlatformAgentRecord(makeResource({ platformData: { docker: { hostSourceId: 'd-1' } } })),
    ).toBeUndefined();
  });

  it('returns undefined when the agent facet is null', () => {
    expect(getPlatformAgentRecord(makeResource({ platformData: { agent: null } }))).toBeUndefined();
  });

  it('returns undefined when the agent facet is a non-object scalar', () => {
    expect(
      getPlatformAgentRecord(makeResource({ platformData: { agent: 'agent-1' } })),
    ).toBeUndefined();
    expect(getPlatformAgentRecord(makeResource({ platformData: { agent: 42 } }))).toBeUndefined();
  });
});

describe('getPreferredResourceKubernetesContext', () => {
  it('returns undefined when no kubernetes context coordinates are present', () => {
    expect(getPreferredResourceKubernetesContext({})).toBeUndefined();
  });

  it('prefers kubernetes.clusterName over context and clusterId', () => {
    expect(
      getPreferredResourceKubernetesContext({
        kubernetes: { clusterName: 'by-name', context: 'ctx', clusterId: 'cid' },
      }),
    ).toBe('by-name');
  });

  it('falls back to kubernetes.context when clusterName is absent', () => {
    expect(
      getPreferredResourceKubernetesContext({ kubernetes: { context: 'ctx', clusterId: 'cid' } }),
    ).toBe('ctx');
  });

  it('falls back to kubernetes.clusterId when name and context are absent', () => {
    expect(getPreferredResourceKubernetesContext({ kubernetes: { clusterId: 'cid' } })).toBe('cid');
  });

  it('uses platformData.kubernetes when the top-level kubernetes facet is absent', () => {
    expect(
      getPreferredResourceKubernetesContext({
        platformData: {
          kubernetes: { clusterName: 'pd-name', context: 'pd-ctx', clusterId: 'pd-cid' },
        },
      }),
    ).toBe('pd-name');
  });

  it('prefers platformData.kubernetes.context over its own clusterId', () => {
    expect(
      getPreferredResourceKubernetesContext({
        platformData: { kubernetes: { context: 'pd-ctx', clusterId: 'pd-cid' } },
      }),
    ).toBe('pd-ctx');
  });

  it('prefers the top-level kubernetes.clusterId over platformData.kubernetes.clusterName', () => {
    expect(
      getPreferredResourceKubernetesContext({
        kubernetes: { clusterId: 'top-cid' },
        platformData: { kubernetes: { clusterName: 'pd-name' } },
      }),
    ).toBe('top-cid');
  });

  it('falls back to the standalone clusterId when no kubernetes facet is present', () => {
    expect(getPreferredResourceKubernetesContext({ clusterId: 'standalone-cluster' })).toBe(
      'standalone-cluster',
    );
  });

  it('treats whitespace-only, empty, and null strings as absent and falls through', () => {
    expect(
      getPreferredResourceKubernetesContext({
        kubernetes: { clusterName: '   ', context: '', clusterId: null },
        platformData: { kubernetes: { clusterName: 'pd-name' } },
      }),
    ).toBe('pd-name');
  });

  it('treats non-string values as absent via asTrimmedString', () => {
    expect(
      getPreferredResourceKubernetesContext({
        kubernetes: { clusterName: 123 as unknown as string, context: 'fallback-ctx' },
      }),
    ).toBe('fallback-ctx');
  });
});

describe('hasDockerFacetEvidence', () => {
  it('returns false for non-record inputs', () => {
    expect(hasDockerFacetEvidence(undefined)).toBe(false);
    expect(hasDockerFacetEvidence(null)).toBe(false);
    expect(hasDockerFacetEvidence('docker')).toBe(false);
    expect(hasDockerFacetEvidence(42)).toBe(false);
    expect(hasDockerFacetEvidence(true)).toBe(false);
  });

  it('returns false for an empty object', () => {
    expect(hasDockerFacetEvidence({})).toBe(false);
  });

  it('returns true when any string value is non-empty', () => {
    expect(hasDockerFacetEvidence({ hostname: 'tower' })).toBe(true);
  });

  it('returns false when string values are empty or whitespace-only', () => {
    expect(hasDockerFacetEvidence({ hostname: '' })).toBe(false);
    expect(hasDockerFacetEvidence({ hostname: '   ', runtime: '\t' })).toBe(false);
  });

  it('returns true for a non-zero finite number and false for zero, NaN, and Infinity', () => {
    expect(hasDockerFacetEvidence({ containerCount: 5 })).toBe(true);
    expect(hasDockerFacetEvidence({ containerCount: 0 })).toBe(false);
    expect(hasDockerFacetEvidence({ containerCount: NaN })).toBe(false);
    expect(hasDockerFacetEvidence({ containerCount: Infinity })).toBe(false);
  });

  it('returns true for boolean true and false for boolean false', () => {
    expect(hasDockerFacetEvidence({ healthy: true })).toBe(true);
    expect(hasDockerFacetEvidence({ healthy: false })).toBe(false);
  });

  it('inspects nested objects recursively', () => {
    expect(hasDockerFacetEvidence({ info: { name: 'tower' } })).toBe(true);
    expect(hasDockerFacetEvidence({ info: {} })).toBe(false);
    expect(hasDockerFacetEvidence({ info: { name: '' } })).toBe(false);
  });

  it('inspects arrays recursively', () => {
    expect(hasDockerFacetEvidence({ nodes: ['node-1'] })).toBe(true);
    expect(hasDockerFacetEvidence({ nodes: [] })).toBe(false);
    expect(hasDockerFacetEvidence({ nodes: ['', 0, false] })).toBe(false);
    expect(hasDockerFacetEvidence({ nodes: [{ name: 'node-1' }] })).toBe(true);
  });

  it('returns true when at least one of many values is meaningful', () => {
    expect(hasDockerFacetEvidence({ a: '', b: 0, c: false, d: 'real' })).toBe(true);
  });

  it('treats a top-level array as a record and inspects its elements', () => {
    expect(hasDockerFacetEvidence(['runtime'])).toBe(true);
    expect(hasDockerFacetEvidence([])).toBe(false);
    expect(hasDockerFacetEvidence(['', 0])).toBe(false);
  });
});
