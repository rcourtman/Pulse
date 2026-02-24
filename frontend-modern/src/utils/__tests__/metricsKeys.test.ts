/**
 * Tests for metricsKeys utility functions
 */
import { describe, expect, it } from 'vitest';
import { buildMetricKey, getMetricKeyPrefix, type MetricResourceKind } from '@/utils/metricsKeys';

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
    const kinds: MetricResourceKind[] = [
      'node',
      'vm',
      'container',
      'dockerHost',
      'dockerContainer',
    ];

    it.each(kinds)('returns correct prefix for %s', (kind) => {
      expect(getMetricKeyPrefix(kind)).toBe(`${kind}:`);
    });

    it('prefixes can be used to filter keys', () => {
      const keys = ['node:pve1', 'node:pve2', 'vm:100', 'dockerContainer:abc123'];

      const nodePrefix = getMetricKeyPrefix('node');
      const nodeKeys = keys.filter((k) => k.startsWith(nodePrefix));

      expect(nodeKeys).toEqual(['node:pve1', 'node:pve2']);
    });
  });
});
