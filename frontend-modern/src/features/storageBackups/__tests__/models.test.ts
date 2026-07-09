import { describe, expect, it } from 'vitest';

import { KNOWN_STORAGE_BACKUP_PLATFORMS } from '@/features/storageBackups/models';
import { KNOWN_SOURCE_PLATFORM_KEYS } from '@/utils/sourcePlatforms';

describe('storageBackups/models', () => {
  describe('KNOWN_STORAGE_BACKUP_PLATFORMS', () => {
    it('re-exports the canonical source platform key list by reference', () => {
      expect(KNOWN_STORAGE_BACKUP_PLATFORMS).toBe(KNOWN_SOURCE_PLATFORM_KEYS);
    });

    it('lists every governed platform plus the availability provider', () => {
      expect([...KNOWN_STORAGE_BACKUP_PLATFORMS]).toEqual([
        'agent',
        'docker',
        'kubernetes',
        'proxmox-pve',
        'proxmox-pbs',
        'proxmox-pmg',
        'truenas',
        'vmware-vsphere',
        'unraid',
        'synology-dsm',
        'microsoft-hyperv',
        'aws',
        'azure',
        'gcp',
        'generic',
        'availability',
      ]);
    });

    it('keeps availability as the final provider-synthesized key', () => {
      expect(KNOWN_STORAGE_BACKUP_PLATFORMS.at(-1)).toBe('availability');
    });

    it('contains only non-empty lowercase string keys with no duplicates', () => {
      const keys = [...KNOWN_STORAGE_BACKUP_PLATFORMS];
      expect(
        keys.every((key) => typeof key === 'string' && key.length > 0 && key === key.toLowerCase()),
      ).toBe(true);
      expect(new Set(keys).size).toBe(keys.length);
    });

    it.each([
      'agent',
      'docker',
      'kubernetes',
      'proxmox-pve',
      'proxmox-pbs',
      'proxmox-pmg',
      'truenas',
      'vmware-vsphere',
      'generic',
      'availability',
    ])('includes the storage-relevant platform %s', (platform) => {
      expect(KNOWN_STORAGE_BACKUP_PLATFORMS).toContain(platform);
    });
  });
});
