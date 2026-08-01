import { createRoot } from 'solid-js';
import { describe, expect, it } from 'vitest';

import type { ThresholdsTableProps } from '../../types';
import { useThresholdsGuestData } from '../useThresholdsGuestData';

describe('useThresholdsGuestData guest filesystems', () => {
  it('projects QEMU guest-agent filesystems with stable override identities', () => {
    createRoot((dispose) => {
      const props = {
        allGuests: () => [
          {
            id: 'cluster-a:node-1:100',
            name: 'flatcar',
            platformId: 'cluster-a',
            status: 'online',
            type: 'vm',
            lastSeen: 1,
            proxmox: {
              instance: 'cluster-a',
              node: 'node-1',
              vmid: 100,
              disks: [
                {
                  device: '/dev/vda1',
                  free: 3,
                  mountpoint: '/boot',
                  total: 100,
                  type: 'ext4',
                  usage: 97,
                  used: 97,
                },
              ],
            },
          },
        ],
        backupDefaults: () => ({ enabled: false }),
        guestDefaults: { disk: 85 },
        nodes: [],
        overrides: () => [
          {
            id: 'guest-disk:guest:cluster-a:100/disk:boot-dev-vda1',
            name: '/boot',
            type: 'guestDisk',
            disabled: true,
            thresholds: {},
          },
        ],
        snapshotDefaults: () => ({ enabled: false }),
      } as unknown as ThresholdsTableProps;

      const data = useThresholdsGuestData({
        props,
        editingId: () => null,
        searchTerm: () => '',
      });

      expect(data.guestDisksWithOverrides()).toEqual([
        expect.objectContaining({
          id: 'cluster-a:node-1:100-disk-boot-dev-vda1',
          overrideStorageId: 'guest-disk:guest:cluster-a:100/disk:boot-dev-vda1',
          name: '/boot',
          node: 'flatcar',
          type: 'guestDisk',
          disabled: true,
          hasOverride: true,
          defaults: { disk: 85 },
        }),
      ]);
      expect(Object.keys(data.guestDisksGroupedByGuest())).toEqual(['flatcar · cluster-a/100']);
      dispose();
    });
  });

  it('projects filesystems whose serialized usage was omitted without crashing', () => {
    createRoot((dispose) => {
      const props = {
        allGuests: () => [
          {
            id: 'cluster-a:node-1:101',
            name: 'files',
            platformId: 'cluster-a',
            status: 'online',
            type: 'ct',
            lastSeen: 1,
            proxmox: {
              instance: 'cluster-a',
              node: 'node-1',
              vmid: 101,
              // The unified resources payload drops zero-valued numerics, so
              // an untouched filesystem arrives with no usage/used fields.
              disks: [
                {
                  device: 'rootfs',
                  mountpoint: '/',
                  total: 107374182400,
                  type: 'ext4',
                },
                {
                  device: 'mp0',
                  mountpoint: '/data',
                  total: 214748364800,
                  used: 107374182400,
                  free: 107374182400,
                  type: 'ext4',
                },
              ],
            },
          },
        ],
        backupDefaults: () => ({ enabled: false }),
        guestDefaults: { disk: 85 },
        nodes: [],
        overrides: () => [],
        snapshotDefaults: () => ({ enabled: false }),
      } as unknown as ThresholdsTableProps;

      const data = useThresholdsGuestData({
        props,
        editingId: () => null,
        searchTerm: () => '',
      });

      expect(data.guestDisksWithOverrides()).toEqual([
        expect.objectContaining({
          name: '/',
          subtitle: '0.0 / 100.0 GB · 0.0%',
        }),
        expect.objectContaining({
          name: '/data',
          subtitle: '100.0 / 200.0 GB · 50.0%',
        }),
      ]);
      dispose();
    });
  });

  it('keeps a persisted filesystem override visible after its guest disappears', () => {
    createRoot((dispose) => {
      const props = {
        allGuests: () => [],
        backupDefaults: () => ({ enabled: false }),
        guestDefaults: { disk: 85 },
        nodes: [],
        overrides: () => [
          {
            id: 'guest-disk:guest:cluster-a:100/disk:boot',
            name: '/boot',
            node: 'flatcar',
            type: 'guestDisk',
            thresholds: { disk: 99 },
          },
        ],
        snapshotDefaults: () => ({ enabled: false }),
      } as unknown as ThresholdsTableProps;

      const data = useThresholdsGuestData({
        props,
        editingId: () => null,
        searchTerm: () => '',
      });

      expect(data.guestDisksWithOverrides()).toEqual([
        expect.objectContaining({
          id: 'guest-disk:guest:cluster-a:100/disk:boot',
          name: '/boot',
          node: 'flatcar',
          status: 'unknown',
          thresholds: { disk: 99 },
        }),
      ]);
      dispose();
    });
  });
});
