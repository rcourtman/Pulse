import { describe, expect, it } from 'vitest';
import { getSourcePlatformBadge, getSourcePlatformLabel } from '@/components/shared/sourcePlatformBadges';

describe('sourcePlatformBadges', () => {
  describe('getSourcePlatformBadge', () => {
    it('returns null for undefined', () => {
      expect(getSourcePlatformBadge(undefined)).toBeNull();
    });

    it('returns null for null', () => {
      expect(getSourcePlatformBadge(null)).toBeNull();
    });

    it('returns null for empty string', () => {
      expect(getSourcePlatformBadge('')).toBeNull();
    });

    it('returns PVE badge for proxmox-pve', () => {
      const result = getSourcePlatformBadge('proxmox-pve');
      expect(result?.label).toBe('PVE');
      expect(result?.classes).toContain('orange');
    });

    it('returns PBS badge for proxmox-pbs', () => {
      const result = getSourcePlatformBadge('proxmox-pbs');
      expect(result?.label).toBe('PBS');
    });

    it('returns PMG badge for proxmox-pmg', () => {
      const result = getSourcePlatformBadge('proxmox-pmg');
      expect(result?.label).toBe('PMG');
    });

    it('returns Containers badge for docker', () => {
      const result = getSourcePlatformBadge('docker');
      expect(result?.label).toBe('Containers');
    });

    it('returns K8s badge for kubernetes', () => {
      const result = getSourcePlatformBadge('kubernetes');
      expect(result?.label).toBe('K8s');
    });

    it('returns TrueNAS badge for truenas', () => {
      const result = getSourcePlatformBadge('truenas');
      expect(result?.label).toBe('TrueNAS');
    });

    it('returns Host badge for host-agent', () => {
      const result = getSourcePlatformBadge('host-agent');
      expect(result?.label).toBe('Host');
    });

    it('returns Unraid badge for unraid', () => {
      const result = getSourcePlatformBadge('unraid');
      expect(result?.label).toBe('Unraid');
    });

    it('returns Synology badge for synology-dsm', () => {
      const result = getSourcePlatformBadge('synology-dsm');
      expect(result?.label).toBe('Synology');
    });

    it('returns vSphere badge for vmware-vsphere', () => {
      const result = getSourcePlatformBadge('vmware-vsphere');
      expect(result?.label).toBe('vSphere');
    });

    it('returns Hyper-V badge for microsoft-hyperv', () => {
      const result = getSourcePlatformBadge('microsoft-hyperv');
      expect(result?.label).toBe('Hyper-V');
    });

    it('returns AWS badge for aws', () => {
      const result = getSourcePlatformBadge('aws');
      expect(result?.label).toBe('AWS');
    });

    it('returns Azure badge for azure', () => {
      const result = getSourcePlatformBadge('azure');
      expect(result?.label).toBe('Azure');
    });

    it('returns GCP badge for gcp', () => {
      const result = getSourcePlatformBadge('gcp');
      expect(result?.label).toBe('GCP');
    });

    it('returns Generic badge for generic', () => {
      const result = getSourcePlatformBadge('generic');
      expect(result?.label).toBe('Generic');
    });

    it('is case-insensitive', () => {
      expect(getSourcePlatformBadge('DOCKER')?.label).toBe('Containers');
      expect(getSourcePlatformBadge('Kubernetes')?.label).toBe('K8s');
      expect(getSourcePlatformBadge('Docker')?.label).toBe('Containers');
    });

    it('returns unknown platform badge for unrecognized platforms', () => {
      const result = getSourcePlatformBadge('custom-platform');
      expect(result?.label).toBe('Custom Platform');
      expect(result?.classes).toContain('surface');
    });

    it('handles whitespace around platform names', () => {
      const result = getSourcePlatformBadge('  docker  ');
      expect(result?.label).toBe('Containers');
    });

    it('handles platform names with underscores', () => {
      const result = getSourcePlatformBadge('proxmox_pve');
      // Underscores are titleized for unknown platforms
      expect(result?.label).toBe('Proxmox Pve');
    });
  });

  describe('getSourcePlatformLabel', () => {
    it('returns label for known platforms', () => {
      expect(getSourcePlatformLabel('docker')).toBe('Containers');
      expect(getSourcePlatformLabel('kubernetes')).toBe('K8s');
    });

    it('returns titleized label for unknown platforms', () => {
      expect(getSourcePlatformLabel('custom-platform')).toBe('Custom Platform');
    });

    it('returns Unknown for empty string', () => {
      expect(getSourcePlatformLabel('')).toBe('Unknown');
    });

    it('returns Unknown for whitespace only', () => {
      expect(getSourcePlatformLabel('   ')).toBe('Unknown');
    });

    it('returns Unknown for undefined', () => {
      expect(getSourcePlatformLabel(undefined)).toBe('Unknown');
    });
  });
});
