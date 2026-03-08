import { describe, expect, it } from 'vitest';
import {
  getWorkloadTypeLabel,
  getWorkloadTypePresentation,
  normalizeWorkloadTypePresentationKey,
} from '@/utils/workloadTypePresentation';

describe('workloadTypePresentation', () => {
  describe('normalizeWorkloadTypePresentationKey', () => {
    it('canonicalizes supported workload aliases', () => {
      expect(normalizeWorkloadTypePresentationKey('host')).toBe('agent');
      expect(normalizeWorkloadTypePresentationKey('docker')).toBe('app-container');
      expect(normalizeWorkloadTypePresentationKey('k8s')).toBe('pod');
      expect(normalizeWorkloadTypePresentationKey('kubernetes')).toBe('pod');
    });

    it('does not normalize removed aliases that v6 intentionally dropped', () => {
      expect(normalizeWorkloadTypePresentationKey('qemu')).toBeNull();
      expect(normalizeWorkloadTypePresentationKey('ct')).toBeNull();
      expect(normalizeWorkloadTypePresentationKey('docker-container')).toBeNull();
    });
  });

  describe('getWorkloadTypePresentation', () => {
    it('returns canonical presentation for known workload types', () => {
      expect(getWorkloadTypePresentation('vm')).toMatchObject({
        label: 'VM',
        title: 'Virtual Machine',
      });
      expect(getWorkloadTypePresentation('agent')).toMatchObject({
        label: 'Agent',
        title: 'Agent',
      });
    });

    it('retains canonical alias compatibility through the utility layer', () => {
      expect(getWorkloadTypePresentation('docker').label).toBe('Containers');
      expect(getWorkloadTypePresentation('host').label).toBe('Agent');
      expect(getWorkloadTypePresentation('k8s').label).toBe('Pod');
    });

    it('falls back to titleized labels for unknown workload types', () => {
      expect(getWorkloadTypePresentation('custom-type')).toMatchObject({
        label: 'Custom Type',
        title: 'Custom Type',
      });
      expect(getWorkloadTypePresentation(undefined).label).toBe('Unknown');
    });
  });

  describe('getWorkloadTypeLabel', () => {
    it('returns the presentation label for known and unknown workload types', () => {
      expect(getWorkloadTypeLabel('vm')).toBe('VM');
      expect(getWorkloadTypeLabel('docker')).toBe('Containers');
      expect(getWorkloadTypeLabel('custom-type')).toBe('Custom Type');
    });
  });
});
