import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getActiveStorageNodeOptions,
  getStorageFilterGroupBy,
  storageResourceMatchesSourceFilter,
  syncExpandedStorageGroups,
  toStorageHealthFilterValue,
  type StoragePageNodeOption,
  type StorageStatusFilterValue,
  type StorageView,
} from '@/components/Storage/storagePageState';

const makeNodeOption = (overrides: Partial<StoragePageNodeOption> = {}): StoragePageNodeOption => ({
  id: 'node-1',
  label: 'pve1',
  ...overrides,
});

const makeFilterableResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'node-1',
    type: 'agent',
    name: 'pve1',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    platformData: { sources: ['proxmox-pve', 'agent'] },
    ...overrides,
  }) as Resource;

describe('storagePageState branch coverage 0723pm', () => {
  describe('getActiveStorageNodeOptions', () => {
    it('returns the diskNodeOptions array (by reference) when view is "disks"', () => {
      const nodes = [makeNodeOption({ id: 'node-1', label: 'pve1' })];
      const disks = [makeNodeOption({ id: 'node-2', label: 'pve2' })];
      // Reference equality proves the function performs no defensive copy and
      // selects the disk set, not the pool set.
      expect(getActiveStorageNodeOptions('disks', nodes, disks)).toBe(disks);
    });

    it('returns the nodeOptions array (by reference) when view is "pools"', () => {
      const nodes = [makeNodeOption({ id: 'node-1', label: 'pve1' })];
      const disks = [makeNodeOption({ id: 'node-2', label: 'pve2' })];
      expect(getActiveStorageNodeOptions('pools', nodes, disks)).toBe(nodes);
    });

    it('preserves a several-node ordering without sorting', () => {
      const disks = [
        makeNodeOption({ id: 'c', label: 'gamma' }),
        makeNodeOption({ id: 'a', label: 'alpha' }),
        makeNodeOption({ id: 'b', label: 'beta' }),
      ];
      const result = getActiveStorageNodeOptions('disks', [], disks);
      expect(result.map((node) => node.id)).toEqual(['c', 'a', 'b']);
    });

    it('does NOT dedupe duplicate ids or labels (returns them as-is)', () => {
      // The function is a pure selector: it performs no deduplication.
      const disks = [
        makeNodeOption({ id: 'dup', label: 'same' }),
        makeNodeOption({ id: 'dup', label: 'same' }),
      ];
      const result = getActiveStorageNodeOptions('disks', [], disks);
      expect(result).toHaveLength(2);
      expect(result.map((node) => node.id)).toEqual(['dup', 'dup']);
    });

    it('preserves options whose label is missing/empty (no filtering by name)', () => {
      const disks = [
        makeNodeOption({ id: 'has-name', label: 'pve1' }),
        makeNodeOption({ id: 'no-name', label: '' }),
      ];
      const result = getActiveStorageNodeOptions('disks', [], disks);
      expect(result.map((node) => node.id)).toEqual(['has-name', 'no-name']);
    });

    it('treats an unrecognized view string as pools (returns nodeOptions)', () => {
      // StorageView is a closed union, but the function only special-cases
      // the literal "disks"; every other value resolves to the pool set.
      const nodes = [makeNodeOption({ id: 'node-1', label: 'pve1' })];
      const disks = [makeNodeOption({ id: 'node-2', label: 'pve2' })];
      const result = getActiveStorageNodeOptions('banana' as unknown as StorageView, nodes, disks);
      expect(result).toBe(nodes);
    });
  });

  describe('getStorageFilterGroupBy (ternary true arm)', () => {
    // Existing coverage only exercised value === 'node', which hits the false
    // arm (default). These exercise the true arm: an accepted canonical key is
    // returned unchanged.
    it.each(['type', 'status', 'none'] as const)(
      'returns %s unchanged when it is a recognized group key (true arm)',
      (value) => {
        expect(getStorageFilterGroupBy(value)).toBe(value);
      },
    );
  });

  describe('toStorageHealthFilterValue (value === "all" arm)', () => {
    // Existing coverage exercised attention/available and the fallthrough, but
    // never the literal "all" early-return.
    it('returns "all" when the value is "all"', () => {
      const input: StorageStatusFilterValue = 'all';
      expect(toStorageHealthFilterValue(input)).toBe('all');
    });
  });

  describe('storageResourceMatchesSourceFilter (sources-is-not-an-array arm)', () => {
    it('falls back to the empty sources list when platformData.sources is missing', () => {
      // Array.isArray(undefined) === false => the `: []` arm fires, so matching
      // must rely on platformType / sourceType only.
      const node = makeFilterableResource({ platformData: {} });
      expect(storageResourceMatchesSourceFilter(node, 'proxmox-pve')).toBe(true);
      expect(storageResourceMatchesSourceFilter(node, 'truenas')).toBe(false);
    });

    it('falls back to the empty sources list when platformData.sources is a non-array', () => {
      const node = makeFilterableResource({
        platformData: { sources: 'proxmox-pve' } as unknown as Resource['platformData'],
      });
      // platformType / sourceType still match even though the bogus sources value is ignored.
      expect(storageResourceMatchesSourceFilter(node, 'proxmox-pve')).toBe(true);
      expect(storageResourceMatchesSourceFilter(node, 'api')).toBe(true);
      expect(storageResourceMatchesSourceFilter(node, 'agent')).toBe(false);
    });

    it('matches via sourceType alone when platformData is undefined', () => {
      const node = makeFilterableResource({
        platformData: undefined,
        platformType: undefined,
        sourceType: 'agent',
      });
      expect(storageResourceMatchesSourceFilter(node, 'agent')).toBe(true);
      expect(storageResourceMatchesSourceFilter(node, 'proxmox-pve')).toBe(false);
    });
  });

  describe('syncExpandedStorageGroups (changed=false arm)', () => {
    it('returns the previous set unchanged (same reference) when every key is already present', () => {
      // No new keys => `changed` stays false => the `: previous` arm fires and
      // the input reference is returned verbatim.
      const previous = new Set(['A', 'B']);
      const result = syncExpandedStorageGroups(previous, ['A', 'B']);
      expect(result).toBe(previous);
      expect(result).toEqual(new Set(['A', 'B']));
    });

    it('returns previous unchanged when allKeys is empty but previous is non-empty', () => {
      const previous = new Set(['A']);
      const result = syncExpandedStorageGroups(previous, []);
      expect(result).toBe(previous);
    });
  });
});
