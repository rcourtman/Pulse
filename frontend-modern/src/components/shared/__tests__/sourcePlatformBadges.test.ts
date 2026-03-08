import { describe, expect, it } from 'vitest';
import * as sourcePlatformBadges from '@/components/shared/sourcePlatformBadges';
import { getSourcePlatformBadge } from '@/components/shared/sourcePlatformBadges';

describe('sourcePlatformBadges', () => {
  it('keeps the badge module rendering-only', () => {
    expect(sourcePlatformBadges).toHaveProperty('getSourcePlatformBadge');
    expect(sourcePlatformBadges).not.toHaveProperty('getSourcePlatformLabel');
    expect(sourcePlatformBadges).not.toHaveProperty('normalizeSourcePlatformKey');
  });

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

    it('normalizes short and generic proxmox aliases to canonical source badges', () => {
      expect(getSourcePlatformBadge('pve')?.label).toBe('PVE');
      expect(getSourcePlatformBadge('proxmox')?.label).toBe('PVE');
      expect(getSourcePlatformBadge('pbs')?.label).toBe('PBS');
      expect(getSourcePlatformBadge('pmg')?.label).toBe('PMG');
      expect(getSourcePlatformBadge('k8s')?.label).toBe('K8s');
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

    it('returns Agent badge for agent', () => {
      const result = getSourcePlatformBadge('agent');
      expect(result?.label).toBe('Agent');
      expect(result?.classes).toContain('emerald');
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
});
