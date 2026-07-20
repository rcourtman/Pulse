import { describe, expect, it } from 'vitest';
import type { Storage } from '@/types/api';
import {
  getStorageSourceOption,
  resolveStorageSourceKey,
  type StorageSourceOption,
} from '@/utils/storageSources';

// Mirrors the factory pattern used by the sibling storageSources.test.ts suite
// so this branch-coverage file stays consistent with existing conventions.
const makeStorage = (overrides: Partial<Pick<Storage, 'platform' | 'type'>>): Storage =>
  ({
    id: 'storage-1',
    name: 'storage-1',
    node: 'node-1',
    instance: 'instance-1',
    type: 'dir',
    status: 'active',
    total: 100,
    used: 10,
    free: 90,
    usage: 10,
    content: 'images',
    shared: false,
    enabled: true,
    active: true,
    ...overrides,
  }) as Storage;

describe('storageSources.branchcov2', () => {
  describe('resolveStorageSourceKey — platform-wins block', () => {
    it.each([
      ['truenas', 'zfs'],
      ['vmware-vsphere', 'vsan'],
      ['proxmox-pbs', 'lvm'],
      ['proxmox-pmg', 'dir'],
      ['kubernetes', 'csi'],
      ['microsoft-hyperv', 'smb'],
    ])(
      'returns the canonical platform %s even when on-disk type %s would resolve elsewhere',
      (platform, type) => {
        expect(resolveStorageSourceKey(makeStorage({ platform, type }))).toBe(platform);
      },
    );

    it.each([
      ['vmware', 'vmware-vsphere', 'vmfs'],
      ['hyper-v', 'microsoft-hyperv', 'smb'],
      ['pbs', 'proxmox-pbs', 'zfs'],
      ['pmg', 'proxmox-pmg', 'dir'],
      ['k8s', 'kubernetes', 'csi'],
    ])(
      'normalizes the raw platform alias %s into the wins-list key %s before resolving (type=%s ignored)',
      (rawPlatform, expected, type) => {
        expect(resolveStorageSourceKey(makeStorage({ platform: rawPlatform, type }))).toBe(
          expected,
        );
      },
    );

    it('prefers a winning platform tag over a type that would itself classify as kubernetes', () => {
      // platform=kubernetes wins before the type-based kubernetes classifier runs.
      expect(
        resolveStorageSourceKey(makeStorage({ platform: 'kubernetes', type: 'csi-driver' })),
      ).toBe('kubernetes');
    });
  });

  describe('resolveStorageSourceKey — platform fallthrough', () => {
    it.each([
      ['proxmox-pve', 'ceph'],
      ['agent', 'dir'],
      ['docker', 'dir'],
      ['ceph', 'dir'],
    ])('ignores non-winning platform tag %s and resolves by on-disk type %s', (platform, type) => {
      // 'dir' -> proxmox-pve; 'ceph' -> ceph
      const expected = type === 'ceph' ? 'ceph' : 'proxmox-pve';
      expect(resolveStorageSourceKey(makeStorage({ platform, type }))).toBe(expected);
    });

    it('coerces an undefined platform to empty string and still resolves by type', () => {
      const storage = { ...makeStorage({}), platform: undefined } as Storage;
      delete storage.platform;
      // type 'dir' -> proxmox-pve
      expect(resolveStorageSourceKey(storage)).toBe('proxmox-pve');
    });
  });

  describe('resolveStorageSourceKey — type-based canonical recognition', () => {
    it.each([['proxmox-pbs'], ['ceph'], ['kubernetes'], ['proxmox-pmg']])(
      'returns the recognized canonical type %s directly when platform is empty',
      (type) => {
        expect(resolveStorageSourceKey(makeStorage({ platform: '', type }))).toBe(type);
      },
    );

    it.each([['cephfs'], ['rbd']])(
      'collapses the ceph family member %s to ceph via CEPH_TYPES',
      (type) => {
        expect(resolveStorageSourceKey(makeStorage({ platform: '', type }))).toBe('ceph');
      },
    );
  });

  describe('resolveStorageSourceKey — proxmox-pve fallback bucket', () => {
    it.each([
      ['dir'],
      ['nfs'],
      ['cifs'],
      ['lvm'],
      ['lvmthin'],
      ['zfspool'],
      ['zfs'],
      ['iscsi'],
      ['glusterfs'],
      ['btrfs'],
    ])('classifies proxmox storage type %s as proxmox-pve', (type) => {
      expect(resolveStorageSourceKey(makeStorage({ platform: '', type }))).toBe('proxmox-pve');
    });

    it.each([[''], ['storage']])(
      'falls back to proxmox-pve for the generic type bucket %s',
      (type) => {
        expect(resolveStorageSourceKey(makeStorage({ platform: '', type }))).toBe('proxmox-pve');
      },
    );
  });

  describe('resolveStorageSourceKey — kubernetes substring classifier', () => {
    it.each([
      ['csi-driver', 'csi'],
      ['my-k8s-pv', 'k8s'],
      ['kubernetes-csi', 'kubernetes'],
      ['foo-csi-bar', 'csi'],
    ])('classifies type %s as kubernetes via the %s substring arm', (type) => {
      expect(resolveStorageSourceKey(makeStorage({ platform: '', type }))).toBe('kubernetes');
    });

    it('does not classify a type containing none of k8s/kubernetes/csi and returns it slugified', () => {
      expect(resolveStorageSourceKey(makeStorage({ platform: '', type: 'random-thing' }))).toBe(
        'random-thing',
      );
    });
  });

  describe('resolveStorageSourceKey — final fallback', () => {
    it('returns proxmox-pve when neither platform nor type resolve to anything', () => {
      expect(resolveStorageSourceKey(makeStorage({ platform: '', type: '' }))).toBe('proxmox-pve');
    });

    it('coerces a missing on-disk type field to empty and falls back to proxmox-pve', () => {
      // Defensive branch: normalizeStorageSourceKey receives undefined (the
      // `type` field is required on `Storage`, so we cast a malformed payload
      // through `unknown` to exercise the `value || ''` coercion).
      const storage = { ...makeStorage({ platform: '' }), type: undefined } as unknown as Storage;
      expect(resolveStorageSourceKey(storage)).toBe('proxmox-pve');
    });
  });

  describe('getStorageSourceOption — all branch', () => {
    it('returns the canonical all option for the normalized all token', () => {
      expect(getStorageSourceOption('all')).toStrictEqual({
        key: 'all',
        label: 'All sources',
        tone: 'slate',
      });
    });

    it('normalizes whitespace/case variants of the all token to the canonical all option', () => {
      expect(getStorageSourceOption('  ALL  ')).toStrictEqual({
        key: 'all',
        label: 'All sources',
        tone: 'slate',
      });
    });
  });

  describe('getStorageSourceOption — label ternary', () => {
    it('uses the literal "Ceph" label for the ceph ternary arm', () => {
      expect(getStorageSourceOption('ceph')).toStrictEqual({
        key: 'ceph',
        label: 'Ceph',
        tone: 'teal',
      });
    });

    it('uses getSourcePlatformLabel for the non-ceph ternary arm (microsoft-hyperv)', () => {
      expect(getStorageSourceOption('microsoft-hyperv')).toStrictEqual({
        key: 'microsoft-hyperv',
        label: 'Hyper-V',
        tone: 'slate',
      });
    });

    it.each([
      ['proxmox-pve', 'PVE', 'orange'],
      ['proxmox-pbs', 'PBS', 'indigo'],
      ['proxmox-pmg', 'PMG', 'rose'],
      ['truenas', 'TrueNAS', 'blue'],
      ['kubernetes', 'K8s', 'cyan'],
    ])('returns the platform label and mapped tone for canonical key %s', (key, label, tone) => {
      expect(getStorageSourceOption(key)).toStrictEqual({ key, label, tone });
    });
  });

  describe('getStorageSourceOption — tone fallback', () => {
    it('falls back to the slate tone for vmware-vsphere (not in the tone map)', () => {
      expect(getStorageSourceOption('vmware-vsphere')).toStrictEqual({
        key: 'vmware-vsphere',
        label: 'vSphere',
        tone: 'slate',
      });
    });

    it('falls back to the slate tone and a title-cased label for an unknown key', () => {
      expect(getStorageSourceOption('foo-bar')).toStrictEqual({
        key: 'foo-bar',
        label: 'Foo Bar',
        tone: 'slate',
      });
    });

    it('returns the Unknown label and slate tone for an empty key', () => {
      expect(getStorageSourceOption('')).toStrictEqual({
        key: '',
        label: 'Unknown',
        tone: 'slate',
      });
    });

    it('normalizes null/undefined to an empty key with the Unknown label', () => {
      expect(getStorageSourceOption(null)).toStrictEqual({
        key: '',
        label: 'Unknown',
        tone: 'slate',
      });
      expect(getStorageSourceOption(undefined)).toStrictEqual({
        key: '',
        label: 'Unknown',
        tone: 'slate',
      });
    });
  });

  describe('getStorageSourceOption — return shape contract', () => {
    it('every StorageSourceTone declared in the tone map is reachable and typed correctly', () => {
      const keys: string[] = [
        'proxmox-pve',
        'proxmox-pbs',
        'proxmox-pmg',
        'ceph',
        'truenas',
        'kubernetes',
      ];
      const tones = keys.map((key) => getStorageSourceOption(key).tone);
      expect(tones).toStrictEqual(['orange', 'indigo', 'rose', 'teal', 'blue', 'cyan']);
      // Type-level check: every option satisfies the StorageSourceOption interface.
      const option = getStorageSourceOption('proxmox-pve') as StorageSourceOption;
      expect(option.tone).toBe('orange');
    });
  });
});
