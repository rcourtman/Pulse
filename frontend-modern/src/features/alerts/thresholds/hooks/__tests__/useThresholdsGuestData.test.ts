import { createRoot, createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { guestDiskDeferrals } from '@/components/Workloads/__fixtures__/guestDiskDeferrals';
import type { ThresholdsTableProps } from '../../types';
import { useThresholdsGuestData } from '../useThresholdsGuestData';

const filesystemGuest = (reason = '', power = 'running'): Resource => ({
  id: 'cluster-a:node-1:100',
  name: 'files',
  displayName: 'files',
  type: 'vm',
  status: 'online',
  platformId: 'cluster-a',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  lastSeen: 1,
  proxmox: {
    instance: 'cluster-a',
    node: 'node-1',
    vmid: 100,
    diskStatusReason: reason,
    runtimeStatus: power,
    disks: [
      {
        device: '/dev/vda1',
        mountpoint: '/data',
        type: 'ext4',
        total: 10 * 1024 ** 3,
        used: 9 * 1024 ** 3,
        usage: 90,
      },
    ],
  },
});

function filesystemData(initial: Resource) {
  const [guest, setGuest] = createSignal(initial);
  const props = {
    allGuests: () => [guest()],
    overrides: () => [
      {
        id: 'guest-disk:guest:cluster-a:100/disk:data-dev-vda1',
        name: '/data',
        node: 'files',
        type: 'guestDisk',
        thresholds: { disk: 95 },
      },
    ],
    nodes: [],
    guestDefaults: { disk: 85 },
    backupDefaults: () => ({ enabled: false }),
    snapshotDefaults: () => ({ enabled: false }),
  } as unknown as ThresholdsTableProps;
  return {
    setGuest,
    data: useThresholdsGuestData({ props, editingId: () => null, searchTerm: () => '' }),
  };
}

describe('useThresholdsGuestData guest filesystems', () => {
  it.each(guestDiskDeferrals)(
    'distinguishes retained and unavailable %s readings without losing alert settings',
    (reason, message) => {
      createRoot((dispose) => {
        const { data, setGuest } = filesystemData(filesystemGuest(`prev-${reason}`));
        expect(data.guestDisksWithOverrides()).toEqual([
          expect.objectContaining({
            id: 'cluster-a:node-1:100-disk-data-dev-vda1',
            overrideStorageId: 'guest-disk:guest:cluster-a:100/disk:data-dev-vda1',
            subtitle: 'Last known 9.0 / 10.0 GB · 90.0%',
            readingMessage: `Using last known disk stats. ${message}`,
            thresholds: { disk: 95 },
            hasOverride: true,
          }),
        ]);
        setGuest(filesystemGuest(reason));
        expect(data.guestDisksWithOverrides()[0]).toMatchObject({
          subtitle: 'Usage unavailable',
          readingMessage: message,
          thresholds: { disk: 95 },
        });
        setGuest(filesystemGuest());
        expect(data.guestDisksWithOverrides()[0]).toMatchObject({
          subtitle: '9.0 / 10.0 GB · 90.0%',
          readingMessage: undefined,
          thresholds: { disk: 95 },
        });
        dispose();
      });
    },
  );

  it('uses native power state ahead of an online carrier and keeps restart deferral until a fresh zero', () => {
    createRoot((dispose) => {
      const { data, setGuest } = filesystemData(filesystemGuest('', 'stopped'));
      expect(data.guestDisksWithOverrides()[0]).toMatchObject({
        subtitle: 'Usage unavailable',
        readingMessage: 'Guest filesystem stats unavailable while the VM is stopped.',
      });
      setGuest(filesystemGuest('prev-agent-busy'));
      expect(data.guestDisksWithOverrides()[0].subtitle).toContain('Last known');
      const fresh = filesystemGuest();
      fresh.proxmox!.disks![0].used = 0;
      fresh.proxmox!.disks![0].usage = 0;
      setGuest(fresh);
      expect(data.guestDisksWithOverrides()[0]).toMatchObject({
        subtitle: '0.0 / 10.0 GB · 0.0%',
        readingMessage: undefined,
      });
      dispose();
    });
  });

  it('does not make LXC platform storage or native QGA filesystems stale because of another producer', () => {
    createRoot((dispose) => {
      const initial = filesystemGuest('prev-agent-busy', 'stopped');
      initial.type = 'system-container';
      const { data, setGuest } = filesystemData(initial);
      expect(data.guestDisksWithOverrides()[0]).toMatchObject({
        subtitle: '9.0 / 10.0 GB · 90.0%',
        readingMessage: undefined,
      });
      const native = filesystemGuest();
      native.agent = { stale: true };
      setGuest(native);
      expect(data.guestDisksWithOverrides()[0]).toMatchObject({
        subtitle: '9.0 / 10.0 GB · 90.0%',
        readingMessage: undefined,
      });
      dispose();
    });
  });

  it('explains a missing reading while keeping its saved override and identity editable', () => {
    createRoot((dispose) => {
      const initial = filesystemGuest('vm-locked');
      initial.proxmox!.disks = [];
      const { data } = filesystemData(initial);
      expect(data.guestDisksWithOverrides()[0]).toMatchObject({
        id: 'guest-disk:guest:cluster-a:100/disk:data-dev-vda1',
        overrideStorageId: 'guest-disk:guest:cluster-a:100/disk:data-dev-vda1',
        subtitle: 'Usage unavailable',
        readingMessage: 'No current filesystem reading. Saved alert settings are retained.',
        thresholds: { disk: 95 },
      });
      dispose();
    });
  });

  it('uses bounded read guidance rather than leaking an unknown provider reason', () => {
    createRoot((dispose) => {
      const { data } = filesystemData(filesystemGuest('private provider text'));
      expect(data.guestDisksWithOverrides()[0].subtitle).toBe('Usage unavailable');
      expect(data.guestDisksWithOverrides()[0].readingMessage).toContain('The cause is unknown');
      expect(data.guestDisksWithOverrides()[0].readingMessage).not.toContain(
        'private provider text',
      );
      dispose();
    });
  });

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
