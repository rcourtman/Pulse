import { describe, expect, it } from 'vitest';

import type { ResourceVMwareMeta } from '@/types/resource';
import { formatVmwareClusterServices } from '@/utils/vmwareDisplay';

describe('vmwareDisplay', () => {
  describe('formatVmwareClusterServices', () => {
    it('returns empty string when meta is undefined', () => {
      expect(formatVmwareClusterServices(undefined)).toBe('');
    });

    it('returns empty string when neither flag is set', () => {
      expect(formatVmwareClusterServices({})).toBe('');
    });

    it.each([
      ['HA enabled', { clusterHaEnabled: true }, 'HA enabled'],
      ['HA disabled', { clusterHaEnabled: false }, 'HA disabled'],
      ['DRS enabled', { clusterDrsEnabled: true }, 'DRS enabled'],
      ['DRS disabled', { clusterDrsEnabled: false }, 'DRS disabled'],
    ])('renders a single flag for %s', (_label, meta, expected) => {
      expect(formatVmwareClusterServices(meta as ResourceVMwareMeta)).toBe(expected);
    });

    it.each([
      ['both enabled', { clusterHaEnabled: true, clusterDrsEnabled: true }, 'HA enabled · DRS enabled'],
      ['both disabled', { clusterHaEnabled: false, clusterDrsEnabled: false }, 'HA disabled · DRS disabled'],
      ['HA enabled DRS disabled', { clusterHaEnabled: true, clusterDrsEnabled: false }, 'HA enabled · DRS disabled'],
      ['HA disabled DRS enabled', { clusterHaEnabled: false, clusterDrsEnabled: true }, 'HA disabled · DRS enabled'],
    ])('joins both flags with " · " for %s', (_label, meta, expected) => {
      expect(formatVmwareClusterServices(meta as ResourceVMwareMeta)).toBe(expected);
    });

    it('ignores unrelated fields on the meta object', () => {
      expect(
        formatVmwareClusterServices({
          clusterName: 'vc-1',
          clusterHaEnabled: true,
        } as ResourceVMwareMeta),
      ).toBe('HA enabled');
    });
  });
});
