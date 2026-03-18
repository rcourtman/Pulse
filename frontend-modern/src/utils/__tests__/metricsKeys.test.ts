/**
 * Tests for metricsKeys utility functions
 */
import { describe, expect, it } from 'vitest';
import {
  buildMetricKey,
  buildMetricKeyForUnifiedResource,
  getMetricKeyPrefix,
  type MetricResourceKind,
} from '@/utils/metricsKeys';

describe('metricsKeys', () => {
  describe('buildMetricKey', () => {
    it('builds a key for node resources', () => {
      expect(buildMetricKey('node', 'pve1')).toBe('node:pve1');
    });

    it('builds a key for VM resources', () => {
      expect(buildMetricKey('vm', 'vm-100')).toBe('vm:vm-100');
    });

    it('builds a key for container resources', () => {
      expect(buildMetricKey('container', 'ct-200')).toBe('container:ct-200');
    });

    it('builds a key for docker host resources', () => {
      expect(buildMetricKey('dockerHost', 'docker-host-1')).toBe('dockerHost:docker-host-1');
    });

    it('builds a canonical key for agent resources', () => {
      expect(buildMetricKey('agent', 'agent-1')).toBe('agent:agent-1');
    });

    it('builds a key for docker container resources', () => {
      expect(buildMetricKey('dockerContainer', 'abc123def')).toBe('dockerContainer:abc123def');
    });

    it('handles IDs with special characters', () => {
      expect(buildMetricKey('vm', 'pve1/qemu/100')).toBe('vm:pve1/qemu/100');
    });

    it('handles empty IDs', () => {
      expect(buildMetricKey('node', '')).toBe('node:');
    });

    it('handles IDs with colons (should not conflict)', () => {
      expect(buildMetricKey('container', 'host:123')).toBe('container:host:123');
    });
  });

  describe('getMetricKeyPrefix', () => {
    const expectations: Array<{ kind: MetricResourceKind; prefix: string }> = [
      { kind: 'node', prefix: 'node:' },
      { kind: 'vm', prefix: 'vm:' },
      { kind: 'container', prefix: 'container:' },
      { kind: 'dockerHost', prefix: 'dockerHost:' },
      { kind: 'dockerContainer', prefix: 'dockerContainer:' },
      { kind: 'agent', prefix: 'agent:' },
    ];

    it.each(expectations)('returns correct prefix for $kind', ({ kind, prefix }) => {
      expect(getMetricKeyPrefix(kind)).toBe(prefix);
    });

    it('prefixes can be used to filter keys', () => {
      const keys = ['node:pve1', 'node:pve2', 'vm:100', 'dockerContainer:abc123'];

      const nodePrefix = getMetricKeyPrefix('node');
      const nodeKeys = keys.filter((k) => k.startsWith(nodePrefix));

      expect(nodeKeys).toEqual(['node:pve1', 'node:pve2']);
    });
  });

  describe('buildMetricKeyForUnifiedResource', () => {
    it('prefers canonical metricsTarget ids for docker-host resources', () => {
      expect(
        buildMetricKeyForUnifiedResource({
          id: 'hash-resource-id',
          type: 'docker-host',
          metricsTarget: { resourceType: 'docker-host', resourceId: 'docker-host-1' },
        } as any),
      ).toBe('dockerHost:docker-host-1');
    });

    it('maps kubernetes metrics targets onto the k8s namespace', () => {
      expect(
        buildMetricKeyForUnifiedResource({
          id: 'hash-k8s-resource-id',
          type: 'k8s-cluster',
          metricsTarget: { resourceType: 'k8s', resourceId: 'cluster-1' },
        } as any),
      ).toBe('k8s:cluster-1');
    });

    it('falls back to unified resource type when metricsTarget is absent', () => {
      expect(
        buildMetricKeyForUnifiedResource({
          id: 'agent-1',
          type: 'agent',
        } as any),
      ).toBe('agent:agent-1');
    });
  });
});
