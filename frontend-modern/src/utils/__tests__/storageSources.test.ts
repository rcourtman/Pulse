import { describe, expect, it } from 'vitest';
import type { Storage } from '@/types/api';
import {
  buildStorageSourceOptionsFromKeys,
  getStorageSourceOption,
  normalizeStorageSourceKey,
  orderStorageSourceKeys,
  resolveStorageSourceKey,
  type StorageSourceOption,
} from '@/utils/storageSources';

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

describe('storageSources', () => {
  describe('normalizeStorageSourceKey', () => {
    it.each([
      ['k8s', 'kubernetes'],
      ['pve', 'proxmox-pve'],
      ['pbs', 'proxmox-pbs'],
      ['pmg', 'proxmox-pmg'],
      ['vmware', 'vmware-vsphere'],
      ['hyper-v', 'microsoft-hyperv'],
      ['truenas', 'truenas'],
      ['agent', 'agent'],
      ['docker', 'docker'],
      ['kubernetes', 'kubernetes'],
    ])('canonicalizes known platform token %s to %s', (input, expected) => {
      expect(normalizeStorageSourceKey(input)).toBe(expected);
    });

    it.each([
      ['ceph', 'ceph'],
      ['cephfs', 'ceph'],
      ['rbd', 'ceph'],
      ['CephFS', 'ceph'],
      ['RBD', 'ceph'],
    ])('collapses the ceph family %s to ceph', (input, expected) => {
      expect(normalizeStorageSourceKey(input)).toBe(expected);
    });

    it('preserves the all token', () => {
      expect(normalizeStorageSourceKey('all')).toBe('all');
      expect(normalizeStorageSourceKey(' ALL ')).toBe('all');
      expect(normalizeStorageSourceKey('All')).toBe('all');
    });

    it.each([
      [''],
      ['   '],
      [undefined],
      [null],
      ['!!!'],
      ['@@@@'],
    ])('returns an empty string for empty or punctuation-only input %s', (input) => {
      expect(normalizeStorageSourceKey(input as string | null | undefined)).toBe('');
    });

    it('slugifies mixed-case and whitespace into a canonical slug', () => {
      expect(normalizeStorageSourceKey('TrueNAS')).toBe('truenas');
      expect(normalizeStorageSourceKey('Proxmox PBS')).toBe('proxmox-pbs');
      expect(normalizeStorageSourceKey('  CEPH  ')).toBe('ceph');
    });

    it('passes unknown slugs through slugified', () => {
      expect(normalizeStorageSourceKey('Ceph FS')).toBe('ceph-fs');
      expect(normalizeStorageSourceKey('foo bar')).toBe('foo-bar');
      expect(normalizeStorageSourceKey('weird/thing!!')).toBe('weird-thing');
    });
  });

  describe('resolveStorageSourceKey', () => {
    it.each([
      ['truenas', 'zfs', 'truenas'],
      ['vmware-vsphere', 'vsan', 'vmware-vsphere'],
      ['proxmox-pbs', 'lvm', 'proxmox-pbs'],
      ['proxmox-pmg', '', 'proxmox-pmg'],
      ['kubernetes', 'csi', 'kubernetes'],
      ['microsoft-hyperv', '', 'microsoft-hyperv'],
    ])(
      'prefers the canonical platform tag %s over on-disk type %s -> %s',
      (platform, type, expected) => {
        expect(resolveStorageSourceKey(makeStorage({ platform, type }))).toBe(expected);
      },
    );

    it('canonicalizes a raw vmware platform alias before resolving', () => {
      expect(resolveStorageSourceKey(makeStorage({ platform: 'vmware', type: 'vmfs' }))).toBe(
        'vmware-vsphere',
      );
    });

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
    ])('resolves proxmox storage type %s to proxmox-pve', (type) => {
      expect(resolveStorageSourceKey(makeStorage({ platform: '', type }))).toBe('proxmox-pve');
    });

    it.each([[''], ['storage']])(
      'falls back to proxmox-pve for empty/generic type %s',
      (type) => {
        expect(resolveStorageSourceKey(makeStorage({ platform: '', type }))).toBe('proxmox-pve');
      },
    );

    it.each([['ceph'], ['cephfs'], ['rbd']])('resolves ceph type %s to ceph', (type) => {
      expect(resolveStorageSourceKey(makeStorage({ platform: '', type }))).toBe('ceph');
    });

    it.each([['proxmox-pbs'], ['ceph'], ['kubernetes'], ['proxmox-pmg']])(
      'returns recognized canonical type %s directly',
      (type) => {
        expect(resolveStorageSourceKey(makeStorage({ platform: '', type }))).toBe(type);
      },
    );

    it.each([['csi-driver'], ['my-k8s-pv'], ['kubernetes-csi'], ['foo-csi-bar']])(
      'classifies kubernetes-flavored type %s as kubernetes',
      (type) => {
        expect(resolveStorageSourceKey(makeStorage({ platform: '', type }))).toBe('kubernetes');
      },
    );

    it('returns an unknown on-disk type slugified as-is', () => {
      expect(resolveStorageSourceKey(makeStorage({ platform: '', type: 'random-thing' }))).toBe(
        'random-thing',
      );
    });

    it('returns proxmox-pve when neither platform nor type resolve to anything', () => {
      expect(resolveStorageSourceKey(makeStorage({ platform: '', type: '' }))).toBe('proxmox-pve');
    });

    it('ignores a proxmox-pve platform tag and resolves by type instead (current behavior)', () => {
      // proxmox-pve is intentionally NOT in the platform-wins list, so the
      // on-disk type still drives the result.
      expect(resolveStorageSourceKey(makeStorage({ platform: 'proxmox-pve', type: 'ceph' }))).toBe(
        'ceph',
      );
      expect(resolveStorageSourceKey(makeStorage({ platform: 'proxmox-pve', type: 'dir' }))).toBe(
        'proxmox-pve',
      );
    });

    it('ignores an agent platform tag and resolves by type instead (current behavior)', () => {
      expect(resolveStorageSourceKey(makeStorage({ platform: 'agent', type: 'dir' }))).toBe(
        'proxmox-pve',
      );
    });

    it('keeps a truenas platform tag even when the on-disk type is empty', () => {
      expect(resolveStorageSourceKey(makeStorage({ platform: 'truenas', type: '' }))).toBe(
        'truenas',
      );
    });
  });

  describe('getStorageSourceOption', () => {
    it('returns the canonical all option for the all token', () => {
      expect(getStorageSourceOption('all')).toEqual({
        key: 'all',
        label: 'All sources',
        tone: 'slate',
      });
      expect(getStorageSourceOption('ALL')).toEqual({
        key: 'all',
        label: 'All sources',
        tone: 'slate',
      });
    });

    it('does not treat undefined as the all token (current behavior)', () => {
      // Only the literal 'all' slug maps to the all option; undefined
      // normalizes to an empty key and falls through to the generic branch.
      expect(getStorageSourceOption(undefined)).toEqual({
        key: '',
        label: 'Unknown',
        tone: 'slate',
      });
      expect(getStorageSourceOption(null)).toEqual({
        key: '',
        label: 'Unknown',
        tone: 'slate',
      });
    });

    it.each([
      ['ceph', 'Ceph', 'teal'],
      ['proxmox-pve', 'PVE', 'orange'],
      ['proxmox-pbs', 'PBS', 'indigo'],
      ['proxmox-pmg', 'PMG', 'rose'],
      ['truenas', 'TrueNAS', 'blue'],
      ['kubernetes', 'K8s', 'cyan'],
    ])('builds a labeled, toned option for canonical key %s', (key, label, tone) => {
      expect(getStorageSourceOption(key)).toEqual({ key, label, tone });
    });

    it('uses the vSphere label and falls back to the slate tone for vmware-vsphere', () => {
      // vmware-vsphere has no entry in the storage source tone map, so it
      // falls back to slate while keeping its canonical platform label.
      expect(getStorageSourceOption('vmware-vsphere')).toEqual({
        key: 'vmware-vsphere',
        label: 'vSphere',
        tone: 'slate',
      });
    });

    it('title-cases the label for an unknown key and uses the slate tone', () => {
      expect(getStorageSourceOption('foo-bar')).toEqual({
        key: 'foo-bar',
        label: 'Foo Bar',
        tone: 'slate',
      });
    });

    it('returns the Unknown label for an empty key', () => {
      const option = getStorageSourceOption('');
      expect(option.key).toBe('');
      expect(option.label).toBe('Unknown');
      expect(option.tone).toBe('slate');
    });

    it('every option key/tone pair is a valid StorageSourceOption', () => {
      const option = getStorageSourceOption('ceph') as StorageSourceOption;
      expect(option.tone).toBe('teal');
      expect(typeof option.label).toBe('string');
    });
  });

  describe('orderStorageSourceKeys', () => {
    it('orders canonical source keys by the preferred source ordering', () => {
      expect(
        orderStorageSourceKeys(['truenas', 'proxmox-pve', 'ceph', 'kubernetes', 'proxmox-pbs']),
      ).toEqual(['proxmox-pve', 'proxmox-pbs', 'ceph', 'truenas', 'kubernetes']);
    });

    it('returns an empty array for empty input', () => {
      expect(orderStorageSourceKeys([])).toEqual([]);
      expect(orderStorageSourceKeys(new Set<string>())).toEqual([]);
    });

    it('dedupes and normalizes keys before ordering', () => {
      expect(orderStorageSourceKeys(['pbs', 'proxmox-pbs', 'PBS'])).toEqual(['proxmox-pbs']);
    });

    it('places the all token after canonical keys (all is not in the source order)', () => {
      expect(orderStorageSourceKeys(['all', 'ceph', 'proxmox-pve'])).toEqual([
        'proxmox-pve',
        'ceph',
        'all',
      ]);
    });

    it('sorts unknown keys after canonical keys using locale-aware ordering', () => {
      expect(orderStorageSourceKeys(['zeta', 'proxmox-pve', 'alpha', 'ceph'])).toEqual([
        'proxmox-pve',
        'ceph',
        'alpha',
        'zeta',
      ]);
    });

    it('keeps a single canonical key as-is', () => {
      expect(orderStorageSourceKeys(['truenas'])).toEqual(['truenas']);
    });
  });

  describe('buildStorageSourceOptionsFromKeys', () => {
    const ALL_OPTION: StorageSourceOption = {
      key: 'all',
      label: 'All sources',
      tone: 'slate',
    };

    it('returns only the all option for empty input', () => {
      expect(buildStorageSourceOptionsFromKeys([])).toEqual([ALL_OPTION]);
    });

    it('prepends the all option and orders the remaining canonical keys', () => {
      expect(buildStorageSourceOptionsFromKeys(['ceph', 'proxmox-pve', 'truenas'])).toEqual([
        ALL_OPTION,
        { key: 'proxmox-pve', label: 'PVE', tone: 'orange' },
        { key: 'ceph', label: 'Ceph', tone: 'teal' },
        { key: 'truenas', label: 'TrueNAS', tone: 'blue' },
      ]);
    });

    it('strips an explicit all key from the input before building options', () => {
      expect(buildStorageSourceOptionsFromKeys(['all', 'truenas'])).toEqual([
        ALL_OPTION,
        { key: 'truenas', label: 'TrueNAS', tone: 'blue' },
      ]);
    });

    it('dedupes repeated keys', () => {
      const options = buildStorageSourceOptionsFromKeys(['ceph', 'ceph', 'ceph']);
      expect(options).toEqual([ALL_OPTION, { key: 'ceph', label: 'Ceph', tone: 'teal' }]);
    });

    it('first option is always the canonical all option', () => {
      const options = buildStorageSourceOptionsFromKeys(['kubernetes', 'proxmox-pbs']);
      expect(options[0]).toEqual(ALL_OPTION);
      expect(options).toHaveLength(3);
    });
  });
});
